import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router, RouterLink } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatTableModule } from '@angular/material/table';
import { MatChipsModule } from '@angular/material/chips';
import { MatCardModule } from '@angular/material/card';
import { ProjectService } from '../../core/services/project.service';
import { Project, ProjectStatus } from '../../shared/models/project.model';

@Component({
  selector: 'app-projects',
  standalone: true,
  imports: [
    CommonModule,
    RouterLink,
    MatButtonModule,
    MatIconModule,
    MatTableModule,
    MatChipsModule,
    MatCardModule,
  ],
  template: `
    <div class="p-6 max-w-7xl mx-auto">
      <div class="flex items-center justify-between mb-6">
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">Projects</h1>
        <a mat-flat-button routerLink="/projects/new" class="!bg-indigo-600 !text-white">
          <mat-icon>add</mat-icon>
          Create New Project
        </a>
      </div>

      <mat-card class="!shadow-sm overflow-hidden">
        @if (projects.length === 0) {
          <mat-card-content>
            <div class="py-12 text-center text-gray-500">
              <mat-icon class="!text-6xl !w-16 !h-16 mb-4">video_library</mat-icon>
              <p class="text-lg mb-4">No projects yet</p>
              <a mat-flat-button routerLink="/projects/new" class="!bg-indigo-600 !text-white">
                Create Your First Project
              </a>
            </div>
          </mat-card-content>
        } @else {
          <table mat-table [dataSource]="projects" class="w-full">
            <!-- Name Column -->
            <ng-container matColumnDef="name">
              <th mat-header-cell *matHeaderCellDef class="!font-semibold">Name</th>
              <td mat-cell *matCellDef="let project">
                <span class="font-medium text-gray-900 dark:text-white">{{ project.name }}</span>
              </td>
            </ng-container>

            <!-- Template Column -->
            <ng-container matColumnDef="template">
              <th mat-header-cell *matHeaderCellDef class="!font-semibold">Template</th>
              <td mat-cell *matCellDef="let project">
                <span class="text-gray-600 dark:text-gray-300">{{ project.template | titlecase }}</span>
              </td>
            </ng-container>

            <!-- Status Column -->
            <ng-container matColumnDef="status">
              <th mat-header-cell *matHeaderCellDef class="!font-semibold">Status</th>
              <td mat-cell *matCellDef="let project">
                <span
                  class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium"
                  [ngClass]="getStatusBadgeClasses(project.status)"
                >
                  {{ getStatusLabel(project.status) }}
                </span>
              </td>
            </ng-container>

            <!-- Created Column -->
            <ng-container matColumnDef="createdAt">
              <th mat-header-cell *matHeaderCellDef class="!font-semibold">Created</th>
              <td mat-cell *matCellDef="let project">
                <span class="text-gray-500">{{ formatDate(project.createdAt) }}</span>
              </td>
            </ng-container>

            <tr mat-header-row *matHeaderRowDef="displayedColumns"></tr>
            <tr
              mat-row
              *matRowDef="let row; columns: displayedColumns"
              class="cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800"
              (click)="navigateToProject(row)"
            ></tr>
          </table>
        }
      </mat-card>
    </div>
  `,
})
export class ProjectsComponent implements OnInit {
  private readonly projectService = inject(ProjectService);
  private readonly router = inject(Router);

  projects: Project[] = [];
  displayedColumns = ['name', 'template', 'status', 'createdAt'];

  ngOnInit(): void {
    this.projectService.getProjects().subscribe((projects) => {
      this.projects = [...projects].sort(
        (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
      );
    });
  }

  navigateToProject(project: Project): void {
    this.router.navigate(['/projects', project.id]);
  }

  getStatusLabel(status: ProjectStatus): string {
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
      case 'completed':
        return 'Completed';
      case 'failed':
        return 'Failed';
    }
  }

  getStatusBadgeClasses(status: ProjectStatus): Record<string, boolean> {
    return {
      'bg-green-100 text-green-800': status === 'completed',
      'bg-blue-100 text-blue-800':
        status === 'parsing' ||
        status === 'generating_slides' ||
        status === 'narrating' ||
        status === 'rendering',
      'bg-red-100 text-red-800': status === 'failed',
      'bg-gray-100 text-gray-800': status === 'created',
    };
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
