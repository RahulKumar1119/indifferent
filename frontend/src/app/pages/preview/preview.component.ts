import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatCardModule } from '@angular/material/card';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { ProjectService } from '../../core/services/project.service';
import { ApiService } from '../../core/services/api.service';
import { Project } from '../../shared/models/project.model';

@Component({
  selector: 'app-preview',
  standalone: true,
  imports: [
    CommonModule,
    RouterLink,
    MatButtonModule,
    MatIconModule,
    MatCardModule,
    MatProgressSpinnerModule,
  ],
  template: `
    <div class="p-6 max-w-5xl mx-auto">
      @if (loading) {
        <div class="flex justify-center items-center py-24">
          <mat-spinner diameter="48"></mat-spinner>
        </div>
      } @else if (project) {
        @if (project.status !== 'completed') {
          <mat-card class="!shadow-sm">
            <mat-card-content>
              <div class="py-12 text-center text-gray-500">
                <mat-icon class="!text-6xl !w-16 !h-16 mb-4">hourglass_empty</mat-icon>
                <p class="text-lg mb-2">Video is not ready yet</p>
                <p class="text-sm text-gray-400 mb-6">
                  Current status: {{ getStatusLabel(project.status) }}
                </p>
                <a
                  mat-flat-button
                  [routerLink]="['/projects', project.id, 'upload']"
                  class="!bg-indigo-600 !text-white"
                >
                  <mat-icon>visibility</mat-icon>
                  View Progress
                </a>
              </div>
            </mat-card-content>
          </mat-card>
        } @else {
          <!-- Header -->
          <div class="mb-6">
            <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
              {{ project.name }}
            </h1>
            @if (project.completedAt) {
              <p class="text-sm text-gray-500 mt-1">
                Completed on: {{ formatDate(project.completedAt) }}
              </p>
            }
          </div>

          <!-- Video Player -->
          <mat-card class="!shadow-sm mb-6">
            <mat-card-content class="!p-0">
              <div class="relative w-full" style="max-width: 1280px;">
                <div class="aspect-video">
                  <video
                    class="w-full h-full rounded-t-lg"
                    controls
                    [src]="project.videoUrl"
                    [poster]="project.thumbnailUrl"
                    preload="metadata"
                  >
                    Your browser does not support the video tag.
                  </video>
                </div>
              </div>
            </mat-card-content>
          </mat-card>

          <!-- Thumbnail Preview -->
          @if (project.thumbnailUrl) {
            <mat-card class="!shadow-sm mb-6">
              <mat-card-header>
                <mat-card-title class="!text-base !font-medium">Thumbnail</mat-card-title>
              </mat-card-header>
              <mat-card-content>
                <img
                  [src]="project.thumbnailUrl"
                  [alt]="project.name + ' thumbnail'"
                  class="w-full max-w-sm rounded-lg border border-gray-200 dark:border-gray-700"
                />
              </mat-card-content>
            </mat-card>
          }

          <!-- Actions -->
          <div class="flex flex-wrap gap-4">
            <button
              mat-flat-button
              class="!bg-indigo-600 !text-white"
              (click)="downloadVideo()"
              [disabled]="downloading"
            >
              <mat-icon>download</mat-icon>
              {{ downloading ? 'Preparing...' : 'Download Video' }}
            </button>
            <a mat-stroked-button routerLink="/projects">
              <mat-icon>arrow_back</mat-icon>
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
          this.downloading = false;
          const link = document.createElement('a');
          link.href = res.downloadUrl;
          link.download = `${this.project!.name}.mp4`;
          link.target = '_blank';
          document.body.appendChild(link);
          link.click();
          document.body.removeChild(link);
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
