import { HttpErrorResponse } from '@angular/common/http';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { Subject, of, throwError } from 'rxjs';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { StoryPlanEditorComponent } from './story-plan-editor.component';
import { StoryPlanRecord } from './story-plan.models';
import { StoryPlanService } from './story-plan.service';
import {
  NavbarStub,
  ServiceSpy,
  STORY_ICONS,
  asset,
  httpError,
  planRecord,
  routeFor,
  searchResponse,
  serviceSpy,
  settle,
  videoServiceSpy,
  VideoServiceSpy,
} from './story-plan-fixtures.spec';
import { StoryVideoService } from './story-video.service';

describe('StoryPlanEditorComponent', () => {
  let service: ServiceSpy;
  let videoService: VideoServiceSpy;
  let fixture: ComponentFixture<StoryPlanEditorComponent>;
  let el: HTMLElement;

  async function create(record: StoryPlanRecord | HttpErrorResponse = planRecord()): Promise<void> {
    service.get.and.returnValue(record instanceof HttpErrorResponse ? throwError(() => record) : of(record));
    TestBed.overrideComponent(StoryPlanEditorComponent, {
      remove: { imports: [NavbarComponent] },
      add: { imports: [NavbarStub] },
    });
    await TestBed.configureTestingModule({
      imports: [StoryPlanEditorComponent],
      providers: [provideRouter([]), routeFor('plan-1'), { provide: StoryPlanService, useValue: service }, { provide: StoryVideoService, useValue: videoService }, STORY_ICONS],
    }).compileComponents();
    fixture = TestBed.createComponent(StoryPlanEditorComponent);
    el = fixture.nativeElement;
    await settle(fixture);
  }

  beforeEach(() => {
    service = serviceSpy();
    videoService = videoServiceSpy();
    service.search.and.returnValue(of(searchResponse()));
  });

  afterEach(() => {
    el?.querySelector('dialog')?.close();
    document.head.querySelectorAll('meta[name="robots"], link[rel="canonical"]').forEach((n) => n.remove());
  });

  const text = () => el.textContent!.replace(/\s+/g, ' ');
  const scene = (n: number) => el.querySelectorAll('app-story-scene-card')[n - 1] as HTMLElement;
  const narrationBox = (n: number) => scene(n).querySelector('textarea') as HTMLTextAreaElement;
  const saveButton = () => Array.from(el.querySelectorAll('button')).find((b) => b.textContent!.includes('Save changes'))!;
  const buttonIn = (root: ParentNode, label: RegExp) =>
    Array.from(root.querySelectorAll('button')).find((b) => label.test(b.textContent!))!;
  const status = () => el.querySelector('[data-testid="save-status"]')!.textContent!.replace(/\s+/g, ' ').trim();
  const problem = () => el.querySelector<HTMLElement>('[data-testid="problem"]');

  async function type(n: number, value: string): Promise<void> {
    narrationBox(n).value = value;
    narrationBox(n).dispatchEvent(new Event('input'));
    await settle(fixture);
  }

  async function pickInPicker(sceneNumber: number, resultIndex: number): Promise<void> {
    buttonIn(scene(sceneNumber), /(Replace|Choose) picture/).click();
    await settle(fixture);
    el.querySelectorAll<HTMLButtonElement>('dialog [data-testid="results"] > li button')[resultIndex].click();
    await settle(fixture);
  }

  describe('showing the plan', () => {
    beforeEach(() => create());

    it('asks the API for the plan in the address', () => {
      expect(service.get).toHaveBeenCalledOnceWith('plan-1');
    });

    it('shows the opening line, the scene count and the length against the limit', () => {
      expect(text()).toContain('The lighthouse stood alone.');
      expect(el.querySelector('[data-testid="summary"]')!.textContent).toContain('3 scenes, about 16 seconds. The limit is 90.');
    });

    it('lists the scenes in order, each with its narration and length', () => {
      expect(el.querySelectorAll('app-story-scene-card').length).toBe(3);
      expect(narrationBox(1).value).toBe('The lighthouse stood alone on the cliff.');
      expect(narrationBox(2).value).toBe('Every night the keeper climbed the stairs.');
      expect(narrationBox(3).value).toBe('One morning the sea was calm again.');
      expect(scene(1).textContent).toContain('Scene 1');
      expect(scene(1).textContent).toContain('About 6.2 s');
      expect(scene(3).textContent).toContain('About 4.4 s');
    });

    it('labels every narration box and says how much room is left', () => {
      const box = narrationBox(1);
      const label = scene(1).querySelector(`label[for="${box.id}"]`)!;
      expect(label.textContent).toContain('Narration');
      const hint = scene(1).querySelector(`#${box.getAttribute('aria-describedby')}`)!;
      expect(hint.textContent).toContain('40 of 600 characters');
    });

    it('marks the narration with the plan\'s language, for screen readers', () => {
      expect(narrationBox(1).getAttribute('lang')).toBe('en');
    });

    it('shows the picture saved for a scene, with its credit', () => {
      const img = scene(1).querySelector('img')!;
      expect(img.getAttribute('src')).toBe('https://cdn.example/pixabay/101.jpg');
      expect(img.getAttribute('alt')).toBe('A picture numbered 101');
      const credit = scene(1).querySelector<HTMLAnchorElement>('[data-testid="credit"]')!;
      expect(credit.textContent).toContain('Photo 101 by Jo Doe on Pixabay');
      expect(credit.getAttribute('href')).toBe('https://pixabay.example/101');
    });

    it('shows a saved clip as a video the user starts themselves', () => {
      const video = scene(2).querySelector('video')!;
      expect(video.getAttribute('src')).toBe('https://cdn.example/pexels/202.mp4');
      expect(video.hasAttribute('controls')).toBeTrue();
      expect(video.hasAttribute('autoplay')).toBeFalse();
    });

    it('says plainly when a scene has no picture, and suggests one', () => {
      expect(scene(3).textContent).toContain('No picture chosen');
      expect(scene(3).textContent).toContain('Suggested: A calm sea at sunrise');
      expect(buttonIn(scene(3), /Choose picture/)).toBeTruthy();
      expect(buttonIn(scene(3), /Remove picture/)).toBeUndefined();
    });

    it('credits both providers on the page', () => {
      const links = Array.from(el.querySelectorAll('[data-testid="attribution"] a')).map((a) => a.getAttribute('href'));
      expect(links).toEqual(['https://www.pexels.com', 'https://pixabay.com']);
    });

    it('starts with nothing to save', () => {
      expect(status()).toBe('No changes yet');
      expect(saveButton().disabled).toBeTrue();
    });

    it('moves focus to the heading once the plan is there', () => {
      expect(document.activeElement).toBe(el.querySelector('h1'));
    });

    it('is kept out of search results', () => {
      expect(document.head.querySelector('meta[name="robots"]')!.getAttribute('content')).toContain('noindex');
    });
  });

  describe('pictures that cannot be shown', () => {
    it('says a picture is gone, who removed it, and what to do', async () => {
      const rec = planRecord();
      rec.media![0] = { provider: 'pixabay', type: 'photo', id: '101', status: 'unavailable' };
      await create(rec);
      expect(scene(1).textContent).toContain('This picture is no longer available');
      expect(scene(1).textContent).toContain('Pixabay no longer has it. Choose another.');
      expect(scene(1).querySelector('img')).toBeNull();
      expect(buttonIn(scene(1), /Replace picture/)).toBeTruthy();
    });

    it('says a picture failed to load for now, and what to do', async () => {
      const rec = planRecord();
      rec.media![1] = { provider: 'pexels', type: 'video', id: '202', status: 'error' };
      await create(rec);
      expect(scene(2).textContent).toContain('This picture could not be loaded');
      expect(scene(2).textContent).toContain('Try again later, or choose another.');
    });

    it('does not offer to save just because a picture is gone', async () => {
      const rec = planRecord();
      rec.media![0] = { provider: 'pixabay', type: 'photo', id: '101', status: 'unavailable' };
      await create(rec);
      expect(saveButton().disabled).toBeTrue();
    });

    it('shows a notice for the repairs made to the model\'s answer', async () => {
      const rec = planRecord();
      rec.plan!.warnings = ['Scene 4 was dropped: it had no narration'];
      await create(rec);
      const w = el.querySelector('[data-testid="warnings"]')!;
      expect(w.textContent).toContain('Notes about this plan');
      expect(w.textContent).toContain('Scene 4 was dropped');
    });
  });

  describe('plans that cannot be opened', () => {
    it('says when the plan is not there, and offers a new one instead of a retry', async () => {
      await create(httpError(404, 'NOT_FOUND', 'Plan not found'));
      expect(problem()!.textContent).toContain("We can't find that plan");
      expect(text()).not.toContain('Try again');
      expect(Array.from(el.querySelectorAll('a')).some((a) => /Plan a new video/.test(a.textContent!))).toBeTrue();
    });

    it('says when the plan never finished', async () => {
      await create(planRecord({ status: 'failed', plan: undefined, errorCode: 'PLAN_FAILED' }));
      expect(problem()!.textContent).toContain("This plan didn't finish");
      expect(el.querySelector('app-story-scene-card')).toBeNull();
    });

    it('says when the plan was interrupted', async () => {
      await create(planRecord({ status: 'pending', plan: undefined }));
      expect(problem()!.textContent).toContain('never finished');
    });

    it('offers to try again when the failure may pass, and recovers', async () => {
      await create(httpError(0));
      service.get.and.returnValue(of(planRecord()));
      expect(problem()!.textContent).toContain("You're offline");
      buttonIn(el, /Try again/).click();
      await settle(fixture);
      expect(problem()).toBeNull();
      expect(el.querySelectorAll('app-story-scene-card').length).toBe(3);
    });

    it('shows a loading message while the plan is on its way', async () => {
      service.get.and.returnValue(new Subject<StoryPlanRecord>());
      TestBed.overrideComponent(StoryPlanEditorComponent, { remove: { imports: [NavbarComponent] }, add: { imports: [NavbarStub] } });
      await TestBed.configureTestingModule({
        imports: [StoryPlanEditorComponent],
        providers: [provideRouter([]), routeFor('plan-1'), { provide: StoryPlanService, useValue: service }, { provide: StoryVideoService, useValue: videoService }, STORY_ICONS],
      }).compileComponents();
      fixture = TestBed.createComponent(StoryPlanEditorComponent);
      el = fixture.nativeElement;
      await settle(fixture);
      expect(el.querySelector('[role="status"]')!.textContent).toContain('Loading your plan');
    });
  });

  describe('editing the narration', () => {
    beforeEach(() => create());

    it('notices a change: says which scene, offers to save, and marks the scene', async () => {
      await type(2, 'Every night the keeper climbed all ninety-nine stairs.');
      expect(status()).toBe('1 scene has changes that are not saved');
      expect(saveButton().disabled).toBeFalse();
      expect(scene(2).querySelector('[data-testid="edited"]')!.textContent).toContain('Edited, not saved');
      expect(scene(1).querySelector('[data-testid="edited"]')).toBeNull();
    });

    it('counts characters as people do', async () => {
      await type(1, 'हिंदी');
      expect(scene(1).textContent).toContain('5 of 600 characters');
    });

    it('sends only what changed, with one entry per scene and the revision it was based on', async () => {
      service.save.and.returnValue(of(planRecord({ revision: 2 })));
      await type(2, 'Every night the keeper climbed all ninety-nine stairs.');
      saveButton().click();
      await settle(fixture);
      expect(service.save).toHaveBeenCalledOnceWith('plan-1', 1, [{}, { narration: 'Every night the keeper climbed all ninety-nine stairs.' }, {}]);
    });

    it('is not a change to type something and put it back', async () => {
      await type(2, 'Something else');
      await type(2, 'Every night the keeper climbed the stairs.');
      expect(status()).toBe('No changes yet');
      expect(saveButton().disabled).toBeTrue();
    });

    it('shows what the server saved, then edits against the new revision', async () => {
      const saved = planRecord({ revision: 2 });
      saved.plan!.scenes[1].narration = 'Cleaned and saved by the server.';
      saved.plan!.scenes[1].duration = 3.3;
      service.save.and.returnValues(of(saved), of(planRecord({ revision: 3 })));
      await type(2, 'Typed by the user');
      saveButton().click();
      await settle(fixture);

      expect(status()).toContain('Saved');
      expect(narrationBox(2).value).toBe('Cleaned and saved by the server.');
      expect(scene(2).textContent).toContain('About 3.3 s');
      expect(scene(2).querySelector('[data-testid="edited"]')).toBeNull();
      expect(saveButton().disabled).toBeTrue();

      await type(1, 'A later edit.');
      saveButton().click();
      await settle(fixture);
      expect(service.save.calls.mostRecent().args[1]).toBe(2);
    });

    it('shows a saving state, and locks the boxes while it waits', async () => {
      const pending = new Subject<StoryPlanRecord>();
      service.save.and.returnValue(pending);
      await type(1, 'Another version of the first scene.');
      saveButton().click();
      await settle(fixture);
      expect(status()).toBe('Saving…');
      expect(saveButton().disabled).toBeTrue();
      expect(narrationBox(1).readOnly).toBeTrue();
      expect(buttonIn(scene(1), /Replace picture/).disabled).toBeTrue();
      pending.next(planRecord({ revision: 2 }));
      await settle(fixture);
      expect(narrationBox(1).readOnly).toBeFalse();
    });

    it('does not save twice if the button is pressed twice', async () => {
      service.save.and.returnValue(new Subject<StoryPlanRecord>());
      await type(1, 'Another version of the first scene.');
      saveButton().click();
      saveButton().click();
      expect(service.save).toHaveBeenCalledTimes(1);
    });

    it('throws away unsaved edits on request', async () => {
      await type(1, 'Another version of the first scene.');
      buttonIn(el, /Discard changes/).click();
      await settle(fixture);
      expect(narrationBox(1).value).toBe('The lighthouse stood alone on the cliff.');
      expect(status()).toBe('No changes yet');
    });
  });

  describe('replacing a picture', () => {
    beforeEach(() => create());

    it('opens a search for that scene, using its keywords', async () => {
      buttonIn(scene(1), /Replace picture/).click();
      await settle(fixture);
      expect(service.search).toHaveBeenCalledWith({ q: 'lighthouse cliff storm', type: 'photo', page: 1, perPage: 12 });
      expect(el.querySelector('dialog')!.open).toBeTrue();
      expect(el.querySelector('dialog #picker-title')!.textContent).toContain('scene 1');
    });

    it('shows the new picture in the scene and counts it as an unsaved change', async () => {
      await pickInPicker(1, 1);
      const img = scene(1).querySelector('img')!;
      expect(img.getAttribute('src')).toBe('https://cdn.example/pixabay/302.jpg');
      expect(scene(1).querySelector('[data-testid="credit"]')!.textContent).toContain('Photo 302 by Jo Doe on Pixabay');
      expect(scene(1).querySelector('[data-testid="edited"]')).not.toBeNull();
      expect(status()).toBe('1 scene has changes that are not saved');
    });

    it('saves the pick as a reference to the asset, never as a link', async () => {
      service.save.and.returnValue(of(planRecord({ revision: 2 })));
      await pickInPicker(1, 1);
      saveButton().click();
      await settle(fixture);
      expect(service.save).toHaveBeenCalledOnceWith('plan-1', 1, [{ media: { provider: 'pixabay', type: 'photo', id: '302' } }, {}, {}]);
      expect(JSON.stringify(service.save.calls.mostRecent().args)).not.toContain('http');
    });

    it('fills a scene that had no picture', async () => {
      service.save.and.returnValue(of(planRecord({ revision: 2 })));
      await pickInPicker(3, 2);
      expect(scene(3).querySelector('[data-testid="credit"]')!.textContent).toContain('Photo 303 by Jo Doe on Pexels');
      expect(buttonIn(scene(3), /Replace picture/)).toBeTruthy();
      expect(buttonIn(scene(3), /Remove picture/)).toBeTruthy();
      saveButton().click();
      await settle(fixture);
      expect(service.save.calls.mostRecent().args[2]).toEqual([{}, {}, { media: { provider: 'pexels', type: 'photo', id: '303' } }]);
    });

    it('is no change at all to choose the picture that is already saved', async () => {
      service.search.and.returnValue(of(searchResponse({ assets: [asset({ id: '999' }), asset({ id: '101' })] })));
      await pickInPicker(1, 0); // a different one first
      expect(status()).toBe('1 scene has changes that are not saved');
      buttonIn(scene(1), /Replace picture/).click();
      await settle(fixture);
      expect(el.querySelector('dialog [data-testid="results"]')!.textContent).toContain('Current choice'); // 999 is now current
      el.querySelectorAll<HTMLButtonElement>('dialog [data-testid="results"] > li button')[0].click(); // back to 101
      await settle(fixture);
      expect(status()).toBe('No changes yet');
    });

    it('keeps the choice if the picture search is closed without choosing', async () => {
      buttonIn(scene(1), /Replace picture/).click();
      await settle(fixture);
      el.querySelector<HTMLButtonElement>('dialog button[aria-label="Close"]')!.click();
      await settle(fixture);
      expect(status()).toBe('No changes yet');
      expect(scene(1).querySelector('img')!.getAttribute('src')).toContain('101');
    });
  });

  describe('removing a picture', () => {
    beforeEach(() => create());

    it('shows the scene without one and saves that as a clear', async () => {
      service.save.and.returnValue(of(planRecord({ revision: 2 })));
      buttonIn(scene(2), /Remove picture/).click();
      await settle(fixture);
      expect(scene(2).textContent).toContain('No picture chosen');
      expect(scene(2).querySelector('video')).toBeNull();
      saveButton().click();
      await settle(fixture);
      expect(service.save.calls.mostRecent().args[2]).toEqual([{}, { clearMedia: true }, {}]);
    });

    it('keeps keyboard focus in the scene when the button that was pressed goes away', async () => {
      buttonIn(scene(2), /Remove picture/).click();
      await settle(fixture);
      expect(document.activeElement).toBe(el.querySelector('#scene-2-choose'));
    });

    it('is no change to add a picture to an empty scene and then remove it', async () => {
      await pickInPicker(3, 0);
      buttonIn(scene(3), /Remove picture/).click();
      await settle(fixture);
      expect(status()).toBe('No changes yet');
    });
  });

  describe('when saving fails', () => {
    beforeEach(() => create());

    async function failWith(error: ReturnType<typeof httpError>): Promise<void> {
      service.save.and.returnValue(throwError(() => error));
      await type(2, 'My careful rewrite of the second scene.');
      await pickInPicker(1, 1);
      saveButton().click();
      await settle(fixture);
    }

    const cases: [string, ReturnType<typeof httpError>, string, string][] = [
      ['the plan changed elsewhere', httpError(409, 'STALE_REVISION'), 'This plan changed somewhere else', 'Reload the latest plan'],
      ['the picture could not be confirmed', httpError(503, 'PLAN_UNAVAILABLE'), "The photo library didn't confirm that picture", 'Try saving again'],
      ['picture choice is switched off', httpError(503, 'UNAVAILABLE'), "Choosing a new picture isn't available yet", ''],
      ['it is too slow', httpError(504), 'That took too long', 'Try saving again'],
      ['there is a server error', httpError(500, 'DB_ERROR'), "Your changes weren't saved", 'Try saving again'],
      ['you are offline', httpError(0), "You're offline", 'Try saving again'],
      ['the narration is empty', httpError(400, 'VALIDATION_ERROR', 'Scene 2: the narration is empty'), "Those changes can't be saved", ''],
      ['the video would be too long', httpError(400, 'VALIDATION_ERROR', 'with these changes the video would be 96 seconds, over the 90 second limit; shorten the narration'), "Those changes can't be saved", ''],
      ['you save too often', httpError(429, 'RATE_LIMITED', '', { retryAfter: 30 }), 'Too many requests', 'Try saving again'],
    ];
    for (const [name, error, title, action] of cases) {
      it(`${name}: says "${title}" and keeps everything typed and chosen`, async () => {
        await failWith(error);
        expect(problem()!.textContent).toContain(title);
        expect(!!Array.from(problem()!.querySelectorAll('button')).find((b) => b.textContent!.includes(action || '\u0000'))).toBe(!!action);
        // Nothing was lost.
        expect(narrationBox(2).value).toBe('My careful rewrite of the second scene.');
        expect(scene(1).querySelector('img')!.getAttribute('src')).toContain('302');
        expect(status()).toContain('2 scenes have changes that are not saved');
        expect(saveButton().disabled).toBeFalse();
        expect(narrationBox(2).readOnly).toBeFalse();
      });
    }

    it('shows the API\'s own words for a problem with the input', async () => {
      await failWith(httpError(400, 'VALIDATION_ERROR', 'Scene 2: the narration is empty'));
      expect(problem()!.textContent).toContain('Scene 2: the narration is empty');
    });

    it('keeps the message while the user keeps typing, so a stale plan is not forgotten', async () => {
      await failWith(httpError(409, 'STALE_REVISION'));
      await type(1, 'Still typing');
      expect(problem()!.textContent).toContain('This plan changed somewhere else');
    });

    it('reloads the newest plan from a stale-plan message, and the old edits are gone', async () => {
      await failWith(httpError(409, 'STALE_REVISION'));
      const newer = planRecord({ revision: 5 });
      newer.plan!.scenes[0].narration = 'Written in another tab.';
      service.get.and.returnValue(of(newer));
      buttonIn(problem()!, /Reload the latest plan/).click();
      await settle(fixture);
      expect(problem()).toBeNull();
      expect(narrationBox(1).value).toBe('Written in another tab.');
      expect(narrationBox(2).value).toBe('Every night the keeper climbed the stairs.');
      expect(status()).toBe('No changes yet');
      service.save.and.returnValue(of(planRecord({ revision: 6 })));
      await type(3, 'Edit made against the new plan.');
      saveButton().click();
      await settle(fixture);
      expect(service.save.calls.mostRecent().args[1]).toBe(5);
    });

    it('tries again from the message and succeeds', async () => {
      await failWith(httpError(504));
      service.save.and.returnValue(of(planRecord({ revision: 2 })));
      buttonIn(problem()!, /Try saving again/).click();
      await settle(fixture);
      expect(problem()).toBeNull();
      expect(status()).toContain('Saved');
      expect(service.save).toHaveBeenCalledTimes(2);
    });
  });

  describe('leaving with edits that are not saved', () => {
    beforeEach(() => create());
    const component = () => fixture.componentInstance;

    /** Fires a real beforeunload at the window. Karma treats that as "the test reloaded the page", so its own handler is set aside meanwhile. */
    function fireBeforeUnload(): Event {
      const karmaHandler = window.onbeforeunload;
      window.onbeforeunload = null;
      const event = new Event('beforeunload', { cancelable: true });
      try {
        window.dispatchEvent(event);
      } finally {
        window.onbeforeunload = karmaHandler;
      }
      return event;
    }

    it('warns before the tab is closed', async () => {
      await type(1, 'Another version of the first scene.');
      expect(fireBeforeUnload().defaultPrevented).toBeTrue();
    });

    it('does not warn when there is nothing to lose', () => {
      expect(fireBeforeUnload().defaultPrevented).toBeFalse();
    });

    it('asks before leaving the page, and leaves only if the user agrees', async () => {
      await type(1, 'Another version of the first scene.');
      const confirmSpy = spyOn(window, 'confirm').and.returnValues(false, true);
      expect(component().canLeave()).toBeFalse();
      expect(component().canLeave()).toBeTrue();
      expect(confirmSpy).toHaveBeenCalledTimes(2);
    });

    it('lets the user go without asking when there are no changes', () => {
      const confirmSpy = spyOn(window, 'confirm');
      expect(component().canLeave()).toBeTrue();
      expect(confirmSpy).not.toHaveBeenCalled();
    });
  });

  describe('keyboard and screen readers', () => {
    beforeEach(() => create());

    it('uses real controls throughout, in reading order, with no positive tab stops', () => {
      expect(el.querySelectorAll('[tabindex]:not([tabindex="-1"]):not([tabindex="0"])').length).toBe(0);
      const clickable = Array.from(el.querySelectorAll('[onclick], [role="button"]'));
      expect(clickable.length).toBe(0);
    });

    it('gives every button a name that says which scene it is for', () => {
      for (const n of [1, 2, 3]) {
        const names = Array.from(scene(n).querySelectorAll('button')).map((b) => b.textContent!.replace(/\s+/g, ' ').trim());
        for (const name of names) expect(name).toContain(`scene ${n}`);
      }
    });

    it('opens links to the provider in a new tab, and says so', () => {
      for (const a of Array.from(el.querySelectorAll<HTMLAnchorElement>('a[target="_blank"]'))) {
        expect(a.rel).toContain('noopener');
        expect(a.textContent).toContain('opens in a new tab');
      }
    });

    it('announces the save state politely', () => {
      expect(el.querySelector('[data-testid="save-status"]')!.getAttribute('role')).toBe('status');
    });
  });

  describe('making the video from the plan', () => {
    const panel = () => el.querySelector('app-story-video-panel') as HTMLElement | null;
    const makeButton = () => panel()?.querySelector<HTMLButtonElement>('[data-testid="make"]') ?? null;
    const blocker = () => panel()?.querySelector('[data-testid="blocker"]')?.textContent ?? '';

    it('does not claim the video is unavailable any more', async () => {
      await create();
      expect(text()).not.toContain('not available yet');
      expect(text()).toContain('make the video below the last scene');
    });

    it('puts the panel after the last scene and its credits, for this plan and revision, in its language', async () => {
      await create(planRecord({ revision: 4 }));
      expect(panel()).toBeTruthy();
      const order = Array.from(el.querySelectorAll('app-story-scene-card, app-story-attribution, app-story-video-panel')).map((n) => n.tagName.toLowerCase());
      expect(order).toEqual(['app-story-scene-card', 'app-story-scene-card', 'app-story-scene-card', 'app-story-attribution', 'app-story-video-panel']);
      expect(videoService.listForPlan).toHaveBeenCalledOnceWith('plan-1');
    });

    it('is not shown for a plan that cannot be opened', async () => {
      await create(httpError(404, 'NOT_FOUND'));
      expect(panel()).toBeNull();
      expect(videoService.listForPlan).not.toHaveBeenCalled();
    });

    it('waits for a picture on every scene, and says which scene has none', async () => {
      await create();
      expect(blocker()).toBe('Scene 3 has no picture. Choose one, then save.');
      expect(makeButton()!.disabled).toBeTrue();
    });

    it('can make the video once every scene has a saved picture', async () => {
      const rec = planRecord();
      rec.plan!.scenes[2].media = { provider: 'pixabay', type: 'photo', id: '303' };
      await create(rec);
      expect(blocker()).toBe('');
      expect(makeButton()!.disabled).toBeFalse();
    });

    describe('with every picture chosen', () => {
      let rec: StoryPlanRecord;
      beforeEach(async () => {
        rec = planRecord();
        rec.plan!.scenes[2].media = { provider: 'pixabay', type: 'photo', id: '303' };
        await create(rec);
      });

      it('holds the button back while there are changes that are not saved, because the video uses the saved plan', async () => {
        await type(1, 'A different first line.');
        expect(blocker()).toContain('Save your changes first');
        expect(makeButton()!.disabled).toBeTrue();
        // Putting it back is no change, and the button comes back.
        await type(1, 'The lighthouse stood alone on the cliff.');
        expect(blocker()).toBe('');
        expect(makeButton()!.disabled).toBeFalse();
      });

      it('lets it go ahead after the changes are saved, for the newer revision', async () => {
        const saved = planRecord({ revision: 2 });
        saved.plan!.scenes[2].media = { provider: 'pixabay', type: 'photo', id: '303' };
        saved.plan!.scenes[0].narration = 'A different first line.';
        service.save.and.returnValue(of(saved));
        videoService.create.and.returnValue(of({
          jobId: 'job-1', planId: 'plan-1', planRevision: 2, status: 'queued', progress: 0, language: 'en', voice: 'Joanna',
          caption: { preset: 'bold' }, createdAt: '2026-10-31T10:00:00Z', updatedAt: '2026-10-31T10:00:00Z',
        }));
        await type(1, 'A different first line.');
        saveButton().click();
        await settle(fixture);
        expect(blocker()).toBe('');
        makeButton()!.click();
        expect(videoService.create).toHaveBeenCalledOnceWith({ planId: 'plan-1', revision: 2, voice: 'Joanna', caption: { preset: 'bold' } });
      });

      it('reloads the plan when the panel says it changed somewhere else', async () => {
        videoService.create.and.returnValue(throwError(() => httpError(409, 'STALE_REVISION')));
        makeButton()!.click();
        await settle(fixture);
        service.get.calls.reset();
        panel()!.querySelector<HTMLButtonElement>('[data-testid="problem"] button')!.click();
        await settle(fixture);
        expect(service.get).toHaveBeenCalledOnceWith('plan-1');
      });
    });

    describe('after a video failed because of one picture', () => {
      it('takes keyboard focus to the picture button of the scene the failure names', async () => {
        videoService.listForPlan.and.returnValue(of({
          hasMore: false,
          videos: [{
            jobId: 'job-1', planId: 'plan-1', planRevision: 1, status: 'failed', progress: 20, language: 'en', voice: 'Joanna',
            caption: { preset: 'bold' }, errorCode: 'MEDIA_UNAVAILABLE', failedStage: 'preparing',
            error: 'Scene 2: the picture is no longer available from Pexels. Choose another and make the video again.',
            createdAt: '2026-10-31T10:00:00Z', updatedAt: '2026-10-31T10:00:00Z',
          }],
        }));
        await create();
        panel()!.querySelector<HTMLButtonElement>('[data-testid="go-to-scene"]')!.click();
        expect(document.activeElement!.id).toBe('scene-2-choose');
      });
    });
  });
});
