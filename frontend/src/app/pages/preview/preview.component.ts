import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { ProjectService } from '../../core/services/project.service';
import { ApiService } from '../../core/services/api.service';
import { Project } from '../../shared/models/project.model';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { projectStatusLabel } from '../projects/project-status';

@Component({
  selector: 'app-preview',
  standalone: true,
  imports: [CommonModule, RouterLink, LucideDynamicIcon, NavbarComponent],
  styles: [`
    :host { display: block; }
    .narrow { width: min(60rem, 100%); }
    .back { display: inline-flex; align-items: center; min-height: 44px; font-size: var(--type-sm); align-self: flex-start; }
    .name { overflow-wrap: anywhere; }
    .player { padding: 0; overflow: hidden; background: oklch(0% 0 0); }
    .player video { display: block; width: 100%; aspect-ratio: 16 / 9; }
    .actions { display: flex; flex-wrap: wrap; gap: var(--space-xs); }
    @media (max-width: 39.99rem) { .actions .ui-btn { flex: 1 1 100%; } }
    .notice { margin: 0; font-size: var(--type-sm); color: var(--color-ink-2); }
    .thumb { display: block; width: 100%; max-width: 24rem; height: auto; margin-top: var(--space-sm); border: 1px solid var(--color-rule); border-radius: var(--radius-control); }
    .ui-empty .ui-btn, .ui-error-card .ui-btn { margin-top: var(--space-xs); }
  `],
  template: `
    <div class="ui-app">
      <app-navbar />
      <main id="main" class="ui-app-main narrow hm">
        <a routerLink="/projects" class="ui-link back">All projects</a>
        @if (loading) {
          <div role="status" aria-label="Loading video" data-testid="loading">
            <div class="ui-skeleton" style="height: 2.25rem; width: 50%"></div>
            <div class="ui-skeleton" style="margin-top: var(--space-md); aspect-ratio: 16 / 9"></div>
          </div>
        } @else if (!project) {
          <div class="ui-card ui-error-card" role="alert" data-testid="not-found">
            <h1 class="ui-h1">Video not found</h1>
            <span class="ui-card-sub">This project doesn't exist or you don't have access to it.</span>
            <a routerLink="/projects" class="ui-btn ui-btn--secondary">Back to your projects</a>
          </div>
        } @else if (project.status !== 'completed') {
          <div class="ui-card ui-empty" data-testid="not-ready">
            <span class="ui-chip" aria-hidden="true"><svg lucideIcon="clock" [size]="18"></svg></span>
            <h1 class="ui-h1">Your video is not ready yet</h1>
            <p>Status: {{ statusLabel(project.status) }}</p>
            <a [routerLink]="['/projects', project.id, 'progress']" class="ui-btn ui-btn--primary">Track progress</a>
          </div>
        } @else {
          <div>
            <h1 class="ui-h1 name">{{ project.name }}</h1>
            @if (project.completedAt) {
              <p class="ui-lead">Finished {{ formatDate(project.completedAt) }}</p>
            }
          </div>

          <div class="ui-card player">
            <video controls [src]="project.videoUrl" [poster]="project.thumbnailUrl" preload="metadata" [attr.aria-label]="project.name + ' quiz video'"></video>
          </div>

          <div class="actions">
            <button type="button" class="ui-btn ui-btn--primary" (click)="downloadVideo()" [disabled]="downloading" [attr.aria-busy]="downloading" data-testid="download">
              @if (downloading) {
                <svg lucideIcon="loader-2" [size]="18" class="ui-spin" aria-hidden="true"></svg>
                Preparing…
              } @else {
                <svg lucideIcon="download" [size]="18" aria-hidden="true"></svg>
                Download video
              }
            </button>
            <a [routerLink]="['/projects', project.id]" class="ui-btn ui-btn--secondary">Project settings</a>
            <a routerLink="/projects" class="ui-btn ui-btn--quiet">Back to projects</a>
          </div>
          <p class="notice" role="status" data-testid="notice">{{ notice }}</p>

          @if (project.thumbnailUrl) {
            <section class="ui-card" aria-labelledby="thumb-title">
              <h2 id="thumb-title" class="ui-card-title">Thumbnail</h2>
              <img [src]="project.thumbnailUrl" [alt]="project.name + ' thumbnail'" class="thumb" />
            </section>
          }
        }
      </main>
    </div>
  `,
})
export class PreviewComponent implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly projectService = inject(ProjectService);
  private readonly apiService = inject(ApiService);

  project: Project | null = null;
  loading = true;
  downloading = false;
  notice = '';
  readonly statusLabel = projectStatusLabel;

  ngOnInit(): void {
    const id = this.route.snapshot.paramMap.get('id');
    if (id) {
      this.projectService.getProject(id).subscribe({
        next: (project) => {
          this.project = project;
          this.loading = false;
        },
        error: () => {
          this.loading = false;
        },
      });
    } else {
      // No :id in the route: stop spinning and show not-found below.
      this.loading = false;
    }
  }

  downloadVideo(): void {
    if (!this.project || this.downloading) return;

    this.downloading = true;
    this.notice = '';
    this.apiService
      .get<{ downloadUrl?: string; code?: string; message?: string }>(`/projects/${this.project.id}/download`)
      .subscribe({
        next: (res) => {
          if (!res.downloadUrl) {
            // 202 RESTORING lands here (2xx): the archived video is being
            // restored to a downloadable copy.
            this.downloading = false;
            this.notice = res.message || 'This video is in cold storage and is being restored. Please check back in a few hours.';
            return;
          }
          // Use fetch to download the file as a blob, then trigger download
          fetch(res.downloadUrl)
            .then(response => response.blob())
            .then(blob => {
              const url = window.URL.createObjectURL(blob);
              const link = document.createElement('a');
              link.href = url;
              link.download = `${this.project!.name || 'video'}.mp4`;
              document.body.appendChild(link);
              link.click();
              document.body.removeChild(link);
              window.URL.revokeObjectURL(url);
              this.downloading = false;
            })
            .catch(() => {
              this.downloading = false;
              this.notice = 'Download failed. Please try again.';
            });
        },
        error: () => {
          this.downloading = false;
          this.notice = 'Download failed. Please try again.';
        },
      });
  }

  formatDate(dateStr: string): string {
    const date = new Date(dateStr);
    return date.toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    });
  }
}
