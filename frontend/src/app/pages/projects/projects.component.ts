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
  styles: [`
    .serif { font-family: 'Cormorant Garamond', 'Playfair Display', Georgia, serif; }
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
    .btn-primary { transition: transform .2s cubic-bezier(0.23,1,0.32,1), background-color .2s; }
    .btn-primary:hover { transform: translateY(-1px); }
    .btn-primary:active { transform: translateY(1px) scale(.98); }
    .row-hover { transition: background-color .2s; }
    .row-hover:hover { background-color: rgba(26,23,20,0.04); }
    @media (prefers-reduced-motion: reduce) {
      .btn-primary, .row-hover { transition: none; }
    }
  `],
  template: `
    <div class="sans bg-[#FAF7F2] text-[#1A1714] antialiased min-h-[100dvh]">
      <!-- App header -->
      <header class="border-b border-black/[0.07] bg-[#FAF7F2]/90 backdrop-blur sticky top-0 z-40">
        <div class="max-w-7xl mx-auto px-5 md:px-8 h-16 flex items-center justify-between">
          <a routerLink="/" class="font-semibold tracking-tight text-[17px]">Indifferent<span class="text-[#D96C3D]">.</span></a>
          <div class="flex items-center gap-2 sm:gap-3">
            <a routerLink="/" class="hidden sm:inline-flex rounded-full px-4 py-2 text-[13.5px] font-medium text-[#3E3A33] hover:bg-black/[0.05] transition-colors">Home</a>
            <a routerLink="/dashboard" class="hidden sm:inline-flex rounded-full px-4 py-2 text-[13.5px] font-medium text-[#3E3A33] hover:bg-black/[0.05] transition-colors">Dashboard</a>
            <a routerLink="/profile" class="hidden sm:inline-flex rounded-full px-4 py-2 text-[13.5px] font-medium text-[#3E3A33] hover:bg-black/[0.05] transition-colors">Profile</a>
            <a routerLink="/projects/new" class="btn-primary inline-flex items-center gap-2 rounded-full bg-[#1A1714] text-white pl-4 pr-1.5 py-1.5 text-[13.5px] font-semibold">
              New project
              <span class="w-7 h-7 rounded-full bg-white/15 flex items-center justify-center">
                <svg lucideIcon="plus" [size]="15"></svg>
              </span>
            </a>
          </div>
        </div>
      </header>

      <main class="max-w-7xl mx-auto px-5 md:px-8 py-10 md:py-14">
        <div class="flex flex-wrap items-end justify-between gap-4">
          <div>
            <p class="text-[11.5px] uppercase tracking-[0.2em] text-[#6B6560]">Studio · Library</p>
            <h1 class="serif mt-2 font-medium leading-none tracking-[-0.02em] text-[clamp(2.4rem,5vw,3.8rem)]">Projects.</h1>
          </div>
          <p class="text-[13.5px] text-[#6B6560]">{{ projects.length }} {{ projects.length === 1 ? 'film' : 'films' }} in the cutting room</p>
        </div>

        <section class="mt-8 rounded-[20px] bg-white border border-black/[0.07] overflow-hidden">
          @if (projects.length === 0) {
            <div class="py-16 text-center px-6">
              <p class="serif text-[30px]">No projects yet.</p>
              <p class="mt-2 text-[14.5px] text-[#6B6560]">Upload a TXT file and render your first quiz video.</p>
              <a routerLink="/projects/new" class="btn-primary mt-6 inline-flex items-center gap-2 rounded-full bg-[#1A1714] text-white px-7 py-3.5 font-semibold text-[14.5px]">
                Create your first project →
              </a>
            </div>
          } @else {
            <!-- Table Header -->
            <div class="hidden sm:grid grid-cols-12 gap-4 px-6 py-3.5 border-b border-black/[0.07] text-[11.5px] font-semibold uppercase tracking-[0.12em] text-[#6B6560]">
              <div class="col-span-4">Name</div>
              <div class="col-span-3">Template</div>
              <div class="col-span-3">Status</div>
              <div class="col-span-2">Created</div>
            </div>

            <!-- Table Rows -->
            @for (project of projects; track project.id) {
              <div
                class="row-hover grid grid-cols-1 sm:grid-cols-12 gap-1.5 sm:gap-4 px-6 py-4 border-b border-black/[0.05] last:border-b-0 cursor-pointer"
                (click)="navigateToProject(project)"
                (keydown.enter)="navigateToProject(project)"
                tabindex="0"
                role="row"
              >
                <div class="sm:col-span-4 font-medium truncate text-[15px]">{{ project.name }}</div>
                <div class="sm:col-span-3 text-[13.5px] text-[#6B6560]">{{ project.template | titlecase }}</div>
                <div class="sm:col-span-3">
                  <span
                    class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium"
                    [ngClass]="getStatusBadgeClasses(project.status)"
                  >
                    <span class="w-1.5 h-1.5 rounded-full" [ngClass]="getStatusDotClass(project.status)"></span>
                    {{ getStatusLabel(project.status) }}
                  </span>
                </div>
                <div class="sm:col-span-2 text-[13px] text-[#6B6560]">{{ formatDate(project.createdAt) }}</div>
              </div>
            }
          }
        </section>

        <footer class="mt-10 pt-6 border-t border-black/[0.07] flex flex-col sm:flex-row justify-between gap-2 text-[12.5px] text-[#6B6560]">
          <span>© {{ currentYear }} Indifferent. All rights reserved.</span>
          <span class="flex gap-5">
            <a routerLink="/" class="hover:text-[#1A1714]">Home</a>
            <a routerLink="/dashboard" class="hover:text-[#1A1714]">Dashboard</a>
            <a routerLink="/contact" class="hover:text-[#1A1714]">Contact</a>
          </span>
        </footer>
      </main>
    </div>
  `,
})
export class ProjectsComponent implements OnInit {
  private readonly projectService = inject(ProjectService);
  private readonly router = inject(Router);

  currentYear = new Date().getFullYear();
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
      'bg-[#1E3A2A]/10 text-[#1E3A2A]': status === 'completed',
      'bg-[#956400]/10 text-[#956400]':
        status === 'parsing' ||
        status === 'generating_slides' ||
        status === 'narrating' ||
        status === 'rendering',
      'bg-[#BC5227]/10 text-[#BC5227]': status === 'failed',
      'bg-black/[0.05] text-[#6B6560]': status === 'created',
    };
  }

  getStatusDotClass(status: ProjectStatus): Record<string, boolean> {
    return {
      'bg-[#1E3A2A]': status === 'completed',
      'bg-[#956400]':
        status === 'parsing' ||
        status === 'generating_slides' ||
        status === 'narrating' ||
        status === 'rendering',
      'bg-[#BC5227]': status === 'failed',
      'bg-[#6B6560]': status === 'created',
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
