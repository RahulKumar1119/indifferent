import { HttpErrorResponse } from '@angular/common/http';
import { StoryPlanProblem, formatWait, retryAfterSeconds } from '../../pages/story/story-plan-problem';

function code(err: HttpErrorResponse): string {
  const c = (err.error as { code?: unknown } | null)?.code;
  return typeof c === 'string' ? c : '';
}

function message(err: HttpErrorResponse): string {
  const m = (err.error as { message?: unknown } | null)?.message;
  return typeof m === 'string' ? m.trim() : '';
}

/** What the person was doing when it failed. The same status reads differently in each. */
export type PostCopyAction = 'load' | 'write' | 'save';

/**
 * What to tell the person when the post copy fails. The API's own words are used
 * only for a 400, where they name the exact problem (a limit, a hashtag). Nothing
 * the person typed is lost by any of these.
 */
export function describePostCopyProblem(error: unknown, action: PostCopyAction): StoryPlanProblem {
  if (!(error instanceof HttpErrorResponse)) {
    return { kind: 'failed', title: 'Something went wrong', message: 'Try again in a moment.', retryable: true };
  }
  const used = action === 'write' ? " This didn't use up one of today's tries." : '';
  switch (error.status) {
    case 0:
      return { kind: 'offline', title: "You're offline", message: 'Check your connection, then try again. What you typed is still here.', retryable: true };
    case 400: {
      if (code(error) === 'NOT_READY') {
        return { kind: 'not_ready', title: 'Not enough to write from', message: message(error) || 'There is not enough spoken text yet.', retryable: false };
      }
      return {
        kind: 'invalid',
        title: action === 'save' ? "That can't be saved yet" : "That can't be used yet",
        message: message(error) || 'Check what you entered and try again.',
        retryable: false,
      };
    }
    case 403:
      if (code(error) === 'EMAIL_NOT_VERIFIED') {
        return { kind: 'disabled', title: 'Confirm your email first', message: 'We sent you a link. You can ask for a new one on your account page.', retryable: false };
      }
      break;
    case 404:
      return { kind: 'not_found', title: "We can't find this", message: 'It may have been removed, or it belongs to another account.', retryable: false };
    case 408:
    case 504:
      return { kind: 'timeout', title: 'Writing took too long', message: `Try again.${used}`, retryable: true };
    case 409:
      if (code(error) === 'IN_PROGRESS') {
        return { kind: 'unavailable', title: 'Still writing', message: 'The first try has not finished. Wait a moment, then try again.', retryable: true };
      }
      if (code(error) === 'NOT_READY') {
        return { kind: 'not_ready', title: 'Not ready yet', message: message(error) || 'Write the post copy once the video is finished.', retryable: false };
      }
      return { kind: 'failed', title: 'That did not work', message: message(error) || 'Start again.', retryable: true };
    case 429: {
      const wait = retryAfterSeconds(error);
      const after = wait === undefined ? 'tomorrow' : `in ${formatWait(wait)}`;
      return {
        kind: 'rate_limited',
        title: "You've used today's tries",
        message: `You can write more ${after}. Copy you already have stays on this page.`,
        retryable: false,
        ...(wait === undefined ? {} : { retryAfterSeconds: wait }),
      };
    }
    case 503:
      if (code(error) === 'UNAVAILABLE') {
        return { kind: 'disabled', title: "Post copy isn't switched on yet", message: "Nothing is wrong with your video. This feature isn't available on this site yet.", retryable: false };
      }
      return { kind: 'unavailable', title: 'The writer is busy', message: `Wait a minute, then try again.${used}`, retryable: true };
    case 502:
      return { kind: 'failed', title: "The writer didn't give a usable answer", message: `Try again.${used}`, retryable: true };
  }
  return { kind: 'failed', title: 'Something went wrong', message: action === 'save' ? 'Your edits are still here. Try saving again.' : 'Try again in a moment.', retryable: true };
}
