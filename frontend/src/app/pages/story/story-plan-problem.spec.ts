import { HttpErrorResponse, HttpHeaders } from '@angular/common/http';
import { describeProblem, formatWait, ProblemContext, ProblemKind, retryAfterSeconds } from './story-plan-problem';

function failure(status: number, code = '', message = '', details?: unknown, headers?: Record<string, string>) {
  return new HttpErrorResponse({
    status,
    error: status === 0 ? new ProgressEvent('error') : { code, message, ...(details ? { details } : {}) },
    headers: new HttpHeaders(headers),
  });
}

describe('formatWait', () => {
  it('reads naturally and rounds up', () => {
    expect(formatWait(0)).toBe('1 second');
    expect(formatWait(1)).toBe('1 second');
    expect(formatWait(45)).toBe('45 seconds');
    expect(formatWait(60)).toBe('1 minute');
    expect(formatWait(61)).toBe('2 minutes');
    expect(formatWait(29 * 60)).toBe('29 minutes');
    expect(formatWait(3600)).toBe('1 hour');
    expect(formatWait(3600 + 60)).toBe('1 hour 1 minute');
    expect(formatWait(2 * 3600 + 5 * 60)).toBe('2 hours 5 minutes');
  });
});

describe('retryAfterSeconds', () => {
  it('prefers the body, which a browser on another origin can read', () => {
    expect(retryAfterSeconds(failure(429, 'RATE_LIMITED', '', { retryAfter: 90 }, { 'Retry-After': '5' }))).toBe(90);
  });

  it('falls back to the header', () => {
    expect(retryAfterSeconds(failure(429, 'RATE_LIMITED', '', undefined, { 'Retry-After': '30' }))).toBe(30);
  });

  it('ignores values that are not a positive number', () => {
    expect(retryAfterSeconds(failure(429, 'RATE_LIMITED', '', { retryAfter: 'soon' }))).toBeUndefined();
    expect(retryAfterSeconds(failure(429, 'RATE_LIMITED', '', { retryAfter: -4 }))).toBeUndefined();
    expect(retryAfterSeconds(failure(429, 'RATE_LIMITED', '', undefined, { 'Retry-After': 'Wed, 21 Oct' }))).toBeUndefined();
    expect(retryAfterSeconds(failure(429, 'RATE_LIMITED'))).toBeUndefined();
  });
});

