import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { ShortsJobSummary, ShortsService, ShortsStatus } from './shorts.service';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { ToastsComponent } from '../../shared/components/toast/toasts.component';

@Component({
  selector: 'app-shorts-history',
  standalone: true,
  imports: [CommonModule, RouterLink, NavbarComponent, ToastsComponent],
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
              <li>
                <a
                  [routerLink]="job.status === 'completed' && job.clipCount > 0 ? ['/shorts', job.jobId, 'clips'] : ['/shorts', job.jobId, 'progress']"
                  class="row-hover flex items-center gap-4 px-2 md:px-3 py-4 rounded-xl"
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
              </li>
            }
          </ul>
        }
      </main>
    </div>
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

  constructor(private readonly shorts: ShortsService) {}

  ngOnInit(): void {
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

  formatDate(iso: string): string {
    const d = new Date(iso);
    return isNaN(d.getTime()) ? iso : d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
  }
}
