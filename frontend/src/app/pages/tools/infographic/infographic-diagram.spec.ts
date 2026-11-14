import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FIT_STEPS, FitResult, InfographicCanvasComponent } from './infographic-canvas.component';
import { DIAGRAM_LAYOUTS, DiagramLayout, InfographicDiagramComponent, LIST_LAYOUTS, ListLayout, MAP_LAYOUTS, MapLayout } from './infographic-diagram.component';
import { InfographicMapComponent, cycleArrows, cyclePositions } from './infographic-map.component';
import { InfographicDoc } from './infographic.model';
import { TEMPLATES, newDoc, templateById } from './templates';

const T = (id: string) => templateById(id)!;

describe('the twenty templates', () => {
  it('are twenty, in the planned categories', () => {
    expect(TEMPLATES.length).toBe(20);
    const count = (c: string) => TEMPLATES.filter((t) => t.category === c).length;
    expect({
      educational: count('Educational'),
      statistics: count('Statistics'),
      steps: count('Step by step'),
      comparison: count('Comparison'),
      timeline: count('Timeline'),
      process: count('Process'),
      mind: count('Mind map'),
    }).toEqual({ educational: 3, statistics: 4, steps: 3, comparison: 3, timeline: 3, process: 2, mind: 2 });
  });

  it('have unique ids and layouts the artboard can draw', () => {
    expect(new Set(TEMPLATES.map((t) => t.id)).size).toBe(20);
    const drawn = new Set<string>(['points', 'concept', 'steps', 'compare', 'timeline', 'bars', 'line', 'donut', 'numbers', ...DIAGRAM_LAYOUTS]);
    for (const t of TEMPLATES) expect(drawn.has(t.layout)).withContext(t.id).toBeTrue();
    expect(LIST_LAYOUTS.length + MAP_LAYOUTS.length).toBe(DIAGRAM_LAYOUTS.length);
  });

  it('start with text that fits their own limits', () => {
    for (const t of TEMPLATES) {
      expect(t.sections.length).withContext(t.id).toBeGreaterThanOrEqual(t.minSections);
      expect(t.sections.length).withContext(t.id).toBeLessThanOrEqual(t.maxSections);
    }
  });

  it('say how to fill in the ones that are not obvious', () => {
    for (const id of ['pros-cons', 'versus', 'cycle', 'mind-map', 'tree', 'checklist']) expect(T(id).hint).withContext(id).toBeTruthy();
  });
});

describe('cycle geometry', () => {
  it('puts the first node at the top and the rest clockwise', () => {
    const p = cyclePositions(4, false);
    expect(p[0].x).toBeCloseTo(50, 5);
    expect(p[0].y).toBeLessThan(20);
    expect(p[1].x).toBeGreaterThan(80);
    expect(p[1].y).toBeCloseTo(50, 5);
    expect(p[2].y).toBeGreaterThan(80);
    expect(p[3].x).toBeLessThan(20);
  });

  it('mirrors for right to left', () => {
    const a = cyclePositions(5, false);
    const b = cyclePositions(5, true);
    for (let i = 0; i < 5; i++) {
      expect(b[i].x).toBeCloseTo(100 - a[i].x, 8);
      expect(b[i].y).toBeCloseTo(a[i].y, 8);
    }
  });

  it('points the arrowheads the way round the circle', () => {
    const [top, right, bottom, left] = cycleArrows(4, false);
    // Between the top and right nodes the path runs down and to the right; at the bottom it runs left.
    expect(top.rot).toBeGreaterThan(0);
    expect(top.rot).toBeLessThan(90);
    expect(right.rot).toBeGreaterThan(90);
    expect(right.rot).toBeLessThan(180);
    expect(bottom.rot).toBeLessThan(-90);
    expect(left.rot).toBeGreaterThan(-90);
    expect(left.rot).toBeLessThan(0);
  });

  it('turns the arrowheads the other way when mirrored', () => {
    const a = cycleArrows(4, false);
    const b = cycleArrows(4, true);
    // A mirrored arrow is the mirror image: its direction is 180 degrees minus the original.
    for (let i = 0; i < 4; i++) {
      const expected = 180 - a[i].rot;
      const diff = ((b[i].rot - expected + 540) % 360) - 180;
      expect(Math.abs(diff)).withContext(`arrow ${i}`).toBeLessThan(0.001);
    }
  });

  it('accounts for a wide box', () => {
    const square = cycleArrows(4, false, 1)[0].rot;
    const wide = cycleArrows(4, false, 1.5)[0].rot;
    expect(wide).not.toBeCloseTo(square, 1);
  });
});

