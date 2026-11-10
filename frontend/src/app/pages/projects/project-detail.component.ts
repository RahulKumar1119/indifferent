import { Component, OnDestroy, OnInit, inject } from '@angular/core';
import { Subject, takeUntil } from 'rxjs';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { ProjectService } from '../../core/services/project.service';
import { Project } from '../../shared/models/project.model';
import { ShortsJob, ShortsService } from '../shorts/shorts.service';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { projectStatusLabel, projectStatusTone } from './project-status';

@Component({
  selector: 'app-project-detail',
  standalone: true,
  imports: [CommonModule, FormsModule, RouterLink, LucideDynamicIcon, NavbarComponent],
  template: `
    <div class="ui-app">
      <app-navbar />
      <main id="main" class="ui-app-main narrow hm">
        <a routerLink="/projects" class="ui-link back">All projects</a>

        @if (isLoading) {
          <div class="stack" role="status" aria-label="Loading project">
            <div class="ui-skeleton" style="height: 2.5rem; width: 60%"></div>
            <div class="ui-skeleton" style="height: 10rem"></div>
          </div>
        } @else if (error) {
          <div class="ui-card ui-error-card" role="alert">
            <p><svg lucideIcon="triangle-alert" [size]="18" aria-hidden="true"></svg>{{ error }}</p>
            <a routerLink="/projects" class="ui-btn ui-btn--secondary">Back to your projects</a>
          </div>
        } @else if (project) {
          <div class="ui-page-head">
            <h1 class="ui-h1 name">{{ project.name }}</h1>
            <span [class]="'ui-pill ' + tone(project.status)" data-testid="status">{{ statusLabel(project.status) }}</span>
          </div>

          <!-- Parse warnings: things that did not stop the render but did affect the result -->
          @if (project.warnings && project.warnings.length > 0) {
            <section class="ui-alert ui-alert--warn warn" role="alert" aria-labelledby="project-warnings-heading">
              <h2 id="project-warnings-heading">
                {{ project.warnings!.length === 1 ? 'One issue' : project.warnings!.length + ' issues' }} with your file
              </h2>
              <ul>
                @for (w of project.warnings; track w.line + '-' + w.reason) {
                  <li><strong>Question {{ w.line }}:</strong> {{ w.reason }}.</li>
                }
              </ul>
              <a routerLink="/blog/quiz-file-format-guide" class="ui-link">How to mark the answer</a>
            </section>
          }

          <section class="ui-card" aria-labelledby="film-title">
            <h2 id="film-title" class="ui-card-title">Quiz video</h2>
            <dl>
              <div><dt>Design</dt><dd class="cap">{{ project.template }}</dd></div>
              <div><dt>Voice</dt><dd>{{ project.voice }}</dd></div>
            </dl>
            @if (project.status === 'created') {
              <!-- Nothing uploaded yet: the upload is the one thing left to do. -->
              <div class="next" data-testid="next-upload">
                <p><strong>Next: upload your questions.</strong> Nothing renders until the project has its TXT file.</p>
                <a [routerLink]="['/projects', project.id, 'upload']" class="ui-btn ui-btn--primary">Upload questions</a>
              </div>
            } @else {
              <div class="ui-row actions">
                @if (project.status === 'completed') {
                  <a [routerLink]="['/projects', project.id, 'preview']" class="ui-btn ui-btn--primary"><svg lucideIcon="play" [size]="18" aria-hidden="true"></svg>Watch and download</a>
                }
                <a [routerLink]="['/projects', project.id, 'progress']" class="ui-btn ui-btn--secondary">Track progress</a>
                <a [routerLink]="['/projects', project.id, 'upload']" class="ui-btn ui-btn--quiet">Upload a new file</a>
              </div>
            }
          </section>

          <section class="ui-card" aria-labelledby="shorts-title">
            <div class="ui-row">
              <h2 id="shorts-title" class="ui-card-title">Shorts</h2>
              <span class="ui-spacer"></span>
              @if (shortsJobs.length > 0) {
                <span class="ui-card-sub" style="margin: 0">{{ shortsJobs.length }} linked</span>
              }
            </div>
            @if (shortsJobs.length === 0) {
              <p class="ui-card-sub">No shorts linked to this project yet.</p>
            } @else {
              <ul class="jobs">
                @for (job of shortsJobs; track job.jobId) {
                  <li>
                    <span class="job">Short {{ job.jobId.slice(0, 8) }} <span class="ui-card-sub">{{ job.status }}</span></span>
                    <span class="ui-row">
                      <a [routerLink]="['/shorts', job.jobId, 'progress']" class="ui-btn ui-btn--quiet" [attr.aria-label]="'Progress of short ' + job.jobId.slice(0, 8)">Progress</a>
                      @if (job.status === 'completed') {
                        <a [routerLink]="['/shorts', job.jobId, 'clips']" class="ui-btn ui-btn--secondary" [attr.aria-label]="'Clips of short ' + job.jobId.slice(0, 8)">Clips</a>
                      }
                    </span>
                  </li>
                }
              </ul>
            }
          </section>

          <section class="ui-card" aria-labelledby="brand-title">
            <h2 id="brand-title" class="ui-card-title">Branding</h2>
            <p class="ui-card-sub">Your logo sits top right on quiz videos and shorts.</p>
            <div class="brand">
              <div class="logo">
                @if (logoPreviewUrl) {
                  <img [src]="logoPreviewUrl" alt="Brand logo" />
                } @else {
                  <span class="ui-hint">No logo</span>
                }
              </div>
              <div class="brand-fields">
                <div class="ui-f">
                  <label for="channel-handle" class="ui-flabel">Channel handle</label>
                  <div class="ui-row">
                    <input id="channel-handle" [(ngModel)]="channelName" placeholder="@yourchannel" maxlength="60" class="ui-field handle" />
                    <button type="button" (click)="saveChannel()" [disabled]="savingChannel" [attr.aria-busy]="savingChannel" class="ui-btn ui-btn--secondary">
                      {{ savingChannel ? 'Saving…' : 'Save handle' }}
                    </button>
                  </div>
                </div>
                <label class="ui-btn ui-btn--secondary file-btn">
                  <input type="file" accept=".png,.jpg,.jpeg,.webp" class="sr" (change)="onLogoSelected($event)" />
                  <svg lucideIcon="upload" [size]="18" aria-hidden="true"></svg>
                  {{ uploadingLogo ? 'Uploading…' : (project.branding?.logoKey ? 'Replace logo' : 'Upload logo') }}
                </label>
                @if (brandingError) {
                  <p class="ui-alert ui-alert--bad" role="alert">{{ brandingError }}</p>
                }
                @if (brandingNotice) {
                  <p class="ok" role="status">{{ brandingNotice }}</p>
                }
              </div>
            </div>
          </section>

          <section class="ui-card" aria-labelledby="wm-title">
            <h2 id="wm-title" class="ui-card-title">Watermark</h2>
            @if (project.watermark?.text) {
              <dl>
                <div><dt>Text</dt><dd>“{{ project.watermark!.text }}”</dd></div>
                @if (project.watermark!.color) {
                  <div><dt>Colour</dt><dd>{{ project.watermark!.color }}</dd></div>
                }
                @if (project.watermark!.opacity != null) {
                  <div><dt>Opacity</dt><dd>{{ project.watermark!.opacity }}</dd></div>
                }
              </dl>
            } @else {
              <p class="ui-card-sub">No watermark saved for this project.</p>
            }
            <div class="actions">
              <a routerLink="/tools/add-watermark" class="ui-btn ui-btn--secondary">Open watermark tool</a>
            </div>
          </section>
        }
      </main>
    </div>
  `,
  styles: [`
    :host { display: block; }
    .narrow { width: min(52rem, 100%); }
    .back { display: inline-flex; align-items: center; min-height: 44px; font-size: var(--type-sm); align-self: flex-start; }
    .stack { display: flex; flex-direction: column; gap: var(--space-md); }
    .name { overflow-wrap: anywhere; }
    .t-ok { color: var(--color-positive); } .t-bad { color: var(--color-negative); } .t-wait { color: var(--color-caution); } .t-new { color: var(--color-ink-2); }
    .warn { margin: 0; }
    .warn h2 { margin: 0; font-size: var(--type-md); font-weight: 600; }
    .warn ul { margin: var(--space-xs) 0; padding-left: var(--space-lg); list-style: disc; }
    .warn li + li { margin-top: var(--space-3xs); }
    .warn a { display: inline-flex; align-items: center; min-height: 44px; font-weight: 600; }
    dl { margin: var(--space-md) 0 0; display: grid; grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr)); gap: var(--space-sm); }
    dt { font-size: var(--type-xs); color: var(--color-muted); }
    dd { margin: 0; font-weight: 500; overflow-wrap: anywhere; }
    .cap { text-transform: capitalize; }
    .actions { margin-top: var(--space-md); }
    .next { margin-top: var(--space-md); display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-sm); padding: var(--space-sm) var(--space-md); border-radius: var(--radius-control); background: color-mix(in oklab, var(--color-accent) 8%, var(--app-card)); border: 1px solid color-mix(in oklab, var(--color-accent) 40%, transparent); }
    .next p { margin: 0; font-size: var(--type-sm); }
    .jobs { margin: var(--space-sm) 0 0; padding: 0; list-style: none; }
    .jobs li { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-xs); padding: var(--space-2xs) 0; }
    .jobs li + li { border-top: 1px solid var(--color-rule); }
    .job { font-weight: 500; }
    .brand { margin-top: var(--space-md); display: flex; flex-wrap: wrap; gap: var(--space-md); align-items: flex-start; }
    .logo { width: 5rem; height: 5rem; flex: none; border: 1px solid var(--color-rule); border-radius: var(--radius-control); background: var(--color-paper-2); display: flex; align-items: center; justify-content: center; overflow: hidden; }
    .logo img { max-width: 100%; max-height: 100%; object-fit: contain; }
    .brand-fields { flex: 1 1 16rem; min-width: 0; display: flex; flex-direction: column; align-items: flex-start; gap: var(--space-sm); }
    .brand-fields .ui-f { align-self: stretch; }
    .handle { flex: 1 1 12rem; width: auto; min-width: 0; }
    .sr { position: absolute; width: 1px; height: 1px; margin: -1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
    .file-btn { position: relative; }
    .file-btn:focus-within { outline: 2px solid var(--color-focus); outline-offset: 2px; }
    .ok { margin: 0; font-size: var(--type-sm); color: var(--color-positive); font-weight: 500; }
    .brand-fields .ui-alert { margin: 0; }
  `],
})
export class ProjectDetailComponent implements OnInit, OnDestroy {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly projects = inject(ProjectService);
  private readonly shorts = inject(ShortsService);
  private readonly destroy$ = new Subject<void>();
  readonly statusLabel = projectStatusLabel;
  readonly tone = projectStatusTone;

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
    this.projects.getProject(id).pipe(takeUntil(this.destroy$)).subscribe({
      next: (project) => {
        this.project = project;
        this.channelName = project.branding?.channelName ?? '';
        this.isLoading = false;
        if (project.branding?.logoKey) {
          this.projects.logoUrl(project.id).pipe(takeUntil(this.destroy$)).subscribe({
            next: (res) => { this.logoPreviewUrl = res.url; },
            error: () => {},
          });
        }
        for (const jobId of project.shortsJobIds ?? []) {
          this.shorts.getStatus(jobId).pipe(takeUntil(this.destroy$)).subscribe({
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

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
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
        this.brandingNotice = 'Channel handle saved. Applies to future renders.';
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
    this.projects.logoUploadUrl(projectId, file.size).subscribe({
      next: (res) => {
        this.projects.uploadLogo(res.uploadUrl, file).subscribe({
          next: () => {
            this.uploadingLogo = false;
            this.brandingNotice = 'Logo uploaded. Applies to future renders.';
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
