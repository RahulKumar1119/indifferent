/**
 * Uploads a Shorts source to S3 (issue #151).
 *
 * Files up to one part use the single presigned PUT the page always used.
 * Larger files go up as 16 MB parts, several at a time. Each part retries on
 * its own, a dead connection is told apart from a slow one by throughput, and
 * every finished part is saved so a drop, a reload or a closed tab continues
 * from the last finished part instead of starting over.
 */
import { firstValueFrom } from 'rxjs';
import type {
  MultipartCompletedPart,
  MultipartInitiateResponse,
  MultipartPartsResponse,
  MultipartStoredPart,
  ShortsService,
} from './shorts.service';
import {
  PersistedUpload,
  fingerprintOf,
  recordUploadSpeed,
  removeUpload,
  saveUpload,
} from './shorts-upload-state';

/** Sources up to this size keep the single presigned PUT. */
export const MULTIPART_THRESHOLD_BYTES = 16 * 1024 * 1024;
/** Parts in flight at once. */
export const UPLOAD_CONCURRENCY = 5;
/** Part URLs requested together, as parts are scheduled. */
export const PART_URL_BATCH = 8;
/** Tries per part, and per single PUT, before the upload round fails. */
export const MAX_PART_ATTEMPTS = 5;
/** Times the whole upload is picked up again after a round fails. */
export const MAX_UPLOAD_ROUNDS = 4;
/** Fresh URLs a part may ask for after a 403 before that counts as a failed try. */
export const URL_REFRESHES_PER_PART = 3;
/** A transfer is dead only if it moved less than the minimum rate over this whole window. */
export const DEAD_WINDOW_MS = 180_000;
export const DEAD_MIN_BYTES_PER_SEC = 1024;
/** Some networks block the S3 accelerate host. After this many accelerated sends in a row that never reach it, the rest of the upload asks for regular URLs. */
export const ACCELERATE_BLOCKED_AFTER = 3;

const RETRY_BASE_MS = 1000;
const RETRY_MAX_MS = 30_000;
const ROUND_DELAYS_MS = [5_000, 20_000, 60_000];
const OFFLINE_WAIT_MS = 10 * 60_000;
const PROGRESS_INTERVAL_MS = 250;
const SPEED_WINDOW_MS = 8_000;

export type UploadFailure =
  /** Connection trouble that outlasted every retry. Saved parts allow a resume. */
  | 'network'
  /** The server refused the file or the request. Retrying cannot help. */
  | 'rejected'
  /** The saved upload is gone (job deleted, upload aborted or expired). */
  | 'session-lost'
  /** The sign-in ended. Saved parts allow a resume after signing in. */
  | 'auth'
  /** The caller stopped the upload. */
  | 'aborted'
  /** The job already left the upload stage. */
  | 'already-started';

export class UploadError extends Error {
  constructor(
    readonly kind: UploadFailure,
    message: string,
  ) {
    super(message);
    this.name = 'UploadError';
  }
}

export interface UploadProgress {
  bytesDone: number;
  bytesTotal: number;
  partsDone: number;
  partsTotal: number;
  partsActive: number;
  /** Recent speed in bytes per second, 0 until it can be measured. */
  bytesPerSecond: number;
  status: 'uploading' | 'retrying' | 'offline' | 'finishing';
  /** Set while a part is waiting for its next try. */
  retry?: { part: number; attempt: number; of: number };
}

/** The API calls the uploader needs, as promises. */
export interface MultipartApi {
  initiate(jobId: string, size: number): Promise<MultipartInitiateResponse>;
  partUrls(
    jobId: string,
    uploadId: string,
    partNumbers: number[],
    accelerate?: boolean,
  ): Promise<MultipartPartsResponse>;
  listParts(jobId: string, uploadId: string): Promise<{ parts: MultipartStoredPart[] }>;
  complete(jobId: string, uploadId: string, parts: MultipartCompletedPart[]): Promise<unknown>;
}

export function multipartApi(shorts: ShortsService): MultipartApi {
  return {
    initiate: (jobId, size) => firstValueFrom(shorts.initiateMultipart(jobId, size)),
    partUrls: (jobId, uploadId, nums, accelerate) =>
      firstValueFrom(shorts.getPartUrls(jobId, uploadId, nums, accelerate)),
    listParts: (jobId, uploadId) => firstValueFrom(shorts.listMultipartParts(jobId, uploadId)),
    complete: (jobId, uploadId, parts) => firstValueFrom(shorts.completeMultipart(jobId, uploadId, parts)),
  };
}