@Component({
  standalone: true,
  imports: [InfographicDiagramComponent, InfographicMapComponent],
  // The artboard's size variables, which the diagram's CSS reads.
  template: `<div style="width: 900px; --k: 1; --gap: 36px; --h1: 84px; --h2: 44px; --body: 32px; --small: 28px; --num: 76px; --cols: 1">@if (isMap()) { <app-infographic-map [layout]="mapLayout()" [doc]="doc()" [rtl]="rtl()" /> } @else { <app-infographic-diagram [layout]="listLayout()" [doc]="doc()" [rtl]="rtl()" /> }</div>`,
})
class Host {
  layout = signal<DiagramLayout>('qa');
  isMap = () => (MAP_LAYOUTS as readonly string[]).includes(this.layout());
  mapLayout = () => this.layout() as MapLayout;
  listLayout = () => this.layout() as ListLayout;
  doc = signal<InfographicDoc>(newDoc(T('quiz-cards')));
  rtl = signal(false);
}

describe('InfographicDiagramComponent', () => {
  let fixture: ComponentFixture<Host>;
  let host: Host;
  const el = () => fixture.nativeElement as HTMLElement;
  function show(id: string, over: Partial<InfographicDoc> = {}, rtl = false) {
    host.layout.set(T(id).layout as DiagramLayout);
    host.doc.set(newDoc(T(id), over));
    host.rtl.set(rtl);
    fixture.detectChanges();
  }

  beforeEach(() => {
    TestBed.configureTestingModule({ imports: [Host] });
    fixture = TestBed.createComponent(Host);
    host = fixture.componentInstance;
    fixture.detectChanges();
  });

  for (const t of TEMPLATES.filter((x) => DIAGRAM_LAYOUTS.includes(x.layout))) {
    it(`${t.id}: shows every section's heading as text`, () => {
      show(t.id);
      const text = el().textContent!;
      for (const s of t.sections) expect(text).withContext(s.heading).toContain(s.heading);
    });
  }

  it('question cards number each question and show its answer', () => {
    show('quiz-cards');
    expect(Array.from(el().querySelectorAll('.badge')).map((b) => b.textContent)).toEqual(['1', '2', '3', '4']);
    expect(el().textContent).toContain(T('quiz-cards').sections[0].body);
  });

  it('staircase indents each step a little more', () => {
    show('staircase');
    const steps = Array.from(el().querySelectorAll<HTMLElement>('.stair'));
    const lefts = steps.map((s) => s.getBoundingClientRect().left);
    for (let i = 1; i < lefts.length; i++) expect(lefts[i]).toBeGreaterThan(lefts[i - 1]);
  });

  it('checklist draws a tick box for each item and omits an empty note', () => {
    const d = T('checklist');
    show('checklist', { sections: d.sections.map((s, i) => ({ id: 'c' + i, label: '', heading: s.heading, body: i === 0 ? '' : s.body })) });
    expect(el().querySelectorAll('.check').length).toBe(5);
    expect(el().querySelectorAll('.tick')[0].querySelector('p.body')).toBeNull();
  });

  it('pros and cons draws a plus for the first list and a minus for the second, from the lines of each', () => {
    show('pros-cons');
    const cols = el().querySelectorAll('.col');
    expect(cols.length).toBe(2);
    expect(cols[0].querySelectorAll('.pts li').length).toBe(3);
    expect(cols[0].querySelector('.sym')!.classList).not.toContain('minus');
    expect(cols[1].querySelector('.sym')!.classList).toContain('minus');
    expect(cols[0].textContent).toContain('Helps you start');
  });

  it('pros and cons ignores blank lines', () => {
    const d = T('pros-cons');
    show('pros-cons', { sections: [{ id: 'a', label: '', heading: 'Pros', body: 'one\n\n  \ntwo' }, { id: 'b', label: '', heading: 'Cons', body: '' }] });
    expect(el().querySelectorAll('.col')[0].querySelectorAll('.pts li').length).toBe(2);
    expect(d.maxSections).toBe(2);
  });

  it('side by side shows a divider only on a landscape artboard', () => {
    show('versus');
    expect(el().querySelector('.vs-mark')).toBeNull();
    show('versus', { format: 'landscape' });
    expect(el().querySelector('.vs-mark')).not.toBeNull();
  });

  it('zigzag starts each event one row after the last, on alternating sides, so they interleave', () => {
    show('zigzag');
    const items = Array.from(el().querySelectorAll<HTMLElement>('.zz > li'));
    expect(items.map((i) => i.style.gridRow)).toEqual(['1 / span 2', '2 / span 2', '3 / span 2', '4 / span 2']);
    const centers = items.map((i) => i.getBoundingClientRect().left + i.getBoundingClientRect().width / 2);
    expect(centers[0]).toBeLessThan(centers[1]);
    expect(centers[2]).toBeLessThan(centers[3]);
  });

  it('milestones shows the date large and uses two columns, or three when landscape', () => {
    show('milestones');
    expect(el().querySelector('.when')!.textContent).toBe('Jan');
    expect(el().querySelector<HTMLElement>('.ms')!.style.getPropertyValue('--cols')).toBe('2');
    show('milestones', { format: 'landscape' });
    expect(el().querySelector<HTMLElement>('.ms')!.style.getPropertyValue('--cols')).toBe('3');
  });

  it('flow chart numbers its boxes and puts an arrow after every box but the last', () => {
    show('flow-chart');
    const items = Array.from(el().querySelectorAll('.fl > li'));
    expect(items.map((i) => i.classList.contains('end'))).toEqual([false, false, false, true]);
    expect(Array.from(el().querySelectorAll('.badge')).map((b) => b.textContent)).toEqual(['1', '2', '3', '4']);
  });

  it('flow chart on a landscape artboard makes rows and ends each row without an arrow', () => {
    show('flow-chart', { format: 'landscape' });
    const items = Array.from(el().querySelectorAll('.fl > li'));
    expect(el().querySelector('.fl')!.classList).toContain('wide');
    expect(items.map((i) => i.classList.contains('end'))).toEqual([false, true, false, true]);
  });

  it('cycle draws a node and an arrowhead for each step', () => {
    show('cycle');
    expect(el().querySelectorAll('.node').length).toBe(4);
    expect(el().querySelectorAll('polygon').length).toBe(4);
    expect(el().querySelector('.cyc')!.getAttribute('style')).toContain('aspect-ratio');
  });

  it('cycle is wider on a landscape artboard', () => {
    show('cycle', { format: 'landscape' });
    expect(el().querySelector('svg')!.getAttribute('viewBox')).toBe('0 0 150 100');
  });

  it('cycle nodes do not overlap at the starting text on any size', () => {
    for (const format of ['portrait', 'square', 'landscape'] as const) {
      show('cycle', { format });
      const r = Array.from(el().querySelectorAll('.node')).map((n) => n.getBoundingClientRect());
      for (let i = 0; i < r.length; i++) {
        for (let j = i + 1; j < r.length; j++) {
          const w = Math.min(r[i].right, r[j].right) - Math.max(r[i].left, r[j].left);
          const h = Math.min(r[i].bottom, r[j].bottom) - Math.max(r[i].top, r[j].top);
          expect(w > 0 && h > 0).withContext(`${format} ${i} ${j}`).toBeFalse();
        }
      }
    }
  });

  it('mind map and tree draw the title themselves, as a heading', () => {
    show('mind-map', { title: 'Centre idea' });
    expect(el().querySelector('.core .title')!.textContent).toBe('Centre idea');
    show('tree', { title: 'Top idea' });
    expect(el().querySelector('.core.top .title')!.textContent).toBe('Top idea');
  });

  it('tree makes a bullet of each line and a column of each group', () => {
    show('tree');
    expect(el().querySelectorAll('.tr-cols > li').length).toBe(3);
    expect(el().querySelectorAll('.tr-cols > li')[0].querySelectorAll('.pts li').length).toBe(3);
  });

  it('treats typed markup as text', () => {
    show('quiz-cards', { sections: [{ id: 'a', label: '', heading: '<img src=x onerror=alert(1)>', body: '<b>x</b>' }, { id: 'b', label: '', heading: 'b', body: '' }] });
    expect(el().querySelector('img')).toBeNull();
    expect(el().querySelector('b')).toBeNull();
    expect(el().textContent).toContain('<img');
  });
});

