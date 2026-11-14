/**
 * Text helpers for the infographic maker (#217): cleaning what people type,
 * telling which writing system and direction it is, and naming files.
 * Pure functions, no document or network.
 */

/** Alphabetic presentation forms that pasted PDF text often holds. The fonts have no glyph for them. */
const LIGATURES: Record<string, string> = {
  '\ufb00': 'ff',
  '\ufb01': 'fi',
  '\ufb02': 'fl',
  '\ufb03': 'ffi',
  '\ufb04': 'ffl',
  '\ufb05': 'st',
  '\ufb06': 'st',
};

/**
 * Cleans one piece of text: composed form (so Hindi, Gujarati and Vietnamese
 * letters are stored one way), ligature characters turned into letters, line
 * endings made `\n`, and control characters removed. `multiline` keeps line
 * breaks; otherwise they become spaces.
 */
export function normalizeText(text: string, multiline = false): string {
  let out = text.normalize('NFC').replace(/[\ufb00-\ufb06]/g, (c) => LIGATURES[c]);
  out = out.replace(/\r\n?/g, '\n').replace(/\t/g, ' ');
  // Control characters, except the new line when it is allowed. The zero-width joiners stay: Indic shaping needs them.
  out = out.replace(/[\u0000-\u0009\u000b-\u001f\u007f-\u009f\u2028\u2029]/g, '');
  return multiline ? out : out.replace(/\n+/g, ' ');
}

export type Script =
  | 'latin' | 'cyrillic' | 'greek'
  | 'devanagari' | 'bengali' | 'gurmukhi' | 'gujarati' | 'odia' | 'tamil' | 'telugu' | 'kannada' | 'malayalam'
  | 'thai' | 'arabic' | 'hebrew' | 'han' | 'kana' | 'hangul' | 'other';

const RANGES: [number, number, Script][] = [
  [0x41, 0x5a, 'latin'], [0x61, 0x7a, 'latin'], [0xaa, 0xaa, 'latin'], [0xba, 0xba, 'latin'], [0xc0, 0x24f, 'latin'],
  [0x1e00, 0x1eff, 'latin'], [0x2c60, 0x2c7f, 'latin'], [0xa720, 0xa7ff, 'latin'], [0x1d00, 0x1dbf, 'latin'],
  [0x370, 0x3ff, 'greek'], [0x1f00, 0x1fff, 'greek'],
  [0x400, 0x52f, 'cyrillic'], [0x1c80, 0x1c8f, 'cyrillic'], [0x2de0, 0x2dff, 'cyrillic'], [0xa640, 0xa69f, 'cyrillic'],
  [0x590, 0x5ff, 'hebrew'],
  [0x600, 0x6ff, 'arabic'], [0x750, 0x77f, 'arabic'], [0x8a0, 0x8ff, 'arabic'], [0xfb50, 0xfdff, 'arabic'], [0xfe70, 0xfeff, 'arabic'],
  [0x900, 0x97f, 'devanagari'], [0xa8e0, 0xa8ff, 'devanagari'],
  [0x980, 0x9ff, 'bengali'], [0xa00, 0xa7f, 'gurmukhi'], [0xa80, 0xaff, 'gujarati'], [0xb00, 0xb7f, 'odia'],
  [0xb80, 0xbff, 'tamil'], [0xc00, 0xc7f, 'telugu'], [0xc80, 0xcff, 'kannada'], [0xd00, 0xd7f, 'malayalam'],
  [0xe00, 0xe7f, 'thai'],
  [0x1100, 0x11ff, 'hangul'], [0x3130, 0x318f, 'hangul'], [0xac00, 0xd7af, 'hangul'],
  [0x3040, 0x30ff, 'kana'], [0x31f0, 0x31ff, 'kana'],
  [0x3400, 0x4dbf, 'han'], [0x4e00, 0x9fff, 'han'], [0xf900, 0xfaff, 'han'], [0x20000, 0x2ffff, 'han'],
];

