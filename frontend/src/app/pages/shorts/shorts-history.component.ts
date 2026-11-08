import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router, RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { ShortsJobSummary, ShortsService, ShortsStatus, startPipeline } from './shorts.service';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { ToastsComponent } from '../../shared/components/toast/toasts.component';
import { ToastService } from '../../shared/components/toast/toast.service';
import { ConfirmDialogComponent } from '../../shared/components/confirm-dialog/confirm-dialog.component';
import { ShortsEffectsPanelComponent } from './shorts-effects-panel.component';

@Component({
  selector: 'app-shorts-history',
  standalone: true,
  imports: [CommonModule, RouterLink, LucideDynamicIcon, NavbarComponent, ToastsComponent, ConfirmDialogComponent, ShortsEffectsPanelComponent],
  template: `
    <div class="ui-app">
      <app-navbar></app-navbar>
      <app-toasts></app-toasts>
      <main id="main" class="ui-app-main hm">
        <div class="head">
          <div>
            <h1>Your shorts</h1>
            <p class="lead">Every upload, newest first. Open one to see its clips or follow its progress.</p>
          </div>
          <a routerLink="/shorts" class="ui-btn ui-btn--primary"><svg lucideIcon="plus" [size]="18" aria-hidden="true"></svg>New short</a>
        </div>

        @if (isLoading) {
          <div class="list" role="status" aria-label="Loading shorts">
            @for (i of [1, 2, 3]; track i) {
              <div class="ui-skeleton" style="height: 4.75rem"></div>
            }
          </div>
        } @else if (error) {
          <div class="ui-card err" role="alert">
            <p>{{ error }}</p>
            <button type="button" class="ui-btn ui-btn--secondary" (click)="reload()">Try again</button>
          </div>
        } @else if (jobs.length === 0) {
          <div class="ui-card empty">
            <span class="ui-chip" aria-hidden="true"><svg lucideIcon="scissors" [size]="18"></svg></span>
            <p><strong>No shorts yet.</strong></p>
            <p>Upload a video or audio file and AI will cut your first clips.</p>
            <a routerLink="/shorts" class="ui-btn ui-btn--primary">Create your first short</a>
          </div>
        } @else {
          <ul class="ui-card rows" aria-label="Your shorts">
            @for (job of jobs; track job.jobId) {
              <li class="item" [attr.data-testid]="'job-' + job.jobId">
                <div class="line">
                  <!-- An uploaded job has no page to show yet: the progress page would read as transcribing. -->
                  <a
                    [routerLink]="job.status === 'uploaded' ? null : job.status === 'completed' && job.clipCount > 0 ? ['/shorts', job.jobId, 'clips'] : ['/shorts', job.jobId, 'progress']"
                    class="open"
                    [class.static]="job.status === 'uploaded'"
                  >
                    <span class="kind" aria-hidden="true">{{ job.fileType.toUpperCase() }}</span>
                    <span class="main">
                      <span class="title">Short {{ job.jobId.slice(0, 8) }}</span>
                      <span class="meta">
                        <span>{{ formatDate(job.createdAt) }}</span>
                        @if (job.status === 'completed') {
                          <span>{{ job.clipCount }} {{ job.clipCount === 1 ? 'clip' : 'clips' }}</span>
                        } @else if (job.status === 'uploaded') {
                          <span>Uploaded. Not started.</span>
                        }
                      </span>
                    </span>
                    @if (job.status !== 'uploaded') {
                      <span class="ui-pill" [class]="'ui-pill ' + tone(job.status)">{{ statusLabel(job.status) }}</span>
                      <svg lucideIcon="chevron-right" [size]="16" class="arrow" aria-hidden="true"></svg>
                    }
                  </a>
                  @if (job.status === 'uploaded' && draftJobId !== job.jobId) {
                    <button type="button" class="ui-btn ui-btn--primary start" (click)="openDraft(job)" [attr.aria-label]="'Start render for short ' + job.jobId.slice(0, 8)">Start render</button>
                  }
                  <button
                    type="button"
                    class="ui-btn ui-btn--quiet del"
                    (click)="askDelete(job)"
                    [disabled]="deletingId === job.jobId"
                    [attr.aria-label]="'Delete short ' + job.jobId.slice(0, 8)"
                    title="Delete short"
                  >
                    @if (deletingId === job.jobId) {
                      <svg lucideIcon="loader-2" [size]="16" class="spin" aria-hidden="true"></svg>
                    } @else {
                      <svg lucideIcon="trash-2" [size]="16" aria-hidden="true"></svg>
                    }
                  </button>
                </div>
                @if (draftJobId === job.jobId) {
                  <div class="draft">
                    <app-shorts-effects-panel [jobId]="job.jobId" [draftMode]="true" (effectsSaved)="onDraftSaved()" (cancelled)="closeDraft()"></app-shorts-effects-panel>
                  </div>
                }
              </li>
            }
          </ul>
        }
      </main>
    </div>

    <app-confirm-dialog
      [open]="pendingDelete !== null"
      title="Delete Short?"
      [message]="deleteMessage()"
      [details]="deleteDetails()"
      [busy]="deletingId !== null"
      (confirmed)="confirmDelete()"
      (cancelled)="pendingDelete = null"
    />
  `,
  styles: [`
    :host { display: block; }
    .head { display: flex; flex-wrap: wrap; align-items: flex-end; justify-content: space-between; gap: var(--space-sm); }
    h1 { margin: 0; font-size: var(--type-2xl); font-weight: 600; line-height: 1.15; letter-spacing: -0.01em; }
    .lead { margin: var(--space-2xs) 0 0; color: var(--color-ink-2); }
    .list { display: flex; flex-direction: column; gap: var(--space-xs); }
    .err, .empty { display: flex; flex-direction: column; align-items: center; gap: var(--space-xs); text-align: center; }
    .err p, .empty p { margin: 0; }
    .err p { color: var(--color-negative); font-weight: 500; }
    .empty p + p { color: var(--color-ink-2); margin-bottom: var(--space-sm); }
    .rows { margin: 0; padding: var(--space-2xs) var(--space-md); list-style: none; }
    .item + .item { border-top: 1px solid var(--color-rule); }
    .line { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-2xs); padding: var(--space-xs) 0; }
    .open { flex: 1; min-width: 0; display: flex; align-items: center; gap: var(--space-sm); min-height: 44px; padding: var(--space-xs); margin-inline: calc(-1 * var(--space-xs)); border-radius: var(--radius-control); color: var(--color-ink); text-decoration: none; }
    .open:not(.static):hover { background: var(--color-paper-3); }
    .kind { width: 2.75rem; height: 2.75rem; flex: none; border-radius: var(--radius-control); display: inline-flex; align-items: center; justify-content: center; background: color-mix(in oklab, var(--color-accent) 12%, var(--app-card)); color: var(--color-accent-text); font-size: var(--type-xs); font-weight: 700; }
    .main { flex: 1; min-width: 0; }
    .title { display: block; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .meta { display: flex; flex-wrap: wrap; gap: 0 var(--space-sm); font-size: var(--type-xs); color: var(--color-muted); }
    .arrow { flex: none; color: var(--color-muted); }
    .del { width: 44px; padding: 0; color: var(--color-ink-2); }
    .del:hover:not(:disabled) { color: var(--color-negative); }
    .start { order: 9; flex-basis: 100%; }
    @media (min-width: 40rem) { .start { order: 0; flex-basis: auto; } }
    .draft { padding-bottom: var(--space-md); }
    .t-ok { color: var(--color-positive); } .t-bad { color: var(--color-negative); } .t-wait { color: var(--color-caution); }
    .spin { animation: spin 0.9s linear infinite; }
    @keyframes spin { to { transform: rotate(360deg); } }
    @media (prefers-reduced-motion: reduce) { .spin { animation: none; } }
  `],
})
export class ShortsHistoryComponent implements OnInit {
  jobs: ShortsJobSummary[] = [];
  isLoading = true;
  error = '';
  pendingDelete: ShortsJobSummary | null = null;
  deletingId: string | null = null;
  /** The uploaded job whose Effects step is open, and whether its start request is in flight. */
  draftJobId: string | null = null;
  starting = false;

