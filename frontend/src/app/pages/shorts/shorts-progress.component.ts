import { Component, ElementRef, OnDestroy, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { Subscription, interval } from 'rxjs';
import { startWith, switchMap } from 'rxjs/operators';
import gsap from 'gsap';
import { ShortsJob, ShortsService, ShortsStatus } from './shorts.service';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { ToastsComponent } from '../../shared/components/toast/toasts.component';
import { ToastService } from '../../shared/components/toast/toast.service';
import { ShortsEffectsPanelComponent } from './shorts-effects-panel.component';

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
  imports: [CommonModule, RouterLink, LucideDynamicIcon, NavbarComponent, ToastsComponent, ShortsEffectsPanelComponent],
  styles: [`
    :host { display: block; }
    .wrap { width: min(44rem, 100%); margin-inline: auto; display: flex; flex-direction: column; gap: var(--space-lg); }
    h1 { margin: 0; font-size: var(--type-2xl); font-weight: 600; line-height: 1.15; letter-spacing: -0.01em; }
    .lead { margin: var(--space-2xs) 0 0; color: var(--color-ink-2); }
    .resumed { display: flex; align-items: center; gap: var(--space-xs); font-size: var(--type-sm); border-left: 4px solid var(--color-accent); }
    .resumed p { margin: 0; }
    ol { margin: 0; padding: 0; list-style: none; }
    .step { display: flex; gap: var(--space-md); }
    .rail { display: flex; flex-direction: column; align-items: center; }
    .dot { position: relative; width: 2.5rem; height: 2.5rem; flex: none; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; border: 2px solid var(--color-rule); color: var(--color-muted); font-size: var(--type-xs); font-weight: 600; }
    .dot.done { background: var(--color-positive); border-color: var(--color-positive); color: var(--color-paper); }
    .dot.active { border-color: var(--color-accent); background: color-mix(in oklab, var(--color-accent) 12%, var(--app-card)); }
    .dot.active span { width: 0.75rem; height: 0.75rem; border-radius: 50%; background: var(--color-accent); }
    .dot.active::after { content: ''; position: absolute; inset: -2px; border-radius: 50%; border: 2px solid var(--color-accent); animation: ping 1.4s var(--ease-out) infinite; }
    @keyframes ping { 0% { transform: scale(1); opacity: 0.5; } 100% { transform: scale(1.5); opacity: 0; } }
    .line { flex: 1; width: 2px; min-height: 1.75rem; margin: var(--space-3xs) 0; border-radius: 2px; background: var(--color-rule); }
    .line.done { background: var(--color-positive); }
    .body { flex: 1; min-width: 0; padding-bottom: var(--space-lg); }
    .step:last-child .body { padding-bottom: 0; }
    .label { display: flex; align-items: center; justify-content: space-between; gap: var(--space-sm); min-height: 2.5rem; }
    .label p { margin: 0; font-weight: 500; color: var(--color-ink-2); }
    .label p.done { color: var(--color-positive); }
    .label p.active { color: var(--color-ink); font-weight: 600; }
    .pct { font-size: var(--type-xs); color: var(--color-muted); font-variant-numeric: tabular-nums; }
    .track { position: relative; height: 8px; border-radius: var(--radius-pill); background: var(--color-paper-3); overflow: hidden; }
    .track .fill { height: 100%; border-radius: inherit; background: var(--color-accent); transition: width 0.6s var(--ease-out); }
    .eq { position: absolute; inset: 0; display: flex; align-items: flex-end; justify-content: center; gap: 3px; padding-bottom: 1px; opacity: 0.45; }
    .eq span { width: 3px; border-radius: 2px; background: var(--color-accent-ink); transform-origin: bottom; animation: eq 1.1s ease-in-out infinite; }
    @keyframes eq { 0%, 100% { transform: scaleY(0.3); } 50% { transform: scaleY(1); } }
    .cancel { margin-top: var(--space-md); display: flex; justify-content: center; }
    .muted { margin: var(--space-md) 0 0; text-align: center; font-size: var(--type-sm); color: var(--color-muted); }
    .thumbs { margin: 0; text-align: center; font-size: var(--type-sm); color: var(--color-ink-2); }
    .result { text-align: center; display: flex; flex-direction: column; align-items: center; gap: var(--space-xs); }
    .result h2 { margin: var(--space-xs) 0 0; font-size: var(--type-lg); font-weight: 600; }
    .result p { margin: 0; color: var(--color-ink-2); font-size: var(--type-sm); max-width: 52ch; }
    .result.ok { border-top: 4px solid var(--color-positive); } .result.ok svg.big { color: var(--color-positive); }
    .result.bad { border-top: 4px solid var(--color-negative); } .result.bad svg.big { color: var(--color-negative); }
    .result.bad h2 { color: var(--color-negative); }
    .fx { width: 100%; text-align: left; margin-top: var(--space-sm); }
    .acts { margin-top: var(--space-sm); display: flex; flex-wrap: wrap; justify-content: center; gap: var(--space-xs); }
    .spin { animation: spin 0.9s linear infinite; }
    @keyframes spin { to { transform: rotate(360deg); } }
    @media (prefers-reduced-motion: reduce) { .eq span, .dot.active::after, .spin { animation: none; } .eq span { transform: none; } .track .fill { transition: none; } }
  `],
  template: `
    <div class="ui-app">
      <app-navbar></app-navbar>
      <app-toasts></app-toasts>
      <main id="main" class="ui-app-main hm">
        <div class="wrap">
          <div>
            <h1>Making your shorts</h1>
            <p class="lead">
              @if (eta && (status === 'transcribing' || status === 'ranking' || status === 'rendering')) {
                {{ eta }}. Feel free to leave, we'll keep working.
              } @else {
                We're analyzing your media and rendering the best moments.
              }
            </p>
          </div>

          @if (resumed) {
            <div class="ui-card resumed" role="status">
              <svg lucideIcon="clock" [size]="16" aria-hidden="true"></svg>
              <p>Your shorts are still processing. You can navigate away and come back anytime.</p>
            </div>
          }

          <section class="ui-card" role="progressbar" aria-label="Processing pipeline">
            <ol>
              @for (stage of stages; track stage.key; let i = $index; let last = $last) {
                <li class="step" [attr.data-testid]="'stage-' + stage.key" [attr.data-state]="stageState(i)">
                  <div class="rail">
                    <span class="dot" [class.done]="stageState(i) === 'done'" [class.active]="stageState(i) === 'active'">
                      @if (stageState(i) === 'active') {
                        <span></span>
                      } @else if (stageState(i) === 'done') {
                        <svg lucideIcon="check" [size]="18" [attr.data-stage-check]="stage.key" aria-hidden="true"></svg>
                      } @else {
                        {{ i + 1 }}
                      }
                    </span>
                    @if (!last) {
                      <span class="line" [class.done]="stageState(i) === 'done'"></span>
                    }
                  </div>
                  <div class="body">
                    <div class="label">
                      <p [class.done]="stageState(i) === 'done'" [class.active]="stageState(i) === 'active'">
                        {{ stage.label }}
                        @if (stageState(i) === 'done') {
                          <span class="sr-only">, done</span>
                        } @else if (stageState(i) === 'active') {
                          <span class="sr-only">, in progress</span>
                        }
                      </p>
                      @if (stageState(i) === 'active') {
                        <span class="pct">{{ stagePercent(i) }}%</span>
                      }
                    </div>
                    @if (stageState(i) === 'active') {
                      <div class="track">
                        <div class="fill" [style.width.%]="stagePercent(i)"></div>
                        <div class="eq" aria-hidden="true">
                          @for (b of eqBars; track b) {
                            <span [style.height.%]="b.h" [style.animation-delay]="b.d"></span>
                          }
                        </div>
                      </div>
                    }
                  </div>
                </li>
              }
            </ol>
            @if (cancelling || cancelDone) {
              <p class="muted">{{ cancelDone ? 'Cancellation requested' : 'Cancelling' }}</p>
            } @else if (status && status !== 'completed' && status !== 'failed') {
              <div class="cancel">
                <button type="button" class="ui-btn ui-btn--danger" [disabled]="cancelling" (click)="cancel()">Cancel processing</button>
              </div>
            }
          </section>

          @if (isVideo && (status === 'ranking' || status === 'rendering')) {
            <p class="thumbs">
              We're also making thumbnail suggestions for your video.
              <a [routerLink]="['/shorts', jobId, 'thumbnails']" class="ui-link">See AI thumbnails</a>
            </p>
          }

          @if (status === 'completed') {
            <div class="ui-card result ok" role="status">
              <svg lucideIcon="circle-check" [size]="44" class="big" aria-hidden="true"></svg>
              <h2>Your shorts are ready</h2>
              <p>Redirecting to your clips</p>
            </div>
          }

          @if (status === 'failed') {
            <div class="ui-card result bad" role="alert">
              <svg lucideIcon="circle-x" [size]="44" class="big" aria-hidden="true"></svg>
              <h2>Processing failed</h2>
              <p>{{ errorMessage || 'Something went wrong while processing your media. Please try again.' }}</p>
              <!-- A failed job can change its video effects before Retry re-renders it. -->
              <app-shorts-effects-panel class="fx"></app-shorts-effects-panel>
              <div class="acts">
                @if (retryableFailure) {
                  <button type="button" class="ui-btn ui-btn--primary" (click)="retry()" [disabled]="retrying">
                    @if (retrying) {
                      <svg lucideIcon="loader-2" [size]="16" class="spin" aria-hidden="true"></svg>
                      Restarting
                    } @else {
                      Retry processing
                    }
                  </button>
                } @else {
                  <p style="width: 100%">This file can't be processed as it is. Try a different file with clear speech, from 30 seconds to an hour long.</p>
                }
                <button type="button" class="ui-btn ui-btn--secondary" (click)="goToUpload()">Start Over</button>
              </div>
            </div>
          }
        </div>
      </main>
    </div>
  `,
})
export class ShortsProgressComponent implements OnInit, OnDestroy {
  private pollSub: Subscription | null = null;
  private redirectTimeout: ReturnType<typeof setTimeout> | null = null;
  jobId = '';
  private furthest = 0;
  /** Video uploads get AI thumbnails; audio uploads have nothing to take them from. */
  isVideo = false;

