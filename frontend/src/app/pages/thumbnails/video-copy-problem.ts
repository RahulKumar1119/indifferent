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

/**
 * What to tell the person when writing the titles fails. The words live here so
 * every failure reads the same everywhere. The API's own text is used only for a
 * 400, where it names the exact problem ("give a topic or a transcript of at
 * least 15 characters"). Nothing the person typed is lost by any of these.
 */
export function describeCopyProblem(error: unknown): StoryPlanProblem {
  if (!(error instanceof HttpErrorResponse)) {
    return { kind: 'failed', title: 'Something went wrong', message: 'Try again in a moment.', retryable: true };
  }
  switch (error.status) {
    case 0:
      return { kind: 'offline', title: "You're offline", message: 'Check your connection, then try again.', retryable: true };
    case 400: {
      if (code(error) === 'NOT_READY') {
        return {
          kind: 'not_ready',
          title: "This video's transcript isn't ready",
          message: message(error) || 'Paste a transcript, or describe the video in a sentence.',
          retryable: false,
        };
      }
      return {
        kind: 'invalid',
        title: "That can't be used yet",
        message: message(error) || 'Check what you entered and try again.',
        retryable: false,
      };
    }
    case 403:
      if (code(error) === 'EMAIL_NOT_VERIFIED') {
        return {
          kind: 'disabled',
          title: 'Confirm your email first',
          message: 'We sent you a link. You can ask for a new one on your account page.',
          retryable: false,
        };
      }
      break;
    case 404:
      return {
        kind: 'not_found',
        title: "We can't find this video",
        message: 'It may have been removed, or it belongs to another account.',
        retryable: false,
      };
    case 408:
    case 504:
      return { kind: 'timeout', title: 'Writing took too long', message: "Try again. This didn't use up one of today's tries.", retryable: true };
    case 429: {
      const wait = retryAfterSeconds(error);
      const after = wait === undefined ? 'tomorrow' : `in ${formatWait(wait)}`;
      return {
        kind: 'rate_limited',
        title: "You've used today's tries",
        message: `You can write more titles ${after}. Titles and descriptions you already have stay on this page.`,
        retryable: false,
        ...(wait === undefined ? {} : { retryAfterSeconds: wait }),
      };
    }
    case 503:
      if (code(error) === 'UNAVAILABLE') {
        return {
          kind: 'disabled',
          title: "Title writing isn't switched on yet",
          message: "Nothing is wrong with your video. This feature isn't available on this site yet.",
          retryable: false,
        };
      }
      return { kind: 'unavailable', title: 'The writer is busy', message: "Wait a minute, then try again. This didn't use up one of today's tries.", retryable: true };
    case 502:
      return {
        kind: 'failed',
        title: "The writer didn't give a usable answer",
        message: "Try again. This didn't use up one of today's tries.",
        retryable: true,
      };
  }
  return { kind: 'failed', title: 'Something went wrong', message: 'Try again in a moment.', retryable: true };
}
