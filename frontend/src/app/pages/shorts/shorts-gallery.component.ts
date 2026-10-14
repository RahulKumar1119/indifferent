import { Component, OnDestroy, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { firstValueFrom } from 'rxjs';
import { ActivatedRoute, Router } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import JSZip from 'jszip';
import { Clip, ShortsService, CaptionStyle, DEFAULT_CAPTION_STYLE } from './shorts.service';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { ToastsComponent } from '../../shared/components/toast/toasts.component';
import { ToastService } from '../../shared/components/toast/toast.service';
import { ConfirmDialogComponent } from '../../shared/components/confirm-dialog/confirm-dialog.component';

/** Hover previews play at most this long before pausing again. */
const HOVER_PREVIEW_MS = 2000;

/** Fractions sampled for the thumbnail frame strip. */
const STRIP_FRACS = [0.1, 0.25, 0.4, 0.55, 0.7];

interface CaptionPresetMeta {
  id: NonNullable<CaptionStyle['preset']>;
  label: string;
  blurb: string;
}

const CAPTION_PRESETS: CaptionPresetMeta[] = [
  { id: 'minimal', label: 'Minimal', blurb: 'Small, subtle, no box' },
  { id: 'bold', label: 'Bold', blurb: 'Large white on dark box' },
  { id: 'karaoke', label: 'Karaoke Highlight', blurb: 'Word-by-word sweep' },
  { id: 'clean', label: 'Clean Subtitles', blurb: 'Classic outlined text' },
];

@Component({
  selector: 'app-shorts-gallery',
  standalone: true,
  imports: [CommonModule, FormsModule, LucideDynamicIcon, NavbarComponent, ToastsComponent, ConfirmDialogComponent],
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
    <app-navbar></app-navbar>
    <app-toasts></app-toasts>
    <div class="max-w-7xl mx-auto px-4 py-8">
      <div class="flex flex-wrap items-center justify-between gap-3 mb-6">
        <div>
          <h1 class="text-2xl font-bold mb-1">Your Shorts</h1>
          <p class="text-[hsl(var(--muted-foreground))]">
            Ranked by predicted engagement. Hover to preview, download or share.
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          @if (clips.length > 1) {
            <button
              class="btn-interactive px-4 py-2 rounded-lg border border-[hsl(var(--border))] hover:bg-white/5 transition-colors text-sm font-medium disabled:opacity-50"
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
              class="btn-interactive px-4 py-2 rounded-lg border border-[hsl(var(--border))] hover:bg-white/5 transition-colors text-sm font-medium"
              (click)="copyAllCaptions()"
            >
              {{ allCopied ? 'Copied!' : 'Copy all captions' }}
            </button>
          }
          <button
            class="btn-interactive px-4 py-2 rounded-lg border border-[hsl(var(--border))] hover:bg-white/5 transition-colors text-sm font-medium inline-flex items-center gap-1.5"
            (click)="toggleStylePanel()"
            [attr.aria-expanded]="styleOpen"
            aria-label="Caption styles"
          >
            <svg lucideIcon="captions" [size]="15"></svg>
            Captions
          </button>
          <button
            class="btn-interactive px-4 py-2 rounded-lg border border-[hsl(var(--border))] hover:bg-white/5 transition-colors text-sm font-medium"
            (click)="goToUpload()"
          >
            New Shorts
          </button>
          <button
            class="btn-interactive px-4 py-2 rounded-lg border border-red-500/30 text-red-400 hover:bg-red-500/10 transition-colors text-sm font-medium inline-flex items-center gap-1.5"
            (click)="pendingDelete = true"
            [disabled]="isLoading || !!error"
            aria-label="Delete this short and all its assets"
          >
            <svg lucideIcon="trash-2" [size]="15"></svg>
            Delete
          </button>
        </div>
      </div>

      @if (isLoading) {
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5" aria-label="Loading clips">
          @for (i of [1,2,3,4]; track i) {
            <div class="skeleton aspect-[9/16] rounded-xl"></div>
          }
        </div>
      }

      @if (error) {
        <div class="p-3 glass-card !border-red-500/30">
          <p class="text-sm text-red-400 flex items-center gap-2">
            <svg lucideIcon="circle-x" [size]="16"></svg>
            {{ error }}
          </p>
          <button
            type="button"
            (click)="reload()"
            class="mt-3 rounded-full border border-[hsl(var(--border))] px-4 py-2 text-[13px] font-medium hover:bg-white/5"
          >
            Try again
          </button>
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
        <!-- Caption studio -->
        @if (styleOpen) {
          <div class="mb-4 glass-card p-5" aria-label="Caption styles">
            <div class="flex items-center justify-between gap-2">
              <h2 class="font-semibold text-[15px]">Caption style</h2>
              <span class="text-[12px] text-[hsl(var(--muted-foreground))]">Saved per job · used by renders and retries</span>
            </div>
            @if (styleLoaded && styleLoadError) {
              <p class="mt-2 rounded-xl border border-red-500/30 bg-red-500/[0.06] px-4 py-2.5 text-[13px] text-red-400" role="alert">
                Couldn't load the saved style. Check your connection and reopen this panel.
              </p>
            } @else if (styleLoaded && !styleEditable) {
              <p class="mt-2 rounded-xl border border-amber-500/30 bg-amber-500/[0.07] px-4 py-2.5 text-[13px] text-[hsl(var(--muted-foreground))]" role="note">
                This job is already rendered — styles apply to future renders. Retry the job to re-render with a new style.
              </p>
            }
            <div class="mt-3 grid grid-cols-2 lg:grid-cols-4 gap-2.5" role="group" aria-label="Caption presets">
              @for (p of captionPresets; track p.id) {
                <button
                  type="button"
                  (click)="selectPreset(p.id)"
                  [attr.aria-pressed]="styleDraft.preset === p.id"
                  class="rounded-xl border p-3 text-left transition-colors min-h-[44px]"
                  [class]="styleDraft.preset === p.id ? 'border-[hsl(var(--primary))] bg-[hsl(var(--primary))]/[.07]' : 'border-[hsl(var(--border))] hover:bg-white/5'"
                >
                  <span class="block text-[13.5px] font-semibold">{{ p.label }}</span>
                  <span class="block text-[11.5px] text-[hsl(var(--muted-foreground))]">{{ p.blurb }}</span>
                  <span class="mt-2 block rounded-lg bg-black px-2 py-3 text-center" aria-hidden="true">
                    @if (p.id === 'karaoke') {
                      <span [ngStyle]="swatchStyle(p.id)">Sample</span>
                      <span style="color:#9AA0A6" [style.font-size]="swatchStyle(p.id)['font-size']"> caption</span>
                    } @else {
                      <span [ngStyle]="swatchStyle(p.id)">Sample caption</span>
                    }
                  </span>
                  <span class="mt-1.5 block rounded-lg bg-zinc-200 px-2 py-3 text-center" aria-hidden="true">
                    @if (p.id === 'karaoke') {
                      <span [ngStyle]="swatchStyle(p.id)">Sample</span>
                      <span style="color:#5b5b5b" [style.font-size]="swatchStyle(p.id)['font-size']"> caption</span>
                    } @else {
                      <span [ngStyle]="swatchStyle(p.id)">Sample caption</span>
                    }
                  </span>
                </button>
              }
            </div>
            <div class="mt-4 grid grid-cols-1 sm:grid-cols-2 gap-4">
              <label class="block">
                <span class="text-[12.5px] font-medium">Size · <span class="tabular-nums">{{ styleDraft.fontSize }}</span></span>
                <input type="range" min="8" max="72" step="1" [(ngModel)]="styleDraft.fontSize" class="w-full" aria-label="Caption font size" />
              </label>
              <div class="flex items-center gap-4">
                <label class="flex items-center gap-2 text-[12.5px] font-medium">
                  Text
                  <input type="color" [(ngModel)]="styleDraft.color" class="w-10 h-10 rounded-lg border border-[hsl(var(--border))] cursor-pointer bg-transparent" aria-label="Caption text color" />
                </label>
                <label class="flex items-center gap-2 text-[12.5px] font-medium">
                  Highlight
                  <input type="color" [(ngModel)]="styleDraft.highlight" class="w-10 h-10 rounded-lg border border-[hsl(var(--border))] cursor-pointer bg-transparent" aria-label="Karaoke highlight color" />
                </label>
              </div>
            </div>
            <div class="mt-3 flex flex-wrap items-center gap-2">
              <span class="text-[12.5px] font-medium" id="cap-pos-label">Position</span>
              <div class="flex gap-1.5" role="group" aria-labelledby="cap-pos-label">
                @for (pos of positionOptions; track pos) {
                  <button
                    type="button"
                    (click)="styleDraft.position = pos"
                    [attr.aria-pressed]="styleDraft.position === pos"
                    class="min-h-[40px] px-4 rounded-full border text-[13px] font-medium capitalize"
                    [class]="styleDraft.position === pos ? 'bg-[hsl(var(--foreground))] text-[hsl(var(--background))] border-transparent' : 'border-[hsl(var(--border))]'"
                  >{{ pos }}</button>
                }
              </div>
              <span class="flex-1"></span>
              <button type="button" (click)="resetStyleDraft()" class="min-h-[40px] px-4 rounded-full border border-[hsl(var(--border))] text-[13px] font-medium hover:bg-white/5">Defaults</button>
              <button
                type="button"
                (click)="saveStyle()"
                [disabled]="styleSaving || !styleEditable"
                [title]="!styleEditable && styleLoaded ? 'Styles apply to future renders only' : 'Save caption style'"
                class="btn-interactive min-h-[40px] px-5 rounded-full bg-[hsl(var(--primary))] text-white text-[13.5px] font-semibold disabled:opacity-50"
              >{{ styleSaving ? 'Saving…' : 'Save style' }}</button>
            </div>
            @if (styleError) {
              <p class="mt-2 text-[13px] text-red-400" role="alert">{{ styleError }}</p>
            }
          </div>
        }
        <!-- Selection toolbar -->        <div class="mb-4 flex flex-wrap items-center gap-2" aria-label="Clip selection">
          <button
            type="button"
            (click)="toggleSelectAll()"
            [attr.aria-pressed]="allSelected"
            class="btn-interactive inline-flex items-center gap-2 px-4 py-2 rounded-lg border border-[hsl(var(--border))] hover:bg-white/5 transition-colors text-sm font-medium min-h-[40px]"
          >
            <span
              class="w-5 h-5 rounded-md border flex items-center justify-center shrink-0"
              [class]="allSelected ? 'bg-[hsl(var(--primary))] border-[hsl(var(--primary))]' : 'border-[hsl(var(--muted-foreground))]'"
              aria-hidden="true"
            >
              @if (allSelected) {
                <svg lucideIcon="check" [size]="14" class="text-white"></svg>
              }
            </span>
            {{ allSelected ? 'Deselect all' : 'Select all' }}
          </button>
          <span class="text-[13px] text-[hsl(var(--muted-foreground))]" role="status" aria-live="polite">
            {{ selectedCount }} of {{ clips.length }} selected
          </span>
          <span class="flex-1"></span>
          @if (selectedCount > 0) {
            <button
              type="button"
              (click)="clearSelection()"
              class="btn-interactive px-4 py-2 rounded-lg border border-[hsl(var(--border))] hover:bg-white/5 transition-colors text-sm font-medium min-h-[40px]"
            >
              Clear
            </button>
          }
          <button
            type="button"
            (click)="downloadSelected()"
            [disabled]="selectedCount === 0 || bulkActive"
            class="btn-interactive inline-flex items-center gap-1.5 px-4 py-2 rounded-lg bg-[hsl(var(--primary))] text-white text-sm font-semibold disabled:opacity-40 min-h-[40px]"
          >
            @if (bulkActive) {
              <svg lucideIcon="loader-2" [size]="15" class="animate-spin"></svg>
              {{ bulkLabel }}
            } @else {
              <svg lucideIcon="download" [size]="15"></svg>
              Download selected
            }
          </button>
        </div>

        @if (bulkActive || bulkFailed.length > 0) {
          <div class="mb-4 glass-card p-4" role="status" aria-live="polite">
            <div class="flex justify-between text-[13px] text-[hsl(var(--muted-foreground))] mb-2">
              <span>{{ bulkLabel }}</span>
              <span>{{ bulkDone }} / {{ bulkTotal }}</span>
            </div>
            <div class="h-2 rounded-full bg-[hsl(var(--secondary))] overflow-hidden">
              <div
                class="h-full rounded-full bg-gradient-to-r from-[hsl(var(--primary))] to-purple-400 transition-all duration-300"
                [style.width.%]="bulkTotal ? (bulkDone / bulkTotal) * 100 : 0"
              ></div>
            </div>
            @if (bulkFailed.length > 0 && !bulkActive) {
              <p class="mt-2 text-[13px] text-red-400">
                Couldn't download: {{ bulkFailed.join(', ') }}. The rest saved successfully — retry these individually.
              </p>
            }
          </div>
        }

        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5">
          @for (clip of clips; track clip.clipId; let i = $index) {
            <div class="glass-card p-3 flex flex-col card-enter group" [style.animation-delay]="(i * 0.05) + 's'" [attr.data-clip]="clip.clipId" [class.ring-2]="isSelected(clip)" [class.ring-[hsl(var(--primary))]]="isSelected(clip)">
              <!-- 9:16 video -->
              <div
                class="relative w-full rounded-lg overflow-hidden bg-black aspect-[9/16]"
                (mouseenter)="hoverPlay(clip, $event)"
                (mouseleave)="hoverStop(clip)"
              >
                @if (showVideo[clip.clipId] && previewUrls[clip.clipId]) {
                  <video
                    [src]="previewUrls[clip.clipId]"
                    muted
                    playsinline
                    preload="metadata"
                    class="absolute inset-0 w-full h-full object-contain bg-black cursor-pointer"
                    (click)="togglePlay(clip, $event)"
                  ></video>
                } @else if (thumbs[clip.clipId]) {
                  <img
                    [src]="thumbs[clip.clipId]"
                    [alt]="'Thumbnail for clip ' + clip.rank"
                    class="absolute inset-0 w-full h-full object-cover"
                    loading="lazy"
                  />
                  <button
                    type="button"
                    (click)="playThumb(clip)"
                    [attr.aria-label]="'Play clip ' + clip.rank"
                    class="absolute inset-0 flex items-center justify-center bg-black/0 hover:bg-black/30 transition-colors group/play"
                  >
                    <span class="w-12 h-12 rounded-full bg-white/90 text-black flex items-center justify-center transition-transform group-hover/play:scale-105" aria-hidden="true">
                      <svg lucideIcon="play" [size]="20"></svg>
                    </span>
                  </button>
                } @else {
                  <button
                    class="absolute inset-0 flex flex-col items-center justify-center gap-2 text-[hsl(var(--muted-foreground))] hover:text-white transition-colors"
                    [disabled]="loadingPreview[clip.clipId]"
                    (click)="playThumb(clip)"
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

                <!-- Selection checkbox -->
                <button
                  type="button"
                  (click)="toggleSelect(clip)"
                  role="checkbox"
                  [attr.aria-checked]="isSelected(clip)"
                  [attr.aria-label]="(isSelected(clip) ? 'Deselect' : 'Select') + ' clip ' + clip.rank"
                  class="absolute top-2 left-2 z-10 w-9 h-9 rounded-full backdrop-blur flex items-center justify-center transition-colors"
                  [class]="isSelected(clip) ? 'bg-[hsl(var(--primary))] text-white' : 'bg-black/55 text-white hover:bg-black/75'"
                >
                  @if (isSelected(clip)) {
                    <svg lucideIcon="check" [size]="17"></svg>
                  } @else {
                    <span class="w-[18px] h-[18px] rounded-full border-2 border-white/80" aria-hidden="true"></span>
                  }
                </button>

                <!-- Rank badge -->
                <span
                  class="absolute bottom-2 left-2 inline-flex items-center text-[11px] font-bold px-2 py-1 rounded-full text-white shadow"
                  [class]="rankBadgeClass(clip.rank)"
                >
                  #{{ clip.rank }}
                </span>

                <!-- Engagement score ring -->
                <span
                  class="absolute top-2 right-2 w-9 h-9 rounded-full bg-black/60 flex items-center justify-center"
                  [attr.aria-label]="'Engagement score ' + scorePercent(clip) + '%'"
                  [title]="'Engagement score ' + scorePercent(clip) + '%'"
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
                <button
                  class="inline-flex items-center gap-1 px-2 py-1.5 rounded-lg hover:bg-white/5 transition-colors text-[13px]"
                  (click)="toggleStrip(clip)"
                  [attr.aria-expanded]="stripFor === clip.clipId"
                  [attr.aria-label]="(stripFor === clip.clipId ? 'Hide' : 'Choose') + ' thumbnail frames for clip ' + clip.rank"
                >
                  <svg lucideIcon="image" [size]="15"></svg>
                  <span class="hidden sm:inline">Frames</span>
                </button>
              </div>

              @if (stripFor === clip.clipId) {
                <div class="mt-2 flex gap-2 overflow-x-auto pb-1" role="group" aria-label="Thumbnail frame choices">
                  @if (stripLoading && !(stripThumbs[clip.clipId]?.length)) {
                    @for (i of [1, 2, 3, 4, 5]; track i) {
                      <div class="skeleton w-[54px] h-[96px] rounded-lg shrink-0"></div>
                    }
                  } @else {
                    @for (f of stripThumbs[clip.clipId] ?? []; track $index; let fi = $index) {
                      <button
                        type="button"
                        (click)="selectStripFrame(clip, fi)"
                        [attr.aria-pressed]="thumbs[clip.clipId] === f"
                        [attr.aria-label]="'Use frame ' + (fi + 1) + ' as thumbnail for clip ' + clip.rank"
                        class="shrink-0 rounded-lg overflow-hidden border-2 transition-colors"
                        [class]="thumbs[clip.clipId] === f ? 'border-[hsl(var(--primary))]' : 'border-transparent hover:border-white/40'"
                      >
                        <img [src]="f" [alt]="'Frame option ' + (fi + 1)" class="w-[54px] h-[96px] object-cover" loading="lazy" />
                      </button>
                    }
                    @if (!(stripThumbs[clip.clipId]?.length)) {
                      <p class="text-[12px] text-[hsl(var(--muted-foreground))] py-2">Play the clip first, then pick a frame.</p>
                    }
                  }
                </div>
              }

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

    <app-confirm-dialog
      [open]="pendingDelete"
      title="Delete Short?"
      [message]="'Permanently delete Short ' + jobLabel + ' (' + clips.length + (clips.length === 1 ? ' clip' : ' clips') + ') and all of its assets?'"
      [details]="['Original uploaded media', 'AI transcript and ranking data', 'All rendered vertical clips', 'Linked project references are cleaned up']"
      [busy]="deleting"
      (confirmed)="confirmDelete()"
      (cancelled)="pendingDelete = false"
    />
  `,
})
export class ShortsGalleryComponent implements OnInit, OnDestroy {
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
  pendingDelete = false;
  deleting = false;

  /** Per-card poster thumbnails captured from the video (data URLs). */
  thumbs: Record<string, string> = {};
  /** Cards currently showing the live video element instead of the poster. */
  showVideo: Record<string, boolean> = {};
  /** Open frame strip (clipId) + captured strip frames + loading flag. */
  stripFor: string | null = null;
  stripThumbs: Record<string, string[]> = {};
  stripLoading = false;

  /** Caption studio state (per-job reusable style). */
  styleOpen = false;
  styleLoaded = false;
  styleLoadError = false;
  styleSaving = false;
  styleError = '';
  styleJobStatus = '';
  styleDraft: Required<CaptionStyle> = { ...DEFAULT_CAPTION_STYLE };
  readonly captionPresets = CAPTION_PRESETS;
  readonly positionOptions: NonNullable<CaptionStyle['position']>[] = ['top', 'middle', 'bottom'];

  /** Selected clip IDs. Keyed by stable clipId so selection survives
   * re-sorts and list refreshes; pruned when the list changes. */
  private readonly selectedIds = new Set<string>();

  bulkActive = false;
  bulkDone = 0;
  bulkTotal = 0;
  bulkFailed: string[] = [];

  get jobLabel(): string {
    return this.jobId ? this.jobId.slice(0, 8) : 'this job';
  }

  get selectedCount(): number {
    return this.selectedIds.size;
  }

  get allSelected(): boolean {
    return this.clips.length > 0 && this.clips.every((c) => this.selectedIds.has(c.clipId));
  }

  get bulkLabel(): string {
    if (!this.bulkActive) return '';
    return `Fetching ${Math.min(this.bulkDone + 1, this.bulkTotal)} of ${this.bulkTotal}…`;
  }

  isSelected(clip: Clip): boolean {
    return this.selectedIds.has(clip.clipId);
  }

  toggleSelect(clip: Clip): void {
    if (this.selectedIds.has(clip.clipId)) {
      this.selectedIds.delete(clip.clipId);
    } else {
      this.selectedIds.add(clip.clipId);
    }
  }

  toggleSelectAll(): void {
    if (this.allSelected) {
      this.selectedIds.clear();
    } else {
      this.clips.forEach((c) => this.selectedIds.add(c.clipId));
    }
  }

  clearSelection(): void {
    this.selectedIds.clear();
  }

  private pruneSelection(): void {
    const live = new Set(this.clips.map((c) => c.clipId));
    [...this.selectedIds].forEach((id) => {
      if (!live.has(id)) this.selectedIds.delete(id);
    });
  }

  private hoverTimers: Record<string, ReturnType<typeof setTimeout>> = {};

  constructor(
    private readonly route: ActivatedRoute,
    private readonly router: Router,
    private readonly shorts: ShortsService,
    private readonly toasts: ToastService,
  ) {}

  ngOnInit(): void {
    this.jobId = this.route.snapshot.paramMap.get('id') || '';
    if (!this.jobId) {
      this.router.navigate(['/shorts']);
      return;
    }
    this.reload();
  }

  reload(): void {
    if (!this.jobId) return;
    this.isLoading = true;
    this.error = '';
    this.shorts.listClips(this.jobId).subscribe({
      next: (clips) => {
        this.clips = [...clips].sort((a, b) => a.rank - b.rank);
        this.pruneSelection();
        this.isLoading = false;
        // Warm poster thumbnails in the background (sequential, quiet):
        // cards show a representative frame without any tap.
        void this.warmThumbnails();
      },
      error: (err) => {
        this.isLoading = false;
        this.error = err?.error?.message || 'Failed to load clips. Please try again.';
      },
    });
  }

  /** Fetch URLs + capture posters for all clips, skipping restoring ones
   * silently (their tap-to-preview fallback stays). Never throws. */
  private async warmThumbnails(): Promise<void> {
    for (const clip of this.clips) {
      if (this.thumbs[clip.clipId] || this.previewUrls[clip.clipId]) continue;
      try {
        const res = await firstValueFrom(this.shorts.getClipUrl(this.jobId, clip.clipId));
        if (!res.url) continue;
        this.previewUrls[clip.clipId] = res.url;
        await this.captureThumb(clip);
      } catch {
        // Poster stays on the tap-to-preview fallback.
      }
    }
  }

  scorePercent(clip: Clip): number {
    return Math.round(clip.score * 100);
  }

  scoreColor(clip: Clip): string {
    const pct = this.scorePercent(clip);
    if (pct > 80) return 'var(--color-success)';
    if (pct >= 50) return 'var(--color-warning)';
    return 'var(--color-error)';
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

  preview(clip: Clip): Promise<void> {
    if (this.previewUrls[clip.clipId] || this.loadingPreview[clip.clipId]) {
      return Promise.resolve();
    }
    this.loadingPreview[clip.clipId] = true;
    return new Promise((resolve) => {
      this.shorts.getClipUrl(this.jobId, clip.clipId).subscribe({
        next: (res) => {
          this.loadingPreview[clip.clipId] = false;
          if (!res.url) {
            // 202 RESTORING lands here (2xx): archived clip is being restored.
            this.error = res.message || 'This clip is in cold storage and is being restored. Please check back in a few hours.';
            resolve();
            return;
          }
          this.previewUrls[clip.clipId] = res.url;
          // Capture the poster thumbnail lazily (async, non-blocking).
          void this.captureThumb(clip).catch(() => {});
          resolve();
        },
        error: () => {
          this.loadingPreview[clip.clipId] = false;
          this.error = 'Failed to load clip preview. Please try again.';
          resolve();
        },
      });
    });
  }

  /** Tap the poster: reveal the player and start playback (with sound). */
  playThumb(clip: Clip): void {
    if (this.showVideo[clip.clipId]) return;
    void this.preview(clip).then(() => {
      if (!this.previewUrls[clip.clipId]) return;
      this.showVideo[clip.clipId] = true;
      requestAnimationFrame(() => {
        const el = document.querySelector(`[data-clip="${clip.clipId}"] video`);
        if (el instanceof HTMLVideoElement) {
          el.muted = false;
          el.play().catch(() => {
            el.muted = true;
            el.play().catch(() => {});
          });
        }
      });
    });
  }

  /** Capture one poster frame at a fraction of the duration (data URL). */
  private captureFrame(url: string, frac: number): Promise<string> {
    return new Promise((resolve, reject) => {
      const video = document.createElement('video');
      video.muted = true;
      video.preload = 'auto';
      video.crossOrigin = 'anonymous';
      const done = (fn: () => void) => {
        window.clearTimeout(timer);
        video.pause();
        video.removeAttribute('src');
        fn();
      };
      const timer = window.setTimeout(() => done(() => reject(new Error('frame timeout'))), 10000);
      video.onloadedmetadata = () => {
        const dur = video.duration || 0;
        if (!isFinite(dur) || dur <= 0) {
          done(() => reject(new Error('no duration')));
          return;
        }
        video.currentTime = Math.min(Math.max(0, frac * dur), Math.max(0, dur - 0.1));
      };
      video.onseeked = () => {
        try {
          const canvas = document.createElement('canvas');
          canvas.width = 270;
          canvas.height = 480;
          const ctx = canvas.getContext('2d');
          if (!ctx) {
            done(() => reject(new Error('no ctx')));
            return;
          }
          // Cover-crop the frame into the 9:16 poster.
          const vw = video.videoWidth || 270;
          const vh = video.videoHeight || 480;
          const scale = Math.max(270 / vw, 480 / vh);
          const dw = vw * scale;
          const dh = vh * scale;
          ctx.drawImage(video, (270 - dw) / 2, (480 - dh) / 2, dw, dh);
          const url = canvas.toDataURL('image/jpeg', 0.7);
          done(() => resolve(url));
        } catch (e) {
          done(() => reject(e instanceof Error ? e : new Error('capture failed')));
        }
      };
      video.onerror = () => done(() => reject(new Error('video error')));
      video.src = url;
    });
  }

  private async captureThumb(clip: Clip): Promise<void> {
    const url = this.previewUrls[clip.clipId];
    if (!url || this.thumbs[clip.clipId]) return;
    try {
      this.thumbs[clip.clipId] = await this.captureFrame(url, 0.25);
    } catch {
      // Poster stays on the tap-to-preview fallback.
    }
  }

  toggleStrip(clip: Clip): void {
    if (this.stripFor === clip.clipId) {
      this.stripFor = null;
      return;
    }
    this.stripFor = clip.clipId;
    if (this.stripThumbs[clip.clipId] || this.stripLoading) return;
    const url = this.previewUrls[clip.clipId];
    if (!url) return;
    this.stripLoading = true;
    (async () => {
      const frames: string[] = [];
      for (const frac of STRIP_FRACS) {
        try {
          frames.push(await this.captureFrame(url, frac));
        } catch {
          // Skip undecodable positions; keep the strip usable.
        }
      }
      // Only apply if the user hasn't moved on (or the job reloaded).
      if (this.stripFor === clip.clipId) {
        this.stripThumbs[clip.clipId] = frames;
      }
      this.stripLoading = false;
    })();
  }

  selectStripFrame(clip: Clip, index: number): void {
    const frames = this.stripThumbs[clip.clipId];
    if (frames && frames[index]) {
      this.thumbs[clip.clipId] = frames[index];
    }
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
      void this.preview(clip).then(() => {
        this.showVideo[clip.clipId] = true;
        requestAnimationFrame(() => {
          const el = document.querySelector(`[data-clip="${clip.clipId}"] video`);
          if (el instanceof HTMLVideoElement) {
            el.muted = true;
            el.play().catch(() => {});
            this.clearHoverTimer(clip.clipId);
            this.hoverTimers[clip.clipId] = setTimeout(() => el.pause(), HOVER_PREVIEW_MS);
          }
        });
      });
    }
  }

  hoverStop(clip: Clip): void {
    this.clearHoverTimer(clip.clipId);
    const el = document.querySelector(`[data-clip="${clip.clipId}"] video`);
    if (el instanceof HTMLVideoElement && !el.paused) {
      el.pause();
    }
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
        this.toasts.show('Caption copied to clipboard.', 'success');
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
        this.toasts.show(`${this.clips.length} captions copied to clipboard.`, 'success');
        setTimeout(() => { this.allCopied = false; }, 2000);
      } else {
        this.error = 'Copy failed. Please try again.';
      }
    });
  }

  async downloadSelected(): Promise<void> {
    const targets = this.clips.filter((c) => this.selectedIds.has(c.clipId));
    if (targets.length === 0 || this.bulkActive) return;
    this.bulkActive = true;
    this.bulkDone = 0;
    this.bulkTotal = targets.length;
    this.bulkFailed = [];
    this.error = '';
    try {
      const zip = new JSZip();
      for (const clip of targets) {
        try {
          const url = await this.clipUrl(clip);
          const res = await fetch(url);
          if (!res.ok) throw new Error(`fetch ${res.status}`);
          zip.file(`short-${clip.rank}.mp4`, await res.blob());
        } catch {
          this.bulkFailed.push(`clip #${clip.rank}`);
        } finally {
          this.bulkDone++;
        }
      }
      if (this.bulkDone - this.bulkFailed.length === 0) {
        throw new Error('all-failed');
      }
      const content = await zip.generateAsync({ type: 'blob' });
      const objectUrl = URL.createObjectURL(content);
      this.triggerDownload(objectUrl, `shorts-selected-${targets.length}.zip`);
      setTimeout(() => URL.revokeObjectURL(objectUrl), 60_000);
      if (this.bulkFailed.length === 0) {
        this.toasts.show(`Downloaded ${targets.length} selected ${targets.length === 1 ? 'clip' : 'clips'}.`, 'success');
      } else {
        this.error = `Downloaded ${targets.length - this.bulkFailed.length} of ${targets.length}. Failed: ${this.bulkFailed.join(', ')}.`;
        this.toasts.show('Some clips failed to download — see details above.', 'error');
      }
    } catch {
      this.bulkFailed = targets.map((c) => `clip #${c.rank}`);
      this.error = 'Selected download failed. Please try clips individually.';
    } finally {
      this.bulkActive = false;
    }
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
      this.toasts.show(`Downloaded ${this.clips.length} clips as ZIP.`, 'success');
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
    } catch (err) {
      // A dismissed share sheet rejects with AbortError: the user declined,
      // so don't touch the clipboard. Fall back to copy only when the
      // native share API is missing or really failed.
      if (err instanceof DOMException && err.name === 'AbortError') {
        return;
      }
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

  ngOnDestroy(): void {
    Object.keys(this.hoverTimers).forEach((id) => this.clearHoverTimer(id));
  }

  // ---------------------------------------------------------- caption studio

  toggleStylePanel(): void {
    this.styleOpen = !this.styleOpen;
    if (this.styleOpen && !this.styleLoaded) {
      this.loadStyle();
    }
  }

  selectPreset(preset: NonNullable<CaptionStyle['preset']>): void {
    this.styleDraft = { ...this.styleDraft, preset };
  }

  resetStyleDraft(): void {
    this.styleDraft = { ...DEFAULT_CAPTION_STYLE };
  }

  saveStyle(): void {
    if (this.styleSaving || !this.jobId) return;
    if (this.styleLoaded && !this.styleEditable) return;
    this.styleSaving = true;
    this.styleError = '';
    this.shorts.updateCaption(this.jobId, { ...this.styleDraft }).subscribe({
      next: () => {
        this.styleSaving = false;
        this.toasts.show('Caption style saved. Applies to future renders and retries.', 'success');
      },
      error: (err) => {
        this.styleSaving = false;
        this.styleError = err?.error?.message || 'Could not save caption style.';
      },
    });
  }

  private loadStyle(): void {
    if (!this.jobId) return;
    this.styleLoadError = false;
    this.shorts.getStatus(this.jobId).subscribe({
      next: (job) => {
        this.styleDraft = { ...DEFAULT_CAPTION_STYLE, ...(job.caption ?? {}) };
        this.styleJobStatus = job.status ?? '';
        this.styleLoaded = true;
      },
      error: () => {
        this.styleDraft = { ...DEFAULT_CAPTION_STYLE };
        this.styleJobStatus = '';
        this.styleLoadError = true;
        this.styleLoaded = true;
      },
    });
  }

  /** Styles apply to future renders: completed/in-flight jobs are read-only. */
  get styleEditable(): boolean {
    return this.styleJobStatus === 'uploaded' || this.styleJobStatus === 'failed';
  }

  /** HTML approximation of a preset for the studio preview cards. */
  swatchStyle(presetId: string): Record<string, string> {
    const px = Math.round(10 + this.styleDraft.fontSize * 0.35);
    const color = this.styleDraft.color || '#FFFFFF';
    switch (presetId) {
      case 'minimal':
        return { color, 'font-size': `${Math.max(11, px - 3)}px`, 'text-shadow': '0 1px 3px rgba(0,0,0,0.8)' };
      case 'karaoke':
        return { color: this.styleDraft.highlight || '#FFD60A', 'font-size': `${px + 3}px`, 'font-weight': '800', 'text-shadow': '0 2px 4px rgba(0,0,0,0.9)' };
      case 'clean':
        return { color, 'font-size': `${px}px`, 'text-shadow': '-1px 0 0 #000, 1px 0 0 #000, 0 -1px 0 #000, 0 1px 0 #000' };
      default:
        return { color, 'font-size': `${px + 3}px`, 'font-weight': '800', background: 'rgba(0,0,0,0.55)', padding: '2px 8px', 'border-radius': '6px' };
    }
  }

  confirmDelete(): void {
    if (!this.jobId || this.deleting) return;
    this.deleting = true;
    this.shorts.deleteJob(this.jobId).subscribe({
      next: () => {
        this.toasts.show('Short deleted successfully.', 'success');
        this.router.navigate(['/shorts/history']);
      },
      error: () => {
        this.deleting = false;
        this.pendingDelete = false;
        this.toasts.show('Could not delete this short. Please try again.', 'error');
      },
    });
  }
}
