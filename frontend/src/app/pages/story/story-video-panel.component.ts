import { DOCUMENT, DatePipe, NgTemplateOutlet } from '@angular/common';
import { HttpErrorResponse } from '@angular/common/http';
import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  Injector,
  OnDestroy,
  afterNextRender,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
  untracked,
} from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { Subscription, timer } from 'rxjs';
import { switchMap } from 'rxjs/operators';
import { StoryPlanProblem } from './story-plan-problem';
import { StoryProblemComponent } from './story-problem.component';
import { StoryVideoOptionsComponent, VideoOptions } from './story-video-options.component';
import { StoryVideoProgressComponent } from './story-video-progress.component';
import { VideoContext, describeVideoProblem } from './story-video-problem';
import {
  CAPTION_PRESETS,
  DEFAULT_CAPTION_PRESET,
  PICTURE_ERROR_CODES,
  StoryVideo,
  StoryVideoLimits,
  captionLabel,
  formatLength,
  isActive,
  sceneInMessage,
  wasCancelled,
} from './story-video.models';
import { StoryVideoService } from './story-video.service';

/** Voices to offer if the limits cannot be loaded. The API refuses any it does not know. */
const FALLBACK_VOICES: Record<string, string[]> = {
  en: ['Joanna', 'Matthew', 'Amy', 'Brian', 'Aditi'],
  hi: ['Aditi'],
};

/** Polling gives up after this many failures in a row and asks the person to retry. */
const MAX_POLL_FAILURES = 5;
const MAX_POLL_DELAY_MS = 30_000;

const PRIMARY =
  'inline-flex min-h-11 items-center justify-center gap-2 rounded-full bg-[#BC5227] px-6 text-sm font-semibold text-white hover:bg-[#a84822] disabled:cursor-not-allowed disabled:opacity-50';
const SECONDARY =
  'inline-flex min-h-11 items-center justify-center gap-2 rounded-full border border-[hsl(var(--border))] px-5 text-sm font-medium hover:bg-[hsl(var(--muted))] disabled:cursor-not-allowed disabled:opacity-50';
const QUIET =
  'inline-flex min-h-11 items-center justify-center px-3 text-sm underline underline-offset-2 hover:text-[hsl(var(--muted-foreground))] disabled:cursor-not-allowed disabled:opacity-50';

/**
 * Turning a saved plan into a video, and everything that follows: choosing the
 * voice and the captions, following the work as it happens, cancelling it,
 * watching and downloading the result, and removing videos.
 *
 * The page can be closed and reopened at any point: on load it asks the API for
 * the videos made from this plan, and picks up one that is still being made.
 * That is also why two tabs cannot start two videos from one plan: the second is
 * told, and follows the first.
 */