@Component({
  standalone: true,
  imports: [InfographicCanvasComponent],
  template: `<div style="width: 600px"><app-infographic-canvas [doc]="doc()" (fitted)="last = $event" /></div>`,
})
class CanvasHost {
  doc = signal<InfographicDoc>(newDoc(T('cycle')));
  last: FitResult | null = null;
}

describe('the artboard with diagrams', () => {
  let fixture: ComponentFixture<CanvasHost>;
  let host: CanvasHost;
  const stage = () => fixture.nativeElement.querySelector('[data-testid="artboard"]') as HTMLElement;
  async function fit(doc: InfographicDoc): Promise<FitResult> {
    host.doc.set(doc);
    fixture.detectChanges();
    await fixture.whenStable();
    return fixture.debugElement.children[0].children[0].componentInstance.fit();
  }

  beforeEach(() => {
    TestBed.configureTestingModule({ imports: [CanvasHost] });
    fixture = TestBed.createComponent(CanvasHost);
    host = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('shows the title once for a mind map and a tree', async () => {
    for (const id of ['mind-map', 'tree']) {
      await fit(newDoc(T(id), { title: 'Only once' }));
      expect(stage().textContent!.split('Only once').length - 1).withContext(id).toBe(1);
    }
  });

  it('keeps the title in the header for the other diagrams', async () => {
    await fit(newDoc(T('flow-chart'), { title: 'In header' }));
    expect(stage().querySelector('header .title')!.textContent).toBe('In header');
  });

  it('every template fits its own starting text at every size', async () => {
    for (const t of TEMPLATES) {
      for (const format of ['portrait', 'square', 'landscape'] as const) {
        const r = await fit(newDoc(t, { format }));
        expect(r.overflowPx).withContext(`${t.id} ${format}`).toBe(0);
      }
    }
  });

  it('reports overlapping cycle nodes as not fitting', async () => {
    const t = T('cycle');
    const long = 'word '.repeat(14);
    const r = await fit({ ...newDoc(t, { format: 'square' }), sections: Array.from({ length: 6 }, (_, i) => ({ id: 'n' + i, label: '', heading: long, body: '' })) });
    expect(r.k).toBe(FIT_STEPS[FIT_STEPS.length - 1]);
    expect(r.overflowPx).toBeGreaterThan(0);
  });

  it('does not report overlap for a cycle with short steps', async () => {
    const r = await fit(newDoc(T('cycle'), { format: 'square' }));
    expect(r.overflowPx).toBe(0);
  });
});
