import { Project } from '../../shared/models/project.model';
import { buildRecentWork, WorkSources } from './recent-work';

const none: WorkSources = { projects: null, shorts: null, videos: null, plans: null, thumbnails: null };

const project = (id: string, status: string, createdAt: string, completedAt?: string) =>
  ({ id, name: `Quiz ${id}`, status, createdAt, completedAt }) as Project;

const plan = (planId: string, status: 'pending' | 'ready' | 'failed', updatedAt: string, hook = 'A hook') => ({
  planId,
  status,
  hook,
  sceneCount: 4,
  createdAt: updatedAt,
  updatedAt,
});

const video = (jobId: string, planId: string, status: string, updatedAt: string, totalSeconds?: number) =>
  ({ jobId, planId, status, updatedAt, createdAt: updatedAt, totalSeconds }) as never;

describe('buildRecentWork', () => {
  it('is empty, with every kind marked as not loaded, when nothing loaded', () => {
    const r = buildRecentWork(none);
    expect(r.items).toEqual([]);
    expect(Object.values(r.counts).every((c) => c === null)).toBeTrue();
  });

  it('merges all five kinds and sorts by latest activity', () => {
    const r = buildRecentWork({
      projects: [project('p1', 'completed', '2026-01-01T00:00:00Z', '2026-01-05T00:00:00Z')],
      shorts: [
        { jobId: 's1', status: 'completed', fileType: 'mp4', clipCount: 3, createdAt: '2026-01-02T00:00:00Z', updatedAt: '2026-01-02T00:00:00Z' } as never,
      ],
      videos: { videos: [video('v1', 'plan-v', 'completed', '2026-01-04T00:00:00Z', 31.6)], hasMore: false },
      plans: { plans: [plan('plan-d', 'ready', '2026-01-03T00:00:00Z')], hasMore: false },
      thumbnails: {
        thumbnails: [
          { jobId: 't1', status: 'completed', thumbnailCount: 3, selected: false, createdAt: '2026-01-06T00:00:00Z', updatedAt: '2026-01-06T00:00:00Z' },
        ],
        hasMore: false,
      },
    });
    expect(r.items.map((i) => i.id)).toEqual(['thumbnails:t1', 'quiz:p1', 'story-video:v1', 'story-plan:plan-d', 'short:s1']);
    expect(r.items.find((i) => i.kind === 'short')!.detail).toBe('3 clips');
    expect(r.items.find((i) => i.kind === 'story-video')!.detail).toBe('32 seconds');
  });

  it('links each kind to where it can be opened', () => {
    const r = buildRecentWork({
      ...none,
      shorts: [
        { jobId: 'a', status: 'completed', fileType: 'mp4', clipCount: 2, createdAt: '', updatedAt: '' } as never,
        { jobId: 'b', status: 'rendering', fileType: 'mp4', clipCount: 0, createdAt: '', updatedAt: '' } as never,
      ],
      videos: { videos: [video('v', 'pl', 'queued', '2026-01-01T00:00:00Z')], hasMore: false },
    });
    const link = (id: string) => r.items.find((i) => i.id === id)!.link;
    expect(link('short:a')).toEqual(['/shorts', 'a', 'clips']);
    expect(link('short:b')).toEqual(['/shorts', 'b', 'progress']);
    expect(link('story-video:v')).toEqual(['/story', 'pl']);
  });

  it('hides a story plan that has a video and shows the video under the plan hook', () => {
    const r = buildRecentWork({
      ...none,
      plans: { plans: [plan('pl', 'ready', '2026-01-01T00:00:00Z', 'The net')], hasMore: false },
      videos: { videos: [video('v', 'pl', 'completed', '2026-01-02T00:00:00Z')], hasMore: false },
    });
    expect(r.items.map((i) => i.id)).toEqual(['story-video:v']);
    expect(r.items[0].title).toBe('The net');
    expect(r.counts['story-plan']).toEqual({ n: 0, more: false });
  });

  it('shows drafts, pending and failed plans with matching states', () => {
    const r = buildRecentWork({
      ...none,
      plans: {
        plans: [plan('a', 'ready', '2026-01-03T00:00:00Z'), plan('b', 'pending', '2026-01-02T00:00:00Z'), plan('c', 'failed', '2026-01-01T00:00:00Z')],
        hasMore: false,
      },
    });
    expect(r.items.map((i) => i.state)).toEqual(['draft', 'working', 'failed']);
  });

  it('maps quiz states, treating a never-started project as a draft', () => {
    const r = buildRecentWork({
      ...none,
      projects: [
        project('a', 'created', '2026-01-04T00:00:00Z'),
        project('b', 'rendering', '2026-01-03T00:00:00Z'),
        project('c', 'failed', '2026-01-02T00:00:00Z'),
        project('d', 'completed', '2026-01-01T00:00:00Z'),
      ],
    });
    expect(r.items.map((i) => i.state)).toEqual(['draft', 'working', 'failed', 'done']);
  });

  it('keeps the other kinds when one source failed, and reports it as null', () => {
    const r = buildRecentWork({ ...none, projects: [project('a', 'completed', '2026-01-01T00:00:00Z')] });
    expect(r.items.length).toBe(1);
    expect(r.counts['quiz']).toEqual({ n: 1, more: false });
    expect(r.counts['short']).toBeNull();
  });

  it('passes hasMore through for capped sources', () => {
    const r = buildRecentWork({ ...none, videos: { videos: [], hasMore: true } });
    expect(r.counts['story-video']).toEqual({ n: 0, more: true });
  });

  it('puts items with an unreadable date last instead of throwing', () => {
    const r = buildRecentWork({
      ...none,
      projects: [project('x', 'completed', 'garbage'), project('y', 'completed', '2026-01-01T00:00:00Z')],
    });
    expect(r.items.map((i) => i.id)).toEqual(['quiz:y', 'quiz:x']);
  });
  it('treats a short waiting to start as a draft, and says when its file never arrived (#275)', () => {
    const short = (jobId: string, status: string, sourceMissing?: boolean) =>
      ({ jobId, status, fileType: 'mp4', clipCount: 0, sourceDuration: 36, createdAt: '2026-11-08T00:00:00Z', updatedAt: '2026-11-08T00:00:00Z', sourceMissing }) as never;
    const r = buildRecentWork({ ...none, shorts: [short('lost', 'uploaded', true), short('ready', 'uploaded'), short('run', 'rendering')] });
    const by = Object.fromEntries(r.items.map((i) => [i.id, i]));
    expect(by['short:lost'].state).toBe('draft');
    expect(by['short:lost'].detail).toBe("Upload didn't finish");
    expect(by['short:lost'].link).toEqual(['/shorts', 'history']);
    expect(by['short:ready'].state).toBe('draft');
    expect(by['short:ready'].detail).toBe('Not started');
    // Only a short that is really being made counts as working (the dashboard's "Rendering now").
    expect(by['short:run'].state).toBe('working');
    expect(by['short:run'].link).toEqual(['/shorts', 'run', 'progress']);
  });
});
