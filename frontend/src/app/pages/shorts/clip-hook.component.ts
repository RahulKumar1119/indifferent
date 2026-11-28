import { ChangeDetectionStrategy, Component, OnDestroy, OnInit, computed, effect, inject, input, output, signal } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { Subscription } from 'rxjs';
import { StoryPlanProblem } from '../story/story-plan-problem';
import { StoryProblemComponent } from '../story/story-problem.component';
import { langOf, newRequestId } from '../../shared/post-copy/post-copy.models';
import {
  ClipHook,
  ClipRenderView,
  HOOK_POSITIONS,
  HOOK_STYLES,
  HookView,
  MAX_HOOK_CHARS,
  MAX_HOOK_SECONDS,
  MIN_HOOK_SECONDS,
  defaultHook,
  hookProblem,
  hookTextLength,
  sameHook,
} from './clip-hook.models';
import { describeHookProblem } from './clip-hook-problem';
import { ClipHookService } from './clip-hook.service';

let nextId = 0;

/**
 * The hook line for one clip (#329): a short line of text burned into the first
 * seconds of the clip. The person writes it, or picks the ranking's line or one
 * of three suggestions written from the clip's own words, chooses a look, a
 * position and how long it shows, and saves it. "Render again" makes a new file
 * with the saved hook, trim and caption corrections; the clip that plays now
 * stays until the new one is ready.
 *
 * The draft is sent up as it changes, so the clip card can show an approximate
 * preview over the video.
 */
