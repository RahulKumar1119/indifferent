import {
  FontSlice,
  buildFontEmbedCss,
  chooseSlices,
  codePointsOf,
  covers,
  nearestWeight,
  parseFontSlices,
  parseUnicodeRange,
  rangesToCss,
  textByWeight,
  toBase64,
} from './font-slices';

/** Two rules in the shape @fontsource writes them. */
const CSS = `
/* noto-sans-devanagari-devanagari-400-normal */
@font-face {
  font-family: 'Noto Sans Devanagari';
  font-style: normal;
  font-display: swap;
  font-weight: 400;
  src: url(./files/noto-sans-devanagari-devanagari-400-normal.woff2) format('woff2'), url(./files/noto-sans-devanagari-devanagari-400-normal.woff) format('woff');
  unicode-range: U+0900-097F,U+1CD0-1CF9,U+200C-200D,U+20A8,U+20B9,U+25CC;
}
/* noto-sans-sc-[4]-400-normal */
@font-face {
  font-family: 'Noto Sans SC';
  font-style: normal;
  font-weight: 400;
  src: url(./files/noto-sans-sc-4-400-normal.woff2) format('woff2'), url(./files/noto-sans-sc-4-400-normal.woff) format('woff');
  unicode-range: U+4e00-4e0f,U+4e2d;
}
@font-face {
  font-family: 'Noto Sans SC';
  font-style: normal;
  font-weight: 700;
  src: url(./files/noto-sans-sc-4-700-normal.woff2) format('woff2');
  unicode-range: U+4e00-4e0f,U+4e2d;
}
`;

const BASE = 'https://example.test/fonts/pkg/400.css';

function slice(family: string, weight: number, ranges: [number, number][], name = `${family}-${weight}`): FontSlice {
  return { family, weight, style: 'normal', url: `https://example.test/${name}.woff2`, ranges };
}

describe('parseUnicodeRange', () => {
  it('reads single points, ranges, mixed case and spaces', () => {
    expect(parseUnicodeRange('U+0900-097F, u+20a8 ,U+25CC')).toEqual([[0x900, 0x97f], [0x20a8, 0x20a8], [0x25cc, 0x25cc]]);
  });
  it('reads a trailing wildcard as a range', () => {
    expect(parseUnicodeRange('U+4??')).toEqual([[0x400, 0x4ff]]);
    expect(parseUnicodeRange('U+1f??')).toEqual([[0x1f00, 0x1fff]]);
  });
  it('skips what it cannot read instead of guessing', () => {
    expect(parseUnicodeRange('U+??4, U+zz, U+20-10, U+110000, U+4?-5, nonsense, U+41')).toEqual([[0x41, 0x41]]);
    expect(parseUnicodeRange('')).toEqual([]);
  });
  it('round trips through rangesToCss', () => {
    const r: [number, number][] = [[0x900, 0x97f], [0x20a8, 0x20a8]];
    expect(rangesToCss(r)).toBe('U+900-97F,U+20A8');
    expect(parseUnicodeRange(rangesToCss(r))).toEqual(r);
  });
});

describe('parseFontSlices', () => {
  const slices = parseFontSlices(CSS, BASE);
  it('reads each rule: family, weight, file and ranges', () => {
    expect(slices.length).toBe(3);
    expect(slices[0]).toEqual({
      family: 'Noto Sans Devanagari',
      weight: 400,
      style: 'normal',
      url: 'https://example.test/fonts/pkg/files/noto-sans-devanagari-devanagari-400-normal.woff2',
      ranges: [[0x900, 0x97f], [0x1cd0, 0x1cf9], [0x200c, 0x200d], [0x20a8, 0x20a8], [0x20b9, 0x20b9], [0x25cc, 0x25cc]],
    });
    expect(slices[2].weight).toBe(700);
  });
  it('takes the woff2 file even when a woff follows it', () => {
    expect(slices.every((s) => s.url.endsWith('.woff2'))).toBeTrue();
  });
  it('ignores comments and rules with no woff2 file', () => {
    const css = `/* @font-face { font-family: 'X'; src: url(a.woff2); } */ @font-face { font-family: 'Y'; src: url(b.woff); }`;
    expect(parseFontSlices(css, BASE)).toEqual([]);
  });
  it('treats a rule with no unicode-range as covering everything, and bold as 700', () => {
    const [s] = parseFontSlices(`@font-face { font-family: "Z"; font-weight: bold; src: url(z.woff2) format('woff2'); }`, BASE);
    expect(s.weight).toBe(700);
    expect(covers(s, 0x41) && covers(s, 0x1f600)).toBeTrue();
  });
});

