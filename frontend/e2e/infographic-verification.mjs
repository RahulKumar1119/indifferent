// Real-browser check of the infographic maker (#217). Not part of `ng test`.
//   cd frontend && npx ng build --configuration development
//   node e2e/infographic-verification.mjs [outDir] [--quick]
// Serves dist/frontend/browser, blocks every request that leaves localhost, and for each template, size and
// language (English from the template, Hindi, Gujarati) renders the page, downloads a PNG and a JPG through the
// page's own buttons, and checks their pixel size, that nothing overflows, that the JPG matches the PNG, and that
// the PNG matches the preview. Also checks the keyboard, undo and redo, autosave and the right-to-left mirror.
// Writes screenshots and results.json to outDir (default ~/shots303/infographic), never into the repo. Exit 1 on any failure.
import fs from 'node:fs';
import { execFileSync } from 'node:child_process';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '..');
const require = createRequire(path.join(root, '/'));
const puppeteer = require('puppeteer');
const { PNG } = require('pngjs');
const pixelmatch = require('pixelmatch').default;

const args = process.argv.slice(2);
const quick = args.includes('--quick');
const outDir = path.resolve(args.find((a) => !a.startsWith('--')) ?? path.join(process.env.HOME, 'shots303/infographic'));
fs.mkdirSync(outDir, { recursive: true });
const dist = path.join(root, 'dist/frontend/browser');
if (!fs.existsSync(path.join(dist, 'index.html'))) throw new Error('Build first: npx ng build --configuration development');

const SIZES = { portrait: [1080, 1920], square: [1080, 1080], landscape: [1920, 1080] };
const DRAFT_KEY = 'indifferent.infographic.draft.v1';

// --- a static server with the single page app fallback
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.mjs': 'text/javascript', '.css': 'text/css', '.woff2': 'font/woff2', '.json': 'application/json', '.svg': 'image/svg+xml', '.png': 'image/png', '.ico': 'image/x-icon', '.webp': 'image/webp', '.jpg': 'image/jpeg' };
const server = http.createServer((req, res) => {
  let p = '/';
  try {
    p = decodeURIComponent(req.url.split('?')[0]);
  } catch {
    /* a malformed escape gets the app page */
  }
  // Resolve against dist and accept only files inside it (the separator keeps a sibling like dist-x out).
  let f = path.resolve(dist, '.' + path.posix.normalize('/' + p));
  const inside = f === dist || f.startsWith(dist + path.sep);
  if (p.includes('\0') || !inside || !fs.existsSync(f) || fs.statSync(f).isDirectory()) f = path.join(dist, 'index.html');
  res.writeHead(200, { 'content-type': mime[path.extname(f)] ?? 'application/octet-stream' });
  fs.createReadStream(f).pipe(res);
});
await new Promise((r) => server.listen(0, '127.0.0.1', r));
const base = `http://127.0.0.1:${server.address().port}`;

// --- texts for the languages. English is each template's own starting text. These are checked for
// characters and layout by this script only; a reader of the language must still look at the screenshots.
const HI = {
  title: 'कहानी से वीडियो कैसे बनता है',
  subtitle: 'तीन आसान चरण, बिना खाते के। आपका डेटा browser में ही रहता है।',
  footer: 'क्षत्रिय · ज्ञान · श्रद्धा · द्वारा · प्रकाश · कृषि · शक्ति · राष्ट्र · संस्कृत · उज्ज्वल',
  sections: [
    ['कहानी लिखें', 'किसी भी भाषा में 5,000 अक्षर तक पेस्ट करें, जैसे ज्ञान, श्रद्धा, क्षत्रिय या विद्यालय।'],
    ['तस्वीरें चुनें', 'हर दृश्य के लिए मुफ़्त photos और clips खोजें।'],
    ['वीडियो बनाएँ', 'आवाज़ और captions चुनें, फिर YouTube Shorts के लिए 9:16 में export करें।'],
    ['डाउनलोड करें', 'तैयार फ़ाइल आपके डिवाइस पर सेव हो जाती है।'],
  ],
};
const GU = {
  title: 'વાર્તા કેવી રીતે વિડિયો બને છે',
  subtitle: 'ત્રણ સરળ પગલાં, ખાતા વગર. તમારી માહિતી તમારા બ્રાઉઝરમાં જ રહે છે.',
  footer: 'ગુજરાતી · ભાષા · શિક્ષણ · વિજ્ઞાન',
  sections: [
    ['વાર્તા લખો', 'કોઈ પણ ભાષામાં ૫,૦૦૦ અક્ષરો સુધી પેસ્ટ કરો.'],
    ['ચિત્રો પસંદ કરો', 'દરેક દૃશ્ય માટે મફત ફોટા અને ક્લિપ્સ શોધો.'],
    ['વિડિયો બનાવો', 'અવાજ અને કૅપ્શન પસંદ કરો, પછી 1080 × 1920 માં export કરો.'],
    ['ડાઉનલોડ કરો', 'તૈયાર ફાઇલ તમારા ઉપકરણ પર સેવ થશે.'],
  ],
};
const AR = {
  title: 'كيف تتحول القصة إلى فيديو',
  subtitle: 'ثلاث خطوات بسيطة، بدون حساب.',
  footer: 'indifferent.fun',
  sections: [
    ['اكتب القصة', 'الصق حتى 5,000 حرف بأي لغة.'],
    ['اختر الصور', 'ابحث عن صور ومقاطع مجانية.'],
    ['اصنع الفيديو', 'اختر الصوت ثم صدّر الفيديو.'],
  ],
};
const LANGS = { hi: HI, gu: GU };