export interface UploaderOptions {
  concurrency: number;
  maxPartAttempts: number;
  maxRounds: number;
  deadWindowMs: number;
  deadMinBytesPerSec: number;
  retryBaseMs: number;
  roundDelaysMs: number[];
  onProgress: (progress: UploadProgress) => void;
}

export interface UploadJob {
  jobId: string;
  /** The single-PUT URL from POST /shorts. Not needed to resume a multipart upload. */
  uploadUrl?: string;
  fileType: string;
  durationSeconds: number;
  language: string;
}

/** Exponential backoff with jitter: the delay doubles per attempt, then lands between half and full. */
export function backoffDelay(attempt: number, baseMs: number, maxMs = RETRY_MAX_MS, random = Math.random): number {
  const ceiling = Math.min(maxMs, baseMs * 2 ** Math.max(0, attempt - 1));
  return Math.round(ceiling * (0.5 + random() * 0.5));
}

/**
 * Tells a dead connection from a slow one. Throughput is measured over a long
 * window; only a transfer that moved less than the minimum rate across the
 * whole window counts as dead. Any steady trickle keeps it alive.
 */
export class StallDetector {
  private samples: { t: number; loaded: number }[] = [];

  constructor(
    private readonly windowMs: number,
    private readonly minBytesPerSec: number,
    private readonly now: () => number = Date.now,
  ) {
    this.record(0);
  }

  record(loaded: number): void {
    this.samples.push({ t: this.now(), loaded });
  }

  isDead(): boolean {
    const now = this.now();
    const cutoff = now - this.windowMs;
    let ref: { t: number; loaded: number } | null = null;
    for (const s of this.samples) {
      if (s.t <= cutoff) ref = s;
      else break;
    }
    // Less history than one window cannot prove anything.
    if (!ref) return false;
    this.samples = this.samples.slice(this.samples.indexOf(ref));
    const latest = this.samples[this.samples.length - 1];
    return latest.loaded - ref.loaded < (this.minBytesPerSec * this.windowMs) / 1000;
  }
}

type PutFailureKind = 'network' | 'stalled' | 'aborted';

class PutFailure extends Error {
  constructor(readonly kind: PutFailureKind) {
    super(kind);
  }
}

interface PutResult {
  status: number;
  etag: string | null;
}

interface PutOptions {
  signal: AbortSignal;
  deadWindowMs: number;
  deadMinBytesPerSec: number;
  onProgress: (loaded: number) => void;
}

/** PUT a blob with upload progress. XHR is used because fetch cannot report upload progress. */
function putBlob(url: string, body: Blob, o: PutOptions): Promise<PutResult> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    const detector = new StallDetector(o.deadWindowMs, o.deadMinBytesPerSec);
    let loaded = 0;
    let settled = false;
    const finish = (settle: () => void): void => {
      if (settled) return;
      settled = true;
      clearInterval(tick);
      o.signal.removeEventListener('abort', onAbort);
      settle();
    };
    const onAbort = (): void => {
      xhr.abort();
      finish(() => reject(new PutFailure('aborted')));
    };
    const tick = setInterval(() => {
      detector.record(loaded);
      if (detector.isDead()) {
        xhr.abort();
        finish(() => reject(new PutFailure('stalled')));
      }
    }, 1000);

    xhr.open('PUT', url, true);
    xhr.setRequestHeader('ngsw-bypass', 'true');
    xhr.upload.onprogress = (event) => {
      loaded = event.loaded;
      detector.record(loaded);
      o.onProgress(loaded);
    };
    xhr.onload = () => finish(() => resolve({ status: xhr.status, etag: xhr.getResponseHeader('ETag') }));
    xhr.onerror = () => finish(() => reject(new PutFailure('network')));
    o.signal.addEventListener('abort', onAbort, { once: true });
    if (o.signal.aborted) {
      onAbort();
      return;
    }
    xhr.send(body);
  });
}

function abortedError(): UploadError {
  return new UploadError('aborted', 'Upload stopped.');
}

