import { DEFAULT_FORMAT, DEFAULT_THEME_ID, InfographicDoc, LIMITS, Section } from './infographic.model';
import { Template, blankSection, newSectionId, newDoc, templateById } from './templates';

/**
 * Changes to a document, each returning a new one and never editing the old,
 * so undo is just keeping the old ones. A change that cannot be made (a
 * template has no room for another section) returns the same document, which
 * callers read as "nothing happened".
 */

/** All the text of a document in one string, to tell its language, direction and writing systems. */
export function docText(doc: InfographicDoc): string {
  return [doc.title, doc.subtitle, doc.footer, ...doc.sections.flatMap((s) => [s.label, s.heading, s.body])].join('\n');
}

function maxSections(template: Template): number {
  return Math.min(template.maxSections, LIMITS.sections);
}

/** True when moving between these two templates cannot keep the sections: text sections are not numbers, and numbers are not text. */
export function replacesSections(from: Template | undefined, to: Template): boolean {
  return !!from && !!from.chart !== !!to.chart;
}

/**
 * Switches template. The title, subtitle and footer stay. The sections stay too,
 * with sections added or dropped to fit, unless the switch is between a text
 * template and a statistics one: then they are replaced by the new template's
 * sample sections, since a paragraph is not a number.
 */
export function withTemplate(doc: InfographicDoc, template: Template): InfographicDoc {
  if (doc.templateId === template.id) return doc;
  if (replacesSections(templateById(doc.templateId), template)) {
    return { ...doc, templateId: template.id, sections: template.sections.map((s) => ({ id: newSectionId(), ...s })) };
  }
  let sections = doc.sections.slice(0, maxSections(template));
  for (let i = sections.length; i < template.minSections; i++) {
    const own = template.sections[i];
    sections = [...sections, own ? { id: newSectionId(), ...own } : blankSection()];
  }
  return { ...doc, templateId: template.id, sections };
}

export function addSection(doc: InfographicDoc, template: Template): InfographicDoc {
  if (doc.sections.length >= maxSections(template)) return doc;
  return { ...doc, sections: [...doc.sections, blankSection()] };
}

export function removeSection(doc: InfographicDoc, template: Template, id: string): InfographicDoc {
  if (doc.sections.length <= template.minSections || !doc.sections.some((s) => s.id === id)) return doc;
  return { ...doc, sections: doc.sections.filter((s) => s.id !== id) };
}

/** Moves a section from one position to another. Positions start at 0. */
export function reorder(doc: InfographicDoc, from: number, to: number): InfographicDoc {
  const n = doc.sections.length;
  if (from === to || from < 0 || to < 0 || from >= n || to >= n) return doc;
  const sections = [...doc.sections];
  const [moved] = sections.splice(from, 1);
  sections.splice(to, 0, moved);
  return { ...doc, sections };
}

/** Moves a section one place up (-1) or down (+1). */
export function moveSection(doc: InfographicDoc, id: string, delta: -1 | 1): InfographicDoc {
  const from = doc.sections.findIndex((s) => s.id === id);
  return from < 0 ? doc : reorder(doc, from, from + delta);
}

/** Puts a copy right after the section. The copy has its own id. */
export function duplicateSection(doc: InfographicDoc, template: Template, id: string): InfographicDoc {
  const at = doc.sections.findIndex((s) => s.id === id);
  if (at < 0 || doc.sections.length >= maxSections(template)) return doc;
  const copy: Section = { ...doc.sections[at], id: newSectionId() };
  const sections = [...doc.sections];
  sections.splice(at + 1, 0, copy);
  return { ...doc, sections };
}

/** Sets one field of one section. */
export function setSectionField(doc: InfographicDoc, id: string, field: 'label' | 'heading' | 'body', value: string): InfographicDoc {
  const at = doc.sections.findIndex((s) => s.id === id);
  if (at < 0 || doc.sections[at][field] === value) return doc;
  const sections = [...doc.sections];
  sections[at] = { ...sections[at], [field]: value };
  return { ...doc, sections };
}

/** Back to the default look. The template, the text and the sections stay as they are. */
export function resetDesign(doc: InfographicDoc): InfographicDoc {
  const same = doc.format === DEFAULT_FORMAT && doc.themeId === DEFAULT_THEME_ID && doc.language === 'auto' && doc.direction === 'auto' && !doc.mark;
  return same ? doc : { ...doc, format: DEFAULT_FORMAT, themeId: DEFAULT_THEME_ID, language: 'auto', direction: 'auto', mark: false };
}

/** A fresh document from the same template, with its own starting text and the default design. */
export function startNew(template: Template): InfographicDoc {
  return newDoc(template);
}
