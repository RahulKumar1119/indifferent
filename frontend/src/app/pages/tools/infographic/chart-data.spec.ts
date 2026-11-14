import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { dataIssues, formatPercent, mix, niceMax, parseNumber, readItems, shares, sliceColors, toAsciiDigits } from './chart-data';
import { InfographicChartComponent } from './infographic-chart.component';
import { InfographicDoc } from './infographic.model';
import { TEMPLATES, newDoc, templateById } from './templates';
import { THEMES } from './themes';
import { contrastRatio } from './text-utils';

const T = (id: string) => templateById(id)!;
const docWith = (id: string, values: [string, string][], over: Partial<InfographicDoc> = {}): InfographicDoc => {
  const d = newDoc(T(id), over);
  return { ...d, sections: values.map(([label, heading], i) => ({ id: 's' + i, label, heading, body: '' })) };
};

describe('chart-data', () => {
  describe('parseNumber', () => {
    it('reads plain numbers, decimals and units', () => {
      expect(parseNumber('12')).toBe(12);
      expect(parseNumber('3.5')).toBe(3.5);
      expect(parseNumber('40%')).toBe(40);
      expect(parseNumber('₹ 1,200')).toBe(1200);
      expect(parseNumber('3.5M')).toBe(3.5);
      expect(parseNumber('.5')).toBe(0.5);
      expect(parseNumber('0')).toBe(0);
    });

    it('reads Indian and Western thousands marks and a decimal comma', () => {
      expect(parseNumber('1,20,000')).toBe(120000);
      expect(parseNumber('1,200,000')).toBe(1200000);
      expect(parseNumber('3,5')).toBe(3.5);
      expect(parseNumber('1,23')).toBe(1.23);
      expect(parseNumber('12,34,567')).toBe(1234567);
      expect(parseNumber('5,')).toBe(5);
    });

    it('reads Hindi, Gujarati and Arabic-Indic digits', () => {
      expect(parseNumber('१२')).toBe(12);
      expect(parseNumber('१,२००')).toBe(1200);
      expect(parseNumber('૩.૫')).toBe(3.5);
      expect(parseNumber('٤٢')).toBe(42);
      expect(toAsciiDigits('abc ९ ૪')).toBe('abc 9 4');
    });

    it('reads a negative number, with a hyphen or a minus sign', () => {
      expect(parseNumber('-4')).toBe(-4);
      expect(parseNumber('\u22124.5')).toBe(-4.5);
    });

    it('gives null for no number, more than one, or a malformed one', () => {
      for (const bad of ['', 'abc', '10-20', 'Q3 2024', '1,2,3', '12,34,5', '1,2345', '१२ से १५']) expect(parseNumber(bad)).withContext(bad).toBeNull();
      expect(parseNumber('9'.repeat(20))).toBeNull();
    });
  });

  describe('readItems and dataIssues', () => {
    it('marks unreadable and negative values and draws them as 0', () => {
      const items = readItems(docWith('bar-chart', [['A', '5'], ['B', 'many'], ['C', '-3']]).sections);
      expect(items.map((i) => i.value)).toEqual([5, 0, 0]);
      expect(items.map((i) => i.valid)).toEqual([true, false, false]);
    });

    it('names the section in each problem', () => {
      const d = docWith('bar-chart', [['A', '5'], ['Beta', 'many'], ['C', '-3']]);
      const issues = dataIssues(d, T('bar-chart'));
      expect(issues.length).toBe(2);
      expect(issues[0]).toContain('Section 2 (Beta)');
      expect(issues[1]).toContain('Section 3 (C)');
      expect(issues[1]).toContain('negative');
    });

    it('says when everything is zero', () => {
      expect(dataIssues(docWith('donut-chart', [['A', '0'], ['B', '0']]), T('donut-chart'))).toEqual(['Every number is 0, so there is nothing to draw.']);
    });

    it('is silent for good data and for non-chart templates', () => {
      expect(dataIssues(newDoc(T('bar-chart')), T('bar-chart'))).toEqual([]);
      expect(dataIssues(newDoc(T('key-points')), T('key-points'))).toEqual([]);
    });

    it('does not complain about zeros in key numbers', () => {
      expect(dataIssues(docWith('big-numbers', [['A', '0'], ['B', '0']]), T('big-numbers'))).toEqual([]);
    });
  });

  describe('numbers', () => {
    it('shares add up to 100, or are all 0 for a zero total', () => {
      const s = shares([40, 25, 15, 10, 10]);
      expect(s.reduce((a, b) => a + b, 0)).toBeCloseTo(100, 8);
      expect(shares([0, 0])).toEqual([0, 0]);
    });

    it('formats percents without a trailing .0', () => {
      expect(formatPercent(50)).toBe('50%');
      expect(formatPercent(33.333)).toBe('33.3%');
      expect(formatPercent(0.04)).toBe('0%');
      expect(formatPercent(99.96)).toBe('100%');
    });

    it('rounds an axis top up to 1, 2, 2.5, 5 or 10 times a power of ten', () => {
      expect(niceMax(0)).toBe(1);
      expect(niceMax(-3)).toBe(1);
      expect(niceMax(7)).toBe(10);
      expect(niceMax(7800)).toBe(10000);
      expect(niceMax(1.9)).toBe(2);
      expect(niceMax(2.2)).toBe(2.5);
      expect(niceMax(4.2)).toBe(5);
      expect(niceMax(10)).toBe(10);
    });
  });

  describe('colours', () => {
    it('mixes two colours', () => {
      expect(mix('#000000', '#ffffff', 0.5)).toBe('#808080');
      expect(mix('#102030', '#102030', 0.3)).toBe('#102030');
      expect(mix('#ff0000', '#0000ff', 0)).toBe('#ff0000');
    });

    for (const t of THEMES) {
      it(`${t.name}: every slice colour shows against the page and the card, and they are all different`, () => {
        const colors = sliceColors(t, 6);
        expect(colors.length).toBe(6);
        expect(new Set(colors).size).toBe(6);
        for (const c of colors) {
          expect(contrastRatio(c, t.bg)).withContext(`${c} on bg`).toBeGreaterThanOrEqual(3);
          expect(contrastRatio(c, t.surface)).withContext(`${c} on surface`).toBeGreaterThanOrEqual(3);
        }
      });

      it(`${t.name}: the accent works as large text on the card`, () => {
        expect(contrastRatio(t.accent, t.surface)).toBeGreaterThanOrEqual(3);
      });
    }

    it('gives fewer colours when asked for fewer, and never more than six', () => {
      expect(sliceColors(THEMES[0], 2).length).toBe(2);
      expect(sliceColors(THEMES[0], 9).length).toBe(6);
      expect(sliceColors(THEMES[0], 0).length).toBe(0);
    });
  });

  describe('the statistics templates', () => {
    const charts = TEMPLATES.filter((t) => t.chart);

    it('are four, in the Statistics category, and read their own starting data without problems', () => {
      expect(charts.map((t) => t.id)).toEqual(['bar-chart', 'line-chart', 'donut-chart', 'big-numbers']);
      for (const t of charts) {
        expect(t.category).toBe('Statistics');
        expect(dataIssues(newDoc(t), t)).withContext(t.id).toEqual([]);
        expect(t.labelName).not.toBe('');
        expect(t.hint).toBeTruthy();
      }
    });

    it('keep the donut inside the six colours', () => {
      expect(T('donut-chart').maxSections).toBeLessThanOrEqual(6);
    });
  });
});

