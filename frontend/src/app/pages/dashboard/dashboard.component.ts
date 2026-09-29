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
            <a routerLink="/projects" class="hidden sm:inline-flex rounded-full px-4 py-2 text-[13.5px] font-medium text-[#3E3A33] hover:bg-black/[0.05] transition-colors">Projects</a>
            <a routerLink="/blog" class="hidden sm:inline-flex rounded-full px-4 py-2 text-[13.5px] font-medium text-[#3E3A33] hover:bg-black/[0.05] transition-colors">Journal</a>
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
            <p class="text-[11.5px] uppercase tracking-[0.2em] text-[#6B6560]">Studio · Overview</p>
            <h1 class="serif mt-2 font-medium leading-none tracking-[-0.02em] text-[clamp(2.4rem,5vw,3.8rem)]">Dashboard.</h1>
          </div>
          <p class="text-[13.5px] text-[#6B6560]">{{ totalProjects }} {{ totalProjects === 1 ? 'film' : 'films' }} in the cutting room</p>
        </div>

        <!-- Summary cards -->
        <div class="mt-8 grid grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="rounded-[20px] bg-white border border-black/[0.07] p-5 md:p-6">
            <div class="flex items-start justify-between">
              <p class="text-[12px] uppercase tracking-[0.14em] text-[#6B6560]">Total</p>
              <span class="w-8 h-8 rounded-full bg-[#1A1714]/[.06] flex items-center justify-center">
                <svg lucideIcon="folder-open" [size]="16" class="text-[#1A1714]"></svg>
              </span>
            </div>
            <p class="serif text-[44px] leading-none mt-2">{{ totalProjects }}</p>
            <p class="text-[12.5px] text-[#6B6560] mt-1">Projects</p>
          </div>
          <div class="rounded-[20px] bg-white border border-black/[0.07] p-5 md:p-6">
            <div class="flex items-start justify-between">
              <p class="text-[12px] uppercase tracking-[0.14em] text-[#6B6560]">Ready</p>
              <span class="w-8 h-8 rounded-full bg-[#1E3A2A]/10 flex items-center justify-center">
                <svg lucideIcon="circle-check" [size]="16" class="text-[#1E3A2A]"></svg>
              </span>
            </div>
            <p class="serif text-[44px] leading-none mt-2 text-[#1E3A2A]">{{ completedCount }}</p>
            <p class="text-[12.5px] text-[#6B6560] mt-1">Completed</p>
          </div>
          <div class="rounded-[20px] bg-white border border-black/[0.07] p-5 md:p-6">
            <div class="flex items-start justify-between">
              <p class="text-[12px] uppercase tracking-[0.14em] text-[#6B6560]">Cutting</p>
              <span class="w-8 h-8 rounded-full bg-[#956400]/10 flex items-center justify-center">
                <svg lucideIcon="clock" [size]="16" class="text-[#956400]"></svg>
              </span>
            </div>
            <p class="serif text-[44px] leading-none mt-2 text-[#956400]">{{ inProgressCount }}</p>
            <p class="text-[12.5px] text-[#6B6560] mt-1">In progress</p>
          </div>
          <div class="rounded-[20px] bg-white border border-black/[0.07] p-5 md:p-6">
            <div class="flex items-start justify-between">
              <p class="text-[12px] uppercase tracking-[0.14em] text-[#6B6560]">Retakes</p>
              <span class="w-8 h-8 rounded-full bg-[#BC5227]/10 flex items-center justify-center">
                <svg lucideIcon="circle-x" [size]="16" class="text-[#BC5227]"></svg>
              </span>
            </div>
            <p class="serif text-[44px] leading-none mt-2 text-[#BC5227]">{{ failedCount }}</p>
            <p class="text-[12.5px] text-[#6B6560] mt-1">Failed</p>
          </div>
        </div>

        <!-- Recent activity -->
        <section class="mt-6 rounded-[20px] bg-white border border-black/[0.07] p-5 md:p-8">
          <div class="flex items-center justify-between mb-2">
            <h2 class="serif text-[26px]">Recent activity</h2>
            <a routerLink="/projects" class="text-[13.5px] font-semibold underline underline-offset-8 decoration-black/25 hover:decoration-black transition">All projects →</a>
          </div>
          @if (recentProjects.length === 0) {
            <div class="py-12 text-center">
              <p class="serif text-[28px]">No films yet.</p>
              <p class="mt-2 text-[14.5px] text-[#6B6560]">Upload a TXT file and render your first quiz video.</p>
              <a routerLink="/projects/new" class="btn-primary mt-6 inline-flex items-center gap-2 rounded-full bg-[#1A1714] text-white px-7 py-3.5 font-semibold text-[14.5px]">
                Create your first video →
              </a>
            </div>
          } @else {
            <ul class="divide-y divide-black/[0.07]">
              @for (project of recentProjects; track project.id) {
                <li>
                  <a
                    [routerLink]="['/projects', project.id]"
                    class="row-hover flex items-center gap-4 px-2 md:px-3 py-4 rounded-xl"
                  >
                    <span class="w-9 h-9 rounded-full bg-[#1A1714]/[.05] flex items-center justify-center shrink-0">
                      <svg [lucideIcon]="getStatusIcon(project.status)" [size]="17" [class]="getStatusColor(project.status)"></svg>
                    </span>
                    <span class="flex-1 min-w-0">
                      <span class="block font-medium truncate text-[15px]">{{ project.name }}</span>
                      <span class="block text-[12.5px] text-[#6B6560]">
                        {{ project.template | titlecase }} &middot; {{ formatDate(project.createdAt) }}
                      </span>
                    </span>
                    <span class="hidden sm:inline-flex rounded-full border border-black/10 px-3 py-1 text-[11.5px] font-medium" [class]="getStatusColor(project.status)">{{ project.status | titlecase }}</span>
                    <svg lucideIcon="chevron-right" [size]="16" class="text-[#6B6560] shrink-0"></svg>
                  </a>
                </li>
              }
            </ul>
          }
        </section>

        <footer class="mt-10 pt-6 border-t border-black/[0.07] flex flex-col sm:flex-row justify-between gap-2 text-[12.5px] text-[#6B6560]">
          <span>© {{ currentYear }} Indifferent. All rights reserved.</span>
          <span class="flex gap-5">
            <a routerLink="/" class="hover:text-[#1A1714]">Home</a>
            <a routerLink="/blog" class="hover:text-[#1A1714]">Journal</a>
            <a routerLink="/contact" class="hover:text-[#1A1714]">Contact</a>
          </span>
        </footer>
      </main>
    </div>
  `,
})
export class DashboardComponent implements OnInit {
  private readonly projectService = inject(ProjectService);

  currentYear = new Date().getFullYear();
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
        return 'text-[#1E3A2A]';
      case 'failed':
        return 'text-[#BC5227]';
      case 'created':
        return 'text-[#6B6560]';
      default:
        return 'text-[#956400]';
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
