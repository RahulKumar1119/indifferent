import { addSection, docText, duplicateSection, moveSection, removeSection, reorder, resetDesign, setSectionField, startNew, withTemplate } from './doc-ops';
import { DocHistory } from './history';
import { DEFAULT_FORMAT, InfographicDoc, LIMITS } from './infographic.model';
import { isReleasedLanguage, LANGUAGES, unreleasedScripts } from './languages';
import { TEMPLATES, newDoc, templateById } from './templates';

const T = (id: string) => templateById(id)!;

describe('templates', () => {
  it('has unique ids and sensible section limits', () => {
    expect(new Set(TEMPLATES.map((t) => t.id)).size).toBe(TEMPLATES.length);
    for (const t of TEMPLATES) {
      expect(t.minSections).toBeGreaterThan(0);
      expect(t.maxSections).toBeGreaterThanOrEqual(t.minSections);
      expect(t.maxSections).toBeLessThanOrEqual(LIMITS.sections);
      expect(t.sections.length).toBeGreaterThanOrEqual(t.minSections);
      expect(t.sections.length).toBeLessThanOrEqual(t.maxSections);
    }
  });

  it('starts every template inside the text limits', () => {
    for (const t of TEMPLATES) {
      const d = newDoc(t);
      expect(Array.from(d.title).length).toBeLessThanOrEqual(LIMITS.title);
      for (const s of d.sections) {
        expect(Array.from(s.heading).length).toBeLessThanOrEqual(LIMITS.heading);
        expect(Array.from(s.body).length).toBeLessThanOrEqual(LIMITS.body);
        expect(Array.from(s.label).length).toBeLessThanOrEqual(LIMITS.label);
      }
    }
  });

  it('gives each new section its own id', () => {
    const ids = newDoc(T('key-points')).sections.map((s) => s.id);
    expect(new Set(ids).size).toBe(ids.length);
  });
});

