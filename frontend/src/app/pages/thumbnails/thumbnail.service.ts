import { Injectable } from '@angular/core';
import { HttpContext } from '@angular/common/http';
import { Observable } from 'rxjs';
import { ApiService } from '../../core';
import { SUPPRESS_GLOBAL_ERROR } from '../../core/interceptors/error.interceptor';

/** The seven style presets (mirrors backend thumbnail.Styles). */
export type ThumbnailStyle =
  | 'viral'
  | 'technology'
  | 'educational'
  | 'news'
  | 'podcast'
  | 'cinematic'
  | 'minimal';

/** 16:9 is the YouTube video shape, 9:16 the Shorts shape. */
export type ThumbnailFormat = '16:9' | '9:16';

export type ThumbnailStatus = 'none' | 'queued' | 'running' | 'completed' | 'failed';

/** Steps a running generation moves through, in order. */
export type ThumbnailStage = 'extracting' | 'selecting' | 'analyzing' | 'concepts' | 'rendering';

export type ThumbnailConcept = 'face' | 'subject' | 'creative';

/** One generated thumbnail (matches backend thumbnailJSON). */
export interface Thumbnail {
  id: string;
  concept: ThumbnailConcept;
  style: ThumbnailStyle;
  format: ThumbnailFormat;
  headline: string;
  secondaryText?: string;
  frameTime: number;
  width: number;
  height: number;
  /** Time-limited link to the image. */
  url?: string;
  selected: boolean;
}

export interface ThumbnailAnalysis {
  topic: string;
  mainSubject: string;
  emotion: string;
  headline: string;
  secondaryText?: string;
  keywords?: string[];
}

export interface ThumbnailFunnel {
  extracted: number;
  usable: number;
  unique: number;
  candidates: number;
}

/** GET /shorts/{id}/thumbnails (matches backend thumbnailsResponse). */
export interface ThumbnailSet {
  status: ThumbnailStatus;
  stage?: ThumbnailStage;
  /** False for audio uploads, which have no picture to use. */
  supported: boolean;
  jobStatus: string;
  errorCode?: string;
  error?: string;
  style?: ThumbnailStyle;
  format?: ThumbnailFormat;
  generation?: number;
  thumbnails: Thumbnail[];
  /** The thumbnail saved against the video, if one was chosen. */
  thumbnail: { id: string; url: string; type: 'ai_generated'; style: ThumbnailStyle; selected: boolean } | null;
  analysis?: ThumbnailAnalysis;
  funnel?: ThumbnailFunnel;
  transcriptUsed: boolean;
  detectionUsed: boolean;
  updatedAt?: string;
}

export interface StartOptions {
  /** Empty lets the AI suggest a style. */
  style?: ThumbnailStyle | '';
  /** Empty follows the video's shape. */
  format?: ThumbnailFormat | '';
}

export interface StyleOption {
  value: ThumbnailStyle;
  label: string;
  blurb: string;
}

export const THUMBNAIL_STYLES: readonly StyleOption[] = [
  { value: 'viral', label: 'Viral', blurb: 'Big yellow type, punchy color' },
  { value: 'technology', label: 'Technology', blurb: 'Cool cyan, monospace type' },
  { value: 'educational', label: 'Educational', blurb: 'Clear white type, amber tag' },
  { value: 'news', label: 'News', blurb: 'Red tag, serious tone' },
  { value: 'podcast', label: 'Podcast', blurb: 'Warm pink and purple' },
  { value: 'cinematic', label: 'Cinematic', blurb: 'Letterbox bars, muted grade' },
  { value: 'minimal', label: 'Minimal', blurb: 'Clean white type, no outline' },
];

export const CONCEPT_LABELS: Record<ThumbnailConcept, string> = {
  face: 'Face focused',
  subject: 'Subject focused',
  creative: 'AI creative',
};

export const THUMBNAIL_STAGES: readonly { key: ThumbnailStage; label: string }[] = [
  { key: 'extracting', label: 'Extracting frames' },
  { key: 'selecting', label: 'Finding best moments' },
  { key: 'analyzing', label: 'Analyzing transcript and frames' },
  { key: 'concepts', label: 'Creating thumbnail concepts' },
  { key: 'rendering', label: 'Rendering thumbnails' },
];

/** What to tell people for each machine-readable failure class. */
export const THUMBNAIL_ERROR_HELP: Record<string, string> = {
  NO_VIDEO_TRACK: 'This upload has no picture, so there is nothing to make a thumbnail from.',
  SOURCE_UNREADABLE: "We couldn't read this video file. Uploading it again as an MP4 usually fixes this.",
  NO_SUITABLE_FRAMES: 'Every frame was too dark, blurry or empty. Footage with a clear, well-lit scene works best.',
  AI_TIMEOUT: 'The AI took too long to respond. This is usually temporary, so try again.',
  AI_FAILED: "The AI couldn't analyze this video this time. Trying again often works.",
  RENDER_FAILED: "We couldn't draw the thumbnails this time. Trying again often works.",
  TASK_FAILED: 'Thumbnail generation stopped unexpectedly. Trying again usually works.',
  PIPELINE_ERROR: "We couldn't start thumbnail generation. Please try again.",
};

/** Failures that retrying cannot fix. */
export const NOT_RETRYABLE = new Set(['NO_VIDEO_TRACK']);

/** Client for the thumbnail routes. */
@Injectable({ providedIn: 'root' })
export class ThumbnailService {
  constructor(private readonly api: ApiService) {}

  /** GET /shorts/{id}/thumbnails. Polled, so the global error toast stays quiet. */
  get(jobId: string): Observable<ThumbnailSet> {
    return this.api.get<ThumbnailSet>(
      `/shorts/${jobId}/thumbnails`,
      undefined,
      true,
      new HttpContext().set(SUPPRESS_GLOBAL_ERROR, true),
    );
  }

  /** POST /shorts/{id}/thumbnails: start, or regenerate with new options. */
  start(jobId: string, options: StartOptions = {}): Observable<{ status: string }> {
    return this.api.post<{ status: string }>(`/shorts/${jobId}/thumbnails`, {
      style: options.style ?? '',
      format: options.format ?? '',
    });
  }

  /** POST /shorts/{id}/thumbnails/select: save the chosen thumbnail with the video. */
  select(jobId: string, id: string): Observable<ThumbnailSet> {
    return this.api.post<ThumbnailSet>(`/shorts/${jobId}/thumbnails/select`, { id });
  }

  /** GET .../{thumbId}/download: a link that saves the JPEG instead of showing it. */
  downloadUrl(jobId: string, id: string): Observable<{ url: string; filename: string }> {
    return this.api.get<{ url: string; filename: string }>(`/shorts/${jobId}/thumbnails/${id}/download`);
  }
}

/** True while a generation is queued or running. */
export function isInFlight(status: ThumbnailStatus | undefined): boolean {
  return status === 'queued' || status === 'running';
}

/** Index of a stage in the list, or -1 if unknown/not started. */
export function stageIndex(stage: ThumbnailStage | undefined): number {
  return THUMBNAIL_STAGES.findIndex((s) => s.key === stage);
}
