import { DOCUMENT } from '@angular/common';
import { ChangeDetectionStrategy, Component, Injector, OnDestroy, OnInit, afterNextRender, computed, inject, input, signal } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { Subscription } from 'rxjs';
import { copyText } from '../../shared/clipboard';
import { StoryPlanProblem } from '../story/story-plan-problem';
import { describeCopyProblem } from './video-copy-problem';
import { CopyRequest, CopyResult, CopyStatus, MAX_TITLE_CHARS, VideoCopyService } from './video-copy.service';

type Source = 'job' | 'paste' | 'topic';

/** The least the server accepts to write from (backend videocopy.MinSourceRunes). */
export const MIN_SOURCE_CHARS = 15;

/** How long a "Copied" mark stays on a button. */
const COPIED_MS = 2000;

/**
 * Ten title ideas, a description and chapters for a finished video (#331). The
 * words come from the video's own transcript, a pasted one, or a sentence the
 * person types. Nothing is posted anywhere and nothing is stored: the result
 * lives on this page, and every piece has a Copy button.
 *
 * It hides itself when the server has the helper switched off.
 */
@Component({
  selector: 'app-thumbnail-copy',
  standalone: true,
  imports: [LucideDynamicIcon],
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: [`
    :host { display: block; }
    h2:focus, h3:focus { outline: none; }
    h3:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 4px; border-radius: 2px; }
    .head { display: flex; align-items: center; gap: var(--space-xs); }
    .head svg { color: var(--color-accent-text); }
    fieldset { margin: var(--space-md) 0 0; padding: 0; border: 0; min-width: 0; }
    legend { margin-bottom: var(--space-xs); font-size: var(--type-sm); font-weight: 600; color: var(--color-ink); }
    .choices { display: grid; gap: var(--space-xs); }
    .choice { position: relative; display: block; cursor: pointer; }
    .choice.off { cursor: not-allowed; opacity: 0.6; }
    .opt { display: flex; flex-direction: column; gap: var(--space-3xs); min-height: 44px; padding: var(--space-sm); border-radius: var(--radius-control); border: 1px solid var(--color-rule); transition: border-color var(--dur-short) var(--ease-out), background-color var(--dur-short) var(--ease-out); }
    .choice:not(.off) .opt:hover { background: var(--color-paper-3); }
    .opt b { font-size: var(--type-sm); font-weight: 600; color: var(--color-ink); }
    .opt small { font-size: var(--type-xs); color: var(--color-muted); }
    input[type='radio']:checked + .opt { border: 2px solid var(--color-accent); background: color-mix(in oklab, var(--color-accent) 8%, var(--app-card, var(--color-paper))); }
    .choice:has(input:focus-visible) { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: var(--radius-control); }
    .field { margin-top: var(--space-md); display: flex; flex-direction: column; gap: var(--space-2xs); }
    textarea { min-height: 7rem; resize: vertical; line-height: 1.5; }
    .count { margin: 0; font-size: var(--type-xs); color: var(--color-muted); }
    .go { margin-top: var(--space-md); display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-sm); }
    .go.tight { margin-top: var(--space-2xs); }
    .left { margin: 0; font-size: var(--type-sm); color: var(--color-ink-2); }
    .result { margin-top: var(--space-lg); display: flex; flex-direction: column; gap: var(--space-lg); }
    .result h3 { margin: 0 0 var(--space-xs); font-size: var(--type-md); font-weight: 600; }
    .titles { margin: 0; padding: 0; list-style: none; display: grid; gap: var(--space-xs); }
    .titles li { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-xs) var(--space-sm); padding: var(--space-sm); border: 1px solid var(--color-rule); border-radius: var(--radius-control); }
    .t { flex: 1 1 16rem; min-width: 0; }
    .t p { margin: 0; overflow-wrap: anywhere; }
    .t .count { margin-top: var(--space-3xs); }
    .desc { margin: 0; padding: var(--space-sm); border: 1px solid var(--color-rule); border-radius: var(--radius-control); white-space: pre-wrap; overflow-wrap: anywhere; }
    .chapters { margin: var(--space-sm) 0 0; padding: 0; list-style: none; display: grid; gap: var(--space-3xs); font-size: var(--type-sm); }
    .chapters li { display: flex; gap: var(--space-sm); }
    .chapters time { flex: none; min-width: 3.5rem; font-variant-numeric: tabular-nums; color: var(--color-ink-2); }
    .acts { margin-top: var(--space-xs); display: flex; flex-wrap: wrap; gap: var(--space-xs); }
    .refine { padding-top: var(--space-md); border-top: 1px solid var(--color-rule); }
    .sr { position: absolute; width: 1px; height: 1px; margin: -1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
    .box { display: flex; align-items: flex-start; gap: var(--space-sm); margin-top: var(--space-md); padding: var(--space-md); border-radius: var(--radius-control); border: 1px solid color-mix(in oklab, var(--color-negative) 45%, transparent); background: color-mix(in oklab, var(--color-negative) 6%, var(--app-card, var(--color-paper))); }
    .box.wait { border-color: color-mix(in oklab, var(--color-caution) 50%, transparent); background: color-mix(in oklab, var(--color-caution) 8%, var(--app-card, var(--color-paper))); }
    .box p { margin: 0; }
    .box .m { margin-top: var(--space-3xs); font-size: var(--type-sm); color: var(--color-ink-2); }
    .box svg { flex: none; margin-top: 2px; }
    @media (prefers-reduced-motion: reduce) { .opt { transition: none; } }
  `],
  template: `
    @if (status(); as st) {
      @if (st.enabled) {
        <section class="ui-card" aria-labelledby="copy-title" data-testid="copy">
          <h2 id="copy-title" class="ui-card-title head">
            <svg lucideIcon="sparkles" [size]="20" aria-hidden="true"></svg>Title and description
          </h2>
          <p class="ui-card-sub">Ten title ideas and a description for YouTube, written from your video. Nothing is posted: you copy the parts you like.</p>

          <fieldset>
            <legend>Write from</legend>
            <div class="choices">
              <label class="choice" [class.off]="!st.transcriptAvailable">
                <input type="radio" name="copy-source" class="sr" value="job" [checked]="source() === 'job'" [disabled]="!st.transcriptAvailable || busy() !== null" (change)="source.set('job')" />
                <span class="opt">
                  <b>This video's transcript</b>
                  <small>{{ st.transcriptAvailable ? 'Also gives chapters, because every word has a time.' : 'This video has no transcript yet.' }}</small>
                </span>
              </label>
              <label class="choice">
                <input type="radio" name="copy-source" class="sr" value="paste" [checked]="source() === 'paste'" [disabled]="busy() !== null" (change)="source.set('paste')" />
                <span class="opt">
                  <b>A transcript I paste</b>
                  <small>Start lines with a time, such as 1:05, to get chapters.</small>
                </span>
              </label>
              <label class="choice">
                <input type="radio" name="copy-source" class="sr" value="topic" [checked]="source() === 'topic'" [disabled]="busy() !== null" (change)="source.set('topic')" />
                <span class="opt">
                  <b>A sentence about the video</b>
                  <small>Quickest. There are no chapters without timestamps.</small>
                </span>
              </label>
            </div>
          </fieldset>

          @if (source() === 'paste') {
            <div class="field">
              <label class="ui-label" for="copy-paste">Transcript</label>
              <textarea id="copy-paste" class="ui-field" rows="8" [value]="pasted()" [readOnly]="busy() !== null" [attr.maxlength]="st.maxTranscriptChars" aria-describedby="copy-paste-count" data-testid="copy-paste" (input)="pasted.set($any($event.target).value)"></textarea>
              <p id="copy-paste-count" class="count">{{ count(pasted()) }} of {{ st.maxTranscriptChars }} characters</p>
            </div>
          } @else if (source() === 'topic') {
            <div class="field">
              <label class="ui-label" for="copy-topic">What is the video about?</label>
              <textarea id="copy-topic" class="ui-field" rows="3" [value]="topic()" [readOnly]="busy() !== null" [attr.maxlength]="st.maxTopicChars" aria-describedby="copy-topic-count" data-testid="copy-topic" (input)="topic.set($any($event.target).value)"></textarea>
              <p id="copy-topic-count" class="count">{{ count(topic()) }} of {{ st.maxTopicChars }} characters</p>
            </div>
          }

          <div class="go">
            <button type="button" class="ui-btn ui-btn--primary" data-testid="copy-write" [disabled]="!canWrite()" [attr.aria-busy]="busy() === 'write'" (click)="write()">
              @if (busy() === 'write') {
                <svg lucideIcon="loader-2" [size]="16" class="ui-spin" aria-hidden="true"></svg>
                Writing
              } @else {
                <svg lucideIcon="sparkles" [size]="16" aria-hidden="true"></svg>
                {{ result() ? 'Write again' : 'Write titles and description' }}
              }
            </button>
            @if (leftNote(); as note) {
              <p class="left" data-testid="copy-left">{{ note }}</p>
            }
          </div>

          @if (problem(); as p) {
            <div class="box" [class.wait]="p.kind === 'rate_limited' || p.kind === 'disabled'" [attr.role]="p.kind === 'rate_limited' || p.kind === 'disabled' ? 'status' : 'alert'" data-testid="copy-problem" [attr.data-kind]="p.kind">
              <svg [lucideIcon]="p.kind === 'rate_limited' ? 'clock' : 'triangle-alert'" [size]="20" aria-hidden="true"></svg>
              <div>
                <p><b>{{ p.title }}</b></p>
                <p class="m">{{ p.message }}</p>
              </div>
            </div>
          }

          @if (result(); as r) {
            <div class="result" data-testid="copy-result">
              <section aria-labelledby="copy-titles-h">
                <h3 id="copy-titles-h" tabindex="-1">Title ideas</h3>
                <ol class="titles">
                  @for (t of r.titles; track $index; let i = $index) {
                    <li>
                      <div class="t">
                        <p [attr.data-testid]="'copy-title-' + (i + 1)">{{ t }}</p>
                        <p class="count">{{ count(t) }} of {{ maxTitle }} characters</p>
                      </div>
                      <button type="button" class="ui-btn ui-btn--secondary" [attr.data-testid]="'copy-title-btn-' + (i + 1)" (click)="copy('title-' + (i + 1), t, 'Title ' + (i + 1))">
                        <svg [lucideIcon]="copied() === 'title-' + (i + 1) ? 'check' : 'copy'" [size]="16" aria-hidden="true"></svg>
                        {{ copied() === 'title-' + (i + 1) ? 'Copied' : 'Copy' }}<span class="sr"> title {{ i + 1 }}</span>
                      </button>
                    </li>
                  }
                </ol>
                <div class="refine field">
                  <label class="ui-label" for="copy-refine">Want different titles? Say how.</label>
                  <textarea id="copy-refine" class="ui-field" rows="2" [value]="refineText()" [readOnly]="busy() !== null" [attr.maxlength]="st.maxRefineChars" aria-describedby="copy-refine-count" placeholder="For example: shorter, or more curious" data-testid="copy-refine" (input)="refineText.set($any($event.target).value)"></textarea>
                  <p id="copy-refine-count" class="count">{{ count(refineText()) }} of {{ st.maxRefineChars }} characters</p>
                  <div class="go tight">
                    <button type="button" class="ui-btn ui-btn--secondary" data-testid="copy-refine-go" [disabled]="!canWrite()" [attr.aria-busy]="busy() === 'refine'" (click)="refine()">
                      @if (busy() === 'refine') {
                        <svg lucideIcon="loader-2" [size]="16" class="ui-spin" aria-hidden="true"></svg>
                        Writing
                      } @else {
                        <svg lucideIcon="sparkles" [size]="16" aria-hidden="true"></svg>
                        Write new titles
                      }
                    </button>
                  </div>
                </div>
              </section>

              <section aria-labelledby="copy-desc-h">
                <h3 id="copy-desc-h">Description</h3>
                <p class="desc" data-testid="copy-description">{{ r.description }}</p>
                @if (r.chapters.length > 0) {
                  <h3 id="copy-chapters-h" style="margin-top: var(--space-md)">Chapters</h3>
                  <ol class="chapters" aria-labelledby="copy-chapters-h" data-testid="copy-chapters">
                    @for (c of r.chapters; track c.start) {
                      <li><time>{{ c.time }}</time><span>{{ c.title }}</span></li>
                    }
                  </ol>
                }
                @for (n of r.notes; track $index) {
                  <p class="ui-alert ui-alert--warn" role="status" data-testid="copy-note">{{ n }}</p>
                }
                <div class="acts">
                  <button type="button" class="ui-btn ui-btn--secondary" data-testid="copy-description-btn" (click)="copy('description', r.description, 'Description')">
                    <svg [lucideIcon]="copied() === 'description' ? 'check' : 'copy'" [size]="16" aria-hidden="true"></svg>
                    {{ copied() === 'description' ? 'Copied' : 'Copy description' }}
                  </button>
                  @if (r.chapters.length > 0) {
                    <button type="button" class="ui-btn ui-btn--secondary" data-testid="copy-full-btn" (click)="copy('full', r.descriptionWithChapters, 'Description with chapters')">
                      <svg [lucideIcon]="copied() === 'full' ? 'check' : 'copy'" [size]="16" aria-hidden="true"></svg>
                      {{ copied() === 'full' ? 'Copied' : 'Copy description with chapters' }}
                    </button>
                  }
                </div>
              </section>
            </div>
          }
          <p class="sr" role="status" data-testid="copy-status">{{ announcement() }}</p>
        </section>
      }
    }
  `,
})
export class ThumbnailCopyComponent implements OnInit, OnDestroy {
  /** The Shorts job whose video the words are for. */
  readonly jobId = input.required<string>();

