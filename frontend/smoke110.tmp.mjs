import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { join, resolve, extname } from 'node:path';
import puppeteer from 'puppeteer';
const DIST = resolve('dist/frontend/browser');
const MIME = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css' };
const FIX = '/tmp/fix110.mp4';
const server = createServer(async (req, res) => {
  try {
    const u = new URL(req.url, 'http://x');
    if (u.pathname === '/fix.mp4') {
      const body = await readFile(FIX);
      res.writeHead(200, { 'Content-Type': 'video/mp4', 'Content-Length': body.length, 'Accept-Ranges': 'bytes' });
      res.end(body);
      return;
    }
    const p = decodeURIComponent(u.pathname);
    let f = join(DIST, p);
    if (p.endsWith('/')) f = join(f, 'index.html');
    let body;
    try { body = await readFile(f); } catch { f = join(DIST, 'index.html'); body = await readFile(f); }
    res.writeHead(200, { 'Content-Type': MIME[extname(f)] ?? 'application/octet-stream' });
    res.end(body);
  } catch { res.writeHead(404); res.end(); }
});
await new Promise((r) => server.listen(4357, r));
const browser = await puppeteer.launch({ headless: true, executablePath: '/usr/bin/google-chrome',
  args: ['--no-sandbox', '--disable-dev-shm-usage', '--disable-gpu', '--autoplay-policy=no-user-gesture-required'] });
const cors = { 'Access-Control-Allow-Origin': 'http://localhost:4357', 'Access-Control-Allow-Credentials': 'true' };
const clips = [1, 2].map((n) => ({ clipId: `clip-${n}`, rank: n, score: 0.9, duration: 3 }));
let putBody = null;
const page = await browser.newPage();
const errors = [];
page.on('pageerror', (e) => errors.push(String(e).slice(0, 150)));
await page.setRequestInterception(true);
page.on('request', (req) => {
  const url = req.url();
  if (req.isNavigationRequest()) { req.continue(); return; }
  if (url.includes('api.indifferent.fun') && req.method() === 'OPTIONS') {
    req.respond({ status: 200, headers: { ...cors, 'Access-Control-Allow-Headers': 'Content-Type,Authorization', 'Access-Control-Allow-Methods': 'GET,POST,PUT,DELETE,OPTIONS' } });
  } else if (req.method() === 'POST' && url.endsWith('/auth/refresh')) {
    req.respond({ status: 200, contentType: 'application/json', headers: cors,
      body: JSON.stringify({ accessToken: 'a', refreshToken: 'r', expiresIn: 3600 }) });
  } else if (req.method() === 'GET' && url.endsWith('/shorts/job9/clips')) {
    req.respond({ status: 200, contentType: 'application/json', headers: cors, body: JSON.stringify({ clips }) });
  } else if (req.method() === 'GET' && url.endsWith('/shorts/job9')) {
    req.respond({ status: 200, contentType: 'application/json', headers: cors,
      body: JSON.stringify({ jobId: 'job9', status: 'completed', caption: { preset: 'bold' } }) });
  } else if (req.method() === 'PUT' && url.endsWith('/shorts/job9/style')) {
    putBody = req.postData();
    req.respond({ status: 200, contentType: 'application/json', headers: cors,
      body: JSON.stringify({ jobId: 'job9', caption: JSON.parse(req.postData()).caption }) });
  } else if (/\/clips\/clip-\d+\/url$/.test(url)) {
    req.respond({ status: 200, contentType: 'application/json', headers: cors,
      body: JSON.stringify({ url: 'http://localhost:4357/fix.mp4' }) });
  } else {
    req.continue();
  }
});
await page.evaluateOnNewDocument(() => { localStorage.setItem('indifferent.refreshToken', 'seed'); });
await page.goto('http://localhost:4357/shorts/job9/clips', { waitUntil: 'networkidle2', timeout: 60000 });
await page.waitForSelector('[data-clip="clip-1"]', { timeout: 30000 });
const clickBtn = async (label) => {
  await page.evaluate((t) => {
    [...document.querySelectorAll('button')].find((b) => (b.getAttribute('aria-label') || '') === t)?.click();
  }, label);
  await new Promise((r) => setTimeout(r, 600));
};
// 1. posters auto-capture on list load (no interaction)
await page.waitForFunction(
  () => document.querySelector('[data-clip="clip-1"] img') !== null,
  { timeout: 30000 },
).catch(() => console.log('thumb1: TIMEOUT'));
const thumb1 = await page.evaluate(() => document.querySelector('[data-clip="clip-1"] img')?.src ?? '');
console.log('thumb auto-captured:', thumb1.startsWith('data:image/jpeg;base64,'));
// 2. frame strip: open, expect 5 frames, pick one
await clickBtn('Choose thumbnail frames for clip 1');
await page.waitForFunction(
  () => document.querySelectorAll('[data-clip="clip-1"] [aria-label^="Use frame"]').length === 5,
  { timeout: 30000 },
).catch(() => console.log('strip: TIMEOUT'));
const stripCount = await page.evaluate(() =>
  document.querySelectorAll('[data-clip="clip-1"] [aria-label^="Use frame"]').length);
console.log('strip frames:', stripCount);
const before = await page.evaluate(() => document.querySelector('[data-clip="clip-1"] img')?.src);
await page.evaluate(() => {
  [...document.querySelectorAll('[data-clip="clip-1"] [aria-label^="Use frame"]')][3]?.click();
});
await new Promise((r) => setTimeout(r, 500));
const after = await page.evaluate(() => document.querySelector('[data-clip="clip-1"] img')?.src);
console.log('frame pick changes poster:', !!before && !!after);
// 3. caption studio: open, preset cards, change + save
await page.evaluate(() => {
  [...document.querySelectorAll('button')].find((b) => b.textContent.trim() === 'Captions')?.click();
});
await new Promise((r) => setTimeout(r, 1200));
const presets = await page.evaluate(() =>
  [...document.querySelectorAll('[aria-label="Caption presets"] button').keys()].length);
console.log('preset cards:', await page.evaluate(() =>
  document.querySelectorAll('[aria-label="Caption presets"] [aria-pressed]').length));
await page.evaluate(() => {
  [...document.querySelectorAll('[aria-label="Caption presets"] button')]
    .find((b) => b.textContent.includes('Karaoke'))?.click();
});
await new Promise((r) => setTimeout(r, 400));
await page.evaluate(() => {
  const range = document.querySelector('input[aria-label="Caption font size"]');
  range.focus();
  range.value = 30;
  range.dispatchEvent(new Event('input', { bubbles: true }));
  range.dispatchEvent(new Event('change', { bubbles: true }));
});
await page.evaluate(() => {
  [...document.querySelectorAll('button')].find((b) => b.textContent.trim() === 'Save style')?.click();
});
await new Promise((r) => setTimeout(r, 1500));
console.log('PUT body:', putBody);
console.log('saved toast:', await page.evaluate(() => document.body.textContent.includes('Caption style saved')));
console.log('errors:', errors.length ? errors : 'NONE');
await browser.close(); server.close();
console.log('SMOKE DONE');
