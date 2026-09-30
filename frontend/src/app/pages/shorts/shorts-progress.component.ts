import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { Subscription, interval } from 'rxjs';
import { startWith, switchMap } from 'rxjs/operators';
import { LucideDynamicIcon } from '@lucide/angular';
import { ShortsService, ShortsStatus } from './shorts.service';

const POLL_INTERVAL_MS = 3000;

interface PipelineStep {
  label: string;
  status: ShortsStatus;
}

@Component({
  selector: 'app-shorts-progress',
  standalone: true,
  imports: [CommonModule, LucideDynamicIcon],
  template: `
    <div class="max-w-3xl mx-auto px-4 py-12">
      <h1 class="text-2xl font-bold text-center mb-2">Generating Your Shorts</h1>
      <p class="text-center text-[hsl(var(--muted-foreground))] mb-10">
        We're analyzing your media and rendering the best moments. This may take a few minutes.
      </p>

      <!-- Pipeline Stepper -->
      <div class="glass-card p-8 mb-8">
        <div
          class="flex items-center justify-between"
          role="progressbar"
          [attr.aria-valuenow]="currentStepIndex"
          [attr.aria-valuemin]="0"
          [attr.aria-valuemax]="steps.length - 1"
        >
          @for (step of steps; track step.status; let i = $index; let last = $last) {
            <div class="flex items-center" [class.flex-1]="!last">
              <div class="flex flex-col items-center">
                <div
                  class="w-10 h-10 rounded-full flex items-center justify-center border-2 transition-all"
                  [ngClass]="{
                    'border-green-500 bg-green-500 text-white': getStepState(i) === 'completed',
                    'border-[hsl(var(--primary))] bg-[hsl(var(--primary))]/10': getStepState(i) === 'active',
                    'border-[hsl(var(--border))] text-[hsl(var(--muted-foreground))]': getStepState(i) === 'pending',
                    'border-red-500 bg-red-500/10': getStepState(i) === 'failed'
                  }"
                >
                  @if (getStepState(i) === 'completed') {
                    <svg lucideIcon="check" [size]="18"></svg>
                  } @else if (getStepState(i) === 'active') {
                    <div class="w-3 h-3 rounded-full bg-[hsl(var(--primary))] animate-ping"></div>
                  } @else if (getStepState(i) === 'failed') {
                    <svg lucideIcon="x" [size]="18" class="text-red-500"></svg>
                  } @else {
                    <span class="text-xs font-medium">{{ i + 1 }}</span>
                  }
                </div>
                <span
                  class="mt-2 text-xs font-medium text-center max-w-[80px]"
                  [ngClass]="{
                    'text-green-400': getStepState(i) === 'completed',
                    'text-[hsl(var(--primary))]': getStepState(i) === 'active',
                    'text-[hsl(var(--muted-foreground))]': getStepState(i) === 'pending',
                    'text-red-400': getStepState(i) === 'failed'
                  }"
                >
                  {{ step.label }}
                </span>
              </div>

              @if (!last) {
                <div
                  class="flex-1 h-0.5 mx-2 mt-[-20px] rounded-full"
                  [ngClass]="{
                    'bg-green-500': getStepState(i) === 'completed',
                    'bg-[hsl(var(--primary))]/40': getStepState(i) === 'active',
                    'bg-[hsl(var(--border))]': getStepState(i) === 'pending' || getStepState(i) === 'failed'
                  }"
                ></div>
              }
            </div>
          }
        </div>
      </div>

      <!-- Completion Message -->
      @if (status === 'completed') {
        <div class="text-center glass-card p-8 !border-green-500/30">
          <svg lucideIcon="circle-check" [size]="48" class="mx-auto mb-3 text-green-400"></svg>
          <h2 class="text-lg font-semibold text-green-400">Shorts Ready!</h2>
          <p class="text-sm text-[hsl(var(--muted-foreground))] mt-1">Redirecting to your clips…</p>
        </div>
      }

      <!-- Error Message -->
      @if (status === 'failed') {
        <div class="text-center glass-card p-8 !border-red-500/30">
          <svg lucideIcon="circle-x" [size]="48" class="mx-auto mb-3 text-red-400"></svg>
          <h2 class="text-lg font-semibold text-red-400">Processing Failed</h2>
          <p class="text-sm text-[hsl(var(--muted-foreground))] mt-2">
            {{ errorMessage || 'Something went wrong while processing your media. Please try again.' }}
          </p>
          <button class="glow-btn mt-4" (click)="goToUpload()">Start Over</button>
        </div>
      }
    </div>
  `,
})
export class ShortsProgressComponent implements OnInit, OnDestroy {
  private pollSub: Subscription | null = null;
  private redirectTimeout: ReturnType<typeof setTimeout> | null = null;
  private jobId = '';

  status: ShortsStatus | null = null;
  errorMessage = '';
  currentStepIndex = 0;

  steps: PipelineStep[] = [
    { label: 'Transcribing', status: 'transcribing' },
    { label: 'Ranking', status: 'ranking' },
    { label: 'Rendering', status: 'rendering' },
    { label: 'Complete', status: 'completed' },
  ];

  constructor(
    private readonly route: ActivatedRoute,
    private readonly router: Router,
    private readonly shorts: ShortsService,
  ) {}

  ngOnInit(): void {
    this.jobId = this.route.snapshot.paramMap.get('id') || '';
    if (!this.jobId) {
      this.router.navigate(['/shorts']);
      return;
    }

    this.pollSub = interval(POLL_INTERVAL_MS)
      .pipe(
        startWith(0),
        switchMap(() => this.shorts.getStatus(this.jobId)),
      )
      .subscribe({
        next: (job) => {
          this.status = job.status;
          this.errorMessage = job.error ?? '';
          this.currentStepIndex = this.getStepIndexForStatus(job.status);

          if (job.status === 'completed') {
            this.pollSub?.unsubscribe();
            this.redirectTimeout = setTimeout(() => {
              this.router.navigate(['/shorts', this.jobId, 'clips']);
            }, 2000);
          } else if (job.status === 'failed') {
            this.pollSub?.unsubscribe();
          }
        },
        error: () => {
          this.status = 'failed';
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
    if (this.status === 'failed') {
      if (index < this.currentStepIndex) return 'completed';
      if (index === this.currentStepIndex) return 'failed';
      return 'pending';
    }
    if (index < this.currentStepIndex) return 'completed';
    if (index === this.currentStepIndex) return 'active';
    return 'pending';
  }

  goToUpload(): void {
    this.router.navigate(['/shorts']);
  }

  private getStepIndexForStatus(status: ShortsStatus): number {
    // 'uploaded' precedes the first visible step; keep the stepper at the start.
    if (status === 'uploaded') return 0;
    const index = this.steps.findIndex((s) => s.status === status);
    return index >= 0 ? index : 0;
  }
}