@Component({
  selector: 'app-story-video-panel',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [DatePipe, NgTemplateOutlet, LucideDynamicIcon, StoryProblemComponent, StoryVideoOptionsComponent, StoryVideoProgressComponent],
  styles: [
    `
      .serif {
        font-family: 'Cormorant Garamond', Georgia, serif;
      }
    `,
  ],
  template: `
    <section class="mt-12 border-t border-[hsl(var(--border))] pt-8" aria-labelledby="video-heading" data-testid="video-panel">
      <h2 id="video-heading" tabindex="-1" class="serif text-3xl font-medium leading-tight sm:text-4xl">Make the video</h2>

      <!-- Said once, politely, when a video finishes, fails or is removed. -->
      <p class="sr-only" role="status" data-testid="announcement">{{ announcement() }}</p>

      @if (loading()) {
        <p role="status" class="mt-4 flex items-center gap-3 text-[hsl(var(--muted-foreground))]" data-testid="loading">
          <svg lucideIcon="loader-2" [size]="18" aria-hidden="true" class="animate-spin motion-reduce:animate-none"></svg>
          Checking for videos from this plan…
        </p>
      } @else if (switchedOff()) {
        <div class="mt-4 max-w-xl">
          <app-story-problem [problem]="offProblem" />
        </div>
      } @else {
        @if (loadProblem(); as p) {
          <div class="mt-4 max-w-xl">
            <app-story-problem [problem]="p" [polite]="true" actionLabel="Try again" (action)="reload()" />
          </div>
        }

        @switch (view()) {
          @case ('active') {
            @if (latest(); as v) {
              <div class="mt-4 max-w-3xl" data-testid="active">
                <h3 data-state-heading tabindex="-1" class="text-xl font-semibold">Making your video</h3>
                @if (followNotice()) {
                  <p class="mt-1 text-sm text-[hsl(var(--muted-foreground))]" data-testid="follow-notice">{{ followNotice() }}</p>
                }
                <div class="mt-4">
                  <app-story-video-progress [status]="v.status" [progress]="v.progress" />
                </div>
                @if (reconnecting()) {
                  <p class="mt-3 text-sm text-[hsl(var(--muted-foreground))]" data-testid="reconnecting">
                    Can't reach the server right now. The video keeps being made. Trying again…
                  </p>
                }
                @if (pollProblem(); as p) {
                  <div class="mt-4 max-w-xl">
                    <app-story-problem [problem]="p" actionLabel="Check again" (action)="checkAgain(v)" />
                  </div>
                }
                <p class="mt-4 max-w-2xl text-sm text-[hsl(var(--muted-foreground))]">
                  You can leave this page. The video keeps being made, and this page picks it up when you come back.
                </p>
                @if (actionProblem(); as ap) {
                  <div class="mt-4 max-w-xl"><app-story-problem [problem]="ap.problem" /></div>
                }
                <div class="mt-4">
                  <button type="button" [class]="secondary" [disabled]="cancelling()" (click)="cancel(v)" data-testid="cancel">
                    @if (cancelling()) {
                      <svg lucideIcon="loader-2" [size]="16" aria-hidden="true" class="animate-spin motion-reduce:animate-none"></svg>
                      Cancelling…
                    } @else {
                      Cancel the video
                    }
                  </button>
                </div>
              </div>
            }
          }

          @case ('failed') {
            @if (latest(); as v) {
              <div class="mt-4 max-w-3xl" data-testid="failed">
                @if (cancelled()) {
                  <h3 data-state-heading tabindex="-1" class="text-xl font-semibold">You cancelled this video</h3>
                  <p class="mt-1 text-[hsl(var(--muted-foreground))]">Nothing was kept. Make it again whenever you like.</p>
                } @else {
                  <h3 data-state-heading tabindex="-1" class="sr-only">The last video was not finished</h3>
                  <app-story-problem [problem]="failureProblem()" [polite]="!justFailed()" />
                  @if (sceneToFix(); as n) {
                    <button type="button" [class]="secondary + ' mt-3'" (click)="goToScene.emit(n)" data-testid="go-to-scene">
                      Go to scene {{ n }}
                    </button>
                  }
                }
                @if (actionProblem(); as ap) {
                  <div class="mt-4 max-w-xl"><app-story-problem [problem]="ap.problem" /></div>
                }
                <button type="button" [class]="quiet + ' mt-2'" [disabled]="busyId() === v.jobId" (click)="remove(v, true)" data-testid="dismiss">
                  Remove this attempt
                </button>
              </div>
              <div class="mt-6">
                <ng-container [ngTemplateOutlet]="makeForm" />
              </div>
            }
          }

          @case ('done') {
            @if (latest(); as v) {
              <div class="mt-4 max-w-3xl" data-testid="done">
                <h3 data-state-heading tabindex="-1" class="text-xl font-semibold">Your video is ready</h3>
                <ng-container [ngTemplateOutlet]="finished" [ngTemplateOutletContext]="{ $implicit: v, latest: true }" />
              </div>
              @if (showForm()) {
                <div class="mt-8">
                  <ng-container [ngTemplateOutlet]="makeForm" />
                </div>
              }
            }
          }

          @default {
            <div class="mt-4">
              <ng-container [ngTemplateOutlet]="makeForm" />
            </div>
          }
        }

        @if (earlier().length > 0) {
          <div class="mt-10 max-w-3xl" data-testid="earlier">
            <h3 class="text-lg font-semibold">Earlier videos from this plan</h3>
            <ul class="mt-3 divide-y divide-[hsl(var(--border))] border-y border-[hsl(var(--border))]">
              @for (v of earlier(); track v.jobId) {
                <li class="py-4">
                  <ng-container [ngTemplateOutlet]="finished" [ngTemplateOutletContext]="{ $implicit: v, latest: false }" />
                </li>
              }
            </ul>
          </div>
        }
      }
    </section>

    <!-- The two choices and the button that starts the video. -->
    <ng-template #makeForm>
      <div class="max-w-3xl" data-testid="make-form">
        @if (view() === 'form') {
          <p class="max-w-2xl text-[hsl(var(--muted-foreground))]">
            Choose a voice and how the captions look, then make the video from this plan. It can take a few minutes. You
            can leave this page and come back.
          </p>
        } @else if (view() === 'done') {
          <h3 class="text-xl font-semibold">Make another version</h3>
        } @else {
          <h3 class="text-xl font-semibold">Make it again</h3>
        }

        @if (voices().length > 0) {
          <div class="mt-6">
            <app-story-video-options
              [language]="language()"
              [voices]="voices()"
              [value]="options()"
              [disabled]="making()"
              (valueChange)="options.set($event)"
            />
          </div>
        }

        @if (reason()) {
          <p id="video-blocker" class="mt-6 max-w-2xl font-medium" data-testid="blocker">{{ reason() }}</p>
        }
        @if (makeProblem(); as p) {
          <div class="mt-6 max-w-xl">
            <app-story-problem [problem]="p" [actionLabel]="makeActionLabel(p)" (action)="onMakeProblemAction(p)" />
          </div>
        }

        <div class="mt-6 flex flex-wrap items-center gap-4">
          <button
            type="button"
            [class]="primary"
            [disabled]="!!reason() || making()"
            [attr.aria-describedby]="reason() ? 'video-blocker' : null"
            (click)="make()"
            data-testid="make"
          >
            @if (making()) {
              <svg lucideIcon="loader-2" [size]="16" aria-hidden="true" class="animate-spin motion-reduce:animate-none"></svg>
              Starting…
            } @else if (view() === 'done') {
              Make this version
            } @else if (view() === 'failed') {
              Make the video again
            } @else {
              Make the video
            }
          </button>
          @if (view() === 'done') {
            <button type="button" [class]="quiet" [disabled]="making()" (click)="closeForm()">Not now</button>
          }
        </div>

        @if (limits(); as l) {
          <p class="mt-6 max-w-2xl text-sm text-[hsl(var(--muted-foreground))]" data-testid="limits-note">
            You can make {{ l.perDay }} videos a day, {{ l.maxActive }} at a time. A video that is cancelled or does not
            finish still counts.
          </p>
        }
      </div>
    </ng-template>

    <!-- A finished video: facts, then what can be done with it. -->
    <ng-template #finished let-v let-isLatest="latest">
      <div [attr.data-testid]="isLatest ? 'finished' : 'finished-earlier'" [attr.data-job]="v.jobId">
        <p [class.mt-2]="isLatest" class="[overflow-wrap:anywhere]">
          @if (v.totalSeconds) {
            <span data-testid="length">{{ length(v) }}</span>
            <span aria-hidden="true"> · </span>
          }
          <span>{{ v.voice }} voice</span>
          <span aria-hidden="true"> · </span>
          <span>{{ caption(v) }} captions</span>
        </p>
        <p class="mt-1 text-sm text-[hsl(var(--muted-foreground))]">
          Made {{ v.completedAt || v.createdAt | date: 'd MMM y, h:mm a' }}.
          @if (isStale(v)) {
            <span data-testid="stale-note"> The plan has changed since, so this video does not include the newest edits.</span>
          }
        </p>

        @if (watchId() === v.jobId && watchUrl()) {
          <video
            class="mt-4 aspect-[9/16] max-h-[70dvh] w-full max-w-xs rounded-xl bg-[hsl(var(--muted))]"
            [src]="watchUrl()"
            controls
            playsinline
            preload="metadata"
            aria-label="Your finished video"
            (error)="onWatchError(v)"
            data-testid="player"
          ></video>
        }

        @if (actionProblem(); as ap) {
          @if (ap.jobId === v.jobId) {
            <div class="mt-4 max-w-xl"><app-story-problem [problem]="ap.problem" /></div>
          }
        }

        <div class="mt-4 flex flex-wrap items-center gap-3">
          <button
            type="button"
            [class]="isLatest && !showForm() ? primary : secondary"
            [disabled]="busyId() === v.jobId"
            (click)="download(v)"
            data-testid="download"
          >
            @if (busyId() === v.jobId && busyAction() === 'download') {
              <svg lucideIcon="loader-2" [size]="16" aria-hidden="true" class="animate-spin motion-reduce:animate-none"></svg>
              Preparing…
            } @else {
              <svg lucideIcon="download" [size]="16" aria-hidden="true"></svg>
              Download MP4
            }
          </button>
          <button
            type="button"
            [class]="secondary"
            [disabled]="busyId() === v.jobId"
            [attr.aria-expanded]="watchId() === v.jobId"
            (click)="toggleWatch(v)"
            data-testid="watch"
          >
            {{ watchId() === v.jobId ? 'Hide the video' : 'Watch it here' }}
          </button>
          @if (isLatest && !showForm()) {
            <button type="button" [class]="secondary" (click)="openForm()" data-testid="another">Make another version</button>
          }
          <button
            type="button"
            [id]="'delete-' + v.jobId"
            [class]="quiet"
            [disabled]="busyId() === v.jobId"
            (click)="askDelete(v)"
            data-testid="delete"
          >
            Delete
          </button>
        </div>

        @if (confirming() === v.jobId) {
          <div
            role="group"
            aria-label="Confirm delete"
            class="mt-4 max-w-md rounded-xl border border-[hsl(var(--border))] p-4"
            (keydown.escape)="cancelDelete(v)"
            data-testid="confirm"
          >
            <p class="font-medium">Delete this video?</p>
            <p class="mt-1 text-sm text-[hsl(var(--muted-foreground))]">
              The file is removed for good. Your plan stays, and you can make another video from it.
            </p>
            <div class="mt-3 flex flex-wrap gap-3">
              <button type="button" [id]="'keep-' + v.jobId" [class]="secondary" (click)="cancelDelete(v)" data-testid="keep">Keep it</button>
              <button type="button" [class]="secondary" [disabled]="busyId() === v.jobId" (click)="remove(v, false)" data-testid="confirm-delete">
                @if (busyId() === v.jobId && busyAction() === 'delete') {
                  <svg lucideIcon="loader-2" [size]="16" aria-hidden="true" class="animate-spin motion-reduce:animate-none"></svg>
                  Deleting…
                } @else {
                  Delete the video
                }
              </button>
            </div>
          </div>
        }
      </div>
    </ng-template>
  `,
})
export class StoryVideoPanelComponent implements OnDestroy {
  private readonly service = inject(StoryVideoService);
  private readonly document = inject(DOCUMENT);
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  /** The saved plan the video is made from. */
  readonly planId = input.required<string>();
  /** Its revision. The API refuses to make a video from any other. */
  readonly revision = input.required<number>();
  readonly language = input.required<string>();
  /** Why a video cannot be made from this plan right now; empty when it can. */
  readonly blocker = input('');
  /** Milliseconds between checks on a video being made. */
  readonly pollMs = input(3000);

