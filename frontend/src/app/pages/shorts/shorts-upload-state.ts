/**
 * Browser-side state for large Shorts uploads (issue #151): the saved upload
 * that lets a reload continue where it stopped, and the measured upload speed
 * behind the time estimate. Storage failures (private mode, quota) are never
 * fatal: the upload just loses its ability to resume.
 */

const UPLOAD_KEY_PREFIX = 'shorts-upload:';
const SPEED_KEY = 'shorts-upload-speed';

/** S3 aborts open uploads after a day, so saved state older than that is dead. */
export const RESUME_TTL_MS = 24 * 60 * 60 * 1000;

/** Assumed speed until an upload has been measured: what a 200 MB upload reached in testing. */
export const DEFAULT_UPLOAD_BYTES_PER_SEC = 1024 * 1024;

/** From this size the page warns that the upload takes a while. */
export const LARGE_UPLOAD_WARNING_BYTES = 500 * 1024 * 1024;

/** A browser cannot reopen a file, so a saved upload is matched to a re-selected file by these. */
export interface FileFingerprint {
  name: string;
  size: number;
  lastModified: number;
}

export interface PersistedUpload {
  v: 1;
  jobId: string;
  uploadId: string;
  partSize: number;
  partCount: number;
  fingerprint: FileFingerprint;
  fileType: string;
  durationSeconds: number;
  language: string;
  /** ETag of each finished part, keyed by part number. */
  etags: Record<string, string>;
  createdAt: number;
}

export function fingerprintOf(file: File): FileFingerprint {
  return { name: file.name, size: file.size, lastModified: file.lastModified };
}

function isPersistedUpload(value: unknown): value is PersistedUpload {
  const s = value as PersistedUpload | null;
  return (
    !!s &&
    s.v === 1 &&
    typeof s.jobId === 'string' &&
    typeof s.uploadId === 'string' &&
    typeof s.partSize === 'number' &&
    typeof s.partCount === 'number' &&
    typeof s.createdAt === 'number' &&
    typeof s.etags === 'object' &&
    s.etags !== null &&
    typeof s.fingerprint?.name === 'string' &&
    typeof s.fingerprint?.size === 'number' &&
    typeof s.fingerprint?.lastModified === 'number'
  );
}

/** Bytes of the file already held by finished parts. The last part is usually shorter than the rest. */
export function storedBytes(state: PersistedUpload): number {
  let bytes = 0;
  for (const key of Object.keys(state.etags)) {
    bytes += Number(key) < state.partCount ? state.partSize : state.fingerprint.size - state.partSize * (state.partCount - 1);
  }
  return bytes;
}

export function storedPercent(state: PersistedUpload): number {
  return Math.min(99, Math.floor((storedBytes(state) / state.fingerprint.size) * 100));
}

export function saveUpload(state: PersistedUpload): void {
  try {
    localStorage.setItem(UPLOAD_KEY_PREFIX + state.jobId, JSON.stringify(state));
  } catch {
    // No storage: the upload still runs, it just cannot be resumed after a reload.
  }
}

export function removeUpload(jobId: string): void {
  try {
    localStorage.removeItem(UPLOAD_KEY_PREFIX + jobId);
  } catch {
    // Nothing to clean up.
  }
}

export function loadUpload(jobId: string): PersistedUpload | null {
  try {
    const raw = localStorage.getItem(UPLOAD_KEY_PREFIX + jobId);
    if (!raw) return null;
    const parsed: unknown = JSON.parse(raw);
    if (isPersistedUpload(parsed) && Date.now() - parsed.createdAt < RESUME_TTL_MS) return parsed;
    removeUpload(jobId);
  } catch {
    removeUpload(jobId);
  }
  return null;
}

/** Saved uploads that can still be resumed, newest first. Expired or damaged entries are dropped. */
export function listUploads(): PersistedUpload[] {
  const found: PersistedUpload[] = [];
  try {
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (key?.startsWith(UPLOAD_KEY_PREFIX)) {
        const state = loadUpload(key.slice(UPLOAD_KEY_PREFIX.length));
        if (state) found.push(state);
      }
    }
  } catch {
    return [];
  }
  return found.sort((a, b) => b.createdAt - a.createdAt);
}

export function findUploadForFile(file: File): PersistedUpload | null {
  const want = fingerprintOf(file);
  return (
    listUploads().find(
      (s) =>
        s.fingerprint.name === want.name &&
        s.fingerprint.size === want.size &&
        s.fingerprint.lastModified === want.lastModified,
    ) ?? null
  );
}

/** Bytes per second of the last upload that was measured, or null if none was. */
export function getMeasuredSpeed(): number | null {
  try {
    const value = Number(localStorage.getItem(SPEED_KEY));
    return Number.isFinite(value) && value > 0 ? value : null;
  } catch {
    return null;
  }
}

/** Keeps the speed of a finished upload for the next estimate. Too small to measure is skipped. */
export function recordUploadSpeed(bytes: number, elapsedMs: number): void {
  if (bytes < 4 * 1024 * 1024 || elapsedMs < 2000) return;
  try {
    localStorage.setItem(SPEED_KEY, String(Math.round((bytes / elapsedMs) * 1000)));
  } catch {
    // The next estimate falls back to the default speed.
  }
}

export function estimateSeconds(bytes: number, bytesPerSecond: number): number {
  return bytes / Math.max(1, bytesPerSecond);
}

/** "under a minute", "about 12 min", "about 1 h 20 min". */
export function formatEta(seconds: number): string {
  if (seconds < 45) return 'under a minute';
  const minutes = Math.max(1, Math.round(seconds / 60));
  if (minutes < 60) return `about ${minutes} min`;
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  return rest === 0 ? `about ${hours} h` : `about ${hours} h ${rest} min`;
}

export function formatSpeed(bytesPerSecond: number): string {
  return `${(bytesPerSecond / (1024 * 1024)).toFixed(1)} MB/s`;
}