describe('codePointsOf', () => {
  it('lists each code point once, sorted, counting an astral character as one', () => {
    expect(codePointsOf('baab😀')).toEqual([0x61, 0x62, 0x1f600]);
  });
  it('leaves out line breaks and control characters but keeps spaces and joiners', () => {
    expect(codePointsOf('a\n\t b\u200d\u200c\u0085')).toEqual([0x20, 0x61, 0x62, 0x200c, 0x200d]);
  });
});

describe('nearestWeight', () => {
  it('prefers an exact weight', () => expect(nearestWeight(700, [400, 700])).toBe(700));
  it('goes lighter first at or under 500, heavier first above', () => {
    expect(nearestWeight(500, [400, 700])).toBe(400);
    expect(nearestWeight(300, [400, 700])).toBe(400);
    expect(nearestWeight(600, [400, 700])).toBe(700);
    expect(nearestWeight(900, [400, 700])).toBe(700);
  });
  it('has no answer with no faces', () => expect(nearestWeight(400, [])).toBeNull());
});

describe('chooseSlices', () => {
  const latin = slice('Noto Sans', 400, [[0x20, 0x7e]], 'latin');
  const deva = slice('Noto Sans Devanagari', 400, [[0x900, 0x97f], [0x200c, 0x200d], [0x20, 0x7e]], 'deva');
  const han1 = slice('Noto Sans SC', 400, [[0x4e00, 0x4e0f]], 'han1');
  const han2 = slice('Noto Sans SC', 400, [[0x5000, 0x500f]], 'han2');
  const bold = slice('Noto Sans', 700, [[0x20, 0x7e]], 'latin-bold');
  const all = [latin, deva, han1, han2, bold];
  const stack = ['Noto Sans', 'Noto Sans Devanagari', 'Noto Sans SC'];

  it('takes each character from the first family in the stack that covers it', () => {
    // The Devanagari file also covers Latin; the Latin family comes first, so it wins.
    const r = chooseSlices('ab हिन्दी', stack, 400, all);
    expect(r.slices.map((s) => s.url.split('/').pop())).toEqual(['latin.woff2', 'deva.woff2']);
    expect(r.uncovered).toEqual([]);
  });
  it('picks only the Chinese slices the text reaches, not all of them', () => {
    expect(chooseSlices('\u4e01', stack, 400, all).slices).toEqual([han1]);
    expect(chooseSlices('\u4e01\u5001', stack, 400, all).slices).toEqual([han1, han2]);
  });
  it('names the characters no family covers', () => {
    const r = chooseSlices('a\u0600\u4e01\u{1f600}', stack, 400, all);
    expect(r.uncovered).toEqual([0x600, 0x1f600]);
    expect(r.slices).toEqual([latin, han1]);
  });
  it('uses the weight asked for, or the nearest the family has', () => {
    expect(chooseSlices('a', stack, 700, all).slices).toEqual([bold]);
    // Devanagari has no bold here, so bold Devanagari text falls back to its regular file.
    expect(chooseSlices('\u0915', stack, 700, all).slices).toEqual([deva]);
  });
  it('ignores a family that is not in the stack', () => {
    expect(chooseSlices('\u0915', ['Noto Sans'], 400, all).uncovered).toEqual([0x915]);
  });
});

describe('toBase64', () => {
  it('matches btoa for a buffer larger than one piece', () => {
    const bytes = new Uint8Array(0x8000 * 2 + 5).map((_, i) => (i * 31 + 7) % 256);
    let binary = '';
    bytes.forEach((b) => (binary += String.fromCharCode(b)));
    expect(toBase64(bytes.buffer)).toBe(btoa(binary));
  });
  it('encodes an empty buffer as empty', () => expect(toBase64(new ArrayBuffer(0))).toBe(''));
});