function sleep(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) return reject(abortedError());
    const onAbort = (): void => {
      clearTimeout(timer);
      reject(abortedError());
    };
    const timer = setTimeout(() => {
      signal.removeEventListener('abort', onAbort);
      resolve();
    }, ms);
    signal.addEventListener('abort', onAbort, { once: true });
  });
}

/** Resolves once the browser is online again. Gives up waiting after OFFLINE_WAIT_MS and lets the next try fail. */
function waitForOnline(signal: AbortSignal): Promise<void> {
  if (typeof navigator === 'undefined' || navigator.onLine) return Promise.resolve();
  return new Promise((resolve, reject) => {
    const done = (): void => {
      window.removeEventListener('online', done);
      signal.removeEventListener('abort', onAbort);
      clearTimeout(timer);
      resolve();
    };
    const onAbort = (): void => {
      window.removeEventListener('online', done);
      clearTimeout(timer);
      reject(abortedError());
    };
    const timer = setTimeout(done, OFFLINE_WAIT_MS);
    window.addEventListener('online', done);
    signal.addEventListener('abort', onAbort, { once: true });
  });
}

interface ApiErrorShape {
  status?: number;
  error?: { code?: string; message?: string };
}

/** Turns a failed API call into the failure the caller handles. */
function toUploadError(err: unknown, fallback: string): UploadError {
  if (err instanceof UploadError) return err;
  const e = err as ApiErrorShape;
  const status = e?.status ?? 0;
  const code = e?.error?.code ?? '';
  const message = e?.error?.message || fallback;
  if (status === 401) return new UploadError('auth', 'Your sign-in expired. Sign in again to continue.');
  if (status === 404 && (code === 'UPLOAD_SESSION_NOT_FOUND' || code === 'NOT_FOUND')) {
    return new UploadError('session-lost', message);
  }
  if (status === 400 && code === 'INVALID_STATE') return new UploadError('already-started', message);
  if (status === 400 && code === 'INVALID_PARTS') return new UploadError('network', message);
  if (status === 0 || status === 408 || status === 429 || status >= 500) return new UploadError('network', message);
  return new UploadError('rejected', message);
}

export class ShortsUploader {
  private readonly o: UploaderOptions;
  private readonly abortController = new AbortController();
  /** The multipart upload in progress. Stays null for the single PUT. */
  private state: PersistedUpload | null = null;
  private readonly etags = new Map<number, string>();
  private readonly unknownEtag = new Set<number>();
  private readonly urls = new Map<number, Promise<string>>();
  /** Parts whose current URL points at the accelerate host, as the server reports it. */
  private readonly acceleratedParts = new Set<number>();
  /** Accelerated sends in a row that got no reply and had sent no byte. Lives for this upload only. */
  private blockedSends = 0;
  private regularUrlsOnly = false;
  private queue: number[] = [];
  private halted = false;
  private done = false;

  private readonly inflight = new Map<number, number>();
  private completedBytes = 0;
  private movedBytes = 0;
  private speedSamples: { t: number; bytes: number }[] = [];
  private lastEmit = 0;
  private status: UploadProgress['status'] = 'uploading';
  private retrying: UploadProgress['retry'];
  private cleanRun = true;
  private startedAt = 0;

  constructor(
    private readonly api: MultipartApi,
    private readonly file: File,
    private readonly job: UploadJob,
    options: Partial<UploaderOptions> = {},
  ) {
    this.o = {
      concurrency: UPLOAD_CONCURRENCY,
      maxPartAttempts: MAX_PART_ATTEMPTS,
      maxRounds: MAX_UPLOAD_ROUNDS,
      deadWindowMs: DEAD_WINDOW_MS,
      deadMinBytesPerSec: DEAD_MIN_BYTES_PER_SEC,
      retryBaseMs: RETRY_BASE_MS,
      roundDelaysMs: ROUND_DELAYS_MS,
      onProgress: () => {},
      ...options,
    };
  }

