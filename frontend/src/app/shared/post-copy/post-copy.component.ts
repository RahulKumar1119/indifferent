import { DOCUMENT } from '@angular/common';
import { HttpErrorResponse } from '@angular/common/http';
import { ChangeDetectionStrategy, Component, Injector, OnDestroy, OnInit, afterNextRender, computed, inject, input, output, signal } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { Subscription } from 'rxjs';
import { StoryPlanProblem } from '../../pages/story/story-plan-problem';
import { StoryProblemComponent } from '../../pages/story/story-problem.component';
import { copyText } from '../clipboard';
import {
  CopyMap,
  PlatformCopy,
  PlatformProblems,
  PlatformSpec,
  PostCopyResult,
  PostCopySaved,
  PostCopyTarget,
  PostCopyView,
  countUnits,
  fullText,
  hasProblems,
  langOf,
  newRequestId,
  parseHashtags,
  platformProblems,
  unitWord,
  withCommas,
} from './post-copy.models';
import { describePostCopyProblem } from './post-copy-problem';
import { PostCopyService } from './post-copy.service';

/** What a person is editing for one platform: the hashtags are one box of text. */
interface Draft {
  title: string;
  text: string;
  tags: string;
}
type Drafts = Record<string, Draft>;

/** How long a "Copied" mark stays on a button. */
const COPIED_MS = 2000;
/** A try that is still being written is asked about again this often, this many times. */
const WAIT_MS = 2000;
const MAX_WAITS = 8;

let nextId = 0;

/** Tabs, carriage returns and other control characters cannot be saved; they become a space or go. */
function clean(s: string): string {
  // eslint-disable-next-line no-control-regex
  return s.replace(/\r\n?/g, '\n').replace(/\t/g, ' ').replace(/[\u0000-\u0009\u000b-\u001f\u007f]/g, '');
}

function toDrafts(specs: PlatformSpec[], copy: CopyMap): Drafts {
  const out: Drafts = {};
  for (const sp of specs) {
    const c = copy[sp.id];
    out[sp.id] = { title: c?.title ?? '', text: c?.text ?? '', tags: (c?.hashtags ?? []).join(' ') };
  }
  return out;
}

function blankDrafts(specs: PlatformSpec[]): Drafts {
  return toDrafts(specs, {});
}

function toCopy(sp: PlatformSpec, d: Draft): { copy: PlatformCopy; bad: string[] } {
  const { tags, bad } = parseHashtags(d.tags);
  const copy: PlatformCopy = { text: d.text, hashtags: tags };
  if (sp.hasTitle) copy.title = d.title;
  return { copy, bad };
}

/**
 * The copy for each platform for one clip or one story video (#330): a YouTube
 * title and description, a TikTok caption and an Instagram Reels caption, each
 * with hashtags. It is written from what the clip or video says. The person can
 * edit every part, sees a live count in the platform's own unit, and cannot copy
 * or save a part that is over its limit. Nothing is posted anywhere.
 *
 * The server writes, counts and stores; this component keeps the same rules so a
 * problem shows as it is typed. A retry of a write sends the same request id, so
 * a dropped connection can never use up two of the day's tries.
 */