  /** The plan changed somewhere else; the page should reload it. */
  readonly planChanged = output<void>();
  /** A scene's number, when the failure is about its picture. */
  readonly goToScene = output<number>();

  protected readonly primary = PRIMARY;
  protected readonly secondary = SECONDARY;
  protected readonly quiet = QUIET;

  protected readonly loading = signal(true);
  protected readonly loadProblem = signal<StoryPlanProblem | null>(null);
  protected readonly limits = signal<StoryVideoLimits | null>(null);
  protected readonly videos = signal<StoryVideo[]>([]);
  protected readonly options = signal<VideoOptions>({ voice: '', caption: DEFAULT_CAPTION_PRESET });
  protected readonly making = signal(false);
  protected readonly makeProblem = signal<StoryPlanProblem | null>(null);
  protected readonly cancelling = signal(false);
  protected readonly followNotice = signal('');
  protected readonly reconnecting = signal(false);
  protected readonly pollProblem = signal<StoryPlanProblem | null>(null);
  protected readonly showForm = signal(false);
  protected readonly announcement = signal('');
  /** True from a failure the person watched happen until the page is left. */
  protected readonly justFailed = signal(false);

  // What is being done to which video: download, delete, or the watch link.
  protected readonly busyId = signal('');
  protected readonly busyAction = signal<'download' | 'delete' | 'watch' | ''>('');
  protected readonly confirming = signal('');
  protected readonly watchId = signal('');
  protected readonly watchUrl = signal('');
  protected readonly actionProblem = signal<{ jobId: string; problem: StoryPlanProblem } | null>(null);

