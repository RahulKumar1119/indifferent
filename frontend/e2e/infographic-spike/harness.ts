// Browser side of the #217 spike. It draws one sample infographic (text in one
// script, three steps, a chart) at one of the three export sizes, then exports
// it the way the tool will: html-to-image with a font stylesheet built from the
// characters on the page (font-slices.ts). run.mjs drives it and checks the result.
import { toJpeg, toPng } from 'html-to-image';
import { BarController, BarElement, CategoryScale, Chart, LinearScale } from 'chart.js';
import {
  FontSlice,
  buildFontEmbedCss,
  chooseSlices,
  parseFontSlices,
  textByWeight,
} from '../../src/app/pages/tools/infographic/font-slices';

Chart.register(BarController, BarElement, CategoryScale, LinearScale);

const STACK = ['Noto Sans', 'Noto Sans Arabic', 'Noto Sans SC'];
const STYLESHEETS = [
  '/fonts/noto-sans/400.css',
  '/fonts/noto-sans/700.css',
  '/fonts/noto-sans-arabic/400.css',
  '/fonts/noto-sans-arabic/700.css',
  '/fonts/noto-sans-sc/400.css',
  '/fonts/noto-sans-sc/700.css',
];

type Lang = 'en' | 'hi' | 'ar' | 'zh';
type Size = 'portrait' | 'square' | 'landscape';
const SIZES: Record<Size, [number, number]> = { portrait: [1080, 1920], square: [1080, 1080], landscape: [1920, 1080] };

interface Sample {
  lang: string;
  dir: 'ltr' | 'rtl';
  title: string;
  sub: string;
  steps: [string, string][];
  chartTitle: string;
  labels: string[];
  stress: string;
  brand: string;
}