/** A stored draft for a template, size and language: the page restores it exactly like a user's own. */
function draft(tpl, format, lang, themeId = 'paper', long = false) {
  const text = lang === 'en' ? null : LANGS[lang] ?? AR;
  const n = Math.min(tpl.max, text ? text.sections.length : tpl.defaults.length);
  const sections = [];
  for (let i = 0; i < Math.max(n, tpl.min); i++) {
    const own = text ? text.sections[i % text.sections.length] : [tpl.defaults[i % tpl.defaults.length].heading, tpl.defaults[i % tpl.defaults.length].body];
    if (tpl.chart && text) {
      const digits = lang === 'gu' ? ['૧૨', '૯', '૭', '૫', '૩', '૨'] : ['१२', '९', '७', '५', '३', '२'];
      const name = long ? own[0].repeat(3).slice(0, 24) : own[0].slice(0, 24);
      sections.push({ id: `s${i}`, label: name, heading: tpl.id === 'big-numbers' ? (lang === 'gu' ? '૧,૨૦૦' : '१,२००') : digits[i % digits.length], body: tpl.noBody ? '' : own[1].slice(0, long ? 120 : 40) });
      continue;
    }
    let body = tpl.noBody ? '' : own[1];
    if (tpl.bullets) body = text ? `${body}\n${own[0]}\n${body.slice(0, 30)}` : body;
    if (long) body = Array(tpl.bullets ? 8 : 3).fill(body).join(tpl.bullets ? '\n' : ' ');
    sections.push({ id: `s${i}`, label: tpl.labelName ? (text ? String(1990 + i * 10) : tpl.defaults[i % tpl.defaults.length].label) : '', heading: own[0], body: body.slice(0, 400) });
  }
  return {
    version: 1,
    templateId: tpl.id,
    format,
    themeId,
    language: 'auto',
    direction: 'auto',
    title: text ? text.title : tpl.title,
    subtitle: text ? text.subtitle : tpl.subtitle,
    sections,
    footer: text ? text.footer : tpl.footer,
    mark: false,
  };
}

// The page's own download buttons are pressed, and Chrome would save every file in ~/Downloads. Send them to a
// temporary folder instead, and remove it at the end; the check reads each file from the page itself.
const downloadDir = fs.mkdtempSync(path.join(os.tmpdir(), 'infographic-downloads-'));
process.on('exit', () => fs.rmSync(downloadDir, { recursive: true, force: true }));
const browser = await puppeteer.launch({ executablePath: '/usr/bin/google-chrome', headless: 'new', args: ['--no-sandbox', '--disable-gpu'] });
await (await browser.target().createCDPSession()).send('Browser.setDownloadBehavior', { behavior: 'allow', downloadPath: downloadDir });
const results = { chrome: await browser.version(), cases: [], interactions: [], failures: [] };
const fail = (msg) => {
  results.failures.push(msg);
  console.log('  FAIL', msg);
};

/**
 * Checks a PDF with the tools a reader would use: pdfinfo (one page, the picture's size at 96 pixels to the inch),
 * qpdf (the file is well formed), pdfimages (one JPEG of the exact pixel size) and pdftoppm (the page, drawn at
 * 96 dpi by two renderers, matches the PNG). Returns the problems found and the PSNR of the drawn page against the PNG.
 */
function checkPdf(bytes, W, H, pngDecoded, label) {
  const problems = [];
  const dir = fs.mkdtempSync(path.join(downloadDir, 'pdf-'));
  const file = path.join(dir, 'x.pdf');
  fs.writeFileSync(file, bytes);
  const run = (cmd, args) => {
    try {
      return { out: execFileSync(cmd, args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }), code: 0 };
    } catch (e) {
      return { out: `${e.stdout ?? ''}${e.stderr ?? ''}`, code: e.status ?? 1 };
    }
  };
  if (String.fromCharCode(...bytes.subarray(0, 5)) !== '%PDF-') problems.push('does not start with %PDF-');
  const info = run('pdfinfo', [file]);
  const pages = Number(/Pages:\s+(\d+)/.exec(info.out)?.[1]);
  const size = /Page size:\s+([\d.]+) x ([\d.]+) pts/.exec(info.out);
  if (info.code !== 0 || pages !== 1) problems.push(`has ${pages} pages (${info.code})`);
  if (!size || Math.abs(Number(size[1]) - W * 0.75) > 0.5 || Math.abs(Number(size[2]) - H * 0.75) > 0.5) problems.push(`page is ${size?.[1]} x ${size?.[2]} pt, expected ${W * 0.75} x ${H * 0.75}`);
  const check = run('qpdf', ['--check', file]);
  if (check.code !== 0 || /WARNING|ERROR/.test(check.out)) problems.push(`qpdf: ${check.out.split('\n').find((l) => /WARNING|ERROR/.test(l)) ?? 'exit ' + check.code}`);
  const images = run('pdfimages', ['-list', file]).out.split('\n').filter((l) => /^\s*1\s/.test(l));
  const cols = (images[0] ?? '').trim().split(/\s+/);
  if (images.length !== 1 || Number(cols[3]) !== W || Number(cols[4]) !== H || cols[8] !== 'jpeg') problems.push(`image list: ${images.join(' | ') || 'none'}`);
  // Two renderers. pdftocairo puts the picture on the page pixel for pixel, so the page must match the PNG
  // closely (the JPEG inside costs a little). pdftoppm smooths the picture when it draws it, so it is only
  // held to "the same page", not to the pixel.
  let psnr = null;
  for (const [tool, args, ext, minPsnr] of [
    ['pdftocairo', ['-r', '96', '-png'], 'c', 38],
    ['pdftoppm', ['-r', '96', '-png'], 'p', 22],
  ]) {
    const drawn = run(tool, [...args, file, path.join(dir, ext)]);
    const page = fs.readdirSync(dir).find((n) => n.startsWith(ext + '-') && n.endsWith('.png'));
    if (drawn.code !== 0 || !page) {
      problems.push(`could not be drawn by ${tool}`);
      continue;
    }
    const png = PNG.sync.read(fs.readFileSync(path.join(dir, page)));
    if (png.width !== W || png.height !== H) {
      problems.push(`${tool} drew ${png.width}x${png.height}, expected ${W}x${H}`);
      continue;
    }
    let sum = 0;
    let n = 0;
    for (let i = 0; i < png.data.length; i += 4) for (let k = 0; k < 3; k++) { const d = png.data[i + k] - pngDecoded.data[i + k]; sum += d * d; n++; }
    const mse = sum / n;
    const value = Math.round((mse === 0 ? 99 : 10 * Math.log10((255 * 255) / mse)) * 10) / 10;
    if (tool === 'pdftocairo') psnr = value;
    if (value < minPsnr) problems.push(`${tool}'s page differs from the PNG (PSNR ${value} dB, needs ${minPsnr})`);
  }
  fs.rmSync(dir, { recursive: true, force: true });
  return { problems, psnr };
}
const settle = (ms = 150) => new Promise((r) => setTimeout(r, ms));
/** Replaces the text of a field the way typing does, without relying on a triple click. */
async function typeInto(page, selector, text) {
  await page.focus(selector);
  await page.evaluate((sel) => { const e = document.querySelector(sel); e.value = ''; e.dispatchEvent(new Event('input', { bubbles: true })); }, selector);
  await page.keyboard.type(text);
  await settle();
}
/** Presses a button from inside the page: the page redraws after a press, which can detach a handle held from outside. */
const pressButton = (page, selector) => page.evaluate((sel) => document.querySelector(sel).click(), selector);

