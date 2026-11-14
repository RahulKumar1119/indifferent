import { InfographicDoc, Section } from './infographic.model';
import { Template } from './templates';
import { Theme } from './themes';
import { contrastRatio } from './text-utils';

/**
 * Numbers for the statistics templates (#217). Each section is one data point:
 * its caption is the name, its heading is the number as the person typed it
 * ("42%", "₹1,200", "३.५"), and the chart reads the number out of that text.
 * Pure functions, no document, so they are tested without a browser.
 */

/** The zero of each decimal digit set the maker's languages use. */
const DIGIT_ZEROS = [0x30, 0x660, 0x6f0, 0x966, 0x9e6, 0xa66, 0xae6, 0xb66, 0xbe6, 0xc66, 0xce6, 0xd66, 0xe50];

/** Hindi, Gujarati, Arabic-Indic and other digits to 0-9. */
export function toAsciiDigits(text: string): string {
  let out = '';
  for (const ch of text) {
    const cp = ch.codePointAt(0)!;
    const zero = DIGIT_ZEROS.find((z) => cp >= z && cp <= z + 9);
    out += zero === undefined ? ch : String(cp - zero);
  }
  return out;
}

/**
 * The one number in a text, or null when there is none or more than one
 * ("10-20", "Q3 2024"). Units and words around it are ignored ("42%", "₹1,200",
 * "3.5M"). Commas are thousands marks ("1,200", "1,20,000") or a decimal comma
 * ("3,5"); anything else is not read.
 */
export function parseNumber(text: string): number | null {
  const t = toAsciiDigits(text).replace(/\u2212/g, '-');
  const tokens = t.match(/-?(?:\d[\d,]*(?:\.\d+)?|\.\d+)/g);
  if (!tokens || tokens.length !== 1) return null;
  let tok = tokens[0].replace(/,+$/, '');
  const negative = tok.startsWith('-');
  if (negative) tok = tok.slice(1);
  if (tok.includes(',')) {
    if (/^(\d{1,3}(,\d{3})+|\d{1,2}(,\d{2})*,\d{3})(\.\d+)?$/.test(tok)) tok = tok.replace(/,/g, '');
    else if (/^\d+,\d{1,2}$/.test(tok)) tok = tok.replace(',', '.');
    else return null;
  }
  const n = Number(tok);
  if (!Number.isFinite(n) || n > 1e12) return null;
  return negative ? -n : n;
}

export interface DataItem {
  id: string;
  /** The name of the point. */
  label: string;
  /** The number as typed, shown on the chart. */
  text: string;
  /** The number the chart draws. 0 when the text holds no usable number. */
  value: number;
  /** False when the text holds no number, more than one, or a negative one. */
  valid: boolean;
  note: string;
}

export function readItems(sections: Section[]): DataItem[] {
  return sections.map((s) => {
    const n = parseNumber(s.heading);
    const valid = n !== null && n >= 0;
    return { id: s.id, label: s.label, text: s.heading, value: valid ? n : 0, valid, note: s.body };
  });
}

/** Sentences for the person, one per problem, naming the section. Empty when the data can be drawn. */
export function dataIssues(doc: InfographicDoc, template: Template): string[] {
  if (!template.chart) return [];
  const issues: string[] = [];
  const items = readItems(doc.sections);
  items.forEach((it, i) => {
    const where = `Section ${i + 1}${it.label ? ` (${it.label})` : ''}`;
    const n = parseNumber(it.text);
    if (n === null) issues.push(`${where}: type one number in the value box, like 12, 3.5 or 40%.`);
    else if (n < 0) issues.push(`${where}: negative numbers are not supported yet. It is drawn as 0.`);
  });
  if ((template.layout === 'donut' || template.layout === 'bars') && items.every((it) => it.value === 0)) {
    issues.push('Every number is 0, so there is nothing to draw.');
  }
  return issues;
}

/** The share of each value in the total, in percent, or all 0 when the total is 0. */
export function shares(values: number[]): number[] {
  const total = values.reduce((a, b) => a + b, 0);
  return values.map((v) => (total > 0 ? (v / total) * 100 : 0));
}

/** `33.3`, `50`, `0.4`: one decimal at most, and no `.0`. */
export function formatPercent(p: number): string {
  const r = Math.round(p * 10) / 10;
  return `${Number.isInteger(r) ? r : r.toFixed(1)}%`;
}

/** The top of a value axis: 1, 2, 2.5, 5 or 10 times a power of ten that is not below `max`. 1 for no data. */
export function niceMax(max: number): number {
  if (!(max > 0)) return 1;
  const pow = Math.pow(10, Math.floor(Math.log10(max)));
  for (const step of [1, 2, 2.5, 5, 10]) if (step * pow >= max) return step * pow;
  return 10 * pow;
}

// --- colours

function channel(hex: string, shift: number): number {
  return (parseInt(hex.slice(1), 16) >> shift) & 255;
}

/** `t` of the way from colour `a` to colour `b`, as `#rrggbb`. */
export function mix(a: string, b: string, t: number): string {
  const part = (shift: number) => Math.round(channel(a, shift) * (1 - t) + channel(b, shift) * t);
  return `#${((1 << 24) | (part(16) << 16) | (part(8) << 8) | part(0)).toString(16).slice(1)}`;
}

function distance(a: string, b: string): number {
  return [16, 8, 0].reduce((sum, shift) => sum + Math.abs(channel(a, shift) - channel(b, shift)), 0);
}

/**
 * Up to six colours for the slices of a donut. Each shows against the page and
 * the card (3:1, as a shape needs) and differs from the others. The theme's own
 * colours come first; a fine ladder from the accent towards the page colour
 * fills in, which is what a one-colour theme such as Mono relies on.
 */
export function sliceColors(theme: Theme, n: number): string[] {
  const preferred = [theme.accent, theme.ink, mix(theme.accent, theme.ink, 0.5), mix(theme.accent, theme.ink, 0.22), mix(theme.accent, theme.ink, 0.78), theme.ink2];
  const ladder = Array.from({ length: 15 }, (_, i) => mix(theme.accent, theme.bg, i * 0.05));
  const picked: string[] = [];
  for (const c of [...preferred, ...ladder]) {
    if (picked.length >= Math.min(n, 6)) break;
    const shows = contrastRatio(c, theme.bg) >= 3 && contrastRatio(c, theme.surface) >= 3;
    if (shows && picked.every((p) => distance(p, c) >= 44)) picked.push(c);
  }
  return picked;
}
