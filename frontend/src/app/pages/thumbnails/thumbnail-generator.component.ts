import { Component, OnDestroy, OnInit, computed, inject, signal } from '@angular/core';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { ToastsComponent } from '../../shared/components/toast/toasts.component';
import { ToastService } from '../../shared/components/toast/toast.service';
import { ThumbnailCardComponent } from './thumbnail-card.component';
import { ThumbnailPreviewComponent } from './thumbnail-preview.component';
import { ThumbnailProgressComponent } from './thumbnail-progress.component';
import { ThumbnailStyleSelectorComponent } from './thumbnail-style-selector.component';
import {
  NOT_RETRYABLE,
  THUMBNAIL_ERROR_HELP,
  Thumbnail,
  ThumbnailFormat,
  ThumbnailService,
  ThumbnailSet,
  ThumbnailStyle,
  isInFlight,
} from './thumbnail.service';

/** How often to ask while a generation is running. */
const POLL_ACTIVE_MS = 2500;
/** How often to look for a generation the pipeline starts by itself. */
const POLL_WAITING_MS = 6000;
/** Retry delay after a failed poll. */
const POLL_ERROR_MS = 6000;

/**
 * AI thumbnails for one Shorts job (issue #77): shows progress while they are
 * made, then three suggestions to preview, choose, download, or regenerate.
 */
