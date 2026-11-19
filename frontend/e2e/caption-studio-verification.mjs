// Real-browser check of the shared caption studio on the AI Shorts clips page (#327). Not part of `ng test`.
//   cd frontend && npx ng build && node e2e/caption-studio-verification.mjs [outDir]
// Serves dist/frontend/browser as https://indifferent.fun and a fake API as https://api.indifferent.fun through
// request interception. Checks the controls, the live preview, what the page sends on save, that a saved job
// keeps its own style over a remembered one, remembering with Functional on and off,
// reduced motion, and the layout on a phone and a desktop in both themes.
// Screenshots go to outDir (default ~/shots303/caption-studio), never into the repo. Exit 1 on any failure.
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '..');
const require = createRequire(path.join(root, '/'));
const puppeteer = require('puppeteer');
const outDir = path.resolve(process.argv[2] ?? path.join(process.env.HOME, 'shots303/caption-studio'));
fs.mkdirSync(outDir, { recursive: true });
const dist = path.join(root, 'dist/frontend/browser');
if (!fs.existsSync(path.join(dist, 'index.html'))) throw new Error('Build first: npx ng build');
const SITE = 'https://indifferent.fun';
const API = 'https://api.indifferent.fun';
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.mjs': 'text/javascript', '.css': 'text/css', '.woff2': 'font/woff2', '.woff': 'font/woff', '.json': 'application/json', '.svg': 'image/svg+xml', '.png': 'image/png', '.ico': 'image/x-icon', '.webp': 'image/webp', '.jpg': 'image/jpeg', '.wasm': 'application/wasm' };
const CORS = { 'access-control-allow-origin': SITE, 'access-control-allow-credentials': 'true', 'access-control-allow-headers': 'Content-Type,Authorization', 'access-control-allow-methods': 'GET,POST,PUT,DELETE,OPTIONS' };
const failures = [];
const check = (ok, what) => {
  console.log(`${ok ? '  ok  ' : '  FAIL'} ${what}`);
  if (!ok) failures.push(what);
};
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const json = (req, status, data) => req.respond({ status, headers: { ...CORS, 'content-type': 'application/json' }, body: JSON.stringify(data) });
const CLIPS = [1, 2].map((r) => ({ clipId: 'c' + r, s3Key: 'k', rank: r, score: 0.9, duration: 30, hookText: 'Hook ' + r, start: r * 60, end: r * 60 + 30 }));
function makeApi({ status = 'uploaded', caption } = {}) {
  const s = { puts: [] };
  s.handle = (req) => {
    const p = new URL(req.url()).pathname;
    if (req.method() === 'OPTIONS') return req.respond({ status: 204, headers: CORS });
    if (p === '/auth/refresh') return json(req, 200, { accessToken: 'e2e', expiresIn: 900 });
    if (p === '/auth/me') return json(req, 200, { id: 'u1', email: 'e2e@example.com', name: 'E2E', emailVerified: true });
    if (p === '/shorts/job1/clips') return json(req, 200, { clips: CLIPS });
    if (p === '/shorts/job1/style' && req.method() === 'PUT') {
      s.puts.push(JSON.parse(req.postData() || '{}'));
      return json(req, 200, { jobId: 'job1' });
    }
    if (p === '/shorts/job1') return json(req, 200, { jobId: 'job1', status, fileType: 'mp4', sourceDuration: 600, clipEdits: {}, caption });
    if (/\/clips\/[^/]+\/url$/.test(p)) return json(req, 200, {});
    return json(req, 404, { code: 'NOT_FOUND' });
  };
  return s;
}
const browser = await puppeteer.launch({ executablePath: '/usr/bin/google-chrome', headless: true, args: ['--no-sandbox'] });
const consentRecord = (functional) => JSON.stringify({ version: 1, savedAt: new Date().toISOString(), expiresAt: new Date(Date.now() + 86400000 * 100).toISOString(), choices: { analytics: false, advertising: false, functional } });
async function open(api, { width = 1280, height = 1000, theme = 'light', functional = true, reduced = false, local = {} } = {}) {
  const ctx = await browser.createBrowserContext();
  const page = await ctx.newPage();
  await page.setViewport({ width, height });
  await page.emulateMediaFeatures([{ name: 'prefers-color-scheme', value: theme }, { name: 'prefers-reduced-motion', value: reduced ? 'reduce' : 'no-preference' }]);
  await page.setRequestInterception(true);
  page.on('request', (req) => {
    const u = new URL(req.url());
    if (u.origin === API) return api.handle(req);
    if (u.origin === SITE) {
      let f = path.resolve(dist, '.' + path.posix.normalize('/' + decodeURIComponent(u.pathname)));
      if (!f.startsWith(dist) || !fs.existsSync(f) || fs.statSync(f).isDirectory()) {
        const d = path.join(f, 'index.html');
        f = f.startsWith(dist) && fs.existsSync(d) ? d : path.join(dist, 'index.html');
      }
      return req.respond({ status: 200, headers: { 'content-type': mime[path.extname(f)] ?? 'application/octet-stream' }, body: fs.readFileSync(f) });
    }
    return req.abort();
  });
  await page.evaluateOnNewDocument((t, rec, extra) => {
    if (sessionStorage.getItem('__seeded')) return;
    sessionStorage.setItem('__seeded', '1');
    localStorage.setItem('indifferent.session', 'e2e');
    localStorage.setItem('indifferent.consent.v1', rec);
    localStorage.setItem('app-theme', t);
    for (const [k, v] of Object.entries(extra)) localStorage.setItem(k, v);
  }, theme, consentRecord(functional), local);
  await page.goto(SITE + '/shorts/job1/clips', { waitUntil: 'networkidle2', timeout: 30000 });
  await sleep(400);
  await page.evaluate(() => document.querySelector('button[aria-label="Caption styles"]').click());
  await sleep(400);
  return { ctx, page };
}
const preset = (page) => page.evaluate(() => document.querySelector('input[name="gal-cap-preset"]:checked')?.value);
const above = (page) => page.evaluate(() => document.querySelector('input[type="checkbox"]').checked);
const clickSel = (page, sel) => page.evaluate((s) => document.querySelector(s).click(), sel);
const save = async (page) => { await page.evaluate(() => Array.from(document.querySelectorAll('button')).find((b) => b.textContent.trim() === 'Save style').click()); await sleep(300); };
const capCss = (page) => page.evaluate(() => { const c = document.querySelector('[data-testid="caption-preview-text"]'); const cs = getComputedStyle(c); const f = document.querySelector('[data-testid="caption-preview"]').getBoundingClientRect(); const r = c.getBoundingClientRect(); return { fontSize: parseFloat(cs.fontSize), frameH: f.height, frameW: f.width, bottomGap: f.bottom - r.bottom, top: r.top - f.top, family: cs.fontFamily, color: cs.color, inside: r.left >= f.left - 1 && r.right <= f.right + 1 }; });
const stored = (page) => page.evaluate(() => localStorage.getItem('indifferent.captionStyle.v1'));

