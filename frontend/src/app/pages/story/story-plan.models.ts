/**
 * Wire types for the Story2Shorts plan editor. They mirror the Go structs in
 * backend/internal/storyplan and backend/internal/api (storyplan.go,
 * stockmedia.go). The page talks only to our API: no provider key and no
 * provider URL other than the preview links the API hands back.
 */

export type StockProvider = 'pexels' | 'pixabay';
export type MediaType = 'photo' | 'video';
export type Orientation = 'portrait' | 'landscape' | 'square' | 'any';

/** A picked asset, named without any URL (providers forbid stored links). */
export interface MediaRef {
  provider: StockProvider;
  type: MediaType;
  id: string;
}

export interface PlanScene {
  /** Spoken text, in the story's own language and script. */
  narration: string;
  /** One English sentence saying what to show; used as alt text. */
  description: string;
  /** English stock-search terms, most concrete first. */
  keywords: string[];
  mediaType: MediaType;
  /** Seconds, estimated from the narration. */
  duration: number;
  media?: MediaRef;
}

export interface StoryPlan {
  /** ISO 639-1 code of the story's language, e.g. "hi", "en". */
  language: string;
  hook: string;
  scenes: PlanScene[];
  totalSeconds: number;
  warnings?: string[];
}

/** An asset as the browser sees it: enough to show and to credit. */
export interface StockAsset {
  provider: StockProvider;
  type: MediaType;
  id: string;
  previewUrl?: string;
  /** A light clip for videos; absent for photos. */
  playUrl?: string;
  pageUrl: string;
  width: number;
  height: number;
  /** Seconds, videos only. */
  duration?: number;
  orientation: string;
  alt?: string;
  author: string;
  authorUrl?: string;
  /** Ready-made credit line, e.g. "Photo by Jo Doe on Pexels". */
  credit: string;
}

export interface StockAttribution {
  provider: StockProvider;
  text: string;
  url: string;
}

/**
 * A looked-up reference. Exactly one of `asset` and `status` is set:
 * `unavailable` means the provider no longer has it (do not retry), `error`
 * means a temporary failure (worth retrying).
 */
export interface StockAssetItem {
  provider: StockProvider;
  type: MediaType;
  id: string;
  asset?: StockAsset;
  status?: 'unavailable' | 'error';
}

export type StoryPlanStatus = 'pending' | 'ready' | 'failed';

/** GET/POST/PUT /story-plans. */
export interface StoryPlanRecord {
  planId: string;
  status: StoryPlanStatus;
  model?: string;
  /** Starts at 1; PUT must send the revision it was based on. */
  revision: number;
  plan?: StoryPlan;
  errorCode?: string;
  createdAt: string;
  updatedAt: string;
  /** Index i is scene i; null for a scene with no pick. */
  media?: (StockAssetItem | null)[];
  mediaAttributions?: StockAttribution[];
}

/** One entry per scene, in order. A missing field leaves that part as it is. */
export interface SceneEdit {
  narration?: string;
  media?: MediaRef;
  clearMedia?: boolean;
}

export interface StockProviderFailure {
  provider: StockProvider;
  reason: 'rate_limited' | 'unavailable' | 'auth' | 'unsupported_language' | 'other';
  /** Seconds to wait when `reason` is rate_limited. */
  retryAfter?: number;
}

export interface StockSearchResponse {
  assets: StockAsset[];
  attributions: StockAttribution[];
  failures?: StockProviderFailure[];
  page: number;
  perPage: number;
  hasMore: boolean;
}

export interface StockSearchParams {
  q: string;
  type: MediaType;
  provider?: StockProvider;
  orientation?: Orientation;
  page?: number;
  perPage?: number;
}

export interface StockAssetsResponse {
  items: StockAssetItem[];
  attributions: StockAttribution[];
}

/** GET /story-plans/limits. The page validates against these numbers. */
export interface StoryPlanLimits {
  enabled: boolean;
  minStoryChars: number;
  maxStoryChars: number;
  minScenes: number;
  maxScenes: number;
  maxTotalSeconds: number;
  perHour: number;
}

/**
 * Used until GET /story-plans/limits answers (or if it fails). `enabled` is
 * true so the form is usable at once; the API's answer replaces all of it.
 */
export const DEFAULT_STORY_PLAN_LIMITS: StoryPlanLimits = {
  enabled: true,
  minStoryChars: 30,
  maxStoryChars: 5000,
  minScenes: 2,
  maxScenes: 10,
  maxTotalSeconds: 90,
  perHour: 20,
};

/**
 * Longest narration the editor allows in one scene (storyplan.MaxNarrationRunes).
 * The API is the authority and refuses more; this only stops typing early.
 */
export const MAX_NARRATION_CHARS = 600;

/** Stable key for a media reference, for comparing and for track-by. */
export function mediaKey(ref: Pick<MediaRef, 'provider' | 'type' | 'id'>): string {
  return `${ref.provider}:${ref.type}:${ref.id}`;
}

export function toMediaRef(a: Pick<StockAsset, 'provider' | 'type' | 'id'>): MediaRef {
  return { provider: a.provider, type: a.type, id: a.id };
}

/** Longest search the API accepts (stockmedia.MaxQueryRunes). */
export const MAX_QUERY_CHARS = 100;

/**
 * Characters as a person counts them, not UTF-16 units: the API limits are in
 * characters, and one Devanagari letter or emoji must not count twice.
 */
export function charCount(text: string): number {
  return Array.from(text).length;
}

/**
 * The search a scene starts with: its three strongest keywords, cut at
 * MAX_QUERY_CHARS. Mirrors storyplan.Scene.SearchQuery, because the providers
 * match every word and a short query of the best terms finds more.
 */
export function sceneSearchQuery(scene: Pick<PlanScene, 'keywords'>): string {
  let out = '';
  for (const keyword of scene.keywords.slice(0, 3)) {
    const next = out ? ` ${keyword}` : keyword;
    if (charCount(out) + charCount(next) > MAX_QUERY_CHARS) break;
    out += next;
  }
  return out;
}

export const PROVIDER_NAMES: Record<StockProvider, string> = { pexels: 'Pexels', pixabay: 'Pixabay' };
