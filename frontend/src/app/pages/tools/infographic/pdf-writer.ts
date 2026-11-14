/**
 * A small PDF writer for the infographic maker (#217): one page that holds one
 * JPEG picture. That is all the maker needs, so there is no library. The page
 * is the picture's size in CSS pixels (1 px = 0.75 pt, so 96 px to the inch) and
 * the picture fills it. The JPEG bytes go in unchanged (DCTDecode), so the PDF
 * is as sharp as the JPG and about the same size.
 *
 * The text in the picture is not selectable. That is stated on the page.
 */

export interface JpegInfo {
  width: number;
  height: number;
  /** 1 for grey, 3 for colour. */
  components: 1 | 3;
  progressive: boolean;
}

/** Reads the size and colour layout from a JPEG's header, or throws when the bytes are not a JPEG this writer can place. */
export function readJpegInfo(bytes: Uint8Array): JpegInfo {
  if (bytes.length < 4 || bytes[0] !== 0xff || bytes[1] !== 0xd8) throw new Error('Not a JPEG: it does not start with the JPEG marker.');
  let i = 2;
  while (i + 4 <= bytes.length) {
    if (bytes[i] !== 0xff) throw new Error('Not a JPEG: a marker is missing where one is expected.');
    // Fill bytes between markers.
    while (bytes[i + 1] === 0xff) i++;
    const marker = bytes[i + 1];
    // Markers that stand alone, without a length.
    if (marker === 0xd8 || marker === 0x01 || (marker >= 0xd0 && marker <= 0xd7)) {
      i += 2;
      continue;
    }
    if (marker === 0xd9 || marker === 0xda) break; // the end, or the picture data: no size was found before it
    const length = (bytes[i + 2] << 8) | bytes[i + 3];
    if (length < 2 || i + 2 + length > bytes.length) throw new Error('Not a JPEG: a segment runs past the end.');
    // Start of frame: baseline (C0), extended (C1) or progressive (C2). Other frame types are not supported in a PDF image this way.
    if (marker === 0xc0 || marker === 0xc1 || marker === 0xc2) {
      if (length < 8) throw new Error('Not a JPEG: the frame header is too short.');
      const bits = bytes[i + 4];
      const height = (bytes[i + 5] << 8) | bytes[i + 6];
      const width = (bytes[i + 7] << 8) | bytes[i + 8];
      const components = bytes[i + 9];
      if (bits !== 8) throw new Error(`Only 8-bit JPEGs are supported, not ${bits}-bit.`);
      if (width < 1 || height < 1) throw new Error('The JPEG has no size.');
      if (components !== 1 && components !== 3) throw new Error(`Only grey and colour JPEGs are supported, not ${components} components.`);
      return { width, height, components, progressive: marker === 0xc2 };
    }
    if (marker >= 0xc3 && marker <= 0xcf && marker !== 0xc4 && marker !== 0xc8 && marker !== 0xcc) {
      throw new Error('This kind of JPEG (lossless or arithmetic coded) is not supported.');
    }
    i += 2 + length;
  }
  throw new Error('Not a JPEG: no frame header was found.');
}

const encoder = new TextEncoder();

/** A PDF text string: plain when it is printable ASCII, otherwise UTF-16 in hex with the byte order mark, so Hindi and Gujarati titles survive. */
export function pdfTextString(text: string): string {
  // Control characters are dropped; the rest of a title is kept.
  const clean = text.replace(/[\u0000-\u001f\u007f-\u009f]/g, ' ').trim();
  if (/^[\x20-\x7e]*$/.test(clean)) return `(${clean.replace(/[\\()]/g, (c) => '\\' + c)})`;
  let hex = 'FEFF';
  for (let k = 0; k < clean.length; k++) hex += clean.charCodeAt(k).toString(16).padStart(4, '0').toUpperCase();
  return `<${hex}>`;
}

/** The size of a CSS pixel in PDF points (a point is 1/72 inch, a CSS pixel 1/96 inch). */
export const PX_TO_PT = 0.75;

function fixed(n: number): string {
  return String(Math.round(n * 1000) / 1000);
}

export interface PdfOptions {
  /** Shown as the document title in a PDF viewer. */
  title?: string;
}

/**
 * Builds the PDF bytes: a catalogue, a page tree, one page, the picture, the
 * page's drawing instructions and the document information, then the
 * cross-reference table that tells a reader where each object starts.
 */
export function buildPdf(jpeg: Uint8Array, options: PdfOptions = {}): Uint8Array<ArrayBuffer> {
  const info = readJpegInfo(jpeg);
  const pageW = info.width * PX_TO_PT;
  const pageH = info.height * PX_TO_PT;
  const colorSpace = info.components === 1 ? '/DeviceGray' : '/DeviceRGB';
  const content = `q ${fixed(pageW)} 0 0 ${fixed(pageH)} 0 0 cm /Im0 Do Q\n`;

  const parts: Uint8Array[] = [];
  const offsets: number[] = [];
  let length = 0;
  const push = (data: Uint8Array | string) => {
    const bytes = typeof data === 'string' ? encoder.encode(data) : data;
    parts.push(bytes);
    length += bytes.length;
  };
  const object = (n: number, body: Uint8Array | string, dict?: string) => {
    offsets[n] = length;
    if (dict !== undefined) {
      push(`${n} 0 obj\n<< ${dict} >>\nstream\n`);
      push(body);
      push('\nendstream\nendobj\n');
    } else {
      push(`${n} 0 obj\n${body as string}\nendobj\n`);
    }
  };

  // A binary comment right after the header tells transfer tools that the file holds binary data.
  push('%PDF-1.4\n');
  push(new Uint8Array([0x25, 0xe2, 0xe3, 0xcf, 0xd3, 0x0a]));
  object(1, '<< /Type /Catalog /Pages 2 0 R >>');
  object(2, '<< /Type /Pages /Kids [3 0 R] /Count 1 >>');
  object(3, `<< /Type /Page /Parent 2 0 R /MediaBox [0 0 ${fixed(pageW)} ${fixed(pageH)}] /Resources << /XObject << /Im0 4 0 R >> /ProcSet [/PDF /ImageC] >> /Contents 5 0 R >>`);
  object(
    4,
    jpeg,
    `/Type /XObject /Subtype /Image /Width ${info.width} /Height ${info.height} /ColorSpace ${colorSpace} /BitsPerComponent 8 /Filter /DCTDecode /Length ${jpeg.length}`,
  );
  const contentBytes = encoder.encode(content);
  object(5, contentBytes, `/Length ${contentBytes.length}`);
  object(6, `<< /Title ${pdfTextString(options.title ?? '')} /Producer (indifferent.fun) >>`);

  const xrefAt = length;
  let xref = `xref\n0 7\n0000000000 65535 f \n`;
  for (let n = 1; n <= 6; n++) xref += `${String(offsets[n]).padStart(10, '0')} 00000 n \n`;
  push(xref);
  push(`trailer\n<< /Size 7 /Root 1 0 R /Info 6 0 R >>\nstartxref\n${xrefAt}\n%%EOF\n`);

  const out = new Uint8Array(new ArrayBuffer(length));
  let at = 0;
  for (const p of parts) {
    out.set(p, at);
    at += p.length;
  }
  return out;
}
