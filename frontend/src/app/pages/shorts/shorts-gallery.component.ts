import { Component, OnDestroy, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import JSZip from 'jszip';
import { Clip, ShortsService } from './shorts.service';

/** Hover previews play at most this long before pausing again. */
const HOVER_PREVIEW_MS = 2000;

@Component({
  selector: 'app-shorts-gallery',
  standalone: true,
  imports: [CommonModule, LucideDynamicIcon],
  styles: [`
    @keyframes card-in {
      from { opacity: 0; transform: translateY(14px); }
      to { opacity: 1; transform: none; }
    }
    .card-enter { opacity: 0; animation: card-in .45s cubic-bezier(0.23,1,0.32,1) forwards; }
    @media (prefers-reduced-motion: reduce) {
      .card-enter { opacity: 1; animation: none; }
    }
  `],
  template: `
    <div class="max-w-7xl mx-auto px-4 py-8">
      <div class="flex flex-wrap items-center justify-between gap-3 mb-6">
        <div>
          <h1 class="text-2xl font-bold mb-1">Your Shorts</h1>
          <p class="text-[hsl(var(--muted-foreground))]">
            Ranked by predicted engagement. Hover to preview, download or share.
          </p>
        </div>
        <div class="flex items-center gap-2">
          @if (clips.length > 1) {
            <button
              class="px-4 py-2 rounded-lg border border-[hsl(var(--border))] hover:bg-white/5 transition-colors text-sm font-medium disabled:opacity-50"
              [disabled]="zipping"
              (click)="downloadAll()"
            >
              @if (zipping) {
                <span class="inline-flex items-center gap-1.5"><svg lucideIcon="loader-2" [size]="16" class="animate-spin"></svg>Zipping…</span>
              } @else {
                Download all (ZIP)
              }
            </button>
            <button
              class="px-4 py-2 rounded-lg border border-[hsl(var(--border))] hover:bg-white/5 transition-colors text-sm font-medium"
              (click)="copyAllCaptions()"
            >
              {{ allCopied ? 'Copied!' : 'Copy all captions' }}
            </button>
          }
          <button
            class="px-4 py-2 rounded-lg border border-[hsl(var(--border))] hover:bg-white/5 transition-colors text-sm font-medium"
            (click)="goToUpload()"
          >
            New Shorts
          </button>
        </div>
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
          <svg lucideIcon="clapperboard" [size]="40" class="mx-auto text-[hsl(var(--muted-foreground))]"></svg>
          <p class="mt-3 font-medium">No clips were generated for this job.</p>
          <p class="mt-1 text-sm text-[hsl(var(--muted-foreground))]">Try a longer video for better results.</p>
        </div>
      }

      @if (clips.length > 0) {
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5">
          @for (clip of clips; track clip.clipId; let i = $index) {
            <div class="glass-card p-3 flex flex-col card-enter group" [style.animation-delay]="(i * 0.05) + 's'">
              <!-- 9:16 video -->
              <div
                class="relative w-full rounded-lg overflow-hidden bg-black aspect-[9/16]"
                (mouseenter)="hoverPlay(clip, $event)"
                (mouseleave)="hoverStop(clip)"
              >
                @if (previewUrls[clip.clipId]) {
                  <video
                    [src]="previewUrls[clip.clipId]"
                    muted
                    playsinline
                    preload="metadata"
                    class="absolute inset-0 w-full h-full object-contain bg-black cursor-pointer"
                    (click)="togglePlay(clip, $event)"
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
                      <span class="text-xs">Tap to preview</span>
                    }
                  </button>
                }

                <!-- Rank badge -->
                <span
                  class="absolute top-2 left-2 inline-flex items-center text-[11px] font-bold px-2 py-1 rounded-full text-white shadow"
                  [class]="rankBadgeClass(clip.rank)"
                >
                  #{{ clip.rank }}
                </span>

                <!-- Virality score ring -->
                <span
                  class="absolute top-2 right-2 w-9 h-9 rounded-full bg-black/60 flex items-center justify-center"
                  [attr.aria-label]="'Virality score ' + scorePercent(clip) + '%'"
                  [title]="'Virality score ' + scorePercent(clip) + '%'"
                >
                  <svg width="36" height="36" viewBox="0 0 36 36" class="-rotate-90">
                    <circle cx="18" cy="18" r="15.5" fill="none" stroke="rgba(255,255,255,0.2)" stroke-width="3.5" />
                    <circle
                      cx="18" cy="18" r="15.5" fill="none"
                      [attr.stroke]="scoreColor(clip)"
                      stroke-width="3.5" stroke-linecap="round"
                      [attr.stroke-dasharray]="ringDash(clip)"
                      stroke-dashoffset="0"
                    />
                  </svg>
                  <span class="absolute text-[9px] font-bold text-white">{{ scorePercent(clip) }}</span>
                </span>
              </div>

              <!-- Action bar: hover on desktop, always visible on touch -->
              <div class="flex items-center justify-between mt-2 md:opacity-0 md:group-hover:opacity-100 md:focus-within:opacity-100 transition-opacity">
                <button
                  class="inline-flex items-center gap-1 px-2 py-1.5 rounded-lg hover:bg-white/5 transition-colors text-[13px]"
                  [disabled]="downloading[clip.clipId]"
                  (click)="download(clip)"
                  aria-label="Download clip"
                >
                  @if (downloading[clip.clipId]) {
                    <svg lucideIcon="loader-2" [size]="15" class="animate-spin"></svg>
                  } @else {
                    <svg lucideIcon="download" [size]="15"></svg>
                  }
                  <span class="hidden sm:inline">Download</span>
                </button>
                <button
                  class="inline-flex items-center gap-1 px-2 py-1.5 rounded-lg hover:bg-white/5 transition-colors text-[13px]"
                  (click)="copyCaption(clip)"
                  aria-label="Copy caption"
                >
                  @if (copied[clip.clipId]) {
                    <svg lucideIcon="check" [size]="15" class="text-green-400"></svg>
                  } @else {
                    <svg lucideIcon="copy" [size]="15"></svg>
                  }
                  <span class="hidden sm:inline">{{ copied[clip.clipId] ? 'Copied' : 'Caption' }}</span>
                </button>
                <button
                  class="inline-flex items-center gap-1 px-2 py-1.5 rounded-lg hover:bg-white/5 transition-colors text-[13px]"
                  (click)="share(clip)"
                  aria-label="Share clip"
                >
                  <svg lucideIcon="share-2" [size]="15"></svg>
                  <span class="hidden sm:inline">Share</span>
                </button>
              </div>

              <p class="mt-1 px-1 text-[11px] text-[hsl(var(--muted-foreground))]">
                {{ formatDuration(clip.duration) }}
                @if (clip.hookText) {
                  · {{ clip.hookText }}
                }
              </p>
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
  copied: Record<string, boolean> = {};
  zipping = false;
  allCopied = false;

  private hoverTimers: Record<string, ReturnType<typeof setTimeout>> = {};

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

  scorePercent(clip: Clip): number {
    return Math.round(clip.score * 100);
  }

  scoreColor(clip: Clip): string {
    const pct = this.scorePercent(clip);
    if (pct > 80) return '#22c55e';
    if (pct >= 50) return '#eab308';
    return '#f97316';
  }

  ringDash(clip: Clip): string {
    const circ = 2 * Math.PI * 15.5;
    return `${(this.scorePercent(clip) / 100) * circ} ${circ}`;
  }

  rankBadgeClass(rank: number): string {
    if (rank === 1) return 'bg-gradient-to-br from-yellow-300 to-amber-600';
    if (rank === 2) return 'bg-gradient-to-br from-slate-200 to-slate-500';
    if (rank === 3) return 'bg-gradient-to-br from-orange-300 to-amber-700';
    return 'bg-black/60';
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

  /** Desktop hover: ensure a URL, then autoplay muted for a short burst. */
  hoverPlay(clip: Clip, event: MouseEvent): void {
    if (window.matchMedia('(hover: none)').matches) return;
    const video = (event.currentTarget as HTMLElement).querySelector('video');
    if (video) {
      video.muted = true;
      video.play().catch(() => {});
      this.clearHoverTimer(clip.clipId);
      this.hoverTimers[clip.clipId] = setTimeout(() => video.pause(), HOVER_PREVIEW_MS);
      return;
    }
    if (!this.previewUrls[clip.clipId] && !this.loadingPreview[clip.clipId]) {
      this.preview(clip);
    }
  }

  hoverStop(clip: Clip): void {
    this.clearHoverTimer(clip.clipId);
  }

  /** Mobile tap: toggle play on the loaded video. */
  togglePlay(clip: Clip, event: MouseEvent): void {
    const video = event.target as HTMLVideoElement;
    if (video.paused) {
      video.muted = false;
      video.play().catch(() => {});
    } else {
      video.pause();
    }
    this.clearHoverTimer(clip.clipId);
  }

  download(clip: Clip): void {
    if (this.downloading[clip.clipId]) return;
    this.downloading[clip.clipId] = true;
    this.withClipUrl(clip, (url) => {
      this.downloading[clip.clipId] = false;
      this.triggerDownload(url, `short-${clip.rank}.mp4`);
    }, () => {
      this.downloading[clip.clipId] = false;
      this.error = 'Failed to download clip. Please try again.';
    });
  }

  copyCaption(clip: Clip): void {
    this.copyText(this.captionText(clip)).then((ok) => {
      if (ok) {
        this.copied[clip.clipId] = true;
        setTimeout(() => { this.copied[clip.clipId] = false; }, 2000);
      } else {
        this.error = 'Copy failed. Please try again.';
      }
    });
  }

  copyAllCaptions(): void {
    const text = this.clips.map((c) => this.captionText(c)).join('\n\n');
    this.copyText(text).then((ok) => {
      if (ok) {
        this.allCopied = true;
        setTimeout(() => { this.allCopied = false; }, 2000);
      } else {
        this.error = 'Copy failed. Please try again.';
      }
    });
  }

  async downloadAll(): Promise<void> {
    if (this.zipping) return;
    this.zipping = true;
    try {
      const zip = new JSZip();
      const urls = await Promise.all(this.clips.map((c) => this.clipUrl(c)));
      const blobs = await Promise.all(
        urls.map(async (url, i) => {
          const res = await fetch(url);
          if (!res.ok) throw new Error(`fetch ${i}`);
          return res.blob();
        }),
      );
      blobs.forEach((blob, i) => {
        zip.file(`short-${this.clips[i].rank}.mp4`, blob);
      });
      const content = await zip.generateAsync({ type: 'blob' });
      const objectUrl = URL.createObjectURL(content);
      this.triggerDownload(objectUrl, 'shorts.zip');
      setTimeout(() => URL.revokeObjectURL(objectUrl), 60_000);
    } catch {
      this.error = 'ZIP download failed. Please try clips individually.';
    } finally {
      this.zipping = false;
    }
  }

  async share(clip: Clip): Promise<void> {
    const text = this.captionText(clip);
    try {
      const url = await this.clipUrl(clip);
      if (navigator.share) {
        await navigator.share({ title: `Short #${clip.rank}`, text, url });
        return;
      }
      throw new Error('no-share');
    } catch {
      const ok = await this.copyText(text);
      this.error = ok ? '' : 'Share failed. Please try again.';
      if (ok) {
        this.copied[clip.clipId] = true;
        setTimeout(() => { this.copied[clip.clipId] = false; }, 2000);
      }
    }
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

  private captionText(clip: Clip): string {
    const hook = (clip.hookText || '').trim();
    const base = hook || `Short #${clip.rank}`;
    return `${base}\n#shorts #viral #fyp`;
  }

  private clipUrl(clip: Clip): Promise<string> {
    if (this.previewUrls[clip.clipId]) {
      return Promise.resolve(this.previewUrls[clip.clipId]);
    }
    return new Promise((resolve, reject) => {
      this.shorts.getClipUrl(this.jobId, clip.clipId).subscribe({
        next: (res) => {
          if (!res.url) {
            this.error = res.message || 'This clip is in cold storage and is being restored. Please check back in a few hours.';
            reject(new Error('restoring'));
            return;
          }
          this.previewUrls[clip.clipId] = res.url;
          resolve(res.url);
        },
        error: (err) => reject(err),
      });
    });
  }

  private withClipUrl(clip: Clip, onUrl: (url: string) => void, onError: () => void): void {
    this.clipUrl(clip).then(onUrl).catch(onError);
  }

  private async copyText(text: string): Promise<boolean> {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // Fallback for non-secure contexts / older browsers.
      try {
        const area = document.createElement('textarea');
        area.value = text;
        area.style.position = 'fixed';
        area.style.opacity = '0';
        document.body.appendChild(area);
        area.select();
        const ok = document.execCommand('copy');
        document.body.removeChild(area);
        return ok;
      } catch {
        return false;
      }
    }
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

  private clearHoverTimer(clipId: string): void {
    const timer = this.hoverTimers[clipId];
    if (timer) {
      clearTimeout(timer);
      delete this.hoverTimers[clipId];
    }
  }
}
