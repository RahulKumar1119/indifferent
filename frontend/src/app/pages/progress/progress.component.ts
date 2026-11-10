import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { Subscription } from 'rxjs';
import { LucideDynamicIcon } from '@lucide/angular';
import { ProjectService } from '../../core';
import { PipelineProgress, ProjectStatus } from '../../shared';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { FORMAT_GUIDE_PATH, SAMPLE_FILE_PATH, STORY_PATH, failureMessage } from './failure-message';

interface PipelineStep {
  label: string;
  status: ProjectStatus;
}

@Component({
  selector: 'app-progress',
  standalone: true,
  imports: [CommonModule, LucideDynamicIcon, RouterLink, NavbarComponent],
  styles: [`
    :host { display: block; }
    .narrow { width: min(44rem, 100%); }
    .back { display: inline-flex; align-items: center; min-height: 44px; font-size: var(--type-sm); }
    .sr { position: absolute; width: 1px; height: 1px; margin: -1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
    .steps { margin: 0; padding: 0; list-style: none; display: flex; flex-direction: column; }
    .steps li { position: relative; display: flex; align-items: center; gap: var(--space-sm); min-height: 3rem; color: var(--color-muted); }
    .steps li + li::before { content: ''; position: absolute; left: calc(1rem - 1px); top: -0.75rem; height: 1.5rem; width: 2px; background: var(--color-rule); }
    .steps li.completed + li::before { background: var(--color-positive); }
    .dot { position: relative; z-index: 1; width: 2rem; height: 2rem; flex: none; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; border: 1px solid var(--color-muted); background: var(--app-card); font-size: var(--type-xs); font-weight: 600; }
    .completed { color: var(--color-ink-2); }
    .completed .dot { background: var(--color-positive); border-color: var(--color-positive); color: var(--color-paper); }
    .active { color: var(--color-ink); font-weight: 600; }
    .active .dot { border-color: var(--color-accent); color: var(--color-accent-text); }
    .failed { color: var(--color-negative); font-weight: 600; }
    .failed .dot { border-color: var(--color-negative); color: var(--color-negative); }
    .bar-head { margin-top: var(--space-md); display: flex; justify-content: space-between; font-size: var(--type-sm); font-weight: 600; }
    .bar { margin-top: var(--space-xs); height: 0.5rem; border-radius: var(--radius-pill); background: var(--color-paper-3); overflow: hidden; }
    .bar > span { display: block; height: 100%; background: var(--color-accent); transition: width var(--dur-short) var(--ease-out); }
    .slides { margin: var(--space-xs) 0 0; font-size: var(--type-sm); color: var(--color-ink-2); }
    .state { display: flex; flex-direction: column; align-items: flex-start; gap: var(--space-xs); }
    .state h2 { margin: 0; display: flex; align-items: center; gap: var(--space-xs); font-size: var(--type-lg); font-weight: 600; }
    .state p { margin: 0; color: var(--color-ink-2); }
    .ok h2 { color: var(--color-positive); }
    .bad h2 { color: var(--color-negative); }
    .links { display: flex; flex-wrap: wrap; gap: 0 var(--space-lg); }
    .links a, .hint a { display: inline-flex; align-items: center; min-height: 44px; font-weight: 600; }
    @media (prefers-reduced-motion: reduce) { .bar > span { transition: none; } }
  `],
  template: `
    <div class="ui-app">
      <app-navbar />
      <main id="main" class="ui-app-main narrow hm">
        <div>
          <a [routerLink]="['/projects', projectId]" class="ui-link back">Back to the project</a>
          <h1 class="ui-h1" data-testid="heading">{{ heading }}</h1>
          <p class="ui-lead">{{ lead }}</p>
        </div>
        <p class="sr" role="status" aria-live="polite" data-testid="live">{{ liveText }}</p>

        @if (progress?.stage === 'failed') {
          <section class="ui-card ui-danger state bad" role="alert" aria-labelledby="fail-title">
            <h2 id="fail-title"><svg lucideIcon="circle-x" [size]="20" aria-hidden="true"></svg>{{ failure.title }}</h2>
            <p>{{ failure.text }}</p>
            <button type="button" class="ui-btn ui-btn--primary" (click)="goToProject()">
              {{ failure.fileProblem ? 'Choose another file' : 'Retry upload' }}
            </button>
            @if (failure.fileProblem) {
              <p class="links">
                <a [routerLink]="formatGuidePath" class="ui-link">Read the format guide</a>
                <a [href]="sampleFilePath" download="quiz-sample.txt" class="ui-link">Download a sample file</a>
              </p>
            }
            @if (failure.storyHint) {
              <p class="hint" data-testid="story-hint">
                Is your file a story or a script rather than questions?
                <a [routerLink]="storyPath" class="ui-link">Turn it into a Story video</a>
              </p>
            }
          </section>
        }

        @if (progress?.stage === 'completed') {
          <section class="ui-card state ok" aria-labelledby="ready-title" data-testid="ready">
            <h2 id="ready-title"><svg lucideIcon="circle-check" [size]="20" aria-hidden="true"></svg>Your video is ready</h2>
            <p>Opening the preview in a moment.</p>
            <a [routerLink]="['/projects', projectId, 'preview']" class="ui-btn ui-btn--primary">Watch and download</a>
          </section>
        }

        <section class="ui-card" aria-labelledby="steps-title">
          <h2 id="steps-title" class="ui-card-title">Steps</h2>
          <ol class="steps" aria-labelledby="steps-title" style="margin-top: var(--space-sm)">
            @for (step of steps; track step.status; let i = $index) {
              <li [class]="getStepState(i)" [attr.aria-current]="getStepState(i) === 'active' ? 'step' : null" [attr.data-testid]="'stage-' + step.status">
                <span class="dot" aria-hidden="true">
                  @switch (getStepState(i)) {
                    @case ('completed') { <svg lucideIcon="check" [size]="16"></svg> }
                    @case ('active') { <svg lucideIcon="loader-2" [size]="16" class="ui-spin"></svg> }
                    @case ('failed') { <svg lucideIcon="x" [size]="16"></svg> }
                    @default { {{ i + 1 }} }
                  }
                </span>
                <span>{{ step.label }}<span class="sr">, {{ stateWords[getStepState(i)] }}</span></span>
              </li>
            }
          </ol>

          @if (progress && progress.stage === 'rendering') {
            <div class="bar-head"><span>Rendering video</span><span>{{ progress.percentage }}%</span></div>
            <div class="bar" role="progressbar" aria-label="Rendering progress" aria-valuemin="0" aria-valuemax="100" [attr.aria-valuenow]="progress.percentage">
              <span [style.width.%]="progress.percentage"></span>
            </div>
            @if (progress!.slidesProcessed != null && progress!.slidesTotal != null) {
              <p class="slides">Slide {{ progress!.slidesProcessed }} of {{ progress!.slidesTotal }}</p>
            }
          }
        </section>
      </main>
    </div>
  `,
})
export class ProgressComponent implements OnInit, OnDestroy {
  private pollSub: Subscription | null = null;
  private redirectTimeout: ReturnType<typeof setTimeout> | null = null;
  projectId = '';