  /** Uploads the file. Pass the saved state to continue an earlier multipart upload. */
  async run(resume: PersistedUpload | null = null): Promise<void> {
    this.startedAt = Date.now();
    if (resume) {
      this.adopt(resume);
      return this.runMultipart(true);
    }
    if (this.file.size <= MULTIPART_THRESHOLD_BYTES) return this.runSinglePut();
    try {
      const init = await this.api.initiate(this.job.jobId, this.file.size);
      this.adopt({
        v: 1,
        jobId: this.job.jobId,
        uploadId: init.uploadId,
        partSize: init.partSize,
        partCount: init.partCount,
        fingerprint: fingerprintOf(this.file),
        fileType: this.job.fileType,
        durationSeconds: this.job.durationSeconds,
        language: this.job.language,
        etags: {},
        createdAt: Date.now(),
      });
      saveUpload(this.state!);
    } catch (err) {
      // The job was created a moment ago, so a 404 means the route is not deployed yet.
      if ((err as ApiErrorShape)?.status === 404 && this.job.uploadUrl) return this.runSinglePut();
      throw toUploadError(err, 'Could not start the upload. Please try again.');
    }
    return this.runMultipart(false);
  }

  /** Stops the transfers and keeps the saved state, so the upload can be resumed. */
  pause(): void {
    this.abortController.abort();
  }

  private get signal(): AbortSignal {
    return this.abortController.signal;
  }

  private adopt(state: PersistedUpload): void {
    this.state = state;
    this.etags.clear();
    for (const [n, etag] of Object.entries(state.etags)) this.etags.set(Number(n), etag);
    this.recount();
  }

  private partBytes(n: number): number {
    const s = this.state!;
    return n < s.partCount ? s.partSize : this.file.size - s.partSize * (s.partCount - 1);
  }

  private recount(): void {
    let bytes = 0;
    for (const n of this.etags.keys()) bytes += this.partBytes(n);
    this.completedBytes = bytes;
  }

  private persist(): void {
    const s = this.state!;
    s.etags = Object.fromEntries([...this.etags].filter(([, etag]) => etag).map(([n, etag]) => [String(n), etag]));
    saveUpload(s);
  }

  /** Files up to one part: the single PUT as before, now with retries and the throughput-based stall check. */
  private async runSinglePut(): Promise<void> {
    const url = this.job.uploadUrl;
    if (!url) throw new UploadError('rejected', 'No upload address for this file.');
    for (let attempt = 1; ; attempt++) {
      try {
        const result = await this.put(url, this.file, 1);
        if (result.status >= 200 && result.status < 300) break;
        // The link cannot be renewed here, and a new job gets a new link.
        if (result.status === 403) throw new UploadError('rejected', 'The upload link expired. Please try again.');
        if (result.status >= 500 || result.status === 408 || result.status === 429) throw new PutFailure('network');
        throw new UploadError('rejected', `Upload failed with status ${result.status}. Please try again.`);
      } catch (err) {
        this.inflight.delete(1);
        const kind = (err as { kind?: string }).kind;
        if (kind === 'aborted') throw abortedError();
        if (err instanceof UploadError && kind !== 'network') throw err;
        if (attempt >= this.o.maxPartAttempts) {
          throw new UploadError('network', 'Upload failed. Check your connection and try again.');
        }
        await this.pauseBeforeRetry(1, attempt);
      }
    }
    this.inflight.clear();
    this.completedBytes = this.file.size;
    this.finishMeasurement();
  }

  private async runMultipart(needsSync: boolean): Promise<void> {
    const jobId = this.state!.jobId;
    let sync = needsSync;
    for (let round = 1; ; round++) {
      try {
        if (sync) await this.sync();
        if (!this.done) {
          await this.uploadMissingParts();
          if (this.unknownEtag.size > 0) await this.sync();
        }
        if (!this.done) await this.complete();
        this.finishMeasurement();
        removeUpload(jobId);
        return;
      } catch (err) {
        const failure = toUploadError(err, 'Upload failed.');
        if (failure.kind === 'rejected' || failure.kind === 'session-lost' || failure.kind === 'already-started') {
          removeUpload(jobId);
        }
        if (failure.kind !== 'network' || round >= this.o.maxRounds) throw failure;
        this.cleanRun = false;
        this.status = 'retrying';
        this.emit(true);
        const delay = this.o.roundDelaysMs[Math.min(round - 1, this.o.roundDelaysMs.length - 1)] ?? 5_000;
        await sleep(Math.round(delay * (0.75 + Math.random() * 0.5)), this.signal);
        await this.waitOnline();
        sync = true;
      }
    }
  }

