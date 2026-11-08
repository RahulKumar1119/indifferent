import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import {
  provideLucideIcons,
  LucideChevronRight,
  LucideCircleCheck,
  LucideCircleX,
  LucideClapperboard,
  LucideClock,
  LucideFilm,
  LucideFolderOpen,
  LucideImage,
  LucidePencil,
  LucidePlus,
  LucideScissors,
  LucideStamp,
  LucideUser,
  LucideSun,
  LucideLayoutDashboard,
  LucideArrowUpRight,
  LucideMoon,
} from '@lucide/angular';
import { Observable, of, throwError } from 'rxjs';
import { DashboardComponent } from './dashboard.component';
import { ProjectService } from '../../core/services/project.service';
import { ShortsService } from '../shorts/shorts.service';
import { StoryPlanService } from '../story/story-plan.service';
import { StoryVideoService } from '../story/story-video.service';
import { ThumbnailService } from '../thumbnails/thumbnail.service';

type Source<T> = Observable<T>;
interface Data {
  projects: Source<unknown>;
  shorts: Source<unknown>;
  videos: Source<unknown>;
  plans: Source<unknown>;
  thumbnails: Source<unknown>;
}

const boom = () => throwError(() => new Error('down'));

const full = (): Data => ({
  projects: of([{ id: 'p1', name: 'Capitals quiz', status: 'completed', createdAt: '2026-01-01T00:00:00Z' }]),
  shorts: of([{ jobId: 's1', status: 'rendering', fileType: 'mp4', clipCount: 0, createdAt: '2026-01-02T00:00:00Z', updatedAt: '2026-01-02T00:00:00Z' }]),
  videos: of({ videos: [{ jobId: 'v1', planId: 'pl1', status: 'failed', createdAt: '2026-01-03T00:00:00Z', updatedAt: '2026-01-03T00:00:00Z' }], hasMore: false }),
  plans: of({
    plans: [
      { planId: 'pl1', status: 'ready', hook: 'Hidden by its video', sceneCount: 3, createdAt: '2026-01-03T00:00:00Z', updatedAt: '2026-01-03T00:00:00Z' },
      { planId: 'pl2', status: 'ready', hook: 'An open draft', sceneCount: 4, createdAt: '2026-01-04T00:00:00Z', updatedAt: '2026-01-04T00:00:00Z' },
    ],
    hasMore: false,
  }),
  thumbnails: of({ thumbnails: [{ jobId: 't1', status: 'completed', thumbnailCount: 3, selected: false, createdAt: '2026-01-05T00:00:00Z', updatedAt: '2026-01-05T00:00:00Z' }], hasMore: false }),
});

function setup(data: Data) {
  TestBed.configureTestingModule({
    imports: [DashboardComponent],
    providers: [
      provideHttpClient(),
      provideHttpClientTesting(),
      provideRouter([]),
      provideLucideIcons(
        LucideChevronRight, LucideCircleCheck, LucideCircleX, LucideClapperboard, LucideClock, LucideFilm,
        LucideFolderOpen, LucideImage, LucidePencil, LucidePlus, LucideScissors, LucideStamp, LucideUser, LucideSun, LucideMoon, LucideLayoutDashboard, LucideArrowUpRight,
      ),
      { provide: ProjectService, useValue: { getProjects: () => data.projects } },
      { provide: ShortsService, useValue: { listJobs: () => data.shorts } },
      { provide: StoryVideoService, useValue: { list: () => data.videos } },
      { provide: StoryPlanService, useValue: { list: () => data.plans } },
      { provide: ThumbnailService, useValue: { list: () => data.thumbnails } },
    ],
  });
  const fixture = TestBed.createComponent(DashboardComponent);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const text = (id: string) => el.querySelector(`[data-testid="${id}"]`)?.textContent?.trim();
  const rows = () => Array.from(el.querySelectorAll('section ul.divide-y li a')).map((a) => a.textContent ?? '');
  return { fixture, el, text, rows };
}

