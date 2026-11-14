import { Injectable, inject } from '@angular/core';
import { FORMATS, InfographicDoc } from './infographic.model';
import { InfographicFonts } from './infographic-fonts.service';
import { buildPdf } from './pdf-writer';
import { themeById } from './themes';
import { slugify } from './text-utils';

export type ExportKind = 'png' | 'jpg' | 'pdf';

export interface ExportedImage {
  blob: Blob;
  filename: string;
  width: number;
  height: number;
  /** Characters no shipped font has, which the image draws with the device's font. */
  uncovered: string[];
}

export const JPG_QUALITY = 0.92;
/** The picture inside a PDF is kept a little sharper than a JPG download, because pages are zoomed and printed. */
export const PDF_JPEG_QUALITY = 0.95;

/** `data:image/png;base64,...` to a Blob, without a network call. */
export function dataUrlToBlob(url: string): Blob {
  const comma = url.indexOf(',');
  const type = /^data:([^;,]+)/.exec(url)?.[1] ?? 'application/octet-stream';
  const binary = atob(url.slice(comma + 1));
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return new Blob([bytes], { type });
}

/** `title-1080x1920.png`, from the title and the real pixel size. */
export function exportFilename(doc: InfographicDoc, kind: ExportKind): string {
  const f = FORMATS[doc.format];
  return `${slugify(doc.title)}-${f.width}x${f.height}.${kind}`;
}

export function downloadBlob(blob: Blob, filename: string, document: Document): void {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.rel = 'noopener';
  document.body.appendChild(a);
  a.click();
  a.remove();
  // Give the browser a moment to start the download before the link is released.
  setTimeout(() => URL.revokeObjectURL(url), 10_000);
}

/** Turns the artboard into a PNG or JPG at its real size, in this browser, with the fonts it shows. */
@Injectable({ providedIn: 'root' })
export class InfographicExporter {
  private readonly fonts = inject(InfographicFonts);

  async export(stage: HTMLElement, doc: InfographicDoc, kind: ExportKind): Promise<ExportedImage> {
    const f = FORMATS[doc.format];
    await this.fonts.ensureLoaded(stage.textContent ?? '');
    const embed = await this.fonts.embed(stage);
    // Loaded here, not at the top, so the library is part of the tool's own chunk and loads with the first export.
    const { toJpeg, toPng } = await import('html-to-image');
    const options = {
      width: f.width,
      height: f.height,
      pixelRatio: 1,
      // Always a string: leaving it out would make the library embed every font file it can find.
      fontEmbedCSS: embed.css,
      // The preview is scaled to fit the screen; the image is made at full size.
      style: { transform: 'none' },
      // Editing aids that are drawn on the artboard (the ring round a selected icon) are not part of the picture.
      filter: (node: Node) => !(node instanceof HTMLElement && node.hasAttribute('data-skip-export')),
    };
    const background = themeById(doc.themeId).bg;
    let blob: Blob;
    if (kind === 'png') {
      blob = dataUrlToBlob(await toPng(stage, options));
    } else if (kind === 'jpg') {
      blob = dataUrlToBlob(await toJpeg(stage, { ...options, quality: JPG_QUALITY, backgroundColor: background }));
    } else {
      const jpeg = dataUrlToBlob(await toJpeg(stage, { ...options, quality: PDF_JPEG_QUALITY, backgroundColor: background }));
      blob = new Blob([buildPdf(new Uint8Array(await jpeg.arrayBuffer()), { title: doc.title })], { type: 'application/pdf' });
    }
    return {
      blob,
      filename: exportFilename(doc, kind),
      width: f.width,
      height: f.height,
      uncovered: embed.uncovered.map((cp) => String.fromCodePoint(cp)),
    };
  }
}
