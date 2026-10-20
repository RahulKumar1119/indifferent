import { Injectable } from '@angular/core';
import { HttpContext } from '@angular/common/http';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { ApiService } from '../../core';
import { SUPPRESS_GLOBAL_ERROR } from '../../core/interceptors/error.interceptor';

/** Status values reported by the Shorts pipeline. */
export type ShortsStatus =
  | 'uploaded'
  | 'transcribing'
  | 'ranking'
  | 'rendering'
  | 'completed'
  | 'failed';

/** A rendered 9:16 vertical clip (matches backend models.Clip). */
export interface Clip {
  clipId: string;
  s3Key: string;
  rank: number;
  score: number;
  duration: number;
  hookText?: string;
  start?: number;
  end?: number;
}

/** Response from GET /shorts/{id}/clips/{clipId}/url. Archived clips answer
 * 202 with code/message and no url while Glacier restore runs. */
export interface ClipUrlResponse {
  url?: string;
  code?: string;
  message?: string;
}

/** A single AI Shorts processing run (matches backend models.ShortsJob). */
export interface ShortsJob {
  userId: string;
  jobId: string;
  status: ShortsStatus;
  fileType: string;
  sourceDuration: number;
  sourceKey: string;
  transcriptKey?: string;
  clips?: Clip[];
  /** 0-100 progress within the current stage (backend-computed). */
  progress?: number;
  error?: string;
  /** Stage that was running on failure + machine-readable error class. */
  failedStage?: string;
  errorCode?: string;
  /** Reusable caption style for this job's renders. */
  caption?: CaptionStyle;
  /** Per-clip user overrides (boundaries + corrected captions). */
  clipEdits?: Record<string, ClipEdit>;
  createdAt: string;
  updatedAt: string;
  completedAt?: string;
}

/** Reusable caption style stored per shorts job (mirrors backend models). */
export interface CaptionStyle {
  preset?: 'minimal' | 'bold' | 'karaoke' | 'clean';
  fontSize?: number;
  color?: string;
  highlight?: string;
  position?: 'top' | 'middle' | 'bottom';
}

export const DEFAULT_CAPTION_STYLE: Required<CaptionStyle> = {
  preset: 'bold',
  fontSize: 18,
  color: '#FFFFFF',
  highlight: '#FFD60A',
  position: 'bottom',
};

/** A word with preserved timing (mirrors backend models.Word). */
export interface CaptionWord {
  text: string;
  start: number;
  end: number;
}

/** A saved per-clip override (mirrors backend models.ClipEdit). */
export interface ClipEdit {
  start: number;
  end: number;
  words?: CaptionWord[];
  updatedAt?: string;
}

/** Trimmed shorts job for history lists (matches backend ShortsJobSummary). */
export interface ShortsJobSummary {
  jobId: string;
  projectId?: string;
  status: ShortsStatus;
  fileType: string;
  sourceDuration: number;
  clipCount: number;
  createdAt: string;
  updatedAt: string;
}

/** Response from POST /shorts (matches backend CreateShortsResponse). */
export interface CreateShortsResponse {
  jobId: string;
  uploadUrl: string;
  sourceKey: string;
}

/**
 * The source limits the Shorts pipeline enforces. Read from GET /shorts/limits
 * so the numbers shown in the UI can never drift from the ones the API
 * applies.
 */
export interface ShortsLimits {
  maxDurationSeconds: number;
  maxDurationMinutes: number;
  maxUploadBytes: number;
  maxUploadMb: number;
  maxClipCount: number;
  minClipSeconds: number;
  maxClipSeconds: number;
  acceptedFileTypes: string[];
}

/**
 * Fallback limits, used only until GET /shorts/limits answers. They match the
 * backend constants; the API response replaces them as soon as it arrives.
 */
export const DEFAULT_SHORTS_LIMITS: ShortsLimits = {
  maxDurationSeconds: 3600,
  maxDurationMinutes: 60,
  maxUploadBytes: 2 * 1024 * 1024 * 1024,
  maxUploadMb: 2048,
  maxClipCount: 3,
  minClipSeconds: 30,
  maxClipSeconds: 90,
  acceptedFileTypes: ['mp3', 'mp4', 'mov', 'wav'],
};

/**
 * Client for the Shorts_API routes. Calls flow through ApiService so JWT auth
 * (cookie credentials) is applied, except presigned S3 uploads which use the
 * absolute-URL passthrough.
 */