// Text a real user might type: each language mixes in Latin words, digits or
// symbols, and each has characters that stress shaping or the font slices.
const SAMPLES: Record<Lang, Sample> = {
  en: {
    lang: 'en', dir: 'ltr',
    title: 'How a story becomes a video',
    sub: 'Three steps, no account. Nothing leaves your browser.',
    steps: [
      ['Write the story', 'Paste up to 5,000 characters in any language.'],
      ['Pick the pictures', 'Search free photos and clips for every scene.'],
      ['Make the video', 'Choose a voice and captions, then export 1080 \u00d7 1920.'],
    ],
    chartTitle: 'Videos made per day',
    labels: ['Mon', 'Tue', 'Wed', 'Thu', 'Fri'],
    stress: 'Office \ufb01ne affl\u00e9 \u2014 \u00c5ngstr\u00f6m \u00b7 na\u00efve \u00b7 \u0161\u0107\u017e \u00b7 \u0422\u0435\u0441\u0442 \u00b7 \u0394\u03bf\u03ba\u03b9\u03bc\u03ae \u00b7 Vi\u1ec7t Nam',
    brand: 'indifferent.fun',
  },
  hi: {
    lang: 'hi', dir: 'ltr',
    title: '\u0915\u0939\u093e\u0928\u0940 \u0938\u0947 \u0935\u0940\u0921\u093f\u092f\u094b \u0915\u0948\u0938\u0947 \u092c\u0928\u0924\u093e \u0939\u0948',
    sub: '\u0924\u0940\u0928 \u0906\u0938\u093e\u0928 \u091a\u0930\u0923, \u092c\u093f\u0928\u093e \u0916\u093e\u0924\u0947 \u0915\u0947\u0964 \u0906\u092a\u0915\u093e \u0921\u0947\u091f\u093e browser \u092e\u0947\u0902 \u0939\u0940 \u0930\u0939\u0924\u093e \u0939\u0948\u0964',
    steps: [
      ['\u0915\u0939\u093e\u0928\u0940 \u0932\u093f\u0916\u0947\u0902', '\u0915\u093f\u0938\u0940 \u092d\u0940 \u092d\u093e\u0937\u093e \u092e\u0947\u0902 5,000 \u0905\u0915\u094d\u0937\u0930 \u0924\u0915 \u092a\u0947\u0938\u094d\u091f \u0915\u0930\u0947\u0902, \u091c\u0948\u0938\u0947 \u091c\u094d\u091e\u093e\u0928, \u0936\u094d\u0930\u0926\u094d\u0927\u093e, \u0915\u094d\u0937\u0924\u094d\u0930\u093f\u092f \u092f\u093e \u0935\u093f\u0926\u094d\u092f\u093e\u0932\u092f\u0964'],
      ['\u0924\u0938\u094d\u0935\u0940\u0930\u0947\u0902 \u091a\u0941\u0928\u0947\u0902', '\u0939\u0930 \u0926\u0943\u0936\u094d\u092f \u0915\u0947 \u0932\u093f\u090f \u092e\u0941\u092b\u093c\u094d\u0924 photos \u0914\u0930 clips \u0916\u094b\u091c\u0947\u0902\u0964'],
      ['\u0935\u0940\u0921\u093f\u092f\u094b \u092c\u0928\u093e\u090f\u0901', '\u0906\u0935\u093e\u091c\u093c \u0914\u0930 captions \u091a\u0941\u0928\u0947\u0902, \u092b\u093f\u0930 YouTube Shorts \u0915\u0947 \u0932\u093f\u090f 9:16 \u092e\u0947\u0902 export \u0915\u0930\u0947\u0902\u0964'],
    ],
    chartTitle: '\u0930\u094b\u095b \u092c\u0928\u0947 \u0935\u0940\u0921\u093f\u092f\u094b',
    labels: ['\u0938\u094b\u092e', '\u092e\u0902\u0917\u0932', '\u092c\u0941\u0927', '\u0917\u0941\u0930\u0941', '\u0936\u0941\u0915\u094d\u0930'],
    stress: '\u0915\u094d\u0937\u0924\u094d\u0930\u093f\u092f \u00b7 \u091c\u094d\u091e\u093e\u0928 \u00b7 \u0936\u094d\u0930\u0926\u094d\u0927\u093e \u00b7 \u0926\u094d\u0935\u093e\u0930\u093e \u00b7 \u092a\u094d\u0930\u0915\u093e\u0936 \u00b7 \u0915\u0943\u0937\u093f \u00b7 \u0936\u0915\u094d\u0924\u093f \u00b7 \u0930\u093e\u0937\u094d\u091f\u094d\u0930 \u00b7 \u0938\u0902\u0938\u094d\u0915\u0943\u0924 \u00b7 \u0909\u091c\u094d\u091c\u094d\u0935\u0932',
    brand: 'indifferent.fun',
  },
  ar: {
    lang: 'ar', dir: 'rtl',
    title: '\u0643\u064a\u0641 \u062a\u062a\u062d\u0648\u0644 \u0627\u0644\u0642\u0635\u0629 \u0625\u0644\u0649 \u0641\u064a\u062f\u064a\u0648',
    sub: '\u062b\u0644\u0627\u062b \u062e\u0637\u0648\u0627\u062a \u0628\u0633\u064a\u0637\u0629\u060c \u0628\u062f\u0648\u0646 \u062d\u0633\u0627\u0628. \u0628\u064a\u0627\u0646\u0627\u062a\u0643 \u062a\u0628\u0642\u0649 \u0641\u064a \u0645\u062a\u0635\u0641\u062d\u0643 browser.',
    steps: [
      ['\u0627\u0643\u062a\u0628 \u0627\u0644\u0642\u0635\u0629', '\u0627\u0644\u0635\u0642 \u062d\u062a\u0649 5,000 \u062d\u0631\u0641 \u0628\u0623\u064a \u0644\u063a\u0629.'],
      ['\u0627\u062e\u062a\u0631 \u0627\u0644\u0635\u0648\u0631', '\u0627\u0628\u062d\u062b \u0639\u0646 \u0635\u0648\u0631 \u0648\u0645\u0642\u0627\u0637\u0639 \u0645\u062c\u0627\u0646\u064a\u0629 \u0644\u0643\u0644 \u0645\u0634\u0647\u062f \u0639\u0644\u0649 Pixabay.'],
      ['\u0627\u0635\u0646\u0639 \u0627\u0644\u0641\u064a\u062f\u064a\u0648', '\u0627\u062e\u062a\u0631 \u0627\u0644\u0635\u0648\u062a \u0648\u0627\u0644\u062a\u0631\u062c\u0645\u0629 \u062b\u0645 \u0635\u062f\u0651\u0631 \u0628\u062d\u062c\u0645 1080 \u00d7 1920 \u0644\u0640 YouTube Shorts.'],
    ],
    chartTitle: '\u0627\u0644\u0641\u064a\u062f\u064a\u0648\u0647\u0627\u062a \u0627\u0644\u0645\u0646\u062a\u062c\u0629 \u064a\u0648\u0645\u064a\u064b\u0627',
    labels: ['\u0627\u0644\u0625\u062b\u0646\u064a\u0646', '\u0627\u0644\u062b\u0644\u0627\u062b\u0627\u0621', '\u0627\u0644\u0623\u0631\u0628\u0639\u0627\u0621', '\u0627\u0644\u062e\u0645\u064a\u0633', '\u0627\u0644\u062c\u0645\u0639\u0629'],
    stress: '\u0645\u064e\u0631\u0652\u062d\u064e\u0628\u064b\u0627 \u0628\u0650\u0643\u064f\u0645\u0652 \u0641\u0650\u064a \u0627\u0644\u0652\u0639\u064e\u0627\u0644\u064e\u0645\u0650 \u00b7 \u0644\u0627 \u00b7 \u0627\u0644\u0644\u0647 \u00b7 \u06f1\u06f2\u06f3 \u00b7 \u0661\u0662\u0663',
    brand: 'indifferent.fun',
  },
  zh: {
    lang: 'zh-Hans', dir: 'ltr',
    title: '\u6545\u4e8b\u5982\u4f55\u53d8\u6210\u89c6\u9891',
    sub: '\u4e09\u4e2a\u7b80\u5355\u6b65\u9aa4\uff0c\u65e0\u9700\u8d26\u53f7\uff0c\u5185\u5bb9\u4e0d\u4f1a\u79bb\u5f00\u4f60\u7684 browser\u3002',
    steps: [
      ['\u5199\u4e0b\u6545\u4e8b', '\u7c98\u8d34\u6700\u591a 5000 \u4e2a\u5b57\u7b26\uff0c\u652f\u6301\u4efb\u4f55\u8bed\u8a00\u3002'],
      ['\u6311\u9009\u56fe\u7247', '\u4e3a\u6bcf\u4e2a\u573a\u666f\u641c\u7d22\u514d\u8d39\u7684\u7167\u7247\u548c\u89c6\u9891\u7247\u6bb5\u3002'],
      ['\u5236\u4f5c\u89c6\u9891', '\u9009\u62e9\u914d\u97f3\u548c\u5b57\u5e55\uff0c\u7136\u540e\u4e3a YouTube Shorts \u5bfc\u51fa 1080\u00d71920\u3002'],
    ],
    chartTitle: '\u6bcf\u65e5\u751f\u6210\u7684\u89c6\u9891',
    labels: ['\u5468\u4e00', '\u5468\u4e8c', '\u5468\u4e09', '\u5468\u56db', '\u5468\u4e94'],
    stress: '\u9b91\u9b45\u9b4d\u9b49 \u00b7 \u9b31 \u00b7 \u9f18 \u00b7 \u56bc \u00b7 \u6e3a \u00b7 \u8bc6\u522b \u00b7 \u8f6c\u6362 \u00b7 \u8d28\u91cf',
    brand: 'indifferent.fun',
  },
};

