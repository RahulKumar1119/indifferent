/**
 * Wire types for making a video from a plan. They mirror the Go types in
 * backend/internal/api/storyvideo.go (the view, never the stored record: no
 * storage key and no scene text ever reaches the browser).
 */

export type VideoStatus = 'queued' | 'preparing' | 'narrating' | 'rendering' | 'completed' | 'failed';

export type CaptionPreset = 'minimal' | 'bold' | 'karaoke' | 'clean';

/** Only the preset is chosen on this page; the API also accepts size, colour and position. */
export interface CaptionStyle {
  preset?: CaptionPreset;
  fontSize?: number;
  color?: string;
  highlight?: string;
  position?: 'top' | 'middle' | 'bottom';
}

/** GET/POST /story-videos. */
export interface StoryVideo {
  jobId: string;
  planId: string;
  planRevision: number;
  status: VideoStatus;
  /** 0 to 100. */
  progress: number;
  language: string;
  voice: string;
  caption: CaptionStyle;
  /** Seconds, once the narration has been recorded. */
  totalSeconds?: number;
  /** Only when `status` is "failed". A sentence for the user. */
  error?: string;
  /** "CANCELLED" when the user cancelled it. Only when `status` is "failed". */
  errorCode?: string;
  failedStage?: string;
  createdAt: string;
  updatedAt: string;
  completedAt?: string;
}

export interface StoryVideoList {
  videos: StoryVideo[];
  /** There are more videos than were returned. */
  hasMore: boolean;
}

/** GET /story-videos/limits. Public. */
export interface StoryVideoLimits {
  enabled: boolean;
  /** Videos one user may have in the making at once. */
  maxActive: number;
  /** Videos one user may start in 24 hours. */
  perDay: number;
  maxSeconds: number;
  /** Narration voices per language code, default first. */
  voices: Record<string, string[]>;
}

export interface CreateStoryVideoRequest {
  planId: string;
  /** The revision the user was looking at. A different one is refused. */
  revision: number;
  voice?: string;
  caption?: CaptionStyle;
}

/** GET /story-videos/{id}/download. The link is short-lived. */
export interface StoryVideoDownload {
  url: string;
  filename: string;
  expiresInSeconds: number;
}

/** The code the API gives a video the user cancelled. */
export const CANCELLED_CODE = 'CANCELLED';

/** A video that is still being made. */
export function isActive(status: VideoStatus): boolean {
  return status !== 'completed' && status !== 'failed';
}

export function wasCancelled(v: Pick<StoryVideo, 'status' | 'errorCode'>): boolean {
  return v.status === 'failed' && v.errorCode === CANCELLED_CODE;
}

/**
 * The three things that happen to a video, in order, as the worker does them.
 * `status` is the stage in progress, so "queued" is before the first one.
 */
export const STAGES: readonly { status: 'preparing' | 'narrating' | 'rendering'; label: string; doing: string }[] = [
  { status: 'preparing', label: 'Pictures', doing: 'Getting your pictures ready' },
  { status: 'narrating', label: 'Narration', doing: 'Recording the narration' },
  { status: 'rendering', label: 'Video', doing: 'Putting the video together' },
];

/** Where a status sits in STAGES: -1 before the first, STAGES.length once finished. */
export function stageIndex(status: VideoStatus): number {
  if (status === 'queued') return -1;
  if (status === 'completed') return STAGES.length;
  return Math.max(
    -1,
    STAGES.findIndex((s) => s.status === status),
  );
}

export interface CaptionPresetMeta {
  id: CaptionPreset;
  label: string;
  blurb: string;
}

/**
 * The caption looks the renderer has. The wording is the same the AI Shorts
 * page uses for the same four presets, so a person sees one name for one look.
 */
export const CAPTION_PRESETS: readonly CaptionPresetMeta[] = [
  { id: 'bold', label: 'Bold', blurb: 'Large white text on a dark box' },
  { id: 'minimal', label: 'Minimal', blurb: 'Small and subtle, no box' },
  { id: 'karaoke', label: 'Karaoke highlight', blurb: 'The spoken word lights up as it is said' },
  { id: 'clean', label: 'Clean subtitles', blurb: 'Classic outlined text' },
];

export const DEFAULT_CAPTION_PRESET: CaptionPreset = 'bold';

export function captionLabel(style: CaptionStyle | undefined): string {
  const id = style?.preset ?? DEFAULT_CAPTION_PRESET;
  return CAPTION_PRESETS.find((p) => p.id === id)?.label ?? id;
}

const VOICE_NOTES: Record<string, string> = {
  Joanna: 'US English, female, clear and professional',
  Matthew: 'US English, male, warm and authoritative',
  Amy: 'British English, female, polished',
  Brian: 'British English, male, natural',
  Aditi: 'Indian English, female, approachable',
};

/** A line about a voice. Aditi is the only voice that reads Hindi, and says so. */
export function voiceNote(voice: string, language: string): string {
  if (language === 'hi') return 'The voice for Hindi narration';
  return VOICE_NOTES[voice] ?? '';
}

const LANGUAGE_NAMES: Record<string, string> = { en: 'English', hi: 'Hindi' };

export function languageName(code: string): string {
  return LANGUAGE_NAMES[code] ?? code;
}

/** "42 seconds", "1 minute 18 seconds". Rounds to the nearest second. */
export function formatLength(seconds: number): string {
  const total = Math.max(0, Math.round(seconds));
  const m = Math.floor(total / 60);
  const s = total % 60;
  const secs = s === 1 ? '1 second' : `${s} seconds`;
  if (m === 0) return secs;
  const mins = m === 1 ? '1 minute' : `${m} minutes`;
  return s === 0 ? mins : `${mins} ${secs}`;
}

/**
 * The scene a failure message is about, from the start of the sentence the API
 * writes ("Scene 2: the picture is no longer available..."). Null when the
 * message is about the whole video. This reads our own message format, written
 * in backend/internal/storyjob/errors.go; if that wording changes the page loses
 * the shortcut to the scene and nothing else.
 */
export function sceneInMessage(message: string | undefined): number | null {
  const m = /^Scene (\d{1,2}):/.exec(message ?? '');
  return m ? Number(m[1]) : null;
}

/** Failures that choosing another picture can fix. Running the same job again cannot. */
export const PICTURE_ERROR_CODES: readonly string[] = ['MEDIA_UNAVAILABLE', 'MEDIA_INVALID'];
