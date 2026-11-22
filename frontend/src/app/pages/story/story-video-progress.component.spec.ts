import { ComponentFixture, TestBed } from '@angular/core/testing';
import { STORY_ICONS, settle } from './story-plan-fixtures.spec';
import { StoryVideoProgressComponent } from './story-video-progress.component';
import { VideoStatus } from './story-video.models';

describe('StoryVideoProgressComponent', () => {
  let fixture: ComponentFixture<StoryVideoProgressComponent>;
  let el: HTMLElement;

  async function show(status: VideoStatus, progress: number): Promise<void> {
    if (!fixture) {
      await TestBed.configureTestingModule({ imports: [StoryVideoProgressComponent], providers: [STORY_ICONS] }).compileComponents();
      fixture = TestBed.createComponent(StoryVideoProgressComponent);
      el = fixture.nativeElement;
    }
    fixture.componentRef.setInput('status', status);
    fixture.componentRef.setInput('progress', progress);
    await settle(fixture);
  }

  beforeEach(() => (fixture = undefined as never));

  const steps = () => Array.from(el.querySelectorAll('ol li'));
  const states = () => steps().map((s) => s.getAttribute('data-state'));
  const bar = () => el.querySelector('progress')!;
  const doing = () => el.querySelector('[data-testid="doing"]')!.textContent;

  it('shows the percent on the bar and in words', async () => {
    await show('rendering', 62);
    expect(bar().value).toBe(62);
    expect(bar().max).toBe(100);
    expect(el.querySelector('[data-testid="percent"]')!.textContent).toBe('62%');
    expect(bar().getAttribute('aria-valuetext')).toBe('62 percent');
    expect(bar().getAttribute('aria-label')).toBe('Video progress');
  });

  it('keeps the percent between 0 and 100 and whole', async () => {
    await show('rendering', 140);
    expect(el.querySelector('[data-testid="percent"]')!.textContent).toBe('100%');
    await show('rendering', -4);
    expect(el.querySelector('[data-testid="percent"]')!.textContent).toBe('0%');
    await show('rendering', 33.6);
    expect(el.querySelector('[data-testid="percent"]')!.textContent).toBe('34%');
  });

  it('lists the three stages in the order they happen', async () => {
    await show('queued', 0);
    expect(steps().map((s) => s.querySelector('.lbl')!.textContent!.trim())).toEqual(['Pictures', 'Narration', 'Video']);
  });

  it('says it is starting up, and that this can take a minute, before the first stage and marks nothing in progress', async () => {
    await show('queued', 0);
    expect(el.querySelector('[data-testid="doing"]')!.textContent).toBe('Starting up. This can take up to a minute.');
    expect(states()).toEqual(['waiting', 'waiting', 'waiting']);
    expect(el.querySelector('[aria-current]')).toBeNull();
  });

  it('marks the stage in progress, the ones done, and the ones waiting', async () => {
    await show('narrating', 30);
    expect(states()).toEqual(['done', 'current', 'waiting']);
    expect(el.querySelector('[data-testid="doing"]')!.textContent).toBe('Recording the narration');
    expect(steps()[1].getAttribute('aria-current')).toBe('step');
    expect(el.querySelectorAll('[aria-current]').length).toBe(1);
  });

  it('says each state in words, not only with a marker', async () => {
    await show('narrating', 30);
    const parts = (s: Element) => Array.from(s.querySelectorAll('.lbl, .word')).map((p) => p.textContent!.trim());
    expect(steps().map(parts)).toEqual([
      ['Pictures', 'Done'],
      ['Narration', 'In progress'],
      ['Video', 'Waiting'],
    ]);
  });

  it('follows the job through all its stages', async () => {
    await show('preparing', 8);
    expect(states()).toEqual(['current', 'waiting', 'waiting']);
    expect(el.querySelector('[data-testid="doing"]')!.textContent).toBe('Getting your pictures ready');
    await show('rendering', 70);
    expect(states()).toEqual(['done', 'done', 'current']);
    expect(el.querySelector('[data-testid="doing"]')!.textContent).toBe('Putting the video together');
  });

  it('puts the accent on the stage in progress and nowhere else', async () => {
    await show('narrating', 30);
    const accent = (s: Element) => s.classList.contains('accent');
    expect(steps().map(accent)).toEqual([false, true, false]);
  });

  it('has nothing that loops', async () => {
    await show('narrating', 30);
    expect(el.innerHTML).not.toContain('animate-');
  });

  it('is announced politely, as a status', async () => {
    await show('narrating', 30);
    expect(el.querySelector('[data-testid="doing"]')!.getAttribute('role')).toBe('status');
  });
  describe('counting the pictures', () => {
    beforeEach(async () => {
      await show('preparing', 5);
      fixture.componentRef.setInput('sceneCount', 8);
      await settle(fixture);
    });
    it('says none are done at the start of the pictures stage', () => {
      expect(doing()).toBe('Getting your pictures ready: 0 of 8 done');
    });
    it('reads the count back from the progress the worker reports, for every number of pictures done', async () => {
      for (const total of [1, 2, 3, 8, 12, 20]) {
        fixture.componentRef.setInput('sceneCount', total);
        for (let done = 0; done <= total; done++) {
          // The worker's own formula: 5 + 20 * done / total, rounded down.
          fixture.componentRef.setInput('progress', 5 + Math.floor((20 * done) / total));
          await settle(fixture);
          expect(doing()).withContext(`${done} of ${total}`).toBe(`Getting your pictures ready: ${done} of ${total} done`);
        }
      }
    });
    it('never reads more pictures than there are, nor a high count with many scenes', async () => {
      fixture.componentRef.setInput('sceneCount', 30);
      for (let done = 0; done <= 30; done++) {
        fixture.componentRef.setInput('progress', 5 + Math.floor((20 * done) / 30));
        await settle(fixture);
        const n = Number(/: (\d+) of/.exec(doing()!)![1]);
        expect(n).withContext(`${done} of 30`).toBeLessThanOrEqual(done);
      }
    });
    it('shows no count when the number of scenes is not known', async () => {
      fixture.componentRef.setInput('sceneCount', 0);
      await settle(fixture);
      expect(doing()).toBe('Getting your pictures ready');
    });
    it('shows no count after the pictures stage', async () => {
      fixture.componentRef.setInput('status', 'narrating');
      fixture.componentRef.setInput('progress', 30);
      await settle(fixture);
      expect(doing()).toBe('Recording the narration');
    });
  });
  it('says how long a video usually takes, without a countdown and without an em dash', async () => {
    await show('preparing', 10);
    const t = el.querySelector('[data-testid="expect"]')!.textContent!;
    expect(t).toContain('Most videos take 2 to 3 minutes');
    expect(t).toContain('longer when the photo library is slow');
    expect(el.textContent).not.toContain('\u2014');
  });
});
