import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router, RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { ProjectService } from '../../core/services/project.service';
import { Project, ProjectStatus } from '../../shared/models/project.model';

@Component({
  selector: 'app-projects',
  standalone: true,
  imports: [
    CommonModule,
    RouterLink,
    LucideDynamicIcon,
  ],
  template: `
    <div class="p-6 max-w-7xl mx-auto">
      <div class="flex items-center justify-between mb-6">
        <h1 class="text-2xl font-semibold">Projects</h1>
        <a routerLink="/projects/new" class="glow-btn !py-2 !px-5 !text-sm">
          <svg lucideIcon="plus" [size]="18"></svg>
          Create New Project
        </a>
      </div>

      <div class="glass-card overflow-hidden">
        @if (projects.length === 0) {
          <div class="py-16 text-center text-[hsl(var(--muted-foreground))]">
            <svg lucideIcon="video" [size]="64" class="mx-auto mb-4 opacity-30"></svg>
            <p class="text-lg mb-4">No projects yet</p>
            <a routerLink="/projects/new" class="glow-btn !py-2 !px-5 !text-sm">
              Create Your First Project
            </a>
          </div>
        } @else {
          <!-- Table Header -->
          <div class="hidden sm:grid grid-cols-12 gap-4 px-6 py-3 border-b border-[hsl(var(--border))] text-sm font-semibold text-[hsl(var(--muted-foreground))]">
            <div class="col-span-4">Name</div>
            <div class="col-span-3">Template</div>
            <div class="col-span-3">Status</div>
            <div class="col-span-2">Created</div>
          </div>

          <!-- Table Rows -->
          @for (project of projects; track project.id) {
            <div
              class="grid grid-cols-1 sm:grid-cols-12 gap-2 sm:gap-4 px-6 py-4 border-b border-[hsl(var(--border))]/50 cursor-pointer hover:bg-white/5 transition-colors"
              (click)="navigateToProject(project)"
              (keydown.enter)="navigateToProject(project)"
              tabindex="0"
              role="row"
            >
              <div class="sm:col-span-4 font-medium truncate">{{ project.name }}</div>
              <div class="sm:col-span-3 text-[hsl(var(--muted-foreground))]">{{ project.template | titlecase }}</div>
              <div class="sm:col-span-3">
                <span
                  class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium"
                  [ngClass]="getStatusBadgeClasses(project.status)"
                >
                  <span class="w-1.5 h-1.5 rounded-full" [ngClass]="getStatusDotClass(project.status)"></span>
                  {{ getStatusLabel(project.status) }}
                </span>
              </div>
              <div class="sm:col-span-2 text-sm text-[hsl(var(--muted-foreground))]">{{ formatDate(project.createdAt) }}</div>
            </div>
          }
        }
      </div>
    </div>
  `,
})
export class ProjectsComponent implements OnInit {
  private readonly projectService = inject(ProjectService);
  private readonly router = inject(Router);

  projects: Project[] = [];

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
      'bg-green-500/10 text-green-400': status === 'completed',
      'bg-blue-500/10 text-blue-400':
        status === 'parsing' ||
        status === 'generating_slides' ||
        status === 'narrating' ||
        status === 'rendering',
      'bg-red-500/10 text-red-400': status === 'failed',
      'bg-[hsl(var(--muted))]/50 text-[hsl(var(--muted-foreground))]': status === 'created',
    };
  }

  getStatusDotClass(status: ProjectStatus): Record<string, boolean> {
    return {
      'bg-green-400 shadow-[0_0_6px_rgba(74,222,128,0.5)]': status === 'completed',
      'bg-blue-400 shadow-[0_0_6px_rgba(96,165,250,0.5)]':
        status === 'parsing' ||
        status === 'generating_slides' ||
        status === 'narrating' ||
        status === 'rendering',
      'bg-red-400': status === 'failed',
      'bg-gray-400': status === 'created',
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
