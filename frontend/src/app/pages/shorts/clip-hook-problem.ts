import { HttpErrorResponse } from '@angular/common/http';
import { StoryPlanProblem, formatWait, retryAfterSeconds } from '../story/story-plan-problem';

function code(err: HttpErrorResponse): string {
  const c = (err.error as { code?: unknown } | null)?.code;
  return typeof c === 'string' ? c : '';
}

function message(err: HttpErrorResponse): string {
  const m = (err.error as { message?: unknown } | null)?.message;
  return typeof m === 'string' ? m.trim() : '';
}

/** What the person was doing when it failed. */
export type HookAction = 'load' | 'save' | 'suggest' | 'render';

/**
 * What to tell the person when the hook panel fails. The API's own words are
 * used only where they name the exact problem (a 400, or a conflict it explains).
 * Nothing typed is lost, and a failed render never touches the clip that plays.
 */
export function describeHookProblem(error: unknown, action: HookAction): StoryPlanProblem {
  if (!(error instanceof HttpErrorResponse)) {
    return { kind: 'failed', title: 'Something went wrong', message: 'Try again in a moment.', retryable: true };
  }
  const notUsed = action === 'suggest' || action === 'render' ? " This didn't use up one of today's tries." : '';
  switch (error.status) {
    case 0:
      return { kind: 'offline', title: "You're offline", message: 'Check your connection, then try again. What you typed is still here.', retryable: true };
    case 400:
      return {
        kind: 'invalid',
        title: action === 'save' ? "That hook can't be saved" : "That can't be done yet",
        message: message(error) || 'Check the hook and try again.',
        retryable: false,
      };
    case 404:
      return { kind: 'not_found', title: "We can't find this clip", message: 'It may have been removed, or it belongs to another account.', retryable: false };
    case 408:
    case 504:
      return { kind: 'timeout', title: 'That took too long', message: `Try again.${notUsed}`, retryable: true };
    case 409:
      switch (code(error)) {
        case 'RENDER_IN_PROGRESS':
          return { kind: 'unavailable', title: 'Already rendering', message: 'This clip is being rendered again. Wait for it to finish.', retryable: false };
        case 'IN_PROGRESS':
          return { kind: 'unavailable', title: 'Still writing', message: 'The first try has not finished. Wait a moment, then try again.', retryable: true };
        case 'CONFLICT':
          return { kind: 'failed', title: 'This clip changed', message: 'It was changed somewhere else. Reload the page, then try again.', retryable: false };
      }
      return { kind: 'not_ready', title: "That can't be done yet", message: message(error) || 'Try again later.', retryable: false };
    case 429: {
      const wait = retryAfterSeconds(error);
      const after = wait === undefined ? 'tomorrow' : `in ${formatWait(wait)}`;
      const what = action === 'render'
        ? `You can render clips again ${after}. Retrying a failed video uses the same allowance. The clip you have stays as it is.`
        : `You can ask for more ${after}. Suggestions share the allowance with titles and post copy. You can still type your own hook.`;
      return {
        kind: 'rate_limited',
        title: action === 'render' ? "You've used today's renders" : "You've used today's suggestions",
        message: what,
        retryable: false,
        ...(wait === undefined ? {} : { retryAfterSeconds: wait }),
      };
    }
    case 502:
      return { kind: 'failed', title: "The writer didn't give a usable answer", message: `Try again, or type your own hook.${notUsed}`, retryable: true };
    case 503:
      if (code(error) === 'UNAVAILABLE') {
        return {
          kind: 'disabled',
          title: action === 'render' ? "Rendering again isn't switched on yet" : "Suggestions aren't switched on yet",
          message: action === 'render' ? 'Your hook is saved and will be used once it is.' : 'You can still type your own hook.',
          retryable: false,
        };
      }
      return { kind: 'unavailable', title: 'Busy right now', message: `Wait a minute, then try again.${notUsed}`, retryable: true };
  }
  return { kind: 'failed', title: 'Something went wrong', message: action === 'save' ? 'Your hook is still here. Try saving again.' : 'Try again in a moment.', retryable: true };
}
