import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { Clip, ShortsService } from './shorts.service';

@Component({
  selector: 'app-shorts-gallery',
  standalone: true,
  imports: [CommonModule, LucideDynamicIcon],
  template: `
    <div class="max-w-5xl mx-auto px-4 py-8">
      <div class="flex items-center justify-between mb-6">
        <div>
          <h1 class="text-2xl font-bold mb-1">Your Shorts</h1>
          <p class="text-[hsl(var(--muted-foreground))]">
            Ranked by predicted engagement. Preview or download each clip.
          </p>
        </div>
        <button
          class="px-4 py-2 rounded-lg border border-[hsl(var(--border))] hover:bg-white/5 transition-colors text-sm font-medium"
          (click)="goToUpload()"
        >
          New Shorts
        </button>
      </div>

      @if (isLoading) {
        <div class="glass-card p-10 text-center">
          <svg lucideIcon="loader-2" [size]="32" class="mx-auto text-[hsl(var(--primary))] animate-spin"></svg>
          <p class="mt-3 text-sm text-[hsl(var(--muted-foreground))]">Loading clips…</p>
        </div>
      }

      @if (error) {
        <div class="p-3 glass-card !border-red-500/30">
          <p class="text-sm text-red-400 flex items-center gap-2">
            <svg lucideIcon="circle-x" [size]="16"></svg>
            {{ error }}
          </p>
        </div>
      }

      @if (!isLoading && !error && clips.length === 0) {
        <div class="glass-card p-10 text-center">
          <p class="text-sm text-[hsl(var(--muted-foreground))]">No clips were generated for this job.</p>
        </div>
      }

      @if (clips.length > 0) {
        <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-6">
          @for (clip of clips; track clip.clipId) {
            <div class="glass-card p-4 flex flex-col">
              <div class="flex items-center justify-between mb-3">
                <span class="inline-flex items-center gap-1 text-xs font-semibold px-2 py-1 rounded-full bg-[hsl(var(--primary))]/15 text-[hsl(var(--primary))]">
                  Rank #{{ clip.rank }}
                </span>
                <span class="text-xs text-[hsl(var(--muted-foreground))]">
                  Score {{ (clip.score * 100) | number: '1.0-0' }}%
                </span>
              </div>

              <!-- 9:16 preview area -->
              <div class="relative w-full rounded-lg overflow-hidden bg-black" style="aspect-ratio: 9 / 16;">
                @if (previewUrls[clip.clipId]) {
                  <video
                    [src]="previewUrls[clip.clipId]"
                    controls
                    playsinline
                    class="absolute inset-0 w-full h-full object-contain bg-black"
                  ></video>
                } @else {
                  <button
                    class="absolute inset-0 flex flex-col items-center justify-center gap-2 text-[hsl(var(--muted-foreground))] hover:text-white transition-colors"
                    [disabled]="loadingPreview[clip.clipId]"
                    (click)="preview(clip)"
                    [attr.aria-label]="'Preview clip ' + clip.rank"
                  >
                    @if (loadingPreview[clip.clipId]) {
                      <svg lucideIcon="loader-2" [size]="36" class="animate-spin"></svg>
                    } @else {
                      <svg lucideIcon="play" [size]="36"></svg>
                      <span class="text-xs">Preview</span>
                    }
                  </button>
                }
              </div>

              <div class="flex items-center justify-between mt-3">
                <span class="text-xs text-[hsl(var(--muted-foreground))]">
                  {{ formatDuration(clip.duration) }}
                </span>
                <button
                  class="inline-flex items-center gap-1 px-3 py-1.5 rounded-lg border border-[hsl(var(--border))] hover:bg-white/5 transition-colors text-sm"
                  [disabled]="downloading[clip.clipId]"
                  (click)="download(clip)"
                >
                  @if (downloading[clip.clipId]) {
                    <svg lucideIcon="loader-2" [size]="16" class="animate-spin"></svg>
                  } @else {
                    <svg lucideIcon="download" [size]="16"></svg>
                  }
                  Download
                </button>
              </div>
            </div>
          }
        </div>
      }
    </div>
  `,
})
export class ShortsGalleryComponent implements OnInit {
  private jobId = '';

  clips: Clip[] = [];
  isLoading = true;
  error = '';

  previewUrls: Record<string, string> = {};
  loadingPreview: Record<string, boolean> = {};
  downloading: Record<string, boolean> = {};

  constructor(
    private readonly route: ActivatedRoute,
    private readonly router: Router,
    private readonly shorts: ShortsService,
  ) {}

  ngOnInit(): void {
    this.jobId = this.route.snapshot.paramMap.get('id') || '';
    if (!this.jobId) {
      this.router.navigate(['/shorts']);
      return;
    }

    this.shorts.listClips(this.jobId).subscribe({
      next: (clips) => {
        this.clips = [...clips].sort((a, b) => a.rank - b.rank);
        this.isLoading = false;
      },
      error: (err) => {
        this.isLoading = false;
        this.error = err?.error?.message || 'Failed to load clips. Please try again.';
      },
    });
  }

  preview(clip: Clip): void {
    if (this.previewUrls[clip.clipId] || this.loadingPreview[clip.clipId]) return;
    this.loadingPreview[clip.clipId] = true;
    this.shorts.getClipUrl(this.jobId, clip.clipId).subscribe({
      next: (res) => {
        this.loadingPreview[clip.clipId] = false;
        if (!res.url) {
          // 202 RESTORING lands here (2xx): archived clip is being restored.
          this.error = res.message || 'This clip is in cold storage and is being restored. Please check back in a few hours.';
          return;
        }
        this.previewUrls[clip.clipId] = res.url;
      },
      error: () => {
        this.loadingPreview[clip.clipId] = false;
        this.error = 'Failed to load clip preview. Please try again.';
      },
    });
  }

  download(clip: Clip): void {
    if (this.downloading[clip.clipId]) return;
    this.downloading[clip.clipId] = true;
    this.shorts.getClipUrl(this.jobId, clip.clipId).subscribe({
      next: (res) => {
        this.downloading[clip.clipId] = false;
        if (!res.url) {
          this.error = res.message || 'This clip is in cold storage and is being restored. Please check back in a few hours.';
          return;
        }
        this.triggerDownload(res.url, `short-${clip.rank}.mp4`);
      },
      error: () => {
        this.downloading[clip.clipId] = false;
        this.error = 'Failed to download clip. Please try again.';
      },
    });
  }

  formatDuration(seconds: number): string {
    const total = Math.round(seconds);
    const mins = Math.floor(total / 60);
    const secs = total % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  }

  goToUpload(): void {
    this.router.navigate(['/shorts']);
  }

  private triggerDownload(url: string, filename: string): void {
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = filename;
    anchor.rel = 'noopener';
    document.body.appendChild(anchor);
    anchor.click();
    document.body.removeChild(anchor);
  }
}