  /**
   * Replaces what the browser remembers with what S3 holds, so a part S3 does
   * not have is sent again.
   */
  private async sync(): Promise<void> {
    const s = this.state!;
    let stored: MultipartStoredPart[];
    try {
      stored = (await this.api.listParts(s.jobId, s.uploadId)).parts;
    } catch (err) {
      const failure = toUploadError(err, 'Could not check the saved upload.');
      // Every part was recorded but the upload is gone: the last complete call may have gone through.
      if (failure.kind === 'session-lost' && this.etags.size === s.partCount && this.unknownEtag.size === 0) {
        try {
          await this.complete();
          return;
        } catch {
          // Not finished after all: report the original failure.
        }
      }
      throw failure;
    }
    this.etags.clear();
    this.unknownEtag.clear();
    for (const p of stored) {
      if (p.partNumber >= 1 && p.partNumber <= s.partCount && p.size === this.partBytes(p.partNumber)) {
        this.etags.set(p.partNumber, p.etag);
      }
    }
    this.recount();
    this.persist();
  }

  private async uploadMissingParts(): Promise<void> {
    const s = this.state!;
    this.queue = [];
    for (let n = 1; n <= s.partCount; n++) if (!this.etags.has(n)) this.queue.push(n);
    this.halted = false;
    this.status = 'uploading';
    this.retrying = undefined;
    this.urls.clear();
    this.emit(true);

    let retryable: UploadError | null = null;
    let fatal: UploadError | null = null;
    const worker = async (): Promise<void> => {
      while (!this.halted && this.queue.length > 0) {
        const n = this.queue.shift()!;
        try {
          await this.uploadPart(n);
        } catch (err) {
          const failure = toUploadError(err, 'Upload failed.');
          this.halted = true;
          if (failure.kind === 'network') retryable ??= failure;
          else fatal ??= failure;
        }
      }
    };
    await Promise.all(Array.from({ length: Math.min(this.o.concurrency, this.queue.length) }, worker));
    if (this.signal.aborted) throw abortedError();
    if (fatal) throw fatal;
    if (retryable) throw retryable;
  }

  /** The URL for a part. One request covers the next few queued parts too, so URLs are fetched as parts get scheduled. */
  private urlFor(n: number): Promise<string> {
    const cached = this.urls.get(n);
    if (cached) return cached;
    const batch = [n];
    for (const m of this.queue) {
      if (batch.length >= PART_URL_BATCH) break;
      if (!this.urls.has(m)) batch.push(m);
    }
    const s = this.state!;
    const request = this.api.partUrls(s.jobId, s.uploadId, batch, this.regularUrlsOnly ? false : undefined);
    for (const m of batch) {
      const url = request.then((res) => {
        if (res.accelerated) this.acceleratedParts.add(m);
        else this.acceleratedParts.delete(m);
        const found = res.urls.find((u) => u.partNumber === m);
        if (!found) throw new UploadError('rejected', 'The server did not return an upload address.');
        return found.url;
      });
      url.catch(() => this.urls.delete(m));
      this.urls.set(m, url);
    }
    return this.urls.get(n)!;
  }

  private async uploadPart(n: number): Promise<void> {
    const s = this.state!;
    const start = (n - 1) * s.partSize;
    const body = this.file.slice(start, start + this.partBytes(n));
    let refreshes = 0;
    for (let attempt = 1; ; attempt++) {
      try {
        const url = await this.urlFor(n).catch((err) => {
          throw toUploadError(err, 'Could not get an upload address.');
        });
        const result = await this.put(url, body, n, this.acceleratedParts.has(n));
        if (result.status >= 200 && result.status < 300) {
          this.inflight.delete(n);
          if (result.etag) {
            this.etags.set(n, result.etag);
          } else {
            // The browser may not read the ETag header: S3 reports it when the parts are checked at the end.
            this.etags.set(n, '');
            this.unknownEtag.add(n);
          }
          this.completedBytes += this.partBytes(n);
          this.persist();
          this.emit(true);
          return;
        }
        if (result.status === 403 && refreshes < URL_REFRESHES_PER_PART) {
          // An expired URL costs a fresh one, not a try.
          refreshes++;
          this.urls.delete(n);
          attempt--;
          continue;
        }
        if (result.status === 404) throw new UploadError('session-lost', 'The upload session no longer exists.');
        if (result.status === 403 || result.status >= 500 || result.status === 408 || result.status === 429) {
          throw new PutFailure('network');
        }
        throw new UploadError('rejected', `Upload failed with status ${result.status}.`);
      } catch (err) {
        this.inflight.delete(n);
        const kind = (err as { kind?: string }).kind;
        if (kind === 'aborted') throw abortedError();
        if (err instanceof UploadError && kind !== 'network') throw err;
        if (attempt >= this.o.maxPartAttempts) throw new UploadError('network', 'Part upload failed.');
        this.urls.delete(n);
        await this.pauseBeforeRetry(n, attempt);
      }
    }
  }

