import { Component, OnInit, OnDestroy, AfterViewInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { Subscription } from 'rxjs';
import { LucideDynamicIcon } from '@lucide/angular';
import gsap from 'gsap';
import { ProjectService } from '../../core';
import { PipelineProgress, ProjectStatus } from '../../shared';

interface PipelineStep {
  label: string;
  status: ProjectStatus;
}

@Component({
  selector: 'app-progress',
  standalone: true,
  imports: [CommonModule, LucideDynamicIcon],
  template: `
    <div class="max-w-3xl mx-auto px-4 py-12">
      <h1 class="text-2xl font-bold text-center mb-2">Processing Your Video</h1>
      <p class="text-center text-[hsl(var(--muted-foreground))] mb-10">
        Your project is being processed. This may take a few minutes.
      </p>

      <!-- Pipeline Stepper -->
      <div class="glass-card p-8 mb-8">
        <div class="flex items-center justify-between" role="progressbar" [attr.aria-valuenow]="currentStepIndex" [attr.aria-valuemin]="0" [attr.aria-valuemax]="steps.length - 1">
          @for (step of steps; track step.status; let i = $index; let last = $last) {
            <div class="flex items-center" [class.flex-1]="!last">
              <!-- Step Circle -->
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

              <!-- Connector Line -->
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

      <!-- Progress Bar (rendering stage only) -->
      @if (progress && progress.stage === 'rendering') {
        <div class="glass-card p-6 mb-8">
          <div class="flex justify-between items-center mb-3">
            <span class="text-sm font-medium">Rendering Video</span>
            <span class="text-sm font-medium text-[hsl(var(--primary))]">{{ progress.percentage }}%</span>
          </div>
          <div class="h-2 rounded-full bg-[hsl(var(--secondary))] overflow-hidden">
            <div
              class="h-full rounded-full bg-gradient-to-r from-[hsl(var(--primary))] to-purple-400 transition-all duration-500"
              [style.width.%]="progress.percentage"
            ></div>
          </div>
          @if (progress!.slidesProcessed != null && progress!.slidesTotal != null) {
            <p class="mt-3 text-sm text-[hsl(var(--muted-foreground))] text-center">
              Processing slide {{ progress!.slidesProcessed }} of {{ progress!.slidesTotal }}
            </p>
          }
        </div>
      }

      <!-- Completion Message -->
      @if (progress?.stage === 'completed') {
        <div class="text-center glass-card p-8 !border-green-500/30">
          <svg lucideIcon="circle-check" [size]="48" class="mx-auto mb-3 text-green-400"></svg>
          <h2 class="text-lg font-semibold text-green-400">Video Ready!</h2>
          <p class="text-sm text-[hsl(var(--muted-foreground))] mt-1">Redirecting to preview...</p>
        </div>
      }

      <!-- Error Message -->
      @if (progress?.stage === 'failed') {
        <div class="text-center glass-card p-8 !border-red-500/30">
          <svg lucideIcon="circle-x" [size]="48" class="mx-auto mb-3 text-red-400"></svg>
          <h2 class="text-lg font-semibold text-red-400">Processing Failed</h2>
          <p class="text-sm text-[hsl(var(--muted-foreground))] mt-2">
            Something went wrong while processing your video. Please try again.
          </p>
          <button class="glow-btn mt-4" (click)="goToProject()">
            Back to Project
          </button>
        </div>
      }
    </div>
  `,
})
export class ProgressComponent implements OnInit, OnDestroy {
  private pollSub: Subscription | null = null;
  private redirectTimeout: ReturnType<typeof setTimeout> | null = null;
  private projectId = '';

  progress: PipelineProgress | null = null;
  currentStepIndex = 0;

  steps: PipelineStep[] = [
    { label: 'Parsing', status: 'parsing' },
    { label: 'Generating Slides', status: 'generating_slides' },
    { label: 'Narrating', status: 'narrating' },
    { label: 'Rendering', status: 'rendering' },
    { label: 'Complete', status: 'completed' },
  ];

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
        this.currentStepIndex = this.getStepIndexForStatus(progress.stage);

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
    const index = this.steps.findIndex((s) => s.status === status);
    return index >= 0 ? index : 0;
  }
}
