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
  styles: [
    `
      .serif {
        font-family: 'Cormorant Garamond', Georgia, serif;
      }
    `,
  ],
  template: `
    <app-navbar></app-navbar>
    <main class="mx-auto w-full max-w-3xl px-4 py-10">
      <h1 class="serif text-4xl font-medium leading-tight sm:text-5xl">Turn a story into a video plan</h1>
      <p class="mt-3 max-w-2xl text-[hsl(var(--muted-foreground))]">
        Paste a story. It is split into {{ limits().minScenes }} to {{ limits().maxScenes }} scenes, up to
        {{ limits().maxTotalSeconds }} seconds in all, with narration for each. Then you choose a picture for every scene.
      </p>

      @if (!limits().enabled) {
        <div class="mt-8 max-w-xl">
          <app-story-problem [problem]="offProblem" />
        </div>
      } @else {
        <form class="mt-8" (submit)="onSubmit($event)" novalidate>
          <label for="story" class="block font-medium">Your story</label>
          <p id="story-hint" class="mt-1 text-sm text-[hsl(var(--muted-foreground))]">
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
            class="mt-2 block min-h-56 w-full resize-y rounded-xl border border-[hsl(var(--border))] bg-[hsl(var(--background))] px-4 py-3 text-base leading-relaxed focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-primary)]"
          ></textarea>

          <div class="mt-2 flex flex-wrap items-start justify-between gap-x-6 gap-y-1 text-sm">
            <p id="story-count" class="text-[hsl(var(--muted-foreground))]">
              {{ chars() | number }} of {{ limits().maxStoryChars | number }} characters
              @if (shortBy() > 0 && chars() > 0) {
                · {{ shortBy() | number }} more needed
              }
            </p>
            @if (fieldError()) {
              <p id="story-error" role="alert" class="flex items-start gap-2 font-medium">
                <svg lucideIcon="triangle-alert" [size]="16" aria-hidden="true" class="mt-0.5 shrink-0 text-[var(--color-error)]"></svg>
                {{ fieldError() }}
              </p>
            }
          </div>

          <div class="mt-6 flex flex-wrap items-center gap-4">
            <button
              type="submit"
              [disabled]="planning()"
              class="inline-flex min-h-11 items-center gap-2 rounded-full bg-[#BC5227] px-6 text-sm font-semibold text-white hover:bg-[#a84822] disabled:cursor-not-allowed disabled:opacity-60"
            >
              @if (planning()) {
                <svg lucideIcon="loader-2" [size]="16" aria-hidden="true" class="animate-spin motion-reduce:animate-none"></svg>
                Planning…
              } @else {
                Plan my video
              }
            </button>
            <p role="status" class="max-w-md text-sm text-[hsl(var(--muted-foreground))]" data-testid="progress">
              @if (planning()) {
                Planning your video. This usually takes 5 to 10 seconds and can take up to 25. Keep this page open.
              }
            </p>
          </div>

          @if (problem(); as p) {
            <div class="mt-6 max-w-xl">
              <app-story-problem [problem]="p" />
            </div>
          }

          <p class="mt-8 max-w-2xl text-sm text-[hsl(var(--muted-foreground))]">
            Your story is sent to an AI model to plan the scenes. We keep the plan, not the story. You can make
            {{ limits().perHour }} plans an hour.
          </p>
        </form>
      }
    </main>
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