async function openPage(viewport, seed, { clear = true } = {}) {
  const page = await browser.newPage();
  await page.setViewport({ ...viewport, deviceScaleFactor: 1 });
  const errors = [];
  page.on('pageerror', (e) => !/CWR|Cognito/.test(String(e)) && errors.push(String(e).slice(0, 200)));
  page.on('console', (m) => m.type() === 'error' && !/Failed to load resource|ERR_BLOCKED|net::|CWR|Cognito/.test(m.text()) && errors.push(m.text().slice(0, 200)));
  await page.setRequestInterception(true);
  page.on('request', (r) => (r.url().startsWith(base) || r.url().startsWith('data:') || r.url().startsWith('blob:') ? r.continue() : r.abort()));
  await page.evaluateOnNewDocument((key, seedJson, clearFirst) => {
    if (clearFirst && !sessionStorage.getItem('seeded')) {
      localStorage.removeItem(key);
      if (seedJson) localStorage.setItem(key, seedJson);
      sessionStorage.setItem('seeded', '1'); // seed once per tab, so a reload keeps what the page saved
    }
    // Keep each exported file, so the check can read it without a download dialog.
    window.__files = [];
    const create = URL.createObjectURL.bind(URL);
    URL.createObjectURL = (b) => {
      if (b instanceof Blob) window.__files.push(b);
      return create(b);
    };
  }, DRAFT_KEY, seed ? JSON.stringify(seed) : '', clear);
  await page.goto(`${base}/tools/infographic-maker`, { waitUntil: 'networkidle0' });
  await page.waitForSelector('[data-testid="artboard"]');
  await page.evaluate(() => document.fonts.ready);
  await new Promise((r) => setTimeout(r, 400));
  return { page, errors };
}

/** Template ids and limits, read from the page itself. */
async function readTemplates() {
  const { page } = await openPage({ width: 1400, height: 1000 }, null);
  const ids = await page.$$eval('input[name="template"]', (els) => els.map((e) => e.value));
  await page.close();
  return ids;
}

// The starting text of each template is read from a fresh page, so this script cannot drift from the registry.
async function readTemplate(id) {
  const { page } = await openPage({ width: 1400, height: 1000 }, null);
  await page.evaluate((v) => document.querySelector(`input[name="template"][value="${v}"]`).click(), id);
  await new Promise((r) => setTimeout(r, 200));
  // Switching keeps the words, so Start new is what gives the template's own starting text.
  await page.evaluate(() => document.getElementById('ig-new').click());
  await new Promise((r) => setTimeout(r, 200));
  await page.evaluate(() => [...document.querySelectorAll('.confirm button')].find((b) => b.textContent.includes('Start new')).click());
  await new Promise((r) => setTimeout(r, 300));
  const t = await page.evaluate(() => {
    const val = (sel) => document.querySelector(sel)?.value ?? '';
    const secs = [...document.querySelectorAll('[data-section]')];
    return {
      title: val('#ig-title'),
      subtitle: val('#ig-subtitle'),
      footer: val('#ig-footer'),
      defaults: secs.map((s) => ({
        label: s.querySelector('input[id^="ig-label-"]')?.value ?? '',
        heading: s.querySelector('input[id^="ig-heading-"]').value,
        body: s.querySelector('textarea')?.value ?? '',
      })),
      labelName: document.querySelector('label[for^="ig-label-"]')?.textContent.trim() ?? '',
      bullets: document.querySelector('label[for^="ig-body-"]')?.textContent.trim() === 'Bullets',
      // The statistics templates call the heading "Value" or "Number" and read a number out of it.
      chart: ['Value', 'Number'].includes(document.querySelector('label[for^="ig-heading-"]')?.textContent.trim()),
      noBody: !document.querySelector('label[for^="ig-body-"]'),
    };
  });
  // The limits, by using the buttons: add until it stops, then remove until it stops.
  const count = () => page.$$eval('[data-section]', (e) => e.length);
  for (let i = 0; i < 20 && !(await page.$eval('#ig-add', (b) => b.disabled)); i++) await pressButton(page, '#ig-add');
  const max = await count();
  for (let i = 0; i < 20; i++) {
    const disabled = await page.$eval('[aria-label="Remove section 1"]', (b) => b.disabled);
    if (disabled) break;
    await pressButton(page, '[aria-label="Remove section 1"]');
  }
  const min = await count();
  await page.close();
  return { id, ...t, min, max };
}

const ids = await readTemplates();
const templates = [];
for (const id of ids) {
  templates.push(await readTemplate(id));
}
console.log(`templates on the page: ${ids.join(', ')}`);

