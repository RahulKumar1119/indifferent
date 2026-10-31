import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { Subject, of, throwError } from 'rxjs';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { StoryPlanPageComponent } from './story-plan-page.component';
import { StoryPlanRecord } from './story-plan.models';
import { StoryPlanService } from './story-plan.service';
import {
  API_FAILURES,
  LIMITS,
  NavbarStub,
  ServiceSpy,
  STORY_ICONS,
  httpError,
  planRecord,
  serviceSpy,
  settle,
} from './story-plan-fixtures.spec';

describe('StoryPlanPageComponent', () => {
  let service: ServiceSpy;
  let fixture: ComponentFixture<StoryPlanPageComponent>;
  let el: HTMLElement;
  let navigate: jasmine.Spy;

  const STORY = 'A lighthouse keeper watched the sea alone for forty years, until one morning the water was calm.';

  async function create(): Promise<void> {
    TestBed.overrideComponent(StoryPlanPageComponent, { remove: { imports: [NavbarComponent] }, add: { imports: [NavbarStub] } });
    await TestBed.configureTestingModule({
      imports: [StoryPlanPageComponent],
      providers: [provideRouter([]), { provide: StoryPlanService, useValue: service }, STORY_ICONS],
    }).compileComponents();
    navigate = spyOn(TestBed.inject(Router), 'navigate').and.resolveTo(true);
    fixture = TestBed.createComponent(StoryPlanPageComponent);
    el = fixture.nativeElement;
    await settle(fixture);
  }

  beforeEach(() => {
    service = serviceSpy();
  });

  afterEach(() => {
    document.head.querySelectorAll('meta[name="robots"], link[rel="canonical"]').forEach((n) => n.remove());
  });

  const box = () => el.querySelector<HTMLTextAreaElement>('#story')!;
  const submitButton = () => el.querySelector<HTMLButtonElement>('button[type="submit"]')!;
  const problem = () => el.querySelector<HTMLElement>('[data-testid="problem"]');
  const count = () => el.querySelector('#story-count')!.textContent!.replace(/\s+/g, ' ').trim();
  const fieldError = () => el.querySelector('#story-error')?.textContent?.replace(/\s+/g, ' ').trim();

  async function write(value: string): Promise<void> {
    box().value = value;
    box().dispatchEvent(new Event('input'));
    await settle(fixture);
  }

  async function submit(): Promise<void> {
    el.querySelector('form')!.dispatchEvent(new Event('submit', { cancelable: true }));
    await settle(fixture);
  }

  describe('the form', () => {
    beforeEach(() => create());

    it('explains what will happen, using the limits from the API', () => {
      expect(el.querySelector('h1')!.textContent).toContain('Turn a story into a video plan');
      expect(el.textContent).toContain('2 to 10 scenes, up to 90 seconds in all');
      expect(service.getLimits).toHaveBeenCalledTimes(1);
    });

    it('has a labelled box for the story, with its hint and counter attached', () => {
      expect(el.querySelector('label[for="story"]')!.textContent).toContain('Your story');
      const described = box().getAttribute('aria-describedby')!.split(' ');
      expect(described).toEqual(['story-hint', 'story-count']);
      expect(el.querySelector('#story-hint')!.textContent).toContain('English or Hindi');
    });

    it('counts characters as the API does, not UTF-16 units', async () => {
      await write('🌊'.repeat(40));
      expect(count()).toContain('40 of 5,000 characters');
      await write('हिंदी कहानी');
      expect(count()).toContain('11 of 5,000 characters');
    });

    it('says how much more is needed while the story is short', async () => {
      await write('Too short.');
      expect(count()).toContain('10 of 5,000 characters · 20 more needed');
      await write(STORY);
      expect(count()).not.toContain('more needed');
    });

    it('tells people their story is not stored', () => {
      expect(el.textContent).toContain('We keep the plan, not the story');
      expect(el.textContent).toContain('20 plans an hour');
    });

    it('is kept out of search results', () => {
      expect(document.head.querySelector('meta[name="robots"]')!.getAttribute('content')).toContain('noindex');
    });
  });

  describe('limits', () => {
    it('uses the limits the API reports', async () => {
      service.getLimits.and.returnValue(of({ ...LIMITS, minStoryChars: 50, maxStoryChars: 800, perHour: 5 }));
      await create();
      expect(count()).toContain('of 800 characters');
      expect(el.textContent).toContain('5 plans an hour');
      await write(STORY.slice(0, 40));
      await submit();
      expect(fieldError()).toContain('Add at least 50 characters. Your story has 40.');
    });

    it('still works with the usual limits if the limits cannot be read', async () => {
      service.getLimits.and.returnValue(throwError(() => httpError(500, 'INTERNAL_ERROR')));
      await create();
      expect(count()).toContain('of 5,000 characters');
      expect(problem()).toBeNull();
      expect(box()).toBeTruthy();
    });

    it('shows no form, and says why, when the planner is switched off', async () => {
      service.getLimits.and.returnValue(of({ ...LIMITS, enabled: false }));
      await create();
      expect(box()).toBeNull();
      expect(submitButton()).toBeNull();
      expect(problem()!.textContent).toContain("Story planning isn't switched on yet");
      expect(problem()!.textContent).toContain('Nothing is wrong with your story');
    });
  });

  describe('checking the story before sending it', () => {
    beforeEach(() => create());

    it('says what is wrong when it is too short, puts focus there, and sends nothing', async () => {
      await write('Too short.');
      await submit();
      expect(fieldError()).toContain('Add at least 30 characters. Your story has 10.');
      expect(box().getAttribute('aria-invalid')).toBe('true');
      expect(box().getAttribute('aria-describedby')).toContain('story-error');
      expect(document.activeElement).toBe(box());
      expect(service.create).not.toHaveBeenCalled();
    });

    it('says what is wrong when it is empty', async () => {
      await submit();
      expect(fieldError()).toContain('Your story has 0');
      expect(service.create).not.toHaveBeenCalled();
    });

    it('says by how much it is too long', async () => {
      await write('a'.repeat(5003));
      await submit();
      expect(fieldError()).toContain('The limit is 5,000 characters. Yours has 5,003; shorten it by 3.');
      expect(service.create).not.toHaveBeenCalled();
    });

    it('does not count surrounding blanks', async () => {
      await write('   ' + 'a'.repeat(25) + '   ');
      await submit();
      expect(fieldError()).toContain('Your story has 25');
    });

    it('clears the message as soon as the story is edited', async () => {
      await write('Too short.');
      await submit();
      await write('Too short. Now a little longer.');
      expect(fieldError()).toBeUndefined();
      expect(box().getAttribute('aria-invalid')).toBeNull();
    });
  });

  describe('planning', () => {
    beforeEach(() => create());

    it('sends the trimmed story and goes to the plan', async () => {
      service.create.and.returnValue(of(planRecord({ planId: 'abc-123' })));
      await write(`  ${STORY}  `);
      await submit();
      expect(service.create).toHaveBeenCalledOnceWith(STORY);
      expect(navigate).toHaveBeenCalledOnceWith(['/story', 'abc-123']);
    });

    it('says it is working, how long that takes, and keeps the story while it waits', async () => {
      const pending = new Subject<StoryPlanRecord>();
      service.create.and.returnValue(pending);
      await write(STORY);
      await submit();
      expect(submitButton().disabled).toBeTrue();
      expect(submitButton().textContent).toContain('Planning…');
      const progress = el.querySelector('[data-testid="progress"]')!;
      expect(progress.getAttribute('role')).toBe('status');
      expect(progress.textContent).toContain('usually takes 5 to 10 seconds and can take up to 25');
      expect(box().readOnly).toBeTrue();
      expect(box().value).toBe(STORY);
      expect(navigate).not.toHaveBeenCalled();
    });

    it('does not send twice if submitted twice', async () => {
      service.create.and.returnValue(new Subject<StoryPlanRecord>());
      await write(STORY);
      await submit();
      await submit();
      expect(service.create).toHaveBeenCalledTimes(1);
    });
  });

  describe('when planning fails', () => {
    beforeEach(() => create());

    for (const f of API_FAILURES) {
      it(`${f.name}: says "${f.title}", keeps the story, and lets the user try again`, async () => {
        service.create.and.returnValue(throwError(() => f.error));
        await write(STORY);
        await submit();

        expect(problem()!.textContent).toContain(f.title);
        expect(problem()!.getAttribute('role')).toMatch(/alert|status/);
        expect(box().value).toBe(STORY);
        expect(box().readOnly).toBeFalse();
        expect(submitButton().disabled).toBeFalse();
        expect(navigate).not.toHaveBeenCalled();

        // And the next attempt works.
        service.create.and.returnValue(of(planRecord({ planId: 'second-try' })));
        await submit();
        expect(problem()).toBeNull();
        expect(navigate).toHaveBeenCalledOnceWith(['/story', 'second-try']);
      });
    }

    it('says when the limit is up and for how long', async () => {
      service.create.and.returnValue(throwError(() => httpError(429, 'RATE_LIMITED', '', { retryAfter: 1740 })));
      await write(STORY);
      await submit();
      expect(problem()!.textContent).toContain('Try again in 29 minutes.');
    });

    it('shows the API\'s own words when the story itself is the problem', async () => {
      service.create.and.returnValue(throwError(() => httpError(400, 'VALIDATION_ERROR', 'The story needs at least 30 characters.')));
      await write(STORY);
      await submit();
      expect(problem()!.textContent).toContain("That story can't be planned");
      expect(problem()!.textContent).toContain('The story needs at least 30 characters.');
    });

    it('says the feature is off when the server says so, and keeps the story', async () => {
      service.create.and.returnValue(throwError(() => httpError(503, 'UNAVAILABLE', 'Story planning is not available yet')));
      await write(STORY);
      await submit();
      expect(problem()!.textContent).toContain("Story planning isn't switched on yet");
      expect(box().value).toBe(STORY);
    });

    it('reassures the user on a timeout that the story is still there', async () => {
      service.create.and.returnValue(throwError(() => httpError(504, 'PLAN_TIMEOUT')));
      await write(STORY);
      await submit();
      expect(problem()!.textContent).toContain('Your story is still here');
    });
  });
});
