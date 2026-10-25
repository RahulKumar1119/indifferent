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
    <app-navbar></app-navbar>
    <app-toasts></app-toasts>
    <div class="bg-[hsl(var(--background))] text-[hsl(var(--foreground))] antialiased min-h-[100dvh]">
      <main class="max-w-4xl mx-auto px-4 pt-10 pb-16">
        <div class="flex items-center justify-between gap-4">
          <div>
            <p class="text-[11.5px] uppercase tracking-[0.1em] text-[hsl(var(--muted-foreground))]">Studio · Shorts</p>
            <h1 class="mt-2 font-medium tracking-[-0.02em] leading-[1.02] text-[clamp(1.8rem,4vw,2.6rem)]">Your shorts.</h1>
          </div>
          <a routerLink="/shorts" class="btn-interactive shrink-0 px-4 py-2 rounded-full bg-[hsl(var(--foreground))] text-[hsl(var(--background))] text-[13.5px] font-medium">+ New</a>
        </div>

        @if (isLoading) {
          <div class="mt-6 space-y-3" aria-label="Loading shorts">
            @for (i of [1,2,3]; track i) {
              <div class="skeleton h-[76px] rounded-[20px]"></div>
            }
          </div>
        } @else if (error) {
          <div class="mt-8 rounded-[16px] border border-red-500/30 p-6 text-center">
            <p class="text-sm text-red-400">{{ error }}</p>
            <button
              type="button"
              (click)="reload()"
              class="btn-interactive mt-4 inline-flex items-center gap-2 rounded-full bg-[hsl(var(--foreground))] text-[hsl(var(--background))] px-5 py-2.5 text-[13.5px] font-semibold"
            >
              Try again
            </button>
          </div>
        } @else if (jobs.length === 0) {
          <div class="mt-8 rounded-[20px] border border-[hsl(var(--border))] p-10 text-center">
            <p class="text-[15px] font-medium">No shorts yet.</p>
            <p class="mt-2 text-sm text-[hsl(var(--muted-foreground))]">Upload a video or audio file and AI will cut your first clips.</p>
            <a routerLink="/shorts" class="btn-interactive mt-5 inline-block px-6 py-3 rounded-full bg-[hsl(var(--foreground))] text-[hsl(var(--background))] text-[14px] font-medium">Create your first short</a>
          </div>
        } @else {
          <ul class="mt-6 divide-y divide-[hsl(var(--border))] rounded-[20px] border border-[hsl(var(--border))] px-2 md:px-4">
            @for (job of jobs; track job.jobId) {
              <li class="flex flex-wrap items-center gap-1 px-1 md:px-2">
                <!-- An uploaded job has no page to show yet: the progress page would read as transcribing. -->
                <a
                  [routerLink]="job.status === 'uploaded' ? null : job.status === 'completed' && job.clipCount > 0 ? ['/shorts', job.jobId, 'clips'] : ['/shorts', job.jobId, 'progress']"
                  [class.row-hover]="job.status !== 'uploaded'"
                  class="flex flex-1 items-center gap-4 px-2 md:px-3 py-4 rounded-xl min-w-0"
                >
                  <span class="w-9 h-9 rounded-full bg-[hsl(var(--foreground))]/[.06] flex items-center justify-center shrink-0 text-[12px] font-bold text-[hsl(var(--muted-foreground))]">
                    {{ job.fileType.toUpperCase() }}
                  </span>
                  <span class="flex-1 min-w-0">
                    <span class="block font-medium truncate text-[15px]">Short {{ job.jobId.slice(0, 8) }}</span>
                    <span class="block text-[12.5px] text-[hsl(var(--muted-foreground))]">
                      {{ formatDate(job.createdAt) }}
                      @if (job.status === 'completed') {
                        &middot; {{ job.clipCount }} {{ job.clipCount === 1 ? 'clip' : 'clips' }}
                      } @else if (job.status === 'uploaded') {
                        &middot; Uploaded. Not started.
                      }
                    </span>
                  </span>
                  @if (job.status !== 'uploaded') {
                    <span class="hidden sm:inline-flex rounded-full border border-[hsl(var(--border))] px-3 py-1 text-[11.5px] font-medium">{{ statusLabel(job.status) }}</span>
                  }
                </a>
                @if (job.status === 'uploaded' && draftJobId !== job.jobId) {
                  <!-- Full width on phones, on the row beside the delete button from sm up. -->
                  <button
                    type="button"
                    (click)="openDraft(job)"
                    class="btn-interactive order-last basis-full sm:order-none sm:basis-auto min-h-[44px] px-4 rounded-full bg-[hsl(var(--foreground))] text-[hsl(var(--background))] text-[13.5px] font-medium"
                    [attr.aria-label]="'Start render for short ' + job.jobId.slice(0, 8)"
                  >
                    Start render
                  </button>
                }
                <button
                  type="button"
                  (click)="askDelete(job)"
                  [disabled]="deletingId === job.jobId"
                  class="btn-interactive shrink-0 w-9 h-9 rounded-lg inline-flex items-center justify-center text-[hsl(var(--muted-foreground))] hover:text-red-400 hover:bg-red-500/10 disabled:opacity-50"
                  [attr.aria-label]="'Delete short ' + job.jobId.slice(0, 8)"
                  title="Delete short"
                >
                  @if (deletingId === job.jobId) {
                    <svg lucideIcon="loader-2" [size]="16" class="animate-spin"></svg>
                  } @else {
                    <svg lucideIcon="trash-2" [size]="16"></svg>
                  }
                </button>
                @if (draftJobId === job.jobId) {
                  <div class="order-last basis-full pb-4">
                    <app-shorts-effects-panel
                      [jobId]="job.jobId"
                      [draftMode]="true"
                      (effectsSaved)="onDraftSaved()"
                      (cancelled)="closeDraft()"
                    ></app-shorts-effects-panel>
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
    .row-hover { transition: background-color var(--transition-fast); }
    .row-hover:hover { background-color: hsl(var(--foreground) / 0.04); }
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
    return `Permanently delete ${bits.join(' · ')} and all of its assets?`;
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