  protected readonly switchedOff = computed(() => this.limits()?.enabled === false);
  protected readonly latest = computed(() => this.videos()[0] ?? null);
  protected readonly view = computed<'form' | 'active' | 'failed' | 'done'>(() => {
    const v = this.latest();
    if (!v) return 'form';
    if (isActive(v.status)) return 'active';
    return v.status === 'completed' ? 'done' : 'failed';
  });
  protected readonly cancelled = computed(() => {
    const v = this.latest();
    return !!v && wasCancelled(v);
  });
  /** Finished videos other than the one on top, newest first. */
  protected readonly earlier = computed(() => this.videos().slice(1).filter((v) => v.status === 'completed'));
  protected readonly voices = computed(() => {
    const fromApi = this.limits()?.voices?.[this.language()];
    return fromApi && fromApi.length > 0 ? fromApi : (FALLBACK_VOICES[this.language()] ?? []);
  });
  /** What stops a video being made now: the page's reason, or a language no voice can read. */
  protected readonly reason = computed(() => {
    if (this.blocker()) return this.blocker();
    return this.voices().length === 0 ? 'Narration is available in English and Hindi. This story is in another language.' : '';
  });
  /** What the panel says when making videos is switched off on this site. */
  protected readonly offProblem = describeVideoProblem(
    new HttpErrorResponse({ status: 503, error: { code: 'UNAVAILABLE' } }),
    'make',
  );
  protected readonly failureProblem = computed<StoryPlanProblem>(() => {
    const v = this.latest();
    return {
      kind: 'failed',
      title: failureTitle(v),
      message: v?.error || 'Something went wrong on our side. Try again.',
      retryable: true,
    };
  });
  protected readonly sceneToFix = computed(() => {
    const v = this.latest();
    return v && v.errorCode && PICTURE_ERROR_CODES.includes(v.errorCode) ? sceneInMessage(v.error) : null;
  });

