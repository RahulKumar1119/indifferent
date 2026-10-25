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

@Component({
  selector: 'app-shorts-upload',
  standalone: true,
  imports: [CommonModule, RouterLink, LucideDynamicIcon, NavbarComponent, ToastsComponent, ShortsEffectsPanelComponent],
  styles: [`
    .dropzone {
      border-width: 2px;
      border-style: dashed;
      border-color: rgb(75 85 99); /* gray-600 */
    }
    /* Magnetism: the active drop area extends 20px beyond the visible border. */
    .dropzone::before {
      content: '';
      position: absolute;
      inset: -20px;
      border-radius: 1rem;
      pointer-events: none;
    }
    .dropzone-active {
      border-style: solid;
      border-color: rgb(99 102 241); /* indigo-500 */
    }
  `],
  template: `
    <app-navbar></app-navbar>
    <app-toasts></app-toasts>
    <div
      class="min-h-[100dvh] flex flex-col px-4 py-8 max-w-5xl mx-auto w-full"
      (dragover)="onDragOver($event)"
      (dragleave)="onDragLeave($event)"
      (drop)="onDrop($event)"
    >
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-2xl font-bold">Create AI Shorts</h1>
          <p class="text-[hsl(var(--muted-foreground))]">
            Drop your footage anywhere. We find the best moments and cut vertical 9:16 clips.
          </p>
        </div>
        <a
          routerLink="/shorts/history"
          class="shrink-0 px-4 py-2 rounded-lg border border-[hsl(var(--border))] hover:bg-white/5 transition-colors text-sm font-medium"
        >
          My shorts
        </a>
      </div>

      <!-- Constraints, stated upfront -->
      <div class="mt-5 grid grid-cols-2 md:grid-cols-4 gap-3" aria-label="Upload requirements">
        <div class="glass-card p-3 text-center">
          <svg lucideIcon="video" [size]="20" class="mx-auto text-[hsl(var(--primary))]"></svg>
          <p class="mt-1.5 text-xs font-semibold">MP4 · MOV</p>
          <p class="text-[11px] text-[hsl(var(--muted-foreground))]">video sources</p>
        </div>
        <div class="glass-card p-3 text-center">
          <svg lucideIcon="mic" [size]="20" class="mx-auto text-[hsl(var(--primary))]"></svg>
          <p class="mt-1.5 text-xs font-semibold">MP3 · WAV</p>
          <p class="text-[11px] text-[hsl(var(--muted-foreground))]">audio sources</p>
        </div>
        <div class="glass-card p-3 text-center">
          <svg lucideIcon="clock" [size]="20" class="mx-auto text-[hsl(var(--primary))]"></svg>
          <p class="mt-1.5 text-xs font-semibold">Max {{ limits.maxDurationMinutes }} min</p>
          <p class="text-[11px] text-[hsl(var(--muted-foreground))]">per upload</p>
        </div>
        <div class="glass-card p-3 text-center">
          <svg lucideIcon="scissors" [size]="20" class="mx-auto text-[hsl(var(--primary))]"></svg>
          <p class="mt-1.5 text-xs font-semibold">Up to 3 clips</p>
          <p class="text-[11px] text-[hsl(var(--muted-foreground))]">1080×1920, captioned</p>
        </div>
      </div>

      <!-- A saved upload from an earlier visit: the browser cannot reopen the file by itself -->
      @if (pending && !selectedFile) {
        <div class="mt-4 glass-card p-3 flex flex-wrap items-center justify-between gap-3" role="status">
          <p class="text-sm">
            Unfinished upload: <span class="font-medium">{{ pending.fingerprint.name }}</span>
            ({{ formatFileSize(pending.fingerprint.size) }}). Choose the same file again to continue where it stopped.
          </p>
          <button
            type="button"
            (click)="discardPending()"
            class="rounded-full border border-[hsl(var(--border))] px-3 py-1 text-[12.5px] font-medium hover:bg-white/5"
          >
            Discard
          </button>
        </div>
      }

      <!-- Full-viewport drop zone -->
      <div
        class="dropzone relative flex-1 mt-4 glass-card p-10 text-center transition-all cursor-pointer flex flex-col items-center justify-center min-h-[400px]"
        [class.dropzone-active]="isDragOver"
        [class.!border-green-500]="selectedFile && !error"
        (click)="fileInput.click()"
        (keydown.enter)="fileInput.click()"
        tabindex="0"
        role="button"
        aria-label="Drop zone for media file upload. Accepts MP4, MOV, MP3, WAV up to {{ limits.maxDurationMinutes }} minutes and {{ limits.maxUploadMb }} MB."
      >
        <input
          #fileInput
          type="file"
          accept=".mp4,.mov,.mp3,.wav"
          class="hidden"
          (change)="onFileSelected($event)"
          aria-hidden="true"
        />

        @if (!selectedFile) {
          <div class="space-y-3">
            <div class="w-20 h-20 rounded-full bg-[hsl(var(--primary))]/10 flex items-center justify-center mx-auto">
              <svg lucideIcon="upload" [size]="36" class="text-[hsl(var(--primary))]" [class.animate-pulse]="isDragOver"></svg>
            </div>
            <p class="text-xl font-medium">{{ isDragOver ? 'Drop it. We take it from here' : 'Drag & drop anywhere on this page' }}</p>
            <p class="text-sm font-medium text-[hsl(var(--foreground))]">MP4, MOV, MP3, WAV · Max {{ limits.maxDurationMinutes }} min · Max {{ limits.maxUploadMb }} MB</p>
            <p class="text-sm text-[hsl(var(--muted-foreground))]">or</p>
            <button
              class="px-6 py-3 rounded-lg bg-[hsl(var(--primary))] text-white hover:opacity-90 transition-opacity text-sm font-semibold min-h-[48px] min-w-[200px]"
              (click)="fileInput.click(); $event.stopPropagation()"
            >
              or click to browse
            </button>
          </div>
        }

        @if (isDragOver && !selectedFile) {
          <div class="pointer-events-none absolute inset-4 rounded-xl border-2 border-dashed border-indigo-400/60 bg-indigo-500/10 backdrop-blur-[1px] flex items-center justify-center gap-4 p-6">
            @if (dragPreviewUrl) {
              <video [src]="dragPreviewUrl" muted playsinline class="h-24 rounded-lg opacity-70"></video>
            } @else {
              <svg lucideIcon="file-text" [size]="40" class="text-indigo-300 opacity-70"></svg>
            }
            <div class="text-left">
              <p class="text-lg font-medium text-indigo-200">Release to upload</p>
              @if (dragFileName) {
                <p class="text-sm text-indigo-200/70 truncate max-w-[240px]">{{ dragFileName }}</p>
              }
            </div>
          </div>
        }

        @if (isProbing) {
          <div class="space-y-3">
            <div class="w-16 h-16 rounded-full bg-[hsl(var(--primary))]/10 flex items-center justify-center mx-auto">
              <svg lucideIcon="loader-2" [size]="32" class="text-[hsl(var(--primary))] animate-spin"></svg>
            </div>
            <p class="text-lg font-medium">Checking file…</p>
          </div>
        }

        @if (selectedFile && !error && !isProbing) {
          <div class="mx-auto w-full max-w-md rounded-xl border border-green-500/40 bg-green-500/5 p-4 flex items-center gap-4 text-left">
            @if (previewUrl && !isAudioFile) {
              <video [src]="previewUrl" muted playsinline preload="metadata" class="h-20 w-14 shrink-0 rounded-lg object-cover bg-black"></video>
            } @else {
              <div class="h-20 w-14 shrink-0 rounded-lg bg-green-500/10 flex items-center justify-center">
                <svg lucideIcon="file-text" [size]="28" class="text-green-400"></svg>
              </div>
            }
            <div class="flex-1 min-w-0">
              <p class="font-medium truncate">{{ selectedFile.name }}</p>
              <p class="text-sm text-[hsl(var(--muted-foreground))]">
                {{ formatFileSize(selectedFile.size) }} · {{ formatDuration(durationSeconds) }}
              </p>
              <button
                class="mt-1 text-sm font-medium text-[hsl(var(--primary))] underline underline-offset-4"
                (click)="replaceFile($event); fileInput.click()"
              >
                Replace
              </button>
            </div>
            <span class="shrink-0 w-8 h-8 rounded-full bg-green-500 flex items-center justify-center" aria-hidden="true">
              <svg lucideIcon="check" [size]="18" class="text-white"></svg>
            </span>
          </div>
        }
      </div>

      <!-- Validation Error -->
      @if (error) {
        <div class="mt-4 p-3 glass-card !border-red-500/30">
          <p class="text-sm text-red-400 flex items-center gap-2">
            <svg lucideIcon="circle-x" [size]="16"></svg>
            {{ error }}
          </p>
        </div>
      }

      <!-- Size and time estimate, before the upload starts -->
      @if (selectedFile && !error && !isProbing && !isUploading && !landedJobId) {
        <div class="mt-4 glass-card p-4 text-left" role="status">
          <p class="text-sm">
            <span class="font-medium">{{ formatFileSize(selectedFile.size) }}</span> to upload.
            Estimated time: <span class="font-medium">{{ estimateLabel }}</span>{{ ' ' }}
            <span class="text-[hsl(var(--muted-foreground))]">({{ estimateBasis }}).</span>
          </p>
          @if (resumable) {
            <p class="mt-2 text-sm">
              An earlier upload of this file reached {{ resumablePercent }}%. Resume upload continues from there.
              <button
                type="button"
                (click)="startOver()"
                class="ml-1 font-medium text-[hsl(var(--primary))] underline underline-offset-4"
              >
                Start over
              </button>
            </p>
          }
          @if (isLargeUpload) {
            <p class="mt-2 text-sm flex items-start gap-2">
              <svg lucideIcon="clock" [size]="16" class="mt-0.5 shrink-0 text-amber-400"></svg>
              <span>
                Large file. The upload keeps running in the background while this tab stays open, and it
                continues from where it stopped if your connection drops. If you close the tab, come back
                to this page and choose the same file to continue.
              </span>
            </p>
          }
        </div>
      }

      <!-- Upload Progress -->
      @if (uploadProgress >= 0 && !landedJobId) {
        <div class="mt-6">
          <div class="flex justify-between items-center text-sm text-[hsl(var(--muted-foreground))] mb-2">
            <span>{{ uploadLabel }}</span>
            <span class="flex items-center gap-3">
              <span>{{ uploadProgress }}%</span>
              <button
                type="button"
                (click)="cancelUpload()"
                class="rounded-full border border-[hsl(var(--border))] px-3 py-1 text-[12.5px] font-medium hover:bg-white/5"
              >
                Cancel
              </button>
            </span>
          </div>
          <div
            class="h-2 rounded-full bg-[hsl(var(--secondary))] overflow-hidden"
            role="progressbar"
            aria-label="Upload progress"
            aria-valuemin="0"
            aria-valuemax="100"
            [attr.aria-valuenow]="uploadProgress"
          >
              <div
                class="h-full rounded-full bg-[hsl(var(--primary))] transition-all duration-300"
                [style.width.%]="uploadProgress"
              ></div>
          </div>
          @if (progress) {
            <p class="mt-2 text-xs text-[hsl(var(--muted-foreground))]">{{ progressDetail }}</p>
            @if (progressNote) {
              <p class="mt-1 text-xs" aria-live="polite">{{ progressNote }}</p>
            }
          }
          @if (isLargeUpload) {
            <p class="mt-2 text-xs text-[hsl(var(--muted-foreground))]">
              Keep this tab open. The upload keeps running while you use other tabs.
            </p>
          }
        </div>
      }

      <!-- Effects step: the file is uploaded, and processing starts only when the user does -->
      @if (landedJobId) {
        <div class="mt-6 text-left" aria-label="Effects before processing">
          <h2 class="font-semibold text-[15px]">Upload complete</h2>
          <p class="mt-1 text-[13px] text-[hsl(var(--muted-foreground))]">
            Choose video effects, then start processing. Effects are off by default, so you can start right away.
          </p>
          <app-shorts-effects-panel
            class="mt-4 block"
            [jobId]="landedJobId"
            [startStep]="true"
            (effectsSaved)="onEffectsSaved()"
          ></app-shorts-effects-panel>
        </div>
      }

      @if (!landedJobId) {
      <!-- Language + Submit -->
      <div class="mt-6 flex flex-col sm:flex-row sm:items-end justify-between gap-4">
        <label class="block text-left">
          <span class="block text-sm font-medium">Transcript language</span>
          <span class="block text-[12px] text-[hsl(var(--muted-foreground))]">
            Auto-detect works for most files. Pick a language if the spoken audio is Hindi or English and detection guesses wrong.
          </span>
          <select
            class="mt-2 w-full sm:w-64 rounded-lg border border-[hsl(var(--border))] bg-[hsl(var(--background))] px-3 py-2 text-sm min-h-[44px]"
            [value]="language"
            (change)="onLanguageChange($event)"
            [disabled]="isUploading"
            aria-label="Transcript language"
          >
            @for (lang of languages; track lang.code) {
              <option [value]="lang.code">{{ lang.label }}</option>
            }
          </select>
        </label>
        <button
          class="glow-btn btn-interactive"
          [disabled]="!selectedFile || !!error || isUploading || isProbing"
          [class.opacity-50]="!selectedFile || !!error || isUploading || isProbing"
          [class.pointer-events-none]="!selectedFile || !!error || isUploading || isProbing"
          (click)="startProcessing()"
        >
          @if (isUploading) {
            <svg lucideIcon="loader-2" [size]="18" class="animate-spin"></svg>
            Uploading…
          } @else {
            <svg lucideIcon="play" [size]="18"></svg>
            {{ resumable ? 'Resume upload' : 'Generate Shorts' }}
          }
        </button>
      </div>
      }

      <!-- Upload Error -->
      @if (uploadError) {
        <div class="mt-4 p-3 glass-card !border-red-500/30">
          <p class="text-sm text-red-400 flex items-center gap-2">
            <svg lucideIcon="circle-x" [size]="16"></svg>
            {{ uploadError }}
          </p>
          @if (!landedJobId) {
            <button
              type="button"
              (click)="retryUpload()"
              [disabled]="isUploading || isProbing"
              class="mt-2 rounded-full border border-[hsl(var(--border))] px-4 py-2 text-[13px] font-medium hover:bg-white/5 disabled:opacity-50"
            >
              {{ resumable ? 'Resume upload' : 'Try again' }}
            </button>
          }
        </div>
      }
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
          this.dragPreviewUrl = URL.createObjectURL(file);
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
    this.shorts.createJob(this.fileType, this.durationSeconds, this.language).subscribe({
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
      this.previewUrl = URL.createObjectURL(file);
    }
    this.probeDuration(file, ext);
  }

  /** Probe media duration with a hidden media element's loadedmetadata event. */
  private probeDuration(file: File, ext: string): void {
    this.isProbing = true;
    const isAudio = ext === 'mp3' || ext === 'wav';
    const media = document.createElement(isAudio ? 'audio' : 'video') as HTMLMediaElement;
    media.preload = 'metadata';
    const objectUrl = URL.createObjectURL(file);

    const cleanup = () => {
      URL.revokeObjectURL(objectUrl);
      this.isProbing = false;
    };

    media.onloadedmetadata = () => {
      const duration = media.duration;
      cleanup();
      if (!isFinite(duration) || duration <= 0) {
        this.error = 'Could not read media duration. Please try a different file.';
        return;
      }
      if (duration > this.limits.maxDurationSeconds) {
        this.error = `File is too long (${this.formatDuration(duration)}). Maximum length is ${this.limits.maxDurationMinutes} minutes.`;
        return;
      }
      this.durationSeconds = duration;
    };

    media.onerror = () => {
      cleanup();
      this.error = 'Could not read this media file. Please try a different file.';
    };

    media.src = objectUrl;
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