  progress: PipelineProgress | null = null;
  readonly formatGuidePath = FORMAT_GUIDE_PATH;
  readonly sampleFilePath = SAMPLE_FILE_PATH;
  readonly storyPath = STORY_PATH;

  get failure() {
    return failureMessage(this.progress?.failureCode);
  }
  currentStepIndex = 0;

  steps: PipelineStep[] = [
    { label: 'Reading questions', status: 'parsing' },
    { label: 'Making slides', status: 'generating_slides' },
    { label: 'Narrating', status: 'narrating' },
    { label: 'Rendering', status: 'rendering' },
    { label: 'Done', status: 'completed' },
  ];

  /** Spoken after each step name, since its state is otherwise only shown by colour and icon. */
  readonly stateWords = { completed: 'done', active: 'in progress', pending: 'not started', failed: 'failed' } as const;

  /** The page heading follows the real state, so a failure never sits under "Making your video". */
  get heading(): string {
    if (this.progress?.stage === 'failed') return 'Something went wrong';
    if (this.progress?.stage === 'completed') return 'Video ready';
    return 'Making your video';
  }

  get lead(): string {
    if (this.progress?.stage === 'failed') {
      return this.failure.fileProblem ? 'The file needs a fix before the video can be made.' : 'Your video could not be finished.';
    }
    if (this.progress?.stage === 'completed') return 'Your quiz video has finished rendering.';
    return 'This usually takes a few minutes. It keeps going if you leave this page.';
  }

  /** What the polite live region announces as the stage changes. */
  get liveText(): string {
    const stage = this.progress?.stage;
    if (!stage) return '';
    if (stage === 'failed') return 'Something went wrong. ' + this.failure.title;
    if (stage === 'completed') return 'Video ready.';
    const step = this.steps[this.currentStepIndex];
    return step ? `${step.label}, in progress.` : '';
  }

  constructor(
    private readonly route: ActivatedRoute,
    private readonly router: Router,
    private readonly projectService: ProjectService,
  ) {}

  ngOnInit(): void {
    this.projectId = this.route.snapshot.paramMap.get('id') || '';
    if (!this.projectId) {
      this.router.navigate(['/projects']);
      return;
    }

    this.pollSub = this.projectService.pollProjectStatus(this.projectId).subscribe({
      next: (progress) => {
        this.progress = progress;
        // Unknown stages ('created', 'failed') must not reset the stepper to
        // Parsing: keep the furthest reached step so failures are attributed
        // to the stage that was actually running.
        const idx = this.getStepIndexForStatus(progress.stage);
        if (idx >= 0) {
          this.currentStepIndex = idx;
        }

        if (progress.stage === 'completed') {
          this.redirectTimeout = setTimeout(() => {
            this.router.navigate(['/projects', this.projectId, 'preview']);
          }, 2000);
        }
      },
      error: () => {
        this.progress = { stage: 'failed', percentage: 0 };
      },
    });
  }

  ngOnDestroy(): void {
    this.pollSub?.unsubscribe();
    if (this.redirectTimeout !== null) {
      clearTimeout(this.redirectTimeout);
    }
  }

  getStepState(index: number): 'completed' | 'active' | 'pending' | 'failed' {
    if (this.progress?.stage === 'failed') {
      if (index < this.currentStepIndex) return 'completed';
      if (index === this.currentStepIndex) return 'failed';
      return 'pending';
    }

    if (index < this.currentStepIndex) return 'completed';
    if (index === this.currentStepIndex) return 'active';
    return 'pending';
  }

  goToProject(): void {
    this.router.navigate(['/projects', this.projectId, 'upload']);
  }

  private getStepIndexForStatus(status: ProjectStatus): number {
    // -1 for stages with no step (created, failed): callers keep the
    // furthest reached index instead of falling back to Parsing.
    return this.steps.findIndex((s) => s.status === status);
  }
}
