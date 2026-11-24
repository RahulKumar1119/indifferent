import { HttpErrorResponse } from '@angular/common/http';
import { describePostCopyProblem } from './post-copy-problem';

function failure(status: number, code = '', message = '', details?: unknown): HttpErrorResponse {
  return new HttpErrorResponse({ status, error: status === 0 ? new ProgressEvent('error') : { code, message, ...(details ? { details } : {}) } });
}

describe('describePostCopyProblem', () => {
  it('says the person is offline and that nothing typed is lost', () => {
    const p = describePostCopyProblem(failure(0), 'write');
    expect(p.kind).toBe('offline');
    expect(p.message).toContain('still here');
    expect(p.retryable).toBeTrue();
  });
  it('uses the API words for a 400 that names the problem', () => {
    const p = describePostCopyProblem(failure(400, 'VALIDATION_ERROR', 'YouTube description with its hashtags is 5,230 bytes; the limit is 5,000 bytes.'), 'save');
    expect(p.kind).toBe('invalid');
    expect(p.title).toBe("That can't be saved yet");
    expect(p.message).toContain('5,230 bytes');
    expect(p.retryable).toBeFalse();
  });
  it('explains a clip with nothing to write from', () => {
    const p = describePostCopyProblem(failure(400, 'NOT_READY', 'This clip has no spoken words, so there is nothing to write from.'), 'write');
    expect(p.kind).toBe('not_ready');
    expect(p.message).toContain('no spoken words');
  });
  it('says a rate limit lasts until later, and what stays', () => {
    const p = describePostCopyProblem(failure(429, 'QUOTA_EXCEEDED', 'x', { retryAfter: 7200 }), 'write');
    expect(p.kind).toBe('rate_limited');
    expect(p.message).toContain('2 hours');
    expect(p.message).toContain('stays on this page');
  });
  it('says a failed write did not use a try', () => {
    for (const status of [502, 503, 504]) {
      const p = describePostCopyProblem(failure(status, status === 503 ? 'COPY_UNAVAILABLE' : ''), 'write');
      expect(p.message).toContain("didn't use up one of today's tries");
      expect(p.retryable).toBeTrue();
    }
  });
  it('does not claim that for a failed save', () => {
    expect(describePostCopyProblem(failure(503, 'COPY_UNAVAILABLE'), 'save').message).not.toContain('tries');
  });
  it('says when the feature is not switched on', () => {
    const p = describePostCopyProblem(failure(503, 'UNAVAILABLE'), 'write');
    expect(p.kind).toBe('disabled');
    expect(p.retryable).toBeFalse();
  });
  it('tells a first try that is still being written to wait', () => {
    const p = describePostCopyProblem(failure(409, 'IN_PROGRESS'), 'write');
    expect(p.title).toBe('Still writing');
    expect(p.retryable).toBeTrue();
  });
  it('says a story video is not finished', () => {
    const p = describePostCopyProblem(failure(409, 'NOT_READY', 'Write the post copy once the video is finished.'), 'load');
    expect(p.kind).toBe('not_ready');
  });
  it('does not say whose a missing clip is', () => {
    expect(describePostCopyProblem(failure(404), 'load').message).toContain('belongs to another account');
  });
  it('asks for a confirmed email', () => {
    expect(describePostCopyProblem(failure(403, 'EMAIL_NOT_VERIFIED'), 'write').title).toContain('Confirm your email');
  });
  it('has a fallback, and uses no em dash anywhere', () => {
    expect(describePostCopyProblem(new Error('x'), 'write').kind).toBe('failed');
    for (const status of [0, 400, 403, 404, 408, 409, 429, 500, 502, 503, 504]) {
      for (const action of ['load', 'write', 'save'] as const) {
        const p = describePostCopyProblem(failure(status, status === 403 ? 'EMAIL_NOT_VERIFIED' : 'X', 'Plain'), action);
        expect(p.title + p.message).not.toContain('\u2014');
      }
    }
  });
});
