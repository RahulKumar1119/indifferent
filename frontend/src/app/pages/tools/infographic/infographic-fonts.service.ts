import { DOCUMENT } from '@angular/common';
import { Injectable, inject } from '@angular/core';
import { EmbedResult, FontSlice, buildFontEmbedCss, parseFontSlices, textByWeight } from './font-slices';

/**
 * The fonts of the infographic maker (#217): Noto Sans (Latin, Cyrillic, Greek,
 * Vietnamese and Devanagari) and Noto Sans Gujarati, self-hosted under
 * /fonts/infographic, copied from the pinned @fontsource packages when the app
 * is built (angular.json). The stylesheets are added to the page only when the
 * tool opens, and the browser fetches only the slices the text uses.
 */
export const FONT_STACK = ['Noto Sans', 'Noto Sans Gujarati'];
const WEIGHTS = [400, 700];
export const FONT_SHEETS = [
  '/fonts/infographic/noto-sans/400.css',
  '/fonts/infographic/noto-sans/700.css',
  '/fonts/infographic/noto-sans-gujarati/400.css',
  '/fonts/infographic/noto-sans-gujarati/700.css',
];

@Injectable({ providedIn: 'root' })
export class InfographicFonts {
  private readonly document = inject(DOCUMENT);
  private slices: Promise<FontSlice[]> | null = null;
  private readonly files = new Map<string, Promise<ArrayBuffer>>();

  /**
   * Puts the stylesheets in the page and reads them, once. When a stylesheet
   * cannot be fetched (offline) its fonts are simply missing and the device's
   * fonts are used, in the page and in the export.
   */
  load(): Promise<FontSlice[]> {
    if (!this.slices) {
      this.slices = (async () => {
        const out: FontSlice[] = [];
        for (const path of FONT_SHEETS) {
          const href = new URL(path, this.document.baseURI).href;
          if (!this.document.head.querySelector(`link[data-infographic-font][href="${href}"]`)) {
            const link = this.document.createElement('link');
            link.rel = 'stylesheet';
            link.href = href;
            link.setAttribute('data-infographic-font', '');
            this.document.head.appendChild(link);
          }
          try {
            const res = await fetch(href);
            if (res.ok) out.push(...parseFontSlices(await res.text(), href));
          } catch {
            /* offline: carry on with what loaded */
          }
        }
        return out;
      })();
    }
    return this.slices;
  }

  /** Waits until the fonts for `text` are loaded, so the page and the export measure and draw the same glyphs. */
  async ensureLoaded(text: string): Promise<void> {
    await this.load();
    const set = this.document.fonts;
    if (!set) return;
    await Promise.all(
      FONT_STACK.flatMap((family) => WEIGHTS.map((w) => set.load(`${w} 40px "${family}"`, text).catch(() => []))),
    );
    await set.ready;
  }

  private bytes(url: string): Promise<ArrayBuffer> {
    let p = this.files.get(url);
    if (!p) {
      p = fetch(url).then((r) => {
        if (!r.ok) throw new Error(`font file ${url} answered ${r.status}`);
        return r.arrayBuffer();
      });
      p.catch(() => this.files.delete(url));
      this.files.set(url, p);
    }
    return p;
  }

  /** The @font-face rules to put inside an export of `stage`: only the files its text needs. */
  async embed(stage: HTMLElement): Promise<EmbedResult> {
    const slices = await this.load();
    return buildFontEmbedCss({ texts: textByWeight(stage), stack: FONT_STACK, slices, load: (u) => this.bytes(u) });
  }
}
