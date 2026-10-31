import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { ApiService } from '../../core/services/api.service';
import { INLINE_ERRORS } from './story-plan.service';
import {
  CreateStoryVideoRequest,
  StoryVideo,
  StoryVideoDownload,
  StoryVideoLimits,
  StoryVideoList,
} from './story-video.models';

/**
 * Story2Shorts API client for the video made from a plan: /story-videos. The
 * page reports every failure inline (see describeVideoProblem), so none of
 * these requests also raises the global error toast.
 */
@Injectable({ providedIn: 'root' })
export class StoryVideoService {
  private readonly api = inject(ApiService);

  /** What the page may offer. Public, so it answers even when signed out. */
  getLimits(): Observable<StoryVideoLimits> {
    return this.api.get<StoryVideoLimits>('/story-videos/limits', undefined, true, INLINE_ERRORS());
  }

  /** Starts a video. Answers at once (202); the rest happens in the background. */
  create(request: CreateStoryVideoRequest): Observable<StoryVideo> {
    return this.api.post<StoryVideo>('/story-videos', request, true, INLINE_ERRORS());
  }

  get(jobId: string): Observable<StoryVideo> {
    return this.api.get<StoryVideo>(`/story-videos/${encodeURIComponent(jobId)}`, undefined, true, INLINE_ERRORS());
  }

  /** The user's videos made from one plan, newest first. */
  listForPlan(planId: string): Observable<StoryVideoList> {
    return this.api.get<StoryVideoList>('/story-videos', { planId }, true, INLINE_ERRORS());
  }

  /** Ends a video that is being made. A video that has already ended is returned unchanged. */
  cancel(jobId: string): Observable<StoryVideo> {
    return this.api.post<StoryVideo>(`/story-videos/${encodeURIComponent(jobId)}/cancel`, {}, true, INLINE_ERRORS());
  }

  /** Removes a finished or cancelled video and its files. */
  remove(jobId: string): Observable<void> {
    return this.api.delete<void>(`/story-videos/${encodeURIComponent(jobId)}`, INLINE_ERRORS());
  }

  /** A link to the finished file. It expires, so ask when it is needed, not before. */
  download(jobId: string): Observable<StoryVideoDownload> {
    return this.api.get<StoryVideoDownload>(
      `/story-videos/${encodeURIComponent(jobId)}/download`,
      undefined,
      true,
      INLINE_ERRORS(),
    );
  }
}