  private pollSub?: Subscription;
  private pollFailures = 0;
  private requests: Subscription[] = [];
  private lastStatus = '';

  constructor() {
    effect(() => {
      const id = this.planId();
      untracked(() => this.load(id));
    });
  }

  ngOnDestroy(): void {
    this.pollSub?.unsubscribe();
    this.requests.forEach((s) => s.unsubscribe());
  }

  protected reload(): void {
    this.load(this.planId());
  }

  // ---------------------------------------------------------------------------
  // Loading, and following a video that is being made

  private load(planId: string): void {
    this.pollSub?.unsubscribe();
    this.loading.set(true);
    this.loadProblem.set(null);
    this.requests.push(
      this.service.getLimits().subscribe({
        next: (l) => {
          if (l && typeof l.perDay === 'number') this.limits.set(l);
          this.pickDefaultVoice();
        },
        error: () => this.pickDefaultVoice(),
      }),
      this.service.listForPlan(planId).subscribe({
        next: (list) => {
          this.videos.set(list.videos ?? []);
          this.loading.set(false);
          const v = this.latest();
          if (v) this.rememberChoices(v);
          if (v && isActive(v.status)) this.follow(v);
        },
        error: (err) => {
          this.loading.set(false);
          this.loadProblem.set(describeVideoProblem(err, 'load'));
        },
      }),
    );
  }

  private pickDefaultVoice(): void {
    const voices = this.voices();
    if (voices.length > 0 && !voices.includes(this.options().voice)) {
      this.options.update((o) => ({ ...o, voice: voices[0] }));
    }
  }

  /** A retry starts from what the last attempt used. */
  private rememberChoices(v: StoryVideo): void {
    const preset = CAPTION_PRESETS.find((p) => p.id === v.caption?.preset)?.id ?? DEFAULT_CAPTION_PRESET;
    const voice = this.voices().includes(v.voice) ? v.voice : this.options().voice;
    this.options.set({ voice, caption: preset });
  }

  private follow(v: StoryVideo): void {
    this.pollFailures = 0;
    this.reconnecting.set(false);
    this.pollProblem.set(null);
    this.lastStatus = v.status;
    this.schedule(v.jobId, this.pollMs());
  }