  private readonly service = inject(VideoCopyService);
  private readonly document = inject(DOCUMENT);
  private readonly injector = inject(Injector);

  protected readonly maxTitle = MAX_TITLE_CHARS;
  protected readonly status = signal<CopyStatus | null>(null);
  protected readonly source = signal<Source>('topic');
  protected readonly pasted = signal('');
  protected readonly topic = signal('');
  protected readonly refineText = signal('');
  protected readonly busy = signal<'write' | 'refine' | null>(null);
  protected readonly result = signal<CopyResult | null>(null);
  protected readonly problem = signal<StoryPlanProblem | null>(null);
  protected readonly copied = signal('');
  protected readonly announcement = signal('');
  /** Uses left today, kept up to date from each answer. */
  private readonly remaining = signal<number | null>(null);

  private statusSub?: Subscription;
  private writeSub?: Subscription;
  private copiedTimer: ReturnType<typeof setTimeout> | null = null;

  /** Whether the chosen source has enough in it to write from. */
  protected readonly canWrite = computed(() => {
    if (this.busy() !== null) return false;
    switch (this.source()) {
      case 'job':
        return !!this.status()?.transcriptAvailable;
      case 'paste':
        return this.count(this.pasted()) >= MIN_SOURCE_CHARS;
      default:
        return this.count(this.topic()) >= MIN_SOURCE_CHARS;
    }
  });

