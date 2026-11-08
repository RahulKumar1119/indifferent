import { Component, OnInit, inject } from '@angular/core';
import { HttpContext } from '@angular/common/http';
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
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { EmailConfirmBannerComponent } from '../../shared/components/email-confirm-banner/email-confirm-banner.component';
import { ApiService } from '../../core/services/api.service';
import { SUPPRESS_GLOBAL_ERROR } from '../../core/interceptors/error.interceptor';
import { UserProfile } from '../../shared/models/user-profile.model';
import { DashboardSidebarComponent } from './dashboard-sidebar.component';
import { ActivityChartComponent } from './activity-chart.component';
import { StatusRingComponent } from './status-ring.component';
import { DayBucket, greetingFor, lastDays } from './activity';

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
    NavbarComponent,
    EmailConfirmBannerComponent,
    DashboardSidebarComponent,
    ActivityChartComponent,
    StatusRingComponent,
  ],
  styles: [`
    :host { --page: var(--color-paper-2); --card: var(--color-paper); display: block; min-height: 100dvh; background: var(--page); color: var(--color-ink); font-family: var(--font-body); }
    :host-context(.dark) { --page: var(--color-paper); --card: var(--color-paper-2); }
    .side { display: none; }
    @media (min-width: 64rem) { .top { display: none; } .shell { display: grid; grid-template-columns: 15rem minmax(0, 1fr); } .side { display: block; } }
    main { max-width: 76rem; margin: 0 auto; padding: var(--space-lg) var(--space-md) var(--space-2xl); display: flex; flex-direction: column; gap: var(--space-lg); }
    @media (min-width: 40rem) { main { padding: var(--space-xl) var(--space-lg) var(--space-2xl); } }
    .card { background: var(--card); border: 1px solid var(--color-rule); border-radius: var(--radius-surface); box-shadow: 0 1px 2px color-mix(in oklab, var(--color-ink) 5%, transparent), 0 8px 24px -16px color-mix(in oklab, var(--color-ink) 18%, transparent); padding: var(--space-lg); }
    .num { font-variant-numeric: lining-nums tabular-nums; }
    .head { display: flex; flex-wrap: wrap; align-items: flex-end; justify-content: space-between; gap: var(--space-md); }
    h1 { font-family: var(--font-display); font-weight: 500; font-size: var(--type-3xl); line-height: 1.05; letter-spacing: var(--tracking-display); }
    .meta { margin-top: var(--space-2xs); font-size: var(--type-sm); color: var(--color-muted); }
    h2 { font-size: var(--type-lg); font-weight: 600; line-height: 1.2; }
    .notice { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-sm); font-size: var(--type-sm); border-left: 4px solid var(--color-caution); }
    .notice.err { border-left-color: var(--color-negative); }
    .stats { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-sm); }
    @media (min-width: 60rem) { .stats { grid-template-columns: repeat(4, minmax(0, 1fr)); gap: var(--space-md); } }
    .stat { padding: var(--space-md); display: flex; flex-direction: column; gap: var(--space-2xs); }
    @media (min-width: 40rem) { .stat { padding: var(--space-lg); } }
    .stat-top { display: flex; align-items: center; justify-content: space-between; font-size: var(--type-sm); font-weight: 600; }
    .chip { width: 2.25rem; height: 2.25rem; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; flex: none; background: color-mix(in oklab, var(--color-accent) 12%, var(--card)); color: var(--color-accent-text); }
    .stat-n { margin-top: var(--space-xs); font-size: var(--type-2xl); font-weight: 600; line-height: 1; }
    .stat-s { font-size: var(--type-xs); color: var(--color-muted); }
    .stat.total { background: var(--color-accent); border-color: var(--color-accent); color: var(--color-accent-ink); }
    .stat.total .stat-s { color: var(--color-accent-ink); }
    .stat.total .chip { background: color-mix(in oklab, var(--color-accent-ink) 18%, var(--color-accent)); color: var(--color-accent-ink); }
    .charts { display: grid; gap: var(--space-lg); }
    @media (min-width: 60rem) { .charts { grid-template-columns: minmax(0, 3fr) minmax(0, 2fr); } }
    .sec-head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 0 var(--space-md); }
    .links { display: flex; flex-wrap: wrap; gap: 0 var(--space-md); }
    .links a { display: inline-flex; align-items: center; min-height: 44px; font-size: var(--type-sm); }
    .tools { margin-top: var(--space-sm); display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-sm); }
    @media (min-width: 40rem) { .tools { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
    @media (min-width: 72rem) { .tools { grid-template-columns: repeat(5, minmax(0, 1fr)); gap: var(--space-md); } }
    .tool { height: 100%; display: flex; flex-direction: column; gap: var(--space-sm); padding: var(--space-md); color: var(--color-ink); text-decoration: none; transition: border-color var(--dur-short) var(--ease-out), transform var(--dur-short) var(--ease-out); }
    .tool:hover { border-color: var(--color-accent); transform: translateY(-2px); }
    .tool:focus-visible, .row:focus-visible, .job:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; }
    .tool-top { display: flex; align-items: center; justify-content: space-between; color: var(--color-muted); }
    .t-name { font-weight: 600; }
    .t-note { font-size: var(--type-xs); color: var(--color-muted); }
    .work { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-lg); align-items: start; }
    @media (min-width: 60rem) { .work.two { grid-template-columns: minmax(0, 2fr) minmax(0, 1fr); } }
    .rows { margin-top: var(--space-sm); }
    .rows li + li { border-top: 1px solid var(--color-rule); }
    .row, .job { display: flex; align-items: center; gap: var(--space-sm); min-height: 44px; padding: var(--space-sm) var(--space-xs); border-radius: var(--radius-control); color: var(--color-ink); text-decoration: none; }
    .row:hover, .job:hover { background: var(--color-paper-3); }
    .r-main { flex: 1; min-width: 0; }
    .r-title { display: block; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .r-meta { display: flex; flex-wrap: wrap; gap: 0 var(--space-sm); font-size: var(--type-xs); color: var(--color-muted); }
    .pill { flex: none; display: inline-flex; align-items: center; min-height: 1.5rem; padding: 0 var(--space-xs); border-radius: var(--radius-pill); font-size: var(--type-xs); font-weight: 600; border: 1px solid currentColor; }
    .s-done { color: var(--color-positive); } .s-failed { color: var(--color-negative); }
    .s-working { color: var(--color-caution); } .s-draft { color: var(--color-ink-2); }
    .arrow { flex: none; color: var(--color-muted); }
    .jobs { margin-top: var(--space-sm); display: flex; flex-direction: column; gap: var(--space-2xs); }
    .hint { margin-top: var(--space-2xs); font-size: var(--type-xs); color: var(--color-muted); }
    .empty { margin-top: var(--space-sm); padding: var(--space-xl) var(--space-md); text-align: center; border-radius: var(--radius-control); background: var(--color-paper-3); }
    .empty p:first-child { font-size: var(--type-lg); font-weight: 600; }
    .empty p + p { margin: var(--space-2xs) auto var(--space-md); max-width: 36ch; color: var(--color-ink-2); }
    @media (max-width: 39.99rem) { .card { padding: var(--space-md); } .row .chip, .row .arrow { display: none; } .row { padding-inline: 0; } }
    @media (prefers-reduced-motion: reduce) { .tool { transition: none; } .tool:hover { transform: none; } }
  `],
  template: `
    <app-navbar class="top" />
    <div class="shell">
      <app-dashboard-sidebar class="side" />
      <main id="main" class="hm">
        <app-email-confirm-banner />
        <div class="head">
          <div>
            <h1>{{ greeting }}</h1>
            <p class="meta">{{ today }}</p>
            <p class="meta num" data-testid="subtitle">{{ total }}{{ anyMore ? '+' : '' }} {{ total === 1 && !anyMore ? 'piece' : 'pieces' }} of work in the cutting room</p>
          </div>
          <a routerLink="/new" class="ui-btn ui-btn--primary">
            <svg lucideIcon="plus" [size]="18" aria-hidden="true"></svg>
            New
          </a>
        </div>

        @if (loadError) {
          <div class="card notice err" role="alert">
            <p><strong>Couldn't load your work.</strong> Check your connection and try again.</p>
            <button type="button" (click)="reload()" class="ui-btn ui-btn--secondary">Retry</button>
          </div>
        }
        @if (!isLoading && !loadError && missing.length > 0) {
          <div class="card notice" role="status" data-testid="partial">
            <p><strong>Some work could not be loaded:</strong> {{ missing.join(', ') }}. The rest is shown below.</p>
            <button type="button" (click)="reload()" class="ui-btn ui-btn--secondary">Retry</button>
          </div>
        }

        @if (isLoading && !loadError) {
          <div role="status" aria-label="Loading your work" class="stats">
            @for (i of [1, 2, 3, 4]; track i) {
              <div class="card stat"><div class="ui-skeleton" style="height: 5.5rem"></div></div>
            }
          </div>
          <div class="card"><div class="ui-skeleton" style="height: 12rem"></div></div>
        }

        @if (!isLoading) {
          <div class="stats">
            @for (s of stats; track s.id) {
              <div class="card stat" [class.total]="s.id === 'total'">
                <div class="stat-top">
                  <span>{{ s.label }}</span>
                  <span class="chip"><svg [lucideIcon]="s.icon" [size]="18" aria-hidden="true"></svg></span>
                </div>
                <p class="stat-n num" [attr.data-testid]="s.id">{{ statValue(s.id) }}</p>
                <p class="stat-s">{{ s.note }}</p>
              </div>
            }
          </div>

          @if (total > 0) {
            <div class="charts">
              <section class="card" aria-labelledby="activity-heading">
                <h2 id="activity-heading">Activity, last 14 days</h2>
                <app-activity-chart [days]="days" [partial]="anyMore || missing.length > 0" />
              </section>
              <section class="card" aria-labelledby="status-heading">
                <h2 id="status-heading">By status</h2>
                <app-status-ring [done]="doneCount" [cutting]="inProgressCount" [retakes]="failedCount" />
              </section>
            </div>
          }

          <section aria-labelledby="tools-heading">
            <h2 id="tools-heading">Create something</h2>
            <ul class="tools">
              @for (tool of tools; track tool.label) {
                <li>
                  <a [routerLink]="tool.link" class="card tool">
                    <span class="tool-top">
                      <span class="chip"><svg [lucideIcon]="tool.icon" [size]="18" aria-hidden="true"></svg></span>
                      <svg lucideIcon="arrow-up-right" [size]="16" aria-hidden="true"></svg>
                    </span>
                    <span>
                      <span class="t-name" style="display: block">{{ tool.label }}</span>
                      <span class="t-note num">{{ toolNote(tool) }}</span>
                    </span>
                  </a>
                </li>
              }
            </ul>
          </section>

          <div class="work" [class.two]="working.length > 0">
            <section class="card" aria-labelledby="recent-heading">
              <div class="sec-head">
                <h2 id="recent-heading">Recent work</h2>
                <span class="links">
                  <a routerLink="/projects" class="ui-link">All projects</a>
                  <a routerLink="/shorts/history" class="ui-link">All shorts</a>
                </span>
              </div>
              @if (recent.length === 0) {
                <div class="empty">
                  <p>Nothing here yet.</p>
                  <p>Make a quiz video, cut shorts from a recording, or turn a story into a video.</p>
                  <a routerLink="/projects/new" class="ui-btn ui-btn--primary">Create your first video</a>
                </div>
              } @else {
                <ul class="rows divide-y">
                  @for (item of recent; track item.id) {
                    <li>
                      <a [routerLink]="item.link" class="row">
                        <span class="chip"><svg [lucideIcon]="kindIcon[item.kind]" [size]="18" aria-hidden="true"></svg></span>
                        <span class="r-main">
                          <span class="r-title">{{ item.title }}</span>
                          <span class="r-meta num">
                            <span>{{ kindLabel[item.kind] }}</span>
                            <span>{{ formatDate(item.date) }}</span>
                            @if (item.detail) {
                              <span>{{ item.detail }}</span>
                            }
                          </span>
                        </span>
                        <span class="pill" [class]="stateClass(item.state)">{{ stateLabel[item.state] }}</span>
                        <svg lucideIcon="chevron-right" [size]="16" class="arrow" aria-hidden="true"></svg>
                      </a>
                    </li>
                  }
                </ul>
              }
            </section>

            @if (working.length > 0) {
              <section class="card" aria-labelledby="rendering-heading" data-testid="rendering">
                <h2 id="rendering-heading">Rendering now</h2>
                <p class="hint">Open a job to follow its progress.</p>
                <ul class="jobs">
                  @for (item of working; track item.id) {
                    <li>
                      <a [routerLink]="progressLink(item)" class="job">
                        <span class="chip s-working"><svg lucideIcon="clock" [size]="18" aria-hidden="true"></svg></span>
                        <span class="r-main">
                          <span class="r-title">{{ item.title }}</span>
                          <span class="r-meta num"><span>{{ kindLabel[item.kind] }}</span><span>{{ formatDate(item.date) }}</span></span>
                        </span>
                        <svg lucideIcon="chevron-right" [size]="16" class="arrow" aria-hidden="true"></svg>
                      </a>
                    </li>
                  }
                </ul>
              </section>
            }
          </div>
        }
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
  private readonly api = inject(ApiService);

  greeting = greetingFor(new Date(), null);
  readonly today = new Date().toLocaleDateString(undefined, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });
  /** Work per day for the activity chart, from every item that loaded. */
  days: DayBucket[] = lastDays([]);
  /** Every item still being made, for the Rendering now panel. */
  working: WorkItem[] = [];

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
  readonly kindIcon: Record<WorkKind, string> = {
    quiz: 'film',
    short: 'scissors',
    'story-video': 'clapperboard',
    'story-plan': 'pencil',
    thumbnails: 'image',
  };
  readonly stats = [
    { id: 'total', label: 'Total', note: 'All work', icon: 'folder-open' },
    { id: 'done', label: 'Ready', note: 'Completed', icon: 'circle-check' },
    { id: 'inProgress', label: 'Cutting', note: 'Rendering or draft', icon: 'clock' },
    { id: 'failed', label: 'Retakes', note: 'Failed', icon: 'circle-x' },
  ] as const;
  readonly tools: Tool[] = [
    { label: 'Quiz videos', icon: 'film', link: '/projects/new', kind: 'quiz', empty: 'Make one', noun: ['project', 'projects'] },
    { label: 'AI Shorts', icon: 'scissors', link: '/shorts', kind: 'short', empty: 'Cut your first', noun: ['job', 'jobs'] },
    { label: 'Story video', icon: 'clapperboard', link: '/story', kind: 'story-video', empty: 'Tell a story', noun: ['video', 'videos'] },
    { label: 'AI thumbnails', icon: 'image', link: '/shorts/history', kind: 'thumbnails', empty: 'Made from a short', noun: ['set', 'sets'] },
    { label: 'Watermark tool', icon: 'stamp', link: '/tools/add-watermark', kind: null, empty: 'Add a watermark to a video', noun: ['', ''] },
  ];

  ngOnInit(): void {
    this.reload();
    // The greeting only needs the name; without it the page still works.
    this.api
      .get<UserProfile>('/auth/me', undefined, true, new HttpContext().set(SUPPRESS_GLOBAL_ERROR, true))
      .subscribe({ next: (p) => (this.greeting = greetingFor(new Date(), p?.name)), error: () => undefined });
  }

  statValue(id: 'total' | 'done' | 'inProgress' | 'failed'): string {
    if (id === 'total') return `${this.total}${this.anyMore ? '+' : ''}`;
    return String(id === 'done' ? this.doneCount : id === 'inProgress' ? this.inProgressCount : this.failedCount);
  }

  /** A quiz project's row opens the project; while it renders, its progress page is the useful one. */
  progressLink(item: WorkItem): string[] {
    return item.kind === 'quiz' ? [...item.link, 'progress'] : item.link;
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
      this.days = lastDays(work.items);
      this.working = work.items.filter((i) => i.state === 'working').slice(0, 5);
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

  stateClass(state: WorkState): string {
    return `s-${state}`;
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