  status: ShortsStatus | null = null;
  progress = 0;
  durationSeconds = 0;
  eta = '';
  errorMessage = '';
  errorCode = '';
  resumed = false;
  cancelling = false;
  cancelDone = false;
  retrying = false;

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

    this.startPolling();
  }

  private startPolling(): void {
    this.pollSub?.unsubscribe();
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

  /** Validation failures can't succeed on retry (same input, same verdict):
   * steer to a different file; everything else restarts in place. */
  get retryableFailure(): boolean {
    return this.status === 'failed' && !this.errorCode.startsWith('VALIDATION');
  }

  retry(): void {
    if (this.retrying || this.status !== 'failed') return;
    this.retrying = true;
    this.shorts.retryJob(this.jobId).subscribe({
      next: () => {
        this.retrying = false;
        this.errorMessage = '';
        this.errorCode = '';
        this.status = null;
        this.progress = 0;
        this.toasts.show('Restarting processing from your source file.', 'success');
        this.startPolling();
      },
      error: (err) => {
        this.retrying = false;
        this.toasts.show(err?.error?.message || 'Could not restart processing.', 'error');
      },
    });
  }

  private onStatus(job: { status: ShortsStatus; error?: string; errorCode?: string; sourceDuration: number; progress?: number; fileType?: string }): void {
    const prevOrder = this.stageOrder();
    this.isVideo = ['mp4', 'mov'].includes((job.fileType ?? '').toLowerCase());
    this.status = job.status;
    this.errorMessage = job.error ?? '';
    this.errorCode = job.errorCode ?? '';
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
    return status === 'completed' ? 100 : 0;
  }

  // The ETA has to stay believable for a long source. Transcribing scales
  // with the file but runs well under real time; rendering depends on the
  // clips (3 of at most 90s), not on the source length, so it must not grow
  // with it. Both are capped, and the caps need recalibrating from real
  // one-hour runs before they are quoted as measured numbers.
  private buildEta(status: ShortsStatus): string {
    let seconds: number;
    if (status === 'transcribing') {
      seconds = Math.min(15 * 60, Math.max(30, this.durationSeconds * 1.0 + 30));
    } else if (status === 'rendering') {
      seconds = Math.min(5 * 60, Math.max(30, this.durationSeconds * 0.6));
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
