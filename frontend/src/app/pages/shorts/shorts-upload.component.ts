import { Component, NgZone, OnDestroy, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router, RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { catchError, of, switchMap } from 'rxjs';
import {
  CreateShortsResponse,
  DEFAULT_SHORTS_LANGUAGES,
  DEFAULT_SHORTS_LIMITS,
  LanguageOption,
  ShortsLimits,
  ShortsService,
  startPipeline,
} from './shorts.service';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { ToastsComponent } from '../../shared/components/toast/toasts.component';
import { ShortsEffectsPanelComponent } from './shorts-effects-panel.component';
import { ShortsUploader, UploadError, UploadProgress, multipartApi } from './shorts-uploader';
import { probeMediaDuration } from '../../shared/media-probe';
import {
  DEFAULT_UPLOAD_BYTES_PER_SEC,
  LARGE_UPLOAD_WARNING_BYTES,
  PersistedUpload,
  estimateSeconds,
  findUploadForFile,
  formatEta,
  formatSpeed,
  getMeasuredSpeed,
  listUploads,
  loadUpload,
  removeUpload,
  storedPercent,
} from './shorts-upload-state';

const ACCEPTED_EXTENSIONS = ['mp4', 'mov', 'mp3', 'wav'];

/**
 * Object URL for a locally chosen File, guaranteed to be a `blob:` URL the
 * browser minted for this document. Assigning it to a media element's `src`
 * cannot inject markup or script. The explicit check documents that and keeps
 * a stray non-blob value (never produced here) from ever reaching a `.src`.
 */
function blobObjectUrl(file: Blob): string {
  const url = URL.createObjectURL(file);
  if (!url.startsWith('blob:')) {
    URL.revokeObjectURL(url);
    throw new Error('Unexpected object URL scheme');
  }
  return url;
}

@Component({
  selector: 'app-shorts-upload',
  standalone: true,
  imports: [CommonModule, RouterLink, LucideDynamicIcon, NavbarComponent, ToastsComponent, ShortsEffectsPanelComponent],
  styles: [`
    :host { display: block; }
    .page { flex: 1; display: flex; flex-direction: column; }
    .head { display: flex; flex-wrap: wrap; align-items: flex-end; justify-content: space-between; gap: var(--space-sm); }
    h1 { margin: 0; font-size: var(--type-2xl); font-weight: 600; line-height: 1.15; letter-spacing: -0.01em; }
    .lead { margin: var(--space-2xs) 0 0; color: var(--color-ink-2); max-width: 60ch; }
    .reqs { margin: 0; padding: 0; list-style: none; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-xs); }
    @media (min-width: 48rem) { .reqs { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
    .req { display: flex; align-items: center; gap: var(--space-sm); padding: var(--space-sm) var(--space-md); border-radius: var(--radius-control); border: 1px solid var(--color-rule); background: var(--app-card); }
    .req b { display: block; font-size: var(--type-sm); font-weight: 600; }
    .req span.t { display: block; font-size: var(--type-xs); color: var(--color-muted); }
    .drop { position: relative; flex: 1; min-height: 22rem; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: var(--space-sm); padding: var(--space-xl) var(--space-md); text-align: center; border: 2px dashed color-mix(in oklab, var(--color-ink) 25%, transparent); border-radius: var(--radius-surface); background: var(--app-card); cursor: pointer; transition: border-color var(--dur-short) var(--ease-out), background-color var(--dur-short) var(--ease-out); }
    .drop::before { content: ''; position: absolute; inset: -20px; border-radius: 1rem; pointer-events: none; }
    .drop:hover, .drop.over { border-color: var(--color-accent); background: color-mix(in oklab, var(--color-accent) 5%, var(--app-card)); }
    .drop.over { border-style: solid; }
    .drop.ok { border-style: solid; border-color: var(--color-positive); }
    .drop:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 3px; }
    .drop h2 { margin: 0; font-size: var(--type-xl); font-weight: 600; }
    .big-icon { width: 4.5rem; height: 4.5rem; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; background: color-mix(in oklab, var(--color-accent) 12%, var(--app-card)); color: var(--color-accent-text); }
    .chips { display: flex; flex-wrap: wrap; justify-content: center; gap: var(--space-2xs); margin: 0; padding: 0; list-style: none; }
    .chips li { padding: var(--space-3xs) var(--space-sm); border-radius: var(--radius-pill); background: var(--color-paper-3); font-size: var(--type-xs); font-weight: 500; color: var(--color-ink-2); }
    .release { position: absolute; inset: var(--space-md); pointer-events: none; display: flex; align-items: center; justify-content: center; gap: var(--space-md); border-radius: var(--radius-control); background: color-mix(in oklab, var(--color-accent) 10%, var(--app-card)); color: var(--color-ink); }
    .release video { height: 6rem; border-radius: var(--radius-control); opacity: 0.8; }
    .release p { margin: 0; text-align: left; }
    .file { width: 100%; max-width: 30rem; display: flex; align-items: center; gap: var(--space-md); padding: var(--space-md); border-radius: var(--radius-control); border: 1px solid color-mix(in oklab, var(--color-positive) 45%, transparent); background: color-mix(in oklab, var(--color-positive) 7%, var(--app-card)); text-align: left; }
    .file video, .file .ph { width: 3.5rem; height: 5rem; flex: none; border-radius: var(--radius-control); object-fit: cover; background: var(--color-ink); }
    .file .ph { display: inline-flex; align-items: center; justify-content: center; background: var(--color-paper-3); color: var(--color-positive); }
    .file .main { flex: 1; min-width: 0; }
    .file .name { margin: 0; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .meta { margin: 0; display: flex; flex-wrap: wrap; gap: 0 var(--space-sm); font-size: var(--type-xs); color: var(--color-muted); }
    .tick { width: 2rem; height: 2rem; flex: none; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; background: var(--color-positive); color: var(--color-paper); }
    .note { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-sm); font-size: var(--type-sm); }
    .note p { margin: 0; }
    .note .row { display: flex; align-items: flex-start; gap: var(--space-xs); margin-top: var(--space-xs); }
    .note .row svg { flex: none; margin-top: 2px; color: var(--color-caution); }
    .err { display: flex; flex-direction: column; align-items: flex-start; gap: var(--space-sm); border-left: 4px solid var(--color-negative); }
    .err p { margin: 0; display: flex; align-items: center; gap: var(--space-xs); color: var(--color-negative); font-weight: 500; font-size: var(--type-sm); }
    .prog-head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-sm); font-size: var(--type-sm); }
    .prog-head .pct { font-weight: 600; font-variant-numeric: tabular-nums; }
    .track { margin-top: var(--space-sm); height: 10px; border-radius: var(--radius-pill); background: var(--color-paper-3); overflow: hidden; }
    .track div { height: 100%; border-radius: inherit; background: var(--color-accent); transition: width var(--dur-long) var(--ease-out); }
    .small { margin: var(--space-xs) 0 0; font-size: var(--type-xs); color: var(--color-muted); font-variant-numeric: tabular-nums; }
    .go { display: flex; flex-direction: column; gap: var(--space-md); }
    @media (min-width: 40rem) { .go { flex-direction: row; align-items: flex-end; justify-content: space-between; } }
    .field { display: flex; flex-direction: column; gap: var(--space-2xs); max-width: 36rem; }
    .field .ui-field { max-width: 18rem; }
    .go .ui-btn { min-height: 52px; padding-inline: var(--space-lg); font-size: var(--type-md); white-space: nowrap; flex: none; }
    .spin { animation: spin 0.9s linear infinite; }
    @keyframes spin { to { transform: rotate(360deg); } }
    @media (prefers-reduced-motion: reduce) { .spin { animation: none; } .drop, .track div { transition: none; } }
  `],
  template: `
    <div class="ui-app" (dragover)="onDragOver($event)" (dragleave)="onDragLeave($event)" (drop)="onDrop($event)">
      <app-navbar></app-navbar>
      <app-toasts></app-toasts>
      <main id="main" class="ui-app-main hm page">
        <div class="head">
          <div>
            <h1>Create AI Shorts</h1>
            <p class="lead">Drop your footage anywhere. We find the best moments and cut vertical 9:16 clips.</p>
          </div>
          <a routerLink="/shorts/history" class="ui-btn ui-btn--secondary">My shorts</a>
        </div>

        <ul class="reqs" aria-label="Upload requirements">
          <li class="req"><span class="ui-chip" aria-hidden="true"><svg lucideIcon="video" [size]="18"></svg></span><span><b>MP4 or MOV</b><span class="t">video sources</span></span></li>
          <li class="req"><span class="ui-chip" aria-hidden="true"><svg lucideIcon="mic" [size]="18"></svg></span><span><b>MP3 or WAV</b><span class="t">audio sources</span></span></li>
          <li class="req"><span class="ui-chip" aria-hidden="true"><svg lucideIcon="clock" [size]="18"></svg></span><span><b>Max {{ limits.maxDurationMinutes }} min</b><span class="t">per upload</span></span></li>
          <li class="req"><span class="ui-chip" aria-hidden="true"><svg lucideIcon="scissors" [size]="18"></svg></span><span><b>Up to 3 clips</b><span class="t">1080×1920, captioned</span></span></li>
        </ul>

        @if (pending && !selectedFile) {
          <div class="ui-card note" role="status">
            <p>
              Unfinished upload: <strong>{{ pending.fingerprint.name }}</strong>
              ({{ formatFileSize(pending.fingerprint.size) }}). Choose the same file again to continue where it stopped.
            </p>
            <button type="button" class="ui-btn ui-btn--secondary" (click)="discardPending()">Discard</button>
          </div>
        }

        <div
          class="drop"
          [class.over]="isDragOver"
          [class.ok]="selectedFile && !error"
          (click)="fileInput.click()"
          (keydown.enter)="fileInput.click()"
          tabindex="0"
          role="button"
          data-testid="dropzone"
          aria-label="Drop zone for media file upload. Accepts MP4, MOV, MP3, WAV up to {{ limits.maxDurationMinutes }} minutes and {{ limits.maxUploadMb }} MB."
        >
          <input #fileInput type="file" accept=".mp4,.mov,.mp3,.wav" class="hidden" (change)="onFileSelected($event)" aria-hidden="true" />

          @if (!selectedFile) {
            <span class="big-icon" aria-hidden="true"><svg lucideIcon="upload" [size]="32"></svg></span>
            <h2>{{ isDragOver ? 'Drop it. We take it from here' : 'Drag & drop anywhere on this page' }}</h2>
            <ul class="chips">
              <li>MP4, MOV, MP3, WAV</li>
              <li>Max {{ limits.maxDurationMinutes }} min</li>
              <li>Max {{ limits.maxUploadMb }} MB</li>
            </ul>
            <button type="button" class="ui-btn ui-btn--primary" (click)="fileInput.click(); $event.stopPropagation()">Choose a file</button>
          }

          @if (isDragOver && !selectedFile) {
            <div class="release">
              @if (dragPreviewUrl) {
                <video [src]="dragPreviewUrl" muted playsinline></video>
              } @else {
                <svg lucideIcon="file-text" [size]="36" aria-hidden="true"></svg>
              }
              <div>
                <p><strong>Release to upload</strong></p>
                @if (dragFileName) {
                  <p class="meta">{{ dragFileName }}</p>
                }
              </div>
            </div>
          }

          @if (isProbing) {
            <span class="big-icon" aria-hidden="true"><svg lucideIcon="loader-2" [size]="30" class="spin"></svg></span>
            <h2>Checking file</h2>
          }

          @if (selectedFile && !error && !isProbing) {
            <div class="file">
              @if (previewUrl && !isAudioFile) {
                <video [src]="previewUrl" muted playsinline preload="metadata"></video>
              } @else {
                <span class="ph" aria-hidden="true"><svg lucideIcon="file-text" [size]="24"></svg></span>
              }
              <div class="main">
                <p class="name">{{ selectedFile.name }}</p>
                <p class="meta"><span>{{ formatFileSize(selectedFile.size) }}</span><span>{{ formatDuration(durationSeconds) }}</span></p>
                <button type="button" class="ui-btn ui-btn--quiet" style="padding-inline: 0" (click)="replaceFile($event); fileInput.click()">Replace</button>
              </div>
              <span class="tick" aria-hidden="true"><svg lucideIcon="check" [size]="18"></svg></span>
            </div>
          }
        </div>

        @if (error) {
          <div class="ui-card err" role="alert">
            <p><svg lucideIcon="circle-x" [size]="16" aria-hidden="true"></svg>{{ error }}</p>
          </div>
        }

        @if (selectedFile && !error && !isProbing && !isUploading && !landedJobId) {
          <div class="ui-card note" role="status" style="display: block">
            <p>
              <strong>{{ formatFileSize(selectedFile.size) }}</strong> to upload.
              Estimated time: <strong>{{ estimateLabel }}</strong>{{ ' ' }}
              <span style="color: var(--color-muted)">({{ estimateBasis }}).</span>
            </p>
            @if (resumable) {
              <p class="row">
                <span>
                  An earlier upload of this file reached {{ resumablePercent }}%. Resume upload continues from there.
                  <button type="button" class="ui-btn ui-btn--quiet" (click)="startOver()">Start over</button>
                </span>
              </p>
            }
            @if (isLargeUpload) {
              <p class="row">
                <svg lucideIcon="clock" [size]="16" aria-hidden="true"></svg>
                <span>
                  Large file. The upload keeps running in the background while this tab stays open, and it
                  continues from where it stopped if your connection drops. If you close the tab, come back
                  to this page and choose the same file to continue.
                </span>
              </p>
            }
          </div>
        }

        @if (uploadProgress >= 0 && !landedJobId) {
          <section class="ui-card" aria-label="Upload">
            <div class="prog-head">
              <span>{{ uploadLabel }}</span>
              <span style="display: flex; align-items: center; gap: var(--space-sm)">
                <span class="pct">{{ uploadProgress }}%</span>
                <button type="button" class="ui-btn ui-btn--secondary" (click)="cancelUpload()">Cancel</button>
              </span>
            </div>
            <div class="track" role="progressbar" aria-label="Upload progress" aria-valuemin="0" aria-valuemax="100" [attr.aria-valuenow]="uploadProgress">
              <div [style.width.%]="uploadProgress"></div>
            </div>
            @if (progress) {
              <p class="small meta">
                @for (bit of progressBits; track $index) {
                  <span>{{ bit }}</span>
                }
              </p>
              @if (progressNote) {
                <p class="small" style="color: var(--color-ink)" aria-live="polite">{{ progressNote }}</p>
              }
            }
            @if (isLargeUpload) {
              <p class="small">Keep this tab open. The upload keeps running while you use other tabs.</p>
            }
          </section>
        }

        @if (landedJobId) {
          <section aria-label="Effects before processing">
            <h2 class="ui-card-title">Upload complete</h2>
            <p class="ui-card-sub">Choose video effects, then start processing. Effects are off by default, so you can start right away.</p>
            <app-shorts-effects-panel style="display: block; margin-top: var(--space-md)" [jobId]="landedJobId" [startStep]="true" (effectsSaved)="onEffectsSaved()"></app-shorts-effects-panel>
          </section>
        }

        @if (!landedJobId) {
          <div class="go">
            <div class="field">
              <label class="ui-label" for="shorts-lang">Transcript language</label>
              <span class="ui-hint" id="shorts-lang-hint">Auto-detect works for most files. Pick a language if the spoken audio is Hindi or English and detection guesses wrong.</span>
              <select id="shorts-lang" class="ui-field" [value]="language" (change)="onLanguageChange($event)" [disabled]="isUploading" aria-describedby="shorts-lang-hint">
                @for (lang of languages; track lang.code) {
                  <option [value]="lang.code">{{ lang.label }}</option>
                }
              </select>
            </div>
            <button type="button" class="ui-btn ui-btn--primary" data-testid="generate" [disabled]="!selectedFile || !!error || isUploading || isProbing" (click)="startProcessing()">
              @if (isUploading) {
                <svg lucideIcon="loader-2" [size]="18" class="spin" aria-hidden="true"></svg>
                Uploading
              } @else {
                <svg lucideIcon="play" [size]="18" aria-hidden="true"></svg>
                {{ resumable ? 'Resume upload' : 'Generate Shorts' }}
              }
            </button>
          </div>
        }

        @if (uploadError) {
          <div class="ui-card err" role="alert">
            <p><svg lucideIcon="circle-x" [size]="16" aria-hidden="true"></svg>{{ uploadError }}</p>
            @if (!landedJobId) {
              <button type="button" class="ui-btn ui-btn--secondary" (click)="retryUpload()" [disabled]="isUploading || isProbing">
                {{ resumable ? 'Resume upload' : 'Try again' }}
              </button>
            }
          </div>
        }
      </main>
    </div>
  `,
})
export class ShortsUploadComponent implements OnInit, OnDestroy {
  selectedFile: File | null = null;
  fileType = '';
  durationSeconds = 0;
  isDragOver = false;
  isProbing = false;
  error = '';
  uploadError = '';
  isUploading = false;
  uploadProgress = -1;
  previewUrl: string | null = null;
  dragFileName = '';
  dragPreviewUrl: string | null = null;
  /** The limits the pipeline enforces, read from GET /shorts/limits. */
  limits: ShortsLimits = DEFAULT_SHORTS_LIMITS;
  /** Selected transcription language; auto-detect unless the user overrides. */
  language = 'auto';

  /** Live detail from the uploader while a transfer runs. */
  progress: UploadProgress | null = null;
  /** A saved upload that matches the selected file, so the upload resumes instead of restarting. */
  resumable: PersistedUpload | null = null;
  /** The newest saved upload, offered until its file is chosen again or it is discarded. */
  pending: PersistedUpload | null = null;
  /** Bytes per second of the last measured upload; null until one has finished. */
  private measuredSpeed: number | null = getMeasuredSpeed();

  private uploader: ShortsUploader | null = null;
  private lastJobId: string | null = null;
  /** A job whose file is fully uploaded and that waits in the effects step for the user to start it. */
  landedJobId: string | null = null;

  private readonly preventWindowDrop = (event: DragEvent): void => {
    // Drops outside the zone must never navigate the browser away.
    event.preventDefault();
  };

  constructor(
    private readonly router: Router,
    private readonly shorts: ShortsService,
    private readonly zone: NgZone,
  ) {}

  ngOnInit(): void {
    window.addEventListener('dragover', this.preventWindowDrop);
    window.addEventListener('drop', this.preventWindowDrop);
    this.loadLimits();
    this.pending = listUploads()[0] ?? null;
  }

  /** Read the enforced limits from the API. The local defaults stay in place
   * if the request fails, so the page still validates against real numbers. */
  private loadLimits(): void {
    this.shorts.getLimits().subscribe({
      next: (limits) => {
        if (limits && limits.maxDurationSeconds > 0 && limits.maxUploadBytes > 0) {
          this.limits = {
            ...DEFAULT_SHORTS_LIMITS,
            ...limits,
            languages: limits.languages?.length ? limits.languages : DEFAULT_SHORTS_LANGUAGES,
          };
        }
      },
      error: () => {},
    });
  }

  /** Language choices from the API, with local defaults as a fallback. */
  get languages(): LanguageOption[] {
    return this.limits.languages?.length ? this.limits.languages : DEFAULT_SHORTS_LANGUAGES;
  }

  onLanguageChange(event: Event): void {
    this.language = (event.target as HTMLSelectElement).value;
  }

  ngOnDestroy(): void {
    window.removeEventListener('dragover', this.preventWindowDrop);
    window.removeEventListener('drop', this.preventWindowDrop);
    // Leaving the page stops the transfer but keeps the saved parts, so the file can be chosen again to resume.
    this.uploader?.pause();
    this.uploader = null;
  }

  get isLargeUpload(): boolean {
    return !!this.selectedFile && this.selectedFile.size >= LARGE_UPLOAD_WARNING_BYTES;
  }

  private get uploadSpeed(): number {
    return this.measuredSpeed ?? DEFAULT_UPLOAD_BYTES_PER_SEC;
  }

  /** Time to upload the selected file at the last measured speed, or at the default before one exists. */
  get estimateLabel(): string {
    return this.selectedFile ? formatEta(estimateSeconds(this.selectedFile.size, this.uploadSpeed)) : '';
  }

  get estimateBasis(): string {
    return this.measuredSpeed
      ? `based on your last upload, ${formatSpeed(this.measuredSpeed)}`
      : `assuming a typical ${formatSpeed(DEFAULT_UPLOAD_BYTES_PER_SEC)} connection`;
  }

  get resumablePercent(): number {
    return this.resumable ? storedPercent(this.resumable) : 0;
  }

  get uploadLabel(): string {
    switch (this.progress?.status) {
      case 'finishing':
        return 'Finishing upload…';
      case 'retrying':
        return 'Retrying…';
      case 'offline':
        return 'Waiting for your connection…';
      default:
        return 'Uploading…';
    }
  }

  /** "384.00 MB of 2.00 GB · 3.2 MB/s · about 8 min left", then the part counts. */
  get progressDetail(): string {
    const p = this.progress;
    if (!p) return '';
    const bits = [`${this.formatFileSize(p.bytesDone)} of ${this.formatFileSize(p.bytesTotal)}`];
    if (p.bytesPerSecond > 0) {
      bits.push(formatSpeed(p.bytesPerSecond));
      bits.push(`${formatEta((p.bytesTotal - p.bytesDone) / p.bytesPerSecond)} left`);
    }
    if (p.partsTotal > 1) bits.push(`${p.partsDone} of ${p.partsTotal} parts uploaded`);
    return bits.join(' · ');
  }

  /** The same facts as progressDetail, one per item, so the page can show them without a joining dot. */
  get progressBits(): string[] {
    return this.progressDetail ? this.progressDetail.split(' · ') : [];
  }

  /** What went wrong right now, in words, for the retry and offline states. */
  get progressNote(): string {
    const p = this.progress;
    if (p?.status === 'offline') return 'You are offline. The upload continues when your connection is back.';
    if (p?.status === 'retrying') {
      return p.retry
        ? `Connection problem on part ${p.retry.part}. Trying again (attempt ${p.retry.attempt} of ${p.retry.of}).`
        : 'Connection problem. Picking the upload up again.';
    }
    return '';
  }

  get isAudioFile(): boolean {
    return this.fileType === 'mp3' || this.fileType === 'wav';
  }

  onDragOver(event: DragEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.isDragOver = true;
    // Ghost preview: file metadata is readable mid-drag; thumbnail is
    // best-effort (some browsers withhold file handles until drop).
    try {
      const file = event.dataTransfer?.files?.[0];
      if (file && !this.dragFileName) {
        this.dragFileName = file.name;
        if (file.type.startsWith('video/')) {
          this.clearDragPreview();
          this.dragPreviewUrl = blobObjectUrl(file);
        }
      }
    } catch {
      // Ignore: ghost stays generic.
    }
  }

  onDragLeave(event: DragEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.isDragOver = false;
    this.clearDragPreview();
  }

  onDrop(event: DragEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.isDragOver = false;
    this.clearDragPreview();
    const files = event.dataTransfer?.files;
    if (files && files.length > 0) {
      this.handleFile(files[0]);
    }
  }

  private clearDragPreview(): void {
    this.dragFileName = '';
    if (this.dragPreviewUrl) {
      URL.revokeObjectURL(this.dragPreviewUrl);
      this.dragPreviewUrl = null;
    }
  }

  onFileSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    if (input.files && input.files.length > 0) {
      this.handleFile(input.files[0]);
    }
  }

  removeFile(event: Event): void {
    event.stopPropagation();
    this.reset();
  }

  replaceFile(event: Event): void {
    event.stopPropagation();
    this.reset();
  }

  formatFileSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  }

  formatDuration(seconds: number): string {
    const total = Math.round(seconds);
    const mins = Math.floor(total / 60);
    const secs = total % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  }

  startProcessing(): void {
    if (!this.selectedFile || this.error || this.isUploading || this.isProbing || this.landedJobId) return;

    this.isUploading = true;
    this.uploadError = '';
    this.uploadProgress = 0;
    this.progress = null;

    const saved = this.resumable;
    if (saved) {
      this.lastJobId = saved.jobId;
      this.transfer(saved.jobId, undefined, saved);
      return;
    }
    this.shorts.createJob(this.fileType, this.durationSeconds, this.language, this.selectedFile.size).subscribe({
      next: (res: CreateShortsResponse) => {
        this.lastJobId = res.jobId;
        this.transfer(res.jobId, res.uploadUrl, null);
      },
      error: (err) => this.failUpload(err, 'Failed to initiate upload. Please try again.'),
    });
  }

  /** Sends the file: one PUT for small files, parallel resumable parts for large ones. */
  private transfer(jobId: string, uploadUrl: string | undefined, saved: PersistedUpload | null): void {
    const uploader = new ShortsUploader(
      multipartApi(this.shorts),
      this.selectedFile!,
      {
        jobId,
        uploadUrl,
        fileType: this.fileType,
        durationSeconds: this.durationSeconds,
        language: this.language,
      },
      { onProgress: (p) => this.zone.run(() => this.onUploadProgress(p)) },
    );
    this.uploader = uploader;
    // Progress events fire many times a second; only the throttled callback above re-enters Angular.
    this.zone.runOutsideAngular(() => {
      uploader.run(saved).then(
        () => this.zone.run(() => this.onUploadDone(jobId)),
        (err) => this.zone.run(() => this.onUploadFailed(uploader, err, jobId)),
      );
    });
  }

  private onUploadProgress(p: UploadProgress): void {
    this.progress = p;
    this.uploadProgress = p.bytesTotal > 0 ? Math.round((p.bytesDone / p.bytesTotal) * 100) : 0;
    this.measuredSpeed = getMeasuredSpeed();
  }

  private onUploadDone(jobId: string): void {
    this.uploader = null;
    this.resumable = null;
    this.uploadProgress = 100;
    this.measuredSpeed = getMeasuredSpeed();
    // The job stays `uploaded` until the user saves the effects and starts it.
    this.isUploading = false;
    this.landedJobId = jobId;
  }

  private onUploadFailed(uploader: ShortsUploader, err: unknown, jobId: string): void {
    if (this.uploader !== uploader) return; // cancelled or replaced: nothing to report
    this.uploader = null;
    this.isUploading = false;
    const failure = err instanceof UploadError ? err : null;
    this.uploadProgress = -1;
    this.progress = null;
    switch (failure?.kind) {
      case 'aborted':
        return;
      case 'already-started':
        this.router.navigate(['/shorts', jobId, 'progress']);
        return;
      case 'session-lost':
        // The saved upload is gone: the next attempt starts a new one.
        this.resumable = null;
        this.discardJob(jobId);
        this.lastJobId = null;
        this.uploadError = 'The saved upload expired, so it has to start again from the beginning.';
        return;
      case 'rejected':
        this.resumable = null;
        this.discardJob(jobId);
        this.lastJobId = null;
        this.uploadError = failure.message;
        return;
      default: {
        // Network trouble or a sign-in that ended: saved parts stay, so the upload can continue.
        this.resumable = loadUpload(jobId);
        if (failure?.kind === 'auth') this.uploadError = failure.message;
        else if (this.resumable) {
          this.uploadError = `Upload paused at ${storedPercent(this.resumable)}%. The parts already sent are saved. Check your connection, then resume.`;
        } else {
          this.uploadError = failure?.message || 'Upload failed. Please try again.';
        }
      }
    }
  }

  /** Cancel the in-flight upload and clean up its orphan job row. */
  cancelUpload(): void {
    this.uploader?.pause();
    this.uploader = null;
    const jobId = this.lastJobId;
    this.lastJobId = null;
    this.resumable = null;
    this.isUploading = false;
    this.uploadProgress = -1;
    this.progress = null;
    this.uploadError = '';
    // Best effort: don't strand an `uploaded` job that will never start.
    if (jobId) this.discardJob(jobId);
  }

  /** Explicit retry: continue a saved upload, or drop the previous attempt's job row and start over. */
  retryUpload(): void {
    if (this.isUploading || this.isProbing || !this.selectedFile || this.error) return;
    if (!this.resumable && !this.landedJobId) {
      const stale = this.lastJobId;
      this.lastJobId = null;
      if (stale) this.discardJob(stale);
    }
    this.startProcessing();
  }

  /** Throws away a saved upload and its job so the file goes up from the start. */
  startOver(): void {
    const saved = this.resumable;
    this.resumable = null;
    if (saved) this.discardJob(saved.jobId);
    if (this.pending?.jobId === saved?.jobId) this.pending = null;
    this.lastJobId = null;
    this.uploadError = '';
  }

  discardPending(): void {
    const pending = this.pending;
    this.pending = null;
    if (pending) this.discardJob(pending.jobId);
  }

  /** Aborts the open S3 upload (if any), forgets the saved parts and deletes the job row. Errors are ignored. */
  private discardJob(jobId: string): void {
    const saved = loadUpload(jobId);
    removeUpload(jobId);
    const abort$ = saved ? this.shorts.abortMultipart(jobId, saved.uploadId) : of(undefined);
    abort$
      .pipe(
        catchError(() => of(undefined)),
        switchMap(() => this.shorts.deleteJob(jobId)),
      )
      .subscribe({ error: () => {} });
  }

  private handleFile(file: File): void {
    this.reset();

    const ext = file.name.split('.').pop()?.toLowerCase() ?? '';
    if (!ACCEPTED_EXTENSIONS.includes(ext)) {
      this.error = 'Unsupported file type. Please select an MP4, MOV, MP3, or WAV file.';
      this.selectedFile = file;
      return;
    }

    if (file.size === 0) {
      this.error = 'File is empty. Please select a file with content.';
      this.selectedFile = file;
      return;
    }

    if (file.size > this.limits.maxUploadBytes) {
      this.error = `File is too large (${this.formatFileSize(file.size)}). Maximum size is ${this.limits.maxUploadMb} MB.`;
      this.selectedFile = file;
      return;
    }

    this.selectedFile = file;
    this.fileType = ext;
    this.resumable = findUploadForFile(file);
    if (this.resumable) {
      // The job already carries these choices, so the picker follows it.
      this.language = this.resumable.language;
      if (this.pending?.jobId === this.resumable.jobId) this.pending = null;
    }
    if (ext === 'mp4' || ext === 'mov') {
      this.previewUrl = blobObjectUrl(file);
    }
    this.probeDuration(file, ext);
  }

  /** Read the media duration without uploading anything (shared off-screen probe). */
  private probeDuration(file: File, ext: string): void {
    this.isProbing = true;
    const isAudio = ext === 'mp3' || ext === 'wav';
    probeMediaDuration(file, isAudio ? 'audio' : 'video').then(
      (duration) => {
        this.isProbing = false;
        if (!isFinite(duration) || duration <= 0) {
          this.error = 'Could not read media duration. Please try a different file.';
          return;
        }
        if (duration > this.limits.maxDurationSeconds) {
          this.error = `File is too long (${this.formatDuration(duration)}). Maximum length is ${this.limits.maxDurationMinutes} minutes.`;
          return;
        }
        this.durationSeconds = duration;
      },
      () => {
        this.isProbing = false;
        this.error = 'Could not read this media file. Please try a different file.';
      },
    );
  }

  /** The effects panel has saved the settings, so the job can start. */
  onEffectsSaved(): void {
    if (!this.landedJobId || this.isUploading) return;
    const jobId = this.landedJobId;
    this.isUploading = true;
    this.uploadError = '';
    startPipeline(this.shorts, this.router, jobId).subscribe({
      next: () => {
        this.isUploading = false;
        this.landedJobId = null;
      },
      error: (err) => {
        // Only a failure that says nothing about the file (network, server) keeps the upload for a quick retry.
        this.landedJobId = !err?.status || err.status >= 500 ? jobId : null;
        this.failUpload(err, 'Failed to start processing. Please try again.');
      },
    });
  }

  private failUpload(err: { error?: { message?: string } }, fallback: string): void {
    this.isUploading = false;
    this.uploadProgress = -1;
    this.uploadError = err?.error?.message || fallback;
  }

  private reset(): void {
    this.uploader?.pause();
    this.uploader = null;
    this.resumable = null;
    this.landedJobId = null;
    this.progress = null;
    if (this.lastJobId) {
      // Replacing the file strands the previous attempt's job row.
      const stale = this.lastJobId;
      this.lastJobId = null;
      this.discardJob(stale);
    }
    if (this.previewUrl) {
      URL.revokeObjectURL(this.previewUrl);
      this.previewUrl = null;
    }
    this.clearDragPreview();
    this.selectedFile = null;
    this.fileType = '';
    this.durationSeconds = 0;
    this.error = '';
    this.uploadError = '';
    this.isUploading = false;
    this.uploadProgress = -1;
    this.isProbing = false;
  }
}
