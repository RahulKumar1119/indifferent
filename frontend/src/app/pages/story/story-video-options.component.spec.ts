import { ComponentFixture, TestBed } from '@angular/core/testing';
import { StoryVideoOptionsComponent, VideoOptions } from './story-video-options.component';
import { settle } from './story-plan-fixtures.spec';
import { CaptionPresetId, NEW_JOB_CAPTION_STYLE } from '../../shared/caption-style';
const look = (preset: CaptionPresetId) => ({ ...NEW_JOB_CAPTION_STYLE, preset });

describe('StoryVideoOptionsComponent', () => {
  let fixture: ComponentFixture<StoryVideoOptionsComponent>;
  let el: HTMLElement;
  let emitted: VideoOptions[];

  async function create(language: string, voices: string[], value: VideoOptions, disabled = false): Promise<void> {
    await TestBed.configureTestingModule({ imports: [StoryVideoOptionsComponent] }).compileComponents();
    fixture = TestBed.createComponent(StoryVideoOptionsComponent);
    fixture.componentRef.setInput('language', language);
    fixture.componentRef.setInput('voices', voices);
    fixture.componentRef.setInput('value', value);
    fixture.componentRef.setInput('disabled', disabled);
    emitted = [];
    fixture.componentInstance.valueChange.subscribe((v) => emitted.push(v));
    el = fixture.nativeElement;
    await settle(fixture);
  }

  const english = ['Joanna', 'Matthew', 'Amy', 'Brian', 'Aditi'];
  const select = () => el.querySelector<HTMLSelectElement>('#video-voice')!;
  const radios = () => Array.from(el.querySelectorAll<HTMLInputElement>('input[type="radio"]')).filter((r) => r.name.startsWith('video-cap-preset'));

  describe('for a story that has a choice of voices', () => {
    beforeEach(() => create('en', english, { voice: 'Matthew', caption: look('karaoke') }));

    it('offers every voice the API lists, in its order, with the current one chosen', () => {
      expect(Array.from(select().options).map((o) => o.value)).toEqual(english);
      expect(select().value).toBe('Matthew');
    });

    it('labels the menu and says what the voice is like', () => {
      const label = el.querySelector(`label[for="${select().id}"]`)!;
      expect(label.textContent).toContain('Voice');
      expect(el.querySelector('[data-testid="voice-note"]')!.textContent).toContain('US English, male, warm and authoritative');
      expect(select().getAttribute('aria-describedby')).toContain('video-voice-note');
    });

    it('reports a new voice and keeps the caption look', () => {
      select().value = 'Amy';
      select().dispatchEvent(new Event('change'));
      expect(emitted).toEqual([{ voice: 'Amy', caption: look('karaoke') }]);
    });

    it('offers the four caption looks as radio buttons in one group, each with its description', () => {
      expect(radios().map((r) => r.value).sort()).toEqual(['bold', 'clean', 'karaoke', 'minimal']);
      expect(new Set(radios().map((r) => r.name)).size).toBe(1);
      expect(el.querySelector('legend')!.textContent).toContain('Caption look');
      expect(el.textContent).toContain('The spoken word lights up as it is said');
    });

    it('marks the current look and only that one', () => {
      expect(radios().filter((r) => r.checked).map((r) => r.value)).toEqual(['karaoke']);
    });

    it('reports a new caption look and keeps the voice', () => {
      radios().find((r) => r.value === 'clean')!.click();
      expect(emitted).toEqual([{ voice: 'Matthew', caption: look('clean') }]);
    });

    it('can be chosen by the label text, not only the dot', () => {
      for (const r of radios()) expect(r.closest('label')).withContext(r.value).toBeTruthy();
    });

    it('gives every control a tap target of at least 44 pixels', () => {
      // Measured, not read from a class name: ui.css and the component set the 44px minimum (#253).
      expect(select().getBoundingClientRect().height).toBeGreaterThanOrEqual(44);
      for (const r of radios()) expect(r.closest('label')!.getBoundingClientRect().height).toBeGreaterThanOrEqual(44);
    });
  });

  describe('for a story that has one voice', () => {
    beforeEach(() => create('hi', ['Aditi'], { voice: 'Aditi', caption: look('bold') }));

    it('states the voice instead of offering a menu with one entry', () => {
      expect(el.querySelector('#video-voice')).toBeNull();
      expect(el.querySelector('[data-testid="voice-fixed"]')!.textContent).toContain('Aditi');
      expect(el.querySelector('[data-testid="voice-note"]')!.textContent).toContain('Hindi narration has one voice, so there is nothing to choose.');
    });

    it('still lets the captions be chosen', () => {
      radios().find((r) => r.value === 'minimal')!.click();
      expect(emitted).toEqual([{ voice: 'Aditi', caption: look('minimal') }]);
    });
  });

  describe('while a video is being started', () => {
    beforeEach(() => create('en', english, { voice: 'Joanna', caption: look('bold') }, true));

    it('cannot be changed', () => {
      expect(select().disabled).toBeTrue();
      expect(el.querySelector('fieldset')!.disabled).toBeTrue();
      for (const r of radios()) expect(r.matches(':disabled')).toBeTrue();
    });
  });

  it('uses no em dash', async () => {
    await create('en', english, { voice: 'Joanna', caption: look('bold') });
    expect(el.textContent).not.toContain('\u2014');
  });
});
