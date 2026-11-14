import { Component, signal, viewChild } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FIT_STEPS, FitResult, InfographicCanvasComponent } from './infographic-canvas.component';
import { InfographicDoc, MARK_TEXT } from './infographic.model';
import { TEMPLATES, newDoc, templateById } from './templates';

@Component({
  standalone: true,
  imports: [InfographicCanvasComponent],
  template: `<div style="width: 600px"><app-infographic-canvas [doc]="doc()" (fitted)="results.push($event)" /></div>`,
})
class HostComponent {
  doc = signal<InfographicDoc>(newDoc(TEMPLATES[0]));
  results: FitResult[] = [];
  canvas = viewChild.required(InfographicCanvasComponent);
}

describe('InfographicCanvasComponent', () => {
  let fixture: ComponentFixture<HostComponent>;
  let host: HostComponent;
  const stage = () => fixture.nativeElement.querySelector('[data-testid="artboard"]') as HTMLElement;

  async function show(doc: InfographicDoc): Promise<FitResult> {
    host.doc.set(doc);
    fixture.detectChanges();
    await fixture.whenStable();
    return host.canvas().fit();
  }

  beforeEach(() => {
    TestBed.configureTestingModule({ imports: [HostComponent] });
    fixture = TestBed.createComponent(HostComponent);
    host = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('draws the title and every section', async () => {
    const d = newDoc(TEMPLATES[0]);
    await show(d);
    expect(stage().querySelector('.title')!.textContent).toContain(d.title);
    expect(stage().querySelectorAll('h2').length).toBe(d.sections.length);
  });

  it('has the real pixel size of the format', async () => {
    for (const [format, w, h] of [['portrait', 1080, 1920], ['square', 1080, 1080], ['landscape', 1920, 1080]] as const) {
      await show({ ...newDoc(TEMPLATES[0]), format });
      expect(stage().style.width).toBe(`${w}px`);
      expect(stage().style.height).toBe(`${h}px`);
    }
  });

  it('draws every template without overflow at every size using its own text', async () => {
    for (const t of TEMPLATES) {
      for (const format of ['portrait', 'square', 'landscape'] as const) {
        const r = await show({ ...newDoc(t), format });
        expect(r.overflowPx).withContext(`${t.id} ${format}`).toBe(0);
        expect(FIT_STEPS).toContain(r.k);
      }
    }
  });

  it('shrinks crowded text and reports it', async () => {
    const t = templateById('concept')!;
    const base = newDoc(t);
    const crowded: InfographicDoc = {
      ...base,
      sections: Array.from({ length: t.maxSections }, (_, i) => ({ id: 's' + i, label: '', heading: 'Heading ' + i, body: 'word '.repeat(70) })),
    };
    const r = await show({ ...crowded, format: 'square' });
    expect(r.k).toBeLessThan(1);
  });

  it('warns by reporting overflow when text cannot fit even at the smallest scale', async () => {
    const t = templateById('steps')!;
    const base = newDoc(t);
    const huge: InfographicDoc = {
      ...base,
      format: 'landscape',
      sections: Array.from({ length: t.maxSections }, (_, i) => ({ id: 's' + i, label: '', heading: 'H' + i, body: 'word '.repeat(80) })),
    };
    const r = await show(huge);
    expect(r.k).toBe(FIT_STEPS[FIT_STEPS.length - 1]);
    expect(r.overflowPx).toBeGreaterThan(0);
  });

  it('lets a very long word break instead of running off the side', async () => {
    const base = newDoc(TEMPLATES[0]);
    const r = await show({ ...base, title: 'W'.repeat(100) });
    expect(r.overflowPx).toBe(0);
  });

  it('detects the language from the text', async () => {
    await show({ ...newDoc(TEMPLATES[0]), title: 'कहानी से वीडियो' });
    expect(stage().getAttribute('lang')).toBe('hi');
    await show({ ...newDoc(TEMPLATES[0]), title: 'વાર્તા' , subtitle: '', sections: newDoc(TEMPLATES[0]).sections.map((s) => ({ ...s, heading: 'ક', body: '', label: '' })) });
    expect(stage().getAttribute('lang')).toBe('gu');
  });

  it('uses the chosen language over the detected one', async () => {
    await show({ ...newDoc(TEMPLATES[0]), language: 'mr' });
    expect(stage().getAttribute('lang')).toBe('mr');
  });

  it('mirrors for Arabic text and follows an explicit direction', async () => {
    await show({ ...newDoc(TEMPLATES[0]), title: 'مرحبا بالعالم' });
    expect(stage().getAttribute('dir')).toBe('rtl');
    await show({ ...newDoc(TEMPLATES[0]), direction: 'rtl' });
    expect(stage().getAttribute('dir')).toBe('rtl');
    await show({ ...newDoc(TEMPLATES[0]), title: 'مرحبا', direction: 'ltr' });
    expect(stage().getAttribute('dir')).toBe('ltr');
    await show(newDoc(TEMPLATES[0]));
    expect(stage().getAttribute('dir')).toBe('ltr');
  });

  it('uses taller lines for Indic scripts', async () => {
    await show({ ...newDoc(TEMPLATES[0]), title: 'कहानी' });
    expect(stage().style.getPropertyValue('--lh')).toBe('1.75');
    await show(newDoc(TEMPLATES[0]));
    expect(stage().style.getPropertyValue('--lh')).toBe('1.45');
  });

  it('shows the mark only when it is turned on', async () => {
    await show(newDoc(TEMPLATES[0]));
    expect(stage().textContent).not.toContain(MARK_TEXT);
    await show({ ...newDoc(TEMPLATES[0]), mark: true });
    expect(stage().textContent).toContain(MARK_TEXT);
  });

  it('paints the colours of the chosen theme', async () => {
    await show({ ...newDoc(TEMPLATES[0]), themeId: 'ocean' });
    expect(stage().style.getPropertyValue('--bg').toLowerCase()).toBe('#eef5f8');
  });

  it('treats typed markup as text', async () => {
    await show({ ...newDoc(TEMPLATES[0]), title: '<img src=x onerror=alert(1)>' });
    expect(stage().querySelector('img')).toBeNull();
    expect(stage().querySelector('.title')!.textContent).toContain('<img');
  });

  it('draws bullets for the compare layout from the lines of the body', async () => {
    const t = templateById('compare')!;
    const d = newDoc(t);
    d.sections[0].body = 'one\ntwo\n\nthree';
    await show(d);
    expect(stage().querySelectorAll('li').length).toBeGreaterThanOrEqual(3);
  });
});