  private async put(url: string, body: Blob, part: number, accelerated = false): Promise<PutResult> {
    await this.waitOnline();
    let previous = 0;
    try {
      const result = await putBlob(url, body, {
        signal: this.signal,
        deadWindowMs: this.o.deadWindowMs,
        deadMinBytesPerSec: this.o.deadMinBytesPerSec,
        onProgress: (loaded) => {
          this.movedBytes += loaded - previous;
          previous = loaded;
          this.inflight.set(part, loaded);
          this.emit(false);
        },
      });
      if (accelerated) this.blockedSends = 0; // any reply, even an error, proves the host is reachable
      return result;
    } catch (err) {
      // No reply and no byte sent: the host itself may be blocked. A drop mid-transfer is not that.
      const blocked = accelerated && (err as PutFailure).kind === 'network' && previous === 0;
      if (blocked && ++this.blockedSends >= ACCELERATE_BLOCKED_AFTER) {
        this.regularUrlsOnly = true;
        this.urls.clear();
      }
      throw err;
    }
  }

  private async pauseBeforeRetry(part: number, attempt: number): Promise<void> {
    this.cleanRun = false;
    this.retrying = { part, attempt: attempt + 1, of: this.o.maxPartAttempts };
    this.status = 'retrying';
    this.emit(true);
    await sleep(backoffDelay(attempt, this.o.retryBaseMs), this.signal);
    await this.waitOnline();
    this.retrying = undefined;
    this.status = 'uploading';
  }

  private async waitOnline(): Promise<void> {
    if (typeof navigator !== 'undefined' && !navigator.onLine) {
      this.cleanRun = false;
      this.status = 'offline';
      this.emit(true);
      await waitForOnline(this.signal);
      this.status = 'uploading';
    }
  }

  private async complete(): Promise<void> {
    const s = this.state!;
    this.status = 'finishing';
    this.emit(true);
    const parts: MultipartCompletedPart[] = [];
    for (let n = 1; n <= s.partCount; n++) parts.push({ partNumber: n, etag: this.etags.get(n) ?? '' });
    try {
      await this.api.complete(s.jobId, s.uploadId, parts);
    } catch (err) {
      throw toUploadError(err, 'Could not finish the upload.');
    }
    this.done = true;
  }

  private finishMeasurement(): void {
    this.status = 'finishing';
    this.emit(true);
    // A run that never hit trouble says what this connection can do; one that did would skew the next estimate.
    if (this.cleanRun) recordUploadSpeed(this.movedBytes, Date.now() - this.startedAt);
  }

  private emit(force: boolean): void {
    const now = Date.now();
    if (!force && now - this.lastEmit < PROGRESS_INTERVAL_MS) return;
    this.lastEmit = now;
    this.speedSamples.push({ t: now, bytes: this.movedBytes });
    while (this.speedSamples.length > 2 && now - this.speedSamples[1].t > SPEED_WINDOW_MS) this.speedSamples.shift();
    const first = this.speedSamples[0];
    const spanMs = now - first.t;
    let inflightBytes = 0;
    for (const loaded of this.inflight.values()) inflightBytes += loaded;
    this.o.onProgress({
      bytesDone: Math.min(this.file.size, this.completedBytes + inflightBytes),
      bytesTotal: this.file.size,
      partsDone: this.state ? this.etags.size : this.completedBytes >= this.file.size ? 1 : 0,
      partsTotal: this.state?.partCount ?? 1,
      partsActive: this.inflight.size,
      bytesPerSecond: spanMs >= 1000 ? ((this.movedBytes - first.bytes) / spanMs) * 1000 : 0,
      status: this.status,
      retry: this.retrying,
    });
  }
}
