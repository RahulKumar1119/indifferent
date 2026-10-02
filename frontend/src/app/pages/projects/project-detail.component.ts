import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { ProjectService } from '../../core/services/project.service';
import { Project } from '../../shared/models/project.model';
import { ShortsJob, ShortsService } from '../shorts/shorts.service';

@Component({
  selector: 'app-project-detail',
  standalone: true,
  imports: [CommonModule, RouterLink],
  template: `
    <div class="sans bg-[#FAF7F2] text-[#1A1714] antialiased min-h-[100dvh]">
      <main class="max-w-4xl mx-auto px-4 pt-16 pb-16">
        <a routerLink="/projects" class="text-[13px] text-[#6B6560] hover:text-[#1A1714]">← All projects</a>

        @if (isLoading) {
          <p class="mt-8 text-center text-sm text-[#6B6560]">Loading project…</p>
        } @else if (error) {
          <div class="mt-8 rounded-[16px] border border-red-500/30 bg-white p-6 text-center">
            <p class="text-sm text-red-600">{{ error }}</p>
          </div>
        } @else if (project) {
          <p class="mt-4 text-[11.5px] uppercase tracking-[0.22em] text-[#6B6560]">Project · {{ project.status }}</p>
          <h1 class="mt-2 font-medium tracking-[-0.02em] leading-[1.02] text-[clamp(1.8rem,4vw,2.8rem)]">{{ project.name }}</h1>

          <!-- Quiz film -->
          <section class="mt-8 rounded-[20px] border border-black/10 bg-white p-6">
            <div class="flex items-center justify-between">
              <h2 class="font-semibold text-[16.5px]">Quiz film</h2>
              <span class="text-[12px] uppercase tracking-[0.14em] text-[#6B6560]">{{ project.status }}</span>
            </div>
            <dl class="mt-3 grid grid-cols-2 gap-2 text-[13.5px]">
              <div><dt class="text-[#6B6560]">Template</dt><dd class="font-medium capitalize">{{ project.template }}</dd></div>
              <div><dt class="text-[#6B6560]">Voice</dt><dd class="font-medium">{{ project.voice }}</dd></div>
            </dl>
            <div class="mt-4 flex flex-wrap gap-2">
              <a [routerLink]="['/projects', project.id, 'upload']" class="px-4 py-2 rounded-full border border-black/15 hover:bg-black/5 transition-colors text-[13.5px] font-medium">Upload TXT</a>
              <a [routerLink]="['/projects', project.id, 'progress']" class="px-4 py-2 rounded-full border border-black/15 hover:bg-black/5 transition-colors text-[13.5px] font-medium">Track progress</a>
              @if (project.status === 'completed') {
                <a [routerLink]="['/projects', project.id, 'preview']" class="px-4 py-2 rounded-full bg-[#1A1714] text-white hover:bg-[#2A2620] transition-colors text-[13.5px] font-medium">Watch & download</a>
              }
            </div>
          </section>

          <!-- Shorts -->
          <section class="mt-5 rounded-[20px] border border-black/10 bg-white p-6">
            <div class="flex items-center justify-between">
              <h2 class="font-semibold text-[16.5px]">Shorts</h2>
              @if (shortsJobs.length > 0) {
                <span class="text-[12px] uppercase tracking-[0.14em] text-[#6B6560]">{{ shortsJobs.length }} linked</span>
              }
            </div>
            @if (shortsJobs.length === 0) {
              <p class="mt-2 text-[13.5px] text-[#6B6560]">No shorts linked to this project yet.</p>
            } @else {
              <ul class="mt-3 space-y-2">
                @for (job of shortsJobs; track job.jobId) {
                  <li class="flex items-center justify-between gap-3 rounded-[12px] border border-black/10 px-4 py-2.5 text-[13.5px]">
                    <span class="font-medium">{{ job.jobId.slice(0, 8) }}… · {{ job.status }}</span>
                    <span class="flex gap-2">
                      <a [routerLink]="['/shorts', job.jobId, 'progress']" class="underline underline-offset-4">Progress</a>
                      @if (job.status === 'completed') {
                        <a [routerLink]="['/shorts', job.jobId, 'clips']" class="underline underline-offset-4">Clips</a>
                      }
                    </span>
                  </li>
                }
              </ul>
            }
          </section>

          <!-- Watermark -->
          <section class="mt-5 rounded-[20px] border border-black/10 bg-white p-6">
            <h2 class="font-semibold text-[16.5px]">Watermark</h2>
            @if (project.watermark?.text) {
              <dl class="mt-3 grid grid-cols-2 gap-2 text-[13.5px]">
                <div><dt class="text-[#6B6560]">Text</dt><dd class="font-medium">“{{ project.watermark!.text }}”</dd></div>
                @if (project.watermark!.color) {
                  <div><dt class="text-[#6B6560]">Color</dt><dd class="font-medium">{{ project.watermark!.color }}</dd></div>
                }
                @if (project.watermark!.opacity != null) {
                  <div><dt class="text-[#6B6560]">Opacity</dt><dd class="font-medium">{{ project.watermark!.opacity }}</dd></div>
                }
              </dl>
            } @else {
              <p class="mt-2 text-[13.5px] text-[#6B6560]">No watermark saved for this project.</p>
            }
            <a routerLink="/tools/add-watermark" class="mt-4 inline-block px-4 py-2 rounded-full border border-black/15 hover:bg-black/5 transition-colors text-[13.5px] font-medium">Open watermark tool</a>
          </section>
        }
      </main>
    </div>
  `,
  styles: [`
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
  `],
})
export class ProjectDetailComponent implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly projects = inject(ProjectService);
  private readonly shorts = inject(ShortsService);

  project: Project | null = null;
  shortsJobs: ShortsJob[] = [];
  isLoading = true;
  error = '';

  ngOnInit(): void {
    const id = this.route.snapshot.paramMap.get('id') || '';
    if (!id) {
      this.router.navigate(['/projects']);
      return;
    }
    this.projects.getProject(id).subscribe({
      next: (project) => {
        this.project = project;
        this.isLoading = false;
        for (const jobId of project.shortsJobIds ?? []) {
          this.shorts.getStatus(jobId).subscribe({
            next: (job) => {
              this.shortsJobs = [...this.shortsJobs, job].sort((a, b) => a.jobId.localeCompare(b.jobId));
            },
            error: () => {},
          });
        }
      },
      error: () => {
        this.isLoading = false;
        this.error = 'Project not found.';
      },
    });
  }
}
