import { HttpContext } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { ApiService } from '../../core';
import { SUPPRESS_GLOBAL_ERROR } from '../../core/interceptors/error.interceptor';
import { ClipHook, ClipRenderResponse, HookSuggestResponse, HookView } from './clip-hook.models';

function base(jobId: string, clipId: string): string {
  return `/shorts/${encodeURIComponent(jobId)}/clips/${encodeURIComponent(clipId)}`;
}

/** Client for a clip's hook line and for rendering one clip again (#329). */
@Injectable({ providedIn: 'root' })
export class ClipHookService {
  private readonly api = inject(ApiService);

  /** The panel reports every failure itself, so the global error toast stays quiet. */
  private quiet(): HttpContext {
    return new HttpContext().set(SUPPRESS_GLOBAL_ERROR, true);
  }

  view(jobId: string, clipId: string): Observable<HookView> {
    return this.api.get<HookView>(`${base(jobId, clipId)}/hook`, undefined, true, this.quiet());
  }

  save(jobId: string, clipId: string, hook: ClipHook): Observable<{ hook: ClipHook }> {
    const { enabled, text, style, position, seconds } = hook;
    return this.api.put<{ hook: ClipHook }>(`${base(jobId, clipId)}/hook`, { hook: { enabled, text: text.trim(), style, position, seconds } }, true, this.quiet());
  }

  reset(jobId: string, clipId: string): Observable<{ reset: boolean; hook: ClipHook }> {
    return this.api.put<{ reset: boolean; hook: ClipHook }>(`${base(jobId, clipId)}/hook`, { reset: true }, true, this.quiet());
  }

  /** Three lines from the clip's words. The same requestId on a retry gets the same answer and uses nothing. */
  suggest(jobId: string, clipId: string, requestId: string): Observable<HookSuggestResponse> {
    return this.api.post<HookSuggestResponse>(`${base(jobId, clipId)}/hook-suggestions`, { requestId }, true, this.quiet());
  }

  /** Renders the clip again with its saved hook, trim and captions. A repeat with the same requestId starts nothing new. */
  render(jobId: string, clipId: string, requestId: string): Observable<ClipRenderResponse> {
    return this.api.post<ClipRenderResponse>(`${base(jobId, clipId)}/render`, { requestId }, true, this.quiet());
  }
}
