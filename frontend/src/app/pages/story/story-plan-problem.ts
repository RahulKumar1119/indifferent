import { HttpErrorResponse } from '@angular/common/http';
import { PROVIDER_NAMES, StockProviderFailure } from './story-plan.models';

/** Where the failed request came from; the same status reads differently in each. */
export type ProblemContext = 'create' | 'load' | 'save' | 'search';

export type ProblemKind =
  | 'invalid' // 400: the input is not acceptable; the API says why
  | 'rate_limited' // 429
  | 'disabled' // 503 UNAVAILABLE: the feature is not switched on
  | 'unavailable' // 503 PLAN_UNAVAILABLE: busy right now
  | 'timeout' // 504, 408
  | 'failed' // 502, 5xx and anything unexpected
  | 'stale' // 409 STALE_REVISION: the plan changed elsewhere
  | 'not_ready' // 409 NOT_READY
  | 'not_found' // 404
  | 'offline' // no response at all
  // The three below are for the video made from a plan (story-video-problem.ts).
  | 'already_running' // 409 ALREADY_RUNNING: a video is being made from this plan
  | 'too_many_active' // 409 TOO_MANY_ACTIVE: the user's limit of videos at once
  | 'still_running'; // 409 STILL_RUNNING: delete needs a cancel first

/** A failure the page can show: what happened, and what to do next. */
export interface StoryPlanProblem {
  kind: ProblemKind;
  title: string;
  /** Names the cause and the next step. */
  message: string;
  /** Seconds to wait before retrying, when the API said so. */
  retryAfterSeconds?: number;
  /** True when trying the same thing again can work (after any wait). */
  retryable: boolean;
  /** The video a conflict is about, when the API names it (ALREADY_RUNNING). */
  jobId?: string;
}

/** "1 second", "45 seconds", "3 minutes", "1 hour 5 minutes". Rounds up. */
export function formatWait(seconds: number): string {
  const s = Math.max(1, Math.ceil(seconds));
  if (s < 60) return s === 1 ? '1 second' : `${s} seconds`;
  const minutes = Math.ceil(s / 60);
  if (minutes < 60) return minutes === 1 ? '1 minute' : `${minutes} minutes`;
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  const hours = h === 1 ? '1 hour' : `${h} hours`;
  if (m === 0) return hours;
  return `${hours} ${m === 1 ? '1 minute' : `${m} minutes`}`;
}

/**
 * Seconds to wait, from the body (details.retryAfter) first. The page and the
 * API are on different origins and the API does not expose Retry-After to
 * scripts, so the header is only a fallback for same-origin setups.
 */
export function retryAfterSeconds(err: HttpErrorResponse): number | undefined {
  const fromBody = (err.error as { details?: { retryAfter?: unknown } } | null)?.details?.retryAfter;
  if (typeof fromBody === 'number' && Number.isFinite(fromBody) && fromBody > 0) return fromBody;
  const header = Number(err.headers?.get('Retry-After'));
  return Number.isFinite(header) && header > 0 ? header : undefined;
}

function serverMessage(err: HttpErrorResponse): string {
  const m = (err.error as { message?: unknown } | null)?.message;
  return typeof m === 'string' ? m.trim() : '';
}

function serverCode(err: HttpErrorResponse): string {
  const c = (err.error as { code?: unknown } | null)?.code;
  return typeof c === 'string' ? c : '';
}

function kindOf(err: HttpErrorResponse): ProblemKind {
  const code = serverCode(err);
  switch (err.status) {
    case 0:
      return 'offline';
    case 400:
      return 'invalid';
    case 404:
      return 'not_found';
    case 408:
    case 504:
      return 'timeout';
    case 409:
      return code === 'NOT_READY' ? 'not_ready' : 'stale';
    case 429:
      return 'rate_limited';
    case 503:
      return code === 'UNAVAILABLE' ? 'disabled' : 'unavailable';
    default:
      return 'failed';
  }
}

function problem(
  kind: ProblemKind,
  title: string,
  message: string,
  retryable: boolean,
  wait?: number,
): StoryPlanProblem {
  return wait === undefined
    ? { kind, title, message, retryable }
    : { kind, title, message, retryable, retryAfterSeconds: wait };
}

/**
 * Turns a failed request into something the page can show. Copy lives here, in
 * one place, so every screen says the same thing about the same failure. The
 * API's own text is used only for 400, where it names the exact problem
 * ("Scene 2: the narration is empty").
 */
