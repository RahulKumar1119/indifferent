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
    expect(steps().map((s) => s.querySelector('span.font-medium')!.textContent!.trim())).toEqual(['Pictures', 'Narration', 'Video']);
  });

  it('says "Starting" before the first stage and marks nothing in progress', async () => {
    await show('queued', 0);
    expect(el.querySelector('[data-testid="doing"]')!.textContent).toBe('Starting');
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
    const parts = (s: Element) => Array.from(s.querySelectorAll('span.block:not(.size-2\\.5)')).map((p) => p.textContent!.trim());
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
    const accent = (s: Element) => s.className.includes('#BC5227') || !!s.querySelector('[class*="#BC5227"]');
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
});
