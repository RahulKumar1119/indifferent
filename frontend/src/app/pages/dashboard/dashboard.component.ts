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
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
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
    NavbarComponent,
    EmailConfirmBannerComponent,
  ],
  styles: [`
    :host { display: block; background: var(--color-paper); color: var(--color-ink); min-height: 100dvh; font-family: var(--font-body); }
    main { max-width: 64rem; margin: 0 auto; padding: var(--space-xl) var(--space-md) var(--space-2xl); }
    h1 { font-size: var(--type-2xl); font-weight: 600; line-height: 1.1; }
    h2 { font-size: var(--type-lg); font-weight: 600; line-height: 1.2; }
    .head { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: var(--space-xs) var(--space-md); }
    .sub { font-size: var(--type-sm); color: var(--color-ink-2); }
    .num { font-variant-numeric: lining-nums tabular-nums; }
    .notice { margin-top: var(--space-md); padding: var(--space-sm) var(--space-md); border-left: 3px solid var(--color-caution); background: var(--color-paper-2); display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-sm); font-size: var(--type-sm); }
    .notice.err { border-left-color: var(--color-negative); }
    .stats { margin-top: var(--space-lg); display: grid; grid-template-columns: repeat(2, 1fr); border-top: 1px solid var(--color-rule); border-bottom: 1px solid var(--color-rule); }
    .stat { padding: var(--space-md) var(--space-md) var(--space-md) 0; }
    .stat:nth-child(even) { padding-left: var(--space-md); border-left: 1px solid var(--color-rule); }
    .stat:nth-child(n+3) { border-top: 1px solid var(--color-rule); }
    .stat-n { font-size: var(--type-xl); font-weight: 600; line-height: 1; }
    .stat-l { margin-top: var(--space-2xs); font-size: var(--type-sm); color: var(--color-ink-2); }
    section { margin-top: var(--space-xl); }
    .sec-head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 0 var(--space-md); }
    .links { display: flex; flex-wrap: wrap; gap: 0 var(--space-md); }
    .links a, .rows a, .tools a { display: flex; align-items: center; min-height: 44px; }
    .links a { font-size: var(--type-sm); }
    .tools { margin-top: var(--space-xs); display: grid; grid-template-columns: repeat(2, 1fr); border-top: 1px solid var(--color-rule); }
    .tools li { border-bottom: 1px solid var(--color-rule); }
    .tools li:nth-child(odd) { padding-right: var(--space-md); }
    .tools li:nth-child(even) { padding-left: var(--space-md); border-left: 1px solid var(--color-rule); }
    .tools a { height: 100%; gap: var(--space-sm); padding: var(--space-sm) 0; border-radius: var(--radius-control); }
    .tools a:hover .t-name { color: var(--color-accent-text); }
    .t-name { font-weight: 600; font-size: var(--type-md); display: block; }
    .t-note { font-size: var(--type-xs); color: var(--color-ink-2); display: block; }
    .icon { flex: none; color: var(--color-ink-2); }
    .rows { margin-top: var(--space-xs); border-top: 1px solid var(--color-rule); }
    .rows li { border-bottom: 1px solid var(--color-rule); }
    .rows a { gap: var(--space-sm); padding: var(--space-xs) var(--space-2xs); }
    .rows a:hover { background: var(--color-paper-2); }
    .r-main { flex: 1; min-width: 0; }
    .r-title { display: block; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .r-meta { display: flex; flex-wrap: wrap; gap: 0 var(--space-sm); font-size: var(--type-xs); color: var(--color-ink-2); }
    .state { display: inline-flex; align-items: center; gap: var(--space-2xs); font-size: var(--type-xs); font-weight: 500; flex: none; }
    .s-done { color: var(--color-positive); } .s-failed { color: var(--color-negative); }
    .s-working { color: var(--color-caution); } .s-draft { color: var(--color-ink-2); }
    .empty { padding: var(--space-xl) 0; border-top: 1px solid var(--color-rule); margin-top: var(--space-xs); }
    .empty p:first-child { font-size: var(--type-lg); font-weight: 600; }
    .empty p + p { margin-top: var(--space-2xs); color: var(--color-ink-2); }
    .empty a { margin-top: var(--space-md); }
    @media (min-width: 640px) {
      main { padding-inline: var(--space-lg); }
      .stats { grid-template-columns: repeat(4, 1fr); }
      .stat, .stat:nth-child(even), .stat:nth-child(n+3) { padding: var(--space-md); border-left: 1px solid var(--color-rule); border-top: 0; }
      .stat:first-child, .stat:nth-child(1) { padding-left: 0; border-left: 0; }
      .tools { grid-template-columns: repeat(5, 1fr); }
      .tools li, .tools li:nth-child(odd), .tools li:nth-child(even) { padding: 0 var(--space-sm); border-left: 1px solid var(--color-rule); }
      .tools li:first-child { padding-left: 0; border-left: 0; }
      .tools a { align-items: flex-start; flex-direction: column; gap: var(--space-xs); padding: var(--space-md) 0; }
    }
  `],
  template: `
    <app-navbar />
    <main id="main" class="hm">
      <app-email-confirm-banner class="block" style="margin-bottom: var(--space-md)" />
      <div class="head">
        <h1>Dashboard</h1>
        <p class="sub num" data-testid="subtitle">{{ total }}{{ anyMore ? '+' : '' }} {{ total === 1 && !anyMore ? 'piece' : 'pieces' }} of work in the cutting room</p>
      </div>

      @if (loadError) {
        <div class="notice err" role="alert">
          <p><strong>Couldn't load your work.</strong> Check your connection and try again.</p>
          <button type="button" (click)="reload()" class="ui-btn ui-btn--secondary">Retry</button>
        </div>
      }

      @if (!isLoading && !loadError && missing.length > 0) {
        <div class="notice" role="status" data-testid="partial">
          <p><strong>Some work could not be loaded:</strong> {{ missing.join(', ') }}. The rest is shown below.</p>
          <button type="button" (click)="reload()" class="ui-btn ui-btn--secondary">Retry</button>
        </div>
      }

      @if (isLoading && !loadError) {
        <div role="status" aria-label="Loading your work">
          <div class="stats">
            @for (i of [1, 2, 3, 4]; track i) {
              <div class="stat"><div class="ui-skeleton" style="height: 1.5rem; width: 3rem"></div><div class="ui-skeleton" style="height: 1rem; width: 5rem; margin-top: var(--space-xs)"></div></div>
            }
          </div>
          <div style="margin-top: var(--space-xl)">
            @for (i of [1, 2, 3]; track i) {
              <div class="ui-skeleton" style="height: 2.75rem; margin-top: var(--space-xs)"></div>
            }
          </div>
        </div>
      }

      @if (!isLoading) {
        <div class="stats">
          <div class="stat">
            <p class="stat-n num" data-testid="total">{{ total }}{{ anyMore ? '+' : '' }}</p>
            <p class="stat-l">Total</p>
          </div>
          <div class="stat">
            <p class="stat-n num" data-testid="done">{{ doneCount }}</p>
            <p class="stat-l">Ready</p>
          </div>
          <div class="stat">
            <p class="stat-n num" data-testid="inProgress">{{ inProgressCount }}</p>
            <p class="stat-l">Cutting</p>
          </div>
          <div class="stat">
            <p class="stat-n num" data-testid="failed">{{ failedCount }}</p>
            <p class="stat-l">Retakes</p>
          </div>
        </div>

        <section aria-labelledby="tools-heading">
          <h2 id="tools-heading">Your tools</h2>
          <ul class="tools">
            @for (tool of tools; track tool.label) {
              <li>
                <a [routerLink]="tool.link">
                  <svg [lucideIcon]="tool.icon" [size]="18" class="icon" aria-hidden="true"></svg>
                  <span>
                    <span class="t-name">{{ tool.label }}</span>
                    <span class="t-note num">{{ toolNote(tool) }}</span>
                  </span>
                </a>
              </li>
            }
          </ul>
        </section>

        <section aria-labelledby="recent-heading">
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
                  <a [routerLink]="item.link">
                    <svg [lucideIcon]="stateIcon(item.state)" [size]="18" class="icon" [class]="stateClass(item.state)" aria-hidden="true"></svg>
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
                    <span class="state" [class]="stateClass(item.state)">{{ stateLabel[item.state] }}</span>
                    <svg lucideIcon="chevron-right" [size]="16" class="icon" aria-hidden="true"></svg>
                  </a>
                </li>
              }
            </ul>
          }
        </section>
      }
    </main>
  `,
})
export class DashboardComponent implements OnInit {
  private readonly projectService = inject(ProjectService);
  private readonly shortsService = inject(ShortsService);
  private readonly storyPlans = inject(StoryPlanService);
  private readonly storyVideos = inject(StoryVideoService);
  private readonly thumbnails = inject(ThumbnailService);

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
