import { Injectable, InjectionToken, inject } from '@angular/core';
import { glyphById } from './decor-library';
import { clampTo } from './doc-ops';
import { DECOR_COLORS, DIRECTIONS, DecorColor, Decoration, Direction, FORMAT_IDS, FormatId, InfographicDoc, LIMITS, Section } from './infographic.model';
import { isReleasedLanguage } from './languages';
import { TEMPLATES, Template, blankSection, newDoc, newSectionId, templateById } from './templates';
import { THEMES } from './themes';
import { normalizeText } from './text-utils';

/** Where the draft lives: this browser's local storage, or nothing when the browser blocks it. */
export const INFOGRAPHIC_STORAGE = new InjectionToken<Storage | null>('infographic-storage', {
  providedIn: 'root',
  factory: () => {
    try {
      return typeof localStorage === 'undefined' ? null : localStorage;
    } catch {
      return null; // storage switched off, as in some private modes
    }
  },
});

export const DRAFT_KEY = 'indifferent.infographic.draft.v1';

function text(value: unknown, max: number, multiline = false): string {
  if (typeof value !== 'string') return '';
  return Array.from(normalizeText(value, multiline)).slice(0, max).join('');
}

/**
 * Turns whatever was stored into a document the maker can use, or null when it
 * is not one. The stored text is the user's own, but it may be old, cut off or
 * edited by hand, so nothing is trusted: unknown choices fall back to the
 * defaults, texts are cleaned and cut to their limits, and the number of
 * sections is brought inside what the template allows.
 */
export function sanitizeDoc(raw: unknown): InfographicDoc | null {
  if (typeof raw !== 'object' || raw === null || Array.isArray(raw)) return null;
  const r = raw as Record<string, unknown>;
  if (r['version'] !== 1) return null;
  const template: Template = templateById(String(r['templateId'])) ?? TEMPLATES[0];
  const base = newDoc(template);

  const seen = new Set<string>();
  const sections: Section[] = [];
  if (Array.isArray(r['sections'])) {
    for (const item of r['sections']) {
      if (sections.length >= Math.min(template.maxSections, LIMITS.sections)) break;
      if (typeof item !== 'object' || item === null) continue;
      const s = item as Record<string, unknown>;
      let id = typeof s['id'] === 'string' && /^[\w-]{1,40}$/.test(s['id']) ? s['id'] : newSectionId();
      if (seen.has(id)) id = newSectionId();
      seen.add(id);
      sections.push({
        id,
        label: text(s['label'], LIMITS.label),
        heading: text(s['heading'], LIMITS.heading),
        body: text(s['body'], LIMITS.body, true),
      });
    }
  }
  while (sections.length < template.minSections) sections.push(blankSection());

  const items: Decoration[] = [];
  if (Array.isArray(r['items'])) {
    for (const raw of r['items']) {
      if (items.length >= LIMITS.items) break;
      if (typeof raw !== 'object' || raw === null) continue;
      const it = raw as Record<string, unknown>;
      if (typeof it['glyph'] !== 'string' || !glyphById(it['glyph'])) continue;
      let id = typeof it['id'] === 'string' && /^[\w-]{1,40}$/.test(it['id']) ? it['id'] : newSectionId();
      if (seen.has(id)) id = newSectionId();
      seen.add(id);
      const num = (v: unknown, fallback: number) => (typeof v === 'number' ? v : fallback);
      items.push({
        id,
        glyph: it['glyph'],
        x: clampTo(num(it['x'], 50), 0, 100),
        y: clampTo(num(it['y'], 50), 0, 100),
        size: clampTo(num(it['size'], 14), LIMITS.itemMin, LIMITS.itemMax),
        color: DECOR_COLORS.includes(it['color'] as DecorColor) ? (it['color'] as DecorColor) : 'accent',
      });
    }
  }
  const language = typeof r['language'] === 'string' && (r['language'] === 'auto' || isReleasedLanguage(r['language'])) ? r['language'] : 'auto';
  return {
    version: 1,
    templateId: template.id,
    format: FORMAT_IDS.includes(r['format'] as FormatId) ? (r['format'] as FormatId) : base.format,
    themeId: THEMES.some((t) => t.id === r['themeId']) ? String(r['themeId']) : base.themeId,
    language,
    direction: DIRECTIONS.includes(r['direction'] as Direction) ? (r['direction'] as Direction) : 'auto',
    title: text(r['title'], LIMITS.title),
    subtitle: text(r['subtitle'], LIMITS.subtitle),
    sections,
    footer: text(r['footer'], LIMITS.footer),
    mark: r['mark'] === true,
    items,
  };
}

/** Reads and writes the draft. Nothing here ever leaves the browser. */
@Injectable({ providedIn: 'root' })
export class DraftStore {
  private readonly storage = inject(INFOGRAPHIC_STORAGE);

  /** True when this browser lets the page keep a draft. */
  get available(): boolean {
    return this.storage !== null;
  }

  load(): InfographicDoc | null {
    if (!this.storage) return null;
    try {
      const raw = this.storage.getItem(DRAFT_KEY);
      return raw ? sanitizeDoc(JSON.parse(raw)) : null;
    } catch {
      return null; // unreadable or not JSON: start fresh rather than fail
    }
  }

  /** Returns false when the draft could not be kept (storage full or blocked). */
  save(doc: InfographicDoc): boolean {
    if (!this.storage) return false;
    try {
      this.storage.setItem(DRAFT_KEY, JSON.stringify(doc));
      return true;
    } catch {
      return false;
    }
  }

  has(): boolean {
    try {
      return !!this.storage?.getItem(DRAFT_KEY);
    } catch {
      return false;
    }
  }

  clear(): void {
    try {
      this.storage?.removeItem(DRAFT_KEY);
    } catch {
      /* nothing to remove */
    }
  }
}
