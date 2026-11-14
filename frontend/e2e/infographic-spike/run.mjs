// Drives the #217 spike in headless Chrome and writes what it finds.
//   cd frontend/e2e/infographic-spike && npm ci && node run.mjs [outDir]
// Needs the app's own node_modules for puppeteer and esbuild (../../node_modules).
// Writes screenshots and results.json to outDir (default ~/shots303/spike), never into the repo.
import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import zlib from 'node:zlib';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import pixelmatch from 'pixelmatch';
import { PNG } from 'pngjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const appRequire = createRequire(path.join(here, '../../') );
const puppeteer = appRequire('puppeteer');
const esbuild = appRequire('esbuild');

const outDir = path.resolve(process.argv[2] ?? path.join(process.env.HOME, 'shots303/spike'));
fs.mkdirSync(outDir, { recursive: true });
const buildDir = path.join(here, '.build');
fs.mkdirSync(buildDir, { recursive: true });

// --- Build the browser side, and measure how big the tool's code would be.
const common = { entryPoints: [path.join(here, 'harness.ts')], bundle: true, format: 'iife', target: 'es2022', logLevel: 'error' };
await esbuild.build({ ...common, outfile: path.join(buildDir, 'harness.js') });
const min = await esbuild.build({ ...common, minify: true, write: false, outfile: 'x.js' });
const minBytes = min.outputFiles[0].contents.length;
const gzBytes = zlib.gzipSync(min.outputFiles[0].contents).length;

// --- A static server: the harness page, its bundle, and the font packages.
const fontRoot = path.join(here, 'node_modules/@fontsource');
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.woff2': 'font/woff2', '.woff': 'font/woff' };
const fontRequests = [];
const server = http.createServer((req, res) => {
  const urlPath = decodeURIComponent(req.url.split('?')[0]);
  let file;
  if (urlPath === '/') file = path.join(here, 'harness.html');
  else if (urlPath === '/harness.js') file = path.join(buildDir, 'harness.js');
  else if (urlPath.startsWith('/fonts/')) {
    file = path.join(fontRoot, urlPath.slice('/fonts/'.length));
    if (!file.startsWith(fontRoot)) file = null;
    else fontRequests.push(urlPath);
  }
  if (!file || !fs.existsSync(file)) {
    res.writeHead(404);
    return res.end();
  }
  res.writeHead(200, { 'content-type': mime[path.extname(file)] ?? 'application/octet-stream', 'access-control-allow-origin': '*' });
  fs.createReadStream(file).pipe(res);
});
await new Promise((r) => server.listen(0, '127.0.0.1', r));
const base = `http://127.0.0.1:${server.address().port}/`;

const SIZES = { portrait: [1080, 1920], square: [1080, 1080], landscape: [1920, 1080] };
const LANGS = ['en', 'hi', 'ar', 'zh'];

const browser = await puppeteer.launch({ executablePath: '/usr/bin/google-chrome', headless: 'new', args: ['--no-sandbox', '--disable-gpu'] });
const results = { cases: [], scaled: [], naive: [], bundle: { minBytes, gzBytes }, chrome: await browser.version() };

async function open(lang, size, scale = 1, long = false) {
  const [w, h] = SIZES[size];
  const page = await browser.newPage();
  page.setDefaultTimeout(180000);
  await page.setViewport({ width: w, height: h, deviceScaleFactor: 1 });
  await page.goto(base, { waitUntil: 'load' });
  await page.evaluate(() => window.spike.init());
  await page.evaluate((l, s, sc, lg) => window.spike.show(l, s, sc, lg), lang, size, scale, long);
  const ready = await page.evaluate(() => window.spike.ready());
  return { page, ready };
}

function compare(refPng, exportB64) {
  const a = PNG.sync.read(refPng);
  const b = PNG.sync.read(Buffer.from(exportB64, 'base64'));
  if (a.width !== b.width || a.height !== b.height) return { sameSize: false, ref: [a.width, a.height], out: [b.width, b.height] };
  const diff = new PNG({ width: a.width, height: a.height });
  const n = pixelmatch(a.data, b.data, diff.data, a.width, a.height, { threshold: 0.1, includeAA: false });
  return { sameSize: true, differingPixels: n, differingPercent: Math.round((n / (a.width * a.height)) * 10000) / 100, diff: PNG.sync.write(diff) };
}

const refs = new Map();
for (const lang of LANGS) {
  for (const size of Object.keys(SIZES)) {
    const { page, ready } = await open(lang, size);
    const ref = await (await page.$('#stage')).screenshot({ type: 'png' });
    refs.set(`${lang}-${size}`, ref);
    const ours = await page.evaluate(() => window.spike.exportOurs());
    const cmp = compare(ref, ours.png.b64);
    fs.writeFileSync(path.join(outDir, `${lang}-${size}-export.png`), Buffer.from(ours.png.b64, 'base64'));
    fs.writeFileSync(path.join(outDir, `${lang}-${size}-preview.png`), ref);
    if (cmp.diff) fs.writeFileSync(path.join(outDir, `${lang}-${size}-diff.png`), cmp.diff);
    delete cmp.diff;
    delete ours.png.b64;
    results.cases.push({ lang, size, overflow: ready.overflow, ...ours, compare: cmp });
    console.log(`${lang.padEnd(2)} ${size.padEnd(9)} files ${String(ours.embed.count).padStart(2)}  font ${(ours.embed.woff2Bytes / 1024).toFixed(0).padStart(4)} kB  embed ${String(ours.embed.ms).padStart(4)} ms  png ${String(ours.png.ms).padStart(5)} ms ${(ours.png.bytes / 1024).toFixed(0).padStart(5)} kB ${ours.png.size.join('x')}  jpg ${(ours.jpg.bytes / 1024).toFixed(0).padStart(4)} kB psnr ${ours.jpg.psnr}  diff ${cmp.differingPercent}%  overflow ${ready.overflow.length}  uncovered ${ours.embed.uncovered.length}`);
    await page.close();
  }
}

