import { Component } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import {
  LucideChevronDown, LucideChevronUp, LucideCopy, LucideDownload, LucideGripVertical, LucideImage, LucideLock, LucidePlus, LucideRedo2, LucideRotateCcw,
  LucideShieldCheck, LucideTrash2, LucideUndo2, LucideUserX, provideLucideIcons,
} from '@lucide/angular';
import { PublicHeaderComponent } from '../../../shared/components/public-header/public-header.component';
import { SiteFooterComponent } from '../../../shared/components/site-footer/site-footer.component';
import { DRAFT_KEY, INFOGRAPHIC_STORAGE } from './draft-store';
import { InfographicExporter } from './infographic-export.service';
import { InfographicFonts } from './infographic-fonts.service';
import { InfographicMakerComponent } from './infographic-maker.component';
import { InfographicDoc, LIMITS } from './infographic.model';
import { TEMPLATES, newDoc, templateById } from './templates';

@Component({ selector: 'app-public-header', standalone: true, template: '' })
class HeaderStub {}
@Component({ selector: 'app-site-footer', standalone: true, template: '' })
class FooterStub {}

class FakeStorage implements Storage {
  data = new Map<string, string>();
  get length() { return this.data.size; }
  clear() { this.data.clear(); }
  getItem(k: string) { return this.data.get(k) ?? null; }
  key(i: number) { return [...this.data.keys()][i] ?? null; }
  removeItem(k: string) { this.data.delete(k); }
  setItem(k: string, v: string) { this.data.set(k, v); }
}

class FakeFonts {
  async load() { return []; }
  async ensureLoaded() {}
  async embed() { return { css: '', uncovered: [] as number[] }; }
}

class FakeExporter {
  calls: string[] = [];
  uncovered: string[] = [];
  fail = false;
  slow = false;
  private gate: (() => void) | null = null;
  release() {
    this.gate?.();
  }
  async export(_s: HTMLElement, doc: InfographicDoc, kind: string) {
    this.calls.push(kind);
    if (this.slow) await new Promise<void>((resolve) => (this.gate = resolve));
    if (this.fail) throw new Error('nope');
    return { blob: new Blob(['x']), filename: `x.${kind}`, width: 1080, height: 1920, uncovered: this.uncovered };
  }
}

const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));

