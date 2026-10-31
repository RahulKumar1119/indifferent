import { Injectable, inject } from '@angular/core';
import { HttpContext } from '@angular/common/http';
import { Observable } from 'rxjs';
import { ApiService } from '../../core/services/api.service';
import { SUPPRESS_GLOBAL_ERROR } from '../../core/interceptors/error.interceptor';
import {
  SceneEdit,
  StockSearchParams,
  StockSearchResponse,
  StoryPlanLimits,
  StoryPlanRecord,
} from './story-plan.models';

// The page reports every failure inline (see describeProblem), so the global
// error toast would only say the same thing twice.
export const INLINE_ERRORS = () => new HttpContext().set(SUPPRESS_GLOBAL_ERROR, true);

/**
 * Story2Shorts API client: /story-plans and /stock-media. The browser talks
 * only to our API; the providers' keys and calls stay in the backend.
 */
@Injectable({ providedIn: 'root' })
export class StoryPlanService {
  private readonly api = inject(ApiService);

  /** What the planner accepts. Public, so it answers even when signed out. */
  getLimits(): Observable<StoryPlanLimits> {
    return this.api.get<StoryPlanLimits>('/story-plans/limits', undefined, true, INLINE_ERRORS());
  }

  /** Plans a video from a story. Synchronous on the server: typically 5 to 10 seconds. */
  create(story: string): Observable<StoryPlanRecord> {
    return this.api.post<StoryPlanRecord>('/story-plans', { story }, true, INLINE_ERRORS());
  }

  get(planId: string): Observable<StoryPlanRecord> {
    return this.api.get<StoryPlanRecord>(`/story-plans/${encodeURIComponent(planId)}`, undefined, true, INLINE_ERRORS());
  }

  /**
   * Saves edits. `scenes` needs exactly one entry per scene, in order, and
   * `revision` must be the one the edit was based on (409 otherwise).
   */
  save(planId: string, revision: number, scenes: SceneEdit[]): Observable<StoryPlanRecord> {
    return this.api.put<StoryPlanRecord>(
      `/story-plans/${encodeURIComponent(planId)}`,
      { revision, scenes },
      true,
      INLINE_ERRORS(),
    );
  }

  /** Photos or clips for one scene. Portrait unless `orientation` says otherwise. */
  search(params: StockSearchParams): Observable<StockSearchResponse> {
    const query: Record<string, string | number> = { q: params.q, type: params.type };
    if (params.provider) query['provider'] = params.provider;
    if (params.orientation) query['orientation'] = params.orientation;
    if (params.page) query['page'] = params.page;
    if (params.perPage) query['perPage'] = params.perPage;
    return this.api.get<StockSearchResponse>('/stock-media/search', query, true, INLINE_ERRORS());
  }
}