// --- every template, every size, three languages
const LANG_LIST = quick ? ['en', 'hi'] : ['en', 'hi', 'gu'];
const SIZE_LIST = quick ? ['portrait', 'landscape'] : Object.keys(SIZES);
for (const lang of LANG_LIST) {
  for (const tpl of templates) {
    for (const size of SIZE_LIST) {
      const id = `${lang}/${tpl.id}/${size}`;
      const seed = draft(tpl, size, lang);
      // A very tall viewport and a wide column, so the preview is shown at full size and can be compared pixel by pixel.
      const { page, errors } = await openPage({ width: 2600, height: 3200 }, seed);
      await page.addStyleTag({ content: '.wrap{width:2500px!important}.grid{grid-template-columns:minmax(0,1fr)!important}.preview{position:static!important}' });
      await new Promise((r) => setTimeout(r, 500));
      const info = await page.evaluate(() => {
        const a = document.querySelector('[data-testid="artboard"]');
        const r = a.getBoundingClientRect();
        return {
          w: Math.round(r.width), h: Math.round(r.height), k: getComputedStyle(a).getPropertyValue('--k').trim(),
          overflowWarning: !!document.querySelector('[data-testid="overflow"]'),
          scrollOk: a.scrollHeight <= a.clientHeight + 1 && a.scrollWidth <= a.clientWidth + 1,
          unsupported: !!document.querySelector('[data-testid="unsupported"]'),
          lang: a.getAttribute('lang'), dir: a.getAttribute('dir'),
        };
      });
      const [W, H] = SIZES[size];
      const entry = { id, ...info };
      if (info.w !== W || info.h !== H) fail(`${id}: preview is ${info.w}x${info.h}, expected ${W}x${H} (the full size was not reached)`);
      if (info.overflowWarning || !info.scrollOk) fail(`${id}: content does not fit (k ${info.k})`);
      if (info.unsupported) fail(`${id}: says fonts are missing for a released language`);
      const ref = await (await page.$('[data-testid="artboard"]')).screenshot({ type: 'png' });
      for (const [index, kind] of ['png', 'jpg', 'pdf'].entries()) {
        await pressButton(page, `[data-testid="export-${kind}"]`);
        await page.waitForFunction((n) => window.__files.length >= n && !document.querySelector('[aria-busy="true"]'), { timeout: 60000 }, index + 1);
      }
      const files = await page.evaluate(async () => {
        const out = [];
        for (const b of window.__files) {
          const buf = new Uint8Array(await b.arrayBuffer());
          let bin = '';
          for (let i = 0; i < buf.length; i += 0x8000) bin += String.fromCharCode(...buf.subarray(i, i + 0x8000));
          out.push({ type: b.type, bytes: buf.length, b64: btoa(bin) });
        }
        return out;
      });
      const message = await page.$eval('[data-testid="export-message"]', (e) => e.textContent.trim()).catch(() => '');
      const pngFile = files.find((f) => f.type === 'image/png');
      const jpgFile = files.find((f) => f.type === 'image/jpeg');
      const pdfFile = files.find((f) => f.type === 'application/pdf');
      if (!pngFile || !jpgFile || !pdfFile) {
        fail(`${id}: no PNG, JPG or PDF was made`);
      } else {
        const png = PNG.sync.read(Buffer.from(pngFile.b64, 'base64'));
        if (png.width !== W || png.height !== H) fail(`${id}: PNG is ${png.width}x${png.height}, expected ${W}x${H}`);
        const jpg = Buffer.from(jpgFile.b64, 'base64');
        let jw = 0, jh = 0;
        for (let i = 2; i < jpg.length - 8; ) {
          if (jpg[i] !== 0xff) { i++; continue; }
          const m = jpg[i + 1];
          if (m >= 0xc0 && m <= 0xcf && m !== 0xc4 && m !== 0xc8 && m !== 0xcc) { jh = jpg.readUInt16BE(i + 5); jw = jpg.readUInt16BE(i + 7); break; }
          i += 2 + jpg.readUInt16BE(i + 2);
        }
        if (jw !== W || jh !== H) fail(`${id}: JPG is ${jw}x${jh}, expected ${W}x${H}`);
        const refPng = PNG.sync.read(ref);
        if (refPng.width === png.width && refPng.height === png.height) {
          const n = pixelmatch(refPng.data, png.data, null, png.width, png.height, { threshold: 0.1, includeAA: false });
          entry.differingPercent = Math.round((n / (png.width * png.height)) * 100) / 100;
          if (entry.differingPercent > 2.5) fail(`${id}: preview and PNG differ in ${entry.differingPercent}% of pixels`);
        }
        entry.pngKB = Math.round(pngFile.bytes / 1024);
        entry.jpgKB = Math.round(jpgFile.bytes / 1024);
        // The JPG against the PNG: mean squared error over the colour channels, as PSNR.
        const psnr = await page.evaluate(async (pngB64, jpgB64) => {
          const load = async (b64, type) => {
            const img = new Image();
            img.src = `data:${type};base64,${b64}`;
            await img.decode();
            const c = document.createElement('canvas');
            c.width = img.naturalWidth; c.height = img.naturalHeight;
            const g = c.getContext('2d'); g.drawImage(img, 0, 0);
            return g.getImageData(0, 0, c.width, c.height).data;
          };
          const [a, b] = await Promise.all([load(pngB64, 'image/png'), load(jpgB64, 'image/jpeg')]);
          let s = 0, n = 0;
          for (let i = 0; i < a.length; i += 4) for (let k = 0; k < 3; k++) { const d = a[i + k] - b[i + k]; s += d * d; n++; }
          const mse = s / n;
          return mse === 0 ? 99 : 10 * Math.log10((255 * 255) / mse);
        }, pngFile.b64, jpgFile.b64);
        entry.psnr = Math.round(psnr * 10) / 10;
        if (psnr < 36) fail(`${id}: JPG quality is low (PSNR ${entry.psnr} dB)`);
        const pdf = checkPdf(Buffer.from(pdfFile.b64, 'base64'), W, H, png, id);
        entry.pdfKB = Math.round(pdfFile.bytes / 1024);
        entry.pdfPsnr = pdf.psnr;
        for (const problem of pdf.problems) fail(`${id}: PDF ${problem}`);
        if (size === 'portrait' || size === 'landscape') fs.writeFileSync(path.join(outDir, `${lang}-${tpl.id}-${size}.png`), Buffer.from(pngFile.b64, 'base64'));
      }
      if (/no font here/.test(message)) fail(`${id}: export says characters have no font: ${message}`);
      if (errors.length) fail(`${id}: page errors: ${errors.join(' | ')}`);
      results.cases.push(entry);
      console.log(`${id.padEnd(30)} k ${entry.k} diff ${entry.differingPercent ?? '-'}%  png ${entry.pngKB ?? '-'} kB  jpg ${entry.jpgKB ?? '-'} kB  psnr ${entry.psnr ?? '-'}  pdf ${entry.pdfKB ?? '-'} kB psnr ${entry.pdfPsnr ?? '-'}`);
      await page.close();
    }
  }
}