describe('InfographicMakerComponent', () => {
  let fixture: ComponentFixture<InfographicMakerComponent>;
  let el: HTMLElement;
  let storage: FakeStorage;
  let exporter: FakeExporter;

  const q = <T extends HTMLElement>(sel: string) => el.querySelector(sel) as T | null;
  const byTest = (id: string) => q(`[data-testid="${id}"]`);
  const button = (text: string) => Array.from(el.querySelectorAll('button')).find((b) => b.textContent?.trim().startsWith(text)) as HTMLButtonElement;
  async function settle() {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  }
  async function type(sel: string, value: string) {
    const input = q<HTMLInputElement | HTMLTextAreaElement>(sel)!;
    input.value = value;
    input.dispatchEvent(new Event('input', { bubbles: true }));
    await settle();
  }
  /** For tests where a long timer is pending (the download link's 10 s clean-up), which whenStable would wait for. */
  async function flush() {
    await wait(40);
    fixture.detectChanges();
  }
  function typeNow(sel: string, value: string) {
    const input = q<HTMLInputElement | HTMLTextAreaElement>(sel)!;
    input.value = value;
    input.dispatchEvent(new Event('input', { bubbles: true }));
    fixture.detectChanges();
  }
  async function click(b: HTMLElement | null) {
    b!.click();
    await settle();
  }
  const saved = (): InfographicDoc | null => (storage.data.has(DRAFT_KEY) ? JSON.parse(storage.data.get(DRAFT_KEY)!) : null);
  const title = () => q<HTMLInputElement>('#ig-title')!.value;
  const headings = () => Array.from(el.querySelectorAll<HTMLInputElement>('input[id^="ig-heading-"]')).map((i) => i.value);

  async function create(initial?: InfographicDoc, store: Storage | null = storage) {
    if (initial) storage.setItem(DRAFT_KEY, JSON.stringify(initial));
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      imports: [InfographicMakerComponent],
      providers: [
        provideLucideIcons(LucideChevronDown, LucideChevronUp, LucideCopy, LucideDownload, LucideGripVertical, LucideImage, LucideLock, LucidePlus, LucideRedo2, LucideRotateCcw, LucideShieldCheck, LucideTrash2, LucideUndo2, LucideUserX),
        { provide: INFOGRAPHIC_STORAGE, useValue: store },
        { provide: InfographicFonts, useValue: new FakeFonts() },
        { provide: InfographicExporter, useValue: exporter },
      ],
    });
    TestBed.overrideComponent(InfographicMakerComponent, {
      remove: { imports: [PublicHeaderComponent, SiteFooterComponent] },
      add: { imports: [HeaderStub, FooterStub] },
    });
    fixture = TestBed.createComponent(InfographicMakerComponent);
    el = fixture.nativeElement;
    await settle();
  }

  beforeEach(() => {
    storage = new FakeStorage();
    exporter = new FakeExporter();
  });

  afterEach(() => fixture?.destroy());

  it('says plainly that everything stays in the browser', async () => {
    await create();
    const text = byTest('privacy')!.textContent!;
    expect(text).toContain('never uploaded');
    expect(text).toContain("this browser's local storage");
  });

  describe('search and sharing', () => {
    afterEach(() => document.head.querySelectorAll('meta[name="robots"], script[data-seo-id], link[rel="canonical"]').forEach((e) => e.remove()));

    it('is open to search engines: no noindex', async () => {
      await create();
      expect(document.head.querySelector('meta[name="robots"]')?.getAttribute('content') ?? '').not.toContain('noindex');
    });

    it('has its own title, description and canonical address, with the facts it states', async () => {
      await create();
      expect(document.title).toBe('Free Infographic Maker: PNG, JPG and PDF | Indifferent');
      const description = document.head.querySelector('meta[name="description"]')!.getAttribute('content')!;
      expect(description).toContain('20 templates');
      expect(description).toContain('PNG, JPG or PDF');
      expect(description).toContain('nothing is uploaded');
      expect(document.head.querySelector('link[rel="canonical"]')!.getAttribute('href')).toBe('https://indifferent.fun/tools/infographic-maker');
    });

    it('describes the tool as free software for structured data', async () => {
      await create();
      const ld = JSON.parse(document.head.querySelector('script[data-seo-id="software-infographic"]')!.textContent!);
      expect(ld['@type']).toBe('SoftwareApplication');
      expect(ld.offers.price).toBe('0');
      expect(ld.url).toBe('https://indifferent.fun/tools/infographic-maker');
    });
  });

  it('opens with a heading that says what it does, and facts a visitor can trust', async () => {
    await create();
    // The title drawn on the artboard is a picture of text, not a heading of the page.
    expect(el.querySelectorAll('h1').length).toBe(1);
    expect(el.querySelector('header.hero h1')!.textContent).toContain('Make an infographic in your browser.');
    const facts = Array.from(el.querySelectorAll('.facts li')).map((l) => l.textContent!.trim());
    expect(facts).toEqual(['Processed in your browser', 'No account needed', '20 templates, 3 sizes']);
  });

  it('starts with the first template and no saved draft', async () => {
    await create();
    expect(title()).toBe(TEMPLATES[0].title);
    expect(byTest('restored')).toBeNull();
    expect(button('Clear saved draft')).toBeUndefined();
    expect(button('Undo').disabled).toBeTrue();
    expect(button('Redo').disabled).toBeTrue();
  });

  describe('editing', () => {
    beforeEach(async () => create());

    it('shows what is typed on the artboard', async () => {
      await type('#ig-title', 'Brand new title');
      expect(byTest('artboard')!.querySelector('.title')!.textContent).toContain('Brand new title');
    });

    it('turns pasted ligatures into letters', async () => {
      await type('#ig-title', 'o\ufb03ce');
      expect(title()).toBe('office');
    });

    it('limits the length of each field', () => {
      expect(q('#ig-title')!.getAttribute('maxlength')).toBe(String(LIMITS.title));
      expect(q('#ig-footer')!.getAttribute('maxlength')).toBe(String(LIMITS.footer));
    });

    it('undoes and redoes with the buttons, and typing is one step', async () => {
      const first = title();
      await type('#ig-title', 'A');
      await type('#ig-title', 'AB');
      await type('#ig-title', 'ABC');
      await click(button('Undo'));
      expect(title()).toBe(first);
      expect(button('Undo').disabled).toBeTrue();
      await click(button('Redo'));
      expect(title()).toBe('ABC');
      expect(button('Redo').disabled).toBeTrue();
    });

    it('undoes with Ctrl+Z outside a text box and leaves it alone inside one', async () => {
      const first = title();
      await type('#ig-title', 'Changed');
      const inBox = new KeyboardEvent('keydown', { key: 'z', ctrlKey: true, bubbles: true, cancelable: true });
      q('#ig-title')!.dispatchEvent(inBox);
      await settle();
      expect(inBox.defaultPrevented).toBeFalse();
      expect(title()).toBe('Changed');
      const outside = new KeyboardEvent('keydown', { key: 'z', ctrlKey: true, bubbles: true, cancelable: true });
      document.body.dispatchEvent(outside);
      await settle();
      expect(outside.defaultPrevented).toBeTrue();
      expect(title()).toBe(first);
      document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'z', ctrlKey: true, shiftKey: true, bubbles: true }));
      await settle();
      expect(title()).toBe('Changed');
      document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'z', ctrlKey: true, bubbles: true }));
      document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'y', metaKey: true, bubbles: true }));
      await settle();
      expect(title()).toBe('Changed');
    });

    it('announces undo for screen readers', async () => {
      await type('#ig-title', 'Changed');
      await click(button('Undo'));
      expect(byTest('announcement')!.textContent).toBe('Undone.');
    });

    it('changes the size and the colours', async () => {
      await click(q('input[name="format"][value="landscape"]'));
      expect(byTest('artboard')!.style.width).toBe('1920px');
      await click(q('input[name="theme"][value="ocean"]'));
      expect(byTest('artboard')!.style.getPropertyValue('--bg').toLowerCase()).toBe('#eef5f8');
    });

    it('keeps the text when the template changes', async () => {
      await type('#ig-title', 'Mine');
      await click(q('input[name="template"][value="steps"]'));
      expect(title()).toBe('Mine');
      expect(byTest('artboard')!.className).toContain('l-steps');
    });

    it('resets the design but not the text', async () => {
      await type('#ig-title', 'Mine');
      await click(q('input[name="format"][value="square"]'));
      await click(q('input[name="theme"][value="berry"]'));
      await click(button('Reset design'));
      expect(byTest('artboard')!.style.width).toBe('1080px');
      expect(byTest('artboard')!.style.height).toBe('1920px');
      expect(title()).toBe('Mine');
      await click(button('Undo'));
      expect(byTest('artboard')!.style.height).toBe('1080px');
    });

    it('says so when the design is already the default', async () => {
      await click(button('Reset design'));
      expect(byTest('announcement')!.textContent).toContain('already the default');
      expect(button('Undo').disabled).toBeTrue();
    });

    it('turns the mark on and off, off by default', async () => {
      expect(byTest('artboard')!.textContent).not.toContain('Made with indifferent.fun');
      await click(q('button[role="switch"]'));
      expect(byTest('artboard')!.textContent).toContain('Made with indifferent.fun');
      expect(q('button[role="switch"]')!.getAttribute('aria-checked')).toBe('true');
    });
  });

  describe('sections', () => {
    beforeEach(async () => create());
    const ids = () => Array.from(el.querySelectorAll<HTMLElement>('[data-section]')).map((s) => s.dataset['section']!);

    it('moves a section down and up with the buttons and keeps focus on a button', async () => {
      const before = headings();
      const [first] = ids();
      await click(q(`#ig-down-${first}`));
      expect(headings()).toEqual([before[1], before[0], ...before.slice(2)]);
      expect(byTest('announcement')!.textContent).toContain('position 2');
      await wait(30);
      await settle();
      expect(document.activeElement!.id).toBe(`ig-down-${first}`);
      await click(q(`#ig-up-${first}`));
      expect(headings()).toEqual(before);
    });

    it('cannot move the first section up or the last down', () => {
      const all = ids();
      expect(q<HTMLButtonElement>(`#ig-up-${all[0]}`)!.disabled).toBeTrue();
      expect(q<HTMLButtonElement>(`#ig-down-${all[all.length - 1]}`)!.disabled).toBeTrue();
    });

    it('adds a section and focuses its heading', async () => {
      const n = ids().length;
      await click(q('#ig-add'));
      expect(ids().length).toBe(n + 1);
      await wait(30);
      await settle();
      expect(document.activeElement!.id).toMatch(/^ig-heading-/);
    });

    it('stops adding at the template maximum', async () => {
      for (let i = 0; i < 20; i++) if (!q<HTMLButtonElement>('#ig-add')!.disabled) await click(q('#ig-add'));
      expect(ids().length).toBe(TEMPLATES[0].maxSections);
      expect(q<HTMLButtonElement>('#ig-add')!.disabled).toBeTrue();
    });

    it('duplicates a section, then removes it', async () => {
      const n = ids().length;
      const before = headings();
      await click(el.querySelector('[aria-label="Duplicate section 1"]'));
      expect(ids().length).toBe(n + 1);
      expect(headings()[1]).toBe(before[0]);
      await click(el.querySelector('[aria-label="Remove section 2"]'));
      expect(headings()).toEqual(before);
    });

    it('cannot remove below the template minimum', async () => {
      await click(q('input[name="template"][value="compare"]'));
      const min = templateById('compare')!.minSections;
      expect(ids().length).toBe(min);
      expect(el.querySelector<HTMLButtonElement>('[aria-label="Remove section 1"]')!.disabled).toBeTrue();
    });
  });

  describe('the draft', () => {
    it('saves a moment after a change, not on every key and not on load', async () => {
      await create();
      await wait(650);
      expect(saved()).toBeNull();
      typeNow('#ig-title', 'Ke');
      typeNow('#ig-title', 'Kept');
      expect(saved()).toBeNull();
      await wait(300);
      expect(saved()).toBeNull();
      await wait(650);
      await settle();
      expect(saved()!.title).toBe('Kept');
      expect(byTest('save-status')!.textContent).toContain('Draft saved in this browser');
      expect(button('Clear saved draft')).toBeDefined();
    });

    it('restores a draft, says so, and does not write it back untouched', async () => {
      const draft = newDoc(TEMPLATES[1], { title: 'From before' });
      await create(draft);
      expect(title()).toBe('From before');
      expect(byTest('restored')).not.toBeNull();
      const raw = storage.data.get(DRAFT_KEY);
      storage.data.set(DRAFT_KEY, raw + ' ');
      await wait(650);
      expect(storage.data.get(DRAFT_KEY)).toBe(raw + ' ');
      expect(button('Undo').disabled).toBeTrue();
    });

    it('hides the restored notice after the first edit', async () => {
      await create(newDoc(TEMPLATES[0], { title: 'From before' }));
      await type('#ig-title', 'Now');
      expect(byTest('restored')).toBeNull();
    });

    it('ignores a damaged draft', async () => {
      storage.setItem(DRAFT_KEY, '{broken');
      await create();
      expect(title()).toBe(TEMPLATES[0].title);
    });

    it('clears the saved draft and does not save the pending change', async () => {
      await create();
      await type('#ig-title', 'Soon gone');
      await wait(650);
      await settle();
      await type('#ig-title', 'Pending');
      await click(button('Clear saved draft'));
      await wait(650);
      expect(saved()).toBeNull();
      expect(button('Clear saved draft')).toBeUndefined();
      await type('#ig-title', 'Again');
      await wait(650);
      await settle();
      expect(saved()!.title).toBe('Again');
    });

    it('does not bring a cleared draft back when the page is closed', async () => {
      await create();
      await type('#ig-title', 'First');
      await wait(650);
      fixture.detectChanges();
      typeNow('#ig-title', 'Pending');
      await click(button('Clear saved draft'));
      fixture.destroy();
      expect(saved()).toBeNull();
    });

    it('saves the last change when the page is closed', async () => {
      await create();
      await type('#ig-title', 'Last words');
      fixture.destroy();
      expect(saved()!.title).toBe('Last words');
    });

    it('says when the browser will not keep a draft', async () => {
      await create(undefined, null);
      expect(byTest('save-status')!.textContent).toContain("won't keep a draft");
      await type('#ig-title', 'Still works');
      expect(byTest('artboard')!.textContent).toContain('Still works');
    });
  });

  describe('start new', () => {
    beforeEach(async () => create());

    it('asks first, with the safe choice focused, and keeps the work on cancel', async () => {
      await type('#ig-title', 'Mine');
      await click(q('#ig-new'));
      expect(q('[aria-label="Confirm start new"]')).not.toBeNull();
      await wait(30);
      await settle();
      expect(document.activeElement!.id).toBe('ig-keep');
      await click(q('#ig-keep'));
      expect(title()).toBe('Mine');
      await wait(30);
      expect(document.activeElement!.id).toBe('ig-new');
    });

    it('replaces the infographic with the template text and can be undone', async () => {
      await type('#ig-title', 'Mine');
      await click(q('#ig-new'));
      const confirm = q('[aria-label="Confirm start new"]')!;
      await click(Array.from(confirm.querySelectorAll('button')).find((b) => b.textContent!.trim() === 'Start new') as HTMLElement);
      expect(title()).toBe(TEMPLATES[0].title);
      expect(q('[aria-label="Confirm start new"]')).toBeNull();
      await click(button('Undo'));
      expect(title()).toBe('Mine');
    });
  });

  describe('statistics templates', () => {
    beforeEach(async () => create());
    const pick = (id: string) => click(q(`input[name="template"][value="${id}"]`));

    it('offers the four statistics templates', () => {
      for (const id of ['bar-chart', 'line-chart', 'donut-chart', 'big-numbers']) expect(q(`input[name="template"][value="${id}"]`)).withContext(id).not.toBeNull();
    });

    it('calls the heading box "Value", hides the text box and shows the hint', async () => {
      await pick('bar-chart');
      expect(el.querySelector('label[for^="ig-heading-"]')!.textContent!.trim()).toBe('Value');
      expect(el.querySelector('textarea[id^="ig-body-"]')).toBeNull();
      expect(byTest('template-hint')!.textContent).toContain('Type a number');
    });

    it('keeps the text box for key numbers, which show a note', async () => {
      await pick('big-numbers');
      expect(el.querySelector('label[for^="ig-heading-"]')!.textContent!.trim()).toBe('Number');
      expect(el.querySelector('textarea[id^="ig-body-"]')).not.toBeNull();
    });

    it('draws the starting data of each without any warning', async () => {
      for (const id of ['bar-chart', 'line-chart', 'donut-chart', 'big-numbers']) {
        await pick(id);
        expect(byTest('data-issues')).withContext(id).toBeNull();
        expect(byTest('artboard')!.querySelector('app-infographic-chart')).withContext(id).not.toBeNull();
      }
    });

    it('says which number cannot be read, and stops when it is fixed', async () => {
      await pick('bar-chart');
      const heading = q<HTMLInputElement>('input[id^="ig-heading-"]')!;
      await type(`#${heading.id}`, 'lots');
      const warning = byTest('data-issues')!;
      expect(warning.textContent).toContain('Section 1');
      expect(warning.textContent).toContain('type one number');
      await type(`#${heading.id}`, '12');
      expect(byTest('data-issues')).toBeNull();
    });

    it('reads Hindi digits without a warning', async () => {
      await pick('bar-chart');
      await type(`#${q<HTMLInputElement>('input[id^="ig-heading-"]')!.id}`, '१२');
      expect(byTest('data-issues')).toBeNull();
    });

    it('switching between text and statistics gives sample sections, says so, and undo brings the text back', async () => {
      const before = headings();
      await type('#ig-title', 'Mine');
      await pick('donut-chart');
      expect(byTest('data-issues')).toBeNull();
      expect(title()).toBe('Mine');
      expect(byTest('announcement')!.textContent).toContain('Sections replaced');
      await click(button('Undo'));
      expect(headings()).toEqual(before);
    });

    it('does not say that between two text templates', async () => {
      await pick('steps');
      expect(byTest('announcement')!.textContent).not.toContain('Sections replaced');
    });
  });

  describe('the template picker', () => {
    beforeEach(async () => create());
    const pick = (id: string) => click(q(`input[name="template"][value="${id}"]`));

    it('lists all twenty templates under seven category headings', () => {
      expect(el.querySelectorAll('input[name="template"]').length).toBe(20);
      expect(Array.from(el.querySelectorAll('.cat')).map((c) => c.textContent!.trim())).toEqual(
        ['Educational', 'Step by step', 'Comparison', 'Timeline', 'Statistics', 'Process', 'Mind map'],
      );
    });

    it('is one radio group, so the arrow keys move through every template', () => {
      expect(el.querySelectorAll('[role="radiogroup"][aria-label="Template"]').length).toBe(1);
      const names = new Set(Array.from(el.querySelectorAll<HTMLInputElement>('input[name="template"]')).map((i) => i.name));
      expect(names.size).toBe(1);
    });

    it('holds a fixed two sections for pros and cons, and says so', async () => {
      await pick('pros-cons');
      expect(el.querySelectorAll('[data-section]').length).toBe(2);
      expect(q<HTMLButtonElement>('#ig-add')!.disabled).toBeTrue();
      expect(el.textContent).toContain('holds up to 2 sections');
      expect(el.querySelector<HTMLButtonElement>('[aria-label="Remove section 1"]')!.disabled).toBeTrue();
      expect(byTest('template-hint')!.textContent).toContain('one point on each line');
    });

    it('lets the cycle take up to six steps and no text box', async () => {
      await pick('cycle');
      for (let i = 0; i < 10; i++) if (!q<HTMLButtonElement>('#ig-add')!.disabled) await click(q('#ig-add'));
      expect(el.querySelectorAll('[data-section]').length).toBe(6);
      expect(el.querySelector('textarea[id^="ig-body-"]')).toBeNull();
    });

    it('names the branches of a mind map and the groups of a tree', async () => {
      await pick('mind-map');
      expect(el.querySelector('label[for^="ig-heading-"]')!.textContent!.trim()).toBe('Branch');
      await pick('tree');
      expect(el.querySelector('label[for^="ig-heading-"]')!.textContent!.trim()).toBe('Group');
      expect(el.querySelector('label[for^="ig-body-"]')!.textContent!.trim()).toBe('Bullets');
    });

    it('keeps the sections when moving between two text templates', async () => {
      const before = headings();
      await pick('staircase');
      expect(headings().slice(0, 2)).toEqual(before.slice(0, 2));
    });

    it('keeps the text of the title for a mind map, drawn once on the artboard', async () => {
      await type('#ig-title', 'Centre');
      await pick('mind-map');
      expect(title()).toBe('Centre');
      expect(byTest('artboard')!.textContent!.split('Centre').length - 1).toBe(1);
    });
  });

  describe('icons and shapes', () => {
    beforeEach(async () => create());
    const decos = () => Array.from(el.querySelectorAll<HTMLElement>('[data-testid="deco"]'));

    it('are added from the panel and drawn on the artboard', async () => {
      await click(q('[aria-label="Add circle"]'));
      await click(q('[aria-label="Add light bulb icon"]'));
      expect(decos().length).toBe(2);
      expect(el.querySelectorAll('li[data-item]').length).toBe(2);
      expect(byTest('announcement')!.textContent).toContain('added as item 2');
    });

    it('move with the arrow keys and add up when the key is held, and one undo takes back the held key', async () => {
      await click(q('[aria-label="Add circle"]'));
      const first = decos()[0];
      const x0 = parseFloat(first.style.left);
      for (let i = 0; i < 4; i++) first.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true, cancelable: true }));
      await settle();
      expect(parseFloat(decos()[0].style.left)).toBeCloseTo(x0 + 4, 5);
      await click(button('Undo'));
      expect(parseFloat(decos()[0].style.left)).toBeCloseTo(x0, 5);
    });

    it('are removed with Delete, and undo brings them back', async () => {
      await click(q('[aria-label="Add circle"]'));
      decos()[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'Delete', bubbles: true, cancelable: true }));
      await settle();
      expect(decos().length).toBe(0);
      expect(byTest('announcement')!.textContent).toContain('Undo brings it back');
      await click(button('Undo'));
      expect(decos().length).toBe(1);
    });

    it('are saved with the draft and come back after a reload', async () => {
      await click(q('[aria-label="Add circle"]'));
      await wait(650);
      expect(saved()!.items.length).toBe(1);
      fixture.destroy();
      await create();
      expect(decos().length).toBe(1);
    });

    it('are removed by Reset design, which says so, and undo brings them back', async () => {
      await click(q('[aria-label="Add circle"]'));
      await click(button('Reset design'));
      expect(decos().length).toBe(0);
      expect(byTest('announcement')!.textContent).toContain('Icons and shapes were removed');
      await click(button('Undo'));
      expect(decos().length).toBe(1);
    });

    it('are kept when the template changes', async () => {
      await click(q('[aria-label="Add circle"]'));
      await click(q('input[name="template"][value="cycle"]'));
      expect(decos().length).toBe(1);
    });

    it('are gone after Start new', async () => {
      await click(q('[aria-label="Add circle"]'));
      await click(q('#ig-new'));
      const confirm = q('[aria-label="Confirm start new"]')!;
      await click(Array.from(confirm.querySelectorAll('button')).find((b) => b.textContent!.trim() === 'Start new') as HTMLElement);
      expect(decos().length).toBe(0);
    });
  });

  describe('languages', () => {
    beforeEach(async () => create());

    it('says nothing about review for English, and says Hindi is not yet reviewed by a reader', async () => {
      expect(byTest('lang-note')).toBeNull();
      await type('#ig-title', 'कहानी');
      const note = byTest('lang-note')!.textContent!;
      expect(note).toContain('Hindi');
      expect(note).toContain('has not reviewed the output yet');
      expect(note).toContain('never translated or changed');
    });

    it('names the language that was chosen by hand, not the one detected', async () => {
      const select = q<HTMLSelectElement>('#ig-lang')!;
      select.value = 'gu';
      select.dispatchEvent(new Event('change'));
      await settle();
      expect(byTest('lang-note')!.textContent).toContain('Gujarati');
    });


    it('shows Hindi and Gujarati with no warning about fonts', async () => {
      await type('#ig-title', 'कहानी से वीडियो ક');
      expect(byTest('unsupported')).toBeNull();
      expect(byTest('artboard')!.getAttribute('lang')).toBe('hi');
    });

    it('warns that Arabic has no font yet and mirrors the artboard', async () => {
      await type('#ig-title', 'مرحبا بالعالم');
      expect(byTest('unsupported')!.textContent).toContain('Arabic');
      expect(byTest('artboard')!.getAttribute('dir')).toBe('rtl');
    });

    it('offers only the released languages', () => {
      const options = Array.from(el.querySelectorAll<HTMLOptionElement>('#ig-lang option')).map((o) => o.value);
      expect(options).toContain('hi');
      expect(options).toContain('gu');
      expect(options).not.toContain('ar');
    });
  });

  describe('downloading', () => {
    beforeEach(async () => {
      await create();
      spyOn(HTMLAnchorElement.prototype, 'click').and.stub();
    });

    it('makes a PNG and reports the file', async () => {
      byTest('export-png')!.click();
      await flush();
      expect(exporter.calls).toEqual(['png']);
      expect(byTest('export-message')!.textContent).toContain('Saved x.png (1080 × 1920)');
    });

    it('makes a JPG', async () => {
      byTest('export-jpg')!.click();
      await flush();
      expect(exporter.calls).toEqual(['jpg']);
    });

    it('warns about characters with no font', async () => {
      exporter.uncovered = ['\u9f18'];
      byTest('export-png')!.click();
      await flush();
      expect(byTest('export-message')!.textContent).toContain('\u9f18');
      expect(byTest('export-message')!.className).toContain('ui-alert--warn');
    });

    it('shows a plain error when the image cannot be made, and allows another try', async () => {
      exporter.fail = true;
      byTest('export-png')!.click();
      await flush();
      expect(byTest('export-message')!.textContent).toContain('could not be made');
      expect((byTest('export-png') as HTMLButtonElement).disabled).toBeFalse();
    });

    it('makes a PDF and says plainly that its words cannot be selected', async () => {
      byTest('export-pdf')!.click();
      await flush();
      expect(exporter.calls).toEqual(['pdf']);
      const message = byTest('export-message')!.textContent!;
      expect(message).toContain('Saved x.pdf: one page, 1080 × 1920 pixels');
      expect(message).toContain('cannot be selected or searched');
    });

    it('disables all three download buttons while one is being made', async () => {
      exporter.slow = true;
      byTest('export-pdf')!.click();
      fixture.detectChanges();
      for (const id of ['export-png', 'export-jpg', 'export-pdf']) expect((byTest(id) as HTMLButtonElement).disabled).withContext(id).toBeTrue();
      expect(byTest('export-pdf')!.textContent).toContain('Making the PDF');
      exporter.release();
      await flush();
      expect((byTest('export-pdf') as HTMLButtonElement).disabled).toBeFalse();
    });

    it('shows the real image size', () => {
      expect(el.textContent).toContain('1080 × 1920 pixels');
    });
  });
});
