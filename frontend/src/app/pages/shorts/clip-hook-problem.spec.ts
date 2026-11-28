import { HttpErrorResponse, HttpHeaders } from '@angular/common/http';
import { describeHookProblem } from './clip-hook-problem';

function failure(status: number, code = '', message = '', details?: unknown): HttpErrorResponse {
  return new HttpErrorResponse({ status, headers: new HttpHeaders(), error: status === 0 ? new ProgressEvent('error') : { code, message, ...(details ? { details } : {}) } });
}

describe('describeHookProblem', () => {
  it('says plainly when today\'s renders are used up, and that the clip stays', () => {
    const p = describeHookProblem(failure(429, 'QUOTA_EXCEEDED', 'x', { retryAfter: 3600 }), 'render');
    expect(p.kind).toBe('rate_limited');
    expect(p.title).toBe("You've used today's renders");
    expect(p.message).toContain('in 1 hour');
    expect(p.message).toContain('Retrying a failed video uses the same allowance');
    expect(p.message).toContain('stays as it is');
    expect(p.retryable).toBeFalse();
    expect(p.retryAfterSeconds).toBe(3600);
  });

  it('says suggestions share the writing allowance', () => {
    const p = describeHookProblem(failure(429, 'QUOTA_EXCEEDED'), 'suggest');
    expect(p.title).toBe("You've used today's suggestions");
    expect(p.message).toContain('tomorrow');
    expect(p.message).toContain('titles and post copy');
  });

  it('uses the API words for a refused hook', () => {
    const p = describeHookProblem(failure(400, 'VALIDATION_ERROR', 'This hook is too long to fit on two lines. Shorten it.'), 'save');
    expect(p.message).toBe('This hook is too long to fit on two lines. Shorten it.');
    expect(p.retryable).toBeFalse();
  });

  it('explains conflicts', () => {
    expect(describeHookProblem(failure(409, 'RENDER_IN_PROGRESS'), 'render').title).toBe('Already rendering');
    expect(describeHookProblem(failure(409, 'NOT_READY', 'Finish first.'), 'render').message).toBe('Finish first.');
    expect(describeHookProblem(failure(409, 'CONFLICT'), 'save').message).toContain('Reload');
  });

  it('says a model or start failure used nothing', () => {
    expect(describeHookProblem(failure(502, 'COPY_FAILED'), 'suggest').message).toContain("didn't use up");
    expect(describeHookProblem(failure(503, 'PIPELINE_ERROR'), 'render').message).toContain("didn't use up");
    expect(describeHookProblem(failure(503, 'PIPELINE_ERROR'), 'render').retryable).toBeTrue();
    expect(describeHookProblem(failure(503, 'UNAVAILABLE'), 'suggest').kind).toBe('disabled');
  });

  it('handles offline and unknown errors', () => {
    expect(describeHookProblem(failure(0), 'save').kind).toBe('offline');
    expect(describeHookProblem(new Error('x'), 'load').retryable).toBeTrue();
    expect(describeHookProblem(failure(404), 'load').kind).toBe('not_found');
  });

  it('never uses an em dash', () => {
    for (const s of [0, 400, 404, 409, 429, 502, 503, 504, 500]) {
      for (const a of ['load', 'save', 'suggest', 'render'] as const) {
        const p = describeHookProblem(failure(s), a);
        expect(p.title + p.message).not.toContain('\u2014');
      }
    }
  });
});
