/**
 * Font files for the infographic maker's export (#217).
 *
 * The page shows text in Noto fonts that are split into slices by Unicode
 * range (the @fontsource packages ship them that way). A visitor's browser
 * downloads only the slices the text uses. The exported image is drawn from a
 * copy of the page inside an SVG, and an SVG cannot fetch files, so the fonts
 * must travel inside it as data. html-to-image would embed every slice named in
 * the stylesheet: for Simplified Chinese that is 102 files, 3.4 MB for one
 * weight. This module picks the slices the text really needs, with the same
 * rule the browser uses, so the export has the same fonts as the preview and
 * stays small.
 *
 * Nothing here touches the network or the document, so it is easy to test.
 * `load` is passed in by the caller.
 */

export interface FontSlice {
  family: string;
  weight: number;
  style: 'normal' | 'italic';
  /** The woff2 file, as an absolute or page-relative URL. */
  url: string;
  /** The code points this slice covers, as inclusive [from, to] pairs. */
  ranges: ReadonlyArray<readonly [number, number]>;
}

const MAX_CODE_POINT = 0x10ffff;

/**
 * Reads a CSS `unicode-range` value: `U+0900-097F, U+20A8, U+4??`.
 * Anything it cannot read is skipped, never guessed.
 */
export function parseUnicodeRange(value: string): [number, number][] {
  const out: [number, number][] = [];
  for (const part of value.split(',')) {
    const m = /^\s*U\+([0-9a-f?]{1,6})(?:-([0-9a-f]{1,6}))?\s*$/i.exec(part);
    if (!m) continue;
    if (m[1].includes('?')) {
      // U+4?? is the range U+400-4FF.
      if (m[2] || /[0-9a-f]/i.test(m[1].slice(m[1].indexOf('?')))) continue;
      out.push([parseInt(m[1].replace(/\?/g, '0'), 16), parseInt(m[1].replace(/\?/g, 'f'), 16)]);
    } else {
      const from = parseInt(m[1], 16);
      const to = m[2] ? parseInt(m[2], 16) : from;
      if (from <= to && to <= MAX_CODE_POINT) out.push([from, to]);
    }
  }
  return out;
}

export function rangesToCss(ranges: ReadonlyArray<readonly [number, number]>): string {
  return ranges
    .map(([a, b]) => (a === b ? `U+${a.toString(16).toUpperCase()}` : `U+${a.toString(16).toUpperCase()}-${b.toString(16).toUpperCase()}`))
    .join(',');
}

function declaration(block: string, name: string): string {
  const m = new RegExp(`(?:^|[;\\s{])${name}\\s*:\\s*([^;]+)`, 'i').exec(block);
  return m ? m[1].trim() : '';
}

/**
 * Reads every @font-face rule of a stylesheet that has a woff2 file. `baseUrl`
 * is where the stylesheet lives, so a relative `url(./files/x.woff2)` resolves.
 */
