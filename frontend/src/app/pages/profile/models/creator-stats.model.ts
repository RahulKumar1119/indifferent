/** Aggregate numbers shown in the creator statistics cards. */
export interface CreatorStats {
  videosCreated: number;
  shortsGenerated: number;
  published: number;
  favorites: number;
  creditsUsed: number;
  creditsQuota: number;
  minutesSaved: number;
}

export function emptyCreatorStats(): CreatorStats {
  return {
    videosCreated: 0,
    shortsGenerated: 0,
    published: 0,
    favorites: 0,
    creditsUsed: 0,
    creditsQuota: 500,
    minutesSaved: 0,
  };
}

/** A unified content card: either a quiz video (project) or an AI short. */
export type CreatorVideoKind = 'video' | 'short';
export type CreatorVideoStatus =
  | 'generated'
  | 'published'
  | 'draft'
  | 'processing'
  | 'failed';

export interface CreatorVideo {
  id: string;
  title: string;
  kind: CreatorVideoKind;
  status: CreatorVideoStatus;
  /** Raw backend status (project/shorts stage) for tooltips. */
  rawStatus: string;
  createdAt: string;
  /** Estimated playback length / source duration label. */
  duration: string;
  aspectRatio: '16:9' | '9:16';
  thumbnailUrl?: string;
  clipCount?: number;
  fileType?: string;
  favorite: boolean;
  /** Backend id used for actions (project id or shorts job id). */
  sourceId: string;
}

export type ContentTab = 'all' | 'generated' | 'published' | 'drafts' | 'favorites';

export const CONTENT_TABS: { id: ContentTab; label: string }[] = [
  { id: 'all', label: 'All' },
  { id: 'generated', label: 'Generated' },
  { id: 'published', label: 'Published' },
  { id: 'drafts', label: 'Drafts' },
  { id: 'favorites', label: 'Favorites' },
];

/** A single row in the recent-activity feed. */
export interface ActivityItem {
  id: string;
  icon: string;
  text: string;
  detail?: string;
  timestamp: string;
  group: 'Today' | 'Yesterday' | 'Earlier';
}

/** One line in the credit-usage breakdown. */
export interface CreditUsage {
  label: string;
  cost: number;
  when: string;
}
