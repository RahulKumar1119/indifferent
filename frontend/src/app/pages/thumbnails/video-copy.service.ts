import { Injectable, inject } from '@angular/core';
import { HttpContext } from '@angular/common/http';
import { Observable } from 'rxjs';
import { ApiService } from '../../core';
import { SUPPRESS_GLOBAL_ERROR } from '../../core/interceptors/error.interceptor';

/** GET /shorts/{id}/copy/status (matches backend CopyStatus). */
export interface CopyStatus {
  /** False when the helper is switched off on the server. */
  enabled: boolean;
  /** This video has a transcript the helper can read. */
  transcriptAvailable: boolean;
  /** Uses left today, when the server could count them. */
  remainingToday?: number;
  dailyLimit: number;
  maxTopicChars: number;
  maxTranscriptChars: number;
  maxRefineChars: number;
}

/** POST /shorts/{id}/copy (matches backend CopyRequest). */
export interface CopyRequest {
  topic?: string;
  /** Pasted text. Lines that begin with a time, such as "1:05 text", give it timestamps. */
  transcript?: string;
  /** Use this video's own transcript, with its real times. */
  useJobTranscript?: boolean;
  /** A short instruction for making different titles. */
  refine?: string;
  previousTitles?: string[];
}

export interface CopyChapter {
  /** Seconds from the start of the video. */
  start: number;
  /** The time as it is written in the description ("1:05"). */
  time: string;
  title: string;
}

/** The body of a successful POST (matches backend CopyResponse). */
export interface CopyResult {
  titles: string[];
  description: string;
  chapters: CopyChapter[];
  /** The description followed by the chapter list, ready to paste. */
  descriptionWithChapters: string;
  hasTimestamps: boolean;
  /** Why something was left out, in words that can be shown. */
  notes: string[];
  remainingToday?: number;
  dailyLimit: number;
}

/** Titles may be at most this long on YouTube. */
export const MAX_TITLE_CHARS = 100;

/** Client for the title, description and chapters helper (#331). */
@Injectable({ providedIn: 'root' })
export class VideoCopyService {
  private readonly api = inject(ApiService);

  /** The page reports every failure itself, so the global error toast stays quiet. */
  private quiet(): HttpContext {
    return new HttpContext().set(SUPPRESS_GLOBAL_ERROR, true);
  }

  status(jobId: string): Observable<CopyStatus> {
    return this.api.get<CopyStatus>(`/shorts/${encodeURIComponent(jobId)}/copy/status`, undefined, true, this.quiet());
  }

  write(jobId: string, body: CopyRequest): Observable<CopyResult> {
    return this.api.post<CopyResult>(`/shorts/${encodeURIComponent(jobId)}/copy`, body, true, this.quiet());
  }
}