  private schedule(jobId: string, delay: number): void {
    this.pollSub?.unsubscribe();
    this.pollSub = timer(delay)
      .pipe(switchMap(() => this.service.get(jobId)))
      .subscribe({
        next: (v) => {
          this.pollFailures = 0;
          this.reconnecting.set(false);
          this.pollProblem.set(null);
          this.replace(v);
          if (v.status !== this.lastStatus) {
            this.lastStatus = v.status;
            this.say(v);
          }
          if (isActive(v.status)) this.schedule(jobId, this.pollMs());
        },
        error: (err) => {
          if (err?.status === 404) {
            // Deleted somewhere else.
            this.videos.update((list) => list.filter((x) => x.jobId !== jobId));
            return;
          }
          this.pollFailures += 1;
          if (this.pollFailures >= MAX_POLL_FAILURES) {
            this.reconnecting.set(false);
            this.pollProblem.set(describeVideoProblem(err, 'check'));
            return;
          }
          this.reconnecting.set(true);
          this.schedule(jobId, Math.min(this.pollMs() * 2 ** this.pollFailures, MAX_POLL_DELAY_MS));
        },
      });
  }

  protected checkAgain(v: StoryVideo): void {
    this.follow(v);
  }

  /** Keeps a video's place in the list and swaps in the newer copy. */
  private replace(v: StoryVideo): void {
    this.videos.update((list) => (list.some((x) => x.jobId === v.jobId) ? list.map((x) => (x.jobId === v.jobId ? v : x)) : [v, ...list]));
  }

  private say(v: StoryVideo): void {
    if (v.status === 'completed') this.announcement.set('Your video is ready.');
    else if (v.status === 'failed') {
      this.justFailed.set(true);
      this.announcement.set(wasCancelled(v) ? 'The video was cancelled.' : `The video was not finished. ${v.error ?? ''}`.trim());
    }
  }

  // ---------------------------------------------------------------------------
  // Making a video

  protected make(): void {
    if (this.reason() || this.making()) return;
    this.makeProblem.set(null);
    this.actionProblem.set(null);
    this.making.set(true);
    const { voice, caption } = this.options();
    this.requests.push(
      this.service.create({ planId: this.planId(), revision: this.revision(), voice, caption: { preset: caption } }).subscribe({
        next: (v) => {
          this.making.set(false);
          this.showForm.set(false);
          this.justFailed.set(false);
          this.followNotice.set('');
          this.replace(v);
          this.follow(v);
          this.focusState();
        },
        error: (err) => {
          this.making.set(false);
          const problem = describeVideoProblem(err, 'make');
          if (problem.kind === 'already_running' && problem.jobId) {
            this.followExisting(problem.jobId);
            return;
          }
          this.makeProblem.set(problem);
        },
      }),
    );
  }

  /** The plan already has a video in the making (another tab): show that one. */
  private followExisting(jobId: string): void {
    this.requests.push(
      this.service.get(jobId).subscribe({
        next: (v) => {
          this.showForm.set(false);
          this.replace(v);
          this.followNotice.set('A video was already being made from this plan, so this is that one.');
          if (isActive(v.status)) this.follow(v);
          this.focusState();
        },
        error: (err) => this.makeProblem.set(describeVideoProblem(err, 'make')),
      }),
    );
  }

  protected makeActionLabel(p: StoryPlanProblem): string {
    if (p.kind === 'stale') return 'Reload the plan';
    return p.retryable ? 'Try again' : '';
  }

  protected onMakeProblemAction(p: StoryPlanProblem): void {
    if (p.kind === 'stale') {
      this.makeProblem.set(null);
      this.planChanged.emit();
    } else {
      this.make();
    }
  }

  protected openForm(): void {
    this.showForm.set(true);
  }

  protected closeForm(): void {
    this.showForm.set(false);
    this.makeProblem.set(null);
  }

  // ---------------------------------------------------------------------------
  // Cancel, download, watch, delete

  protected cancel(v: StoryVideo): void {
    if (this.cancelling()) return;
    this.cancelling.set(true);
    this.actionProblem.set(null);
    this.requests.push(
      this.service.cancel(v.jobId).subscribe({
        next: (updated) => {
          this.cancelling.set(false);
          this.pollSub?.unsubscribe();
          this.reconnecting.set(false);
          this.pollProblem.set(null);
          this.lastStatus = updated.status;
          this.replace(updated);
          this.say(updated);
          this.focusState();
        },
        error: (err) => {
          this.cancelling.set(false);
          this.actionProblem.set({ jobId: v.jobId, problem: describeVideoProblem(err, 'cancel') });
        },
      }),
    );
  }