export function describeProblem(error: unknown, context: ProblemContext): StoryPlanProblem {
  if (!(error instanceof HttpErrorResponse)) {
    return problem('failed', 'Something went wrong', 'Try again in a moment.', true);
  }
  const kind = kindOf(error);
  const wait = kind === 'rate_limited' ? retryAfterSeconds(error) : undefined;

  switch (kind) {
    case 'offline':
      return problem('offline', "You're offline", 'Check your connection, then try again.', true);

    case 'invalid': {
      const title = {
        create: "That story can't be planned",
        load: "That plan can't be opened",
        save: "Those changes can't be saved",
        search: "That search can't be run",
      }[context];
      return problem('invalid', title, serverMessage(error) || 'Check what you entered and try again.', false);
    }

    case 'rate_limited': {
      const after = wait === undefined ? 'in a little while' : `in ${formatWait(wait)}`;
      if (context === 'create') {
        return problem('rate_limited', "You've used this hour's plans", `Try again ${after}.`, true, wait);
      }
      if (context === 'search') {
        const providers = serverCode(error) === 'PROVIDER_RATE_LIMITED';
        return problem(
          'rate_limited',
          providers ? 'The photo libraries are busy' : 'Searching too fast',
          `Search again ${after}. You can keep editing the narration meanwhile.`,
          true,
          wait,
        );
      }
      return problem('rate_limited', 'Too many requests', `Try again ${after}.`, true, wait);
    }

    case 'disabled':
      if (context === 'search') {
        return problem(
          'disabled',
          "Picture search isn't switched on yet",
          "You can still edit the narration. Pictures can't be changed until search is available.",
          false,
        );
      }
      if (context === 'save') {
        return problem(
          'disabled',
          "Choosing a new picture isn't available yet",
          'Narration edits and the pictures already chosen can still be saved.',
          false,
        );
      }
      return problem(
        'disabled',
        "Story planning isn't switched on yet",
        "Nothing is wrong with your story. This feature isn't available on this site yet.",
        false,
      );

    case 'unavailable':
      if (context === 'save') {
        return problem(
          'unavailable',
          "The photo library didn't confirm that picture",
          'Nothing was saved. Try again in a moment, or keep the current picture.',
          true,
        );
      }
      if (context === 'create') {
        return problem('unavailable', 'The planner is busy', 'Wait a minute, then try again. Your story is still here.', true);
      }
      return problem('unavailable', 'Temporarily unavailable', 'Try again in a minute.', true);

    case 'timeout':
      if (context === 'create') {
        return problem(
          'timeout',
          'The planner took too long',
          'Your story is still here. Try again, or shorten it a little.',
          true,
        );
      }
      if (context === 'search') {
        return problem('timeout', 'The photo libraries took too long', 'Try the search again.', true);
      }
      return problem('timeout', 'That took too long', 'Try again.', true);

    case 'stale':
      return problem(
        'stale',
        'This plan changed somewhere else',
        'Your changes were not saved. Reload the plan to see the newest version, then make them again.',
        false,
      );

    case 'not_ready':
      return problem('not_ready', 'This plan has nothing to edit', "It didn't finish. Start a new plan.", false);

    case 'not_found':
      return problem(
        'not_found',
        context === 'load' ? "We can't find that plan" : 'This plan no longer exists',
        'It may have been removed, or it belongs to another account. Start a new plan.',
        false,
      );

    default:
      if (context === 'create' && error.status === 502) {
        return problem(
          'failed',
          "The planner didn't return a usable plan",
          'Try again. If it keeps failing, reword the story a little.',
          true,
        );
      }
      if (context === 'search') {
        return problem('failed', "Couldn't reach the photo libraries", 'Try again in a moment.', true);
      }
      if (context === 'save') {
        return problem('failed', "Your changes weren't saved", 'Something went wrong on our side. Try again.', true);
      }
      return problem('failed', 'Something went wrong on our side', 'Nothing was lost. Try again in a moment.', true);
  }
}

/** One sentence about providers that did not answer a search, for a notice above the results. */
export interface FailureNotice {
  text: string;
  /** The longest wait among the providers that said to wait. */
  retryAfterSeconds?: number;
}

/**
 * Says which provider did not answer and why, so "Pexels is busy" never reads
 * as "nothing matches". Null when every provider answered.
 */
export function describeProviderFailures(failures: StockProviderFailure[] | undefined): FailureNotice | null {
  if (!failures?.length) return null;
  let wait: number | undefined;
  const parts = failures.map((f) => {
    const name = PROVIDER_NAMES[f.provider] ?? f.provider;
    if (f.reason === 'rate_limited') {
      if (f.retryAfter && f.retryAfter > 0) wait = Math.max(wait ?? 0, f.retryAfter);
      return f.retryAfter && f.retryAfter > 0 ? `${name} is busy (try again in ${formatWait(f.retryAfter)})` : `${name} is busy`;
    }
    if (f.reason === 'auth') return `${name} is not available on this site`;
    return `${name} did not answer`;
  });
  const notice: FailureNotice = { text: `${parts.join('. ')}.` };
  if (wait !== undefined) notice.retryAfterSeconds = wait;
  return notice;
}

/**
 * A search that found nothing because every provider failed. This is a
 * failure, not "no results", and the page must not say "nothing matches".
 */
export function problemFromFailures(failures: StockProviderFailure[]): StoryPlanProblem {
  const waits = failures.filter((f) => f.reason === 'rate_limited');
  if (waits.length === failures.length) {
    const wait = Math.max(0, ...waits.map((f) => f.retryAfter ?? 0));
    return problem(
      'rate_limited',
      'The photo libraries are busy',
      `Search again ${wait > 0 ? `in ${formatWait(wait)}` : 'in a little while'}. You can keep editing the narration meanwhile.`,
      true,
      wait > 0 ? wait : undefined,
    );
  }
  return problem('failed', "Couldn't reach the photo libraries", 'Try the search again in a moment.', true);
}