  private readonly shorts = inject(ShortsService);
  private readonly toasts = inject(ToastService);
  private readonly router = inject(Router);

  ngOnInit(): void {
    this.reload();
  }

  reload(): void {
    this.isLoading = true;
    this.error = '';
    this.shorts.listJobs().subscribe({
      next: (jobs) => {
        this.jobs = [...jobs].sort(
          (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
        );
        this.isLoading = false;
      },
      error: () => {
        this.isLoading = false;
        this.error = 'Failed to load shorts. Please try again.';
      },
    });
  }

  statusLabel(status: ShortsStatus): string {
    return status.charAt(0).toUpperCase() + status.slice(1);
  }

  /** Colour of the status pill: ready, failed, or still working. */
  tone(status: ShortsStatus): string {
    return status === 'completed' ? 't-ok' : status === 'failed' ? 't-bad' : 't-wait';
  }

  openDraft(job: ShortsJobSummary): void {
    if (this.starting) return;
    this.draftJobId = job.jobId;
  }

  closeDraft(): void {
    if (this.starting) return;
    this.draftJobId = null;
  }

  /** The Effects step saved the draft: start the render the same way the upload page does. */
  onDraftSaved(): void {
    const jobId = this.draftJobId;
    if (this.starting || !jobId) return;
    this.starting = true;
    startPipeline(this.shorts, this.router, jobId).subscribe({
      next: () => {
        this.starting = false;
      },
      error: (err) => {
        this.starting = false;
        // Network and server failures keep the step open for a retry. A rejected start means the job changed, so the list reloads.
        if (err?.status && err.status < 500) {
          this.draftJobId = null;
          this.reload();
        }
        this.toasts.show(err?.error?.message || 'Failed to start processing. Please try again.', 'error');
      },
    });
  }

  askDelete(job: ShortsJobSummary): void {
    if (this.deletingId !== null) return;
    this.pendingDelete = job;
  }

  deleteMessage(): string {
    const job = this.pendingDelete;
    if (!job) return '';
    const bits = [`Short ${job.jobId.slice(0, 8)}`, job.fileType.toUpperCase()];
    if (job.status === 'completed' && job.clipCount > 0) {
      bits.push(`${job.clipCount} rendered ${job.clipCount === 1 ? 'clip' : 'clips'}`);
    } else {
      bits.push(`status: ${this.statusLabel(job.status)}`);
    }
    return `Permanently delete ${bits.join(', ')} and all of its assets?`;
  }

  deleteDetails(): string[] {
    const job = this.pendingDelete;
    if (!job) return [];
    const details = ['Original uploaded media', 'AI transcript and ranking data'];
    if (job.status === 'completed' && job.clipCount > 0) {
      details.push(`${job.clipCount} rendered vertical ${job.clipCount === 1 ? 'clip' : 'clips'}`);
    }
    details.push('Linked project references are cleaned up');
    return details;
  }

  confirmDelete(): void {
    const job = this.pendingDelete;
    if (!job || this.deletingId !== null) return;
    this.deletingId = job.jobId;
    this.shorts.deleteJob(job.jobId).subscribe({
      next: () => {
        this.jobs = this.jobs.filter((j) => j.jobId !== job.jobId);
        this.pendingDelete = null;
        this.deletingId = null;
        this.toasts.show('Short deleted successfully.', 'success');
      },
      error: () => {
        this.pendingDelete = null;
        this.deletingId = null;
        this.toasts.show('Could not delete this short. Please try again.', 'error');
      },
    });
  }

  formatDate(iso: string): string {
    const d = new Date(iso);
    return isNaN(d.getTime()) ? iso : d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
  }
}