@Component({
  selector: 'app-thumbnail-generator',
  standalone: true,
  imports: [
    RouterLink,
    LucideDynamicIcon,
    NavbarComponent,
    ToastsComponent,
    ThumbnailCardComponent,
    ThumbnailPreviewComponent,
    ThumbnailProgressComponent,
    ThumbnailStyleSelectorComponent,
  ],
  styles: [`
    :host { display: block; }
    .back { display: inline-flex; align-items: center; gap: var(--space-2xs); min-height: 44px; color: var(--color-ink-2); font-size: var(--type-sm); text-decoration: none; align-self: flex-start; }
    .back:hover { color: var(--color-ink); text-decoration: underline; }
    .title { display: flex; align-items: center; gap: var(--space-xs); }
    .title svg { color: var(--color-accent-text); }
    .grid { display: grid; gap: var(--space-lg); grid-template-columns: minmax(0, 1fr); }
    @media (min-width: 40rem) { .grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
    @media (min-width: 64rem) { .grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
    .tall { width: 100%; max-width: 18.75rem; margin-inline: auto; }
    .sub { margin: var(--space-2xs) 0 var(--space-md); color: var(--color-ink-2); font-size: var(--type-sm); }
    .bar { margin-top: var(--space-lg); display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-sm); }
    .ok { font-size: var(--type-sm); color: var(--color-positive); font-weight: 500; }
    .fail p + p { margin: 0; color: var(--color-ink-2); font-size: var(--type-sm); font-weight: 400; }
    .opts .ui-btn--primary { margin-top: var(--space-lg); }
    .hint { margin: var(--space-sm) 0 0; font-size: var(--type-xs); color: var(--color-muted); }
  `],
  template: `
    <div class="ui-app">
      <app-navbar></app-navbar>
      <app-toasts></app-toasts>
      <main id="main" class="ui-app-main hm">
        <a [routerLink]="['/shorts', jobId, 'clips']" class="back">
          <svg lucideIcon="chevron-right" [size]="16" style="transform: rotate(180deg)" aria-hidden="true"></svg>
          Back to your clips
        </a>
        <div>
          <h1 class="ui-h1 title"><svg lucideIcon="sparkles" [size]="24" aria-hidden="true"></svg>AI thumbnails</h1>
          <p class="ui-lead">We look through your video, pick its strongest moments and design thumbnail suggestions with the text already on them.</p>
        </div>

        @if (loading()) {
          <div class="grid" role="status" aria-label="Loading thumbnails">
            @for (i of [1, 2, 3]; track i) {
              <div class="ui-skeleton" style="aspect-ratio: 16 / 9"></div>
            }
          </div>
        } @else if (loadError()) {
          <div class="ui-card ui-error-card" role="alert">
            <p><svg lucideIcon="circle-x" [size]="16" aria-hidden="true"></svg>We couldn't load your thumbnails.</p>
            <button type="button" class="ui-btn ui-btn--secondary" (click)="reload()">Try again</button>
          </div>
        } @else if (set(); as s) {
          @if (!s.supported) {
            <div class="ui-card" role="status">
              <h2 class="ui-card-title">No video to work from</h2>
              <p class="ui-card-sub">This job was made from an audio file, so there is no picture to take thumbnails from. Upload a video to get thumbnails.</p>
            </div>
          } @else if (inFlight()) {
            <section class="ui-card" aria-labelledby="making-title">
              <h2 id="making-title" class="ui-card-title">Making your thumbnails</h2>
              <p class="sub">This usually takes a minute or two. You can leave this page and come back; it keeps going.</p>
              <app-thumbnail-progress [stage]="s.stage"></app-thumbnail-progress>
            </section>
          } @else {
            @if (s.status === 'failed') {
              <div class="ui-card ui-error-card fail" role="alert">
                <p><svg lucideIcon="circle-x" [size]="18" aria-hidden="true"></svg>Thumbnail generation didn't finish</p>
                <p>{{ errorHelp() }}</p>
                @if (canRetry() && s.thumbnails.length > 0) {
                  <button type="button" class="ui-btn ui-btn--secondary" [disabled]="starting()" (click)="generate()">
                    @if (starting()) {
                      <svg lucideIcon="loader-2" [size]="16" class="ui-spin" aria-hidden="true"></svg>
                    } @else {
                      <svg lucideIcon="refresh-cw" [size]="16" aria-hidden="true"></svg>
                    }
                    Retry
                  </button>
                }
              </div>
            }
            @if (s.thumbnails.length > 0) {
              <section aria-labelledby="results-title">
                <h2 id="results-title" class="ui-card-title">We analyzed your video and created {{ s.thumbnails.length }} thumbnail suggestions</h2>
                <p class="sub">{{ summary() }}</p>
                <div class="grid">
                  @for (t of s.thumbnails; track t.id; let i = $index) {
                    <div [class.tall]="isPortrait()">
                      <app-thumbnail-card
                        [thumb]="t"
                        [number]="i + 1"
                        [chosen]="chosenId() === t.id"
                        [saved]="t.selected"
                        [downloading]="downloadingId() === t.id"
                        (choose)="choose(t.id)"
                        (download)="download(t)"
                        (preview)="previewId.set(t.id)"
                      ></app-thumbnail-card>
                    </div>
                  }
                </div>
                <div class="bar">
                  <button type="button" class="ui-btn ui-btn--secondary" [attr.aria-expanded]="showOptions()" aria-controls="regen-panel" (click)="showOptions.set(!showOptions())">
                    <svg lucideIcon="refresh-cw" [size]="16" aria-hidden="true"></svg>
                    Regenerate
                  </button>
                  <div class="ui-row">
                    @if (savedThumb()) {
                      <span class="ok" role="status">Saved as the thumbnail for this video</span>
                    }
                    <button type="button" class="ui-btn ui-btn--primary" data-testid="use-selected" [disabled]="!canSave() || saving()" (click)="useSelected()">
                      @if (saving()) {
                        <svg lucideIcon="loader-2" [size]="16" class="ui-spin" aria-hidden="true"></svg>
                      }
                      Use selected thumbnail
                    </button>
                  </div>
                </div>
              </section>
            } @else if (s.status !== 'failed') {
              <section class="ui-card" aria-labelledby="start-title">
                <h2 id="start-title" class="ui-card-title">Create thumbnails for this video</h2>
                @if (waitingForAutoStart()) {
                  <p class="ui-card-sub" role="status">Your video is still being transcribed. Thumbnails start by themselves once that is done, so you can wait here or come back later.</p>
                } @else {
                  <p class="ui-card-sub">Choose a style or leave it on Auto, and we'll make three different designs.</p>
                }
              </section>
            }
            @if ((showOptions() || s.thumbnails.length === 0) && !waitingForAutoStart() && canRetry()) {
              <section id="regen-panel" class="ui-card opts" aria-label="Thumbnail options">
                <app-thumbnail-style-selector [(style)]="style" [(format)]="format"></app-thumbnail-style-selector>
                <button type="button" class="ui-btn ui-btn--primary" data-testid="generate" [disabled]="starting()" (click)="generate()">
                  @if (starting()) {
                    <svg lucideIcon="loader-2" [size]="16" class="ui-spin" aria-hidden="true"></svg>
                  } @else {
                    <svg lucideIcon="sparkles" [size]="16" aria-hidden="true"></svg>
                  }
                  {{ s.thumbnails.length > 0 ? 'Regenerate thumbnails' : 'Generate thumbnails' }}
                </button>
                @if (s.thumbnails.length > 0) {
                  <p class="hint">Regenerating makes a fresh set and replaces these suggestions.</p>
                }
              </section>
            }
          }
        }
        <app-thumbnail-preview
          [thumb]="previewThumb()"
          [number]="previewNumber()"
          (closed)="previewId.set(null)"
          (choose)="previewChoose()"
          (download)="previewDownload()"
        ></app-thumbnail-preview>
      </main>
    </div>
  `,
})
export class ThumbnailGeneratorComponent implements OnInit, OnDestroy {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly thumbs = inject(ThumbnailService);
  private readonly toasts = inject(ToastService);