const el = (tag: string, cls = '', text = ''): HTMLElement => {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text) e.textContent = text;
  return e;
};

let slices: FontSlice[] = [];
const fileCache = new Map<string, Promise<ArrayBuffer>>();
const load = (url: string): Promise<ArrayBuffer> => {
  if (!fileCache.has(url)) fileCache.set(url, fetch(url).then((r) => r.arrayBuffer()));
  return fileCache.get(url)!;
};

let chart: Chart | null = null;
let chartLabels: string[] = [];

function build(lang: Lang, size: Size, scale: number, long = false): void {
  const s = SAMPLES[lang];
  const [w, h] = SIZES[size];
  const stage = document.getElementById('stage')!;
  const holder = document.getElementById('holder')!;
  stage.replaceChildren();
  stage.className = `stage ${size}`;
  stage.setAttribute('lang', s.lang);
  stage.setAttribute('dir', s.dir);
  stage.style.width = `${w}px`;
  stage.style.height = `${h}px`;
  stage.style.transform = scale === 1 ? '' : `scale(${scale})`;
  stage.style.transformOrigin = 'top left';
  holder.style.width = `${w * scale}px`;
  holder.style.height = `${h * scale}px`;
  holder.style.overflow = 'hidden';

  const header = el('header');
  const h1 = el('h1', '', s.title);
  h1.setAttribute('dir', 'auto');
  const sub = el('p', 'sub', s.sub);
  sub.setAttribute('dir', 'auto');
  header.append(h1, sub);

  const steps = el('ol', 'steps');
  s.steps.forEach(([head, body], i) => {
    const li = el('li');
    li.append(el('span', 'n', String(i + 1)));
    const text = el('div');
    const h2 = el('h2', '', head);
    h2.setAttribute('dir', 'auto');
    // `long` repeats each step's text eight times, to prove too much text is noticed.
    const p = el('p', '', long ? Array(8).fill(body).join(' ') : body);
    p.setAttribute('dir', 'auto');
    text.append(h2, p);
    li.append(text);
    steps.append(li);
  });

  const panel = el('section', 'panel');
  const h3 = el('h3', '', s.chartTitle);
  h3.setAttribute('dir', 'auto');
  const canvas = document.createElement('canvas');
  const cw = size === 'landscape' ? 760 : 880;
  const ch = size === 'portrait' ? 420 : size === 'square' ? 170 : 480;
  canvas.width = cw;
  canvas.height = ch;
  canvas.style.width = `${cw}px`;
  canvas.style.height = `${ch}px`;
  canvas.setAttribute('dir', s.dir);
  panel.append(h3, canvas);

  const footer = el('footer');
  const stress = el('span', 'stress', s.stress);
  stress.setAttribute('dir', 'auto');
  footer.append(stress, el('span', '', s.brand));

  if (size === 'landscape') {
    const right = el('div');
    right.style.display = 'flex';
    right.style.flexDirection = 'column';
    right.append(panel);
    stage.append(header, steps, right, footer);
  } else {
    stage.append(header, steps, panel, footer);
  }

  chartLabels = s.labels;
  chart?.destroy();
  chart = new Chart(canvas, {
    type: 'bar',
    data: { labels: s.labels, datasets: [{ data: [12, 19, 8, 15, 22], backgroundColor: '#BC5227', borderRadius: 6 }] },
    options: {
      animation: false,
      responsive: false,
      devicePixelRatio: 1,
      plugins: { legend: { display: false }, tooltip: { enabled: false } },
      scales: {
        x: { reverse: s.dir === 'rtl', ticks: { font: { family: STACK.map((f) => `'${f}'`).join(','), size: 24 }, color: '#231F1A' }, grid: { display: false } },
        y: { position: s.dir === 'rtl' ? 'right' : 'left', ticks: { font: { family: STACK.map((f) => `'${f}'`).join(','), size: 22 }, color: '#4A4238' } },
      },
    },
  });
}

