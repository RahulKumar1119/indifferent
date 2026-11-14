import { TestBed } from '@angular/core/testing';
import { DRAFT_KEY, DraftStore, INFOGRAPHIC_STORAGE, sanitizeDoc } from './draft-store';
import { LIMITS } from './infographic.model';
import { TEMPLATES, newDoc, templateById } from './templates';

class FakeStorage implements Storage {
  data = new Map<string, string>();
  throwOnSet = false;
  get length() {
    return this.data.size;
  }
  clear() {
    this.data.clear();
  }
  getItem(k: string) {
    return this.data.get(k) ?? null;
  }
  key(i: number) {
    return [...this.data.keys()][i] ?? null;
  }
  removeItem(k: string) {
    this.data.delete(k);
  }
  setItem(k: string, v: string) {
    if (this.throwOnSet) throw new DOMException('full', 'QuotaExceededError');
    this.data.set(k, v);
  }
}

describe('sanitizeDoc', () => {
  const good = () => newDoc(TEMPLATES[0]);

  it('accepts a good document unchanged in content', () => {
    const d = good();
    const out = sanitizeDoc(JSON.parse(JSON.stringify(d)))!;
    expect(out.title).toBe(d.title);
    expect(out.sections.map((s) => s.heading)).toEqual(d.sections.map((s) => s.heading));
    expect(out.templateId).toBe(d.templateId);
  });

  it('rejects things that are not documents', () => {
    for (const bad of [null, undefined, 5, 'x', [], { version: 2 }, {}]) expect(sanitizeDoc(bad)).toBeNull();
  });

  it('falls back to defaults for unknown choices', () => {
    const out = sanitizeDoc({ ...good(), templateId: 'nope', format: 'huge', themeId: 'neon', language: 'xx', direction: 'up' })!;
    expect(out.templateId).toBe(TEMPLATES[0].id);
    expect(out.format).toBe('portrait');
    expect(out.themeId).toBe('paper');
    expect(out.language).toBe('auto');
    expect(out.direction).toBe('auto');
  });

  it('does not accept a language whose font is not released', () => {
    expect(sanitizeDoc({ ...good(), language: 'ar' })!.language).toBe('auto');
    expect(sanitizeDoc({ ...good(), language: 'hi' })!.language).toBe('hi');
  });

  it('cuts texts to their limits by characters, not code units', () => {
    const out = sanitizeDoc({ ...good(), title: '😀'.repeat(500), footer: 'x'.repeat(500) })!;
    expect(Array.from(out.title).length).toBe(LIMITS.title);
    expect(out.footer.length).toBe(LIMITS.footer);
  });

  it('cleans text and ligatures', () => {
    const out = sanitizeDoc({ ...good(), title: 'o\ufb03ce\u0000\n' })!;
    expect(out.title).toBe('office ');
  });

  it('keeps line breaks in the body only', () => {
    const d = good();
    d.sections[0].body = 'a\nb';
    d.sections[0].heading = 'h\ni';
    const out = sanitizeDoc(d)!;
    expect(out.sections[0].body).toBe('a\nb');
    expect(out.sections[0].heading).toBe('h i');
  });

  it('brings the section count inside the template limits', () => {
    const t = templateById('compare')!;
    const many = { ...newDoc(t), sections: Array.from({ length: 30 }, (_, i) => ({ id: 'a' + i, label: '', heading: 'h', body: '' })) };
    expect(sanitizeDoc(many)!.sections.length).toBe(t.maxSections);
    const none = { ...newDoc(t), sections: [] };
    expect(sanitizeDoc(none)!.sections.length).toBe(t.minSections);
    expect(sanitizeDoc({ ...newDoc(t), sections: 'oops' })!.sections.length).toBe(t.minSections);
  });

  it('replaces unsafe or repeated section ids', () => {
    const d = good();
    d.sections[0].id = '<img src=x>';
    d.sections[1].id = 'same';
    d.sections[2].id = 'same';
    const ids = sanitizeDoc(d)!.sections.map((s) => s.id);
    expect(new Set(ids).size).toBe(ids.length);
    expect(ids.every((id) => /^[\w-]{1,40}$/.test(id))).toBeTrue();
  });

  it('skips section entries that are not objects', () => {
    const d = { ...good(), sections: [null, 4, 'x', { id: 'ok', label: '', heading: 'Real', body: '' }] };
    const out = sanitizeDoc(d)!;
    expect(out.sections.some((s) => s.heading === 'Real')).toBeTrue();
  });

  it('turns non-string text into empty text and only true into mark', () => {
    const out = sanitizeDoc({ ...good(), title: 5, mark: 'yes' })!;
    expect(out.title).toBe('');
    expect(out.mark).toBeFalse();
    expect(sanitizeDoc({ ...good(), mark: true })!.mark).toBeTrue();
  });
});

describe('DraftStore', () => {
  let storage: FakeStorage;
  let store: DraftStore;

  function setup(s: Storage | null) {
    TestBed.configureTestingModule({ providers: [{ provide: INFOGRAPHIC_STORAGE, useValue: s }] });
    store = TestBed.inject(DraftStore);
  }

  beforeEach(() => {
    storage = new FakeStorage();
  });

  it('saves, finds, loads and clears a draft', () => {
    setup(storage);
    const d = newDoc(TEMPLATES[1], { title: 'Saved title' });
    expect(store.has()).toBeFalse();
    expect(store.save(d)).toBeTrue();
    expect(store.has()).toBeTrue();
    expect(store.load()!.title).toBe('Saved title');
    store.clear();
    expect(store.has()).toBeFalse();
    expect(store.load()).toBeNull();
  });

  it('starts fresh when the stored text is not JSON or not a document', () => {
    setup(storage);
    storage.setItem(DRAFT_KEY, '{not json');
    expect(store.load()).toBeNull();
    storage.setItem(DRAFT_KEY, '{"version":9}');
    expect(store.load()).toBeNull();
  });

  it('reports a failed save when storage is full', () => {
    setup(storage);
    storage.throwOnSet = true;
    expect(store.save(newDoc(TEMPLATES[0]))).toBeFalse();
  });

  it('works without storage', () => {
    setup(null);
    expect(store.available).toBeFalse();
    expect(store.save(newDoc(TEMPLATES[0]))).toBeFalse();
    expect(store.load()).toBeNull();
    expect(store.has()).toBeFalse();
    expect(() => store.clear()).not.toThrow();
  });

  it('survives storage that throws on read', () => {
    const angry = new FakeStorage();
    angry.getItem = () => {
      throw new Error('blocked');
    };
    setup(angry);
    expect(store.load()).toBeNull();
    expect(store.has()).toBeFalse();
  });
});