describe('DashboardComponent', () => {
  it('lists work from every feature in one list, newest first', () => {
    const { rows } = setup(full());
    const r = rows();
    expect(r.length).toBe(5);
    expect(r[0]).toContain('AI thumbnails');
    expect(r[1]).toContain('An open draft');
    expect(r.join('|')).toContain('Capitals quiz');
    expect(r.join('|')).toContain('AI Shorts');
  });

  it('leaves out a plan that already has a video, so it is not counted twice', () => {
    const { rows } = setup(full());
    // The video carries the plan's hook as its title; the plan itself has no row.
    const mine = rows().filter((r) => r.includes('Hidden by its video'));
    expect(mine.length).toBe(1);
    expect(mine[0]).toContain('Story video');
    expect(mine[0]).not.toContain('Story plan');
  });

  it('counts all work in the summary cards', () => {
    const { text } = setup(full());
    expect(text('total')).toBe('5');
    expect(text('done')).toBe('2'); // quiz + thumbnails
    expect(text('inProgress')).toBe('2'); // short rendering + open draft
    expect(text('failed')).toBe('1'); // the story video
    expect(text('subtitle')).toBe('5 pieces of work in the cutting room');
  });

  it('shows a tool card for every feature, including the watermark tool', () => {
    const { el } = setup(full());
    const tools = el.querySelector('#tools-heading')!.parentElement!.textContent!;
    for (const name of ['Quiz videos', 'AI Shorts', 'Story video', 'AI thumbnails', 'Watermark tool']) {
      expect(tools).toContain(name);
    }
    const hrefs = Array.from(el.querySelectorAll('#tools-heading ~ ul a')).map((a) => a.getAttribute('href'));
    expect(hrefs).toEqual(['/projects/new', '/shorts', '/story', '/shorts/history', '/tools/add-watermark']);
  });

  it('shows a plus sign when a source has more than it returned', () => {
    const d = full();
    d.videos = of({ videos: [], hasMore: true });
    const { text } = setup(d);
    expect(text('total')).toContain('+');
  });

  it('keeps showing what loaded when one source fails, and says which failed', () => {
    const d = full();
    d.thumbnails = boom();
    const { text, rows, el } = setup(d);
    expect(rows().length).toBe(4);
    expect(text('partial')).toContain('Thumbnails');
    expect(el.textContent).not.toContain("Couldn't load your work");
  });

  it('shows the error banner only when every source fails', () => {
    const { el, text } = setup({ projects: boom(), shorts: boom(), videos: boom(), plans: boom(), thumbnails: boom() });
    expect(el.textContent).toContain("Couldn't load your work");
    expect(text('partial')).toBeUndefined();
  });

  it('shows an empty state when there is no work at all', () => {
    const empty = of([]);
    const { el } = setup({
      projects: empty,
      shorts: empty,
      videos: of({ videos: [], hasMore: false }),
      plans: of({ plans: [], hasMore: false }),
      thumbnails: of({ thumbnails: [], hasMore: false }),
    });
    expect(el.textContent).toContain('Nothing here yet.');
  });

  it('caps the list at ten rows but counts everything', () => {
    const d = full();
    d.projects = of(
      Array.from({ length: 12 }, (_, i) => ({ id: `q${i}`, name: `Quiz ${i}`, status: 'completed', createdAt: `2026-02-${String(i + 1).padStart(2, '0')}T00:00:00Z` })),
    );
    const { rows, text } = setup(d);
    expect(rows().length).toBe(10);
    expect(text('total')).toBe('16');
  });
  it('uses the shared app navbar and exactly one h1', () => {
    const { el } = setup(full());
    expect(el.querySelector('app-navbar')).toBeTruthy();
    expect(el.querySelectorAll('h1').length).toBe(1);
    expect(el.querySelector('header')?.closest('main')).toBeNull();
  });
  it('shows the state word on every row', () => {
    const { rows } = setup(full());
    const r = rows().join('|');
    expect(r).toContain('Completed');
    expect(r).toContain('In progress');
    expect(r).toContain('Failed');
  });
  it('keeps hex colours and tracked capitals out of the markup', () => {
    const { el } = setup(full());
    expect(el.innerHTML).not.toMatch(/#[0-9A-Fa-f]{6}\b/);
    expect(el.innerHTML).not.toContain('uppercase');
    expect(el.textContent).not.toContain('\u00b7');
  });
  it('greets the signed-in user by first name, and still greets without one', () => {
    const { fixture, el } = setup(full());
    const http = TestBed.inject(HttpTestingController);
    expect(el.querySelector('h1')!.textContent).toMatch(/^Good (morning|afternoon|evening)$/);
    http.match((r) => r.url.endsWith('/auth/me')).forEach((r) => r.flush({ email: 'a@example.com', name: 'Asha Rao', avatarUrl: '', createdAt: '' }));
    fixture.detectChanges();
    expect(el.querySelector('h1')!.textContent).toMatch(/^Good (morning|afternoon|evening), Asha$/);
  });
  it('puts Total on the terracotta card and keeps the status labels', () => {
    const { el } = setup(full());
    const cards = Array.from(el.querySelectorAll('.stat'));
    expect(cards.map((c) => c.querySelector('.stat-top span')!.textContent!.trim())).toEqual(['Total', 'Ready', 'Cutting', 'Retakes']);
    expect(cards[0].classList).toContain('total');
    expect(cards.slice(1).some((c) => c.classList.contains('total'))).toBeFalse();
  });
  it('charts real recent work and shows the status split', () => {
    const d = full();
    const now = new Date().toISOString();
    d.projects = of([
      { id: 'a', name: 'Today one', status: 'completed', createdAt: now },
      { id: 'b', name: 'Today two', status: 'failed', createdAt: now },
    ]);
    const { el } = setup(d);
    expect(el.querySelector('[data-testid="activity-empty"]')).toBeNull();
    const cells = Array.from(el.querySelectorAll('app-activity-chart table td')).map((c) => c.textContent!.trim());
    expect(cells[cells.length - 1]).toBe('2');
    expect(el.querySelector('[data-testid="ring-Ready"]')!.textContent).toBe('2');
    expect(el.querySelector('[data-testid="ring-Cutting"]')!.textContent).toBe('2');
    expect(el.querySelector('[data-testid="ring-Retakes"]')!.textContent).toBe('2');
  });
  it('says so when nothing was made in the last 14 days', () => {
    const { el } = setup(full());
    expect(el.querySelector('[data-testid="activity-empty"]')).toBeTruthy();
  });
  it('leaves the charts out when there is no work at all', () => {
    const empty = of([]);
    const { el } = setup({
      projects: empty,
      shorts: empty,
      videos: of({ videos: [], hasMore: false }),
      plans: of({ plans: [], hasMore: false }),
      thumbnails: of({ thumbnails: [], hasMore: false }),
    });
    expect(el.querySelector('app-activity-chart')).toBeNull();
    expect(el.querySelector('app-status-ring')).toBeNull();
  });
  it('shows the rendering panel only for jobs that are being made, linked to their progress pages', () => {
    const d = full();
    d.projects = of([{ id: 'p9', name: 'Rendering quiz', status: 'parsing', createdAt: '2026-01-06T00:00:00Z' }]);
    const { el } = setup(d);
    const links = Array.from(el.querySelectorAll('[data-testid="rendering"] a')).map((a) => a.getAttribute('href'));
    expect(links).toEqual(['/projects/p9/progress', '/shorts/s1/progress']);
    const none = full();
    none.shorts = of([]);
    TestBed.resetTestingModule();
    expect(setup(none).el.querySelector('[data-testid="rendering"]')).toBeNull();
  });
  it('has desktop sidebar links to existing pages', () => {
    const { el } = setup(full());
    const hrefs = Array.from(el.querySelectorAll('app-dashboard-sidebar nav a')).map((a) => a.getAttribute('href'));
    expect(hrefs).toEqual(['/dashboard', '/projects', '/shorts', '/story', '/shorts/history', '/tools/add-watermark', '/profile']);
  });
});