export function parseFontSlices(css: string, baseUrl: string): FontSlice[] {
  const slices: FontSlice[] = [];
  const clean = css.replace(/\/\*[\s\S]*?\*\//g, '');
  for (const m of clean.matchAll(/@font-face\s*{([^}]*)}/gi)) {
    const block = m[1];
    const family = declaration(block, 'font-family').replace(/^['"]|['"]$/g, '');
    const src = /url\(\s*['"]?([^'")]+?\.woff2)['"]?\s*\)/i.exec(declaration(block, 'src'));
    if (!family || !src) continue;
    const w = declaration(block, 'font-weight').toLowerCase();
    const weight = w === 'bold' ? 700 : w === 'normal' || w === '' ? 400 : parseInt(w, 10) || 400;
    const range = declaration(block, 'unicode-range');
    slices.push({
      family,
      weight,
      style: /italic|oblique/i.test(declaration(block, 'font-style')) ? 'italic' : 'normal',
      url: new URL(src[1], baseUrl).href,
      ranges: range ? parseUnicodeRange(range) : [[0, MAX_CODE_POINT]],
    });
  }
  return slices;
}

export function covers(slice: FontSlice, codePoint: number): boolean {
  return slice.ranges.some(([a, b]) => codePoint >= a && codePoint <= b);
}

/**
 * The distinct code points of `text` that need a font file. Line breaks and
 * other control characters are left out; the zero-width joiners (U+200C and
 * U+200D) stay, because Indic shaping needs them.
 */
export function codePointsOf(text: string): number[] {
  const seen = new Set<number>();
  for (const ch of text) {
    const cp = ch.codePointAt(0)!;
    if (cp < 0x20 || (cp >= 0x7f && cp <= 0x9f)) continue;
    seen.add(cp);
  }
  return [...seen].sort((a, b) => a - b);
}

/** The weight a face is used for when the text asks for `wanted`, as CSS picks it. */
export function nearestWeight(wanted: number, available: number[]): number | null {
  if (available.length === 0) return null;
  const sorted = [...available].sort((a, b) => a - b);
  if (sorted.includes(wanted)) return wanted;
  // Under 400 looks lighter first, then heavier; over 500 looks heavier first.
  const lighter = sorted.filter((w) => w < wanted).reverse();
  const heavier = sorted.filter((w) => w > wanted);
  const order = wanted > 500 ? [...heavier, ...lighter] : [...lighter, ...heavier];
  return order[0];
}

export interface Chosen {
  /** The slices to embed, each once. */
  slices: FontSlice[];
  /** Code points no family in the stack has a slice for. They will fall back to the device's font. */
  uncovered: number[];
}

/**
 * For each code point, takes the first family of `stack` that has a slice
 * covering it at this weight. That is how a browser walks a font stack, so the
 * export picks the file the preview showed.
 */
export function chooseSlices(text: string, stack: string[], weight: number, all: FontSlice[]): Chosen {
  const chosen = new Map<string, FontSlice>();
  const uncovered: number[] = [];
  const byFamily = new Map<string, FontSlice[]>();
  for (const family of stack) {
    const faces = all.filter((s) => s.family === family && s.style === 'normal');
    const w = nearestWeight(weight, [...new Set(faces.map((s) => s.weight))]);
    byFamily.set(family, faces.filter((s) => s.weight === w));
  }
  for (const cp of codePointsOf(text)) {
    let found: FontSlice | undefined;
    for (const family of stack) {
      found = byFamily.get(family)?.find((s) => covers(s, cp));
      if (found) break;
    }
    if (found) chosen.set(found.url, found);
    else uncovered.push(cp);
  }
  return { slices: [...chosen.values()], uncovered };
}

/** Binary to base64, in pieces so a large font does not overflow the call stack. */
export function toBase64(bytes: ArrayBuffer): string {
  const view = new Uint8Array(bytes);
  let binary = '';
  for (let i = 0; i < view.length; i += 0x8000) {
    binary += String.fromCharCode(...view.subarray(i, i + 0x8000));
  }
  return btoa(binary);
}

function quote(family: string): string {
  return `'${family.replace(/['\\]/g, '\\$&')}'`;
}

export interface EmbedResult {
  /** CSS to give html-to-image as `fontEmbedCSS`. */
  css: string;
  slices: FontSlice[];
  uncovered: number[];
  /** The woff2 bytes embedded, before base64. */
  bytes: number;
}

/**
 * Builds the @font-face rules for an export. `texts` maps each weight in use to
 * the text shown at that weight. `load` fetches one woff2 file (and may cache).
 */
export async function buildFontEmbedCss(opts: {
  texts: ReadonlyMap<number, string>;
  stack: string[];
  slices: FontSlice[];
  load: (url: string) => Promise<ArrayBuffer>;
}): Promise<EmbedResult> {
  const rules: string[] = [];
  const used: FontSlice[] = [];
  const uncovered = new Set<number>();
  const done = new Set<string>();
  let bytes = 0;
  for (const [weight, text] of opts.texts) {
    const picked = chooseSlices(text, opts.stack, weight, opts.slices);
    picked.uncovered.forEach((cp) => uncovered.add(cp));
    for (const slice of picked.slices) {
      if (done.has(slice.url)) continue;
      done.add(slice.url);
      const data = await opts.load(slice.url);
      bytes += data.byteLength;
      used.push(slice);
      rules.push(
        `@font-face{font-family:${quote(slice.family)};font-style:${slice.style};font-weight:${slice.weight};` +
          `src:url(data:font/woff2;base64,${toBase64(data)}) format('woff2');unicode-range:${rangesToCss(slice.ranges)};}`,
      );
    }
  }
  return { css: rules.join('\n'), slices: used, uncovered: [...uncovered].sort((a, b) => a - b), bytes };
}

/**
 * The text under `root` grouped by the weight it is drawn at, so a bold title
 * does not make the export carry the bold file for every other character.
 */
export function textByWeight(root: Element): Map<number, string> {
  const out = new Map<number, string>();
  const walker = root.ownerDocument.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    const text = node.textContent ?? '';
    const parent = node.parentElement;
    if (!parent || text.trim() === '') continue;
    const weight = parseInt(getComputedStyle(parent).fontWeight, 10) || 400;
    out.set(weight, (out.get(weight) ?? '') + text);
  }
  return out;
}
