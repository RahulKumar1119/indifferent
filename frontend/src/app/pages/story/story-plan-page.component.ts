import { DOCUMENT, DecimalPipe } from '@angular/common';
import { HttpErrorResponse } from '@angular/common/http';
import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  OnDestroy,
  OnInit,
  computed,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { Router } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { Subscription } from 'rxjs';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { ROBOTS_NOINDEX, setPageSeo, setRobots } from '../../shared/seo';
import { StoryPlanProblem, describeProblem } from './story-plan-problem';
import { DEFAULT_STORY_PLAN_LIMITS, StoryPlanLimits, charCount } from './story-plan.models';
import { StoryPlanService } from './story-plan.service';
import { StoryProblemComponent } from './story-problem.component';

/**
 * Story2Shorts, step one: paste a story, get a plan. The planner is slow
 * (5 to 10 seconds, up to 25), so the page says so while it waits and never
 * clears the story: a failed attempt must not cost the user their text.
 */
@Component({
  selector: 'app-story-plan-page',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [DecimalPipe, LucideDynamicIcon, NavbarComponent, StoryProblemComponent],
  styles: [`
    :host { display: block; }
    .narrow { width: min(48rem, 100%); }
    form { display: flex; flex-direction: column; gap: var(--space-md); }
    label { display: block; font-weight: 600; }
    .hint { margin: var(--space-3xs) 0 0; font-size: var(--type-sm); color: var(--color-ink-2); }
    textarea { margin-top: var(--space-xs); min-height: 14rem; resize: vertical; line-height: 1.6; }
    .under { margin-top: var(--space-xs); display: flex; flex-wrap: wrap; align-items: flex-start; justify-content: space-between; gap: var(--space-2xs) var(--space-lg); font-size: var(--type-sm); }
    .count { margin: 0; color: var(--color-ink-2); }
    .err { margin: 0; display: flex; align-items: flex-start; gap: var(--space-xs); font-weight: 500; color: var(--color-negative); }
    .go { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-md); }
    .wait { margin: 0; max-width: 28rem; font-size: var(--type-sm); color: var(--color-ink-2); }
    @media (max-width: 39.99rem) { .go .ui-btn { width: 100%; } }
    .fine { margin: 0; max-width: 42rem; font-size: var(--type-sm); color: var(--color-ink-2); }
  `],
  template: `
    <div class="ui-app">
      <app-navbar></app-navbar>
      <main id="main" class="ui-app-main narrow hm">
        <div>
          <h1 class="ui-h1">Turn a story into a video plan</h1>
          <p class="ui-lead">
            Paste a story. It is split into {{ limits().minScenes }} to {{ limits().maxScenes }} scenes, up to
            {{ limits().maxTotalSeconds }} seconds in all, with narration for each. Then you choose a picture for every scene.
          </p>
        </div>

        @if (!limits().enabled) {
          <app-story-problem [problem]="offProblem" />
        } @else {
          <form class="ui-card" (submit)="onSubmit($event)" novalidate>
            <div>
              <label for="story">Your story</label>
              <p id="story-hint" class="hint">
                Write in English or Hindi. The narration stays in your language. The words used to search for pictures are
                made in English.
              </p>
              <textarea
                #field
                id="story"
                rows="10"
                [value]="story()"
                (input)="onInput($event)"
                [readOnly]="planning()"
                [attr.aria-describedby]="describedBy()"
                [attr.aria-invalid]="fieldError() ? 'true' : null"
                class="ui-field"
              ></textarea>
              <div class="under">
                <p id="story-count" class="count">
                  {{ chars() | number }} of {{ limits().maxStoryChars | number }} characters
                  @if (shortBy() > 0 && chars() > 0) {
                    · {{ shortBy() | number }} more needed
                  }
                </p>
                @if (fieldError()) {
                  <p id="story-error" role="alert" class="err">
                    <svg lucideIcon="triangle-alert" [size]="16" aria-hidden="true" style="flex: none; margin-top: 2px"></svg>
                    {{ fieldError() }}
                  </p>
                }
              </div>
            </div>

            <div class="go">
              <button type="submit" [disabled]="planning()" [attr.aria-busy]="planning()" class="ui-btn ui-btn--primary">
                @if (planning()) {
                  <svg lucideIcon="loader-2" [size]="16" aria-hidden="true" class="ui-spin"></svg>
                  Planning…
                } @else {
                  Plan my video
                }
              </button>
              <p role="status" class="wait" data-testid="progress">
                @if (planning()) {
                  Planning your video. This usually takes 5 to 10 seconds and can take up to 25. Keep this page open.
                }
              </p>
            </div>

            @if (problem(); as p) {
              <app-story-problem [problem]="p" />
            }

            <p class="fine">
              Your story is sent to an AI model to plan the scenes. We keep the plan, not the story. You can make
              {{ limits().perHour }} plans an hour.
            </p>
          </form>
        }
      </main>
    </div>
  `,
})
export class StoryPlanPageComponent implements OnInit, OnDestroy {
  private readonly service = inject(StoryPlanService);
  private readonly router = inject(Router);
  private readonly document = inject(DOCUMENT);
  private readonly field = viewChild<ElementRef<HTMLTextAreaElement>>('field');

