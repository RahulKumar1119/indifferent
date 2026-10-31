import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Subject, of, throwError } from 'rxjs';
import { StoryMediaPickerComponent } from './story-media-picker.component';
import { PlanScene, StockAsset, StockSearchResponse } from './story-plan.models';
import { StoryPlanService } from './story-plan.service';
import {
  API_FAILURES,
  ServiceSpy,
  STORY_ICONS,
  asset,
  httpError,
  planRecord,
  searchResponse,
  serviceSpy,
  settle,
} from './story-plan-fixtures.spec';

describe('StoryMediaPickerComponent', () => {
  let service: ServiceSpy;
  let fixture: ComponentFixture<StoryMediaPickerComponent>;
  let picker: StoryMediaPickerComponent;
  let el: HTMLElement;
  const scenes = planRecord().plan!.scenes;
  const photoScene: PlanScene = scenes[0];
  const clipScene: PlanScene = scenes[1];

  beforeEach(async () => {
    service = serviceSpy();
    service.search.and.returnValue(of(searchResponse()));
    await TestBed.configureTestingModule({
      imports: [StoryMediaPickerComponent],
      providers: [{ provide: StoryPlanService, useValue: service }, STORY_ICONS],
    }).compileComponents();
    fixture = TestBed.createComponent(StoryMediaPickerComponent);
    picker = fixture.componentInstance;
    el = fixture.nativeElement;
    fixture.detectChanges();
  });

  afterEach(() => el.querySelector('dialog')?.close());

  async function openFor(scene: PlanScene = photoScene, current = scene.media ?? null): Promise<void> {
    picker.open(scene, 1, current);
    await settle(fixture);
  }

  const text = () => el.textContent!.replace(/\s+/g, ' ');
  const buttons = () => Array.from(el.querySelectorAll('button'));
  const button = (label: RegExp) => buttons().find((b) => label.test(b.textContent!))!;
  const resultsText = () => el.querySelector('[data-testid="results"]')?.textContent ?? '';
  const problem = () => el.querySelector<HTMLElement>('[data-testid="problem"]');

  it('opens as a modal dialog and searches with the scene\'s three strongest keywords', async () => {
    await openFor();
    const dialog = el.querySelector('dialog')!;
    expect(dialog.open).toBeTrue();
    expect(service.search).toHaveBeenCalledOnceWith({ q: 'lighthouse cliff storm', type: 'photo', page: 1, perPage: 12 });
    expect((el.querySelector('#picker-query') as HTMLInputElement).value).toBe('lighthouse cliff storm');
    expect(dialog.getAttribute('aria-labelledby')).toBe('picker-title');
    expect(el.querySelector('#picker-title')!.textContent).toContain('Choose a photo for scene 1');
  });

  it('looks for clips first when the scene prefers a clip', async () => {
    await openFor(clipScene);
    expect(service.search.calls.mostRecent().args[0]).toEqual({ q: 'spiral staircase lamp', type: 'video', page: 1, perPage: 12 });
    expect(el.querySelector('#picker-title')!.textContent).toContain('Choose a clip');
    expect(button(/Clips/).getAttribute('aria-pressed')).toBe('true');
    expect(button(/Photos/).getAttribute('aria-pressed')).toBe('false');
  });

  it('says it is searching while it waits, without showing stale results', async () => {
    const pending = new Subject<StockSearchResponse>();
    service.search.and.returnValue(pending);
    await openFor();
    expect(el.querySelector('[data-testid="picker-status"]')!.textContent).toContain('Searching');
    expect(el.querySelector('[data-testid="results"]')).toBeNull();
    pending.next(searchResponse());
    await settle(fixture);
    expect(el.querySelector('[data-testid="results"]')).not.toBeNull();
  });

  it('shows each result with a preview, its credit and a button to use it', async () => {
    await openFor();
    const items = el.querySelectorAll('[data-testid="results"] > li');
    expect(items.length).toBe(3);
    const img = items[0].querySelector('img')!;
    expect(img.getAttribute('src')).toBe('https://cdn.example/pixabay/301.jpg');
    expect(img.getAttribute('alt')).toBe('A picture numbered 301');
    const credit = items[0].querySelector('a')!;
    expect(credit.textContent).toContain('Photo 301 by Jo Doe on Pixabay');
    expect(credit.getAttribute('href')).toBe('https://pixabay.example/301');
    expect(credit.getAttribute('rel')).toContain('noopener');
    expect(items[0].querySelector('button')!.textContent).toContain('Use this photo');
    expect(el.querySelector('[data-testid="picker-status"]')!.textContent).toContain('3 photos for “lighthouse cliff storm”');
  });

  it('credits the providers whenever results are shown', async () => {
    await openFor();
    const links = Array.from(el.querySelectorAll('[data-testid="attribution"] a')).map((a) => a.getAttribute('href'));
    expect(links).toEqual(['https://www.pexels.com', 'https://pixabay.com']);
    expect(text()).toContain('Photos provided by Pexels');
    expect(text()).toContain('Photos provided by Pixabay');
  });

  it('plays a clip with the browser\'s own controls, and does not start it by itself', async () => {
    service.search.and.returnValue(of(searchResponse({ assets: [asset({ id: '9', type: 'video' })] })));
    await openFor(clipScene);
    const video = el.querySelector('[data-testid="results"] video')!;
    expect(video.hasAttribute('controls')).toBeTrue();
    expect(video.hasAttribute('autoplay')).toBeFalse();
    expect(video.getAttribute('preload')).toBe('none');
    expect(video.getAttribute('src')).toBe('https://cdn.example/pixabay/9.mp4');
    expect(video.getAttribute('aria-label')).toContain('Preview of');
    expect(el.querySelector('[data-testid="results"] button')!.textContent).toContain('Use this clip');
  });

  it('never shows or links the full-resolution file, only the preview', async () => {
    const full = { ...asset({ id: '5' }), mediaUrl: 'https://full.example/5-original.jpg' } as StockAsset;
    service.search.and.returnValue(of(searchResponse({ assets: [full] })));
    await openFor();
    expect(el.innerHTML).not.toContain('5-original');
  });

  describe('choosing', () => {
    it('hands the asset back and closes', async () => {
      const picked: StockAsset[] = [];
      picker.picked.subscribe((a) => picked.push(a));
      await openFor();
      el.querySelectorAll<HTMLButtonElement>('[data-testid="results"] > li button')[1].click();
      await settle(fixture);
      expect(picked.map((a) => a.id)).toEqual(['302']);
      expect(el.querySelector('dialog')!.open).toBeFalse();
    });

    it('marks the current choice with words and gives no button to pick it again', async () => {
      service.search.and.returnValue(of(searchResponse({ assets: [asset({ id: '101' }), asset({ id: '302' })] })));
      await openFor(photoScene, { provider: 'pixabay', type: 'photo', id: '101' });
      const items = el.querySelectorAll('[data-testid="results"] > li');
      expect(items[0].textContent).toContain('Current choice');
      expect(items[0].querySelector('button')).toBeNull();
      expect(items[1].querySelector('button')).not.toBeNull();
    });

    it('does not mix up the same id from the other library', async () => {
      service.search.and.returnValue(of(searchResponse({ assets: [asset({ id: '101', provider: 'pexels' })] })));
      await openFor(photoScene, { provider: 'pixabay', type: 'photo', id: '101' });
      expect(resultsText()).not.toContain('Current choice');
    });

    it('closes without choosing from the close button', async () => {
      const picked = jasmine.createSpy('picked');
      picker.picked.subscribe(picked);
      await openFor();
      el.querySelector<HTMLButtonElement>('button[aria-label="Close"]')!.click();
      expect(el.querySelector('dialog')!.open).toBeFalse();
      expect(picked).not.toHaveBeenCalled();
    });
  });

  describe('searching again', () => {
    it('searches for clips when the type is switched', async () => {
      await openFor();
      button(/Clips/).click();
      await settle(fixture);
      expect(service.search.calls.mostRecent().args[0]).toEqual({ q: 'lighthouse cliff storm', type: 'video', page: 1, perPage: 12 });
      expect(button(/Clips/).getAttribute('aria-pressed')).toBe('true');
    });

    it('runs the words that were typed, from the keyboard (Enter submits the form)', async () => {
      await openFor();
      const input = el.querySelector<HTMLInputElement>('#picker-query')!;
      input.value = '  old harbour  ';
      input.dispatchEvent(new Event('input'));
      el.querySelector('form')!.dispatchEvent(new Event('submit', { cancelable: true }));
      await settle(fixture);
      expect(service.search.calls.mostRecent().args[0]).toEqual({ q: 'old harbour', type: 'photo', page: 1, perPage: 12 });
    });

    it('does not search for nothing, and says what to type', async () => {
      await openFor();
      service.search.calls.reset();
      const input = el.querySelector<HTMLInputElement>('#picker-query')!;
      input.value = '   ';
      input.dispatchEvent(new Event('input'));
      el.querySelector('form')!.dispatchEvent(new Event('submit', { cancelable: true }));
      await settle(fixture);
      expect(service.search).not.toHaveBeenCalled();
      const error = el.querySelector('#picker-query-error')!;
      expect(error.textContent).toContain('Type a word to search for, in English');
      expect(input.getAttribute('aria-invalid')).toBe('true');
      expect(input.getAttribute('aria-describedby')).toBe('picker-query-error');
    });

    it('drops the answer to an old search when a newer one is made', async () => {
      const first = new Subject<StockSearchResponse>();
      service.search.and.returnValues(first, of(searchResponse({ assets: [asset({ id: '777' })] })));
      await openFor();
      button(/Clips/).click();
      await settle(fixture);
      first.next(searchResponse({ assets: [asset({ id: '111' })] })); // arrives late
      await settle(fixture);
      expect(resultsText()).toContain('777');
      expect(resultsText()).not.toContain('111');
    });
  });

  describe('no results', () => {
    beforeEach(() => service.search.and.returnValue(of(searchResponse({ assets: [], attributions: [] }))));

    it('names what was searched and what to try next', async () => {
      await openFor();
      const empty = el.querySelector('[data-testid="empty"]')!;
      expect(empty.textContent).toContain('No photos found for “lighthouse cliff storm”');
      expect(empty.textContent).toContain('Try fewer or different English words');
      expect(problem()).toBeNull();
    });

    it('offers the other kind of media in one press', async () => {
      await openFor();
      button(/Search clips instead/).click();
      await settle(fixture);
      expect(service.search.calls.mostRecent().args[0].type).toBe('video');
    });
  });

  describe('when the photo libraries fail', () => {
    for (const f of API_FAILURES.filter((x) => x.name !== 'rate limited')) {
      it(`${f.name}: says so, shows no results, and offers to search again`, async () => {
        service.search.and.returnValue(throwError(() => f.error));
        await openFor();
        expect(problem()).not.toBeNull();
        expect(el.querySelector('[data-testid="results"]')).toBeNull();
        expect(el.querySelector('[data-testid="empty"]')).toBeNull(); // a failure is not "no results"
        expect(text()).not.toContain('No photos found');
      });
    }

    const cases: [string, ReturnType<typeof httpError>, string, boolean][] = [
      ['you are searching too fast', httpError(429, 'RATE_LIMITED', '', { retryAfter: 20 }), 'Searching too fast', true],
      ['the libraries are rate limited', httpError(429, 'PROVIDER_RATE_LIMITED', '', { retryAfter: 60 }), 'The photo libraries are busy', true],
      ['search is not switched on', httpError(503, 'UNAVAILABLE'), "Picture search isn't switched on yet", false],
      ['the libraries are slow', httpError(504, 'SEARCH_TIMEOUT'), 'The photo libraries took too long', true],
      ['the libraries are unreachable', httpError(502, 'SEARCH_FAILED'), "Couldn't reach the photo libraries", true],
      ['you are offline', httpError(0), "You're offline", true],
      ['the words are Hindi', httpError(400, 'VALIDATION_ERROR', 'Search in English: try "mandir".'), "That search can't be run", false],
    ];
    for (const [name, error, title, retry] of cases) {
      it(`${name}: "${title}"`, async () => {
        service.search.and.returnValue(throwError(() => error));
        await openFor();
        const p = problem()!;
        expect(p.textContent).toContain(title);
        expect(!!button(/Search again/)).toBe(retry);
        expect(el.querySelector('[data-testid="results"]')).toBeNull();
      });
    }

    it('says how long to wait on a rate limit', async () => {
      service.search.and.returnValue(throwError(() => httpError(429, 'PROVIDER_RATE_LIMITED', '', { retryAfter: 60 })));
      await openFor();
      expect(problem()!.textContent).toContain('1 minute');
    });

    it('shows the API\'s own words for an input problem, such as the English-only rule', async () => {
      service.search.and.returnValue(throwError(() => httpError(400, 'VALIDATION_ERROR', 'Search in English: try "mandir".')));
      await openFor();
      expect(problem()!.textContent).toContain('Search in English: try "mandir".');
    });

    it('searches again when asked, and recovers', async () => {
      service.search.and.returnValues(throwError(() => httpError(504, 'SEARCH_TIMEOUT')), of(searchResponse()));
      await openFor();
      button(/Search again/).click();
      await settle(fixture);
      expect(problem()).toBeNull();
      expect(el.querySelectorAll('[data-testid="results"] > li').length).toBe(3);
    });

    it('treats 200 with nothing but failures as a failure, not as no results', async () => {
      service.search.and.returnValue(
        of(searchResponse({ assets: [], failures: [{ provider: 'pixabay', reason: 'rate_limited', retryAfter: 90 }] })),
      );
      await openFor();
      expect(problem()!.textContent).toContain('The photo libraries are busy');
      expect(problem()!.textContent).toContain('2 minutes');
      expect(el.querySelector('[data-testid="empty"]')).toBeNull();
    });

    it('tells which library did not answer when the other one did', async () => {
      service.search.and.returnValue(
        of(searchResponse({ failures: [{ provider: 'pexels', reason: 'rate_limited', retryAfter: 45 }] })),
      );
      await openFor();
      const notice = el.querySelector('[data-testid="partial-notice"]')!;
      expect(notice.textContent).toContain('Pexels is busy (try again in 45 seconds)');
      expect(notice.textContent).toContain('other library only');
      expect(el.querySelectorAll('[data-testid="results"] > li').length).toBe(3);
    });
  });

  describe('more results', () => {
    it('adds the next page below the first, without repeats', async () => {
      service.search.and.returnValues(
        of(searchResponse({ assets: [asset({ id: '1' }), asset({ id: '2' })], hasMore: true })),
        of(searchResponse({ assets: [asset({ id: '2' }), asset({ id: '3' })], hasMore: false, page: 2 })),
      );
      await openFor();
      button(/Show more results/).click();
      await settle(fixture);
      expect(service.search.calls.mostRecent().args[0]).toEqual({ q: 'lighthouse cliff storm', type: 'photo', page: 2, perPage: 12 });
      expect(el.querySelectorAll('[data-testid="results"] > li').length).toBe(3);
      expect(buttons().some((b) => /Show more/.test(b.textContent!))).toBeFalse();
    });

    it('keeps the results already shown when the next page fails, and says why', async () => {
      service.search.and.returnValues(
        of(searchResponse({ assets: [asset({ id: '1' })], hasMore: true })),
        throwError(() => httpError(429, 'PROVIDER_RATE_LIMITED', '', { retryAfter: 30 })),
      );
      await openFor();
      button(/Show more results/).click();
      await settle(fixture);
      expect(el.querySelectorAll('[data-testid="results"] > li').length).toBe(1);
      expect(problem()!.textContent).toContain('30 seconds');
    });
  });
});
