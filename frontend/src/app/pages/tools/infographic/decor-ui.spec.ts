import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { LucideTrash2, provideLucideIcons } from '@lucide/angular';
import { InfographicCanvasComponent } from './infographic-canvas.component';
import { InfographicDecorPanelComponent } from './infographic-decor-panel.component';
import { DocChange } from './infographic-design-panel.component';
import { Decoration, InfographicDoc, LIMITS } from './infographic.model';
import { addDecoration } from './doc-ops';
import { TEMPLATES, newDoc } from './templates';

const item = (over: Partial<Decoration> = {}): Decoration => ({ id: 'a1', glyph: 'circle', x: 50, y: 50, size: 20, color: 'accent', ...over });
const docWith = (items: Decoration[]): InfographicDoc => ({ ...newDoc(TEMPLATES[0]), items });

@Component({
  standalone: true,
  imports: [InfographicCanvasComponent],
  template: `<div style="width: 540px"><app-infographic-canvas
    [doc]="doc()" [selected]="selected()"
    (fitted)="fits = fits + 1"
    (decoMoved)="moved.push($event)" (decoNudged)="nudged.push($event)" (decoSelected)="picked.push($event)"
    (decoRemoved)="removed.push($event)" (decoDropped)="dropped = dropped + 1" /></div>`,
})
class CanvasHost {
  doc = signal<InfographicDoc>(docWith([item()]));
  selected = signal<string | null>(null);
  fits = 0;
  dropped = 0;
  moved: { id: string; x: number; y: number }[] = [];
  nudged: { id: string; dx: number; dy: number }[] = [];
  picked: string[] = [];
  removed: string[] = [];
}