// --- long text: too much text must be warned about, never silently cut off
let longTextReacted = false;
for (const tpl of templates) {
  const { page } = await openPage({ width: 1400, height: 1600 }, draft(tpl, 'square', 'en', 'paper', true));
  await new Promise((r) => setTimeout(r, 500));
  const shown = await page.evaluate(() => {
    const a = document.querySelector('[data-testid="artboard"]');
    return { warn: !!document.querySelector('[data-testid="overflow"]'), shrunk: !!document.querySelector('[data-testid="shrunk"]'), cutOff: a.scrollHeight > a.clientHeight + 1 || a.scrollWidth > a.clientWidth + 1 };
  });
  results.interactions.push({ check: `long text, ${tpl.id}, square`, ...shown });
  longTextReacted ||= shown.warn || shown.shrunk;
  if (shown.cutOff && !shown.warn) fail(`long text in ${tpl.id}: content is cut off and the page does not say so`);
  await page.close();
}

if (!longTextReacted) fail('long text: no template shrank or warned, so the check proved nothing');

// --- the interface: keyboard, undo and redo, autosave, start new, clear draft, mirror
{
  const { page, errors } = await openPage({ width: 1400, height: 1000 }, null);
  const text = (sel) => page.$eval(sel, (e) => e.value);
  const ok = (name, cond, detail = '') => {
    results.interactions.push({ check: name, ok: !!cond, detail });
    if (!cond) fail(`${name} ${detail}`);
  };

  // typing, one undo step per field, redo
  await typeInto(page, '#ig-title', 'A brand new title');
  ok('typing changes the artboard', (await page.$eval('[data-testid="artboard"] h1', (e) => e.textContent)) === 'A brand new title');
  await page.evaluate(() => document.activeElement?.blur());
  await page.keyboard.down('Control'); await page.keyboard.press('KeyZ'); await page.keyboard.up('Control');
  await settle();
  ok('Ctrl+Z outside a field undoes the whole typing at once', (await page.$eval('[data-testid="artboard"] h1', (e) => e.textContent)) !== 'A brand new title');
  await page.keyboard.down('Control'); await page.keyboard.down('Shift'); await page.keyboard.press('KeyZ'); await page.keyboard.up('Shift'); await page.keyboard.up('Control');
  await settle();
  ok('Ctrl+Shift+Z redoes it', (await page.$eval('[data-testid="artboard"] h1', (e) => e.textContent)) === 'A brand new title');

  // sections: keyboard only
  const headings = () => page.$$eval('[data-section] input[id^="ig-heading-"]', (els) => els.map((e) => e.value));
  const before = await headings();
  await page.focus('[id^="ig-down-"]');
  await page.keyboard.press('Enter');
  await settle();
  const after = await headings();
  ok('Move down with the keyboard swaps the first two sections', after[0] === before[1] && after[1] === before[0], JSON.stringify([before[0], after[1]]));
  ok('focus stays on a Move button of the moved section', await page.evaluate(() => /^ig-(up|down)-/.test(document.activeElement?.id ?? '')), await page.evaluate(() => document.activeElement?.id));
  const count0 = (await headings()).length;
  await page.focus('[aria-label="Duplicate section 1"]');
  await page.keyboard.press('Enter');
  await settle();
  ok('Duplicate adds a copy after the section', (await headings()).length === count0 + 1 && (await headings())[0] === (await headings())[1]);
  ok('focus goes to the copy\'s heading', await page.evaluate(() => document.activeElement?.id?.startsWith('ig-heading-')));
  await page.focus('[aria-label="Remove section 2"]');
  await page.keyboard.press('Enter');
  await settle();
  ok('Remove takes the copy away', (await headings()).length === count0);
  await pressButton(page, '#ig-add');
  await settle();
  ok('Add section adds an empty one and focuses its heading', (await headings()).length === count0 + 1 && (await page.evaluate(() => document.activeElement?.id?.startsWith('ig-heading-'))));

  // design: template, size, colours, mark, reset
  await page.evaluate(() => document.querySelector('input[name="theme"][value="ink"]').click());
  await settle();
  await page.evaluate(() => document.querySelector('input[name="format"][value="landscape"]').click());
  await settle();
  await page.evaluate(() => document.querySelector('[role="switch"]').click());
  await settle();
  const design = await page.evaluate(() => {
    const a = document.querySelector('[data-testid="artboard"]');
    return { cls: a.className, bg: getComputedStyle(a).backgroundColor, mark: !!a.querySelector('.mark') };
  });
  ok('size, colours and the mark apply', design.cls.includes('f-landscape') && design.bg === 'rgb(20, 17, 15)' && design.mark, JSON.stringify(design));
  await page.evaluate(() => [...document.querySelectorAll('button')].find((b) => b.textContent.includes('Reset design')).click());
  await settle();
  const reset = await page.evaluate(() => {
    const a = document.querySelector('[data-testid="artboard"]');
    return { cls: a.className, bg: getComputedStyle(a).backgroundColor, mark: !!a.querySelector('.mark'), title: a.querySelector('h1').textContent };
  });
  ok('Reset design restores the look and keeps the words', reset.cls.includes('f-portrait') && reset.bg === 'rgb(251, 246, 238)' && !reset.mark && reset.title === 'A brand new title', JSON.stringify(reset));

  // autosave: the draft survives a reload, and can be cleared
  await new Promise((r) => setTimeout(r, 900));
  ok('a draft was saved in local storage', await page.evaluate((k) => !!localStorage.getItem(k), DRAFT_KEY));
  await page.reload({ waitUntil: 'networkidle0' });
  await page.waitForSelector('[data-testid="artboard"]');
  await settle(400);
  ok('the draft comes back after a reload, with a notice', (await page.$eval('[data-testid="artboard"] h1', (e) => e.textContent)) === 'A brand new title' && !!(await page.$('[data-testid="restored"]')));
  await page.evaluate(() => [...document.querySelectorAll('button')].find((b) => b.textContent.includes('Clear saved draft')).click());
  await settle();
  ok('Clear saved draft removes it from local storage', await page.evaluate((k) => localStorage.getItem(k) === null, DRAFT_KEY));

  // start new: asks first, with the safe choice focused
  await pressButton(page, '#ig-new');
  await settle();
  ok('Start new asks first and focuses Keep editing', await page.evaluate(() => document.activeElement?.id === 'ig-keep'));
  await page.keyboard.press('Enter');
  await settle();
  ok('Keep editing leaves the work alone', (await page.$eval('[data-testid="artboard"] h1', (e) => e.textContent)) === 'A brand new title');
  await pressButton(page, '#ig-new');
  await settle();
  await page.evaluate(() => [...document.querySelectorAll('.confirm button')].find((b) => b.textContent.includes('Start new')).click());
  await settle();
  ok('Start new replaces the work', (await page.$eval('[data-testid="artboard"] h1', (e) => e.textContent)) !== 'A brand new title');
  await page.evaluate(() => document.activeElement?.blur());
  await page.keyboard.down('Control'); await page.keyboard.press('KeyZ'); await page.keyboard.up('Control');
  await settle();
  ok('and Undo brings it back', (await page.$eval('[data-testid="artboard"] h1', (e) => e.textContent)) === 'A brand new title');

  // keyboard reach: every control can be tabbed to, in a sensible order, with a visible focus ring somewhere
  const stops = [];
  // Start the walk at the top of the main content, as a person arriving on the page would.
  await page.evaluate(() => { window.scrollTo(0, 0); const m = document.getElementById('main'); m.setAttribute('tabindex', '-1'); m.focus(); });
  for (let i = 0; i < 110; i++) {
    await page.keyboard.press('Tab');
    stops.push(await page.evaluate(() => {
      const e = document.activeElement;
      const cs = e ? getComputedStyle(e) : null;
      const r = e?.getBoundingClientRect();
      return { tag: e?.tagName, id: e?.id || '', name: (e?.getAttribute('aria-label') || e?.textContent || '').trim().slice(0, 30), h: Math.round(r?.height ?? 0), w: Math.round(r?.width ?? 0), outline: cs?.outlineStyle };
    }));
  }
  const reached = (re) => stops.some((s) => re.test(s.name) || re.test(s.id));
  const wanted = { Redo: /Redo/, 'Reset design': /Reset design/, title: /ig-title/, 'Download PNG': /Download PNG/, 'Add section': /ig-add|Add section/, 'Move up': /Move section 2 up/, Duplicate: /Duplicate section 1/, Remove: /Remove section 1/ };
  const missing = Object.entries(wanted).filter(([, re]) => !reached(re)).map(([k]) => k);
  ok('Tab reaches Redo (Undo is disabled: nothing is left to undo after the reload), Reset design, the title, Download PNG, Add section and each section button', missing.length === 0, `missing: ${missing.join(', ')}; ${stops.length} stops, e.g. ${stops.slice(0, 40).map((x) => x.name || x.id || x.tag).join(' > ')}`);
  // A radio is a 1 px box hidden under its tile, which is the target. Everything else must be big enough to press.
  ok('a button with nothing to do is disabled, so Tab skips it', await page.evaluate(() => [...document.querySelectorAll('button')].find((b) => b.textContent.trim() === 'Undo').disabled));
  const small = stops.filter((s) => ['BUTTON', 'INPUT', 'SELECT', 'TEXTAREA'].includes(s.tag) && s.h > 1 && s.h < 40);
  ok('every control reached by Tab is at least 40 px tall', small.length === 0, JSON.stringify(small.slice(0, 3)));

  // the page itself
  const privacy = await page.$eval('[data-testid="privacy"]', (e) => e.textContent.replace(/\s+/g, ' '));
  ok('the page says it runs in the browser and where the draft lives', /never uploaded/.test(privacy) && /local storage/.test(privacy), privacy.slice(0, 80));
  ok('no request left the machine (everything else was blocked)', errors.length === 0, errors.join(' | '));
  await page.close();
}