  protected readonly leftNote = computed(() => {
    const st = this.status();
    const left = this.remaining();
    if (!st || left === null || !st.dailyLimit) return '';
    return `${left} of ${st.dailyLimit} tries left today. Writing new titles uses one too.`;
  });

  ngOnInit(): void {
    this.statusSub = this.service.status(this.jobId()).subscribe({
      next: (st) => {
        this.status.set(st);
        this.remaining.set(typeof st.remainingToday === 'number' ? st.remainingToday : null);
        this.source.set(st.transcriptAvailable ? 'job' : 'topic');
      },
      // The section is a bonus. If it cannot say whether it is on, it stays out of the way.
      error: () => this.status.set(null),
    });
  }

  ngOnDestroy(): void {
    this.statusSub?.unsubscribe();
    this.writeSub?.unsubscribe();
    if (this.copiedTimer !== null) clearTimeout(this.copiedTimer);
  }

  /** Characters the way the server counts them: what a person sees, not UTF-16 units. */
  protected count(text: string): number {
    return Array.from(text.trim()).length;
  }

  private request(): CopyRequest {
    switch (this.source()) {
      case 'job':
        return { useJobTranscript: true };
      case 'paste':
        return { transcript: this.pasted() };
      default:
        return { topic: this.topic() };
    }
  }