  protected readonly limits = signal<StoryPlanLimits>(DEFAULT_STORY_PLAN_LIMITS);
  protected readonly story = signal('');
  protected readonly planning = signal(false);
  protected readonly problem = signal<StoryPlanProblem | null>(null);
  protected readonly fieldError = signal('');

  protected readonly chars = computed(() => charCount(this.story().trim()));
  protected readonly shortBy = computed(() => this.limits().minStoryChars - this.chars());
  protected readonly describedBy = computed(() =>
    ['story-hint', 'story-count', this.fieldError() ? 'story-error' : ''].filter(Boolean).join(' '),
  );

  /** What the page says when the planner is switched off on this site. */
  protected readonly offProblem = describeProblem(
    new HttpErrorResponse({ status: 503, error: { code: 'UNAVAILABLE' } }),
    'create',
  );

  private subs: Subscription[] = [];

  ngOnInit(): void {
    setPageSeo(this.document, {
      title: 'Plan a video from a story | Indifferent',
      description: 'Paste a story and get a scene-by-scene plan for a short video, then choose a picture for each scene.',
      canonical: 'https://indifferent.fun/story',
    });
    setRobots(this.document, ROBOTS_NOINDEX);

    // The limits come from the API so the counter and the checks never drift from
    // what the backend enforces. If the request fails the defaults still apply.
    this.subs.push(
      this.service.getLimits().subscribe({
        next: (l) => {
          if (l && l.maxStoryChars > 0 && l.minStoryChars > 0) {
            this.limits.set({ ...DEFAULT_STORY_PLAN_LIMITS, ...l });
          }
        },
        error: () => undefined,
      }),
    );
  }

  ngOnDestroy(): void {
    this.subs.forEach((s) => s.unsubscribe());
  }

  protected onInput(event: Event): void {
    this.story.set((event.target as HTMLTextAreaElement).value);
    if (this.fieldError()) this.fieldError.set('');
  }

  protected onSubmit(event: Event): void {
    event.preventDefault();
    if (this.planning()) return;

    const { minStoryChars: min, maxStoryChars: max } = this.limits();
    const n = this.chars();
    if (n < min) {
      this.fieldError.set(`Add at least ${min} characters. Your story has ${n}.`);
      this.field()?.nativeElement.focus();
      return;
    }
    if (n > max) {
      this.fieldError.set(`The limit is ${max.toLocaleString('en-US')} characters. Yours has ${n.toLocaleString('en-US')}; shorten it by ${(n - max).toLocaleString('en-US')}.`);
      this.field()?.nativeElement.focus();
      return;
    }

    this.fieldError.set('');
    this.problem.set(null);
    this.planning.set(true);
    this.subs.push(
      this.service.create(this.story().trim()).subscribe({
        next: (rec) => {
          this.planning.set(false);
          void this.router.navigate(['/story', rec.planId]);
        },
        error: (err) => {
          // The story stays in the box.
          this.planning.set(false);
          this.problem.set(describeProblem(err, 'create'));
        },
      }),
    );
  }
}