// --- right to left: the whole template mirrors
for (const tpl of templates) {
  const { page } = await openPage({ width: 1400, height: 1400 }, draft(tpl, 'portrait', 'ar'));
  await new Promise((r) => setTimeout(r, 400));
  const m = await page.evaluate(() => {
    const a = document.querySelector('[data-testid="artboard"]');
    const ab = a.getBoundingClientRect();
    const q = (sel) => a.querySelector(sel);
    // Where the text of a heading really starts, not the width of its box.
    const box = (el) => {
      if (!el) return null;
      if (!el.matches('h2, h1')) return el.getBoundingClientRect();
      const r = document.createRange();
      r.selectNodeContents(el);
      return r.getBoundingClientRect();
    };
    const nodes = [...a.querySelectorAll('.node')];
    const first = nodes.length > 1 ? nodes[1] : q('.num') ?? q('.badge') ?? q('.check') ?? q('.pill') ?? q('.when') ?? q('.sym') ?? q('.band h2') ?? q('.band') ?? q('.card') ?? q('.rh h2') ?? q('.xs span') ?? q('.sw') ?? q('.nc');
    const fr = box(first);
    const cycle = nodes.length > 1;
    return { dir: a.getAttribute('dir'), lang: a.getAttribute('lang'), titleAlign: getComputedStyle(a.querySelector('h1')).textAlign, firstCenter: fr ? (fr.left + fr.width / 2 - ab.left) / ab.width : null, cycle };
  });
  const rtl = m.dir === 'rtl';
  const side = tpl.id === 'compare' ? true : m.cycle ? m.firstCenter < 0.5 : m.firstCenter > 0.5; // number badges, dots and bands start on the right
  results.interactions.push({ check: `right to left, ${tpl.id}`, ...m, ok: rtl && side });
  if (!rtl) fail(`right to left ${tpl.id}: dir is ${m.dir}`);
  else if (!side) fail(`right to left ${tpl.id}: first element is at ${m.firstCenter}, not on the right`);
  await page.screenshot({ path: path.join(outDir, `ar-${tpl.id}-page.png`) });
  await page.close();
}

