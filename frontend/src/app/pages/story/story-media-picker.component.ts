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
  styles: [`
    dialog { margin: auto; padding: 0; width: 100%; max-width: none; height: 100dvh; max-height: 100dvh; overflow: hidden; border: 0; background: var(--color-paper); color: var(--color-ink); font-family: var(--font-body); }
    @media (min-width: 40rem) { dialog { height: auto; max-height: 90dvh; max-width: 64rem; border: 1px solid var(--color-rule); border-radius: var(--radius-surface); } }
    dialog::backdrop { background: color-mix(in oklab, oklch(0% 0 0) 60%, transparent); }
    .shell { display: flex; flex-direction: column; height: 100%; max-height: 100dvh; }
    @media (min-width: 40rem) { .shell { max-height: 90dvh; } }
    .head { flex: none; padding: var(--space-md); border-bottom: 1px solid var(--color-rule); }
    @media (min-width: 40rem) { .head { padding: var(--space-md) var(--space-lg); } }
    .top { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--space-md); }
    h2 { margin: 0; font-size: var(--type-xl); font-weight: 600; line-height: 1.2; }
    .sub { margin: var(--space-3xs) 0 0; font-size: var(--type-sm); color: var(--color-ink-2); overflow-wrap: anywhere; }
    .x { width: 44px; padding: 0; flex: none; }
    form { margin-top: var(--space-md); display: flex; flex-direction: column; gap: var(--space-sm); }
    @media (min-width: 40rem) { form { flex-direction: row; align-items: flex-end; } }
    .q { flex: 1; min-width: 0; }
    .q label { display: block; font-size: var(--type-sm); font-weight: 600; margin-bottom: var(--space-2xs); }
    .err { margin: var(--space-2xs) 0 0; display: flex; align-items: flex-start; gap: var(--space-xs); font-size: var(--type-sm); font-weight: 500; color: var(--color-negative); }
    .ctl { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-xs); }
    .seg { display: flex; gap: var(--space-xs); }
    .go { flex: 1; }
    @media (min-width: 40rem) { .go { flex: none; } }
    .body { flex: 1; min-height: 0; overflow-y: auto; overscroll-behavior: contain; padding: var(--space-md); display: flex; flex-direction: column; gap: var(--space-sm); }
    @media (min-width: 40rem) { .body { padding: var(--space-md) var(--space-lg); } }
    /* A phone held sideways has about 375px of height: a pinned header would
       leave almost none for the results, so the whole dialog scrolls instead. */
    @media (max-height: 600px) { .shell { overflow-y: auto; } .body { flex: none; overflow: visible; } }
    .status { margin: 0; font-size: var(--type-sm); color: var(--color-ink-2); }
    .care p { margin: 0; max-width: 65ch; font-size: var(--type-sm); color: var(--color-ink-2); }
    .care a { display: inline-flex; align-items: center; min-height: 44px; font-size: var(--type-sm); }
    .note { margin: 0; display: flex; align-items: flex-start; gap: var(--space-xs); padding: var(--space-sm); border-radius: var(--radius-control); border: 1px solid color-mix(in oklab, var(--color-caution) 50%, transparent); font-size: var(--type-sm); }
    .grid { margin: 0; padding: 0; list-style: none; display: grid; gap: var(--space-sm); grid-template-columns: repeat(2, minmax(0, 1fr)); }
    @media (min-width: 40rem) { .grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
    @media (min-width: 64rem) { .grid { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
    .ph { aspect-ratio: 3 / 4; border-radius: var(--radius-control); }
    .tile { display: flex; flex-direction: column; min-width: 0; overflow: hidden; border: 1px solid var(--color-rule); border-radius: var(--radius-control); background: var(--color-paper-2); }
    .tile.cur { border-color: var(--color-accent); box-shadow: inset 0 0 0 1px var(--color-accent); }
    .pic { aspect-ratio: 3 / 4; background: var(--color-paper-3); }
    .pic img, .pic video { display: block; width: 100%; height: 100%; object-fit: contain; }
    .pic p { margin: 0; display: flex; align-items: center; justify-content: center; height: 100%; padding: var(--space-sm); text-align: center; font-size: var(--type-sm); }
    .info { flex: 1; display: flex; flex-direction: column; gap: var(--space-2xs); padding: var(--space-sm); }
    .info .o { margin: 0; font-size: var(--type-xs); color: var(--color-ink-2); }
    .info a { display: inline-flex; align-items: center; min-height: 44px; font-size: var(--type-xs); overflow-wrap: anywhere; }
    .info .ui-btn { margin-top: auto; padding: 0 var(--space-sm); }
    .now { margin: auto 0 0; display: inline-flex; align-items: center; gap: var(--space-xs); min-height: 44px; font-size: var(--type-sm); font-weight: 600; color: var(--color-accent-text); }
    .empty p { margin: 0; }
    .empty p + p { margin-top: var(--space-3xs); font-size: var(--type-sm); color: var(--color-ink-2); }
    .empty .ui-btn, .more { margin-top: var(--space-sm); }
    .more { display: flex; justify-content: center; }
  `],
  template: `
    <dialog #dialog aria-labelledby="picker-title" (close)="onClosed()" class="hm">
      <div class="shell">
        <header class="head">
          <div class="top">
            <div style="min-width: 0">
              <h2 id="picker-title">Choose a {{ mediaType() === 'video' ? 'clip' : 'photo' }} for scene {{ sceneNumber() }}</h2>
              @if (scene(); as s) {
                <p class="sub">Suggested: {{ s.description }}</p>
              }
            </div>
            <button type="button" (click)="close()" aria-label="Close" class="ui-btn ui-btn--secondary x">
              <svg lucideIcon="x" [size]="18" aria-hidden="true"></svg>
            </button>
          </div>

          <form role="search" (submit)="onSubmit($event)">
            <div class="q">
              <label for="picker-query">Search in English</label>
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
                class="ui-field"
              />
              @if (queryError()) {
                <p id="picker-query-error" role="alert" class="err">
                  <svg lucideIcon="triangle-alert" [size]="16" aria-hidden="true" style="flex: none; margin-top: 2px"></svg>
                  {{ queryError() }}
                </p>
              }
            </div>
            <div class="ctl">
              <div role="group" aria-label="What to look for" class="seg">
                <button type="button" [attr.aria-pressed]="mediaType() === 'photo'" (click)="setType('photo')" class="ui-btn ui-btn--secondary ui-seg">
                  <svg lucideIcon="image" [size]="16" aria-hidden="true"></svg>
                  Photos
                </button>
                <button type="button" [attr.aria-pressed]="mediaType() === 'video'" (click)="setType('video')" class="ui-btn ui-btn--secondary ui-seg">
                  <svg lucideIcon="film" [size]="16" aria-hidden="true"></svg>
                  Clips
                </button>
              </div>
              <button type="submit" class="ui-btn ui-btn--primary go">
                <svg lucideIcon="search" [size]="16" aria-hidden="true"></svg>
                Search
              </button>
            </div>
          </form>
        </header>

        <div class="body">
          <p role="status" class="status" data-testid="picker-status">
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

          @if (phase() === 'ready') {
            <div class="care" data-testid="picker-care">
              <app-story-attribution [attributions]="attributions()" />
              <p>
                Check each picture before you choose it. Avoid visible logos or brand names, people who could be recognised shown in a bad
                light or as if they back something, and private homes.
              </p>
              <a href="/terms#stock" target="_blank" rel="noopener noreferrer" class="ui-link" data-testid="picker-terms"
                >What the terms say<span class="sr-only"> (opens in a new tab)</span></a
              >
            </div>
          }

          @if (problem(); as p) {
            <app-story-problem [problem]="p" [actionLabel]="p.retryable ? 'Search again' : ''" (action)="retry()" />
          }

          @if (notice(); as n) {
            <p class="note" role="status" data-testid="partial-notice">
              <svg lucideIcon="triangle-alert" [size]="16" aria-hidden="true" style="flex: none; margin-top: 2px"></svg>
              <span>{{ n.text }} Showing results from the other library only.</span>
            </p>
          }

          @if (phase() === 'loading') {
            <ul class="grid" aria-hidden="true">
              @for (i of placeholders; track i) {
                <li class="ui-skeleton ph"></li>
              }
            </ul>
          }

          @if (phase() === 'ready' && assets().length === 0 && !problem()) {
            <div class="ui-card empty" data-testid="empty">
              <p><strong>No {{ mediaType() === 'video' ? 'clips' : 'photos' }} found for “{{ searched() }}”</strong></p>
              <p>Try fewer or different English words, or look for {{ mediaType() === 'video' ? 'photos' : 'clips' }} instead.</p>
              <button type="button" (click)="setType(mediaType() === 'video' ? 'photo' : 'video')" class="ui-btn ui-btn--secondary">
                Search {{ mediaType() === 'video' ? 'photos' : 'clips' }} instead
              </button>
            </div>
          }

          @if (assets().length) {
            <ul class="grid" data-testid="results">
              @for (asset of assets(); track assetKey(asset)) {
                <li class="tile" [class.cur]="isCurrent(asset)">
                  <div class="pic">
                    @if (asset.type === 'video' && asset.playUrl) {
                      <video
                        [src]="asset.playUrl"
                        [poster]="asset.previewUrl ?? ''"
                        controls
                        muted
                        playsinline
                        preload="none"
                        [attr.aria-label]="'Preview of ' + altOf(asset)"
                      ></video>
                    } @else if (asset.previewUrl) {
                      <img [src]="asset.previewUrl" [alt]="altOf(asset)" loading="lazy" />
                    } @else {
                      <p>No preview available</p>
                    }
                  </div>
                  <div class="info">
                    <p class="o">
                      {{ orientationLabel(asset) }}@if (asset.duration) {
                        · {{ asset.duration | number: '1.0-0' }} s
                      }
                    </p>
                    <a [href]="asset.pageUrl" target="_blank" rel="noopener noreferrer" class="ui-link"
                      >{{ asset.credit }}<span class="sr-only"> (opens in a new tab)</span></a
                    >
                    @if (isCurrent(asset)) {
                      <p class="now">
                        <svg lucideIcon="check" [size]="16" aria-hidden="true"></svg>
                        Current choice
                      </p>
                    } @else {
                      <button type="button" (click)="choose(asset)" class="ui-btn ui-btn--secondary">
                        Use this {{ asset.type === 'video' ? 'clip' : 'photo' }}<span class="sr-only">, {{ altOf(asset) }}</span>
                      </button>
                    }
                  </div>
                </li>
              }
            </ul>

            @if (hasMore()) {
              <div class="more">
                <button type="button" (click)="loadMore()" [disabled]="loadingMore()" [attr.aria-busy]="loadingMore()" class="ui-btn ui-btn--secondary">
                  @if (loadingMore()) {
                    <svg lucideIcon="loader-2" [size]="16" aria-hidden="true" class="ui-spin"></svg>
                    Loading…
                  } @else {
                    Show more results
                  }
                </button>
              </div>
            }
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
