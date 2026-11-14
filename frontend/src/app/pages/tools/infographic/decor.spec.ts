import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { DecorGlyphComponent } from './decor-glyph.component';
import { GLYPHS, GLYPH_ATTRS, GLYPH_TAGS, Glyph, ICONS, SHAPES, glyphById } from './decor-library';
import { DEFAULT_ITEM_SIZE, addDecoration, clampTo, removeDecoration, resetDesign, updateDecoration } from './doc-ops';
import { sanitizeDoc } from './draft-store';
import { DocHistory } from './history';
import { Decoration, LIMITS } from './infographic.model';
import { TEMPLATES, newDoc } from './templates';

const base = () => newDoc(TEMPLATES[0]);
const item = (over: Partial<Decoration> = {}): Decoration => ({ id: 'a1', glyph: 'circle', x: 50, y: 50, size: 14, color: 'accent', ...over });

describe('decor-library', () => {
  it('has unique ids and a name for each', () => {
    expect(new Set(GLYPHS.map((g) => g.id)).size).toBe(GLYPHS.length);
    for (const g of GLYPHS) expect(g.label.trim()).withContext(g.id).not.toBe('');
    expect(SHAPES.length).toBeGreaterThanOrEqual(8);
    expect(ICONS.length).toBeGreaterThanOrEqual(24);
  });

  it('draws every glyph from elements and attributes the glyph component will make', () => {
    for (const g of GLYPHS) {
      expect(g.nodes.length).withContext(g.id).toBeGreaterThan(0);
      for (const [tag, attrs] of g.nodes) {
        expect(GLYPH_TAGS.has(tag)).withContext(`${g.id} <${tag}>`).toBeTrue();
        for (const name of Object.keys(attrs)) expect(GLYPH_ATTRS.has(name)).withContext(`${g.id} ${name}`).toBeTrue();
      }
    }
  });

  it('marks shapes as filled and icons as outlines', () => {
    expect(SHAPES.every((g) => g.kind === 'shape')).toBeTrue();
    expect(ICONS.every((g) => g.kind === 'icon')).toBeTrue();
  });

  it('finds a glyph by id and returns nothing for an unknown one', () => {
    expect(glyphById('lightbulb')?.label).toBe('Light bulb');
    expect(glyphById('nope')).toBeUndefined();
    expect(glyphById('__proto__')).toBeUndefined();
  });
});

describe('icons and shapes in the document', () => {
  it('starts a document with none', () => {
    expect(base().items).toEqual([]);
  });

  it('adds one near the middle, with a default size and the accent colour', () => {
    const d = addDecoration(base(), 'star');
    expect(d.items.length).toBe(1);
    expect(d.items[0]).toEqual(jasmine.objectContaining({ glyph: 'star', size: DEFAULT_ITEM_SIZE, color: 'accent' }));
    expect(d.items[0].x).toBeGreaterThanOrEqual(30);
    expect(d.items[0].x).toBeLessThanOrEqual(70);
  });

  it('puts each new one in a different place and gives it its own id', () => {
    let d = base();
    for (let i = 0; i < 4; i++) d = addDecoration(d, 'circle');
    expect(new Set(d.items.map((i) => `${i.x},${i.y}`)).size).toBe(4);
    expect(new Set(d.items.map((i) => i.id)).size).toBe(4);
  });

  it('ignores an unknown glyph and stops at the limit', () => {
    const d = base();
    expect(addDecoration(d, 'nope')).toBe(d);
    let full = d;
    for (let i = 0; i < LIMITS.items + 5; i++) full = addDecoration(full, 'circle');
    expect(full.items.length).toBe(LIMITS.items);
    expect(addDecoration(full, 'circle')).toBe(full);
  });

  it('removes by id and ignores an unknown id', () => {
    const d = { ...base(), items: [item({ id: 'a' }), item({ id: 'b' })] };
    expect(removeDecoration(d, 'a').items.map((i) => i.id)).toEqual(['b']);
    expect(removeDecoration(d, 'zzz')).toBe(d);
  });

  it('holds place and size inside what the artboard can show', () => {
    const d = { ...base(), items: [item()] };
    const out = updateDecoration(d, 'a1', { x: 500, y: -40, size: 1 }).items[0];
    expect([out.x, out.y, out.size]).toEqual([100, 0, LIMITS.itemMin]);
    expect(updateDecoration(d, 'a1', { size: 999 }).items[0].size).toBe(LIMITS.itemMax);
  });

  it('turns a value that is not a number into the low end', () => {
    expect(clampTo(NaN, 4, 60)).toBe(4);
    expect(clampTo(Infinity, 0, 100)).toBe(0);
    expect(clampTo(12.34, 0, 100)).toBe(12.3);
  });

  it('keeps the same document when nothing changed, and when the colour is not one of the theme colours', () => {
    const d = { ...base(), items: [item()] };
    expect(updateDecoration(d, 'a1', { x: 50 })).toBe(d);
    expect(updateDecoration(d, 'a1', { color: 'accent' })).toBe(d);
    expect(updateDecoration(d, 'a1', { color: 'pink' as never })).toBe(d);
    expect(updateDecoration(d, 'missing', { x: 1 })).toBe(d);
    expect(updateDecoration(d, 'a1', { color: 'ink' }).items[0].color).toBe('ink');
  });

  it('does not change the other items', () => {
    const d = { ...base(), items: [item({ id: 'a' }), item({ id: 'b', x: 10 })] };
    const out = updateDecoration(d, 'a', { x: 90 });
    expect(out.items[1]).toBe(d.items[1]);
    expect(d.items[0].x).toBe(50);
  });

  it('is removed by Reset design, which then reports that nothing is left to reset', () => {
    const d = { ...base(), items: [item()] };
    const r = resetDesign(d);
    expect(r.items).toEqual([]);
    expect(r.title).toBe(d.title);
    expect(resetDesign(r)).toBe(r);
  });
});

