import { HttpErrorResponse, HttpHeaders } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { Subject, of, throwError } from 'rxjs';
import { StoryPlanService } from './story-plan.service';
import { VoiceTake } from './story-plan.models';
import { PLAYBACK_FACTORY, PlaybackElement, StoryVoicePreview, VoiceRequest, describeVoiceProblem } from './story-voice-preview';

/** An audio element that does nothing until the test says so. */
class FakeAudio implements PlaybackElement {
  src = '';
  paused = false;
  playResult: Promise<void> = Promise.resolve();
  private listeners = new Map<string, Set<() => void>>();
  play(): Promise<void> {
    return this.playResult;
  }
  pause(): void {
    this.paused = true;
  }
  addEventListener(type: string, fn: () => void): void {
    if (!this.listeners.has(type)) this.listeners.set(type, new Set());
    this.listeners.get(type)!.add(fn);
  }
  removeEventListener(type: string, fn: () => void): void {
    this.listeners.get(type)?.delete(fn);
  }
  emit(type: 'playing' | 'ended' | 'error'): void {
    this.listeners.get(type)?.forEach((f) => f());
  }
  listening(): number {
    return [...this.listeners.values()].reduce((n, s) => n + s.size, 0);
  }
}

const take = (over: Partial<VoiceTake> = {}): VoiceTake => ({
  url: 'https://bucket.example/story-preview/u/abc.mp3?X-Amz-Signature=s',
  expiresInSeconds: 300,
  cached: false,
  remainingToday: 59,
  dailyLimit: 60,
  ...over,
});

const err = (status: number, code = '', extra: object = {}, headers: Record<string, string> = {}) =>
  new HttpErrorResponse({ status, error: { code, message: 'server words', ...extra }, headers: new HttpHeaders(headers) });

const req = (over: Partial<VoiceRequest> = {}): VoiceRequest => ({
  planId: 'plan-1', revision: 3, scene: 0, text: '  The lighthouse stood alone.  ', voice: '', ...over,
});