  jobId = '';
  private timer: ReturnType<typeof setTimeout> | null = null;
  private destroyed = false;

  readonly set = signal<ThumbnailSet | null>(null);
  readonly loading = signal(true);
  readonly loadError = signal(false);
  readonly starting = signal(false);
  readonly saving = signal(false);
  readonly downloadingId = signal<string | null>(null);
  readonly showOptions = signal(false);
  readonly style = signal<ThumbnailStyle | ''>('');
  readonly format = signal<ThumbnailFormat | ''>('');
  /** The thumbnail picked but not yet saved. */
  readonly chosenId = signal<string | null>(null);
  readonly previewId = signal<string | null>(null);

  readonly inFlight = computed(() => isInFlight(this.set()?.status));
  readonly isPortrait = computed(() => this.set()?.format === '9:16');
  readonly savedThumb = computed(() => this.set()?.thumbnail ?? null);
  readonly previewThumb = computed<Thumbnail | null>(() => this.set()?.thumbnails.find((t) => t.id === this.previewId()) ?? null);
  readonly previewNumber = computed(() => (this.set()?.thumbnails.findIndex((t) => t.id === this.previewId()) ?? -1) + 1 || 1);

  /** Nothing to save until a different thumbnail from the saved one is picked. */
  readonly canSave = computed(() => {
    const id = this.chosenId();
    return !!id && id !== this.savedThumb()?.id;
  });

  /** The video is still being transcribed, and the pipeline starts thumbnails afterwards. */
  readonly waitingForAutoStart = computed(() => {
    const s = this.set();
    return !!s && s.status === 'none' && (s.jobStatus === 'uploaded' || s.jobStatus === 'transcribing');
  });

  readonly canRetry = computed(() => !NOT_RETRYABLE.has(this.set()?.errorCode ?? ''));

  readonly errorHelp = computed(() => {
    const s = this.set();
    if (!s) return '';
    return THUMBNAIL_ERROR_HELP[s.errorCode ?? ''] ?? s.error ?? 'Something went wrong. Please try again.';
  });

  /** One line on what was looked at, built only from facts the server reported. */
  readonly summary = computed(() => {
    const s = this.set();
    if (!s) return '';
    const parts: string[] = [];
    if (s.funnel) parts.push(`Looked at ${s.funnel.extracted} frames and kept the best ${s.funnel.candidates}.`);
    if (s.analysis?.topic) parts.push(`Topic: ${s.analysis.topic}.`);
    parts.push(
      s.transcriptUsed
        ? 'The headline comes from your transcript.'
        : 'No transcript was available, so the headline is based on the frames alone.',
    );
    return parts.join(' ');
  });

