import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CONSENT_STORAGE } from '../../../core/consent/consent.service';
import { MemoryStorage } from '../../../core/consent/testing';
import { CaptionStyleValue, NEW_JOB_CAPTION_STYLE } from '../../caption-style';
import { CaptionStudioComponent } from './caption-studio.component';

describe('CaptionStudioComponent', () => {
  let fixture: ComponentFixture<CaptionStudioComponent>;
  let el: HTMLElement;
  let emitted: CaptionStyleValue[];

  async function create(value: CaptionStyleValue = NEW_JOB_CAPTION_STYLE, inputs: Record<string, unknown> = {}): Promise<void> {
    await TestBed.configureTestingModule({
      imports: [CaptionStudioComponent],
      providers: [{ provide: CONSENT_STORAGE, useValue: new MemoryStorage() }],
    }).compileComponents();
    fixture = TestBed.createComponent(CaptionStudioComponent);
    fixture.componentRef.setInput('value', value);
    for (const [k, v] of Object.entries(inputs)) fixture.componentRef.setInput(k, v);
    emitted = [];
    fixture.componentInstance.valueChange.subscribe((v) => emitted.push(v));
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
  }
  const q = <T extends Element>(sel: string) => el.querySelector<T>(sel)!;
  const text = () => q('[data-testid="caption-preview-text"]').textContent!.trim();

  it('offers the four looks, the four fonts, the three positions and the safe-zone box', async () => {
    await create();
    expect(Array.from(el.querySelectorAll<HTMLInputElement>('input[name="cs-preset"]')).map((r) => r.value).sort()).toEqual(['bold', 'clean', 'karaoke', 'minimal']);
    expect(Array.from(q<HTMLSelectElement>('select').options).map((o) => o.textContent!.trim())).toEqual(['Default', 'Sans', 'Serif', 'Condensed']);
    expect(Array.from(el.querySelectorAll<HTMLInputElement>('input[name="cs-pos"]')).map((r) => r.value)).toEqual(['top', 'middle', 'bottom']);
    expect(q<HTMLInputElement>('input[type="checkbox"]').checked).toBeTrue();
    expect(el.textContent).toContain('Above platform controls');
  });

  it('reports each change as a whole style', async () => {
    await create();
    q<HTMLInputElement>('input[name="cs-preset"][value="clean"]').click();
    expect(emitted.at(-1)).toEqual({ ...NEW_JOB_CAPTION_STYLE, preset: 'clean' });
    const sel = q<HTMLSelectElement>('select');
    sel.value = 'condensed';
    sel.dispatchEvent(new Event('change'));
    expect(emitted.at(-1)!.font).toBe('condensed');
    q<HTMLInputElement>('input[type="checkbox"]').click();
    expect(emitted.at(-1)!.aboveControls).toBeFalse();
    q<HTMLInputElement>('input[name="cs-pos"][value="top"]').click();
    expect(emitted.at(-1)!.position).toBe('top');
    const size = q<HTMLInputElement>('input[type="range"]');
    size.value = '40';
    size.dispatchEvent(new Event('input'));
    expect(emitted.at(-1)!.fontSize).toBe(40);
  });

  it('has a size range of 8 to 72', async () => {
    await create();
    const size = q<HTMLInputElement>('input[type="range"]');
    expect([size.min, size.max]).toEqual(['8', '72']);
  });

  it('draws the preview from the style: size as a share of the frame, colour, font, bottom lift', async () => {
    await create({ ...NEW_JOB_CAPTION_STYLE, preset: 'minimal', fontSize: 30, color: '#112233', font: 'serif', aboveControls: true });
    const frame = q<HTMLElement>('[data-testid="caption-preview"]');
    expect(frame.style.getPropertyValue('--cs')).toBe('30');
    const cap = q<HTMLElement>('[data-testid="caption-preview-text"]');
    expect(cap.style.color).toBe('rgb(17, 34, 51)');
    expect(cap.style.fontFamily).toContain('Serif');
    expect(parseFloat(cap.style.bottom)).toBeGreaterThan(15);
  });

  it('moves the caption with the position', async () => {
    await create({ ...NEW_JOB_CAPTION_STYLE, position: 'top' });
    expect(q<HTMLElement>('[data-testid="caption-preview-text"]').style.top).toBe('6%');
  });

  it('shows a sample in the script of the language, or the real words when given', async () => {
    await create(NEW_JOB_CAPTION_STYLE, { language: 'hi' });
    expect(/[\u0900-\u097F]/.test(text())).toBeTrue();
    fixture.componentRef.setInput('language', 'en');
    fixture.componentRef.setInput('sampleText', ' hello   there ');
    fixture.detectChanges();
    expect(text()).toBe('hello there');
  });

  it('labels the preview as approximate for screen readers and sighted users', async () => {
    await create();
    expect(q('[data-testid="caption-preview"]').getAttribute('aria-label')).toContain('Approximate preview');
    expect(el.textContent).toContain('Approximate.');
  });

  it('says the style is not remembered without Functional, and links to cookie settings', async () => {
    await create();
    expect(q('[data-testid="remember-note"]').textContent).toContain('will not be remembered');
    expect(q('[data-testid="remember-note"] a')).toBeTruthy();
  });

  describe('karaoke', () => {
    const karaoke = { ...NEW_JOB_CAPTION_STYLE, preset: 'karaoke' as const };
    it('lights words in the highlight colour and offers a pause', async () => {
      await create({ ...karaoke, highlight: '#ff0000', color: '#00ff00' }, { sampleText: 'one two three four' });
      const words = Array.from(el.querySelectorAll<HTMLElement>('.cap .t'));
      expect(words.length).toBe(4);
      const colours = new Set(words.map((w) => w.style.color));
      expect(Array.from(colours).every((c) => c === 'rgb(255, 0, 0)' || c === 'rgb(0, 255, 0)')).toBeTrue();
      const matches = typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;
      expect(!!el.querySelector('[data-testid="preview-pause"]')).toBe(!matches);
    });
    it('does not offer karaoke controls for other looks', async () => {
      await create();
      expect(el.querySelector('[data-testid="preview-pause"]')).toBeNull();
      expect(el.querySelector('input[aria-label="Karaoke highlight colour"]')).toBeNull();
    });
  });

  it('cannot be changed while disabled', async () => {
    await create(NEW_JOB_CAPTION_STYLE, { disabled: true });
    for (const f of Array.from(el.querySelectorAll('fieldset'))) expect((f as HTMLFieldSetElement).disabled).toBeTrue();
    expect(q<HTMLSelectElement>('select').disabled).toBeTrue();
    expect(q<HTMLInputElement>('input[type="checkbox"]').disabled).toBeTrue();
  });

  it('gives every option a tap target of at least 44 pixels', async () => {
    await create();
    for (const r of Array.from(el.querySelectorAll<HTMLInputElement>('input[type="radio"]'))) {
      expect(r.closest('label')!.getBoundingClientRect().height).withContext(r.value).toBeGreaterThanOrEqual(44);
    }
  });

  it('uses no em dash', async () => {
    await create();
    expect(el.textContent).not.toContain('\u2014');
  });
});
