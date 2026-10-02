import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { ShortsJobSummary, ShortsService, ShortsStatus } from './shorts.service';

@Component({
  selector: 'app-shorts-history',
  standalone: true,
  imports: [CommonModule, RouterLink],
  template: `
    <div class="sans bg-[#FAF7F2] text-[#1A1714] antialiased min-h-[100dvh]">
      <main class="max-w-4xl mx-auto px-4 pt-16 pb-16">
        <div class="flex items-center justify-between gap-4">
          <div>
            <p class="text-[11.5px] uppercase tracking-[0.22em] text-[#6B6560]">Studio · Shorts</p>
            <h1 class="mt-2 font-medium tracking-[-0.02em] leading-[1.02] text-[clamp(1.8rem,4vw,2.6rem)]">Your shorts.</h1>
          </div>
          <a routerLink="/shorts" class="shrink-0 px-4 py-2 rounded-full bg-[#1A1714] text-white text-[13.5px] font-medium hover:bg-[#2A2620] transition-colors">+ New</a>
        </div>

        @if (isLoading) {
          <p class="mt-8 text-center text-sm text-[#6B6560]">Loading shorts…</p>
        } @else if (error) {
          <div class="mt-8 rounded-[16px] border border-red-500/30 bg-white p-6 text-center">
            <p class="text-sm text-red-600">{{ error }}</p>
          </div>
        } @else if (jobs.length === 0) {
          <div class="mt-8 rounded-[20px] border border-black/10 bg-white p-10 text-center">
            <p class="text-[15px] font-medium">No shorts yet.</p>
            <p class="mt-2 text-sm text-[#6B6560]">Upload a video or audio file and AI will cut your first clips.</p>
            <a routerLink="/shorts" class="mt-5 inline-block px-6 py-3 rounded-full bg-[#1A1714] text-white text-[14px] font-medium hover:bg-[#2A2620] transition-colors">Create your first short</a>
          </div>
        } @else {
          <ul class="mt-6 divide-y divide-black/[0.07] rounded-[20px] border border-black/[0.07] bg-white px-2 md:px-4">
            @for (job of jobs; track job.jobId) {
              <li>
                <a
                  [routerLink]="job.status === 'completed' && job.clipCount > 0 ? ['/shorts', job.jobId, 'clips'] : ['/shorts', job.jobId, 'progress']"
                  class="row-hover flex items-center gap-4 px-2 md:px-3 py-4 rounded-xl"
                >
                  <span class="w-9 h-9 rounded-full bg-[#1A1714]/[.05] flex items-center justify-center shrink-0 text-[12px] font-bold text-[#6B6560]">
                    {{ job.fileType.toUpperCase() }}
                  </span>
                  <span class="flex-1 min-w-0">
                    <span class="block font-medium truncate text-[15px]">Short {{ job.jobId.slice(0, 8) }}</span>
                    <span class="block text-[12.5px] text-[#6B6560]">
                      {{ formatDate(job.createdAt) }}
                      @if (job.status === 'completed') {
                        &middot; {{ job.clipCount }} {{ job.clipCount === 1 ? 'clip' : 'clips' }}
                      }
                    </span>
                  </span>
                  <span class="hidden sm:inline-flex rounded-full border border-black/10 px-3 py-1 text-[11.5px] font-medium">{{ statusLabel(job.status) }}</span>
                </a>
              </li>
            }
          </ul>
        }
      </main>
    </div>
  `,
  styles: [`
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
    .row-hover { transition: background-color .2s; }
    .row-hover:hover { background-color: rgba(26,23,20,0.04); }
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