describe('StoryVoicePreview', () => {
  let svc: StoryVoicePreview;
  let api: jasmine.SpyObj<StoryPlanService>;
  let audios: FakeAudio[];

  beforeEach(() => {
    audios = [];
    api = jasmine.createSpyObj<StoryPlanService>('StoryPlanService', ['previewVoice']);
    api.previewVoice.and.returnValue(of(take()));
    TestBed.configureTestingModule({
      providers: [
        StoryVoicePreview,
        { provide: StoryPlanService, useValue: api },
        { provide: PLAYBACK_FACTORY, useValue: () => { const a = new FakeAudio(); audios.push(a); return a; } },
      ],
    });
    svc = TestBed.inject(StoryVoicePreview);
  });
  const kind = (i = 0) => svc.view(i).kind;

  it('asks for the scene (1-based) with the text as it is now, trimmed, and no voice when none is chosen', () => {
    svc.toggle(req());
    expect(api.previewVoice).toHaveBeenCalledOnceWith('plan-1', { revision: 3, scene: 1, narration: 'The lighthouse stood alone.' });
  });

  it('sends the chosen voice', () => {
    svc.toggle(req({ voice: 'Matthew', scene: 2 }));
    expect(api.previewVoice.calls.mostRecent().args[1]).toEqual({ revision: 3, scene: 3, narration: 'The lighthouse stood alone.', voice: 'Matthew' });
  });

  it('goes from making the voice, to playing, to idle when it ends', () => {
    const wait = new Subject<VoiceTake>();
    api.previewVoice.and.returnValue(wait);
    svc.toggle(req());
    expect(kind()).toBe('loading');
    wait.next(take());
    expect(audios.length).toBe(1);
    expect(audios[0].src).toContain('story-preview');
    audios[0].emit('playing');
    expect(kind()).toBe('playing');
    audios[0].emit('ended');
    expect(kind()).toBe('idle');
    expect(audios[0].listening()).toBe(0);
  });

  it('only the scene that is playing shows it', () => {
    svc.toggle(req({ scene: 1 }));
    audios[0].emit('playing');
    expect(kind(1)).toBe('playing');
    expect(kind(0)).toBe('idle');
    expect(kind(2)).toBe('idle');
  });

  it('pressing the button while it plays, or is being made, stops it', () => {
    svc.toggle(req());
    audios[0].emit('playing');
    svc.toggle(req());
    expect(kind()).toBe('idle');
    expect(audios[0].paused).toBeTrue();
    const wait = new Subject<VoiceTake>();
    api.previewVoice.and.returnValue(wait);
    svc.toggle(req());
    expect(kind()).toBe('loading');
    svc.toggle(req());
    expect(kind()).toBe('idle');
    expect(wait.observed).toBeFalse();
  });

  it('starting another scene stops the first: only one voice at a time', () => {
    svc.toggle(req({ scene: 0, text: 'one' }));
    audios[0].emit('playing');
    svc.toggle(req({ scene: 1, text: 'two' }));
    expect(audios[0].paused).toBeTrue();
    expect(kind(0)).toBe('idle');
    expect(kind(1)).toBe('loading');
  });

  it('an answer that arrives after a stop is ignored', () => {
    const wait = new Subject<VoiceTake>();
    api.previewVoice.and.returnValue(wait);
    svc.toggle(req());
    svc.stop();
    wait.next(take());
    expect(audios.length).toBe(0);
    expect(kind()).toBe('idle');
  });

  it('plays the same text again from the kept link, with no new request', () => {
    svc.toggle(req());
    audios[0].emit('ended');
    svc.toggle(req());
    expect(api.previewVoice).toHaveBeenCalledTimes(1);
    expect(audios.length).toBe(2);
  });

  it('asks again for other text, another voice, or once the link is about to expire', () => {
    svc.toggle(req());
    audios[0].emit('ended');
    svc.toggle(req({ text: 'other words' }));
    audios[1].emit('ended');
    svc.toggle(req({ voice: 'Brian' }));
    expect(api.previewVoice).toHaveBeenCalledTimes(3);
    jasmine.clock().install();
    try {
      jasmine.clock().mockDate(new Date(2026, 10, 16, 10, 0, 0));
      api.previewVoice.calls.reset();
      svc.stop();
      svc.toggle(req({ text: 'timed', scene: 1 }));
      audios[audios.length - 1].emit('ended');
      jasmine.clock().mockDate(new Date(2026, 10, 16, 10, 4, 45)); // 285 s on: inside the 30 s margin of a 300 s link
      svc.toggle(req({ text: 'timed', scene: 1 }));
      expect(api.previewVoice).toHaveBeenCalledTimes(2);
    } finally {
      jasmine.clock().uninstall();
    }
  });

  it('a link that stops working is replaced once, then it gives up with a plain message', () => {
    svc.toggle(req());
    audios[0].emit('error');
    expect(api.previewVoice).toHaveBeenCalledTimes(2); // asked again
    audios[audios.length - 1].emit('error');
    expect(kind()).toBe('problem');
    expect((svc.view(0) as { message: string }).message).toContain('could not be played');
    expect(api.previewVoice).toHaveBeenCalledTimes(2); // and no loop
  });

  it('a browser that blocks the sound says so; a stop we caused is not an error', async () => {
    const blocked = new FakeAudio();
    blocked.playResult = Promise.reject({ name: 'NotAllowedError' });
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [StoryVoicePreview, { provide: StoryPlanService, useValue: api }, { provide: PLAYBACK_FACTORY, useValue: () => blocked }],
    });
    const s = TestBed.inject(StoryVoicePreview);
    s.toggle(req());
    await Promise.resolve();
    await Promise.resolve();
    expect((s.view(0) as { message: string }).message).toContain('blocked the sound');
    const aborted = new FakeAudio();
    aborted.playResult = Promise.reject({ name: 'AbortError' });
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [StoryVoicePreview, { provide: StoryPlanService, useValue: api }, { provide: PLAYBACK_FACTORY, useValue: () => aborted }],
    });
    const s2 = TestBed.inject(StoryVoicePreview);
    s2.toggle(req());
    await Promise.resolve();
    await Promise.resolve();
    expect(s2.view(0).kind).not.toBe('problem');
  });

  it('empty text makes no request', () => {
    svc.toggle(req({ text: '   ' }));
    expect(api.previewVoice).not.toHaveBeenCalled();
    expect(kind()).toBe('idle');
  });

  it('learns how many new takes are left, and keeps the last figure when a replay does not say', () => {
    svc.toggle(req());
    expect(svc.remaining()).toBe(59);
    expect(svc.dailyLimit()).toBe(60);
    audios[0].emit('ended');
    api.previewVoice.and.returnValue(of(take({ cached: true, remainingToday: undefined })));
    svc.toggle(req({ text: 'a take we already had' }));
    expect(svc.remaining()).toBe(59);
  });

  it('stops the sound when the page goes', () => {
    svc.toggle(req());
    audios[0].emit('playing');
    TestBed.resetTestingModule();
    expect(audios[0].paused).toBeTrue();
  });

  describe('when the voice cannot be had', () => {
    let n = 0;
    // Every call uses new words, so a kept link from an earlier call is never reused.
    const say = (e: unknown) => {
      api.previewVoice.and.returnValue(throwError(() => e));
      svc.toggle(req({ text: `words number ${++n}` }));
      return svc.view(0) as { kind: string; message: string; limit: boolean };
    };
    it('names a used-up day, and says replays still work', () => {
      svc.toggle(req());
      audios[0].emit('ended');
      const v = say(err(429, 'QUOTA_EXCEEDED', {}, {}));
      expect(v.kind).toBe('problem');
      expect(v.limit).toBeTrue();
      expect(v.message).toContain("today's 60 new voice previews");
      expect(v.message).toContain('already heard still play');
    });
    it('says the voice is busy, for as long as the server said', () => {
      const v = say(err(429, 'VOICE_BUSY', { details: { retryAfter: 10 } }));
      expect(v.message).toBe('The voice is busy. Try again in 10 seconds.');
      expect(v.limit).toBeFalse();
    });
    it('asks for a reload when the plan changed elsewhere', () => {
      expect(say(err(409, 'STALE_REVISION')).message).toContain('Reload the page');
    });
    it('shows the reason for a refused text or an unconfirmed account', () => {
      expect(say(err(400, 'VALIDATION_ERROR', { message: 'Scene 1: the narration is empty' })).message).toBe('Scene 1: the narration is empty');
      expect(say(err(403, 'EMAIL_NOT_VERIFIED', { message: 'Confirm your email address first.' })).message).toBe('Confirm your email address first.');
    });
    it('is plain about being offline and about a failing voice, and leaks nothing', () => {
      expect(say(new HttpErrorResponse({ status: 0 })).message).toContain('offline');
      const v = say(err(502, 'VOICE_FAILED', { message: 'ServiceFailureException: secret detail' }));
      expect(v.message).toBe('The voice could not be made. Try again in a few minutes.');
    });
    it('hides the buttons for good when previews are switched off', () => {
      const v = say(err(503, 'UNAVAILABLE'));
      expect(svc.available()).toBeFalse();
      expect(v.kind).toBe('idle');
    });
    it('can try again after a problem', () => {
      say(err(502, 'VOICE_FAILED'));
      api.previewVoice.and.returnValue(of(take()));
      svc.toggle(req({ text: `words number ${n}` }));
      expect(kind()).not.toBe('problem');
    });
  });

  it('describeVoiceProblem gives the generic limit text without a known figure', () => {
    expect(describeVoiceProblem(err(429, 'QUOTA_EXCEEDED'), null).message).toContain("today's new voice previews");
  });
});