console.log('A new job, Functional on, desktop light');
{
  const api = makeApi();
  const { ctx, page } = await open(api);
  check(await page.evaluate(() => !!document.querySelector('app-caption-studio [data-testid="caption-preview"]')), 'the studio and its preview are on the page');
  check(await preset(page) === 'bold' && await above(page), 'a new job starts on Bold, above platform controls');
  const a = await capCss(page);
  check(Math.abs(a.fontSize / a.frameH - 18 / 288) < 0.004, `text is 18/288 of the frame height (${a.fontSize.toFixed(2)} of ${a.frameH.toFixed(0)})`);
  check(a.bottomGap / a.frameH > 0.13 && a.bottomGap / a.frameH < 0.2, `the caption sits about 15% up (${(a.bottomGap / a.frameH * 100).toFixed(1)}%)`);
  await page.screenshot({ path: path.join(outDir, 'studio-desktop-light.png'), fullPage: true });
  await clickSel(page, '#gal-cap-above');
  await sleep(100);
  const b = await capCss(page);
  check(b.bottomGap / b.frameH < 0.06, `turning it off drops the caption to the old margin (${(b.bottomGap / b.frameH * 100).toFixed(1)}%)`);
  await clickSel(page, '#gal-cap-above');
  await clickSel(page, 'input[name="gal-cap-preset"][value="karaoke"]');
  await sleep(150);
  check(await page.evaluate(() => !!document.querySelector('input[aria-label="Karaoke highlight colour"]')), 'karaoke shows its highlight colour');
  check(await page.evaluate(() => !!document.querySelector('[data-testid="preview-pause"]')), 'karaoke offers a pause button');
  const lit = () => page.evaluate(() => Array.from(document.querySelectorAll('.cap .t')).map((e) => getComputedStyle(e).color).join('|'));
  const l1 = await lit(); await sleep(1400); const l2 = await lit();
  check(l1 !== l2, 'the karaoke highlight moves');
  await clickSel(page, '[data-testid="preview-pause"]');
  const l3 = await lit(); await sleep(1500); const l4 = await lit();
  check(l3 === l4, 'Pause stops it');
  await page.evaluate(() => { const s = document.querySelector('#gal-cap-font'); s.value = 'serif'; s.dispatchEvent(new Event('change')); });
  await sleep(100);
  check((await capCss(page)).family.includes('Serif'), 'the Serif choice changes the preview font');
  await page.evaluate(() => { const r = document.querySelector('#gal-cap-size'); r.value = '72'; r.dispatchEvent(new Event('input')); });
  await sleep(100);
  const big = await capCss(page);
  check(Math.abs(big.fontSize / big.frameH - 72 / 288) < 0.01 && big.inside, 'size 72 stays inside the frame and scales with it');
  await clickSel(page, 'input[name="gal-cap-pos"][value="top"]');
  await sleep(100);
  check((await capCss(page)).top / big.frameH < 0.12, 'Top places the caption at the top');
  await page.screenshot({ path: path.join(outDir, 'studio-karaoke-top-serif-72.png'), fullPage: true });
  await save(page);
  check(api.puts.length === 1, 'saving makes one request');
  const sent = api.puts[0]?.caption;
  check(JSON.stringify(sent) === JSON.stringify({ preset: 'karaoke', fontSize: 72, color: '#FFFFFF', highlight: '#FFD60A', position: 'top', font: 'serif', aboveControls: true }), `it sends the whole style (${JSON.stringify(sent)})`);
  check((await stored(page))?.includes('"karaoke"'), 'the style is remembered in this browser');
  await ctx.close();
}

