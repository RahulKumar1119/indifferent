import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonModule } from '@angular/material/button';
import { Subscription } from 'rxjs';
import { ProjectService } from '../../core';
import { PipelineProgress, ProjectStatus } from '../../shared';

interface PipelineStep {
  label: string;
  status: ProjectStatus;
}

@Component({
  selector: 'app-progress',
  standalone: true,
  imports: [CommonModule, MatProgressBarModule, MatIconModule, MatButtonModule],
  template: `
    <div class="max-w-3xl mx-auto px-4 py-12">
      <h1 class="text-2xl font-bold text-center mb-2">Processing Your Video</h1>
      <p class="text-center text-gray-500 dark:text-gray-400 mb-10">
        Your project is being processed. This may take a few minutes.
      </p>

      <!-- Pipeline Stepper -->
      <div class="flex items-center justify-between mb-12" role="progressbar" [attr.aria-valuenow]="currentStepIndex" [attr.aria-valuemin]="0" [attr.aria-valuemax]="steps.length - 1">
        @for (step of steps; track step.status; let i = $index; let last = $last) {
          <div class="flex items-center" [class.flex-1]="!last">
            <!-- Step Circle -->
            <div class="flex flex-col items-center">
              <div
                class="w-10 h-10 rounded-full flex items-center justify-center border-2 transition-all"
                [class.border-green-500]="getStepState(i) === 'completed'"
                [class.bg-green-500]="getStepState(i) === 'completed'"
                [class.text-white]="getStepState(i) === 'completed'"
                [class.border-blue-500]="getStepState(i) === 'active'"
                [class.bg-blue-50]="getStepState(i) === 'active'"
                [class.dark:bg-blue-900/30]="getStepState(i) === 'active'"
                [class.animate-pulse]="getStepState(i) === 'active'"
                [class.border-gray-300]="getStepState(i) === 'pending'"
                [class.dark:border-gray-600]="getStepState(i) === 'pending'"
                [class.text-gray-400]="getStepState(i) === 'pending'"
                [class.dark:text-gray-500]="getStepState(i) === 'pending'"
                [class.border-red-500]="getStepState(i) === 'failed'"
                [class.bg-red-50]="getStepState(i) === 'failed'"
                [class.dark:bg-red-900/30]="getStepState(i) === 'failed'"
              >
                @if (getStepState(i) === 'completed') {
                  <mat-icon class="text-sm !w-5 !h-5 !text-[20px]">check</mat-icon>
                } @else if (getStepState(i) === 'active') {
                  <div class="w-3 h-3 rounded-full bg-blue-500 animate-ping"></div>
                } @else if (getStepState(i) === 'failed') {
                  <mat-icon class="text-sm !w-5 !h-5 !text-[20px] text-red-500">close</mat-icon>
                } @else {
                  <span class="text-xs font-medium">{{ i + 1 }}</span>
                }
              </div>
              <span
                class="mt-2 text-xs font-medium text-center max-w-[80px]"
                [class.text-green-600]="getStepState(i) === 'completed'"
                [class.dark:text-green-400]="getStepState(i) === 'completed'"
                [class.text-blue-600]="getStepState(i) === 'active'"
                [class.dark:text-blue-400]="getStepState(i) === 'active'"
                [class.text-gray-400]="getStepState(i) === 'pending'"
                [class.dark:text-gray-500]="getStepState(i) === 'pending'"
                [class.text-red-600]="getStepState(i) === 'failed'"
                [class.dark:text-red-400]="getStepState(i) === 'failed'"
              >
                {{ step.label }}
              </span>
            </div>

            <!-- Connector Line -->
            @if (!last) {
              <div
                class="flex-1 h-0.5 mx-2 mt-[-20px]"
                [class.bg-green-500]="getStepState(i) === 'completed'"
                [class.bg-blue-300]="getStepState(i) === 'active'"
                [class.bg-gray-300]="getStepState(i) === 'pending' || getStepState(i) === 'failed'"
                [class.dark:bg-gray-600]="getStepState(i) === 'pending' || getStepState(i) === 'failed'"
              ></div>
            }
          </div>
        }
      </div>

      <!-- Progress Bar (rendering stage only) -->
      @if (progress && progress.stage === 'rendering') {
        <div class="mb-8">
          <div class="flex justify-between items-center mb-2">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">Rendering Video</span>
            <span class="text-sm font-medium text-blue-600 dark:text-blue-400">{{ progress.percentage }}%</span>
          </div>
          <mat-progress-bar
            mode="determinate"
            [value]="progress.percentage"
            class="rounded-full"
          ></mat-progress-bar>
          @if (progress!.slidesProcessed != null && progress!.slidesTotal != null) {
            <p class="mt-2 text-sm text-gray-500 dark:text-gray-400 text-center">
              Processing slide {{ progress!.slidesProcessed }} of {{ progress!.slidesTotal }}
            </p>
          }
        </div>
      }

      <!-- Completion Message -->
      @if (progress?.stage === 'completed') {
        <div class="text-center p-6 bg-green-50 dark:bg-green-900/20 rounded-lg">
          <mat-icon class="!text-[48px] !w-12 !h-12 text-green-500 mb-3">check_circle</mat-icon>
          <h2 class="text-lg font-semibold text-green-700 dark:text-green-300">Video Ready!</h2>
          <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">Redirecting to preview...</p>
        </div>
      }

      <!-- Error Message -->
      @if (progress?.stage === 'failed') {
        <div class="text-center p-6 bg-red-50 dark:bg-red-900/20 rounded-lg">
          <mat-icon class="!text-[48px] !w-12 !h-12 text-red-500 mb-3">error</mat-icon>
          <h2 class="text-lg font-semibold text-red-700 dark:text-red-300">Processing Failed</h2>
          <p class="text-sm text-gray-600 dark:text-gray-400 mt-2">
            Something went wrong while processing your video. Please try again.
          </p>
          <button
            mat-flat-button
            color="primary"
            class="mt-4"
            (click)="goToProject()"
          >
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
