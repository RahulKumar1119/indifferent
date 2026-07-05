import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { MatCardModule } from '@angular/material/card';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatListModule } from '@angular/material/list';
import { MatBadgeModule } from '@angular/material/badge';
import { ProjectService } from '../../core/services/project.service';
import { Project, ProjectStatus } from '../../shared/models/project.model';

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [
    CommonModule,
    RouterLink,
    MatCardModule,
    MatButtonModule,
    MatIconModule,
    MatListModule,
    MatBadgeModule,
  ],
  template: `
    <div class="p-6 max-w-7xl mx-auto">
      <div class="flex items-center justify-between mb-8">
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">Dashboard</h1>
        <a mat-flat-button routerLink="/projects/new" class="!bg-indigo-600 !text-white">
          <mat-icon>add</mat-icon>
          New Project
        </a>
      </div>

      <!-- Summary Cards -->
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        <mat-card class="!shadow-sm">
          <mat-card-content class="!p-4">
            <div class="flex items-center justify-between">
              <div>
                <p class="text-sm text-gray-500">Total Projects</p>
                <p class="text-3xl font-bold text-gray-900 dark:text-white">{{ totalProjects }}</p>
              </div>
              <mat-icon class="!text-4xl !w-10 !h-10 text-gray-400">folder</mat-icon>
            </div>
          </mat-card-content>
        </mat-card>

        <mat-card class="!shadow-sm">
          <mat-card-content class="!p-4">
            <div class="flex items-center justify-between">
              <div>
                <p class="text-sm text-gray-500">Completed</p>
                <p class="text-3xl font-bold text-green-600">{{ completedCount }}</p>
              </div>
              <mat-icon class="!text-4xl !w-10 !h-10 text-green-400">check_circle</mat-icon>
            </div>
          </mat-card-content>
        </mat-card>

        <mat-card class="!shadow-sm">
          <mat-card-content class="!p-4">
            <div class="flex items-center justify-between">
              <div>
                <p class="text-sm text-gray-500">In Progress</p>
                <p class="text-3xl font-bold text-blue-600">{{ inProgressCount }}</p>
              </div>
              <mat-icon class="!text-4xl !w-10 !h-10 text-blue-400">pending</mat-icon>
            </div>
          </mat-card-content>
        </mat-card>

        <mat-card class="!shadow-sm">
          <mat-card-content class="!p-4">
            <div class="flex items-center justify-between">
              <div>
                <p class="text-sm text-gray-500">Failed</p>
                <p class="text-3xl font-bold text-red-600">{{ failedCount }}</p>
              </div>
              <mat-icon class="!text-4xl !w-10 !h-10 text-red-400">error</mat-icon>
            </div>
          </mat-card-content>
        </mat-card>
      </div>

      <!-- Recent Activity -->
      <mat-card class="!shadow-sm">
        <mat-card-header>
          <mat-card-title class="!text-lg !font-medium">Recent Activity</mat-card-title>
        </mat-card-header>
        <mat-card-content>
          @if (recentProjects.length === 0) {
            <div class="py-8 text-center text-gray-500">
              <mat-icon class="!text-5xl !w-12 !h-12 mb-2">movie_creation</mat-icon>
              <p>No projects yet. Create your first video!</p>
            </div>
          } @else {
            <mat-list>
              @for (project of recentProjects; track project.id) {
                <a mat-list-item [routerLink]="['/projects', project.id]">
                  <mat-icon matListItemIcon [class]="getStatusColor(project.status)">
                    {{ getStatusIcon(project.status) }}
                  </mat-icon>
                  <span matListItemTitle>{{ project.name }}</span>
                  <span matListItemLine class="text-sm text-gray-500">
                    {{ project.template | titlecase }} &middot; {{ formatDate(project.createdAt) }}
                  </span>
                </a>
              }
            </mat-list>
          }
        </mat-card-content>
      </mat-card>
    </div>
  `,
})
export class DashboardComponent implements OnInit {
  private readonly projectService = inject(ProjectService);

  projects: Project[] = [];
  recentProjects: Project[] = [];
  totalProjects = 0;
  completedCount = 0;
  inProgressCount = 0;
  failedCount = 0;

  ngOnInit(): void {
    this.projectService.getProjects().subscribe((projects) => {
      this.projects = projects;
      this.totalProjects = projects.length;
      this.completedCount = projects.filter((p) => p.status === 'completed').length;
      this.inProgressCount = projects.filter((p) =>
        ['parsing', 'generating_slides', 'narrating', 'rendering'].includes(p.status),
      ).length;
      this.failedCount = projects.filter((p) => p.status === 'failed').length;
      this.recentProjects = [...projects]
        .sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime())
        .slice(0, 5);
    });
  }

  getStatusIcon(status: ProjectStatus): string {
    switch (status) {
      case 'completed':
        return 'check_circle';
      case 'failed':
        return 'error';
      case 'created':
        return 'radio_button_unchecked';
      default:
        return 'pending';
    }
  }

  getStatusColor(status: ProjectStatus): string {
    switch (status) {
      case 'completed':
        return 'text-green-600';
      case 'failed':
        return 'text-red-600';
      case 'created':
        return 'text-gray-400';
      default:
        return 'text-blue-600';
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
