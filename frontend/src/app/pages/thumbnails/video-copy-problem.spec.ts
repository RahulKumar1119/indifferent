import { HttpErrorResponse, HttpHeaders } from '@angular/common/http';
import { describeCopyProblem } from './video-copy-problem';

function failure(status: number, code = '', message = '', details?: unknown, headers?: Record<string, string>) {
  return new HttpErrorResponse({
    status,
    error: status === 0 ? new ProgressEvent('error') : { code, message, ...(details ? { details } : {}) },
    headers: new HttpHeaders(headers),
  });
}

describe('describeCopyProblem', () => {
  const table: [string, HttpErrorResponse, string, boolean][] = [
    ['offline', failure(0), 'offline', true],
    ['validation', failure(400, 'VALIDATION_ERROR', 'give a topic or a transcript of at least 15 characters'), 'invalid', false],
    ['transcript not ready', failure(400, 'NOT_READY', 'This video has no transcript yet. Paste one.'), 'not_ready', false],
    ['email not confirmed', failure(403, 'EMAIL_NOT_VERIFIED', 'x'), 'disabled', false],
    ['video missing', failure(404, 'NOT_FOUND'), 'not_found', false],
    ['timeout 504', failure(504, 'COPY_TIMEOUT'), 'timeout', true],
    ['timeout 408', failure(408), 'timeout', true],
    ['over the daily limit', failure(429, 'QUOTA_EXCEEDED', '', { retryAfter: 7200 }), 'rate_limited', false],
    ['switched off', failure(503, 'UNAVAILABLE'), 'disabled', false],
    ['busy', failure(503, 'COPY_UNAVAILABLE'), 'unavailable', true],
    ['unusable answer', failure(502, 'COPY_FAILED'), 'failed', true],
    ['anything else', failure(500, 'INTERNAL_ERROR'), 'failed', true],
    ['an odd 403', failure(403, 'FORBIDDEN'), 'failed', true],
  ];
  for (const [name, err, kind, retryable] of table) {
    it(`${name}: ${kind}`, () => {
      const p = describeCopyProblem(err);
      expect(p.kind).toBe(kind as never);
      expect(p.retryable).toBe(retryable);
      expect(p.title).not.toBe('');
      expect(p.message).not.toBe('');
    });
  }

  it('shows the API text only for a 400, where it names the problem', () => {
    expect(describeCopyProblem(failure(400, 'VALIDATION_ERROR', 'the topic is too long: keep it under 300 characters')).message).toContain('under 300');
    for (const status of [429, 502, 503, 504, 500]) {
      expect(describeCopyProblem(failure(status, 'X', 'internal request id abc-123 at 10.0.0.1')).message).not.toContain('abc-123');
    }
  });

  it('says how long to wait when the API said so, and does not invent a time when it did not', () => {
    expect(describeCopyProblem(failure(429, 'QUOTA_EXCEEDED', '', { retryAfter: 7200 })).message).toContain('in 2 hours');
    const p = describeCopyProblem(failure(429, 'QUOTA_EXCEEDED'));
    expect(p.message).toContain('tomorrow');
    expect(p.retryAfterSeconds).toBeUndefined();
  });

  it('tells the person a failure did not use up a try, where that is true', () => {
    for (const status of [502, 503, 504]) {
      const p = describeCopyProblem(failure(status, status === 503 ? 'COPY_UNAVAILABLE' : ''));
      expect(p.message).toContain("didn't use up");
    }
  });

  it('copes with something that is not an HTTP error', () => {
    expect(describeCopyProblem(new Error('x')).kind).toBe('failed');
    expect(describeCopyProblem(null).kind).toBe('failed');
  });

  it('uses no em dashes or emoji in any message', () => {
    for (const [, err] of table) {
      const p = describeCopyProblem(err);
      expect(p.title + p.message).not.toMatch(/\u2014|\p{Extended_Pictographic}/u);
    }
  });
});