@Component({
  standalone: true,
  imports: [InfographicChartComponent],
  template: `<app-infographic-chart [layout]="layout()" [doc]="doc()" [rtl]="rtl()" />`,
})
class Host {
  layout = signal<'bars' | 'line' | 'donut' | 'numbers'>('bars');
  doc = signal<InfographicDoc>(newDoc(T('bar-chart')));
  rtl = signal(false);
}

describe('InfographicChartComponent', () => {
  let fixture: ComponentFixture<Host>;
  let host: Host;
  const el = () => fixture.nativeElement as HTMLElement;
  function show(layout: Host['layout'] extends () => infer L ? L : never, doc: InfographicDoc, rtl = false) {
    host.layout.set(layout);
    host.doc.set(doc);
    host.rtl.set(rtl);
    fixture.detectChanges();
  }

  beforeEach(() => {
    TestBed.configureTestingModule({ imports: [Host] });
    fixture = TestBed.createComponent(Host);
    host = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('bars: shows every name and value as text and scales to the largest', () => {
    show('bars', docWith('bar-chart', [['Maths', '12'], ['Art', '3'], ['Odd', 'x']]));
    expect(el().textContent).toContain('Maths');
    expect(el().textContent).toContain('12');
    const widths = Array.from(el().querySelectorAll<HTMLElement>('.fill')).map((f) => f.style.width);
    expect(widths).toEqual(['100%', '25%', '0%']);
  });

  it('bars: draws nothing wide when every value is 0', () => {
    show('bars', docWith('bar-chart', [['A', '0'], ['B', '0']]));
    expect(Array.from(el().querySelectorAll<HTMLElement>('.fill')).map((f) => f.style.width)).toEqual(['0%', '0%']);
  });

  it('line: draws a point and a text label for each value, in order', () => {
    show('line', docWith('line-chart', [['Mon', '10'], ['Tue', '20'], ['Wed', '5']]));
    expect(el().querySelectorAll('circle').length).toBe(3);
    expect(Array.from(el().querySelectorAll('.pv')).map((e) => e.textContent)).toEqual(['10', '20', '5']);
    expect(Array.from(el().querySelectorAll('.xs span')).map((e) => e.textContent)).toEqual(['Mon', 'Tue', 'Wed']);
    const xs = Array.from(el().querySelectorAll('circle')).map((c) => Number(c.getAttribute('cx')));
    expect(xs[0]).toBeLessThan(xs[1]);
    expect(xs[1]).toBeLessThan(xs[2]);
  });

  it('line: the higher value sits higher, and zero sits on the base', () => {
    show('line', docWith('line-chart', [['a', '0'], ['b', '50'], ['c', '100']]));
    const ys = Array.from(el().querySelectorAll('circle')).map((c) => Number(c.getAttribute('cy')));
    expect(ys[0]).toBeGreaterThan(ys[1]);
    expect(ys[1]).toBeGreaterThan(ys[2]);
  });

  it('line: runs from the right when the page reads right to left', () => {
    const d = docWith('line-chart', [['a', '1'], ['b', '2'], ['c', '3']]);
    show('line', d, true);
    const xs = Array.from(el().querySelectorAll('circle')).map((c) => Number(c.getAttribute('cx')));
    expect(xs[0]).toBeGreaterThan(xs[2]);
  });

  it('line: keeps the points inside the drawing for every size', () => {
    for (const format of ['portrait', 'square', 'landscape'] as const) {
      show('line', { ...docWith('line-chart', [['a', '1'], ['b', '9'], ['c', '5']]), format });
      const [, , , h] = el().querySelector('svg')!.getAttribute('viewBox')!.split(' ').map(Number);
      for (const c of Array.from(el().querySelectorAll('circle'))) {
        expect(Number(c.getAttribute('cy'))).toBeGreaterThan(0);
        expect(Number(c.getAttribute('cy'))).toBeLessThan(h);
      }
    }
  });

  it('donut: lists each slice with its share and a different colour', () => {
    show('donut', docWith('donut-chart', [['A', '1'], ['B', '1'], ['C', '2']]));
    const rows = Array.from(el().querySelectorAll('.lg')).map((r) => r.textContent!.replace(/\s+/g, ' ').trim());
    expect(rows[0]).toContain('A');
    expect(rows[0]).toContain('25%');
    expect(rows[2]).toContain('50%');
    const stroke = Array.from(el().querySelectorAll('circle')).slice(1).map((c) => c.getAttribute('stroke'));
    expect(new Set(stroke).size).toBe(3);
  });

  it('donut: slices follow on from each other and fill the ring', () => {
    show('donut', docWith('donut-chart', [['A', '1'], ['B', '3']]));
    const slices = Array.from(el().querySelectorAll('circle')).slice(1);
    expect(Number(slices[0].getAttribute('stroke-dashoffset'))).toBeCloseTo(0, 5);
    expect(Number(slices[1].getAttribute('stroke-dashoffset'))).toBeCloseTo(-25, 5);
  });

  it('donut: leaves a thin gap between slices, but never hides a tiny one', () => {
    show('donut', docWith('donut-chart', [['A', '1'], ['B', '3']]));
    const dash = (i: number) => Number(Array.from(el().querySelectorAll('circle'))[i + 1].getAttribute('stroke-dasharray')!.split(' ')[0]);
    expect(dash(0)).toBeCloseTo(24.2, 5);
    expect(dash(1)).toBeCloseTo(74.2, 5);
    show('donut', docWith('donut-chart', [['A', '1'], ['B', '300']]));
    expect(dash(0)).toBeGreaterThan(0);
  });

  it('donut: shows 0% and only the grey ring when the total is 0', () => {
    show('donut', docWith('donut-chart', [['A', '0'], ['B', '0']]));
    expect(el().textContent).toContain('0%');
  });

  it('donut: is mirrored for right to left', () => {
    show('donut', newDoc(T('donut-chart')), true);
    expect(el().querySelector<SVGElement>('svg')!.style.transform).toContain('scaleX(-1)');
    show('donut', newDoc(T('donut-chart')), false);
    expect(el().querySelector<SVGElement>('svg')!.style.transform).toBe('');
  });

  it('numbers: shows the number as typed, with the name and the note', () => {
    show('numbers', newDoc(T('big-numbers')));
    const first = el().querySelector('.nc')!;
    expect(first.querySelector('.big')!.textContent).toBe('1,200');
    expect(first.querySelector('h2')!.textContent).toBe('Students');
    expect(first.querySelector('.note')!.textContent).toContain('Across all classes');
  });

  it('numbers: leaves out an empty note and uses sensible columns', () => {
    const d = docWith('big-numbers', [['A', '1'], ['B', '2'], ['C', '3']]);
    show('numbers', d);
    expect(el().querySelector('.note')).toBeNull();
    expect(el().querySelector<HTMLElement>('.nums')!.style.getPropertyValue('--cols')).toBe('2');
    expect(el().querySelector('.nums')!.classList).toContain('odd');
    show('numbers', { ...d, format: 'landscape' });
    expect(el().querySelector<HTMLElement>('.nums')!.style.getPropertyValue('--cols')).toBe('3');
  });

  it('treats typed markup as text', () => {
    show('bars', docWith('bar-chart', [['<img src=x onerror=alert(1)>', '5'], ['b', '1']]));
    expect(el().querySelector('img')).toBeNull();
    expect(el().textContent).toContain('<img');
  });
});