// --- screenshots for a look: the whole page on a desktop and on a phone, light and dark
for (const [name, vp] of [['desktop', { width: 1366, height: 900 }], ['phone', { width: 375, height: 812 }]]) {
  const tpl = templates[0];
  for (const theme of ['paper', 'ink']) {
    const { page } = await openPage(vp, draft(tpl, 'portrait', 'hi', theme));
    await page.evaluate((t) => localStorage.setItem('app-theme', t === 'ink' ? 'dark' : 'light'), theme);
    await page.screenshot({ path: path.join(outDir, `page-${name}-${theme}.png`), fullPage: name === 'phone' });
    const overflowX = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
    results.interactions.push({ check: `no sideways scroll, ${name}`, ok: !overflowX });
    if (overflowX) fail(`${name}: the page scrolls sideways`);
    await page.close();
  }
}

// --- icons and shapes: add, drag, keys, undo, export without the ring, PDF, reset, restore
{
  const tpl = templates.find((t) => t.id === 'key-points');
  const { page, errors } = await openPage({ width: 1400, height: 1500 }, draft(tpl, 'portrait', 'en'));
  const ok = (name, cond, detail = '') => {
    results.interactions.push({ check: name, ok: !!cond, detail });
    if (!cond) fail(`${name} ${detail}`);
  };
  const items = () => page.$$eval('[data-testid="deco"]', (els) => els.map((e) => ({ id: e.dataset.id, x: parseFloat(e.style.left), y: parseFloat(e.style.top), w: parseFloat(e.style.width), label: e.getAttribute('aria-label') })));
  const art = () => page.$eval('[data-testid="artboard"]', (e) => { const r = e.getBoundingClientRect(); return { w: r.width, h: r.height }; });
  const center = (i) => page.$$eval('[data-testid="deco"]', (els, k) => { const r = els[k].getBoundingClientRect(); return { x: r.x + r.width / 2, y: r.y + r.height / 2 }; }, i);
  const drag = async (i, dx, dy) => {
    const c = await center(i);
    await page.mouse.move(c.x, c.y);
    await page.mouse.down();
    await page.mouse.move(c.x + dx, c.y + dy, { steps: 10 });
    await page.mouse.up();
    await settle();
  };
  const undo = async () => {
    await page.evaluate(() => document.activeElement?.blur());
    await page.keyboard.down('Control'); await page.keyboard.press('KeyZ'); await page.keyboard.up('Control');
    await settle();
  };
  const near = (a, b, tol = 0.35) => Math.abs(a - b) <= tol;
  const exportOf = async (kind) => {
    const before = await page.evaluate(() => window.__files.length);
    await pressButton(page, `[data-testid="export-${kind}"]`);
    await page.waitForFunction((n) => window.__files.length > n && !document.querySelector('[aria-busy="true"]'), { timeout: 60000 }, before);
    const b64 = await page.evaluate(async () => {
      const b = window.__files[window.__files.length - 1];
      const buf = new Uint8Array(await b.arrayBuffer());
      let bin = '';
      for (let i = 0; i < buf.length; i += 0x8000) bin += String.fromCharCode(...buf.subarray(i, i + 0x8000));
      return btoa(bin);
    });
    return Buffer.from(b64, 'base64');
  };

  await pressButton(page, '[aria-label="Add light bulb icon"]');
  await pressButton(page, '[aria-label="Add circle"]');
  await settle();
  let list = await items();
  ok('adding an icon and a shape puts both on the artboard', list.length === 2, JSON.stringify(list));
  ok('each one has a spoken name that says how to move it', list.every((l) => /arrow keys/.test(l.label)) && /Light bulb/.test(list[0].label) && /Circle/.test(list[1].label));
  ok('the panel lists them with number boxes', (await page.$$('input[id^="ig-x-"]')).length === 2);

  // dragging
  const a = await art();
  const start = list[0];
  await drag(0, 60, 40);
  let now = (await items())[0];
  ok('dragging moves the icon by the distance dragged, in artboard percent', near(now.x, start.x + (60 / a.w) * 100) && near(now.y, start.y + (40 / a.h) * 100), `${JSON.stringify(start)} -> ${JSON.stringify(now)}`);
  ok('the panel shows the new place', near(Number(await page.$eval(`#ig-x-${now.id}`, (e) => e.value)), now.x, 0.1));
  await drag(0, 4000, -4000);
  now = (await items())[0];
  ok('dragging far outside stops at the edge of the artboard', now.x === 100 && now.y === 0, JSON.stringify(now));
  await undo();
  now = (await items())[0];
  ok('one drag is one undo step, even when two drags follow each other closely', near(now.x, start.x + (60 / a.w) * 100) && near(now.y, start.y + (40 / a.h) * 100), JSON.stringify(now));
  await undo();
  now = (await items())[0];
  ok('another undo puts the icon back where it was added', near(now.x, start.x) && near(now.y, start.y), JSON.stringify(now));

  // keys
  await page.focus('[data-testid="deco"]');
  const k0 = (await items())[0];
  for (let i = 0; i < 3; i++) await page.keyboard.press('ArrowRight');
  await page.keyboard.down('Shift'); await page.keyboard.press('ArrowDown'); await page.keyboard.up('Shift');
  await settle();
  const k1 = (await items())[0];
  ok('arrow keys move 1 percent, and 5 with Shift', near(k1.x, k0.x + 3, 0.15) && near(k1.y, k0.y + 5, 0.15), `${JSON.stringify(k0)} -> ${JSON.stringify(k1)}`);
  ok('a focused icon shows a ring', await page.$eval('[data-testid="deco"]', (e) => getComputedStyle(e.querySelector('.ring')).display !== 'none'));
  await page.keyboard.press('Delete');
  await settle();
  ok('Delete removes the focused icon, and says so', (await items()).length === 1 && /Removed/.test(await page.$eval('[data-testid="announcement"]', (e) => e.textContent)));
  await undo();
  ok('undo brings it back', (await items()).length === 2);

  // size, colour and place from the panel
  const id0 = (await items())[0].id;
  await page.$eval(`#ig-size-${id0}`, (e) => { e.value = '30'; e.dispatchEvent(new Event('input', { bubbles: true })); });
  await settle();
  ok('the size slider resizes the icon', near((await items())[0].w, 30, 0.01));
  await page.select(`#ig-color-${id0}`, 'ink2');
  await settle();
  ok('the colour list recolours it with a colour of the theme', await page.$eval('[data-testid="deco"]', (e) => getComputedStyle(e).color) === 'rgb(74, 66, 56)');
  await typeInto(page, `#ig-x-${id0}`, '50');
  await typeInto(page, `#ig-y-${id0}`, '84');
  const placed = (await items())[0];
  ok('the number boxes place the icon', placed.x === 50 && placed.y === 84, JSON.stringify(placed));

  // export: the ring is not in the picture, the icon is, and the PDF holds it too
  await page.focus('[data-testid="deco"]');
  const withRing = await exportOf('png');
  await page.evaluate(() => document.activeElement?.blur());
  await settle(700);
  await page.reload({ waitUntil: 'networkidle0' });
  await page.waitForSelector('[data-testid="artboard"]');
  await page.evaluate(() => document.fonts.ready);
  await settle(500);
  ok('the draft keeps icons and shapes across a reload', (await items()).length === 2);
  const clean = await exportOf('png');
  const A = PNG.sync.read(withRing);
  const B = PNG.sync.read(clean);
  const diffPx = pixelmatch(A.data, B.data, null, A.width, A.height, { threshold: 0.1, includeAA: false });
  ok('the ring round a selected icon is not in the exported picture', A.width === B.width && diffPx / (A.width * A.height) < 0.0005, `${diffPx} pixels differ`);
  fs.writeFileSync(path.join(outDir, 'decor-export.png'), clean);
  await (await page.$('[data-testid="artboard"]')).screenshot({ path: path.join(outDir, 'decor-preview.png') });
  const it = (await items())[0];
  const pdf = await exportOf('pdf');
  const pdfCheck = checkPdf(pdf, 1080, 1920, B, 'icons');
  ok('the PDF with icons passes pdfinfo, qpdf, pdfimages and both renderers', pdfCheck.problems.length === 0, pdfCheck.problems.join('; '));
  // Take the icon away and export again: the pictures must differ inside the icon's box, and only there.
  await pressButton(page, '[aria-label="Remove light bulb 1"]');
  await settle();
  const without = PNG.sync.read(await exportOf('png'));
  const cx = Math.round((it.x / 100) * B.width);
  const cy = Math.round((it.y / 100) * B.height);
  const half = Math.round((it.w / 100) * B.width * 0.6);
  let inside = 0;
  let outside = 0;
  for (let yy = 0; yy < B.height; yy++) for (let xx = 0; xx < B.width; xx++) {
    const o = (yy * B.width + xx) * 4;
    if (Math.abs(B.data[o] - without.data[o]) + Math.abs(B.data[o + 1] - without.data[o + 1]) + Math.abs(B.data[o + 2] - without.data[o + 2]) > 60) {
      if (Math.abs(xx - cx) <= half && Math.abs(yy - cy) <= half) inside++;
      else outside++;
    }
  }
  ok('the icon is drawn in the exported picture, inside its own box', inside > 300, `${inside} pixels differ inside the icon's box`);
  ok('and nowhere else changes when it is removed', outside < 50, `${outside} pixels differ outside it`);
  await undo();

  // reset, and a draft with a damaged item
  await page.evaluate(() => [...document.querySelectorAll('button')].find((b) => /Reset design/.test(b.textContent)).click());
  await settle();
  ok('Reset design removes the icons and shapes and says so', (await items()).length === 0 && /Icons and shapes were removed/.test(await page.$eval('[data-testid="announcement"]', (e) => e.textContent)));
  await undo();
  ok('undo brings them back', (await items()).length === 2);
  ok('no page errors', errors.length === 0, errors.join(' | '));
  await page.close();

  // a saved draft with damaged items opens without error
  const bad = draft(tpl, 'portrait', 'en');
  bad.items = [{ id: '<x>', glyph: 'lightbulb', x: 'a', y: 500, size: 9000, color: 'pink' }, { id: 'b', glyph: 'nope', x: 1, y: 1, size: 5, color: 'ink' }, null, 5];
  const second = await openPage({ width: 1400, height: 1500 }, bad);
  const dec = await second.page.$$eval('[data-testid="deco"]', (els) => els.map((e) => ({ x: parseFloat(e.style.left), y: parseFloat(e.style.top), w: parseFloat(e.style.width), id: e.dataset.id })));
  ok('a damaged saved item is repaired and an unknown one is dropped', dec.length === 1 && dec[0].x === 50 && dec[0].y === 100 && dec[0].w === 60 && /^[\w-]+$/.test(dec[0].id), JSON.stringify(dec));
  await second.page.close();
}
await browser.close();
server.close();
fs.writeFileSync(path.join(outDir, 'results.json'), JSON.stringify(results, null, 2));
console.log(`\n${results.cases.length} renders, ${results.interactions.length} checks. Results and screenshots: ${outDir}`);
if (results.failures.length) {
  console.log(`\nFAILED (${results.failures.length}):\n  ${results.failures.join('\n  ')}`);
  process.exitCode = 1;
} else {
  console.log('\nAll checks passed.');
}
