import { Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { ApiService } from '../../core';

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
  createdAt: string;
  updatedAt: string;
  completedAt?: string;
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
 * Client for the Shorts_API routes. Calls flow through ApiService so JWT auth
 * (cookie credentials) is applied, except presigned S3 uploads which use the
 * absolute-URL passthrough.
 */
@Injectable({ providedIn: 'root' })
export class ShortsService {
  constructor(private readonly api: ApiService) {}

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

  /** GET /shorts/{id}: fetch current job status. */
  getStatus(jobId: string): Observable<ShortsJob> {
    return this.api.get<ShortsJob>(`/shorts/${jobId}`);
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
