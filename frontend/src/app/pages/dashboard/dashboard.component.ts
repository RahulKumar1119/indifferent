import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { ProjectService } from '../../core/services/project.service';
import { Observable, catchError, forkJoin, of } from 'rxjs';
import { ShortsService } from '../shorts/shorts.service';
import { StoryPlanService } from '../story/story-plan.service';
import { StoryVideoService } from '../story/story-video.service';
import { ThumbnailService } from '../thumbnails/thumbnail.service';
import { buildRecentWork, RecentWork, WORK_KINDS, WorkItem, WorkKind, WorkState } from './recent-work';
import { EmailConfirmBannerComponent } from '../../shared/components/email-confirm-banner/email-confirm-banner.component';

interface Tool {
  label: string;
  icon: string;
  link: string;
  kind: WorkKind | null;
  /** What the card says when none has been made yet. */
  empty: string;
  noun: [string, string];
}

const RECENT_LIMIT = 10;

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [
    CommonModule,
    RouterLink,
    LucideDynamicIcon,
    EmailConfirmBannerComponent,
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
            <a routerLink="/profile" class="hidden sm:inline-flex rounded-full px-4 py-2 text-[13.5px] font-medium text-[#3E3A33] hover:bg-black/[0.05] transition-colors">Profile</a>
            <a routerLink="/profile" class="sm:hidden w-9 h-9 rounded-full hover:bg-black/[0.05] inline-flex items-center justify-center text-[#3E3A33]" aria-label="Profile">
              <svg lucideIcon="user" [size]="17"></svg>
            </a>
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
        <app-email-confirm-banner class="mb-8 block" />
        <div class="flex flex-wrap items-end justify-between gap-4">
          <div>
            <p class="text-[11.5px] uppercase tracking-[0.08em] text-[#6B6560]">Studio · Overview</p>
            <h1 class="serif mt-2 font-medium leading-none tracking-[-0.02em] text-[clamp(2.4rem,5vw,3.8rem)]">Dashboard.</h1>
          </div>
          <p class="text-[13.5px] text-[#6B6560]" data-testid="subtitle">{{ total }}{{ anyMore ? '+' : '' }} {{ total === 1 && !anyMore ? 'piece' : 'pieces' }} of work in the cutting room</p>
        </div>

        @if (loadError) {
          <div class="mt-6 rounded-[20px] border border-red-500/30 bg-red-500/[0.06] p-5 flex flex-wrap items-center justify-between gap-3" role="alert">
            <p class="text-[14px]"><span class="font-semibold">Couldn't load your work.</span> Check your connection and try again.</p>
            <button type="button" (click)="reload()" class="rounded-full bg-[#1A1714] text-white px-5 py-2.5 text-[13.5px] font-semibold">Retry</button>
          </div>
        }

        @if (!isLoading && !loadError && missing.length > 0) {
          <div class="mt-6 rounded-[20px] border border-[#956400]/30 bg-[#956400]/[0.06] p-5 flex flex-wrap items-center justify-between gap-3" role="status" data-testid="partial">
            <p class="text-[14px]"><span class="font-semibold">Some work could not be loaded:</span> {{ missing.join(', ') }}. The rest is shown below.</p>
            <button type="button" (click)="reload()" class="min-h-11 rounded-full bg-[#1A1714] text-white px-5 py-2.5 text-[13.5px] font-semibold">Retry</button>
          </div>
        }
        @if (isLoading && !loadError) {
          <div class="mt-8 flex flex-col gap-4">
            <div class="skeleton h-[24px] w-[18%]"></div>
            <div class="grid grid-cols-2 lg:grid-cols-4 gap-4">
              @for (i of [1, 2, 3, 4]; track i) {
                <div class="rounded-[20px] bg-white border border-black/[0.07] p-5">
                  <div class="skeleton h-[44px] mb-2"></div>
                  <div class="skeleton h-[14px] w-[35%]"></div>
                </div>
              }
            </div>
            <section class="mt-6 rounded-[20px] bg-white border border-black/[0.07] p-5">
              <h2 class="skeleton h-[26px] w-[40%]"></h2>
              <div class="skeleton h-[16px] w-[60%] mt-2"></div>
              <div class="skeleton h-[14px] w-[100%] mt-3"></div>
              <div class="skeleton h-[14px] w-[100%] mt-2"></div>
            </section>
          </div>
        }

        @if (!isLoading) {
          <!-- Summary cards -->
        <div class="mt-8 grid grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="rounded-[20px] bg-white border border-black/[0.07] p-5 md:p-6">
            <div class="flex items-start justify-between">
              <p class="text-[12px] uppercase tracking-[0.14em] text-[#6B6560]">Total</p>
              <span class="w-8 h-8 rounded-full bg-[#1A1714]/[.06] flex items-center justify-center">
                <svg lucideIcon="folder-open" [size]="16" class="text-[#1A1714]"></svg>
              </span>
            </div>
            <p class="serif text-[44px] leading-none mt-2" data-testid="total">{{ total }}{{ anyMore ? '+' : '' }}</p>
            <p class="text-[12.5px] text-[#6B6560] mt-1">All work</p>
          </div>
          <div class="rounded-[20px] bg-white border border-black/[0.07] p-5 md:p-6">
            <div class="flex items-start justify-between">
              <p class="text-[12px] uppercase tracking-[0.14em] text-[#6B6560]">Ready</p>
              <span class="w-8 h-8 rounded-full bg-[#1E3A2A]/10 flex items-center justify-center">
                <svg lucideIcon="circle-check" [size]="16" class="text-[#1E3A2A]"></svg>
              </span>
            </div>
            <p class="serif text-[44px] leading-none mt-2 text-[#1E3A2A]" data-testid="done">{{ doneCount }}</p>
            <p class="text-[12.5px] text-[#6B6560] mt-1">Completed</p>
          </div>
          <div class="rounded-[20px] bg-white border border-black/[0.07] p-5 md:p-6">
            <div class="flex items-start justify-between">
              <p class="text-[12px] uppercase tracking-[0.14em] text-[#6B6560]">Cutting</p>
              <span class="w-8 h-8 rounded-full bg-[#956400]/10 flex items-center justify-center">
                <svg lucideIcon="clock" [size]="16" class="text-[#956400]"></svg>
              </span>
            </div>
            <p class="serif text-[44px] leading-none mt-2 text-[#956400]" data-testid="inProgress">{{ inProgressCount }}</p>
            <p class="text-[12.5px] text-[#6B6560] mt-1">Rendering or draft</p>
          </div>
          <div class="rounded-[20px] bg-white border border-black/[0.07] p-5 md:p-6">
            <div class="flex items-start justify-between">
              <p class="text-[12px] uppercase tracking-[0.14em] text-[#6B6560]">Retakes</p>
              <span class="w-8 h-8 rounded-full bg-[#BC5227]/10 flex items-center justify-center">
                <svg lucideIcon="circle-x" [size]="16" class="text-[#BC5227]"></svg>
              </span>
            </div>
            <p class="serif text-[44px] leading-none mt-2 text-[#BC5227]" data-testid="failed">{{ failedCount }}</p>
            <p class="text-[12.5px] text-[#6B6560] mt-1">Failed</p>
          </div>
        </div>
        <!-- Tools -->
        <section class="mt-6" aria-labelledby="tools-heading">
          <h2 id="tools-heading" class="serif text-[26px] mb-3">Your tools</h2>
          <ul class="grid grid-cols-2 lg:grid-cols-5 gap-4">
            @for (tool of tools; track tool.label) {
              <li>
                <a
                  [routerLink]="tool.link"
                  class="row-hover flex h-full min-h-11 flex-col gap-2 rounded-[20px] bg-white border border-black/[0.07] p-5"
                >
                  <span class="w-8 h-8 rounded-full bg-[#1A1714]/[.06] flex items-center justify-center">
                    <svg [lucideIcon]="tool.icon" [size]="16" class="text-[#1A1714]"></svg>
                  </span>
                  <span class="font-medium text-[15px]">{{ tool.label }}</span>
                  <span class="text-[12.5px] text-[#6B6560]">{{ toolNote(tool) }}</span>
                </a>
              </li>
            }
          </ul>
        </section>
        <!-- Recent work -->
        <section class="mt-6 rounded-[20px] bg-white border border-black/[0.07] p-5 md:p-8">
          <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 mb-2">
            <h2 class="serif text-[26px]">Recent work</h2>
            <span class="flex flex-wrap gap-x-5 gap-y-1">
              <a routerLink="/projects" class="inline-flex min-h-11 items-center text-[13.5px] font-semibold underline underline-offset-8 decoration-black/25 hover:decoration-black transition">All projects</a>
              <a routerLink="/shorts/history" class="inline-flex min-h-11 items-center text-[13.5px] font-semibold underline underline-offset-8 decoration-black/25 hover:decoration-black transition">All shorts</a>
            </span>
          </div>
          @if (recent.length === 0) {
            <div class="py-12 text-center">
              <p class="serif text-[28px]">Nothing here yet.</p>
              <p class="mt-2 text-[14.5px] text-[#6B6560]">Make a quiz video, cut shorts from a recording, or turn a story into a video.</p>
              <a routerLink="/projects/new" class="btn-primary mt-6 inline-flex items-center gap-2 rounded-full bg-[#1A1714] text-white px-7 py-3.5 font-semibold text-[14.5px]">
                Create your first video →
              </a>
            </div>
          } @else {
            <ul class="divide-y divide-black/[0.07]">
              @for (item of recent; track item.id) {
                <li>
                  <a [routerLink]="item.link" class="row-hover flex items-center gap-4 px-2 md:px-3 py-4 rounded-xl">
                    <span class="w-9 h-9 rounded-full bg-[#1A1714]/[.05] flex items-center justify-center shrink-0">
                      <svg [lucideIcon]="stateIcon(item.state)" [size]="17" [class]="stateColor(item.state)"></svg>
                    </span>
                    <span class="flex-1 min-w-0">
                      <span class="block font-medium truncate text-[15px]">{{ item.title }}</span>
                      <span class="block text-[12.5px] text-[#6B6560]">
                        {{ kindLabel[item.kind] }} &middot; {{ formatDate(item.date) }}
                        @if (item.detail) {
                          &middot; {{ item.detail }}
                        }
                      </span>
                    </span>
                    <span class="sm:hidden sr-only">{{ stateLabel[item.state] }}</span>
                    <span class="hidden sm:inline-flex rounded-full border border-black/10 px-3 py-1 text-[11.5px] font-medium" [class]="stateColor(item.state)">{{ stateLabel[item.state] }}</span>
                    <svg lucideIcon="chevron-right" [size]="16" class="text-[#6B6560] shrink-0"></svg>
                  </a>
                </li>
              }
            </ul>
          }
        </section>
        }
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
  private readonly shortsService = inject(ShortsService);
  private readonly storyPlans = inject(StoryPlanService);
  private readonly storyVideos = inject(StoryVideoService);
  private readonly thumbnails = inject(ThumbnailService);
  currentYear = new Date().getFullYear();

  recent: WorkItem[] = [];
  counts: RecentWork['counts'] = buildRecentWork({ projects: null, shorts: null, videos: null, plans: null, thumbnails: null }).counts;
  total = 0;
  doneCount = 0;
  inProgressCount = 0;
  failedCount = 0;
  /** A capped source left some out, so counts are lower bounds. */
  anyMore = false;
  /** Names of the kinds whose source failed to load. */
  missing: string[] = [];
  loadError = false;
  isLoading = true;

  readonly kindLabel: Record<WorkKind, string> = {
    quiz: 'Quiz video',
    short: 'AI Shorts',
    'story-video': 'Story video',
    'story-plan': 'Story plan',
    thumbnails: 'Thumbnails',
  };
  readonly stateLabel: Record<WorkState, string> = {
    done: 'Completed',
    working: 'In progress',
    failed: 'Failed',
    draft: 'Draft',
  };
  readonly tools: Tool[] = [
    { label: 'Quiz videos', icon: 'film', link: '/projects/new', kind: 'quiz', empty: 'Make one', noun: ['project', 'projects'] },
    { label: 'AI Shorts', icon: 'scissors', link: '/shorts', kind: 'short', empty: 'Cut your first', noun: ['job', 'jobs'] },
    { label: 'Story video', icon: 'clapperboard', link: '/story', kind: 'story-video', empty: 'Tell a story', noun: ['video', 'videos'] },
    { label: 'AI thumbnails', icon: 'image', link: '/shorts/history', kind: 'thumbnails', empty: 'Made from a short', noun: ['set', 'sets'] },
    { label: 'Watermark tool', icon: 'stamp', link: '/tools/add-watermark', kind: null, empty: 'Add a watermark to a video', noun: ['', ''] },
  ];

  ngOnInit(): void {
    this.reload();
  }

  reload(): void {
    this.loadError = false;
    this.isLoading = true;
    // Each source fails on its own, so one slow or broken list never hides the rest.
    const safe = <T>(o: Observable<T>) => o.pipe(catchError(() => of(null)));
    forkJoin({
      projects: safe(this.projectService.getProjects()),
      shorts: safe(this.shortsService.listJobs()),
      videos: safe(this.storyVideos.list()),
      plans: safe(this.storyPlans.list()),
      thumbnails: safe(this.thumbnails.list()),
    }).subscribe((src) => {
      const loaded = Object.values(src).filter((v) => v !== null).length;
      const work = buildRecentWork(src);
      this.counts = work.counts;
      this.recent = work.items.slice(0, RECENT_LIMIT);
      this.total = work.items.length;
      this.doneCount = work.items.filter((i) => i.state === 'done').length;
      this.inProgressCount = work.items.filter((i) => i.state === 'working' || i.state === 'draft').length;
      this.failedCount = work.items.filter((i) => i.state === 'failed').length;
      this.anyMore = Object.values(work.counts).some((c) => c?.more);
      this.missing = WORK_KINDS.filter((k) => work.counts[k] === null).map((k) => this.kindLabel[k]);
      this.loadError = loaded === 0;
      this.isLoading = false;
    });
  }

  /** The second line of a tool card: how many are made, or a prompt to start. */
  toolNote(tool: Tool): string {
    if (!tool.kind) return tool.empty;
    const c = this.counts[tool.kind];
    if (c === null) return '—';
    if (c.n === 0 && !c.more) return tool.empty;
    return `${c.n}${c.more ? '+' : ''} ${c.n === 1 && !c.more ? tool.noun[0] : tool.noun[1]}`;
  }

  stateIcon(state: WorkState): string {
    return { done: 'circle-check', failed: 'circle-x', draft: 'pencil', working: 'clock' }[state];
  }

  stateColor(state: WorkState): string {
    return { done: 'text-[#1E3A2A]', failed: 'text-[#9B3D1C]', draft: 'text-[#6B6560]', working: 'text-[#956400]' }[state];
  }

  formatDate(dateStr: string): string {
    const date = new Date(dateStr);
    if (Number.isNaN(date.getTime())) return '';
    return date.toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    });
  }
}
