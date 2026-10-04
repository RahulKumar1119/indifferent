import { Component, OnDestroy, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router, RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { ShortsService } from './shorts.service';

const MAX_DURATION_SECONDS = 600; // 10 minutes (Requirement 9.1)
const ACCEPTED_EXTENSIONS = ['mp4', 'mov', 'mp3', 'wav'];

@Component({
  selector: 'app-shorts-upload',
  standalone: true,
  imports: [CommonModule, LucideDynamicIcon],
  template: `
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
            Drop your footage anywhere — we find the best moments and cut vertical 9:16 clips.
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
          <p class="mt-1.5 text-xs font-semibold">Max 10 min</p>
          <p class="text-[11px] text-[hsl(var(--muted-foreground))]">per upload</p>
        </div>
        <div class="glass-card p-3 text-center">
          <svg lucideIcon="scissors" [size]="20" class="mx-auto text-[hsl(var(--primary))]"></svg>
          <p class="mt-1.5 text-xs font-semibold">3 × 9:16 clips</p>
          <p class="text-[11px] text-[hsl(var(--muted-foreground))]">1080×1920, captioned</p>
        </div>
      </div>

      <!-- Full-viewport drop zone -->
      <div
        class="relative flex-1 mt-4 glass-card p-10 text-center transition-all cursor-pointer flex flex-col items-center justify-center min-h-[46dvh]"
        [class.!border-[hsl(var(--primary))]]="isDragOver"
        [class.!shadow-[0_0_30px_rgba(120,60,255,0.2)]]="isDragOver"
        [class.!border-green-500]="selectedFile && !error"
        (click)="fileInput.click()"
        (keydown.enter)="fileInput.click()"
        tabindex="0"
        role="button"
        aria-label="Drop zone for media file upload. Accepts MP4, MOV, MP3, WAV up to 10 minutes."
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
            <p class="text-xl font-medium">{{ isDragOver ? 'Drop it — we take it from here' : 'Drag & drop anywhere on this page' }}</p>
            <p class="text-sm text-[hsl(var(--muted-foreground))]">or</p>
            <button
              class="px-4 py-2 rounded-lg border border-[hsl(var(--border))] hover:bg-white/5 transition-colors text-sm font-medium"
              (click)="$event.stopPropagation()"
            >
              Browse Files
            </button>
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
          <div class="space-y-3">
            <div class="w-16 h-16 rounded-full bg-green-500/10 flex items-center justify-center mx-auto">
              <svg lucideIcon="file-text" [size]="32" class="text-green-400"></svg>
            </div>
            <p class="text-lg font-medium">{{ selectedFile.name }}</p>
            <p class="text-sm text-[hsl(var(--muted-foreground))]">
              {{ formatFileSize(selectedFile.size) }} · {{ formatDuration(durationSeconds) }}
            </p>
            <button
              class="px-4 py-2 rounded-lg border border-red-500/30 text-red-400 hover:bg-red-500/10 transition-colors text-sm"
              (click)="removeFile($event)"
            >
              Remove
            </button>
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

      <!-- Upload Progress -->
      @if (uploadProgress >= 0) {
        <div class="mt-6">
          <div class="flex justify-between text-sm text-[hsl(var(--muted-foreground))] mb-2">
            <span>Uploading…</span>
            <span>{{ uploadProgress }}%</span>
          </div>
          <div class="h-2 rounded-full bg-[hsl(var(--secondary))] overflow-hidden">
            <div
              class="h-full rounded-full bg-gradient-to-r from-[hsl(var(--primary))] to-purple-400 transition-all duration-300"
              [style.width.%]="uploadProgress"
            ></div>
          </div>
        </div>
      }

      <!-- Submit -->
      <div class="flex justify-end mt-6">
        <button
          class="glow-btn"
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
            Generate Shorts
          }
        </button>
      </div>

      <!-- Upload Error -->
      @if (uploadError) {
        <div class="mt-4 p-3 glass-card !border-red-500/30">
          <p class="text-sm text-red-400 flex items-center gap-2">
            <svg lucideIcon="circle-x" [size]="16"></svg>
            {{ uploadError }}
          </p>
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

  private readonly preventWindowDrop = (event: DragEvent): void => {
    // Drops outside the zone must never navigate the browser away.
    event.preventDefault();
  };

  constructor(
    private readonly router: Router,
    private readonly shorts: ShortsService,
  ) {}

  ngOnInit(): void {
    window.addEventListener('dragover', this.preventWindowDrop);
    window.addEventListener('drop', this.preventWindowDrop);
  }

  ngOnDestroy(): void {
    window.removeEventListener('dragover', this.preventWindowDrop);
    window.removeEventListener('drop', this.preventWindowDrop);
  }

  onDragOver(event: DragEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.isDragOver = true;
  }

  onDragLeave(event: DragEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.isDragOver = false;
  }

  onDrop(event: DragEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.isDragOver = false;
    const files = event.dataTransfer?.files;
    if (files && files.length > 0) {
      this.handleFile(files[0]);
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

  formatFileSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
  }

  formatDuration(seconds: number): string {
    const total = Math.round(seconds);
    const mins = Math.floor(total / 60);
    const secs = total % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  }

  startProcessing(): void {
    if (!this.selectedFile || this.error || this.isUploading || this.isProbing) return;

    this.isUploading = true;
    this.uploadError = '';
    this.uploadProgress = 0;

    this.shorts.createJob(this.fileType, this.durationSeconds).subscribe({
      next: (res) => this.uploadToS3(res.uploadUrl, res.jobId),
      error: (err) => this.failUpload(err, 'Failed to initiate upload. Please try again.'),
    });
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

    this.selectedFile = file;
    this.fileType = ext;
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
      if (duration > MAX_DURATION_SECONDS) {
        this.error = `File is too long (${this.formatDuration(duration)}). Maximum length is 10 minutes.`;
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

  private uploadToS3(uploadUrl: string, jobId: string): void {
    const file = this.selectedFile!;
    const xhr = new XMLHttpRequest();
    xhr.open('PUT', uploadUrl, true);
    xhr.setRequestHeader('ngsw-bypass', 'true');

    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) {
        this.uploadProgress = Math.round((event.loaded / event.total) * 100);
      }
    };

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        this.uploadProgress = 100;
        this.startPipeline(jobId);
      } else {
        this.uploadProgress = -1;
        this.isUploading = false;
        this.uploadError = `Upload failed with status ${xhr.status}. Please try again.`;
      }
    };

    xhr.onerror = () => {
      this.uploadProgress = -1;
      this.isUploading = false;
      this.uploadError = 'Upload failed. Please try again.';
    };

    xhr.send(file);
  }

  private startPipeline(jobId: string): void {
    this.shorts.startJob(jobId).subscribe({
      next: () => {
        this.isUploading = false;
        this.router.navigate(['/shorts', jobId, 'progress']);
      },
      error: (err) => this.failUpload(err, 'Failed to start processing. Please try again.'),
    });
  }

  private failUpload(err: { error?: { message?: string } }, fallback: string): void {
    this.isUploading = false;
    this.uploadProgress = -1;
    this.uploadError = err?.error?.message || fallback;
  }

  private reset(): void {
    this.selectedFile = null;
    this.fileType = '';
    this.durationSeconds = 0;
    this.error = '';
    this.uploadError = '';
    this.isProbing = false;
  }
}
