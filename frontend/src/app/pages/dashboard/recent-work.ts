import { Project } from '../../shared/models/project.model';
import { ShortsJobSummary } from '../shorts/shorts.service';
import { StoryPlanList } from '../story/story-plan.models';
import { StoryVideoList } from '../story/story-video.models';
import { ThumbnailList } from '../thumbnails/thumbnail.service';

/** The kinds of work the dashboard lists. */
export type WorkKind = 'quiz' | 'short' | 'story-video' | 'story-plan' | 'thumbnails';

/**
 * Where a piece of work stands. `draft` is made but not finished off by the
 * user: a story plan with no video yet, or a quiz project that was never started.
 */
export type WorkState = 'done' | 'working' | 'failed' | 'draft';

export interface WorkItem {
  /** Unique across kinds, for list tracking. */
  id: string;
  kind: WorkKind;
  title: string;
  state: WorkState;
  /** A short second line such as "5 clips". May be empty. */
  detail: string;
  /** Router link commands. */
  link: string[];
  /** ISO time of the latest activity. */
  date: string;
}

/** What each source returned; `null` means that source failed to load. */
export interface WorkSources {
  projects: Project[] | null;
  shorts: ShortsJobSummary[] | null;
  videos: StoryVideoList | null;
  plans: StoryPlanList | null;
  thumbnails: ThumbnailList | null;
}

/** How many of one kind there are. `more` means the server left some out. */
export interface KindCount {
  n: number;
  more: boolean;
}

export interface RecentWork {
  /** Everything loaded, newest activity first. */
  items: WorkItem[];
  /** `null` for a kind whose source failed to load. */
  counts: Record<WorkKind, KindCount | null>;
}

export const WORK_KINDS: WorkKind[] = ['quiz', 'short', 'story-video', 'story-plan', 'thumbnails'];

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;

function quizState(status: Project['status']): WorkState {
  if (status === 'completed') return 'done';
  if (status === 'failed') return 'failed';
  return status === 'created' ? 'draft' : 'working';
}

export function fromProject(p: Project): WorkItem {
  return {
    id: `quiz:${p.id}`,
    kind: 'quiz',
    title: p.name || 'Untitled quiz video',
    state: quizState(p.status),
    detail: '',
    link: ['/projects', p.id],
    date: p.completedAt || p.createdAt,
  };
}

export function fromShort(j: ShortsJobSummary): WorkItem {
  // "uploaded" means waiting for the user to start it, or (sourceMissing) a
  // file that never arrived. Neither is rendering, so neither is "working" (#275).
  const waiting = j.status === 'uploaded';
  const state: WorkState =
    j.status === 'completed' ? 'done' : j.status === 'failed' ? 'failed' : waiting ? 'draft' : 'working';
  const detail =
    state === 'done' ? plural(j.clipCount, 'clip', 'clips') : waiting ? (j.sourceMissing ? "Upload didn't finish" : 'Not started') : '';
  return {
    id: `short:${j.jobId}`,
    kind: 'short',
    title: j.fileType === 'mp3' ? 'Shorts from audio' : 'Shorts from video',
    state,
    detail,
    // Start, upload again and delete live on the history page; it has no progress to show yet.
    link: waiting ? ['/shorts', 'history'] : ['/shorts', j.jobId, state === 'done' && j.clipCount > 0 ? 'clips' : 'progress'],
    date: j.updatedAt || j.createdAt,
  };
}

export function fromThumbnails(t: ThumbnailList['thumbnails'][number]): WorkItem {
  const state: WorkState =
    t.status === 'completed' ? 'done' : t.status === 'failed' ? 'failed' : t.status === 'none' ? 'draft' : 'working';
  return {
    id: `thumbnails:${t.jobId}`,
    kind: 'thumbnails',
    title: 'AI thumbnails',
    state,
    detail: state === 'done' ? plural(t.thumbnailCount, 'option', 'options') : '',
    link: ['/shorts', t.jobId, 'thumbnails'],
    date: t.updatedAt || t.createdAt,
  };
}

/**
 * Merges every source into one list. A story plan that already has a video is
 * left out, since the video stands for it. Dates that do not parse sort last.
 */
export function buildRecentWork(src: WorkSources): RecentWork {
  const items: WorkItem[] = [];
  const counts = Object.fromEntries(WORK_KINDS.map((k) => [k, null])) as RecentWork['counts'];

  if (src.projects) {
    items.push(...src.projects.map(fromProject));
    counts['quiz'] = { n: src.projects.length, more: false };
  }
  if (src.shorts) {
    items.push(...src.shorts.map(fromShort));
    counts['short'] = { n: src.shorts.length, more: false };
  }
  if (src.thumbnails) {
    items.push(...src.thumbnails.thumbnails.map(fromThumbnails));
    counts['thumbnails'] = { n: src.thumbnails.thumbnails.length, more: src.thumbnails.hasMore };
  }

  const hooks = new Map<string, string>();
  for (const p of src.plans?.plans ?? []) if (p.hook) hooks.set(p.planId, p.hook);

  if (src.videos) {
    for (const v of src.videos.videos) {
      items.push({
        id: `story-video:${v.jobId}`,
        kind: 'story-video',
        title: hooks.get(v.planId) || 'Story video',
        state: v.status === 'completed' ? 'done' : v.status === 'failed' ? 'failed' : 'working',
        detail: v.status === 'completed' && v.totalSeconds ? `${Math.round(v.totalSeconds)} seconds` : '',
        link: ['/story', v.planId],
        date: v.updatedAt || v.createdAt,
      });
    }
    counts['story-video'] = { n: src.videos.videos.length, more: src.videos.hasMore };
  }

  if (src.plans) {
    const made = new Set((src.videos?.videos ?? []).map((v) => v.planId));
    const open = src.plans.plans.filter((p) => !made.has(p.planId));
    for (const p of open) {
      items.push({
        id: `story-plan:${p.planId}`,
        kind: 'story-plan',
        title: p.hook || 'Story plan',
        state: p.status === 'ready' ? 'draft' : p.status === 'failed' ? 'failed' : 'working',
        detail: p.status === 'ready' ? plural(p.sceneCount, 'scene', 'scenes') : '',
        link: ['/story', p.planId],
        date: p.updatedAt || p.createdAt,
      });
    }
    counts['story-plan'] = { n: open.length, more: src.plans.hasMore };
  }

  const time = (d: string) => {
    const t = Date.parse(d);
    return Number.isNaN(t) ? 0 : t;
  };
  items.sort((a, b) => time(b.date) - time(a.date));
  return { items, counts };
}