async function fontsReady(stage: HTMLElement): Promise<void> {
  const text = [...textByWeight(stage).values()].join('') + chartLabels.join('');
  await Promise.all(
    STACK.flatMap((f) => [400, 700].map((w) => document.fonts.load(`${w} 40px "${f}"`, text).catch(() => []))),
  );
  await document.fonts.ready;
}

/** Elements whose text does not fit, or content taller than the canvas. */
function overflow(stage: HTMLElement): string[] {
  const problems: string[] = [];
  if (stage.scrollHeight > stage.clientHeight + 1) problems.push(`content is ${stage.scrollHeight - stage.clientHeight}px taller than the canvas`);
  if (stage.scrollWidth > stage.clientWidth + 1) problems.push(`content is ${stage.scrollWidth - stage.clientWidth}px wider than the canvas`);
  stage.querySelectorAll<HTMLElement>('h1, .sub, h2, .steps p, h3, footer span').forEach((e) => {
    if (e.scrollWidth > e.clientWidth + 1) problems.push(`${e.tagName.toLowerCase()} text is wider than its box`);
  });
  return problems;
}

async function psnr(pngUrl: string, jpgUrl: string): Promise<number> {
  const draw = async (url: string) => {
    const img = new Image();
    img.src = url;
    await img.decode();
    const c = document.createElement('canvas');
    c.width = img.naturalWidth;
    c.height = img.naturalHeight;
    const g = c.getContext('2d')!;
    g.drawImage(img, 0, 0);
    return g.getImageData(0, 0, c.width, c.height).data;
  };
  const [a, b] = await Promise.all([draw(pngUrl), draw(jpgUrl)]);
  let sum = 0;
  let n = 0;
  for (let i = 0; i < a.length; i += 4) {
    for (let k = 0; k < 3; k++) {
      const d = a[i + k] - b[i + k];
      sum += d * d;
      n++;
    }
  }
  const mse = sum / n;
  return mse === 0 ? 99 : 10 * Math.log10((255 * 255) / mse);
}