describe('doc-ops', () => {
  const kp = () => T('key-points');
  const doc = () => newDoc(kp());

  it('switching template keeps the content', () => {
    const d = { ...doc(), title: 'My own title' };
    const next = withTemplate(d, T('steps'));
    expect(next.templateId).toBe('steps');
    expect(next.title).toBe('My own title');
    expect(next.sections[0].heading).toBe(d.sections[0].heading);
  });

  it('switching to the same template returns the same document', () => {
    const d = doc();
    expect(withTemplate(d, kp())).toBe(d);
  });

  it('switching to a template with fewer slots drops extra sections, and more slots fills them', () => {
    const compare = T('compare');
    let d = doc();
    for (let i = 0; i < 20; i++) d = addSection(d, kp());
    expect(d.sections.length).toBeGreaterThan(compare.maxSections);
    expect(withTemplate(d, compare).sections.length).toBe(compare.maxSections);
    const one = { ...d, sections: d.sections.slice(0, 1) };
    expect(withTemplate(one, compare).sections.length).toBe(compare.minSections);
  });

  it('replaces the sections when going between a text template and a statistics one, and keeps the title', () => {
    const d = { ...doc(), title: 'Mine', footer: 'Foot' };
    const toChart = withTemplate(d, T('bar-chart'));
    expect(toChart.title).toBe('Mine');
    expect(toChart.footer).toBe('Foot');
    expect(toChart.sections.map((s) => s.heading)).toEqual(T('bar-chart').sections.map((s) => s.heading));
    expect(toChart.sections.some((s) => d.sections.some((o) => o.id === s.id))).toBeFalse();
    const back = withTemplate(toChart, T('steps'));
    expect(back.sections.map((s) => s.heading)).toEqual(T('steps').sections.map((s) => s.heading));
  });

  it('keeps the sections between two statistics templates', () => {
    const d = withTemplate(doc(), T('bar-chart'));
    const edited = setSectionField(d, d.sections[0].id, 'heading', '99');
    const next = withTemplate(edited, T('donut-chart'));
    expect(next.sections[0].heading).toBe('99');
    expect(next.sections[0].id).toBe(d.sections[0].id);
  });

  it('adds a section until the template is full, then does nothing', () => {
    let d = doc();
    for (let i = 0; i < 20; i++) d = addSection(d, kp());
    expect(d.sections.length).toBe(kp().maxSections);
    expect(addSection(d, kp())).toBe(d);
  });

  it('removes a section but never below the template minimum', () => {
    let d = doc();
    for (let i = 0; i < 20; i++) d = removeSection(d, kp(), d.sections[0].id);
    expect(d.sections.length).toBe(kp().minSections);
    expect(removeSection(d, kp(), 'missing')).toBe(d);
  });

  it('reorders and moves, and ignores impossible moves', () => {
    const d = doc();
    const ids = d.sections.map((s) => s.id);
    const moved = reorder(d, 0, 2);
    expect(moved.sections.map((s) => s.id)).toEqual([ids[1], ids[2], ids[0], ...ids.slice(3)]);
    expect(reorder(d, 0, 0)).toBe(d);
    expect(reorder(d, -1, 1)).toBe(d);
    expect(reorder(d, 0, 99)).toBe(d);
    expect(moveSection(d, ids[0], -1)).toBe(d); // already first
    expect(moveSection(d, ids[ids.length - 1], 1)).toBe(d); // already last
    expect(moveSection(d, ids[0], 1).sections[1].id).toBe(ids[0]);
    expect(moveSection(d, 'missing', 1)).toBe(d);
  });

  it('duplicates right after the original with a new id and the same text', () => {
    const d = { ...doc() };
    const small = removeSection(d, kp(), d.sections[d.sections.length - 1].id);
    const dup = duplicateSection(small, kp(), small.sections[0].id);
    expect(dup.sections.length).toBe(small.sections.length + 1);
    expect(dup.sections[1].heading).toBe(small.sections[0].heading);
    expect(dup.sections[1].id).not.toBe(small.sections[0].id);
    expect(duplicateSection(small, kp(), 'missing')).toBe(small);
  });

  it('does not duplicate past the maximum', () => {
    let d = doc();
    for (let i = 0; i < 20; i++) d = addSection(d, kp());
    expect(duplicateSection(d, kp(), d.sections[0].id)).toBe(d);
  });

  it('sets one field and leaves the original unchanged', () => {
    const d = doc();
    const id = d.sections[1].id;
    const next = setSectionField(d, id, 'heading', 'Changed');
    expect(next.sections[1].heading).toBe('Changed');
    expect(d.sections[1].heading).not.toBe('Changed');
    expect(setSectionField(next, id, 'heading', 'Changed')).toBe(next);
    expect(setSectionField(d, 'missing', 'heading', 'x')).toBe(d);
  });

  it('resets the design but keeps the content', () => {
    const d: InfographicDoc = { ...doc(), title: 'Keep me', format: 'landscape', themeId: 'ocean', language: 'hi', direction: 'rtl', mark: true };
    const r = resetDesign(d);
    expect(r.title).toBe('Keep me');
    expect(r.sections).toBe(d.sections);
    expect(r.format).toBe(DEFAULT_FORMAT);
    expect(r.themeId).toBe('paper');
    expect(r.language).toBe('auto');
    expect(r.direction).toBe('auto');
    expect(r.mark).toBeFalse();
    expect(resetDesign(r)).toBe(r);
  });

  it('starts new with the template text and default design', () => {
    const d = startNew(T('steps'));
    expect(d.templateId).toBe('steps');
    expect(d.title).toBe(T('steps').title);
    expect(d.mark).toBeFalse();
  });

  it('joins all text for language detection', () => {
    const d = { ...doc(), title: 'AAA', footer: 'ZZZ' };
    const t = docText(d);
    expect(t).toContain('AAA');
    expect(t).toContain('ZZZ');
    expect(t).toContain(d.sections[0].heading);
  });
});

