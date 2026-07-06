import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { ProjectService } from '../../core/services/project.service';
import { Project, ProjectStatus } from '../../shared/models/project.model';

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [
    CommonModule,
    RouterLink,
    LucideDynamicIcon,
  ],
  template: `
    <div class="p-6 max-w-7xl mx-auto">
      <div class="flex items-center justify-between mb-8">
        <h1 class="text-2xl font-semibold">Dashboard</h1>
        <a routerLink="/projects/new" class="glow-btn !py-2 !px-5 !text-sm">
          <svg lucideIcon="plus" [size]="18"></svg>
          New Project
        </a>
      </div>

      <!-- Summary Cards -->
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        <div class="glass-card spotlight-card p-5" (mousemove)="onSpotlight($event)">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm text-[hsl(var(--muted-foreground))]">Total Projects</p>
              <p class="text-3xl font-bold mt-1">{{ totalProjects }}</p>
            </div>
            <div class="w-10 h-10 rounded-lg bg-[hsl(var(--primary))]/10 flex items-center justify-center">
              <svg lucideIcon="folder-open" [size]="20" class="text-[hsl(var(--primary))]"></svg>
            </div>
          </div>
        </div>

        <div class="glass-card spotlight-card p-5" (mousemove)="onSpotlight($event)">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm text-[hsl(var(--muted-foreground))]">Completed</p>
              <p class="text-3xl font-bold mt-1 text-green-400">{{ completedCount }}</p>
            </div>
            <div class="w-10 h-10 rounded-lg bg-green-500/10 flex items-center justify-center">
              <svg lucideIcon="circle-check" [size]="20" class="text-green-400"></svg>
            </div>
          </div>
        </div>

        <div class="glass-card spotlight-card p-5" (mousemove)="onSpotlight($event)">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm text-[hsl(var(--muted-foreground))]">In Progress</p>
              <p class="text-3xl font-bold mt-1 text-blue-400">{{ inProgressCount }}</p>
            </div>
            <div class="w-10 h-10 rounded-lg bg-blue-500/10 flex items-center justify-center">
              <svg lucideIcon="clock" [size]="20" class="text-blue-400"></svg>
            </div>
          </div>
        </div>

        <div class="glass-card spotlight-card p-5" (mousemove)="onSpotlight($event)">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm text-[hsl(var(--muted-foreground))]">Failed</p>
              <p class="text-3xl font-bold mt-1 text-red-400">{{ failedCount }}</p>
            </div>
            <div class="w-10 h-10 rounded-lg bg-red-500/10 flex items-center justify-center">
              <svg lucideIcon="circle-x" [size]="20" class="text-red-400"></svg>
            </div>
          </div>
        </div>
      </div>

      <!-- Recent Activity -->
      <div class="glass-card p-6">
        <h2 class="text-lg font-medium mb-4">Recent Activity</h2>
        @if (recentProjects.length === 0) {
          <div class="py-8 text-center text-[hsl(var(--muted-foreground))]">
            <svg lucideIcon="video" [size]="48" class="mx-auto mb-3 opacity-40"></svg>
            <p>No projects yet. Create your first video!</p>
          </div>
        } @else {
          <div class="space-y-3">
            @for (project of recentProjects; track project.id) {
              <a
                [routerLink]="['/projects', project.id]"
                class="flex items-center gap-4 p-3 rounded-lg hover:bg-white/5 transition-colors"
              >
                <svg [lucideIcon]="getStatusIcon(project.status)" [size]="20" [class]="getStatusColor(project.status)"></svg>
                <div class="flex-1 min-w-0">
                  <p class="font-medium truncate">{{ project.name }}</p>
                  <p class="text-sm text-[hsl(var(--muted-foreground))]">
                    {{ project.template | titlecase }} &middot; {{ formatDate(project.createdAt) }}
                  </p>
                </div>
                <svg lucideIcon="chevron-right" [size]="16" class="text-[hsl(var(--muted-foreground))]"></svg>
              </a>
            }
          </div>
        }
      </div>
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

  onSpotlight(event: MouseEvent): void {
    const card = event.currentTarget as HTMLElement;
    const rect = card.getBoundingClientRect();
    const x = ((event.clientX - rect.left) / rect.width) * 100;
    const y = ((event.clientY - rect.top) / rect.height) * 100;
    card.style.setProperty('--mouse-x', `${x}%`);
    card.style.setProperty('--mouse-y', `${y}%`);
  }

  getStatusIcon(status: ProjectStatus): string {
    switch (status) {
      case 'completed':
        return 'circle-check';
      case 'failed':
        return 'circle-x';
      case 'created':
        return 'plus';
      default:
        return 'clock';
    }
  }

  getStatusColor(status: ProjectStatus): string {
    switch (status) {
      case 'completed':
        return 'text-green-400';
      case 'failed':
        return 'text-red-400';
      case 'created':
        return 'text-[hsl(var(--muted-foreground))]';
      default:
        return 'text-blue-400';
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
