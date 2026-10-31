import { formatDate } from '@angular/common';
import { HttpErrorResponse } from '@angular/common/http';
import { ComponentFixture, TestBed, fakeAsync, flush, tick } from '@angular/core/testing';
import { Observable, Subject, of, throwError } from 'rxjs';
import {
  STORY_ICONS,
  VIDEO_LIMITS,
  VideoServiceSpy,
  cancelledVideo,
  completed,
  failed,
  httpError,
  rendering,
  video,
  videoServiceSpy,
} from './story-plan-fixtures.spec';
import { StoryVideoPanelComponent } from './story-video-panel.component';
import { StoryVideo, StoryVideoLimits, StoryVideoList } from './story-video.models';
import { StoryVideoService } from './story-video.service';

interface Mount {
  videos?: StoryVideo[];
  list?: HttpErrorResponse;
  /** The list request itself, for a test that controls when it answers. */
  listSource?: Observable<StoryVideoList>;
  limits?: StoryVideoLimits | HttpErrorResponse;
  language?: string;
  blocker?: string;
  revision?: number;
  pollMs?: number;
}

describe('StoryVideoPanelComponent', () => {
  let svc: VideoServiceSpy;
  let fixture: ComponentFixture<StoryVideoPanelComponent>;
  let el: HTMLElement;
  let changed: number;
  let scenes: number[];

  beforeEach(async () => {
    svc = videoServiceSpy();
    changed = 0;
    scenes = [];
    await TestBed.configureTestingModule({
      imports: [StoryVideoPanelComponent],
      providers: [{ provide: StoryVideoService, useValue: svc }, STORY_ICONS],
    }).compileComponents();
  });

  afterEach(() => fixture?.destroy());

  /** Builds the panel without rendering it. */
  function build(o: Mount = {}): void {
    svc.listForPlan.and.returnValue(
      o.listSource ?? (o.list ? throwError(() => o.list) : of({ videos: o.videos ?? [], hasMore: false })),
    );
    if (o.limits instanceof HttpErrorResponse) svc.getLimits.and.returnValue(throwError(() => o.limits));
    else if (o.limits) svc.getLimits.and.returnValue(of(o.limits));
    fixture = TestBed.createComponent(StoryVideoPanelComponent);
    const set = (k: string, v: unknown) => fixture.componentRef.setInput(k, v);
    set('planId', 'plan-1');
    set('revision', o.revision ?? 1);
    set('language', o.language ?? 'en');
    set('blocker', o.blocker ?? '');
    set('pollMs', o.pollMs ?? 3000);
    fixture.componentInstance.planChanged.subscribe(() => changed++);
    fixture.componentInstance.goToScene.subscribe((n) => scenes.push(n));
    el = fixture.nativeElement;
  }

  /**
   * Renders and lets the effects and the focus change run. It does not use
   * whenStable(): a video being made has a poll timer pending, and the zone
   * counts that as work in progress, so waiting for stability would wait for it.
   */
  async function render(): Promise<void> {
    fixture.detectChanges();
    await Promise.resolve();
    fixture.detectChanges();
    await Promise.resolve();
    fixture.detectChanges();
  }

  async function mount(o: Mount = {}): Promise<void> {
    build(o);
    await render();
  }

  /** The same, inside fakeAsync, for tests that move the clock. */
  function mountFake(o: Mount = {}): void {
    build(o);
    pump();
  }

  function pump(ms = 0): void {
    fixture.detectChanges();
    tick(ms);
    fixture.detectChanges();
  }

  const text = () => el.textContent!.replace(/\s+/g, ' ');
  const q = <T extends HTMLElement = HTMLElement>(id: string) => el.querySelector<T>(`[data-testid="${id}"]`);
  const make = () => q<HTMLButtonElement>('make')!;
  const problem = () => q('problem');
  const heading = () => el.querySelector<HTMLElement>('[data-state-heading]');
  const percent = () => q('percent')?.textContent;

  // -------------------------------------------------------------------------

  describe('before any video', () => {
    beforeEach(() => mount());

    it('asks for the limits and for this plan\'s videos', () => {
      expect(svc.getLimits).toHaveBeenCalledTimes(1);
      expect(svc.listForPlan).toHaveBeenCalledOnceWith('plan-1');
    });

    it('offers the choices and the one button that makes the video', () => {
      expect(el.querySelector('h2')!.textContent).toBe('Make the video');
      expect(make().textContent!.trim()).toBe('Make the video');
      expect(make().disabled).toBeFalse();
      expect(el.querySelector('app-story-video-options')).toBeTruthy();
      expect(text()).toContain('It can take a few minutes. You can leave this page and come back.');
    });

    it('starts on the first voice for the language and the bold captions', () => {
      expect(el.querySelector<HTMLSelectElement>('select')!.value).toBe('Joanna');
      expect(el.querySelector<HTMLInputElement>('input[type="radio"]:checked')!.value).toBe('bold');
    });

    it('states the limits from the API, and that an attempt that does not finish still counts', () => {
      expect(q('limits-note')!.textContent!.replace(/\s+/g, ' ').trim()).toBe(
        'You can make 10 videos a day, 2 at a time. A video that is cancelled or does not finish still counts.',
      );
    });

    it('moves nothing and shows nothing about a video that does not exist', () => {
      expect(q('active')).toBeNull();
      expect(q('failed')).toBeNull();
      expect(q('done')).toBeNull();
      expect(q('earlier')).toBeNull();
      expect(problem()).toBeNull();
    });
  });

  describe('while it looks for videos', () => {
    it('says so', async () => {
      const list = new Subject<StoryVideoList>();
      build({ listSource: list });
      await render();
      expect(q('loading')!.textContent).toContain('Checking for videos from this plan');
      expect(make()).toBeNull();
      list.next({ videos: [], hasMore: false });
      await render();
      expect(q('loading')).toBeNull();
      expect(make()).toBeTruthy();
    });
  });

  describe('when it cannot', () => {
    it('says what went wrong, quietly, and still lets a video be made', async () => {
      await mount({ list: httpError(500, 'DB_ERROR') });
      const p = problem()!;
      expect(p.textContent).toContain("Your videos couldn't be loaded");
      expect(p.getAttribute('role')).toBe('status');
      expect(make()).toBeTruthy();
    });

    it('tries again when asked', async () => {
      await mount({ list: httpError(500, 'DB_ERROR') });
      svc.listForPlan.and.returnValue(of({ videos: [completed()], hasMore: false }));
      problem()!.querySelector('button')!.click();
      await render();
      expect(svc.listForPlan).toHaveBeenCalledTimes(2);
      expect(problem()).toBeNull();
      expect(q('done')).toBeTruthy();
    });
  });

  describe('limits', () => {
    it('are not required: without them the voices still come from the language', async () => {
      await mount({ limits: httpError(500, 'X') });
      expect(Array.from(el.querySelectorAll('select option')).map((o) => (o as HTMLOptionElement).value)).toEqual(['Joanna', 'Matthew', 'Amy', 'Brian', 'Aditi']);
      expect(q('limits-note')).toBeNull();
      expect(make().disabled).toBeFalse();
    });

    it('say when making videos is switched off, and offer nothing to press', async () => {
      await mount({ limits: { ...VIDEO_LIMITS, enabled: false } });
      expect(problem()!.textContent).toContain("Making videos isn't switched on yet");
      expect(make()).toBeNull();
      expect(el.querySelector('app-story-video-options')).toBeNull();
    });
  });

  describe('a plan that cannot be made into a video yet', () => {
    it('shows the reason and keeps the button off, tied to the reason for screen readers', async () => {
      await mount({ blocker: 'Scene 3 has no picture. Choose one, then save.' });
      expect(q('blocker')!.textContent).toBe('Scene 3 has no picture. Choose one, then save.');
      expect(make().disabled).toBeTrue();
      expect(make().getAttribute('aria-describedby')).toBe(q('blocker')!.id);
    });

    it('does nothing when pressed anyway', async () => {
      await mount({ blocker: 'Save your changes first.' });
      make().click();
      expect(svc.create).not.toHaveBeenCalled();
    });

    it('refuses even if the action is reached some other way than the button', async () => {
      // A disabled button never fires a click, so this is the guard behind it.
      await mount({ blocker: 'Save your changes first.' });
      (fixture.componentInstance as unknown as { make(): void }).make();
      expect(svc.create).not.toHaveBeenCalled();
      await mount({ language: 'fr', limits: { ...VIDEO_LIMITS, voices: { en: ['Joanna'], hi: ['Aditi'] } } });
      (fixture.componentInstance as unknown as { make(): void }).make();
      expect(svc.create).not.toHaveBeenCalled();
    });

    it('comes back to life when the reason goes', async () => {
      await mount({ blocker: 'Save your changes first.' });
      fixture.componentRef.setInput('blocker', '');
      await render();
      expect(make().disabled).toBeFalse();
      expect(q('blocker')).toBeNull();
      expect(make().getAttribute('aria-describedby')).toBeNull();
    });

    it('says narration is not available when no voice reads the language', async () => {
      await mount({ language: 'fr', limits: { ...VIDEO_LIMITS, voices: { en: ['Joanna'], hi: ['Aditi'] } } });
      expect(q('blocker')!.textContent).toBe('Narration is available in English and Hindi. This story is in another language.');
      expect(make().disabled).toBeTrue();
      expect(el.querySelector('app-story-video-options')).toBeNull();
    });

    it('does not offer a choice of voices for Hindi', async () => {
      await mount({ language: 'hi' });
      expect(el.querySelector('select')).toBeNull();
      expect(q('voice-fixed')!.textContent).toContain('Aditi');
    });
  });

  // -------------------------------------------------------------------------

  describe('making the video', () => {
    it('sends the plan, its revision, and the voice and captions that were chosen', async () => {
      svc.create.and.returnValue(of(video()));
      await mount({ revision: 4 });
      const select = el.querySelector<HTMLSelectElement>('select')!;
      select.value = 'Amy';
      select.dispatchEvent(new Event('change'));
      await render();
      el.querySelector<HTMLInputElement>('input[value="karaoke"]')!.click();
      await render();
      make().click();
      expect(svc.create).toHaveBeenCalledOnceWith({ planId: 'plan-1', revision: 4, voice: 'Amy', caption: { preset: 'karaoke' } });
    });

    it('is a single request however often the button is pressed', async () => {
      const pending = new Subject<StoryVideo>();
      svc.create.and.returnValue(pending);
      await mount();
      make().click();
      make().click();
      await render();
      expect(svc.create).toHaveBeenCalledTimes(1);
      expect(make().disabled).toBeTrue();
      expect(make().textContent).toContain('Starting…');
      expect(el.querySelector<HTMLSelectElement>('select')!.disabled).toBeTrue();
    });

    it('turns into the progress view and moves focus to it', async () => {
      svc.create.and.returnValue(of(video()));
      await mount();
      make().click();
      await render();
      expect(q('active')).toBeTruthy();
      expect(make()).toBeNull();
      expect(heading()!.textContent).toBe('Making your video');
      expect(document.activeElement).toBe(heading());
    });

    it('lets a person leave and come back: that is said', async () => {
      svc.create.and.returnValue(of(video()));
      await mount();
      make().click();
      await render();
      expect(text()).toContain('You can leave this page. The video keeps being made, and this page picks it up when you come back.');
    });
  });

  describe('when a video cannot be made', () => {
    const attempt = async (error: HttpErrorResponse, o: Mount = {}) => {
      svc.create.and.returnValue(throwError(() => error));
      await mount(o);
      make().click();
      await render();
    };

    it('keeps the choices and says what to fix, in the API\'s words about the scenes', async () => {
      await attempt(httpError(400, 'MISSING_PICTURES', 'Scenes 2, 3 have no picture. Choose one for each and try again.'));
      expect(problem()!.textContent).toContain('Scenes 2, 3 have no picture');
      expect(problem()!.getAttribute('role')).toBe('alert');
      expect(make()).toBeTruthy();
      expect(q('active')).toBeNull();
    });

    it('can try again after a failure that may pass', async () => {
      await attempt(httpError(0));
      expect(problem()!.textContent).toContain("You're offline");
      svc.create.and.returnValue(of(video()));
      problem()!.querySelector('button')!.click();
      await render();
      expect(svc.create).toHaveBeenCalledTimes(2);
      expect(q('active')).toBeTruthy();
    });

    it('offers no retry for a failure that trying again will not fix', async () => {
      await attempt(httpError(400, 'UNSUPPORTED_LANGUAGE'));
      expect(problem()!.querySelector('button')).toBeNull();
    });

    it('asks the page to reload the plan when it changed, and starts nothing', async () => {
      await attempt(httpError(409, 'STALE_REVISION'));
      expect(problem()!.textContent).toContain('This plan changed somewhere else');
      const reload = problem()!.querySelector('button')!;
      expect(reload.textContent!.trim()).toBe('Reload the plan');
      reload.click();
      await render();
      expect(changed).toBe(1);
      expect(problem()).toBeNull();
      expect(svc.create).toHaveBeenCalledTimes(1);
    });

    it('says when the limit of videos at once is reached', async () => {
      await attempt(httpError(409, 'TOO_MANY_ACTIVE'));
      expect(problem()!.getAttribute('data-kind')).toBe('too_many_active');
    });

    it('says when the day\'s limit is reached, and when to come back', async () => {
      await attempt(httpError(429, 'RATE_LIMITED', '', { retryAfter: 7200 }));
      expect(problem()!.textContent).toContain("You've made today's videos");
      expect(problem()!.textContent).toContain('Try again in 2 hours');
      expect(problem()!.getAttribute('role')).toBe('status');
    });

    it('says when making videos is not switched on', async () => {
      await attempt(httpError(503, 'UNAVAILABLE'));
      expect(problem()!.getAttribute('data-kind')).toBe('disabled');
    });

    it('takes a video that is already being made from this plan and follows it, saying so', async () => {
      svc.get.and.returnValue(of(rendering({ jobId: 'job-9', progress: 40 })));
      await attempt(httpError(409, 'ALREADY_RUNNING', '', { jobId: 'job-9' }));
      expect(svc.get).toHaveBeenCalledWith('job-9');
      expect(problem()).toBeNull();
      expect(q('active')).toBeTruthy();
      expect(percent()).toBe('40%');
      expect(q('follow-notice')!.textContent).toContain('already being made from this plan');
    });

    it('shows the problem if that video cannot be found either', async () => {
      svc.get.and.returnValue(throwError(() => httpError(404, 'NOT_FOUND')));
      await attempt(httpError(409, 'ALREADY_RUNNING', '', { jobId: 'job-9' }));
      expect(problem()).toBeTruthy();
      expect(q('active')).toBeNull();
    });
  });

  // -------------------------------------------------------------------------

  describe('following a video', () => {
    it('shows progress, and checks again every few seconds', fakeAsync(() => {
      svc.get.and.returnValues(of(rendering({ progress: 70 })), of(rendering({ progress: 85 })));
      mountFake({ videos: [video({ status: 'narrating', progress: 30 })], pollMs: 1000 });
      expect(percent()).toBe('30%');
      expect(svc.get).not.toHaveBeenCalled();
      pump(1000);
      expect(svc.get).toHaveBeenCalledOnceWith('job-1');
      expect(percent()).toBe('70%');
      pump(1000);
      expect(percent()).toBe('85%');
      expect(svc.get).toHaveBeenCalledTimes(2);
      fixture.destroy();
      flush();
    }));

    it('picks up a video that was being made before the page was opened', fakeAsync(() => {
      svc.get.and.returnValue(of(completed()));
      mountFake({ videos: [rendering()], pollMs: 1000 });
      expect(q('active')).toBeTruthy();
      pump(1000);
      expect(q('done')).toBeTruthy();
    }));

    it('stops when the video is done, says so, and does not take focus', fakeAsync(() => {
      svc.get.and.returnValue(of(completed()));
      mountFake({ videos: [rendering()], pollMs: 1000 });
      const other = document.createElement('input');
      document.body.appendChild(other);
      other.focus();
      pump(1000);
      expect(q('announcement')!.textContent).toBe('Your video is ready.');
      expect(document.activeElement).toBe(other);
      other.remove();
      pump(10_000);
      expect(svc.get).toHaveBeenCalledTimes(1);
    }));

    it('shows a failure the person watched happen as an alert, and stops checking', fakeAsync(() => {
      svc.get.and.returnValue(of(failed({ errorCode: 'INTERNAL', error: 'Something went wrong on our side while making the video. Try again.' })));
      mountFake({ videos: [rendering()], pollMs: 1000 });
      pump(1000);
      expect(q('failed')).toBeTruthy();
      expect(problem()!.getAttribute('role')).toBe('alert');
      expect(q('announcement')!.textContent).toContain('The video was not finished. Something went wrong');
      pump(10_000);
      expect(svc.get).toHaveBeenCalledTimes(1);
    }));

    it('does not announce a cancel as a failure', fakeAsync(() => {
      svc.get.and.returnValue(of(cancelledVideo()));
      mountFake({ videos: [rendering()], pollMs: 1000 });
      pump(1000);
      expect(q('announcement')!.textContent).toBe('The video was cancelled.');
      expect(text()).toContain('You cancelled this video');
    }));

    it('is quiet about a stage change that already happened and says when it changes', fakeAsync(() => {
      svc.get.and.returnValues(of(video({ status: 'narrating', progress: 30 })), of(rendering()));
      mountFake({ videos: [video({ status: 'preparing', progress: 8 })], pollMs: 1000 });
      expect(q('doing')!.textContent).toBe('Getting your pictures ready');
      pump(1000);
      expect(q('doing')!.textContent).toBe('Recording the narration');
      pump(1000);
      expect(q('doing')!.textContent).toBe('Putting the video together');
      fixture.destroy();
      flush();
    }));

    it('stops when the page goes away', fakeAsync(() => {
      svc.get.and.returnValue(of(rendering()));
      mountFake({ videos: [rendering()], pollMs: 1000 });
      pump(1000);
      fixture.destroy();
      tick(10_000);
      expect(svc.get).toHaveBeenCalledTimes(1);
    }));

    it('forgets a video that was deleted somewhere else', fakeAsync(() => {
      svc.get.and.returnValue(throwError(() => httpError(404, 'NOT_FOUND')));
      mountFake({ videos: [rendering()], pollMs: 1000 });
      pump(1000);
      expect(q('active')).toBeNull();
      expect(make()).toBeTruthy();
      tick(10_000);
      expect(svc.get).toHaveBeenCalledTimes(1);
    }));
  });

  describe('when the connection drops while a video is being made', () => {
    it('says so, backs off, and carries on when it is back', fakeAsync(() => {
      svc.get.and.returnValues(
        throwError(() => httpError(0)),
        throwError(() => httpError(0)),
        of(rendering({ progress: 88 })),
      );
      mountFake({ videos: [rendering({ progress: 60 })], pollMs: 1000 });
      pump(1000); // first check fails
      expect(q('reconnecting')!.textContent).toContain('The video keeps being made');
      expect(percent()).toBe('60%'); // what was last known stays on screen
      pump(1999); // the next try waits 2 s
      expect(svc.get).toHaveBeenCalledTimes(1);
      pump(1); // 2 s after the first failure
      expect(svc.get).toHaveBeenCalledTimes(2);
      pump(3999); // then 4 s
      expect(svc.get).toHaveBeenCalledTimes(2);
      pump(1);
      expect(svc.get).toHaveBeenCalledTimes(3);
      expect(q('reconnecting')).toBeNull();
      expect(percent()).toBe('88%');
      fixture.destroy();
      flush();
    }));

    it('gives up after five failures in a row, says what to do, and resumes when asked', fakeAsync(() => {
      svc.get.and.returnValue(throwError(() => httpError(0)));
      mountFake({ videos: [rendering()], pollMs: 100 });
      pump(100_000);
      expect(svc.get).toHaveBeenCalledTimes(5);
      expect(q('reconnecting')).toBeNull();
      const p = problem()!;
      expect(p.textContent).toContain("You're offline");
      const calls = svc.get.calls.count();
      pump(60_000);
      expect(svc.get.calls.count()).toBe(calls); // it has stopped
      svc.get.and.returnValue(of(completed()));
      p.querySelector('button')!.click();
      pump(100);
      expect(q('done')).toBeTruthy();
      expect(problem()).toBeNull();
    }));

    it('never waits longer than half a minute between tries', fakeAsync(() => {
      svc.get.and.returnValue(throwError(() => httpError(0)));
      mountFake({ videos: [rendering()], pollMs: 20_000 });
      pump(20_000); // failure 1, next in min(40 s, 30 s) = 30 s
      pump(29_999);
      expect(svc.get).toHaveBeenCalledTimes(1);
      pump(1);
      expect(svc.get).toHaveBeenCalledTimes(2);
      fixture.destroy();
      flush();
    }));
  });

  // -------------------------------------------------------------------------

  describe('cancelling', () => {
    it('asks the API, stops following, and turns into the cancelled view with focus on it', fakeAsync(() => {
      svc.cancel.and.returnValue(of(cancelledVideo()));
      svc.get.and.returnValue(of(rendering()));
      mountFake({ videos: [rendering()], pollMs: 1000 });
      q<HTMLButtonElement>('cancel')!.click();
      pump();
      expect(svc.cancel).toHaveBeenCalledOnceWith('job-1');
      expect(q('failed')).toBeTruthy();
      expect(heading()!.textContent).toBe('You cancelled this video');
      expect(text()).toContain('Nothing was kept. Make it again whenever you like.');
      expect(document.activeElement).toBe(heading());
      tick(10_000);
      expect(svc.get).not.toHaveBeenCalled();
    }));

    it('is not an error: no alert, and the button to make it again keeps the choices', fakeAsync(() => {
      svc.cancel.and.returnValue(of(cancelledVideo({ voice: 'Matthew', caption: { preset: 'clean' } })));
      mountFake({ videos: [rendering({ voice: 'Matthew', caption: { preset: 'clean' } })], pollMs: 1000 });
      q<HTMLButtonElement>('cancel')!.click();
      pump();
      expect(problem()).toBeNull();
      expect(make().textContent!.trim()).toBe('Make the video again');
      expect(el.querySelector<HTMLSelectElement>('select')!.value).toBe('Matthew');
      expect(el.querySelector<HTMLInputElement>('input[type="radio"]:checked')!.value).toBe('clean');
    }));

    it('is a single request however often the button is pressed', async () => {
      const pending = new Subject<StoryVideo>();
      svc.cancel.and.returnValue(pending);
      await mount({ videos: [rendering()], pollMs: 10_000_000 });
      q<HTMLButtonElement>('cancel')!.click();
      q<HTMLButtonElement>('cancel')!.click();
      await render();
      expect(svc.cancel).toHaveBeenCalledTimes(1);
      expect(q<HTMLButtonElement>('cancel')!.disabled).toBeTrue();
      expect(q('cancel')!.textContent).toContain('Cancelling…');
    });

    it('says what went wrong, and the video is still being followed', fakeAsync(() => {
      svc.cancel.and.returnValue(throwError(() => httpError(500, 'DB_ERROR')));
      svc.get.and.returnValue(of(rendering({ progress: 90 })));
      mountFake({ videos: [rendering()], pollMs: 1000 });
      q<HTMLButtonElement>('cancel')!.click();
      pump();
      expect(problem()!.textContent).toContain("The video wasn't cancelled");
      expect(q('active')).toBeTruthy();
      expect(q<HTMLButtonElement>('cancel')!.disabled).toBeFalse();
      pump(1000);
      expect(percent()).toBe('90%');
      fixture.destroy();
      flush();
    }));

    it('shows the finished video when it finished before the cancel got there', fakeAsync(() => {
      svc.cancel.and.returnValue(of(completed()));
      mountFake({ videos: [rendering()], pollMs: 1000 });
      q<HTMLButtonElement>('cancel')!.click();
      pump();
      expect(q('done')).toBeTruthy();
      expect(q('failed')).toBeNull();
    }));
  });

  // -------------------------------------------------------------------------

  describe('a video that did not finish', () => {
    it('says why in the API\'s words, and offers to make it again with the same choices', async () => {
      await mount({ videos: [failed({ voice: 'Brian', caption: { preset: 'minimal' } })] });
      expect(q('failed')).toBeTruthy();
      expect(problem()!.textContent).toContain("The video wasn't finished");
      expect(problem()!.textContent).toContain('Something went wrong on our side while making the video. Try again.');
      expect(make().textContent!.trim()).toBe('Make the video again');
      expect(el.querySelector<HTMLSelectElement>('select')!.value).toBe('Brian');
      expect(el.querySelector<HTMLInputElement>('input[type="radio"]:checked')!.value).toBe('minimal');
    });

    it('is reported quietly when found on load: it already happened', async () => {
      await mount({ videos: [failed()] });
      expect(problem()!.getAttribute('role')).toBe('status');
    });

    it('makes the video again from the choices', async () => {
      svc.create.and.returnValue(of(video({ jobId: 'job-2' })));
      await mount({ videos: [failed({ voice: 'Brian' })], revision: 3 });
      make().click();
      expect(svc.create).toHaveBeenCalledOnceWith({ planId: 'plan-1', revision: 3, voice: 'Brian', caption: { preset: 'bold' } });
    });

    it('titles a timeout and an over-long narration by what they are', async () => {
      await mount({ videos: [failed({ errorCode: 'TIMED_OUT', error: 'Making the video took too long and was stopped. Try again, or shorten the story.' })] });
      expect(problem()!.textContent).toContain('The video took too long');
      fixture.destroy();
      await mount({ videos: [failed({ errorCode: 'NARRATION_TOO_LONG', error: 'The narration runs 94 seconds, over the 90 second limit. Shorten the narration in the plan and try again.' })] });
      expect(problem()!.textContent).toContain('The narration is too long');
      expect(problem()!.textContent).toContain('over the 90 second limit');
    });

    it('takes the person to the scene whose picture is the problem', async () => {
      await mount({
        videos: [failed({ errorCode: 'MEDIA_UNAVAILABLE', error: 'Scene 2: the picture is no longer available from Pixabay. Choose another and make the video again.' })],
      });
      expect(problem()!.textContent).toContain('A picture can no longer be used');
      const go = q<HTMLButtonElement>('go-to-scene')!;
      expect(go.textContent!.trim()).toBe('Go to scene 2');
      go.click();
      expect(scenes).toEqual([2]);
    });

    it('offers no such shortcut for a failure that is not about a picture, or names no scene', async () => {
      await mount({ videos: [failed({ errorCode: 'INTERNAL', error: 'Scene 2: something went wrong on our side. Try again.' })] });
      expect(q('go-to-scene')).toBeNull();
      fixture.destroy();
      await mount({ videos: [failed({ errorCode: 'MEDIA_INVALID', error: 'The chosen picture can\'t be used.' })] });
      expect(q('go-to-scene')).toBeNull();
    });

    it('can be removed without a question, since nothing is lost', async () => {
      svc.remove.and.returnValue(of(undefined));
      await mount({ videos: [failed()] });
      q<HTMLButtonElement>('dismiss')!.click();
      await render();
      expect(svc.remove).toHaveBeenCalledOnceWith('job-1');
      expect(q('failed')).toBeNull();
      expect(make().textContent!.trim()).toBe('Make the video');
      expect(q('announcement')!.textContent).toBe('The attempt was removed.');
    });

    it('says when removing it did not work', async () => {
      svc.remove.and.returnValue(throwError(() => httpError(500, 'DB_ERROR')));
      await mount({ videos: [failed()] });
      q<HTMLButtonElement>('dismiss')!.click();
      await render();
      expect(q('failed')).toBeTruthy();
      expect(el.querySelectorAll('[data-testid="problem"]').length).toBe(2);
      expect(text()).toContain("The video wasn't deleted");
    });
  });

  // -------------------------------------------------------------------------

  describe('a finished video', () => {
    beforeEach(() => mount({ videos: [completed({ voice: 'Matthew', caption: { preset: 'karaoke' }, totalSeconds: 78 })] }));

    it('says it is ready, how long it is, and how it was made', () => {
      expect(q('done')!.querySelector('h3')!.textContent).toBe('Your video is ready');
      const facts = q('finished')!.querySelector('p')!.textContent!.replace(/\s+/g, ' ').trim();
      expect(facts).toBe('1 minute 18 seconds · Matthew voice · Karaoke highlight captions');
    });

    it('says when it was made, in the time of the person looking', () => {
      // Local time, so the expectation is built the same way and not written out.
      const when = formatDate('2026-10-31T10:03:00Z', 'd MMM y, h:mm a', 'en-US');
      expect(q('finished')!.textContent).toContain(`Made ${when}.`);
    });

    it('offers the download as the one main action', () => {
      const primaries = Array.from(el.querySelectorAll('button')).filter((b) => b.className.includes('bg-[#BC5227]'));
      expect(primaries.length).toBe(1);
      expect(primaries[0].textContent).toContain('Download MP4');
    });

    it('has no form until another version is asked for', () => {
      expect(q('make-form')).toBeNull();
      expect(q('another')).toBeTruthy();
    });

    it('does not ask for a link until one is needed', () => {
      expect(svc.download).not.toHaveBeenCalled();
    });
  });

  describe('downloading', () => {
    let clicks: { href: string; download: string }[];

    beforeEach(async () => {
      clicks = [];
      spyOn(HTMLAnchorElement.prototype, 'click').and.callFake(function (this: HTMLAnchorElement) {
        clicks.push({ href: this.href, download: this.download });
      });
      await mount({ videos: [completed()] });
    });

    it('asks for a fresh link at the moment of the click, and follows it', async () => {
      svc.download.and.returnValue(of({ url: 'https://files.example/v.mp4?sig=abc', filename: 'story-video.mp4', expiresInSeconds: 900 }));
      q<HTMLButtonElement>('download')!.click();
      await render();
      expect(svc.download).toHaveBeenCalledOnceWith('job-1');
      expect(clicks).toEqual([{ href: 'https://files.example/v.mp4?sig=abc', download: 'story-video.mp4' }]);
      expect(document.body.querySelector('a[download]')).toBeNull();
    });

    it('shows that it is working, and is a single request', async () => {
      const pending = new Subject<{ url: string; filename: string; expiresInSeconds: number }>();
      svc.download.and.returnValue(pending);
      q<HTMLButtonElement>('download')!.click();
      q<HTMLButtonElement>('download')!.click();
      await render();
      expect(svc.download).toHaveBeenCalledTimes(1);
      expect(q<HTMLButtonElement>('download')!.disabled).toBeTrue();
      expect(q('download')!.textContent).toContain('Preparing…');
      pending.next({ url: 'https://files.example/v.mp4', filename: 'story-video.mp4', expiresInSeconds: 900 });
      await render();
      expect(q<HTMLButtonElement>('download')!.disabled).toBeFalse();
      expect(q('download')!.textContent).toContain('Download MP4');
    });

    it('says what went wrong, and tries again when pressed again', async () => {
      svc.download.and.returnValue(throwError(() => httpError(500, 'S3_ERROR')));
      q<HTMLButtonElement>('download')!.click();
      await render();
      expect(problem()!.textContent).toContain("The download couldn't be prepared");
      expect(clicks).toEqual([]);
      svc.download.and.returnValue(of({ url: 'https://files.example/v.mp4', filename: 'story-video.mp4', expiresInSeconds: 900 }));
      q<HTMLButtonElement>('download')!.click();
      await render();
      expect(clicks.length).toBe(1);
      expect(problem()).toBeNull();
    });

    it('says when the file is gone', async () => {
      svc.download.and.returnValue(throwError(() => httpError(404, 'NOT_FOUND')));
      q<HTMLButtonElement>('download')!.click();
      await render();
      expect(problem()!.textContent).toContain("We can't find that video");
    });
  });

  describe('watching it here', () => {
    beforeEach(async () => {
      svc.download.and.returnValue(of({ url: 'https://files.example/v.mp4?sig=abc', filename: 'story-video.mp4', expiresInSeconds: 900 }));
      await mount({ videos: [completed()] });
    });

    it('shows no player until asked, then a player the person starts themselves', async () => {
      expect(q('player')).toBeNull();
      q<HTMLButtonElement>('watch')!.click();
      await render();
      const player = q<HTMLVideoElement>('player')!;
      expect(player.getAttribute('src')).toBe('https://files.example/v.mp4?sig=abc');
      expect(player.hasAttribute('controls')).toBeTrue();
      expect(player.hasAttribute('autoplay')).toBeFalse();
      expect(player.getAttribute('aria-label')).toBe('Your finished video');
      expect(q('watch')!.getAttribute('aria-expanded')).toBe('true');
      expect(q('watch')!.textContent!.trim()).toBe('Hide the video');
    });

    it('hides it again', async () => {
      q<HTMLButtonElement>('watch')!.click();
      await render();
      q<HTMLButtonElement>('watch')!.click();
      await render();
      expect(q('player')).toBeNull();
      expect(q('watch')!.getAttribute('aria-expanded')).toBe('false');
    });

    it('says what to do when the link has expired and the video cannot play', async () => {
      q<HTMLButtonElement>('watch')!.click();
      await render();
      q<HTMLVideoElement>('player')!.dispatchEvent(new Event('error'));
      await render();
      expect(q('player')).toBeNull();
      expect(problem()!.textContent).toContain("The video couldn't be played here");
      expect(problem()!.textContent).toContain('Choose "Watch it here" again, or download the file.');
    });

    it('says what went wrong when no link can be had', async () => {
      svc.download.and.returnValue(throwError(() => httpError(0)));
      q<HTMLButtonElement>('watch')!.click();
      await render();
      expect(q('player')).toBeNull();
      expect(problem()!.textContent).toContain("You're offline");
    });
  });

  describe('making another version', () => {
    it('opens the choices, makes the main action the new video, and closes again', async () => {
      await mount({ videos: [completed()] });
      q<HTMLButtonElement>('another')!.click();
      await render();
      expect(q('make-form')).toBeTruthy();
      expect(make().textContent!.trim()).toBe('Make this version');
      const primaries = Array.from(el.querySelectorAll('button')).filter((b) => b.className.includes('bg-[#BC5227]'));
      expect(primaries.length).withContext('one main action at a time').toBe(1);
      expect(primaries[0]).toBe(make());
      expect(q('another')).toBeNull();
      Array.from(el.querySelectorAll('button')).find((b) => b.textContent!.trim() === 'Not now')!.click();
      await render();
      expect(q('make-form')).toBeNull();
      expect(q('another')).toBeTruthy();
    });

    it('keeps the finished video on the page, in the list of earlier ones, once a new one is started', async () => {
      svc.create.and.returnValue(of(video({ jobId: 'job-2' })));
      await mount({ videos: [completed()] });
      q<HTMLButtonElement>('another')!.click();
      await render();
      make().click();
      await render();
      expect(q('active')).toBeTruthy();
      expect(q('earlier')).toBeTruthy();
      expect(q('earlier')!.querySelectorAll('[data-testid="finished-earlier"]').length).toBe(1);
      expect(q('earlier')!.querySelector('[data-testid="finished-earlier"]')!.getAttribute('data-job')).toBe('job-1');
    });

    it('tells a video made from an older plan that the plan has changed since', async () => {
      await mount({ videos: [completed({ planRevision: 2 })], revision: 3 });
      expect(q('stale-note')!.textContent).toContain('The plan has changed since');
      fixture.destroy();
      await mount({ videos: [completed({ planRevision: 3 })], revision: 3 });
      expect(q('stale-note')).toBeNull();
    });
  });

  describe('earlier videos', () => {
    const list = [completed({ jobId: 'job-3' }), completed({ jobId: 'job-2', voice: 'Amy' }), failed({ jobId: 'job-1' })];

    it('lists the finished ones below the newest, newest first, and leaves out the ones that failed', async () => {
      await mount({ videos: list });
      const rows = Array.from(q('earlier')!.querySelectorAll('[data-testid="finished-earlier"]'));
      expect(rows.map((r) => r.getAttribute('data-job'))).toEqual(['job-2']);
      expect(rows[0].textContent).toContain('Amy voice');
    });

    it('has no list when there is nothing earlier', async () => {
      await mount({ videos: [completed()] });
      expect(q('earlier')).toBeNull();
    });

    it('lets each be downloaded and deleted by its own id', async () => {
      svc.remove.and.returnValue(of(undefined));
      await mount({ videos: list });
      const row = q('earlier')!;
      row.querySelector<HTMLButtonElement>('[data-testid="delete"]')!.click();
      await render();
      row.querySelector<HTMLButtonElement>('[data-testid="confirm-delete"]')!.click();
      await render();
      expect(svc.remove).toHaveBeenCalledOnceWith('job-2');
      expect(q('earlier')).toBeNull();
      expect(q('done')).toBeTruthy();
    });
  });

  describe('deleting', () => {
    beforeEach(() => mount({ videos: [completed()] }));

    const ask = async () => {
      q<HTMLButtonElement>('delete')!.click();
      await render();
    };

    it('asks first, and puts focus on the safe choice', async () => {
      await ask();
      expect(q('confirm')!.textContent).toContain('Delete this video?');
      expect(q('confirm')!.textContent).toContain('The file is removed for good. Your plan stays');
      expect(document.activeElement).toBe(q('keep'));
      expect(svc.remove).not.toHaveBeenCalled();
    });

    it('keeps the video when told to, and returns focus to the button that was pressed', async () => {
      await ask();
      q<HTMLButtonElement>('keep')!.click();
      await render();
      expect(q('confirm')).toBeNull();
      expect(svc.remove).not.toHaveBeenCalled();
      expect(document.activeElement).toBe(q('delete'));
    });

    it('closes on Escape', async () => {
      await ask();
      q('confirm')!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
      await render();
      expect(q('confirm')).toBeNull();
      expect(svc.remove).not.toHaveBeenCalled();
    });

    it('deletes when confirmed, says so, and is ready for another video', async () => {
      svc.remove.and.returnValue(of(undefined));
      await ask();
      q<HTMLButtonElement>('confirm-delete')!.click();
      await render();
      expect(svc.remove).toHaveBeenCalledOnceWith('job-1');
      expect(q('done')).toBeNull();
      expect(make().textContent!.trim()).toBe('Make the video');
      expect(q('announcement')!.textContent).toBe('The video was deleted.');
    });

    it('is a single request however often it is confirmed', async () => {
      const pending = new Subject<void>();
      svc.remove.and.returnValue(pending);
      await ask();
      q<HTMLButtonElement>('confirm-delete')!.click();
      q<HTMLButtonElement>('confirm-delete')!.click();
      await render();
      expect(svc.remove).toHaveBeenCalledTimes(1);
      expect(q<HTMLButtonElement>('confirm-delete')!.disabled).toBeTrue();
      expect(q('confirm-delete')!.textContent).toContain('Deleting…');
    });

    it('keeps the video and says what went wrong when the delete fails', async () => {
      svc.remove.and.returnValue(throwError(() => httpError(500, 'S3_CLEANUP_FAILED')));
      await ask();
      q<HTMLButtonElement>('confirm-delete')!.click();
      await render();
      expect(q('done')).toBeTruthy();
      expect(problem()!.textContent).toContain("The video's files weren't deleted");
      expect(problem()!.textContent).toContain('still in your list');
    });

    it('stops showing a player for a video that is gone', async () => {
      svc.download.and.returnValue(of({ url: 'https://files.example/v.mp4', filename: 'story-video.mp4', expiresInSeconds: 900 }));
      svc.remove.and.returnValue(of(undefined));
      q<HTMLButtonElement>('watch')!.click();
      await render();
      expect(q('player')).toBeTruthy();
      await ask();
      q<HTMLButtonElement>('confirm-delete')!.click();
      await render();
      expect(q('player')).toBeNull();
    });
  });

  // -------------------------------------------------------------------------

  describe('moving to another plan', () => {
    it('looks for that plan\'s videos', async () => {
      await mount();
      fixture.componentRef.setInput('planId', 'plan-2');
      await render();
      expect(svc.listForPlan.calls.allArgs()).toEqual([['plan-1'], ['plan-2']]);
    });
  });

  describe('every state', () => {
    it('uses no em dash and no emoji in what it says', async () => {
      const states: Mount[] = [
        {},
        { blocker: 'Scene 3 has no picture. Choose one, then save.' },
        { videos: [rendering()] },
        { videos: [failed()] },
        { videos: [cancelledVideo()] },
        { videos: [completed(), completed({ jobId: 'job-0' })] },
        { limits: { ...VIDEO_LIMITS, enabled: false } },
      ];
      for (const s of states) {
        await mount(s);
        const t = text();
        expect(t).not.toContain('\u2014');
        expect(t).not.toMatch(/\p{Extended_Pictographic}/u);
        fixture.destroy();
      }
    });

    it('never has more than one main action on screen', async () => {
      const states: Mount[] = [{}, { videos: [rendering()] }, { videos: [failed()] }, { videos: [completed(), completed({ jobId: 'job-0' })] }];
      for (const s of states) {
        await mount(s);
        const primaries = Array.from(el.querySelectorAll('button')).filter((b) => b.className.includes('bg-[#BC5227]'));
        expect(primaries.length).withContext(JSON.stringify(s)).toBeLessThanOrEqual(1);
        fixture.destroy();
      }
    });

    it('gives every button a tap target of at least 44 pixels', async () => {
      await mount({ videos: [completed(), completed({ jobId: 'job-0' })] });
      q<HTMLButtonElement>('delete')!.click();
      await render();
      for (const b of Array.from(el.querySelectorAll('button'))) {
        expect(b.className).withContext(b.textContent!.trim()).toContain('min-h-11');
      }
    });

    it('announces results in a polite status region that is off screen', async () => {
      await mount();
      const a = q('announcement')!;
      expect(a.getAttribute('role')).toBe('status');
      expect(a.classList.contains('sr-only')).toBeTrue();
    });
  });
});
