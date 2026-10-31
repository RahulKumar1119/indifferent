import { describeVideoProblem } from './story-video-problem';
import { httpError } from './story-plan-fixtures.spec';

const NO_EM_DASH = '\u2014';

describe('describeVideoProblem', () => {
  it('names the scenes with no picture, in the API\'s own words', () => {
    const p = describeVideoProblem(httpError(400, 'MISSING_PICTURES', 'Scenes 2, 3 have no picture. Choose one for each and try again.'), 'make');
    expect(p.title).toBe('Some scenes have no picture');
    expect(p.message).toContain('Scenes 2, 3');
    expect(p.retryable).toBeFalse();
  });

  it('says the story cannot be narrated, and why', () => {
    const p = describeVideoProblem(httpError(400, 'UNSUPPORTED_LANGUAGE', 'Narration is available in English and Hindi. This story is in another language.'), 'make');
    expect(p.title).toBe("This story can't be narrated");
    expect(p.message).toContain('English and Hindi');
  });

  it('passes on what is wrong with a voice or caption choice', () => {
    const p = describeVideoProblem(httpError(400, 'VALIDATION_ERROR', 'the voice "Joanna" cannot narrate a story in "hi": choose Aditi'), 'make');
    expect(p.title).toBe("Those settings can't be used");
    expect(p.message).toContain('choose Aditi');
  });

  it('asks to reload when the plan changed, and promises nothing was started', () => {
    const p = describeVideoProblem(httpError(409, 'STALE_REVISION'), 'make');
    expect(p.kind).toBe('stale');
    expect(p.message).toContain('No video was started');
    expect(p.retryable).withContext('trying the same thing again changes nothing; the plan has to be reloaded').toBeFalse();
  });

  it('carries the id of the video that is already being made', () => {
    const p = describeVideoProblem(httpError(409, 'ALREADY_RUNNING', '', { jobId: 'job-7' }), 'make');
    expect(p.kind).toBe('already_running');
    expect(p.jobId).toBe('job-7');
  });

  it('ignores an id that is not text', () => {
    const p = describeVideoProblem(httpError(409, 'ALREADY_RUNNING', '', { jobId: 7 }), 'make');
    expect(p.jobId).toBeUndefined();
  });

  it('says the limit of videos at once, and that trying again can work', () => {
    const p = describeVideoProblem(httpError(409, 'TOO_MANY_ACTIVE'), 'make');
    expect(p.kind).toBe('too_many_active');
    expect(p.retryable).toBeTrue();
  });

  it('says to cancel before deleting', () => {
    const p = describeVideoProblem(httpError(409, 'STILL_RUNNING'), 'delete');
    expect(p.kind).toBe('still_running');
    expect(p.message).toContain('Cancel it first');
  });

  it('tells a download that came too early from a plan that never finished', () => {
    expect(describeVideoProblem(httpError(409, 'NOT_READY'), 'download').retryable).toBeTrue();
    expect(describeVideoProblem(httpError(409, 'NOT_READY'), 'make').retryable).toBeFalse();
  });

  it('gives the day limit and when to come back, and does not repeat the API\'s sentence', () => {
    const p = describeVideoProblem(httpError(429, 'RATE_LIMITED', 'You can make 10 videos a day. Try again later.', { retryAfter: 3600 }), 'make');
    expect(p.kind).toBe('rate_limited');
    expect(p.retryAfterSeconds).toBe(3600);
    expect(p.message).toContain('Try again in 1 hour');
    expect(p.message).not.toContain('Try again later');
    expect(p.message).toContain('cancelled or did not finish');
  });

  it('copes with a limit that says no time', () => {
    expect(describeVideoProblem(httpError(429, 'RATE_LIMITED'), 'make').message).toContain('in a little while');
  });

  it('says plainly when making videos is switched off', () => {
    const p = describeVideoProblem(httpError(503, 'UNAVAILABLE'), 'make');
    expect(p.kind).toBe('disabled');
    expect(p.title).toBe("Making videos isn't switched on yet");
  });

  it('distinguishes a missing plan from a missing video', () => {
    expect(describeVideoProblem(httpError(404, 'NOT_FOUND'), 'make').title).toBe('This plan no longer exists');
    expect(describeVideoProblem(httpError(404, 'NOT_FOUND'), 'cancel').title).toBe("We can't find that video");
  });

  it('says offline, and anything unexpected, without alarming words', () => {
    expect(describeVideoProblem(httpError(0), 'make').kind).toBe('offline');
    expect(describeVideoProblem(new Error('x'), 'make').title).toBe('Something went wrong');
  });

  it('says a failed cleanup left the video in the list', () => {
    const p = describeVideoProblem(httpError(500, 'S3_CLEANUP_FAILED'), 'delete');
    expect(p.message).toContain('still in your list');
  });

  it('words an unexpected failure by what the person was doing', () => {
    const titles = (['make', 'check', 'cancel', 'download', 'delete', 'load'] as const).map((c) => describeVideoProblem(httpError(500, 'DB_ERROR'), c).title);
    expect(new Set(titles).size).toBe(titles.length);
    for (const t of titles) expect(t.length).toBeGreaterThan(5);
  });

  it('never shows the server\'s text for an error it does not know', () => {
    const p = describeVideoProblem(httpError(500, 'DB_ERROR', 'dynamodb: throttled at table projects'), 'make');
    expect(JSON.stringify(p)).not.toContain('dynamodb');
  });

  it('uses no em dash', () => {
    const statuses: [number, string][] = [[400, 'MISSING_PICTURES'], [404, ''], [409, 'STALE_REVISION'], [409, 'TOO_MANY_ACTIVE'], [429, 'RATE_LIMITED'], [503, 'UNAVAILABLE'], [504, ''], [500, '']];
    for (const [status, code] of statuses) {
      for (const ctx of ['make', 'check', 'load', 'cancel', 'download', 'delete'] as const) {
        expect(JSON.stringify(describeVideoProblem(httpError(status, code), ctx))).not.toContain(NO_EM_DASH);
      }
    }
  });
});
