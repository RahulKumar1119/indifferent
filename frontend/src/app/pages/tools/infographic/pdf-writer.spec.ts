import { PX_TO_PT, buildPdf, pdfTextString, readJpegInfo } from './pdf-writer';

/** A JPEG made by the browser, so the test uses what the maker really gets. */
async function realJpeg(width: number, height: number): Promise<Uint8Array> {
  const canvas = document.createElement('canvas');
  canvas.width = width;
  canvas.height = height;
  const g = canvas.getContext('2d')!;
  g.fillStyle = '#bc5227';
  g.fillRect(0, 0, width, height);
  g.fillStyle = '#ffffff';
  g.fillRect(width / 4, height / 4, width / 2, height / 2);
  const blob = await new Promise<Blob>((resolve) => canvas.toBlob((b) => resolve(b!), 'image/jpeg', 0.9));
  return new Uint8Array(await blob.arrayBuffer());
}

/** A JPEG header only, with the frame type, bit depth, size and component count chosen by the test. */
function header(marker: number, opts: { bits?: number; width?: number; height?: number; components?: number } = {}): Uint8Array {
  const { bits = 8, width = 40, height = 30, components = 3 } = opts;
  return new Uint8Array([
    0xff, 0xd8,
    0xff, 0xe0, 0x00, 0x04, 0x00, 0x00, // an APP0 segment first, to be skipped
    0xff, marker, 0x00, 8 + components * 3, bits, height >> 8, height & 255, width >> 8, width & 255, components,
    ...new Array(components * 3).fill(0),
    0xff, 0xda,
  ]);
}

const text = (bytes: Uint8Array) => Array.from(bytes, (b) => String.fromCharCode(b)).join('');