@Component({
  selector: 'app-post-copy',
  standalone: true,
  imports: [LucideDynamicIcon, StoryProblemComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: [`
    :host { display: block; }
    h3:focus { outline: none; }
    h3:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 4px; border-radius: 2px; }
    .intro { margin: 0 0 var(--space-sm); font-size: var(--type-sm); color: var(--color-ink-2); }
    .top { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-sm); }
    .left { margin: 0; font-size: var(--type-sm); color: var(--color-ink-2); }
    .plats { margin-top: var(--space-md); display: grid; gap: var(--space-md); }
    .plat { padding: var(--space-md); border: 1px solid var(--color-rule); border-radius: var(--radius-control); display: grid; gap: var(--space-sm); min-width: 0; }
    .plat h4 { margin: 0; font-size: var(--type-md); font-weight: 600; }
    .field { display: grid; gap: var(--space-3xs); min-width: 0; }
    textarea { min-height: 5rem; resize: vertical; line-height: 1.5; }
    textarea.tags { min-height: 3.25rem; }
    .count { margin: 0; font-size: var(--type-xs); color: var(--color-muted); font-variant-numeric: tabular-nums; }
    .count.over { color: var(--color-negative); font-weight: 600; }
    .why { margin: 0; display: flex; gap: var(--space-2xs); align-items: flex-start; font-size: var(--type-sm); color: var(--color-negative); }
    .why svg { flex: none; margin-top: 2px; }
    .acts { display: flex; flex-wrap: wrap; gap: var(--space-xs); }
    .bar { margin-top: var(--space-md); display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-sm); }
    .state { margin: 0; font-size: var(--type-sm); color: var(--color-ink-2); }
    .notes { margin: var(--space-sm) 0 0; padding: 0; list-style: none; display: grid; gap: var(--space-2xs); font-size: var(--type-sm); color: var(--color-ink-2); }
    .confirm { margin-top: var(--space-sm); padding: var(--space-md); border: 1px solid var(--color-rule); border-radius: var(--radius-control); background: var(--color-paper-3); }
    .confirm p { margin: 0 0 var(--space-sm); }
    .sr { position: absolute; width: 1px; height: 1px; margin: -1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
    .none { margin: var(--space-md) 0 0; display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-sm); }
    textarea[lang='hi'], input[lang='hi'] { font-family: var(--font-body), 'Noto Sans Devanagari', sans-serif; }
  `],
  template: `
    @if (view(); as v) {
      @if (v.enabled) {
        <section [attr.aria-labelledby]="uid + '-h'" data-testid="post-copy">
          <h3 [id]="uid + '-h'" tabindex="-1" class="ui-card-title">Post copy</h3>
          <p class="intro">
            Title, captions and hashtags for YouTube, TikTok and Instagram Reels, written from what {{ what() }} says. Read them before you post:
            they can still get things wrong. Nothing is posted for you.
          </p>
          <div class="top">
            <button type="button" class="ui-btn ui-btn--primary" data-testid="pc-write" [disabled]="busy() !== null" [attr.aria-busy]="busy() === 'write'" (click)="write()">
              @if (busy() === 'write') {
                <svg lucideIcon="loader-2" [size]="16" class="ui-spin" aria-hidden="true"></svg>
                Writing
              } @else {
                <svg lucideIcon="sparkles" [size]="16" aria-hidden="true"></svg>
                {{ drafts() ? 'Write again' : 'Write for all three platforms' }}
              }
            </button>
            @if (leftNote(); as note) {
              <p class="left" data-testid="pc-left">{{ note }}</p>
            }
          </div>
          @if (confirming()) {
            <div class="confirm" role="group" [attr.aria-labelledby]="uid + '-confirm'" data-testid="pc-confirm" (keydown.escape)="confirming.set(false)">
              <p [id]="uid + '-confirm'"><b>Write again?</b> This replaces everything on this page{{ dirty() ? ', including changes you have not saved' : '' }}.</p>
              <div class="acts">
                <button type="button" class="ui-btn ui-btn--primary" data-testid="pc-confirm-yes" (click)="confirmWrite()">Replace it</button>
                <button type="button" class="ui-btn ui-btn--secondary" data-testid="pc-confirm-no" (click)="confirming.set(false)">Keep what I have</button>
              </div>
            </div>
          }
          @if (problem(); as p) {
            <div style="margin-top: var(--space-sm)">
              <app-story-problem [problem]="p" [actionLabel]="p.retryable ? 'Try again' : ''" (action)="retry()" />
            </div>
          }
          @if (drafts(); as d) {
            @if (savedNotes().length > 0) {
              <ul class="notes" data-testid="pc-notes" aria-label="What was changed or left out">
                @for (n of savedNotes(); track $index) {
                  <li>{{ n }}</li>
                }
              </ul>
            }
            <div class="plats">
              @for (sp of v.platforms; track sp.id) {
                <section class="plat" [attr.aria-labelledby]="uid + '-' + sp.id" [attr.data-testid]="'pc-' + sp.id">
                  <h4 [id]="uid + '-' + sp.id">{{ sp.label }}</h4>
                  @if (sp.hasTitle) {
                    <div class="field">
                      <label class="ui-label" [for]="uid + '-' + sp.id + '-title'">Title</label>
                      <input
                        type="text" class="ui-field" [id]="uid + '-' + sp.id + '-title'" [value]="d[sp.id].title" [attr.lang]="lang(d[sp.id].title)"
                        [attr.aria-invalid]="problems()[sp.id].title ? 'true' : null" [attr.aria-describedby]="uid + '-' + sp.id + '-title-c'"
                        [attr.data-testid]="'pc-' + sp.id + '-title'" (input)="edit(sp.id, 'title', $event)"
                      />
                      <p class="count" [class.over]="!!problems()[sp.id].title" [id]="uid + '-' + sp.id + '-title-c'" [attr.data-testid]="'pc-' + sp.id + '-title-count'">
                        {{ titleCount(sp, d[sp.id]) }} of {{ commas(sp.titleMax ?? 0) }} {{ unit(sp.titleUnit ?? 'characters') }}
                      </p>
                      @if (problems()[sp.id].title; as why) {
                        <p class="why" [attr.data-testid]="'pc-' + sp.id + '-title-why'"><svg lucideIcon="triangle-alert" [size]="16" aria-hidden="true"></svg>{{ why }}</p>
                      }
                    </div>
                  }
                  <div class="field">
                    <label class="ui-label" [for]="uid + '-' + sp.id + '-text'">{{ sp.textLabel }}</label>
                    <textarea
                      class="ui-field" [id]="uid + '-' + sp.id + '-text'" [rows]="sp.hasTitle ? 7 : 4" [value]="d[sp.id].text" [attr.lang]="lang(d[sp.id].text)"
                      [attr.aria-invalid]="problems()[sp.id].text ? 'true' : null" [attr.aria-describedby]="uid + '-' + sp.id + '-text-c'"
                      [attr.data-testid]="'pc-' + sp.id + '-text'" (input)="edit(sp.id, 'text', $event)"
                    ></textarea>
                    <p class="count" [class.over]="!!problems()[sp.id].text" [id]="uid + '-' + sp.id + '-text-c'" [attr.data-testid]="'pc-' + sp.id + '-text-count'">
                      {{ textCount(sp, d[sp.id]) }} of {{ commas(sp.textMax) }} {{ unit(sp.textUnit) }}, hashtags included
                    </p>
                    @if (problems()[sp.id].text; as why) {
                      <p class="why" [attr.data-testid]="'pc-' + sp.id + '-text-why'"><svg lucideIcon="triangle-alert" [size]="16" aria-hidden="true"></svg>{{ why }}</p>
                    }
                  </div>
                  <div class="field">
                    <label class="ui-label" [for]="uid + '-' + sp.id + '-tags'">Hashtags</label>
                    <textarea
                      class="ui-field tags" rows="2" [id]="uid + '-' + sp.id + '-tags'" [value]="d[sp.id].tags" [attr.lang]="lang(d[sp.id].tags)" autocapitalize="off" spellcheck="false"
                      [attr.aria-invalid]="problems()[sp.id].hashtags ? 'true' : null" [attr.aria-describedby]="uid + '-' + sp.id + '-tags-c'"
                      [attr.data-testid]="'pc-' + sp.id + '-tags'" (input)="edit(sp.id, 'tags', $event)" (keydown.enter)="$event.preventDefault()" (blur)="tidyTags(sp.id)"
                    ></textarea>
                    <p class="count" [id]="uid + '-' + sp.id + '-tags-c'" [attr.data-testid]="'pc-' + sp.id + '-tags-count'">
                      {{ tagCount(d[sp.id]) }} of {{ sp.hashtagsMax }} hashtags. Separate them with spaces.
                    </p>
                    @if (problems()[sp.id].hashtags; as why) {
                      <p class="why" [attr.data-testid]="'pc-' + sp.id + '-tags-why'"><svg lucideIcon="triangle-alert" [size]="16" aria-hidden="true"></svg>{{ why }}</p>
                    }
                  </div>
                  <div class="acts">
                    @if (sp.hasTitle) {
                      <button type="button" class="ui-btn ui-btn--secondary" [attr.data-testid]="'pc-' + sp.id + '-copy-title'" [disabled]="!!problems()[sp.id].title" (click)="copy(sp, 'title')">
                        <svg [lucideIcon]="copied() === sp.id + '-title' ? 'check' : 'copy'" [size]="16" aria-hidden="true"></svg>
                        {{ copied() === sp.id + '-title' ? 'Copied' : 'Copy title' }}<span class="sr"> for {{ sp.label }}</span>
                      </button>
                    }
                    <button type="button" class="ui-btn ui-btn--secondary" [attr.data-testid]="'pc-' + sp.id + '-copy-text'" [disabled]="!!problems()[sp.id].text || !!problems()[sp.id].hashtags" (click)="copy(sp, 'text')">
                      <svg [lucideIcon]="copied() === sp.id + '-text' ? 'check' : 'copy'" [size]="16" aria-hidden="true"></svg>
                      {{ copied() === sp.id + '-text' ? 'Copied' : 'Copy ' + sp.textLabel.toLowerCase() }}<span class="sr"> and hashtags for {{ sp.label }}</span>
                    </button>
                    <button type="button" class="ui-btn ui-btn--secondary" [attr.data-testid]="'pc-' + sp.id + '-copy-tags'" [disabled]="!!problems()[sp.id].hashtags || tagCount(d[sp.id]) === 0" (click)="copy(sp, 'tags')">
                      <svg [lucideIcon]="copied() === sp.id + '-tags' ? 'check' : 'copy'" [size]="16" aria-hidden="true"></svg>
                      {{ copied() === sp.id + '-tags' ? 'Copied' : 'Copy hashtags' }}<span class="sr"> for {{ sp.label }}</span>
                    </button>
                  </div>
                </section>
              }
            </div>
            <div class="bar">
              <button type="button" class="ui-btn ui-btn--primary" data-testid="pc-save" [disabled]="!dirty() || anyProblem() || busy() !== null" [attr.aria-busy]="busy() === 'save'" (click)="save()">
                @if (busy() === 'save') {
                  <svg lucideIcon="loader-2" [size]="16" class="ui-spin" aria-hidden="true"></svg>
                  Saving
                } @else {
                  Save changes
                }
              </button>
              @if (dirty()) {
                <button type="button" class="ui-btn ui-btn--secondary" data-testid="pc-undo" [disabled]="busy() !== null" (click)="undo()">Undo my changes</button>
              }
              <p class="state" data-testid="pc-state">{{ stateText() }}</p>
            </div>
          } @else if (!busy()) {
            <p class="none">
              <button type="button" class="ui-btn ui-btn--secondary" data-testid="pc-blank" (click)="startBlank()">Write it myself</button>
              <span class="left">Or start with empty boxes and the same limits.</span>
            </p>
          }
          <p class="sr" role="status" data-testid="pc-status">{{ announcement() }}</p>
        </section>
      }
    } @else if (loadProblem(); as p) {
      <app-story-problem [problem]="p" [polite]="true" [actionLabel]="p.retryable ? 'Try again' : ''" (action)="load()" />
    } @else {
      <p class="left" role="status" data-testid="pc-loading">Loading</p>
    }
  `,
})
export class PostCopyComponent implements OnInit, OnDestroy {
  /** What the copy is for. */
  readonly target = input.required<PostCopyTarget>();
  /** Told after the copy is written or saved, so a page can use the newest copy. */
  readonly savedChange = output<PostCopySaved>();

  private readonly service = inject(PostCopyService);
  private readonly document = inject(DOCUMENT);
  private readonly injector = inject(Injector);

  protected readonly uid = `pc${++nextId}`;
  protected readonly view = signal<PostCopyView | null>(null);
  protected readonly loadProblem = signal<StoryPlanProblem | null>(null);
  protected readonly drafts = signal<Drafts | null>(null);
  /** The drafts as last saved or written, as text, to tell what has changed. */
  private readonly baseline = signal('');
  protected readonly savedNotes = signal<string[]>([]);
  protected readonly source = signal<'generated' | 'edited' | null>(null);
  protected readonly busy = signal<'write' | 'save' | null>(null);
  protected readonly problem = signal<StoryPlanProblem | null>(null);
  protected readonly confirming = signal(false);
  protected readonly copied = signal('');
  protected readonly announcement = signal('');
  protected readonly justSaved = signal(false);
  private readonly remaining = signal<number | null>(null);

  /** The id of a write that has not finished, so a retry cannot spend a second try. */
  private pendingId: string | null = null;
  private waits = 0;
  /** Which action the problem on screen came from, so Try again repeats that one. */
  private problemFrom: 'write' | 'save' = 'write';
  private subs: Subscription[] = [];
  private copiedTimer: ReturnType<typeof setTimeout> | null = null;
  private waitTimer: ReturnType<typeof setTimeout> | null = null;

  protected readonly specs = computed(() => this.view()?.platforms ?? []);

  /** Per platform: what is wrong, in words. Parts that are fine have no entry. */
  protected readonly problems = computed<Record<string, PlatformProblems>>(() => {
    const d = this.drafts();
    const out: Record<string, PlatformProblems> = {};
    for (const sp of this.specs()) {
      if (!d) {
        out[sp.id] = {};
        continue;
      }
      const { copy, bad } = toCopy(sp, d[sp.id]);
      out[sp.id] = platformProblems(sp, copy, bad);
    }
    return out;
  });

  protected readonly anyProblem = computed(() => Object.values(this.problems()).some(hasProblems));
  protected readonly dirty = computed(() => {
    const d = this.drafts();
    return d !== null && JSON.stringify(d) !== this.baseline();
  });
  protected readonly leftNote = computed(() => {
    const v = this.view();
    const left = this.remaining();
    if (!v || left === null || !v.dailyLimit) return '';
    return `${left} of ${v.dailyLimit} tries left today. Writing again uses one.`;
  });
  protected readonly stateText = computed(() => {
    if (this.dirty()) return 'You have changes that are not saved.';
    if (this.justSaved()) return 'Saved. This is what the clip keeps.';
    const s = this.source();
    if (s === 'edited') return 'Saved with your changes.';
    if (s === 'generated') return 'Saved as it was written. It is kept with this ' + (this.target().kind === 'clip' ? 'clip.' : 'video.');
    return '';
  });

  protected what(): string {
    return this.target().kind === 'clip' ? 'this clip' : 'this story';
  }

  ngOnInit(): void {
    this.load();
  }

  ngOnDestroy(): void {
    this.subs.forEach((s) => s.unsubscribe());
    if (this.copiedTimer !== null) clearTimeout(this.copiedTimer);
    if (this.waitTimer !== null) clearTimeout(this.waitTimer);
  }

  protected load(): void {
    this.loadProblem.set(null);
    this.subs.push(
      this.service.view(this.target()).subscribe({
        next: (v) => {
          this.view.set(v);
          this.remaining.set(typeof v.remainingToday === 'number' ? v.remainingToday : null);
          if (v.enabled && v.saved) this.apply(v.saved, false);
        },
        error: (err) => this.loadProblem.set(describePostCopyProblem(err, 'load')),
      }),
    );
  }

  // --- helpers for the template -------------------------------------------

  protected commas = withCommas;
  protected unit = unitWord;
  protected lang = langOf;

  protected titleCount(sp: PlatformSpec, d: Draft): number {
    return countUnits(d.title, sp.titleUnit ?? 'characters');
  }

  protected textCount(sp: PlatformSpec, d: Draft): number {
    return countUnits(fullText(toCopy(sp, d).copy), sp.textUnit);
  }

  protected tagCount(d: Draft): number {
    return parseHashtags(d.tags).tags.length;
  }

  // --- editing --------------------------------------------------------------

  protected edit(id: string, field: keyof Draft, event: Event): void {
    const el = event.target as HTMLInputElement | HTMLTextAreaElement;
    const cleaned = field === 'title' || field === 'tags' ? clean(el.value).replace(/\n/g, ' ') : clean(el.value);
    if (cleaned !== el.value) el.value = cleaned;
    const before = this.problems();
    this.drafts.update((d) => (d ? { ...d, [id]: { ...d[id], [field]: cleaned } } : d));
    this.justSaved.set(false);
    this.announceChange(id, field, before);
  }

  /** Says when a part crosses its limit or comes back under it, not on every key. */
  private announceChange(id: string, field: keyof Draft, before: Record<string, PlatformProblems>): void {
    const key = field === 'tags' ? 'hashtags' : field;
    const was = !!before[id]?.[key];
    const now = this.problems()[id]?.[key];
    const sp = this.specs().find((s) => s.id === id);
    if (!sp) return;
    if (!was && now) this.announcement.set(`${sp.label}: ${now}`);
    else if (was && !now) this.announcement.set(`${sp.label}: that part is within its limit.`);
  }

  /** Rewrites the hashtag box to the clean form when every piece is a hashtag. */
  protected tidyTags(id: string): void {
    const d = this.drafts();
    if (!d) return;
    const { tags, bad } = parseHashtags(d[id].tags);
    const tidy = tags.join(' ');
    if (bad.length === 0 && tidy !== d[id].tags) {
      this.drafts.update((x) => (x ? { ...x, [id]: { ...x[id], tags: tidy } } : x));
    }
  }

  protected undo(): void {
    const base = this.baseline();
    if (!base) {
      this.drafts.set(null);
      return;
    }
    this.drafts.set(JSON.parse(base) as Drafts);
    this.problem.set(null);
    this.announcement.set('Your changes were undone.');
  }

  protected startBlank(): void {
    this.drafts.set(blankDrafts(this.specs()));
    this.baseline.set('');
    this.source.set(null);
    this.savedNotes.set([]);
    this.problem.set(null);
    this.announcement.set('Empty boxes are ready.');
  }

  // --- writing --------------------------------------------------------------

  /** Asks first when there is something here that writing would replace. */
  protected write(): void {
    if (this.busy() !== null) return;
    if (this.drafts() !== null && (this.dirty() || this.source() === 'edited')) {
      this.confirming.set(true);
      afterNextRender(() => this.document.querySelector<HTMLElement>('[data-testid="pc-confirm-yes"]')?.focus(), { injector: this.injector });
      return;
    }
    this.run();
  }

  protected confirmWrite(): void {
    this.confirming.set(false);
    this.run();
  }

  protected retry(): void {
    if (!this.problem()) return;
    if (this.problemFrom === 'save') this.save();
    else this.run();
  }

  private run(): void {
    this.busy.set('write');
    this.problem.set(null);
    this.confirming.set(false);
    this.announcement.set('Writing for the three platforms.');
    // A try that did not finish keeps its id, so trying again is the same request.
    const id = (this.pendingId ??= newRequestId());
    this.subs.push(
      this.service.write(this.target(), id).subscribe({
        next: (res) => this.written(res),
        error: (err) => this.writeFailed(err),
      }),
    );
  }

  private written(res: PostCopyResult): void {
    this.pendingId = null;
    this.waits = 0;
    this.busy.set(null);
    if (typeof res.remainingToday === 'number') this.remaining.set(res.remainingToday);
    this.apply(res, true);
    this.announcement.set(res.replayed ? 'That copy was already written. Nothing more was used.' : 'The copy for the three platforms is ready.');
    afterNextRender(() => this.document.getElementById(this.uid + '-h')?.focus(), { injector: this.injector });
  }

  private writeFailed(err: unknown): void {
    const http = err instanceof HttpErrorResponse ? err : null;
    const inProgress = http?.status === 409 && (http.error as { code?: string } | null)?.code === 'IN_PROGRESS';
    if (inProgress && this.waits < MAX_WAITS) {
      // The first try is still being written. Ask again with the same id.
      this.waits++;
      this.waitTimer = setTimeout(() => this.run(), WAIT_MS);
      return;
    }
    this.waits = 0;
    this.busy.set(null);
    const p = describePostCopyProblem(err, 'write');
    this.problemFrom = 'write';
    this.problem.set(p);
    this.announcement.set('');
    if (p.kind === 'rate_limited') this.remaining.set(0);
    // A request the server answered for good is finished with. One that may not
    // have reached it, or may still be running, keeps its id.
    const keep = !http || http.status === 0 || http.status === 504 || http.status === 408 || inProgress;
    if (!keep) this.pendingId = null;
  }

  // --- saving ---------------------------------------------------------------

  protected save(): void {
    const d = this.drafts();
    if (!d || !this.dirty() || this.anyProblem() || this.busy() !== null) return;
    const copy: CopyMap = {};
    for (const sp of this.specs()) copy[sp.id] = toCopy(sp, d[sp.id]).copy;
    this.busy.set('save');
    this.problem.set(null);
    this.subs.push(
      this.service.save(this.target(), copy).subscribe({
        next: (saved) => {
          this.busy.set(null);
          this.apply(saved, true);
          this.justSaved.set(true);
          this.announcement.set('Saved.');
        },
        error: (err) => {
          this.busy.set(null);
          this.problemFrom = 'save';
          this.problem.set(describePostCopyProblem(err, 'save'));
        },
      }),
    );
  }

  private apply(saved: PostCopySaved, emit: boolean): void {
    const drafts = toDrafts(this.specs(), saved.copy);
    this.drafts.set(drafts);
    this.baseline.set(JSON.stringify(drafts));
    this.savedNotes.set(saved.source === 'generated' ? saved.notes : []);
    this.source.set(saved.source);
    this.justSaved.set(false);
    this.copied.set('');
    if (emit) this.savedChange.emit(saved);
  }

  // --- copying --------------------------------------------------------------

  protected async copy(sp: PlatformSpec, part: 'title' | 'text' | 'tags'): Promise<void> {
    const d = this.drafts();
    if (!d) return;
    const { copy } = toCopy(sp, d[sp.id]);
    const text = part === 'title' ? (copy.title ?? '') : part === 'tags' ? copy.hashtags.join(' ') : fullText(copy);
    const label = part === 'title' ? `${sp.label} title` : part === 'tags' ? `${sp.label} hashtags` : `${sp.label} ${sp.textLabel.toLowerCase()}`;
    const ok = await copyText(text, this.document);
    if (this.copiedTimer !== null) clearTimeout(this.copiedTimer);
    if (ok) {
      this.copied.set(`${sp.id}-${part}`);
      this.announcement.set(`${label} copied.`);
      this.copiedTimer = setTimeout(() => this.copied.set(''), COPIED_MS);
    } else {
      this.copied.set('');
      this.announcement.set('');
      this.problem.set({ kind: 'failed', title: "Couldn't copy", message: 'Your browser did not allow it. Select the text on the page and copy it yourself.', retryable: false });
    }
  }
}
