import { Component, ElementRef, OnDestroy, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { Subscription, interval } from 'rxjs';
import { startWith, switchMap } from 'rxjs/operators';
import gsap from 'gsap';
import { ShortsJob, ShortsService, ShortsStatus } from './shorts.service';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { ToastsComponent } from '../../shared/components/toast/toasts.component';
import { ToastService } from '../../shared/components/toast/toast.service';

const POLL_INTERVAL_MS = 3000;

interface Stage {
  key: string;
  label: string;
  statuses: ShortsStatus[];
}

const STAGES: Stage[] = [
  { key: 'upload', label: 'Upload', statuses: [] },
  { key: 'transcribing', label: 'Transcribing', statuses: ['uploaded', 'transcribing'] },
  { key: 'ranking', label: 'Ranking', statuses: ['ranking'] },
  { key: 'rendering', label: 'Rendering', statuses: ['rendering'] },
];

@Component({
  selector: 'app-shorts-progress',
  standalone: true,
  imports: [CommonModule, LucideDynamicIcon, NavbarComponent, ToastsComponent],
  styles: [`
    @keyframes eq-bounce {
      0%, 100% { transform: scaleY(0.3); }
      50% { transform: scaleY(1); }
    }
    .eq-bar {
      transform-origin: bottom;
      animation: eq-bounce 1.1s ease-in-out infinite;
    }
    .stage-bar { transition: width .6s cubic-bezier(0.23,1,0.32,1); }
    @media (prefers-reduced-motion: reduce) {
      .eq-bar { animation: none; transform: none; }
    }
  `],
  template: `
    <app-navbar></app-navbar>
    <app-toasts></app-toasts>
    <div class="max-w-3xl mx-auto px-4 py-12">
      <h1 class="text-2xl font-bold text-center mb-2">Generating Your Shorts</h1>
      <p class="text-center text-[hsl(var(--muted-foreground))] mb-8">
        @if (eta && (status === 'transcribing' || status === 'ranking' || status === 'rendering')) {
          {{ eta }} — feel free to leave, we'll keep working.
        } @else {
          We're analyzing your media and rendering the best moments.
        }
      </p>

      <!-- Resume toast -->
      @if (resumed) {
        <div class="glass-card p-3 mb-6 !border-[hsl(var(--primary))]/40">
          <p class="text-sm flex items-center gap-2">
            <svg lucideIcon="clock" [size]="16" class="text-[hsl(var(--primary))]"></svg>
            Your shorts are still processing — you can navigate away and come back anytime.
          </p>
        </div>
      }

      <!-- Vertical stepper -->
      <div class="glass-card p-6 md:p-8 mb-6" role="progressbar" aria-label="Processing pipeline">
        @for (stage of stages; track stage.key; let i = $index; let last = $last) {
          <div class="flex gap-4" [class.pb-8]="!last">
            <div class="flex flex-col items-center">
              <div
                class="relative w-10 h-10 rounded-full flex items-center justify-center border-2 transition-all shrink-0"
                [ngClass]="{
                  'border-green-500 bg-green-500 text-white': stageState(i) === 'done',
                  'border-[hsl(var(--primary))] bg-[hsl(var(--primary))]/10': stageState(i) === 'active',
                  'border-[hsl(var(--border))] text-[hsl(var(--muted-foreground))]': stageState(i) === 'pending'
                }"
              >
                @if (stageState(i) === 'active') {
                  <span class="absolute inset-0 rounded-full border-2 border-[hsl(var(--primary))] animate-ping opacity-40"></span>
                  <span class="w-3 h-3 rounded-full bg-[hsl(var(--primary))]"></span>
                } @else if (stageState(i) === 'done') {
                  <svg lucideIcon="check" [size]="18" [attr.data-stage-check]="stage.key"></svg>
                } @else {
                  <span class="text-xs font-medium">{{ i + 1 }}</span>
                }
              </div>
              @if (!last) {
                <div
                  class="flex-1 w-0.5 my-1 rounded-full min-h-[28px]"
                  [ngClass]="stageState(i) === 'done' ? 'bg-green-500' : 'bg-[hsl(var(--border))]'"
                ></div>
              }
            </div>
            <div class="flex-1 min-w-0">
              <div class="flex items-center justify-between gap-3">
                <p
                  class="font-medium"
                  [ngClass]="{
                    'text-green-400': stageState(i) === 'done',
                    'text-[hsl(var(--primary))]': stageState(i) === 'active'
                  }"
                >
                  {{ stage.label }}
                </p>
                @if (stageState(i) === 'active') {
                  <span class="text-xs text-[hsl(var(--muted-foreground))] tabular-nums">{{ stagePercent(i) }}%</span>
                }
                @if (stageState(i) === 'done') {
                  <svg lucideIcon="check" [size]="14" class="text-green-400"></svg>
                }
              </div>
              @if (stageState(i) === 'active') {
                <div class="relative mt-2 h-2 rounded-full bg-[hsl(var(--secondary))] overflow-hidden">
                  <div
                    class="stage-bar h-full rounded-full bg-gradient-to-r from-[hsl(var(--primary))] to-purple-400"
                    [style.width.%]="stagePercent(i)"
                  ></div>
                  <div class="absolute inset-0 flex items-end justify-center gap-[3px] pb-[3px] opacity-40" aria-hidden="true">
                    @for (b of eqBars; track b) {
                      <span class="eq-bar w-[3px] rounded-full bg-white" [style.height.%]="b.h" [style.animation-delay]="b.d"></span>
                    }
                  </div>
                </div>
              }
            </div>
          </div>
        }

        @if (cancelling || cancelDone) {
          <p class="mt-4 text-center text-sm text-[hsl(var(--muted-foreground))]">
            {{ cancelDone ? 'Cancellation requested…' : 'Cancelling…' }}
          </p>
        } @else if (status && status !== 'completed' && status !== 'failed') {
          <div class="mt-6 text-center">
            <button
              class="btn-interactive px-4 py-2 rounded-lg border border-red-500/30 text-red-400 hover:bg-red-500/10 transition-colors text-sm"
              [disabled]="cancelling"
              (click)="cancel()"
            >
              Cancel processing
            </button>
          </div>
        }
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
  private furthest = 0;

  status: ShortsStatus | null = null;
  progress = 0;
  durationSeconds = 0;
  eta = '';
  errorMessage = '';
  resumed = false;
  cancelling = false;
  cancelDone = false;

  stages: Stage[] = STAGES;
  eqBars = [
    { h: 70, d: '0s' },
    { h: 100, d: '0.15s' },
    { h: 55, d: '0.3s' },
    { h: 85, d: '0.45s' },
    { h: 60, d: '0.6s' },
  ];
  constructor(
    private readonly route: ActivatedRoute,
    private readonly router: Router,
    private readonly shorts: ShortsService,
    private readonly toasts: ToastService,
    private readonly host: ElementRef,
  ) {}

  ngOnInit(): void {
    this.jobId = this.route.snapshot.paramMap.get('id') || '';
    if (!this.jobId) {
      this.router.navigate(['/shorts']);
      return;
    }

    const seenKey = `shorts-seen-${this.jobId}`;
    try {
      if (sessionStorage.getItem(seenKey)) {
        this.resumed = true;
        setTimeout(() => { this.resumed = false; }, 6000);
      } else {
        sessionStorage.setItem(seenKey, '1');
      }
    } catch {
      // Private mode: toast simply never shows.
    }

    this.pollSub = interval(3000)
      .pipe(
        startWith(0),
        switchMap(() => this.shorts.getStatus(this.jobId)),
      )
      .subscribe({
        next: (job) => this.onStatus(job),
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

  stageState(index: number): 'done' | 'active' | 'pending' {
    const reached = this.status === 'failed' ? this.furthest : this.stageOrder();
    if (index < reached) return 'done';
    if (index === reached) return 'active';
    return 'pending';
  }

  stagePercent(index: number): number {
    if (index === 0) return 100;
    return this.progress;
  }

  cancel(): void {
    if (this.cancelling || this.cancelDone) return;
    this.cancelling = true;
    this.shorts.cancelJob(this.jobId).subscribe({
      next: () => {
        this.cancelling = false;
        this.cancelDone = true;
        this.pollSub?.unsubscribe();
        this.toasts.show('Processing cancelled.', 'info');
        setTimeout(() => this.router.navigate(['/shorts/history']), 1200);
      },
      error: () => {
        this.cancelling = false;
        this.errorMessage = 'Could not cancel. Please try again.';
      },
    });
  }

  goToUpload(): void {
    this.router.navigate(['/shorts']);
  }

  private onStatus(job: { status: ShortsStatus; error?: string; sourceDuration: number; progress?: number }): void {
    const prevOrder = this.stageOrder();
    this.status = job.status;
    this.errorMessage = job.error ?? '';
    this.durationSeconds = job.sourceDuration || 0;
    this.progress = Math.max(0, Math.min(100, job.progress ?? this.fallbackProgress(job.status)));
    this.eta = this.buildEta(job.status);
    this.furthest = Math.max(this.furthest, this.stageOrder());
    this.popCompletedChecks(prevOrder);

    if (job.status === 'completed') {
      this.pollSub?.unsubscribe();
      this.redirectTimeout = setTimeout(() => {
        this.router.navigate(['/shorts', this.jobId, 'clips']);
      }, 2000);
    } else if (job.status === 'failed') {
      this.pollSub?.unsubscribe();
    }
  }
  private stageOrder(): number {
    if (!this.status) return 0;
    if (this.status === 'uploaded') return 1;
    if (this.status === 'completed') return 4;
    const map: Record<string, number> = { transcribing: 1, ranking: 2, rendering: 3 };
    return map[this.status] ?? 1;
  }

  private fallbackProgress(status: ShortsStatus): number {
    switch (status) {
      case 'completed': return 100;
      case 'failed': return 0;
      case 'rendering': return 78;
      case 'ranking': return 55;
      case 'transcribing': return 25;
      default: return 5;
    }
  }

  private buildEta(status: ShortsStatus): string {
    let seconds: number;
    if (status === 'transcribing') {
      seconds = Math.max(30, this.durationSeconds * 1.0 + 30);
    } else if (status === 'ranking') {
      seconds = 45;
    } else if (status === 'rendering') {
      seconds = Math.max(30, this.durationSeconds * 0.6);
    } else {
      return '';
    }
    if (seconds < 90) return `~${Math.round(seconds)} sec remaining`;
    return `~${Math.max(1, Math.round(seconds / 60))} min remaining`;
  }

  private popCompletedChecks(prevOrder: number): void {
    const order = this.stageOrder();
    if (order <= prevOrder) return;
    // GSAP pop-in for every stage that just completed (300ms, back.out).
    requestAnimationFrame(() => {
      try {
        const root: HTMLElement = this.host.nativeElement;
        root.querySelectorAll<HTMLElement>('[data-stage-check]').forEach((el) => {
          gsap.fromTo(el, { scale: 0 }, { scale: 1, duration: 0.3, ease: 'back.out(2)' });
        });
      } catch {
        // Animation is decorative; never break status rendering.
      }
    });
  }
}