  /** Asks for a fresh link and calls `then` with it. Links expire, so none is kept. */
  private withLink(v: StoryVideo, action: 'download' | 'watch', context: VideoContext, then: (url: string, filename: string) => void): void {
    if (this.busyId()) return;
    this.busyId.set(v.jobId);
    this.busyAction.set(action);
    this.actionProblem.set(null);
    this.requests.push(
      this.service.download(v.jobId).subscribe({
        next: (d) => {
          this.busyId.set('');
          this.busyAction.set('');
          then(d.url, d.filename);
        },
        error: (err) => {
          this.busyId.set('');
          this.busyAction.set('');
          this.actionProblem.set({ jobId: v.jobId, problem: describeVideoProblem(err, context) });
        },
      }),
    );
  }

  protected download(v: StoryVideo): void {
    this.withLink(v, 'download', 'download', (url, filename) => {
      const a = this.document.createElement('a');
      a.href = url;
      a.download = filename;
      a.rel = 'noopener';
      a.style.display = 'none';
      this.document.body.appendChild(a);
      a.click();
      a.remove();
    });
  }

  protected toggleWatch(v: StoryVideo): void {
    if (this.watchId() === v.jobId) {
      this.watchId.set('');
      this.watchUrl.set('');
      return;
    }
    this.withLink(v, 'watch', 'download', (url) => {
      this.watchId.set(v.jobId);
      this.watchUrl.set(url);
    });
  }

  /** The link expired while the page was open, or the file is gone. */
  protected onWatchError(v: StoryVideo): void {
    this.watchId.set('');
    this.watchUrl.set('');
    this.actionProblem.set({
      jobId: v.jobId,
      problem: {
        kind: 'failed',
        title: "The video couldn't be played here",
        message: 'The link may have expired. Choose "Watch it here" again, or download the file.',
        retryable: true,
      },
    });
  }

  protected askDelete(v: StoryVideo): void {
    this.actionProblem.set(null);
    this.confirming.set(v.jobId);
    afterNextRender(() => this.document.getElementById(`keep-${v.jobId}`)?.focus(), { injector: this.injector });
  }

  protected cancelDelete(v: StoryVideo): void {
    this.confirming.set('');
    afterNextRender(() => this.document.getElementById(`delete-${v.jobId}`)?.focus(), { injector: this.injector });
  }

  /** `quiet` is for removing a failed attempt, which needs no confirmation: nothing is lost. */
  protected remove(v: StoryVideo, quiet: boolean): void {
    if (this.busyId()) return;
    this.busyId.set(v.jobId);
    this.busyAction.set('delete');
    this.actionProblem.set(null);
    this.requests.push(
      this.service.remove(v.jobId).subscribe({
        next: () => {
          this.busyId.set('');
          this.busyAction.set('');
          this.confirming.set('');
          if (this.watchId() === v.jobId) {
            this.watchId.set('');
            this.watchUrl.set('');
          }
          this.videos.update((list) => list.filter((x) => x.jobId !== v.jobId));
          this.showForm.set(false);
          this.announcement.set(quiet ? 'The attempt was removed.' : 'The video was deleted.');
          this.focusState();
        },
        error: (err) => {
          this.busyId.set('');
          this.busyAction.set('');
          this.actionProblem.set({ jobId: v.jobId, problem: describeVideoProblem(err, 'delete') });
        },
      }),
    );
  }

  // ---------------------------------------------------------------------------

  protected length(v: StoryVideo): string {
    return formatLength(v.totalSeconds ?? 0);
  }

  protected caption(v: StoryVideo): string {
    return captionLabel(v.caption);
  }

  /** Made from an earlier version of the plan than the one on screen. */
  protected isStale(v: StoryVideo): boolean {
    return v.planRevision !== this.revision();
  }

  /** After the person's own action changes what is on screen, put focus on the new heading. */
  private focusState(): void {
    afterNextRender(
      () => {
        const root = this.host.nativeElement;
        (root.querySelector<HTMLElement>('[data-state-heading]') ?? root.querySelector<HTMLElement>('#video-heading'))?.focus();
      },
      { injector: this.injector },
    );
  }
}

function failureTitle(v: StoryVideo | null): string {
  switch (v?.errorCode) {
    case 'TIMED_OUT':
      return 'The video took too long';
    case 'NARRATION_TOO_LONG':
      return 'The narration is too long';
    case 'MEDIA_UNAVAILABLE':
    case 'MEDIA_INVALID':
      return 'A picture can no longer be used';
    default:
      return "The video wasn't finished";
  }
}
