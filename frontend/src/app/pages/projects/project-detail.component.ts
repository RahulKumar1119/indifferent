import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { ProjectService } from '../../core/services/project.service';
import { Project } from '../../shared/models/project.model';
import { ShortsJob, ShortsService } from '../shorts/shorts.service';

@Component({
  selector: 'app-project-detail',
  standalone: true,
  imports: [CommonModule, FormsModule, RouterLink],
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

          <!-- Branding -->
          <section class="mt-5 rounded-[20px] border border-black/10 bg-white p-6">
            <h2 class="font-semibold text-[16.5px]">Branding</h2>
            <p class="mt-1 text-[13px] text-[#6B6560]">Logo overlays top-right on quiz videos and shorts.</p>
            <div class="mt-3 flex flex-col sm:flex-row sm:items-center gap-4">
              <div class="w-20 h-20 rounded-[12px] border border-black/10 bg-[#1A1714]/[.03] flex items-center justify-center overflow-hidden shrink-0">
                @if (logoPreviewUrl) {
                  <img [src]="logoPreviewUrl" alt="Brand logo" class="max-w-full max-h-full object-contain" />
                } @else {
                  <span class="text-[11px] text-[#6B6560] px-2 text-center">No logo</span>
                }
              </div>
              <div class="flex-1 space-y-2">
                <div class="flex flex-wrap items-center gap-2">
                  <input [(ngModel)]="channelName" placeholder="@yourchannel" maxlength="60"
                    class="h-10 px-3 rounded-[10px] bg-[#1A1714]/[.03] border border-black/15 text-[13.5px] focus:outline-none focus:border-[#BC5227] w-52" />
                  <button (click)="saveChannel()" [disabled]="savingChannel"
                    class="px-4 h-10 rounded-full bg-[#1A1714] text-white hover:bg-[#2A2620] transition-colors text-[13px] font-medium disabled:opacity-60">
                    {{ savingChannel ? 'Saving…' : 'Save handle' }}
                  </button>
                </div>
                <label class="inline-flex items-center gap-2 text-[13px] font-medium text-[#BC5227] underline underline-offset-4 cursor-pointer">
                  <input type="file" accept=".png,.jpg,.jpeg,.webp" class="hidden" (change)="onLogoSelected($event)" />
                  {{ uploadingLogo ? 'Uploading…' : (project.branding?.logoKey ? 'Replace logo' : 'Upload logo (PNG)') }}
                </label>
                @if (brandingError) {
                  <p class="text-[13px] text-[#B3372F]">{{ brandingError }}</p>
                }
                @if (brandingNotice) {
                  <p class="text-[13px] text-[#1E3A2A]">{{ brandingNotice }}</p>
                }
              </div>
            </div>
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
  logoPreviewUrl: string | null = null;
  channelName = '';
  savingChannel = false;
  uploadingLogo = false;
  brandingError = '';
  brandingNotice = '';

  ngOnInit(): void {
    const id = this.route.snapshot.paramMap.get('id') || '';
    if (!id) {
      this.router.navigate(['/projects']);
      return;
    }
    this.projects.getProject(id).subscribe({
      next: (project) => {
        this.project = project;
        this.channelName = project.branding?.channelName ?? '';
        this.isLoading = false;
        if (project.branding?.logoKey) {
          this.projects.logoUrl(project.id).subscribe({
            next: (res) => { this.logoPreviewUrl = res.url; },
            error: () => {},
          });
        }
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

  saveChannel(): void {
    if (!this.project || this.savingChannel) return;
    this.brandingError = '';
    this.brandingNotice = '';
    this.savingChannel = true;
    this.projects.updateProject(this.project.id, { channelName: this.channelName.trim() }).subscribe({
      next: (updated) => {
        this.project = updated;
        this.savingChannel = false;
        this.brandingNotice = 'Channel handle saved — applies to future renders.';
      },
      error: (err) => {
        this.savingChannel = false;
        this.brandingError = err?.error?.message || 'Could not save. Please try again.';
      },
    });
  }

  onLogoSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    if (!input.files?.length || !this.project || this.uploadingLogo) return;
    const file = input.files[0];
    if (!/^image\/(png|jpeg|webp)$/.test(file.type) || file.size === 0 || file.size > 5 * 1024 * 1024) {
      this.brandingError = 'Choose a PNG, JPG or WebP image up to 5MB.';
      return;
    }
    this.brandingError = '';
    this.brandingNotice = '';
    this.uploadingLogo = true;
    const projectId = this.project.id;
    this.projects.logoUploadUrl(projectId).subscribe({
      next: (res) => {
        this.projects.uploadLogo(res.uploadUrl, file).subscribe({
          next: () => {
            this.uploadingLogo = false;
            this.brandingNotice = 'Logo uploaded — applies to future renders.';
            this.projects.getProject(projectId).subscribe({
              next: (p) => {
                this.project = p;
                this.projects.logoUrl(projectId).subscribe({
                  next: (r) => { this.logoPreviewUrl = r.url; },
                  error: () => {},
                });
              },
            });
          },
          error: () => {
            this.uploadingLogo = false;
            this.brandingError = 'Logo upload failed. Please try again.';
          },
        });
      },
      error: () => {
        this.uploadingLogo = false;
        this.brandingError = 'Could not start logo upload. Please try again.';
      },
    });
  }
}