describe('saved icons and shapes', () => {
  const withItems = (items: unknown): unknown => ({ ...JSON.parse(JSON.stringify(base())), items });

  it('are read back as they were saved', () => {
    const saved = [item({ id: 'a', glyph: 'lightbulb', x: 12.3, y: 45.6, size: 22, color: 'ink2' })];
    expect(sanitizeDoc(withItems(saved))!.items).toEqual(saved);
  });

  it('are none for an older draft without the field', () => {
    const old = JSON.parse(JSON.stringify(base()));
    delete old.items;
    expect(sanitizeDoc(old)!.items).toEqual([]);
    expect(sanitizeDoc(withItems('oops'))!.items).toEqual([]);
  });

  it('drop an unknown glyph and anything that is not an object', () => {
    const out = sanitizeDoc(withItems([item({ glyph: 'nope' }), null, 5, 'x', item({ id: 'ok' })]))!.items;
    expect(out.map((i) => i.id)).toEqual(['ok']);
  });

  it('are brought inside the limits, with a safe colour', () => {
    const out = sanitizeDoc(withItems([{ id: 'a', glyph: 'circle', x: 'left', y: 900, size: 0, color: 'pink' }]))!.items[0];
    expect([out.x, out.y, out.size, out.color]).toEqual([50, 100, LIMITS.itemMin, 'accent']);
  });

  it('are never more than the limit, and never share an id or carry an unsafe one', () => {
    const many = Array.from({ length: 50 }, (_, i) => item({ id: i % 2 ? 'same' : '<b>' }));
    const out = sanitizeDoc(withItems(many))!.items;
    expect(out.length).toBe(LIMITS.items);
    expect(new Set(out.map((i) => i.id)).size).toBe(out.length);
    expect(out.every((i) => /^[\w-]{1,40}$/.test(i.id))).toBeTrue();
  });

  it('do not share an id with a section', () => {
    const d = JSON.parse(JSON.stringify(base()));
    d.items = [item({ id: d.sections[0].id })];
    const out = sanitizeDoc(d)!;
    expect(out.items[0].id).not.toBe(out.sections[0].id);
  });
});

describe('DocHistory.endGroup', () => {
  const a = base();
  it('makes the next change a new step even for the same key within the window', () => {
    const h = new DocHistory(a);
    const b = { ...a, title: 'b' };
    h.push(b, 'k', 1000);
    h.endGroup();
    h.push({ ...a, title: 'c' }, 'k', 1100);
    expect(h.undo()).toBe(b);
    expect(h.undo()).toBe(a);
  });

  it('merges without it', () => {
    const h = new DocHistory(a);
    h.push({ ...a, title: 'b' }, 'k', 1000);
    h.push({ ...a, title: 'c' }, 'k', 1100);
    expect(h.undo()).toBe(a);
  });
});

@Component({ standalone: true, imports: [DecorGlyphComponent], template: `<app-decor-glyph [glyph]="glyph()" />` })
class GlyphHost {
  glyph = signal<Glyph | undefined>(undefined);
}

describe('DecorGlyphComponent', () => {
  let fixture: ComponentFixture<GlyphHost>;
  const svg = () => (fixture.nativeElement as HTMLElement).querySelector('svg');

  beforeEach(() => {
    TestBed.configureTestingModule({ imports: [GlyphHost] });
    fixture = TestBed.createComponent(GlyphHost);
  });

  function show(g: Glyph | undefined) {
    fixture.componentInstance.glyph.set(g);
    fixture.detectChanges();
  }

  it('draws a shape filled and an icon as an outline, both hidden from screen readers', () => {
    show(glyphById('circle'));
    expect(svg()!.getAttribute('fill')).toBe('currentColor');
    expect(svg()!.getAttribute('aria-hidden')).toBe('true');
    show(glyphById('lightbulb'));
    expect(svg()!.getAttribute('fill')).toBe('none');
    expect(svg()!.getAttribute('stroke')).toBe('currentColor');
    expect(svg()!.querySelectorAll('path').length).toBeGreaterThan(0);
  });

  it('uses the SVG namespace so the drawing shows', () => {
    show(glyphById('hexagon'));
    expect(svg()!.namespaceURI).toBe('http://www.w3.org/2000/svg');
    expect(svg()!.firstElementChild!.namespaceURI).toBe('http://www.w3.org/2000/svg');
  });

  it('replaces the old drawing and draws nothing for no glyph', () => {
    show(glyphById('circle'));
    show(glyphById('bar'));
    expect((fixture.nativeElement as HTMLElement).querySelectorAll('svg').length).toBe(1);
    expect(svg()!.querySelector('rect')).not.toBeNull();
    show(undefined);
    expect(svg()).toBeNull();
  });

  it('makes only the elements and attributes it knows', () => {
    const bad: Glyph = {
      id: 'x', label: 'x', kind: 'shape',
      nodes: [
        ['script', { src: 'x' }],
        ['circle', { cx: 5, cy: 5, r: 3, onload: 'alert(1)', style: 'x', href: 'javascript:alert(1)' }],
      ],
    };
    show(bad);
    expect(svg()!.querySelector('script')).toBeNull();
    const circle = svg()!.querySelector('circle')!;
    expect(circle.getAttribute('r')).toBe('3');
    expect(circle.hasAttribute('onload')).toBeFalse();
    expect(circle.hasAttribute('style')).toBeFalse();
    expect(circle.hasAttribute('href')).toBeFalse();
  });
});