  protected write(): void {
    if (!this.canWrite()) return;
    this.run('write', this.request());
  }

  /** Asks for different titles and keeps the description and chapters that are already shown. */
  protected refine(): void {
    const r = this.result();
    if (!r || !this.canWrite()) return;
    this.run('refine', { ...this.request(), refine: this.refineText(), previousTitles: r.titles });
  }

  private run(kind: 'write' | 'refine', body: CopyRequest): void {
    this.busy.set(kind);
    this.problem.set(null);
    this.announcement.set(kind === 'write' ? 'Writing the titles and description.' : 'Writing new titles.');
    this.writeSub?.unsubscribe();
    this.writeSub = this.service.write(this.jobId(), body).subscribe({
      next: (res) => {
        this.busy.set(null);
        if (typeof res.remainingToday === 'number') this.remaining.set(res.remainingToday);
        if (kind === 'write') {
          this.result.set(res);
          this.refineText.set('');
          this.announcement.set(
            res.chapters.length > 0
              ? `${res.titles.length} titles, a description and ${res.chapters.length} chapters are ready.`
              : `${res.titles.length} titles and a description are ready.`,
          );
        } else {
          const cur = this.result();
          if (cur) this.result.set({ ...cur, titles: res.titles });
          this.refineText.set('');
          this.announcement.set(`${res.titles.length} new titles are ready.`);
        }
        this.copied.set('');
        afterNextRender(() => this.document.getElementById('copy-titles-h')?.focus(), { injector: this.injector });
      },
      error: (err) => {
        this.busy.set(null);
        const p = describeCopyProblem(err);
        this.problem.set(p);
        this.announcement.set('');
        if (p.kind === 'rate_limited') this.remaining.set(0);
      },
    });
  }

  protected async copy(key: string, text: string, label: string): Promise<void> {
    const ok = await copyText(text, this.document);
    if (this.copiedTimer !== null) clearTimeout(this.copiedTimer);
    if (ok) {
      this.copied.set(key);
      this.announcement.set(`${label} copied.`);
      this.copiedTimer = setTimeout(() => this.copied.set(''), COPIED_MS);
    } else {
      this.copied.set('');
      this.announcement.set('');
      this.problem.set({
        kind: 'failed',
        title: "Couldn't copy",
        message: 'Your browser did not allow it. Select the text on the page and copy it yourself.',
        retryable: true,
      });
    }
  }
}