describe('icons and shapes on the artboard', () => {
  let fixture: ComponentFixture<CanvasHost>;
  let host: CanvasHost;
  const el = () => fixture.nativeElement as HTMLElement;
  const deco = () => el().querySelector<HTMLElement>('[data-testid="deco"]')!;
  const stage = () => el().querySelector<HTMLElement>('[data-testid="artboard"]')!;
  const pointer = (type: string, init: PointerEventInit & { target?: HTMLElement } = {}) => {
    const target = init.target ?? deco();
    target.dispatchEvent(new PointerEvent(type, { bubbles: true, cancelable: true, pointerId: 7, button: 0, ...init }));
  };

  beforeEach(async () => {
    TestBed.configureTestingModule({ imports: [CanvasHost] });
    fixture = TestBed.createComponent(CanvasHost);
    host = fixture.componentInstance;
    fixture.detectChanges();
    await fixture.whenStable();
  });

  it('draws each one where its percentages say, as a button with a spoken name', () => {
    host.doc.set(docWith([item({ x: 25, y: 75, size: 30, glyph: 'lightbulb' })]));
    fixture.detectChanges();
    expect(deco().style.left).toBe('25%');
    expect(deco().style.top).toBe('75%');
    expect(deco().style.width).toBe('30%');
    expect(deco().getAttribute('role')).toBe('button');
    expect(deco().tabIndex).toBe(0);
    expect(deco().getAttribute('aria-label')).toContain('Light bulb, 25 percent across and 75 percent down');
    expect(deco().querySelector('svg')).not.toBeNull();
  });

  it('is painted in a colour of the theme', () => {
    host.doc.set(docWith([item({ color: 'ink' })]));
    fixture.detectChanges();
    expect(deco().style.color).toBe('var(--ink)');
    host.doc.set(docWith([item({ color: 'ink2' })]));
    fixture.detectChanges();
    expect(deco().style.color).toBe('var(--ink2)');
  });

  it('has a ring that is not part of the picture, shown only for the selected one or the focused one', () => {
    const ring = () => deco().querySelector<HTMLElement>('.ring')!;
    expect(ring().hasAttribute('data-skip-export')).toBeTrue();
    expect(getComputedStyle(ring()).display).toBe('none');
    host.selected.set('a1');
    fixture.detectChanges();
    expect(getComputedStyle(ring()).display).toBe('block');
  });

  it('lets a drag move it by the distance the pointer travelled, as percent of the artboard as shown', () => {
    const box = stage().getBoundingClientRect();
    pointer('pointerdown', { clientX: 100, clientY: 100 });
    pointer('pointermove', { clientX: 100 + box.width * 0.1, clientY: 100 + box.height * 0.2 });
    expect(host.picked).toEqual(['a1']);
    expect(host.moved.length).toBe(1);
    expect(host.moved[0].x).toBeCloseTo(60, 3);
    expect(host.moved[0].y).toBeCloseTo(70, 3);
  });

  it('measures a drag on a scaled-down artboard in the scale it is shown at', () => {
    // The artboard is 1080 wide but shown 540 wide; 54 screen pixels is 10 percent either way.
    const box = stage().getBoundingClientRect();
    expect(box.width).toBeLessThan(stage().offsetWidth);
    pointer('pointerdown', { clientX: 0, clientY: 0 });
    pointer('pointermove', { clientX: box.width / 10, clientY: 0 });
    expect(host.moved[0].x).toBeCloseTo(60, 3);
    expect(host.moved[0].y).toBeCloseTo(50, 3);
  });

  it('stops at the edges', () => {
    pointer('pointerdown', { clientX: 0, clientY: 0 });
    pointer('pointermove', { clientX: 99999, clientY: -99999 });
    expect(host.moved[0].x).toBe(100);
    expect(host.moved[0].y).toBe(0);
  });

  it('does not move without a press, for another pointer, or after the pointer is let go', () => {
    pointer('pointermove', { clientX: 50, clientY: 50 });
    expect(host.moved.length).toBe(0);
    pointer('pointerdown', { clientX: 0, clientY: 0 });
    pointer('pointermove', { clientX: 50, clientY: 50, pointerId: 9 });
    expect(host.moved.length).toBe(0);
    pointer('pointerup', { pointerId: 7 });
    pointer('pointermove', { clientX: 80, clientY: 80 });
    expect(host.moved.length).toBe(0);
  });

  it('says when the pointer is let go, once, and not for a stray pointer', () => {
    pointer('pointerdown', { clientX: 0, clientY: 0 });
    pointer('pointerup', { pointerId: 9 });
    expect(host.dropped).toBe(0);
    pointer('pointerup', { pointerId: 7 });
    expect(host.dropped).toBe(1);
    pointer('pointerup', { pointerId: 7 });
    expect(host.dropped).toBe(1);
  });

  it('ignores a press with a button other than the main one', () => {
    pointer('pointerdown', { clientX: 0, clientY: 0, button: 2 });
    expect(host.picked.length).toBe(0);
  });

  it('is moved by the arrow keys, 1 percent and 5 with Shift, and removed by Delete', () => {
    const press = (key: string, shiftKey = false) => deco().dispatchEvent(new KeyboardEvent('keydown', { key, shiftKey, bubbles: true, cancelable: true }));
    press('ArrowRight');
    press('ArrowLeft', true);
    press('ArrowDown');
    press('ArrowUp', true);
    expect(host.nudged).toEqual([
      { id: 'a1', dx: 1, dy: 0 },
      { id: 'a1', dx: -5, dy: 0 },
      { id: 'a1', dx: 0, dy: 1 },
      { id: 'a1', dx: 0, dy: -5 },
    ]);
    press('Delete');
    press('Backspace');
    expect(host.removed).toEqual(['a1', 'a1']);
  });

  it('leaves keys with Ctrl, Cmd or Alt alone, so undo still works from an icon', () => {
    const e = new KeyboardEvent('keydown', { key: 'ArrowRight', ctrlKey: true, bubbles: true, cancelable: true });
    deco().dispatchEvent(e);
    expect(e.defaultPrevented).toBeFalse();
    expect(host.nudged.length).toBe(0);
    const z = new KeyboardEvent('keydown', { key: 'Delete', metaKey: true, bubbles: true, cancelable: true });
    deco().dispatchEvent(z);
    expect(host.removed.length).toBe(0);
  });

  it('is selected when it takes focus', () => {
    deco().dispatchEvent(new FocusEvent('focus'));
    expect(host.picked).toEqual(['a1']);
  });

  it('does not fit the text again when only an icon moves, but does when the text changes', async () => {
    await fixture.whenStable();
    const base = host.fits;
    host.doc.update((d) => ({ ...d, items: [item({ x: 70 })] }));
    fixture.detectChanges();
    await fixture.whenStable();
    expect(host.fits).toBe(base);
    host.doc.update((d) => ({ ...d, title: 'A different title' }));
    fixture.detectChanges();
    await fixture.whenStable();
    expect(host.fits).toBeGreaterThan(base);
  });

  it('does not count as text running off the page, even at the edge', () => {
    host.doc.set(docWith([item({ x: 100, y: 100, size: 60 })]));
    fixture.detectChanges();
    expect(stage().scrollHeight).toBeLessThanOrEqual(stage().clientHeight + 1);
  });
});

@Component({
  standalone: true,
  imports: [InfographicDecorPanelComponent],
  template: `<app-infographic-decor-panel [doc]="doc()" [selected]="selected()" (docChange)="changes.push($event)" (announce)="said.push($event)" (pick)="picked.push($event)" />`,
})
class PanelHost {
  doc = signal<InfographicDoc>(docWith([]));
  selected = signal<string | null>(null);
  changes: DocChange[] = [];
  said: string[] = [];
  picked: string[] = [];
}

