import { DecimalPipe } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  OnDestroy,
  inject,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { Subscription } from 'rxjs';
import {
  FailureNotice,
  StoryPlanProblem,
  describeProblem,
  describeProviderFailures,
  problemFromFailures,
} from './story-plan-problem';
import {
  MAX_QUERY_CHARS,
  MediaRef,
  MediaType,
  PlanScene,
  StockAsset,
  StockAttribution,
  StockSearchResponse,
  mediaKey,
  sceneSearchQuery,
} from './story-plan.models';
import { StoryPlanService } from './story-plan.service';
import { StoryAttributionComponent } from './story-attribution.component';
import { StoryProblemComponent } from './story-problem.component';

const PAGE_SIZE = 12;

type Phase = 'idle' | 'loading' | 'ready' | 'failed';

/**
 * Finds a photo or clip for one scene. It opens as a native modal dialog, so
 * the browser keeps focus inside, closes on Escape and puts focus back on the
 * button that opened it. It searches as soon as it opens, with the scene's own
 * keywords, and talks only to our API.
 *
 * A search that found nothing because the libraries failed is shown as a
 * failure, never as "no results".
 */
@Component({
  selector: 'app-story-media-picker',
  standalone: true,
  imports: [DecimalPipe, LucideDynamicIcon, StoryAttributionComponent, StoryProblemComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: [
    `
      dialog::backdrop {
        background: rgb(0 0 0 / 0.6);
      }
      .serif {
        font-family: 'Cormorant Garamond', Georgia, serif;
      }
      /* A phone held sideways has about 375px of height: a pinned header would
         leave almost none for the results, so the whole dialog scrolls instead. */
      @media (max-height: 600px) {
        .picker-shell {
          overflow-y: auto;
        }
        .picker-body {
          flex: none;
          overflow: visible;
        }
      }
    `,
  ],
  template: `
    <dialog
      #dialog
      aria-labelledby="picker-title"
      (close)="onClosed()"
      class="m-auto h-[100dvh] w-full max-w-none overflow-hidden bg-[hsl(var(--background))] p-0 text-[hsl(var(--foreground))] sm:h-auto sm:max-h-[90dvh] sm:max-w-5xl sm:rounded-2xl sm:border sm:border-[hsl(var(--border))]"
    >
      <div class="picker-shell flex h-full max-h-[100dvh] flex-col sm:max-h-[90dvh]">
        <header class="picker-header shrink-0 border-b border-[hsl(var(--border))] px-4 py-4 sm:px-6">
          <div class="flex items-start justify-between gap-4">
            <div class="min-w-0">
              <h2 id="picker-title" class="serif text-2xl font-medium leading-tight">
                Choose a {{ mediaType() === 'video' ? 'clip' : 'photo' }} for scene {{ sceneNumber() }}
              </h2>
              @if (scene(); as s) {
                <p class="mt-1 text-sm text-[hsl(var(--muted-foreground))] [overflow-wrap:anywhere]">
                  Suggested: {{ s.description }}
                </p>
              }
            </div>
            <button
              type="button"
              (click)="close()"
              aria-label="Close"
              class="inline-flex size-11 shrink-0 items-center justify-center rounded-full border border-[hsl(var(--border))] hover:bg-[hsl(var(--muted))]"
            >
              <svg lucideIcon="x" [size]="18" aria-hidden="true"></svg>
            </button>
          </div>

          <form class="mt-4 flex flex-col gap-3 sm:flex-row sm:items-end" role="search" (submit)="onSubmit($event)">
            <div class="min-w-0 flex-1">
              <label for="picker-query" class="block text-sm font-medium">Search in English</label>
              <input
                id="picker-query"
                type="search"
                autofocus
                autocomplete="off"
                [value]="query()"
                (input)="onQueryInput($event)"
                [attr.maxlength]="maxQuery"
                [attr.aria-invalid]="queryError() ? 'true' : null"
                [attr.aria-describedby]="queryError() ? 'picker-query-error' : null"
                class="mt-1 block min-h-11 w-full rounded-lg border border-[hsl(var(--border))] bg-[hsl(var(--background))] px-3 text-base"
              />
              @if (queryError()) {
                <p id="picker-query-error" role="alert" class="mt-1 flex items-start gap-2 text-sm font-medium">
                  <svg lucideIcon="triangle-alert" [size]="16" aria-hidden="true" class="mt-0.5 shrink-0 text-[var(--color-error)]"></svg>
                  {{ queryError() }}
                </p>
              }
            </div>
            <div class="flex items-center gap-2">
            <div role="group" aria-label="What to look for" class="flex gap-2">
              <button
                type="button"
                [attr.aria-pressed]="mediaType() === 'photo'"
                (click)="setType('photo')"
                [class]="typeButtonClass(mediaType() === 'photo')"
              >
                <svg lucideIcon="image" [size]="16" aria-hidden="true"></svg>
                Photos
              </button>
              <button
                type="button"
                [attr.aria-pressed]="mediaType() === 'video'"
                (click)="setType('video')"
                [class]="typeButtonClass(mediaType() === 'video')"
              >
                <svg lucideIcon="film" [size]="16" aria-hidden="true"></svg>
                Clips
              </button>
            </div>
            <button
              type="submit"
              class="inline-flex min-h-11 flex-1 items-center justify-center gap-2 rounded-full bg-[#BC5227] px-5 text-sm font-semibold text-white hover:bg-[#a84822] sm:flex-none"
            >
              <svg lucideIcon="search" [size]="16" aria-hidden="true"></svg>
              Search
            </button>
            </div>
          </form>
        </header>

        <div class="picker-body min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 py-4 sm:px-6">
          <p role="status" class="mb-3 text-sm text-[hsl(var(--muted-foreground))]" data-testid="picker-status">
            @switch (phase()) {
              @case ('loading') {
                Searching the photo libraries…
              }
              @case ('ready') {
                @if (assets().length) {
                  {{ assets().length }} {{ mediaType() === 'video' ? 'clips' : 'photos' }} for “{{ searched() }}”
                }
              }
            }
          </p>

          @if (problem(); as p) {
            <app-story-problem
              [problem]="p"
              [actionLabel]="p.retryable ? 'Search again' : ''"
              (action)="retry()"
            />
          }

          @if (notice(); as n) {
            <p
              class="mb-3 flex items-start gap-2 rounded-lg border border-[hsl(var(--border))] p-3 text-sm"
              role="status"
              data-testid="partial-notice"
            >
              <svg lucideIcon="triangle-alert" [size]="16" aria-hidden="true" class="mt-0.5 shrink-0"></svg>
              <span>{{ n.text }} Showing results from the other library only.</span>
            </p>
          }

          @if (phase() === 'loading') {
            <ul class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4" aria-hidden="true">
              @for (i of placeholders; track i) {
                <li class="aspect-[3/4] rounded-xl bg-[hsl(var(--muted))]"></li>
              }
            </ul>
          }

          @if (phase() === 'ready' && assets().length === 0 && !problem()) {
            <div class="rounded-xl border border-[hsl(var(--border))] p-5" data-testid="empty">
              <p class="font-semibold">
                No {{ mediaType() === 'video' ? 'clips' : 'photos' }} found for “{{ searched() }}”
              </p>
              <p class="mt-1 text-sm text-[hsl(var(--muted-foreground))]">
                Try fewer or different English words, or look for
                {{ mediaType() === 'video' ? 'photos' : 'clips' }} instead.
              </p>
              <button
                type="button"
                (click)="setType(mediaType() === 'video' ? 'photo' : 'video')"
                class="mt-3 inline-flex min-h-11 items-center rounded-full border border-[hsl(var(--border))] px-4 text-sm font-medium hover:bg-[hsl(var(--muted))]"
              >
                Search {{ mediaType() === 'video' ? 'photos' : 'clips' }} instead
              </button>
            </div>
          }

          @if (assets().length) {
            <ul class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4" data-testid="results">
              @for (asset of assets(); track assetKey(asset)) {
                <li class="flex min-w-0 flex-col overflow-hidden rounded-xl border border-[hsl(var(--border))]">
                  <div class="aspect-[3/4] bg-[hsl(var(--muted))]">
                    @if (asset.type === 'video' && asset.playUrl) {
                      <video
                        [src]="asset.playUrl"
                        [poster]="asset.previewUrl ?? ''"
                        controls
                        muted
                        playsinline
                        preload="none"
                        [attr.aria-label]="'Preview of ' + altOf(asset)"
                        class="size-full object-contain"
                      ></video>
                    } @else if (asset.previewUrl) {
                      <img [src]="asset.previewUrl" [alt]="altOf(asset)" loading="lazy" class="size-full object-contain" />
                    } @else {
                      <p class="flex size-full items-center justify-center p-3 text-center text-sm">No preview available</p>
                    }
                  </div>
                  <div class="flex flex-1 flex-col gap-2 p-3">
                    <p class="text-xs text-[hsl(var(--muted-foreground))]">
                      {{ orientationLabel(asset) }}@if (asset.duration) {
                        · {{ asset.duration | number: '1.0-0' }} s
                      }
                    </p>
                    <a
                      [href]="asset.pageUrl"
                      target="_blank"
                      rel="noopener noreferrer"
                      class="inline-flex min-h-11 items-center text-xs underline underline-offset-2 [overflow-wrap:anywhere]"
                      >{{ asset.credit }}<span class="sr-only"> (opens in a new tab)</span></a
                    >
                    @if (isCurrent(asset)) {
                      <p class="mt-auto inline-flex min-h-11 items-center gap-2 text-sm font-semibold">
                        <svg lucideIcon="check" [size]="16" aria-hidden="true"></svg>
                        Current choice
                      </p>
                    } @else {
                      <button
                        type="button"
                        (click)="choose(asset)"
                        class="mt-auto inline-flex min-h-11 items-center justify-center rounded-full border border-[hsl(var(--foreground))] px-4 text-sm font-semibold hover:bg-[hsl(var(--foreground))] hover:text-[hsl(var(--background))]"
                      >
                        Use this {{ asset.type === 'video' ? 'clip' : 'photo' }}<span class="sr-only">, {{ altOf(asset) }}</span>
                      </button>
                    }
                  </div>
                </li>
              }
            </ul>

            @if (hasMore()) {
              <div class="mt-4 flex justify-center">
                <button
                  type="button"
                  (click)="loadMore()"
                  [disabled]="loadingMore()"
                  class="inline-flex min-h-11 items-center gap-2 rounded-full border border-[hsl(var(--border))] px-5 text-sm font-medium hover:bg-[hsl(var(--muted))] disabled:opacity-60"
                >
                  @if (loadingMore()) {
                    <svg lucideIcon="loader-2" [size]="16" aria-hidden="true" class="animate-spin motion-reduce:animate-none"></svg>
                    Loading…
                  } @else {
                    Show more results
                  }
                </button>
              </div>
            }

            <div class="mt-4">
              <app-story-attribution [attributions]="attributions()" />
            </div>
          }
        </div>
      </div>
    </dialog>
  `,
})
export class StoryMediaPickerComponent implements OnDestroy {
  private readonly api = inject(StoryPlanService);
  private readonly dialog = viewChild.required<ElementRef<HTMLDialogElement>>('dialog');

  /** The user chose an asset. The dialog closes itself. */
  readonly picked = output<StockAsset>();

  protected readonly maxQuery = MAX_QUERY_CHARS;
  protected readonly placeholders = [1, 2, 3, 4, 5, 6, 7, 8];

  protected readonly scene = signal<PlanScene | null>(null);
  protected readonly sceneNumber = signal(0);
  protected readonly query = signal('');
  protected readonly mediaType = signal<MediaType>('photo');
  protected readonly phase = signal<Phase>('idle');
  protected readonly assets = signal<StockAsset[]>([]);
  protected readonly attributions = signal<StockAttribution[]>([]);
  protected readonly notice = signal<FailureNotice | null>(null);
  protected readonly problem = signal<StoryPlanProblem | null>(null);
  protected readonly hasMore = signal(false);
  protected readonly loadingMore = signal(false);
  protected readonly queryError = signal('');
  /** The query the shown results are for (the box may have been edited since). */
  protected readonly searched = signal('');

  private current: MediaRef | null = null;
  private page = 1;
  private sub?: Subscription;
  /** What the last failed request was, so "Search again" repeats that one. */
  private lastAction: 'search' | 'more' = 'search';

  /** Opens the picker for a scene and searches with the scene's keywords. */
  open(scene: PlanScene, sceneNumber: number, current: MediaRef | null): void {
    this.scene.set(scene);
    this.sceneNumber.set(sceneNumber);
    this.current = current;
    this.query.set(sceneSearchQuery(scene));
    this.mediaType.set(scene.mediaType === 'video' ? 'video' : 'photo');
    this.queryError.set('');
    this.resetResults();
    const el = this.dialog().nativeElement;
    if (!el.open) el.showModal();
    this.search();
  }

  close(): void {
    const el = this.dialog().nativeElement;
    if (el.open) el.close();
    this.onClosed();
  }

  ngOnDestroy(): void {
    this.sub?.unsubscribe();
  }

  protected onClosed(): void {
    this.sub?.unsubscribe();
    this.loadingMore.set(false);
  }

  protected onSubmit(event: Event): void {
    event.preventDefault();
    this.search();
  }

  protected onQueryInput(event: Event): void {
    this.query.set((event.target as HTMLInputElement).value);
    if (this.queryError()) this.queryError.set('');
  }

  protected setType(type: MediaType): void {
    this.mediaType.set(type);
    this.search();
  }

  protected retry(): void {
    if (this.lastAction === 'more') this.loadMore();
    else this.search();
  }

  protected search(): void {
    const q = this.query().trim();
    if (!q) {
      this.queryError.set('Type a word to search for, in English.');
      return;
    }
    this.queryError.set('');
    this.lastAction = 'search';
    this.sub?.unsubscribe();
    this.resetResults();
    this.page = 1;
    this.phase.set('loading');
    this.sub = this.api.search({ q, type: this.mediaType(), page: 1, perPage: PAGE_SIZE }).subscribe({
      next: (res) => this.show(res, q, false),
      error: (err) => {
        this.problem.set(describeProblem(err, 'search'));
        this.phase.set('failed');
      },
    });
  }

  protected loadMore(): void {
    const q = this.searched();
    if (!q || this.loadingMore()) return;
    this.lastAction = 'more';
    this.problem.set(null);
    this.loadingMore.set(true);
    this.sub?.unsubscribe();
    this.sub = this.api.search({ q, type: this.mediaType(), page: this.page + 1, perPage: PAGE_SIZE }).subscribe({
      next: (res) => {
        this.page += 1;
        this.show(res, q, true);
      },
      error: (err) => {
        // Keep what is already on screen; say why there is no more.
        this.problem.set(describeProblem(err, 'search'));
        this.loadingMore.set(false);
      },
    });
  }

  protected choose(asset: StockAsset): void {
    this.picked.emit(asset);
    this.close();
  }

  protected isCurrent(asset: StockAsset): boolean {
    return this.current !== null && mediaKey(this.current) === mediaKey(asset);
  }

  protected assetKey(asset: StockAsset): string {
    return mediaKey(asset);
  }

  protected altOf(asset: StockAsset): string {
    return asset.alt?.trim() || asset.credit;
  }

  protected orientationLabel(asset: StockAsset): string {
    const o = asset.orientation;
    return o ? o.charAt(0).toUpperCase() + o.slice(1) : '';
  }

  protected typeButtonClass(on: boolean): string {
    const base = 'inline-flex min-h-11 items-center gap-2 rounded-full border px-4 text-sm font-medium';
    return on
      ? `${base} border-[hsl(var(--foreground))] bg-[hsl(var(--foreground))] text-[hsl(var(--background))]`
      : `${base} border-[hsl(var(--border))] hover:bg-[hsl(var(--muted))]`;
  }

  private resetResults(): void {
    this.assets.set([]);
    this.attributions.set([]);
    this.notice.set(null);
    this.problem.set(null);
    this.hasMore.set(false);
    this.loadingMore.set(false);
    this.searched.set('');
    this.phase.set('idle');
  }

  private show(res: StockSearchResponse, q: string, append: boolean): void {
    const failures = res.failures ?? [];
    this.loadingMore.set(false);
    if (res.assets.length === 0 && failures.length > 0 && !append) {
      // Every library failed: that is not "nothing matches".
      this.problem.set(problemFromFailures(failures));
      this.phase.set('failed');
      return;
    }
    const seen = new Set(append ? this.assets().map(mediaKey) : []);
    const fresh = res.assets.filter((a) => !seen.has(mediaKey(a)));
    this.assets.set(append ? [...this.assets(), ...fresh] : fresh);
    this.attributions.set(res.attributions ?? []);
    this.notice.set(describeProviderFailures(failures));
    this.hasMore.set(res.hasMore);
    this.searched.set(q);
    this.phase.set('ready');
  }
}