@Injectable({ providedIn: 'root' })
export class ShortsService {
  constructor(private readonly api: ApiService) {}

  /** GET /shorts/limits: the source limits the pipeline enforces. */
  getLimits(): Observable<ShortsLimits> {
    return this.api.get<ShortsLimits>('/shorts/limits');
  }

  /** POST /shorts: create a job and get a presigned upload URL. */
  createJob(fileType: string, duration: number): Observable<CreateShortsResponse> {
    return this.api.post<CreateShortsResponse>('/shorts', { fileType, duration });
  }

  /** PUT the source media to the presigned S3 URL (no auth cookies). */
  uploadSource(uploadUrl: string, file: File): Observable<unknown> {
    return this.api.putAbsolute<unknown>(uploadUrl, file);
  }

  /** POST /shorts/{id}/start: kick off the processing pipeline. */
  startJob(jobId: string): Observable<unknown> {
    return this.api.post<unknown>(`/shorts/${jobId}/start`);
  }

  /** POST /shorts/{id}/cancel: stop a running pipeline. */
  cancelJob(jobId: string): Observable<{ status: string }> {
    return this.api.post<{ status: string }>(`/shorts/${jobId}/cancel`);
  }

  /** POST /shorts/{id}/retry: restart a failed job from its source media. */
  retryJob(jobId: string): Observable<{ status: string }> {
    return this.api.post<{ status: string }>(`/shorts/${jobId}/retry`);
  }

  /** PUT /shorts/{id}/style: store a validated caption style for future renders. */
  updateCaption(jobId: string, caption: CaptionStyle): Observable<ShortsJob> {
    return this.api.put<ShortsJob>(`/shorts/${jobId}/style`, { caption });
  }

  /** GET /shorts/{id}/clips/{clipId}/captions: words inside the clip window. */
  getClipCaptions(jobId: string, clipId: string): Observable<{ start: number; end: number; words: CaptionWord[] }> {
    return this.api.get<{ start: number; end: number; words: CaptionWord[] }>(`/shorts/${jobId}/clips/${clipId}/captions`);
  }

  /** PUT /shorts/{id}/clips/{clipId}: save boundary/caption override (or reset). */
  updateClipEdit(
    jobId: string,
    clipId: string,
    body: { start?: number; end?: number; words?: CaptionWord[]; reset?: boolean },
  ): Observable<{ edit?: ClipEdit; droppedWords?: number; reset?: boolean }> {
    return this.api.put<{ edit?: ClipEdit; droppedWords?: number; reset?: boolean }>(
      `/shorts/${jobId}/clips/${clipId}`,
      body,
    );
  }

  /** DELETE /shorts/{id}: permanently delete a job and all its S3 assets. */
  deleteJob(jobId: string): Observable<void> {
    return this.api.delete<void>(`/shorts/${jobId}`);
  }

  /** GET /shorts/{id}: fetch current job status. */
  getStatus(jobId: string): Observable<ShortsJob> {
    // Polled by progress pages: keep the global error toast quiet here.
    return this.api.get<ShortsJob>(
      `/shorts/${jobId}`,
      undefined,
      true,
      new HttpContext().set(SUPPRESS_GLOBAL_ERROR, true),
    );
  }

  /** GET /shorts: list my shorts jobs, newest first. */
  listJobs(): Observable<ShortsJobSummary[]> {
    return this.api
      .get<{ jobs: ShortsJobSummary[] }>('/shorts')
      .pipe(map((res) => (Array.isArray(res?.jobs) ? res.jobs : [])));
  }

  /** GET /shorts/{id}/clips: list rendered clips ordered by rank. */
  listClips(jobId: string): Observable<Clip[]> {
    // The API returns an envelope {"clips": [...]}; unwrap it here so callers
    // always receive an array (a bare array is also tolerated defensively).
    return this.api.get<{ clips: Clip[] } | Clip[]>(`/shorts/${jobId}/clips`).pipe(
      map((res) => (Array.isArray(res) ? res : (res?.clips ?? []))),
    );
  }

  /** GET /shorts/{id}/clips/{clipId}/url: presigned GET URL for a clip. */
  getClipUrl(jobId: string, clipId: string): Observable<ClipUrlResponse> {
    return this.api.get<ClipUrlResponse>(`/shorts/${jobId}/clips/${clipId}/url`);
  }
}