const b64 = (dataUrl: string) => dataUrl.slice(dataUrl.indexOf(',') + 1);
const decodedSize = async (url: string) => {
  const img = new Image();
  img.src = url;
  await img.decode();
  return [img.naturalWidth, img.naturalHeight];
};

const spike = {
  async init(): Promise<void> {
    const all: FontSlice[] = [];
    for (const path of STYLESHEETS) {
      const css = await (await fetch(path)).text();
      all.push(...parseFontSlices(css, new URL(path, location.href).href));
    }
    slices = all;
  },

  show(lang: Lang, size: Size, scale = 1, long = false): void {
    build(lang, size, scale, long);
  },

  async ready(): Promise<{ overflow: string[]; slices: number }> {
    const stage = document.getElementById('stage')!;
    await fontsReady(stage);
    chart?.update('none');
    await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
    return { overflow: overflow(stage), slices: slices.length };
  },

  /** Export as the tool will: our font CSS, the transform reset, real size. */
  async exportOurs(): Promise<unknown> {
    const stage = document.getElementById('stage')!;
    const [w, h] = [parseInt(stage.style.width, 10), parseInt(stage.style.height, 10)];
    const t0 = performance.now();
    const texts = textByWeight(stage);
    texts.set(400, (texts.get(400) ?? '') + chartLabels.join(''));
    const embed = await buildFontEmbedCss({ texts, stack: STACK, slices, load });
    const t1 = performance.now();
    const opts = { width: w, height: h, pixelRatio: 1, fontEmbedCSS: embed.css, style: { transform: 'none' } };
    const png = await toPng(stage, opts);
    const t2 = performance.now();
    const jpg = await toJpeg(stage, { ...opts, quality: 0.92, backgroundColor: '#FBF6EE' });
    const t3 = performance.now();
    return {
      embed: {
        files: embed.slices.map((s) => s.url.split('/').pop()),
        count: embed.slices.length,
        woff2Bytes: embed.bytes,
        cssBytes: embed.css.length,
        uncovered: embed.uncovered.map((c) => 'U+' + c.toString(16).toUpperCase()),
        ms: Math.round(t1 - t0),
      },
      png: { b64: b64(png), bytes: Math.round((b64(png).length * 3) / 4), size: await decodedSize(png), ms: Math.round(t2 - t1) },
      jpg: { bytes: Math.round((b64(jpg).length * 3) / 4), size: await decodedSize(jpg), ms: Math.round(t3 - t2), psnr: Math.round((await psnr(png, jpg)) * 10) / 10 },
      embeddedChars: [...new Set([...texts.values()].join(''))].length,
      // For the font check: which file each character was taken from.
      perChar: [...texts.entries()].flatMap(([weight, text]) =>
        [...new Set([...text])].map((ch) => {
          const cp = ch.codePointAt(0)!;
          const found = chooseSlices(ch, STACK, weight, slices).slices[0];
          return [cp, weight, found ? found.url.split('/').pop() : null];
        }),
      ),
    };
  },

  /** What html-to-image does by itself, to see why the slicing is needed. */
  async exportNaive(): Promise<unknown> {
    const stage = document.getElementById('stage')!;
    const [w, h] = [parseInt(stage.style.width, 10), parseInt(stage.style.height, 10)];
    const t0 = performance.now();
    const png = await toPng(stage, { width: w, height: h, pixelRatio: 1, style: { transform: 'none' } });
    return { ms: Math.round(performance.now() - t0), bytes: Math.round((b64(png).length * 3) / 4), size: await decodedSize(png) };
  },
};

(window as unknown as { spike: typeof spike }).spike = spike;