// Too much text must be noticed, not silently clipped: eight times the text per step.
results.longText = [];
for (const lang of LANGS) {
  for (const size of Object.keys(SIZES)) {
    const { page, ready } = await open(lang, size, 1, true);
    results.longText.push({ lang, size, overflow: ready.overflow });
    console.log(`long text ${lang} ${size}: ${ready.overflow.length ? 'noticed (' + ready.overflow[0] + ')' : 'NOT NOTICED'}`);
    await page.close();
  }
}

// The preview is shown scaled to fit the screen; the export must still be full size and the same picture.
for (const lang of LANGS) {
  for (const size of ['portrait', 'landscape']) {
    const { page } = await open(lang, size, 0.3);
    const ours = await page.evaluate(() => window.spike.exportOurs());
    const cmp = compare(refs.get(`${lang}-${size}`), ours.png.b64);
    delete cmp.diff;
    results.scaled.push({ lang, size, scale: 0.3, png: { size: ours.png.size, bytes: ours.png.bytes }, compare: cmp });
    console.log(`scaled 0.3 ${lang} ${size}: ${ours.png.size.join('x')} diff ${cmp.differingPercent}%`);
    await page.close();
  }
}

// What html-to-image does by itself, with no font slicing.
for (const lang of ['en', 'hi', 'zh']) {
  const { page } = await open(lang, 'portrait');
  fontRequests.length = 0;
  let outcome;
  try {
    outcome = await page.evaluate(() => window.spike.exportNaive());
  } catch (e) {
    outcome = { error: String(e).slice(0, 200) };
  }
  const woff2 = fontRequests.filter((u) => u.endsWith('.woff2')).length;
  results.naive.push({ lang, ...outcome, woff2Requests: woff2 });
  console.log(`naive ${lang}: ${JSON.stringify(outcome)} woff2 requests during export: ${woff2}`);
  await page.close();
}

await browser.close();
server.close();

// --- What must hold. The run fails if any of it does not.
const failures = [];
const MAX_DIFF_PERCENT = 2; // preview vs export: edge anti-aliasing only; see RESULTS.md
const EXPECTED_UNCOVERED = { zh: ['U+9F18'] }; // a rare character on purpose: it has no slice in Noto Sans SC
for (const c of results.cases) {
  const [w, h] = SIZES[c.size];
  const id = `${c.lang} ${c.size}`;
  if (c.png.size.join('x') !== `${w}x${h}`) failures.push(`${id}: PNG is ${c.png.size.join('x')}, expected ${w}x${h}`);
  if (c.jpg.size.join('x') !== `${w}x${h}`) failures.push(`${id}: JPG is ${c.jpg.size.join('x')}, expected ${w}x${h}`);
  if (c.jpg.psnr < 38) failures.push(`${id}: JPG quality is poor (PSNR ${c.jpg.psnr} dB)`);
  if (c.overflow.length) failures.push(`${id}: text overflows: ${c.overflow.join('; ')}`);
  if (!c.compare.sameSize) failures.push(`${id}: preview and export differ in size`);
  else if (c.compare.differingPercent > MAX_DIFF_PERCENT) failures.push(`${id}: preview and export differ in ${c.compare.differingPercent}% of pixels`);
  const want = (EXPECTED_UNCOVERED[c.lang] ?? []).join();
  if (c.embed.uncovered.join() !== want) failures.push(`${id}: characters with no font ${c.embed.uncovered.join() || '(none)'}, expected ${want || '(none)'}`);
}
for (const s of results.scaled) {
  const [w, h] = SIZES[s.size];
  if (s.png.size.join('x') !== `${w}x${h}`) failures.push(`scaled ${s.lang} ${s.size}: export is ${s.png.size.join('x')}, not full size`);
  const base = results.cases.find((c) => c.lang === s.lang && c.size === s.size);
  if (!s.compare.sameSize || s.compare.differingPixels !== base.compare.differingPixels) failures.push(`scaled ${s.lang} ${s.size}: differs from the unscaled export`);
}
for (const l of results.longText) if (!l.overflow.length) failures.push(`long text ${l.lang} ${l.size}: overflow was not noticed`);
results.failures = failures;
fs.writeFileSync(path.join(outDir, 'results.json'), JSON.stringify(results, null, 2));
console.log(`\nbundle (harness: html-to-image + chart.js bar + font-slices): ${(minBytes / 1024).toFixed(0)} kB minified, ${(gzBytes / 1024).toFixed(0)} kB gzip`);
console.log(`results and screenshots: ${outDir}`);
if (failures.length) {
  console.log('\nFAILED:\n  ' + failures.join('\n  '));
  process.exitCode = 1;
} else {
  console.log('\nAll checks passed.');
}