  ngOnInit(): void {
    this.jobId = this.route.snapshot.paramMap.get('id') || '';
    if (!this.jobId) {
      this.router.navigate(['/shorts/history']);
      return;
    }
    this.load(true);
  }

  ngOnDestroy(): void {
    this.destroyed = true;
    this.clearTimer();
  }

  reload(): void {
    this.loadError.set(false);
    this.loading.set(true);
    this.load(true);
  }

  private load(first = false): void {
    this.clearTimer();
    this.thumbs.get(this.jobId).subscribe({
      next: (s) => {
        if (this.destroyed) return;
        this.applySet(s);
        this.loading.set(false);
        this.loadError.set(false);
        this.schedule(s);
      },
      error: () => {
        if (this.destroyed) return;
        this.loading.set(false);
        // Keep what is on screen if a later poll fails; only a first load is an error page.
        if (first && !this.set()) this.loadError.set(true);
        else this.timer = setTimeout(() => this.load(), POLL_ERROR_MS);
      },
    });
  }

  private applySet(s: ThumbnailSet): void {
    this.set.set(s);
    // Keep the picked thumbnail only while it still exists; follow a saved choice otherwise.
    const ids = new Set(s.thumbnails.map((t) => t.id));
    const current = this.chosenId();
    if (!current || !ids.has(current)) this.chosenId.set(s.thumbnail?.id ?? null);
  }

  private schedule(s: ThumbnailSet): void {
    this.clearTimer();
    if (isInFlight(s.status)) {
      this.timer = setTimeout(() => this.load(), POLL_ACTIVE_MS);
    } else if (s.status === 'none' && s.supported && (s.jobStatus === 'uploaded' || s.jobStatus === 'transcribing')) {
      this.timer = setTimeout(() => this.load(), POLL_WAITING_MS);
    }
  }

  private clearTimer(): void {
    if (this.timer !== null) {
      clearTimeout(this.timer);
      this.timer = null;
    }
  }

  generate(): void {
    if (this.starting()) return;
    this.starting.set(true);
    this.thumbs.start(this.jobId, { style: this.style(), format: this.format() }).subscribe({
      next: () => {
        this.starting.set(false);
        this.showOptions.set(false);
        this.chosenId.set(null);
        // Show progress straight away; the next poll fills in the real stage.
        const cur = this.set();
        if (cur) this.set.set({ ...cur, status: 'queued', stage: undefined, errorCode: undefined, error: undefined });
        this.load();
      },
      error: (err) => {
        this.starting.set(false);
        if (err?.status === 409) {
          // A run is already going; just follow it.
          this.load();
          return;
        }
        this.toasts.show(err?.error?.message || "Couldn't start thumbnail generation. Please try again.", 'error');
      },
    });
  }

  choose(id: string): void {
    this.chosenId.set(this.chosenId() === id ? null : id);
  }

  useSelected(): void {
    const id = this.chosenId();
    if (!id || this.saving()) return;
    this.saving.set(true);
    this.thumbs.select(this.jobId, id).subscribe({
      next: (s) => {
        this.saving.set(false);
        this.applySet(s);
        this.toasts.show('Saved as the thumbnail for this video.', 'success');
      },
      error: (err) => {
        this.saving.set(false);
        this.toasts.show(err?.error?.message || "Couldn't save your choice. Please try again.", 'error');
      },
    });
  }

  download(t: Thumbnail): void {
    if (this.downloadingId()) return;
    this.downloadingId.set(t.id);
    this.thumbs.downloadUrl(this.jobId, t.id).subscribe({
      next: ({ url }) => {
        this.downloadingId.set(null);
        // The link carries a "save as" header, so the browser downloads it and stays on this page.
        window.location.assign(url);
      },
      error: () => {
        this.downloadingId.set(null);
        this.toasts.show("Couldn't prepare the download. Please try again.", 'error');
      },
    });
  }

  previewChoose(): void {
    const id = this.previewId();
    if (id) this.chosenId.set(id);
  }

  previewDownload(): void {
    const t = this.previewThumb();
    if (t) this.download(t);
  }
}