/** Digits, spaces, punctuation and symbols belong to no one script. */
function isCommon(cp: number): boolean {
  return (
    cp <= 0x40 || (cp >= 0x5b && cp <= 0x60) || (cp >= 0x7b && cp <= 0xa9) || (cp >= 0xab && cp <= 0xb9) || (cp >= 0xbb && cp <= 0xbf) ||
    cp === 0xd7 || cp === 0xf7 || (cp >= 0x2000 && cp <= 0x2bff) || (cp >= 0x3000 && cp <= 0x303f) || (cp >= 0xff00 && cp <= 0xffef) ||
    cp === 0x200c || cp === 0x200d || (cp >= 0x1f000 && cp <= 0x1faff) || (cp >= 0xfe00 && cp <= 0xfe0f)
  );
}

export function scriptOf(cp: number): Script | 'common' {
  if (isCommon(cp)) return 'common';
  for (const [from, to, script] of RANGES) if (cp >= from && cp <= to) return script;
  return 'other';
}

/** The writing systems used in `text`, ignoring digits and punctuation. */
export function scriptsIn(text: string): Set<Script> {
  const found = new Set<Script>();
  for (const ch of text) {
    const s = scriptOf(ch.codePointAt(0)!);
    if (s !== 'common') found.add(s);
  }
  return found;
}

/** The language code a text most likely is, from its writing system. English when it cannot tell. */
export function detectLanguage(text: string): string {
  const scripts = scriptsIn(text);
  const order: [Script, string][] = [
    ['kana', 'ja'], ['hangul', 'ko'], ['han', 'zh-Hans'], ['devanagari', 'hi'], ['gujarati', 'gu'], ['bengali', 'bn'],
    ['gurmukhi', 'pa'], ['odia', 'or'], ['tamil', 'ta'], ['telugu', 'te'], ['kannada', 'kn'], ['malayalam', 'ml'],
    ['thai', 'th'], ['arabic', 'ar'], ['hebrew', 'he'], ['cyrillic', 'ru'], ['greek', 'el'],
  ];
  for (const [script, lang] of order) if (scripts.has(script)) return lang;
  return 'en';
}

/** The direction of the first letter that has one, as a browser's `dir="auto"` decides it. */
export function detectDirection(text: string): 'ltr' | 'rtl' {
  for (const ch of text) {
    const s = scriptOf(ch.codePointAt(0)!);
    if (s === 'arabic' || s === 'hebrew') return 'rtl';
    if (s !== 'common') return 'ltr';
  }
  return 'ltr';
}

/** Script names for people, for the notice about fonts that are not released yet. */
export const SCRIPT_NAMES: Record<Script, string> = {
  latin: 'Latin', cyrillic: 'Cyrillic', greek: 'Greek', devanagari: 'Devanagari', bengali: 'Bengali', gurmukhi: 'Gurmukhi (Punjabi)',
  gujarati: 'Gujarati', odia: 'Odia', tamil: 'Tamil', telugu: 'Telugu', kannada: 'Kannada', malayalam: 'Malayalam', thai: 'Thai',
  arabic: 'Arabic', hebrew: 'Hebrew', han: 'Chinese characters', kana: 'Japanese kana', hangul: 'Korean', other: 'other',
};

/** A file name made from the title, keeping letters, marks and digits of any language. */
export function slugify(title: string): string {
  const slug = title
    .normalize('NFKC')
    .toLowerCase()
    .replace(/[^\p{L}\p{M}\p{N}]+/gu, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 60)
    .replace(/-+$/g, '');
  return slug || 'infographic';
}

/** `#rrggbb` to relative luminance, as WCAG defines it. */
function luminance(hex: string): number {
  const n = parseInt(hex.slice(1), 16);
  const lin = (v: number) => {
    const c = v / 255;
    return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
  };
  return 0.2126 * lin((n >> 16) & 255) + 0.7152 * lin((n >> 8) & 255) + 0.0722 * lin(n & 255);
}

/** WCAG contrast ratio of two `#rrggbb` colours, 1 to 21. */
export function contrastRatio(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}
