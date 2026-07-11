import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { ProjectService } from '../../core/services/project.service';
import { ApiService } from '../../core/services/api.service';
import { Project } from '../../shared/models/project.model';

@Component({
  selector: 'app-preview',
  standalone: true,
  imports: [
    CommonModule,
    RouterLink,
    LucideDynamicIcon,
  ],
  template: `
    <div class="p-6 max-w-5xl mx-auto">
      @if (loading) {
        <div class="flex justify-center items-center py-24">
          <svg lucideIcon="loader-2" [size]="48" class="animate-spin text-[hsl(var(--primary))]"></svg>
        </div>
      } @else if (project) {
        @if (project.status !== 'completed') {
          <div class="glass-card p-12 text-center">
            <svg lucideIcon="clock" [size]="64" class="mx-auto mb-4 text-[hsl(var(--muted-foreground))] opacity-50"></svg>
            <p class="text-lg mb-2">Video is not ready yet</p>
            <p class="text-sm text-[hsl(var(--muted-foreground))] mb-6">
              Current status: {{ getStatusLabel(project.status) }}
            </p>
            <a [routerLink]="['/projects', project.id, 'upload']" class="glow-btn">
              <svg lucideIcon="play" [size]="18"></svg>
              View Progress
            </a>
          </div>
        } @else {
          <!-- Header -->
          <div class="mb-6">
            <h1 class="text-2xl font-semibold">{{ project.name }}</h1>
            @if (project.completedAt) {
              <p class="text-sm text-[hsl(var(--muted-foreground))] mt-1">
                Completed on: {{ formatDate(project.completedAt) }}
              </p>
            }
          </div>

          <!-- Video Player -->
          <div class="glass-card overflow-hidden mb-6">
            <div class="relative w-full" style="max-width: 1280px;">
              <div class="aspect-video">
                <video
                  class="w-full h-full"
                  controls
                  [src]="project.videoUrl"
                  [poster]="project.thumbnailUrl"
                  preload="metadata"
                >
                  Your browser does not support the video tag.
                </video>
              </div>
            </div>
          </div>

          <!-- Thumbnail Preview -->
          @if (project.thumbnailUrl) {
            <div class="glass-card p-4 mb-6">
              <p class="text-sm font-medium mb-3">Thumbnail</p>
              <img
                [src]="project.thumbnailUrl"
                [alt]="project.name + ' thumbnail'"
                class="w-full max-w-sm rounded-lg border border-[hsl(var(--border))]"
              />
            </div>
          }

          <!-- Actions -->
          <div class="flex flex-wrap gap-4">
            <button
              class="glow-btn"
              (click)="downloadVideo()"
              [disabled]="downloading"
              [class.opacity-50]="downloading"
            >
              @if (downloading) {
                <svg lucideIcon="loader-2" [size]="18" class="animate-spin"></svg>
                Preparing...
              } @else {
                <svg lucideIcon="download" [size]="18"></svg>
                Download Video
              }
            </button>
            <a
              routerLink="/projects"
              class="inline-flex items-center gap-2 px-5 py-3 rounded-lg border border-[hsl(var(--border))] hover:bg-white/5 transition-colors font-medium"
            >
              <svg lucideIcon="arrow-right" [size]="18" class="rotate-180"></svg>
              Back to Projects
            </a>
          </div>
        }
      }
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
    }
  }

  downloadVideo(): void {
    if (!this.project || this.downloading) return;

    this.downloading = true;
    this.apiService
      .get<{ downloadUrl: string }>(`/projects/${this.project.id}/download`)
      .subscribe({
        next: (res) => {
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
            });
        },
        error: () => {
          this.downloading = false;
        },
      });
  }

  getStatusLabel(status: string): string {
    switch (status) {
      case 'created':
        return 'Created';
      case 'parsing':
        return 'Parsing';
      case 'generating_slides':
        return 'Generating Slides';
      case 'narrating':
        return 'Narrating';
      case 'rendering':
        return 'Rendering';
      case 'failed':
        return 'Failed';
      default:
        return status;
    }
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