describe('describeProblem', () => {
  // Every status and code the backend can answer on these routes.
  const cases: {
    name: string;
    err: HttpErrorResponse;
    context: ProblemContext;
    kind: ProblemKind;
    retryable: boolean;
  }[] = [
    { name: 'no connection', err: failure(0), context: 'create', kind: 'offline', retryable: true },
    { name: 'bad story', err: failure(400, 'VALIDATION_ERROR', 'The story is too short.'), context: 'create', kind: 'invalid', retryable: false },
    { name: 'bad edit', err: failure(400, 'VALIDATION_ERROR', 'Scene 2: the narration is empty'), context: 'save', kind: 'invalid', retryable: false },
    { name: 'Hindi-only search', err: failure(400, 'VALIDATION_ERROR', 'Search in English: try "mandir".'), context: 'search', kind: 'invalid', retryable: false },
    { name: 'plan limit', err: failure(429, 'RATE_LIMITED', '', { retryAfter: 1740 }), context: 'create', kind: 'rate_limited', retryable: true },
    { name: 'search allowance', err: failure(429, 'RATE_LIMITED', '', { retryAfter: 20 }), context: 'search', kind: 'rate_limited', retryable: true },
    { name: 'provider limit', err: failure(429, 'PROVIDER_RATE_LIMITED', '', { retryAfter: 60 }), context: 'search', kind: 'rate_limited', retryable: true },
    { name: 'planner off', err: failure(503, 'UNAVAILABLE', 'Story planning is not available yet'), context: 'create', kind: 'disabled', retryable: false },
    { name: 'search off', err: failure(503, 'UNAVAILABLE', 'Stock media search is not available yet'), context: 'search', kind: 'disabled', retryable: false },
    { name: 'new pick while search is off', err: failure(503, 'UNAVAILABLE', 'Stock media is not available yet'), context: 'save', kind: 'disabled', retryable: false },
    { name: 'planner busy', err: failure(503, 'PLAN_UNAVAILABLE', 'The planner is busy'), context: 'create', kind: 'unavailable', retryable: true },
    { name: 'pick not confirmed', err: failure(503, 'PLAN_UNAVAILABLE', 'The photo library could not confirm that choice'), context: 'save', kind: 'unavailable', retryable: true },
    { name: 'planner slow', err: failure(504, 'PLAN_TIMEOUT'), context: 'create', kind: 'timeout', retryable: true },
    { name: 'search slow', err: failure(504, 'SEARCH_TIMEOUT'), context: 'search', kind: 'timeout', retryable: true },
    { name: 'search cancelled', err: failure(408, 'CANCELED'), context: 'search', kind: 'timeout', retryable: true },
    { name: 'unusable plan', err: failure(502, 'PLAN_FAILED'), context: 'create', kind: 'failed', retryable: true },
    { name: 'providers unreachable', err: failure(502, 'SEARCH_FAILED'), context: 'search', kind: 'failed', retryable: true },
    { name: 'database error', err: failure(500, 'DB_ERROR'), context: 'save', kind: 'failed', retryable: true },
    { name: 'plan changed elsewhere', err: failure(409, 'STALE_REVISION'), context: 'save', kind: 'stale', retryable: false },
    { name: 'plan never finished', err: failure(409, 'NOT_READY'), context: 'save', kind: 'not_ready', retryable: false },
    { name: 'unknown plan', err: failure(404, 'NOT_FOUND'), context: 'load', kind: 'not_found', retryable: false },
  ];

  for (const c of cases) {
    it(`${c.name} (${c.context}): ${c.kind}`, () => {
      const p = describeProblem(c.err, c.context);
      expect(p.kind).toBe(c.kind);
      expect(p.retryable).toBe(c.retryable);
      // The rule for every state: say what happened, and what to do next.
      expect(p.title.length).toBeGreaterThan(0);
      expect(p.message.length).toBeGreaterThan(0);
      expect(`${p.title} ${p.message}`).not.toMatch(/undefined|\[object|NaN|null/);
    });
  }

  it('shows the API text for a 400, because it names the exact problem', () => {
    const p = describeProblem(failure(400, 'VALIDATION_ERROR', 'Scene 2: the narration is empty'), 'save');
    expect(p.message).toBe('Scene 2: the narration is empty');
  });

  it('has a fallback for a 400 with no readable message', () => {
    const p = describeProblem(failure(400, 'VALIDATION_ERROR', ''), 'create');
    expect(p.message).toBe('Check what you entered and try again.');
  });

  it('says how long to wait on a 429, taken from the body', () => {
    const p = describeProblem(failure(429, 'RATE_LIMITED', '', { retryAfter: 1740 }), 'create');
    expect(p.message).toBe('Try again in 29 minutes.');
    expect(p.retryAfterSeconds).toBe(1740);
  });

  it('still reads well when a 429 has no wait time', () => {
    const p = describeProblem(failure(429, 'RATE_LIMITED'), 'create');
    expect(p.message).toBe('Try again in a little while.');
    expect(p.retryAfterSeconds).toBeUndefined();
  });

  it('tells a provider limit from the per-user search allowance', () => {
    const own = describeProblem(failure(429, 'RATE_LIMITED', '', { retryAfter: 20 }), 'search');
    const providers = describeProblem(failure(429, 'PROVIDER_RATE_LIMITED', '', { retryAfter: 60 }), 'search');
    expect(own.title).toBe('Searching too fast');
    expect(providers.title).toBe('The photo libraries are busy');
    expect(providers.message).toContain('1 minute');
  });

  it('tells the user a stale save lost nothing it could keep, and what to do', () => {
    const p = describeProblem(failure(409, 'STALE_REVISION'), 'save');
    expect(p.message).toContain('not saved');
    expect(p.message).toContain('Reload');
  });

  it('keeps the story in the box when the planner fails (the page must not clear it)', () => {
    for (const status of [503, 504]) {
      const p = describeProblem(failure(status, status === 503 ? 'PLAN_UNAVAILABLE' : 'PLAN_TIMEOUT'), 'create');
      expect(p.message).toContain('still here');
    }
  });

  it('survives an error that is not an HTTP response', () => {
    const p = describeProblem(new Error('boom'), 'create');
    expect(p.kind).toBe('failed');
    expect(p.retryable).toBeTrue();
  });

  it('survives an error body that is not JSON (a gateway page)', () => {
    const err = new HttpErrorResponse({ status: 504, error: '<html>Gateway Timeout</html>' });
    expect(describeProblem(err, 'create').kind).toBe('timeout');
    const bad = new HttpErrorResponse({ status: 400, error: null });
    expect(describeProblem(bad, 'search').message).toBe('Check what you entered and try again.');
  });
});