describe('DocHistory', () => {
  const a = newDoc(T('key-points'));
  const edit = (d: InfographicDoc, title: string): InfographicDoc => ({ ...d, title });

  it('undoes and redoes', () => {
    const h = new DocHistory(a);
    const b = edit(a, 'b');
    expect(h.push(b)).toBeTrue();
    expect(h.canUndo).toBeTrue();
    expect(h.undo()).toBe(a);
    expect(h.canRedo).toBeTrue();
    expect(h.redo()).toBe(b);
    expect(h.undo()).toBe(a);
    expect(h.undo()).toBeNull();
  });

  it('ignores the same document', () => {
    const h = new DocHistory(a);
    expect(h.push(a)).toBeFalse();
    expect(h.canUndo).toBeFalse();
  });

  it('drops redo steps after a new change', () => {
    const h = new DocHistory(a);
    h.push(edit(a, 'b'));
    h.undo();
    h.push(edit(a, 'c'));
    expect(h.canRedo).toBeFalse();
    expect(h.redo()).toBeNull();
  });

  it('merges typing in the same field within the window into one step', () => {
    const h = new DocHistory(a);
    h.push(edit(a, 'b'), 'title', 1000);
    h.push(edit(a, 'bc'), 'title', 1300);
    h.push(edit(a, 'bcd'), 'title', 1700);
    expect(h.undo()).toBe(a);
    expect(h.canUndo).toBeFalse();
  });

  it('does not merge after the window or across fields', () => {
    const h = new DocHistory(a);
    const b = edit(a, 'b');
    h.push(b, 'title', 1000);
    const c = edit(a, 'bc');
    h.push(c, 'title', 5000);
    expect(h.undo()).toBe(b);
    const h2 = new DocHistory(a);
    const b2 = edit(a, 'b');
    h2.push(b2, 'title', 1000);
    h2.push({ ...b2, subtitle: 's' }, 'subtitle', 1100);
    expect(h2.undo()).toBe(b2);
  });

  it('never merges changes without a key', () => {
    const h = new DocHistory(a);
    const b = edit(a, 'b');
    h.push(b, '', 1000);
    h.push(edit(a, 'c'), '', 1001);
    expect(h.undo()).toBe(b);
  });

  it('keeps at most `limit` steps', () => {
    const h = new DocHistory(a, 3);
    let d = a;
    for (let i = 0; i < 10; i++) {
      d = edit(d, 't' + i);
      h.push(d);
    }
    let n = 0;
    while (h.undo()) n++;
    expect(n).toBe(3);
  });

  it('reset clears both stacks', () => {
    const h = new DocHistory(a);
    h.push(edit(a, 'b'));
    h.undo();
    h.reset(a);
    expect(h.canUndo).toBeFalse();
    expect(h.canRedo).toBeFalse();
    expect(h.present).toBe(a);
  });
});

describe('languages', () => {
  it('lists batch 1 only, with codes the canvas can use', () => {
    const codes = LANGUAGES.map((l) => l.code);
    for (const c of ['en', 'hi', 'gu', 'mr', 'ru', 'vi']) expect(codes).toContain(c);
    for (const c of ['ar', 'ja', 'ko', 'zh-Hans', 'th', 'ta']) expect(codes).not.toContain(c);
  });

  it('knows which languages are released', () => {
    expect(isReleasedLanguage('hi')).toBeTrue();
    expect(isReleasedLanguage('ar')).toBeFalse();
    expect(isReleasedLanguage('xx')).toBeFalse();
  });

  it('reports scripts that have no font yet', () => {
    expect(unreleasedScripts(['latin', 'devanagari', 'gujarati'])).toEqual([]);
    expect(unreleasedScripts(['latin', 'arabic', 'thai'])).toEqual(['arabic', 'thai']);
  });
});
