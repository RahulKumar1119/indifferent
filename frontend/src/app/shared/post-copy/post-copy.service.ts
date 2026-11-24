import { HttpContext } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { ApiService } from '../../core';
import { SUPPRESS_GLOBAL_ERROR } from '../../core/interceptors/error.interceptor';
import { CopyMap, PostCopyListView, PostCopyResult, PostCopySaved, PostCopyTarget, PostCopyView } from './post-copy.models';

/** The API path of one clip's or one story video's copy. */
export function postCopyPath(t: PostCopyTarget): string {
  return t.kind === 'clip'
    ? `/shorts/${encodeURIComponent(t.jobId)}/clips/${encodeURIComponent(t.clipId)}/post-copy`
    : `/story-videos/${encodeURIComponent(t.videoId)}/post-copy`;
}

/** Client for the copy written for each platform (#330). */
@Injectable({ providedIn: 'root' })
export class PostCopyService {
  private readonly api = inject(ApiService);

  /** The page reports every failure itself, so the global error toast stays quiet. */
  private quiet(): HttpContext {
    return new HttpContext().set(SUPPRESS_GLOBAL_ERROR, true);
  }

  view(target: PostCopyTarget): Observable<PostCopyView> {
    return this.api.get<PostCopyView>(postCopyPath(target), undefined, true, this.quiet());
  }

  /** The saved copy of every clip of a Shorts job, in one request. */
  list(jobId: string): Observable<PostCopyListView> {
    return this.api.get<PostCopyListView>(`/shorts/${encodeURIComponent(jobId)}/post-copy`, undefined, true, this.quiet());
  }

  /** Writes the copy. The same requestId on a retry gets the same answer and uses nothing. */
  write(target: PostCopyTarget, requestId: string): Observable<PostCopyResult> {
    return this.api.post<PostCopyResult>(postCopyPath(target), { requestId }, true, this.quiet());
  }

  /** Saves what the person edited. The server refuses a copy that is over a limit. */
  save(target: PostCopyTarget, copy: CopyMap): Observable<PostCopySaved> {
    return this.api.put<PostCopySaved>(postCopyPath(target), { copy }, true, this.quiet());
  }
}