@Component({
  selector: 'app-clip-hook',
  standalone: true,
  imports: [LucideDynamicIcon, StoryProblemComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: [`
    :host { display: block; }
    h3 { margin: 0; }
    .intro, .note { margin: 0; font-size: var(--type-sm); color: var(--color-ink-2); }
    .note { font-size: var(--type-xs); color: var(--color-muted); }
    .stack { display: grid; gap: var(--space-sm); }
    .row { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-xs); }
    .count { margin: 0; font-size: var(--type-xs); color: var(--color-muted); font-variant-numeric: tabular-nums; }
    .count.over { color: var(--color-negative); font-weight: 600; }
    .why { margin: 0; font-size: var(--type-sm); color: var(--color-negative); }
    fieldset { margin: 0; padding: 0; border: 0; min-width: 0; }
    legend { padding: 0; font-size: var(--type-xs); font-weight: 600; color: var(--color-ink-2); }
    .opts { margin-top: var(--space-2xs); display: grid; gap: var(--space-xs); grid-template-columns: repeat(auto-fit, minmax(8.5rem, 1fr)); }
    .opt { display: flex; align-items: flex-start; gap: var(--space-xs); min-height: 44px; padding: var(--space-xs) var(--space-sm); border: 1px solid var(--color-muted); border-radius: var(--radius-control); cursor: pointer; }
    .opt:has(input:checked) { border-color: var(--color-accent); box-shadow: inset 0 0 0 1px var(--color-accent); }
    .opt:has(input:focus-visible) { outline: 2px solid var(--color-focus); outline-offset: 2px; }
    .opt input { margin: 3px 0 0; width: 1.125rem; height: 1.125rem; flex: none; accent-color: var(--color-accent); }
    .opt .name { display: block; font-weight: 600; font-size: var(--type-sm); }
    .opt .blurb { display: block; font-size: var(--type-xs); color: var(--color-muted); }
    .ideas { margin: 0; padding: 0; list-style: none; display: grid; gap: var(--space-2xs); }
    .ideas li { display: flex; align-items: center; gap: var(--space-xs); padding: var(--space-2xs) var(--space-xs); border: 1px solid var(--color-rule); border-radius: var(--radius-control); }
    .ideas .idea { flex: 1; min-width: 0; display: grid; gap: 2px; }
    .ideas .line { overflow-wrap: anywhere; font-size: var(--type-sm); }
    .ideas .src { font-size: var(--type-xs); color: var(--color-muted); }
    .render { padding-top: var(--space-sm); border-top: 1px solid var(--color-rule); display: grid; gap: var(--space-xs); }
    .state { margin: 0; display: flex; align-items: center; gap: var(--space-2xs); font-size: var(--type-sm); }
    .state.bad { color: var(--color-negative); }
    .state.ok { color: var(--color-positive); }
    input[lang='hi'], span[lang='hi'] { font-family: var(--font-body), 'Noto Sans Devanagari', sans-serif; }
  `],
  template: `
    <section class="stack" [attr.aria-labelledby]="uid + '-h'" data-testid="clip-hook">
      <h3 [id]="uid + '-h'" class="ui-card-title">Hook</h3>
      <p class="intro">A short line shown in large text over the first seconds of this clip.</p>

      @if (loadProblem(); as p) {
        <app-story-problem [problem]="p" actionLabel="Try again" (action)="load()" />
      } @else if (!view()) {
        <div class="ui-skeleton" style="height: 6rem" aria-label="Loading the hook"></div>
      } @else {
        <div class="row">
          <button type="button" role="switch" class="ui-switch" [attr.aria-checked]="draft().enabled" [attr.aria-labelledby]="uid + '-on'" (click)="patch({ enabled: !draft().enabled })" data-testid="hook-enabled"><span></span></button>
          <span [id]="uid + '-on'">Show a hook on this clip</span>
        </div>

        <div class="ui-f">
          <label class="ui-label" [for]="uid + '-text'">Hook text</label>
          <input type="text" class="ui-field" [id]="uid + '-text'" [value]="draft().text" [attr.lang]="lang()" [attr.aria-describedby]="uid + '-count'"
            (input)="onText($event)" data-testid="hook-text" />
          <p class="count" [class.over]="chars() > maxChars" [id]="uid + '-count'" aria-live="polite">{{ chars() }} / {{ maxChars }} characters</p>
        </div>

        @if (ranker() || view()!.suggestionsEnabled) {
          <div class="stack" style="gap: var(--space-xs)">
            @if (ideas().length > 0) {
              <ul class="ideas" aria-label="Hook ideas" data-testid="hook-ideas">
                @for (idea of ideas(); track idea.text) {
                  <li>
                    <span class="idea">
                      <span class="line" [attr.lang]="langFor(idea.text)">{{ idea.text }}</span>
                      <span class="src">{{ idea.from }}</span>
                    </span>
                    <button type="button" class="ui-btn ui-btn--quiet" (click)="use(idea.text)" [attr.aria-label]="'Use this hook: ' + idea.text">Use</button>
                  </li>
                }
              </ul>
            }
            @if (view()!.suggestionsEnabled) {
              <div class="row">
                <button type="button" class="ui-btn ui-btn--secondary" (click)="suggest()" [disabled]="suggesting()" data-testid="hook-suggest">
                  @if (suggesting()) {
                    <svg lucideIcon="loader-2" [size]="16" class="ui-spin" aria-hidden="true"></svg>Writing
                  } @else {
                    <svg lucideIcon="sparkles" [size]="16" aria-hidden="true"></svg>Suggest hooks
                  }
                </button>
                @if (suggestLeft() !== null) {
                  <span class="note">{{ suggestLeft() }} of {{ view()!.suggestDailyLimit }} left today, shared with titles and post copy</span>
                }
              </div>
              @if (suggestNote()) {
                <p class="note" role="status">{{ suggestNote() }}</p>
              }
            }
            @if (suggestProblem(); as p) {
              <app-story-problem [problem]="p" [polite]="true" [actionLabel]="p.retryable ? 'Try again' : ''" (action)="suggest()" />
            }
          </div>
        }

        <fieldset>
          <legend>Look</legend>
          <div class="opts" role="radiogroup" aria-label="Hook look">
            @for (s of styles; track s.id) {
              <label class="opt">
                <input type="radio" [name]="uid + '-style'" [value]="s.id" [checked]="draft().style === s.id" (change)="patch({ style: s.id })" />
                <span><span class="name">{{ s.label }}</span><span class="blurb">{{ s.blurb }}</span></span>
              </label>
            }
          </div>
        </fieldset>

        <fieldset>
          <legend>Position</legend>
          <div class="row" role="group" aria-label="Hook position" style="margin-top: var(--space-2xs)">
            @for (p of positions; track p.id) {
              <button type="button" class="ui-btn ui-btn--secondary ui-seg" [attr.aria-pressed]="draft().position === p.id" (click)="patch({ position: p.id })">{{ p.label }}</button>
            }
          </div>
        </fieldset>

        <label class="ui-f">
          <span class="ui-flabel">Shows for <span class="ui-val">{{ draft().seconds }} s</span></span>
          <input type="range" class="ui-range" [min]="minSeconds" [max]="maxSeconds" step="0.5" [value]="draft().seconds" (input)="onSeconds($event)" aria-label="How long the hook shows, in seconds" />
        </label>

        <p class="note">The preview on the clip is approximate. The render fits the text with the real fonts, and moves the hook if it would cover the captions.</p>

        @if (problem()) {
          <p class="why" role="alert">{{ problem() }}</p>
        }
        @if (saveProblem(); as p) {
          <app-story-problem [problem]="p" [polite]="true" />
        }
        <div class="row">
          <button type="button" class="ui-btn ui-btn--primary" (click)="save()" [disabled]="saving() || !dirty() || !!problem()" data-testid="hook-save">{{ saving() ? 'Saving' : 'Save hook' }}</button>
          @if (view()!.saved) {
            <button type="button" class="ui-btn ui-btn--quiet" (click)="remove()" [disabled]="saving()">Remove hook</button>
          }
          @if (savedNote()) {
            <span class="note" role="status">{{ savedNote() }}</span>
          }
        </div>

        @if (view()!.renderEnabled) {
          <div class="render" data-testid="hook-render">
            @switch (renderState()?.state) {
              @case ('rendering') {
                <p class="state" role="status"><svg lucideIcon="loader-2" [size]="16" class="ui-spin" aria-hidden="true"></svg>Rendering this clip again. It usually takes a few minutes, and you can leave this page.</p>
              }
              @case ('failed') {
                <p class="state bad" role="alert"><svg lucideIcon="circle-x" [size]="16" aria-hidden="true"></svg>{{ renderState()?.error || 'The render did not finish. The clip you had is unchanged.' }}</p>
              }
              @case ('done') {
                <p class="state ok" role="status"><svg lucideIcon="check" [size]="16" aria-hidden="true"></svg>Rendered again. The clip above is the new one.</p>
              }
            }
            <div class="row">
              <button type="button" class="ui-btn ui-btn--secondary" (click)="renderAgain()" [disabled]="!canRender()" data-testid="hook-render-again">
                <svg lucideIcon="refresh-cw" [size]="16" aria-hidden="true"></svg>{{ rendering() ? 'Starting' : 'Render this clip again' }}
              </button>
              @if (dirty()) {
                <span class="note">Save the hook first.</span>
              }
            </div>
            <p class="note">Uses the saved hook, trim and caption corrections. The clip you have stays until the new one is ready.
              @if (renderLeft() !== null) {
                {{ renderLeft() }} of {{ view()!.renderDailyLimit }} left today, shared with retries.
              }
            </p>
            @if (renderProblem(); as p) {
              <app-story-problem [problem]="p" [polite]="true" [actionLabel]="p.retryable ? 'Try again' : ''" (action)="renderAgain()" />
            }
          </div>
        }
      }
    </section>
  `,
})
export class ClipHookComponent implements OnInit, OnDestroy {
  readonly jobId = input.required<string>();
  readonly clipId = input.required<string>();
  /** The latest single-clip render, kept up to date by the page. */
  readonly render = input<ClipRenderView | null>(null);
  /** The draft as it changes, for the preview over the clip. */
  readonly draftChange = output<ClipHook>();
  /** A render was accepted; the page follows it. */
  readonly renderStarted = output<ClipRenderView>();

  private readonly service = inject(ClipHookService);
  private readonly subs = new Subscription();

  readonly uid = `clip-hook-${++nextId}`;
  readonly maxChars = MAX_HOOK_CHARS;
  readonly minSeconds = MIN_HOOK_SECONDS;
  readonly maxSeconds = MAX_HOOK_SECONDS;
  readonly styles = HOOK_STYLES;
  readonly positions = HOOK_POSITIONS;

  readonly view = signal<HookView | null>(null);
  readonly loadProblem = signal<StoryPlanProblem | null>(null);
  readonly draft = signal<ClipHook>(defaultHook());
  private readonly savedHook = signal<ClipHook>(defaultHook());
  readonly saving = signal(false);
  readonly saveProblem = signal<StoryPlanProblem | null>(null);
  readonly savedNote = signal('');
  readonly suggesting = signal(false);
  readonly suggestions = signal<string[]>([]);
  readonly suggestProblem = signal<StoryPlanProblem | null>(null);
  readonly suggestNote = signal('');
  private readonly suggestRemaining = signal<number | null>(null);
  readonly rendering = signal(false);
  readonly renderProblem = signal<StoryPlanProblem | null>(null);
  private readonly renderRemaining = signal<number | null>(null);
  private readonly ownRender = signal<ClipRenderView | null>(null);
  /** Kept across a retryable failure, so a retry cannot be charged twice. */
  private suggestId: string | null = null;
  private renderId: string | null = null;

  readonly chars = computed(() => hookTextLength(this.draft().text.trim()));
  readonly lang = computed(() => langOf(this.draft().text));
  readonly problem = computed(() => hookProblem(this.draft()));
  readonly dirty = computed(() => !sameHook(this.draft(), this.savedHook()));
  readonly ranker = computed(() => this.view()?.ranker ?? '');
  readonly ideas = computed(() => {
    const out: { text: string; from: string }[] = [];
    const r = this.ranker();
    if (r) out.push({ text: r, from: 'From the ranking' });
    for (const s of this.suggestions()) out.push({ text: s, from: 'Suggested' });
    return out;
  });
  readonly suggestLeft = computed(() => this.suggestRemaining() ?? this.view()?.suggestRemainingToday ?? null);
  readonly renderLeft = computed(() => this.renderRemaining() ?? this.view()?.renderRemainingToday ?? null);
  /** The page's view wins once it has one; until then, what this panel last learned. */
  readonly renderState = computed(() => this.render() ?? this.ownRender() ?? this.view()?.render ?? null);
  readonly canRender = computed(
    () => !this.rendering() && !this.dirty() && !this.saving() && this.renderState()?.state !== 'rendering',
  );

  constructor() {
    effect(() => this.draftChange.emit(this.draft()));
  }

  ngOnInit(): void {
    this.load();
  }

  ngOnDestroy(): void {
    this.subs.unsubscribe();
  }

  langFor(text: string): string | null {
    return langOf(text);
  }

  load(): void {
    this.loadProblem.set(null);
    this.subs.add(
      this.service.view(this.jobId(), this.clipId()).subscribe({
        next: (v) => {
          this.view.set(v);
          this.savedHook.set({ ...v.hook });
          this.draft.set({ ...v.hook });
        },
        error: (err) => this.loadProblem.set(describeHookProblem(err, 'load')),
      }),
    );
  }

  patch(p: Partial<ClipHook>): void {
    this.savedNote.set('');
    this.draft.update((d) => ({ ...d, ...p }));
  }

  onText(e: Event): void {
    this.patch({ text: (e.target as HTMLInputElement).value });
  }

  onSeconds(e: Event): void {
    const v = Number((e.target as HTMLInputElement).value);
    if (Number.isFinite(v)) this.patch({ seconds: v });
  }

  /** Puts a line in the box and switches the hook on. Nothing is saved yet. */
  use(text: string): void {
    this.patch({ text, enabled: true });
  }

  save(): void {
    if (this.saving() || !this.dirty() || this.problem()) return;
    this.saving.set(true);
    this.saveProblem.set(null);
    this.subs.add(
      this.service.save(this.jobId(), this.clipId(), this.draft()).subscribe({
        next: ({ hook }) => {
          this.saving.set(false);
          this.savedHook.set({ ...hook });
          this.draft.set({ ...hook });
          this.view.update((v) => (v ? { ...v, saved: true, hook } : v));
          this.savedNote.set('Saved. Render the clip again to see it in the video.');
        },
        error: (err) => {
          this.saving.set(false);
          this.saveProblem.set(describeHookProblem(err, 'save'));
        },
      }),
    );
  }

  remove(): void {
    if (this.saving()) return;
    this.saving.set(true);
    this.saveProblem.set(null);
    this.subs.add(
      this.service.reset(this.jobId(), this.clipId()).subscribe({
        next: () => {
          this.saving.set(false);
          const blank = defaultHook();
          this.savedHook.set(blank);
          this.draft.set({ ...blank });
          this.view.update((v) => (v ? { ...v, saved: false, hook: blank } : v));
          this.savedNote.set('Hook removed. Render the clip again to take it out of the video.');
        },
        error: (err) => {
          this.saving.set(false);
          this.saveProblem.set(describeHookProblem(err, 'save'));
        },
      }),
    );
  }

  suggest(): void {
    if (this.suggesting()) return;
    this.suggesting.set(true);
    this.suggestProblem.set(null);
    this.suggestNote.set('');
    const id = (this.suggestId ??= newRequestId());
    this.subs.add(
      this.service.suggest(this.jobId(), this.clipId(), id).subscribe({
        next: (res) => {
          this.suggesting.set(false);
          this.suggestId = null;
          this.suggestions.set(res.suggestions ?? []);
          if (typeof res.remainingToday === 'number') this.suggestRemaining.set(res.remainingToday);
          if ((res.suggestions ?? []).length === 0) {
            this.suggestNote.set('No new lines passed the checks. Use the ranking line or type your own.');
          } else if (res.dropped > 0) {
            this.suggestNote.set(`${res.dropped} more ${res.dropped === 1 ? 'line was' : 'lines were'} left out because ${res.dropped === 1 ? 'it' : 'they'} said something the clip does not, or would not fit.`);
          }
        },
        error: (err) => {
          this.suggesting.set(false);
          const p = describeHookProblem(err, 'suggest');
          if (!p.retryable) this.suggestId = null;
          this.suggestProblem.set(p);
        },
      }),
    );
  }

  renderAgain(): void {
    if (!this.canRender()) return;
    this.rendering.set(true);
    this.renderProblem.set(null);
    const id = (this.renderId ??= newRequestId());
    this.subs.add(
      this.service.render(this.jobId(), this.clipId(), id).subscribe({
        next: (res) => {
          this.rendering.set(false);
          this.renderId = null;
          this.ownRender.set(res.render);
          if (typeof res.remainingToday === 'number') this.renderRemaining.set(res.remainingToday);
          this.renderStarted.emit(res.render);
        },
        error: (err) => {
          this.rendering.set(false);
          const p = describeHookProblem(err, 'render');
          if (!p.retryable) this.renderId = null;
          this.renderProblem.set(p);
        },
      }),
    );
  }
}