console.log('The next new job starts from the remembered style; a saved job keeps its own');
{
  const remembered = JSON.stringify({ preset: 'clean', fontSize: 26, color: '#FFFFFF', highlight: '#FFD60A', position: 'middle', font: 'condensed', aboveControls: false });
  const fresh = await open(makeApi(), { local: { 'indifferent.captionStyle.v1': remembered } });
  check(await preset(fresh.page) === 'clean' && !(await above(fresh.page)), 'a job with no saved style starts from the remembered one');
  await fresh.ctx.close();
  const savedCaption = { preset: 'minimal', fontSize: 40, color: '#FFFFFF', highlight: '#FFD60A', position: 'bottom' };
  const saved = await open(makeApi({ caption: savedCaption }), { local: { 'indifferent.captionStyle.v1': remembered } });
  check(await preset(saved.page) === 'minimal', 'a job with a saved style keeps it over the remembered one');
  check(!(await above(saved.page)), 'a job saved before the option existed shows it off, as it will render');
  await saved.ctx.close();
}

console.log('Functional off');
{
  const api = makeApi();
  const { ctx, page } = await open(api, { functional: false });
  check(await page.evaluate(() => document.querySelector('[data-testid="remember-note"]').textContent.includes('will not be remembered')), 'the note says the style will not be remembered');
  check(await page.evaluate(() => !!document.querySelector('[data-testid="remember-note"] a')), 'and links to cookie settings');
  await clickSel(page, 'input[name="gal-cap-preset"][value="clean"]');
  await save(page);
  check(api.puts.length === 1, 'the job still saves');
  check((await stored(page)) === null, 'nothing is written to the browser');
  await page.evaluate(() => document.querySelector('[data-testid="remember-note"] a').click());
  await sleep(400);
  check(await page.evaluate(() => !!document.querySelector('[data-cookie-preferences]')), 'the link opens cookie settings');
  await ctx.close();
}

console.log('Reduced motion');
{
  const { ctx, page } = await open(makeApi(), { reduced: true });
  await clickSel(page, 'input[name="gal-cap-preset"][value="karaoke"]');
  await sleep(150);
  const lit = () => page.evaluate(() => Array.from(document.querySelectorAll('.cap .t')).map((e) => getComputedStyle(e).color).join('|'));
  const a = await lit(); await sleep(1500); const b = await lit();
  check(a === b, 'the highlight does not move');
  check(new Set(a.split('|')).size === 2, 'a fixed half-lit caption is shown instead');
  check(await page.evaluate(() => !document.querySelector('[data-testid="preview-pause"]')), 'there is nothing to pause');
  await ctx.close();
}

console.log('A rendered job');
{
  const { ctx, page } = await open(makeApi({ status: 'completed' }));
  check(await page.evaluate(() => document.querySelector('#gal-cap-font').disabled && document.querySelector('input[type="checkbox"]').disabled), 'the controls are locked');
  check(await page.evaluate(() => /already rendered/.test(document.body.textContent)), 'and the page says why');
  await ctx.close();
}

for (const [name, opts] of [['phone-light', { width: 390, height: 844 }], ['phone-dark', { width: 390, height: 844, theme: 'dark' }], ['desktop-dark', { width: 1280, height: 1000, theme: 'dark' }]]) {
  console.log(`Layout, ${name}`);
  const { ctx, page } = await open(makeApi(), opts);
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  check(overflow <= 0, `no sideways scrolling (${overflow}px)`);
  const small = await page.evaluate(() => Array.from(document.querySelectorAll('app-caption-studio label.opt, app-caption-studio select, app-caption-studio button')).filter((e) => e.getBoundingClientRect().height < 44).length);
  check(small === 0, `every control is at least 44 px tall (${small} smaller)`);
  await page.screenshot({ path: path.join(outDir, `studio-${name}.png`), fullPage: true });
  await ctx.close();
}
await browser.close();
if (failures.length) {
  console.log(`\n${failures.length} FAILED`);
  process.exit(1);
}
console.log('\nAll checks passed');