describe('buildFontEmbedCss', () => {
  const latin = slice('Noto Sans', 400, [[0x20, 0x7e]], 'latin');
  const latinBold = slice('Noto Sans', 700, [[0x20, 0x7e]], 'latin-bold');
  const han = slice('Noto Sans SC', 400, [[0x4e00, 0x4e0f]], 'han');
  const unused = slice('Noto Sans SC', 400, [[0x6000, 0x600f]], 'unused');
  const stack = ['Noto Sans', 'Noto Sans SC'];
  const bytesFor = (url: string) => new TextEncoder().encode(url).buffer as ArrayBuffer;

  it('embeds each needed file once, as a data URL with its unicode-range, and never loads the others', async () => {
    const loaded: string[] = [];
    const r = await buildFontEmbedCss({
      texts: new Map([[400, 'ab\u4e01'], [700, 'ab']]),
      stack,
      slices: [latin, latinBold, han, unused],
      load: async (u) => (loaded.push(u), bytesFor(u)),
    });
    expect(loaded.sort()).toEqual([han.url, latin.url, latinBold.url].sort());
    expect(r.slices.length).toBe(3);
    expect(r.css.match(/@font-face/g)!.length).toBe(3);
    expect(r.css).toContain("font-family:'Noto Sans SC'");
    expect(r.css).toContain('unicode-range:U+4E00-4E0F;');
    expect(r.css).toContain(`base64,${btoa(han.url)}`);
    expect(r.css).not.toContain('unused');
    expect(r.bytes).toBe([han.url, latin.url, latinBold.url].reduce((n, u) => n + u.length, 0));
    expect(r.uncovered).toEqual([]);
  });
  it('loads a file shared by two weights only once', async () => {
    const both = slice('Noto Sans', 400, [[0x20, 0x7e]], 'both');
    let calls = 0;
    await buildFontEmbedCss({ texts: new Map([[400, 'a'], [500, 'b']]), stack: ['Noto Sans'], slices: [both], load: async (u) => (calls++, bytesFor(u)) });
    expect(calls).toBe(1);
  });
  it('reports characters it has no font for, and still builds the rest', async () => {
    const r = await buildFontEmbedCss({ texts: new Map([[400, 'a\u0600']]), stack, slices: [latin], load: async (u) => bytesFor(u) });
    expect(r.uncovered).toEqual([0x600]);
    expect(r.slices).toEqual([latin]);
  });
  it('makes no CSS for no text', async () => {
    const r = await buildFontEmbedCss({ texts: new Map(), stack, slices: [latin], load: async () => new ArrayBuffer(0) });
    expect(r.css).toBe('');
  });
  it('lets a failed load stop the export instead of embedding a broken file', async () => {
    await expectAsync(
      buildFontEmbedCss({ texts: new Map([[400, 'a']]), stack, slices: [latin], load: async () => { throw new Error('offline'); } }),
    ).toBeRejectedWithError('offline');
  });
  it('quotes a family name safely', async () => {
    const odd = slice("Odd'Name", 400, [[0x20, 0x7e]]);
    const r = await buildFontEmbedCss({ texts: new Map([[400, 'a']]), stack: ["Odd'Name"], slices: [odd], load: async (u) => bytesFor(u) });
    expect(r.css).toContain("font-family:'Odd\\'Name'");
  });
});

describe('textByWeight', () => {
  let host: HTMLElement;
  beforeEach(() => {
    host = document.createElement('div');
    document.body.appendChild(host);
  });
  afterEach(() => host.remove());

  it('groups text by the weight it is drawn at, and skips blank nodes', () => {
    host.innerHTML = '<p style="font-weight:400">one <b style="font-weight:700">two</b></p><h1 style="font-weight:700">three</h1><p> </p>';
    const m = textByWeight(host);
    expect(m.get(400)).toBe('one ');
    expect(m.get(700)).toBe('twothree');
    expect(m.size).toBe(2);
  });
  it('reads text inside nested elements and lines in other scripts', () => {
    host.innerHTML = '<div dir="rtl"><span>\u0645\u0631\u062d\u0628\u0627</span></div><div lang="zh"><i>\u4e2d\u6587</i></div>';
    expect(m400(textByWeight(host))).toContain('\u0645\u0631\u062d\u0628\u0627');
    expect(m400(textByWeight(host))).toContain('\u4e2d\u6587');
  });
});

function m400(m: Map<number, string>): string {
  return m.get(400) ?? '';
}