describe('InfographicDecorPanelComponent', () => {
  let fixture: ComponentFixture<PanelHost>;
  let host: PanelHost;
  const el = () => fixture.nativeElement as HTMLElement;
  const q = <T extends HTMLElement>(sel: string) => el().querySelector(sel) as T;

  beforeEach(() => {
    TestBed.configureTestingModule({ imports: [PanelHost], providers: [provideLucideIcons(LucideTrash2)] });
    fixture = TestBed.createComponent(PanelHost);
    host = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('offers every shape and icon as a button with its name', () => {
    const labels = Array.from(el().querySelectorAll('.pick button')).map((b) => b.getAttribute('aria-label'));
    expect(labels.length).toBeGreaterThanOrEqual(33);
    expect(labels).toContain('Add circle');
    expect(labels).toContain('Add light bulb icon');
    expect(new Set(labels).size).toBe(labels.length);
  });

  it('adds one, selects it and says so', () => {
    q('[aria-label="Add star outline icon"]').click();
    expect(host.changes.length).toBe(1);
    expect(host.changes[0].doc.items.length).toBe(1);
    expect(host.changes[0].doc.items[0].glyph).toBe('star');
    expect(host.picked).toEqual([host.changes[0].doc.items[0].id]);
    expect(host.said[0]).toContain('added as item 1');
  });

  it('disables adding at the limit and says why', () => {
    let d = host.doc();
    for (let i = 0; i < LIMITS.items; i++) d = addDecoration(d, 'circle');
    host.doc.set(d);
    fixture.detectChanges();
    expect(Array.from(el().querySelectorAll<HTMLButtonElement>('.pick button')).every((b) => b.disabled)).toBeTrue();
    expect(q('[data-testid="decor-full"]').textContent).toContain(`${LIMITS.items}`);
  });

  describe('with items', () => {
    beforeEach(() => {
      host.doc.set(docWith([item({ id: 'a', glyph: 'lightbulb' }), item({ id: 'b', x: 10, y: 20 })]));
      host.selected.set('b');
      fixture.detectChanges();
    });

    it('lists them in order, marking the selected one', () => {
      const rows = el().querySelectorAll('li[data-item]');
      expect(Array.from(rows).map((r) => (r as HTMLElement).dataset['item'])).toEqual(['a', 'b']);
      expect(rows[1].classList).toContain('on');
      expect(rows[0].classList).not.toContain('on');
      expect(rows[0].textContent).toContain('Light bulb 1');
    });

    it('has labelled boxes for size, place and colour', () => {
      for (const f of ['size', 'x', 'y', 'color']) {
        const input = q(`#ig-${f}-a`);
        expect(input).withContext(f).not.toBeNull();
        expect(el().querySelector(`label[for="ig-${f}-a"]`)).withContext(f).not.toBeNull();
      }
    });

    it('changes the place from the number boxes, as one undo step per box', () => {
      const x = q<HTMLInputElement>('#ig-x-a');
      x.value = '75';
      x.dispatchEvent(new Event('input'));
      expect(host.changes[0].doc.items[0].x).toBe(75);
      expect(host.changes[0].key).toBe('deco:a:x');
    });

    it('holds a number that is too large or too small inside the artboard', () => {
      const y = q<HTMLInputElement>('#ig-y-a');
      y.value = '900';
      y.dispatchEvent(new Event('input'));
      expect(host.changes[0].doc.items[0].y).toBe(100);
    });

    it('leaves an empty box alone while someone is typing', () => {
      const x = q<HTMLInputElement>('#ig-x-a');
      x.value = '';
      x.dispatchEvent(new Event('input'));
      expect(host.changes.length).toBe(0);
    });

    it('resizes with the slider and recolours with the list', () => {
      const size = q<HTMLInputElement>('#ig-size-a');
      size.value = '33';
      size.dispatchEvent(new Event('input'));
      expect(host.changes[0].doc.items[0].size).toBe(33);
      const color = q<HTMLSelectElement>('#ig-color-a');
      color.value = 'ink2';
      color.dispatchEvent(new Event('change'));
      expect(host.changes[1].doc.items[0].color).toBe('ink2');
    });

    it('selects an item when one of its boxes takes focus', () => {
      q<HTMLInputElement>('#ig-x-a').dispatchEvent(new FocusEvent('focus'));
      expect(host.picked).toEqual(['a']);
    });

    it('removes one with its own button, named by its place in the list', () => {
      q('[aria-label="Remove circle 2"]').click();
      expect(host.changes[0].doc.items.map((i) => i.id)).toEqual(['a']);
      expect(host.said[0]).toContain('removed');
    });

    it('explains how to use the keys', () => {
      expect(el().querySelector('.hint')!.textContent).toContain('arrow keys');
    });
  });
});
