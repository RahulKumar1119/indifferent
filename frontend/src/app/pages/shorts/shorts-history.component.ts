import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { ShortsJobSummary, ShortsService, ShortsStatus } from './shorts.service';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { ToastsComponent } from '../../shared/components/toast/toasts.component';
import { ToastService } from '../../shared/components/toast/toast.service';
import { ConfirmDialogComponent } from '../../shared/components/confirm-dialog/confirm-dialog.component';

@Component({
  selector: 'app-shorts-history',
  standalone: true,
  imports: [CommonModule, RouterLink, LucideDynamicIcon, NavbarComponent, ToastsComponent, ConfirmDialogComponent],
  template: `
    <app-navbar></app-navbar>
    <app-toasts></app-toasts>
    <div class="bg-[hsl(var(--background))] text-[hsl(var(--foreground))] antialiased min-h-[100dvh]">
      <main class="max-w-4xl mx-auto px-4 pt-10 pb-16">
        <div class="flex items-center justify-between gap-4">
          <div>
            <p class="text-[11.5px] uppercase tracking-[0.22em] text-[hsl(var(--muted-foreground))]">Studio · Shorts</p>
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
              <li class="flex items-center gap-1 px-1 md:px-2">
                <a
                  [routerLink]="job.status === 'completed' && job.clipCount > 0 ? ['/shorts', job.jobId, 'clips'] : ['/shorts', job.jobId, 'progress']"
                  class="row-hover flex flex-1 items-center gap-4 px-2 md:px-3 py-4 rounded-xl min-w-0"
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
                      }
                    </span>
                  </span>
                  <span class="hidden sm:inline-flex rounded-full border border-[hsl(var(--border))] px-3 py-1 text-[11.5px] font-medium">{{ statusLabel(job.status) }}</span>
                </a>
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

  private readonly shorts = inject(ShortsService);
  private readonly toasts = inject(ToastService);

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
