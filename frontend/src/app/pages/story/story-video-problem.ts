import { HttpErrorResponse } from '@angular/common/http';
import { ProblemKind, StoryPlanProblem, formatWait, retryAfterSeconds } from './story-plan-problem';

/** What the person was doing when the request failed; the same status reads differently in each. */
export type VideoContext = 'make' | 'check' | 'load' | 'cancel' | 'download' | 'delete';

function code(err: HttpErrorResponse): string {
  const c = (err.error as { code?: unknown } | null)?.code;
  return typeof c === 'string' ? c : '';
}

/** The API's own sentence, used only where it names the exact problem (a scene, a voice). */
function serverMessage(err: HttpErrorResponse): string {
  const m = (err.error as { message?: unknown } | null)?.message;
  return typeof m === 'string' ? m.trim() : '';
}

function jobIdOf(err: HttpErrorResponse): string | undefined {
  const id = (err.error as { details?: { jobId?: unknown } } | null)?.details?.jobId;
  return typeof id === 'string' && id ? id : undefined;
}

function problem(
  kind: ProblemKind,
  title: string,
  message: string,
  retryable: boolean,
  extra: Pick<StoryPlanProblem, 'retryAfterSeconds' | 'jobId'> = {},
): StoryPlanProblem {
  return { kind, title, message, retryable, ...extra };
}

/**
 * Turns a failed video request into something the page can show: what happened
 * and what to do next. Copy lives here so every control says the same thing
 * about the same failure. The API's own text is used where it names the exact
 * problem (which scenes have no picture, which voice cannot read the language).
 */
export function describeVideoProblem(error: unknown, context: VideoContext): StoryPlanProblem {
  if (!(error instanceof HttpErrorResponse)) {
    return problem('failed', 'Something went wrong', 'Try again in a moment.', true);
  }
  if (error.status === 0) {
    return problem('offline', "You're offline", 'Check your connection, then try again.', true);
  }
  const c = code(error);

  switch (error.status) {
    case 400:
      if (c === 'MISSING_PICTURES') {
        return problem('invalid', 'Some scenes have no picture', serverMessage(error) || 'Choose a picture for every scene, save, then try again.', false);
      }
      if (c === 'UNSUPPORTED_LANGUAGE') {
        return problem('invalid', "This story can't be narrated", serverMessage(error) || 'Narration is available in English and Hindi.', false);
      }
      return problem('invalid', "Those settings can't be used", serverMessage(error) || 'Check the voice and captions and try again.', false);

    case 404:
      if (context === 'make') {
        return problem('not_found', 'This plan no longer exists', 'It may have been removed, or it belongs to another account. Start a new plan.', false);
      }
      return problem('not_found', "We can't find that video", 'It may have been deleted. Reload the page to see what is left.', false);

    case 409:
      switch (c) {
        case 'STALE_REVISION':
          return problem(
            'stale',
            'This plan changed somewhere else',
            'No video was started. Reload the plan to see the newest version, then make the video from it.',
            false,
          );
        case 'NOT_READY':
          return context === 'download'
            ? problem('not_ready', "This video isn't finished yet", 'Wait until it is ready, then download it.', true)
            : problem('not_ready', 'This plan has nothing to make a video from', "It didn't finish. Start a new plan.", false);
        case 'ALREADY_RUNNING': {
          const jobId = jobIdOf(error);
          return problem('already_running', 'A video is already being made from this plan', 'You can follow it here, or cancel it and start again.', false, jobId ? { jobId } : {});
        }
        case 'TOO_MANY_ACTIVE':
          return problem('too_many_active', 'Your other videos are still being made', 'Wait for one to finish, or cancel one from its plan, then try again.', true);
        case 'STILL_RUNNING':
          return problem('still_running', 'This video is still being made', 'Cancel it first, then delete it.', false);
      }
      return problem('failed', 'That could not be done', 'Reload the page and try again.', true);

    case 429: {
      const wait = retryAfterSeconds(error);
      const after = wait === undefined ? 'in a little while' : `in ${formatWait(wait)}`;
      return problem(
        'rate_limited',
        "You've made today's videos",
        `The limit is per day, and it counts videos that were cancelled or did not finish. Try again ${after}.`,
        true,
        wait === undefined ? {} : { retryAfterSeconds: wait },
      );
    }

    case 503:
      if (c === 'UNAVAILABLE') {
        return problem('disabled', "Making videos isn't switched on yet", "Nothing is wrong with your plan. This feature isn't available on this site yet.", false);
      }
      return problem('unavailable', 'Temporarily unavailable', 'Try again in a minute.', true);

    case 408:
    case 504:
      return problem('timeout', 'That took too long', 'Try again.', true);
  }

  if (c === 'S3_CLEANUP_FAILED') {
    return problem('failed', "The video's files weren't deleted", 'The video is still in your list. Try again.', true);
  }
  switch (context) {
    case 'make':
      return problem('failed', "The video couldn't be started", 'Nothing was made. Try again in a moment.', true);
    case 'check':
      return problem('failed', "Can't check on your video", 'It keeps being made while we try again.', true);
    case 'cancel':
      return problem('failed', "The video wasn't cancelled", 'Something went wrong on our side. Try again.', true);
    case 'download':
      return problem('failed', "The download couldn't be prepared", 'Something went wrong on our side. Try again.', true);
    case 'delete':
      return problem('failed', "The video wasn't deleted", 'Something went wrong on our side. Try again.', true);
    default:
      return problem('failed', "Your videos couldn't be loaded", 'Something went wrong on our side. Try again.', true);
  }
}