describe('pdf-writer', () => {
  describe('readJpegInfo', () => {
    it('reads the size of a JPEG made by the browser', async () => {
      const info = readJpegInfo(await realJpeg(120, 80));
      expect(info.width).toBe(120);
      expect(info.height).toBe(80);
      expect(info.components).toBe(3);
      expect(info.progressive).toBeFalse();
    });

    it('reads sizes above 255 in both bytes', () => {
      const info = readJpegInfo(header(0xc0, { width: 1920, height: 1080 }));
      expect([info.width, info.height]).toEqual([1920, 1080]);
    });

    it('accepts grey, and progressive and extended frames', () => {
      expect(readJpegInfo(header(0xc0, { components: 1 })).components).toBe(1);
      expect(readJpegInfo(header(0xc2)).progressive).toBeTrue();
      expect(readJpegInfo(header(0xc1)).progressive).toBeFalse();
    });

    it('refuses what it cannot place', () => {
      expect(() => readJpegInfo(new Uint8Array([1, 2, 3, 4, 5]))).toThrowError(/Not a JPEG/);
      expect(() => readJpegInfo(new Uint8Array([]))).toThrowError(/Not a JPEG/);
      expect(() => readJpegInfo(new TextEncoder().encode('<svg></svg>'))).toThrowError(/Not a JPEG/);
      expect(() => readJpegInfo(header(0xc0, { bits: 12 }))).toThrowError(/8-bit/);
      expect(() => readJpegInfo(header(0xc0, { components: 4 }))).toThrowError(/grey and colour/);
      expect(() => readJpegInfo(header(0xc3))).toThrowError(/not supported/);
      expect(() => readJpegInfo(header(0xc0, { width: 0 }))).toThrowError(/no size/);
    });

    it('refuses a JPEG that ends before its frame header', () => {
      expect(() => readJpegInfo(new Uint8Array([0xff, 0xd8, 0xff, 0xd9]))).toThrowError(/no frame header/);
      expect(() => readJpegInfo(new Uint8Array([0xff, 0xd8, 0xff, 0xe0, 0x00, 0xff, 0x00]))).toThrowError(/runs past the end/);
    });
  });

  describe('pdfTextString', () => {
    it('writes plain text as it is, with the brackets and the backslash escaped', () => {
      expect(pdfTextString('Habits')).toBe('(Habits)');
      expect(pdfTextString('a (b) \\ c')).toBe('(a \\(b\\) \\\\ c)');
      expect(pdfTextString('')).toBe('()');
    });

    it('writes other scripts as UTF-16 in hex with the byte order mark', () => {
      expect(pdfTextString('कहानी')).toBe('<FEFF0915 0939 093E 0928 0940>'.replace(/ /g, ''));
      expect(pdfTextString('વાર્તા')).toMatch(/^<FEFF(0A[0-9A-F]{2}){5,}>$/);
    });

    it('writes characters beyond the first plane as a surrogate pair', () => {
      expect(pdfTextString('😀')).toBe('<FEFFD83DDE00>');
    });

    it('drops control characters', () => {
      expect(pdfTextString('a\u0000b\nc')).toBe('(a b c)');
    });
  });

  describe('buildPdf', () => {
    let jpeg: Uint8Array;
    let pdf: Uint8Array;
    let raw: string;

    beforeAll(async () => {
      jpeg = await realJpeg(108, 192);
      pdf = buildPdf(jpeg, { title: 'My title (draft)' });
      raw = text(pdf);
    });

    it('starts and ends the way a PDF does', () => {
      expect(raw.startsWith('%PDF-1.4\n')).toBeTrue();
      expect(raw.endsWith('%%EOF\n')).toBeTrue();
      // The comment with high bytes that tells tools the file is binary.
      expect(pdf[10]).toBeGreaterThan(127);
    });

    it('has one page, as large as the picture is in CSS pixels', () => {
      expect(raw).toContain('/Type /Pages /Kids [3 0 R] /Count 1');
      expect(raw).toContain(`/MediaBox [0 0 ${108 * PX_TO_PT} ${192 * PX_TO_PT}]`);
      expect(PX_TO_PT).toBe(0.75);
    });

    it('places the JPEG bytes in the file unchanged', () => {
      const at = raw.indexOf('stream\n', raw.indexOf('/Subtype /Image')) + 'stream\n'.length;
      expect(Array.from(pdf.subarray(at, at + jpeg.length))).toEqual(Array.from(jpeg));
      expect(raw).toContain(`/Width 108 /Height 192 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length ${jpeg.length}`);
    });

    it('draws the picture over the whole page', () => {
      expect(raw).toContain(`q ${108 * PX_TO_PT} 0 0 ${192 * PX_TO_PT} 0 0 cm /Im0 Do Q`);
    });

    it('records the title and the producer', () => {
      expect(raw).toContain('/Title (My title \\(draft\\))');
      expect(raw).toContain('/Producer (indifferent.fun)');
    });

    it('says where every object starts, and the table itself', () => {
      const startxref = Number(/startxref\n(\d+)\n%%EOF/.exec(raw)![1]);
      expect(raw.slice(startxref, startxref + 4)).toBe('xref');
      const rows = raw.slice(startxref).split('\n').slice(2, 9);
      expect(rows.length).toBe(7);
      expect(rows[0]).toBe('0000000000 65535 f ');
      for (let n = 1; n <= 6; n++) {
        const offset = Number(rows[n].slice(0, 10));
        expect(raw.slice(offset, offset + `${n} 0 obj`.length)).withContext(`object ${n}`).toBe(`${n} 0 obj`);
        expect(rows[n].endsWith(' 00000 n ')).toBeTrue();
      }
      expect(raw).toContain('/Size 7 /Root 1 0 R /Info 6 0 R');
    });

    it('has a stream length that is the length of the stream', () => {
      const content = /5 0 obj\n<< \/Length (\d+) >>\nstream\n([\s\S]*?)\nendstream/.exec(raw)!;
      expect(Number(content[1])).toBe(content[2].length);
    });

    it('uses a grey colour space for a grey picture', () => {
      const grey = buildPdf(header(0xc0, { components: 1, width: 10, height: 10 }));
      expect(text(grey)).toContain('/ColorSpace /DeviceGray');
    });

    it('stores a Hindi title as UTF-16', () => {
      const out = text(buildPdf(header(0xc0), { title: 'कहानी' }));
      expect(out).toContain('/Title <FEFF09150939093E09280940>');
    });

    it('gives the same bytes for the same input', () => {
      expect(Array.from(buildPdf(jpeg, { title: 'a' }))).toEqual(Array.from(buildPdf(jpeg, { title: 'a' })));
    });

    it('does not put a date or any other personal detail in the file', () => {
      expect(raw).not.toMatch(/CreationDate|ModDate|Author|Creator/);
    });

    it('refuses bytes that are not a JPEG', () => {
      expect(() => buildPdf(new Uint8Array([1, 2, 3, 4]))).toThrowError(/Not a JPEG/);
    });
  });
});
