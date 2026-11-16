// Real-browser check of the "Title and description" section on the thumbnails page (#331). Not part of `ng test`.
//   cd frontend && npx ng build && node e2e/thumbnail-copy-verification.mjs [outDir]
// Serves dist/frontend/browser as https://indifferent.fun and a fake API as https://api.indifferent.fun through
// request interception. Checks the requests the page makes, what it shows, the real clipboard (read back from
// Chrome, not from a spy), keyboard use, focus, the limit and failure messages, the helper being off, a title
// that contains markup, Hindi text, and the layout on a phone and a desktop in both themes.
// Screenshots go to outDir (default ~/shots303/thumbnail-copy), never into the repo. Exit 1 on any failure.
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '..');
const require = createRequire(path.join(root, '/'));
const puppeteer = require('puppeteer');
const outDir = path.resolve(process.argv[2] ?? path.join(process.env.HOME, 'shots303/thumbnail-copy'));
fs.mkdirSync(outDir, { recursive: true });
const dist = path.join(root, 'dist/frontend/browser');
if (!fs.existsSync(path.join(dist, 'index.html'))) throw new Error('Build first: npx ng build');
const SITE = 'https://indifferent.fun';
const API = 'https://api.indifferent.fun';
const IMG = 'https://cdn.example';
const PNG = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==', 'base64');
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.mjs': 'text/javascript', '.css': 'text/css', '.woff2': 'font/woff2', '.woff': 'font/woff', '.json': 'application/json', '.svg': 'image/svg+xml', '.png': 'image/png', '.ico': 'image/x-icon', '.webp': 'image/webp', '.jpg': 'image/jpeg', '.wasm': 'application/wasm' };
const CORS = { 'access-control-allow-origin': SITE, 'access-control-allow-credentials': 'true', 'access-control-allow-headers': 'Content-Type,Authorization', 'access-control-allow-methods': 'GET,POST,PUT,DELETE,OPTIONS' };
const failures = [];
const check = (ok, what) => {
  console.log(`${ok ? '  ok  ' : '  FAIL'} ${what}`);
  if (!ok) failures.push(what);
};
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const json = (req, status, data) => req.respond({ status, headers: { ...CORS, 'content-type': 'application/json' }, body: JSON.stringify(data) });

const EN_TITLES = Array.from({ length: 10 }, (_, i) => `Restoring my grandfather's brass lamp, idea ${i + 1}`);
const HI_TITLES = ['दादाजी के पुराने पीतल के दीपक को नया जीवन दिया', 'पीतल के दीपक की सफाई और पॉलिश का सही तरीका', ...Array.from({ length: 8 }, (_, i) => `पुराने दीपक की मरम्मत, तरीका ${i + 1}`)];
const DESC = 'This video restores an old brass lamp step by step.\n\nIt covers cleaning, rewiring and polishing.';
const CHAPTERS = [{ start: 0, time: '0:00', title: 'Introduction' }, { start: 60, time: '1:00', title: 'Soaking the parts' }, { start: 150, time: '2:30', title: 'Rewiring' }, { start: 240, time: '4:00', title: 'Polishing' }];
const FULL = DESC + '\n\n' + CHAPTERS.map((c) => `${c.time} ${c.title}`).join('\n');

function makeApi({ enabled = true, transcript = true, remaining = 20, mode = 'ok' } = {}) {
  const s = { writes: [], statusCalls: 0, remaining };
  s.handle = (req) => {
    const p = new URL(req.url()).pathname;
    if (req.method() === 'OPTIONS') return req.respond({ status: 204, headers: CORS });
    if (p === '/auth/refresh') return json(req, 200, { accessToken: 'e2e', expiresIn: 900 });
    if (p === '/auth/me') return json(req, 200, { id: 'u1', email: 'e2e@example.com', name: 'E2E', emailVerified: true });
    if (p === '/shorts/job1/thumbnails') {
      return json(req, 200, {
        status: 'completed', supported: true, jobStatus: 'completed', format: '16:9', style: 'viral', generation: 1, transcriptUsed: true, detectionUsed: false,
        thumbnails: [1, 2, 3].map((i) => ({ id: 'thumb-0' + i, concept: ['face', 'subject', 'creative'][i - 1], style: 'viral', format: '16:9', headline: 'HEADLINE ' + i, frameTime: i * 10, width: 1280, height: 720, url: `${IMG}/t${i}.png`, selected: false })),
        thumbnail: null,
      });
    }
    if (p === '/shorts/job1/copy/status') {
      s.statusCalls++;
      return json(req, 200, { enabled, transcriptAvailable: transcript, remainingToday: s.remaining, dailyLimit: 20, maxTopicChars: 300, maxTranscriptChars: 20000, maxRefineChars: 200 });
    }
    if (p === '/shorts/job1/copy' && req.method() === 'POST') {
      const body = JSON.parse(req.postData() || '{}');
      s.writes.push(body);
      if (mode === 'limit') return json(req, 429, { code: 'QUOTA_EXCEEDED', message: 'limit', details: { retryAfter: 7200 } });
      if (mode === 'busy') return json(req, 503, { code: 'COPY_UNAVAILABLE', message: 'x' });
      s.remaining = Math.max(0, s.remaining - 1);
      const hi = mode === 'hindi';
      const refined = body.refine !== undefined;
      const titles = hi ? HI_TITLES : refined ? EN_TITLES.map((t) => t.replace('idea', 'new idea')) : mode === 'markup' ? ['<img src=x onerror="window.__pwned=1"> Free', ...EN_TITLES.slice(1)] : EN_TITLES;
      const timed = !!body.useJobTranscript || /^\d+:\d\d/.test(body.transcript ?? '');
      return setTimeout(() => json(req, 200, {
        titles, description: hi ? 'इस वीडियो में पुराने पीतल के दीपक को साफ करने की प्रक्रिया दिखाई गई है।\n\nसफाई, तार और चमकाने की पूरी जानकारी।' : DESC,
        chapters: timed && !hi ? CHAPTERS : [], descriptionWithChapters: timed && !hi ? FULL : DESC, hasTimestamps: timed,
        notes: timed ? [] : ['There are no timestamps, so there are no chapters.'], remainingToday: s.remaining, dailyLimit: 20,
      }), 400);
    }
    return json(req, 404, { code: 'NOT_FOUND' });
  };
  return s;
}

const browser = await puppeteer.launch({ executablePath: '/usr/bin/google-chrome', headless: true, args: ['--no-sandbox'] });
async function open(api, { width = 1280, height = 900, theme = 'light' } = {}) {
  const ctx = await browser.createBrowserContext();
  await ctx.overridePermissions(SITE, ['clipboard-read', 'clipboard-write', 'clipboard-sanitized-write']);
  const page = await ctx.newPage();
  await page.setViewport({ width, height });
  await page.emulateMediaFeatures([{ name: 'prefers-color-scheme', value: theme }]);
  await page.setRequestInterception(true);
  page.on('request', (req) => {
    const u = new URL(req.url());
    if (u.origin === API) return api.handle(req);
    if (u.origin === IMG) return req.respond({ status: 200, headers: { 'content-type': 'image/png', 'access-control-allow-origin': '*' }, body: PNG });
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
  await page.evaluateOnNewDocument((t) => {
    if (sessionStorage.getItem('__seeded')) return;
    sessionStorage.setItem('__seeded', '1');
    localStorage.setItem('indifferent.session', 'e2e');
    localStorage.setItem('indifferent.consent.v1', JSON.stringify({ version: 1, savedAt: new Date().toISOString(), expiresAt: new Date(Date.now() + 86400000 * 100).toISOString(), choices: { analytics: false, advertising: false, functional: true } }));
    localStorage.setItem('app-theme', t);
  }, theme);
  await page.goto(SITE + '/shorts/job1/thumbnails', { waitUntil: 'networkidle2', timeout: 30000 });
  await sleep(500);
  return { ctx, page };
}
const q = (page, id) => page.evaluate((i) => document.querySelector(`[data-testid="${i}"]`)?.textContent?.replace(/\s+/g, ' ').trim() ?? null, id);
const click = (page, id) => page.evaluate((i) => document.querySelector(`[data-testid="${i}"]`).click(), id);
const exists = (page, sel) => page.evaluate((s) => !!document.querySelector(s), sel);
const clip = (page) => page.evaluate(() => navigator.clipboard.readText());
const typeInto = async (page, id, text) => {
  await page.focus(`[data-testid="${id}"]`);
  await page.evaluate((i) => { document.querySelector(`[data-testid="${i}"]`).value = ''; }, id);
  await page.keyboard.type(text);
};

console.log('From the video transcript, desktop light');
{
  const api = makeApi();
  const { ctx, page } = await open(api);
  check(await exists(page, 'app-thumbnail-copy section'), 'the section is on the page');
  check(await page.evaluate(() => { const c = document.querySelectorAll('app-thumbnail-card'); const s = document.querySelector('app-thumbnail-copy'); return c.length === 3 && !!(c[2].compareDocumentPosition(s) & Node.DOCUMENT_POSITION_FOLLOWING); }), 'it comes after the three thumbnails');
  check(api.statusCalls === 1, 'one status request');
  check(await page.evaluate(() => document.querySelector('input[value="job"]').checked), 'it starts on this video\'s transcript');
  check((await q(page, 'copy-left')) === '20 of 20 tries left today. Writing new titles uses one too.', `tries left are shown (${await q(page, 'copy-left')})`);
  await page.screenshot({ path: path.join(outDir, 'form-desktop-light.png') });
  await click(page, 'copy-write');
  await sleep(120);
  check((await page.evaluate(() => document.querySelector('[data-testid="copy-write"]').getAttribute('aria-busy'))) === 'true', 'the button says it is busy while it writes');
  check((await q(page, 'copy-status')).includes('Writing the titles and description.'), 'a screen reader is told it is writing');
  await sleep(700);
  check(JSON.stringify(api.writes[0]) === '{"useJobTranscript":true}', `the request is just the transcript choice (${JSON.stringify(api.writes[0])})`);
  check(await page.evaluate(() => document.querySelectorAll('ol.titles li').length === 10), 'ten titles');
  check(await page.evaluate(() => document.activeElement?.id === 'copy-titles-h'), 'focus moved to the titles heading');
  check((await q(page, 'copy-status')).includes('10 titles, a description and 4 chapters are ready.'), 'ready is announced');
  check(await page.evaluate(() => document.querySelectorAll('[data-testid="copy-chapters"] li').length === 4), 'four chapters with their times');
  check((await q(page, 'copy-left')).startsWith('19 of 20'), 'tries left went down by one');
  await page.screenshot({ path: path.join(outDir, 'result-desktop-light.png'), fullPage: true });
  await click(page, 'copy-title-btn-3');
  await sleep(150);
  check((await clip(page)) === EN_TITLES[2], 'the real clipboard holds title 3');
  check((await q(page, 'copy-title-btn-3')).startsWith('Copied'), 'the button says Copied');
  check((await q(page, 'copy-status')).includes('Title 3 copied.'), 'the copy is announced');
  await sleep(2200);
  check(!(await q(page, 'copy-title-btn-3')).startsWith('Copied'), 'Copied goes away after a moment');
  await click(page, 'copy-full-btn');
  await sleep(150);
  check((await clip(page)) === FULL, 'the real clipboard holds the description with its chapters, line for line');
  await click(page, 'copy-description-btn');
  await sleep(150);
  check((await clip(page)) === DESC, 'the plain description copies without the chapters');
  // refine
  await typeInto(page, 'copy-refine', 'shorter');
  await click(page, 'copy-refine-go');
  await sleep(900);
  const w = api.writes[1];
  check(w.useJobTranscript === true && w.refine === 'shorter' && w.previousTitles.length === 10 && w.previousTitles[0] === EN_TITLES[0], 'refine sends the instruction and the ten titles it is changing');
  check((await q(page, 'copy-title-1')).includes('new idea 1'), 'the titles changed');
  check((await q(page, 'copy-description')) === DESC.replace(/\n\n/, ' '), 'the description stayed');
  check(await page.evaluate(() => document.querySelectorAll('[data-testid="copy-chapters"] li').length === 4), 'the chapters stayed');
  check((await q(page, 'copy-left')).startsWith('18 of 20'), 'the refine used a try, as promised');
  await ctx.close();
}

console.log('A sentence only, and a title with markup in it');
{
  const api = makeApi({ transcript: false, mode: 'markup' });
  const { ctx, page } = await open(api);
  check(await page.evaluate(() => document.querySelector('input[value="job"]').disabled && document.querySelector('input[value="topic"]').checked), 'with no transcript it starts on a sentence and the other choice is off');
  check(await page.evaluate(() => document.querySelector('[data-testid="copy-write"]').disabled), 'Write waits for enough text');
  await typeInto(page, 'copy-topic', 'short');
  check(await page.evaluate(() => document.querySelector('[data-testid="copy-write"]').disabled), 'five characters are not enough');
  await typeInto(page, 'copy-topic', 'A video about restoring a brass lamp');
  check(await page.evaluate(() => !document.querySelector('[data-testid="copy-write"]').disabled), 'enough text unlocks it');
  await page.keyboard.press('Tab');
  await page.focus('[data-testid="copy-write"]');
  await page.keyboard.press('Enter');
  await sleep(900);
  check(JSON.stringify(api.writes[0]) === '{"topic":"A video about restoring a brass lamp"}', 'the sentence is sent as typed');
  check(!(await exists(page, '[data-testid="copy-chapters"]')) && (await q(page, 'copy-note')).includes('no timestamps'), 'no chapters, and the page says why');
  check(!(await exists(page, '[data-testid="copy-full-btn"]')), 'no copy-with-chapters button');
  check((await q(page, 'copy-title-1')) === '<img src=x onerror="window.__pwned=1"> Free', 'a title with markup is shown as text');
  check(!(await page.evaluate(() => window.__pwned || !!document.querySelector('app-thumbnail-copy img'))), 'and does nothing');
  await ctx.close();
}

console.log('A pasted transcript, keyboard choice of source');
{
  const api = makeApi();
  const { ctx, page } = await open(api);
  await page.focus('input[value="job"]');
  await page.keyboard.press('ArrowDown');
  await sleep(100);
  check(await page.evaluate(() => document.querySelector('input[value="paste"]').checked), 'the arrow key moves to the next choice');
  check(await page.evaluate(() => !!document.querySelector('label[for="copy-paste"]')), 'the pasted box has a label');
  await typeInto(page, 'copy-paste', '0:00 Welcome to the video\n1:05 The first step\n2:10 The second step');
  await click(page, 'copy-write');
  await sleep(900);
  check(api.writes[0].transcript === '0:00 Welcome to the video\n1:05 The first step\n2:10 The second step', 'the pasted text goes as typed, with its line breaks');
  check(await page.evaluate(() => document.querySelectorAll('[data-testid="copy-chapters"] li').length === 4), 'timed text gets chapters');
  await ctx.close();
}

console.log('Hindi');
{
  const api = makeApi({ mode: 'hindi' });
  const { ctx, page } = await open(api);
  await click(page, 'copy-write');
  await sleep(900);
  check((await q(page, 'copy-title-1')) === HI_TITLES[0], 'Hindi titles are shown');
  const ok = await page.evaluate(() => { const r = document.querySelector('[data-testid="copy-title-1"]').getBoundingClientRect(); return r.width > 50 && r.height > 10; });
  check(ok, 'and take up room on the page');
  await page.screenshot({ path: path.join(outDir, 'result-hindi.png') });
  await click(page, 'copy-title-btn-1');
  await sleep(150);
  check((await clip(page)) === HI_TITLES[0], 'a Hindi title copies exactly');
  await ctx.close();
}

console.log('Limits and failures');
for (const [mode, kind, words] of [['limit', 'rate_limited', 'in 2 hours'], ['busy', 'unavailable', 'busy']]) {
  const api = makeApi({ mode });
  const { ctx, page } = await open(api);
  await click(page, 'copy-write');
  await sleep(500);
  const box = await page.evaluate(() => { const b = document.querySelector('[data-testid="copy-problem"]'); return b && { kind: b.dataset.kind, role: b.getAttribute('role'), text: b.textContent.replace(/\s+/g, ' ') }; });
  check(box && box.kind === kind && box.text.includes(words), `${mode}: says what happened (${box?.text.slice(0, 70)})`);
  check(box && box.role === (kind === 'rate_limited' ? 'status' : 'alert'), `${mode}: announced as ${box?.role}`);
  check(await page.evaluate(() => !document.querySelector('[data-testid="copy-write"]').disabled), `${mode}: the button is usable again`);
  if (mode === 'limit') await page.screenshot({ path: path.join(outDir, 'limit-desktop-light.png') });
  await ctx.close();
}

console.log('Switched off');
{
  const api = makeApi({ enabled: false });
  const { ctx, page } = await open(api);
  check(!(await exists(page, 'app-thumbnail-copy section')), 'nothing is shown when the server has it off');
  check(await exists(page, 'app-thumbnail-card'), 'the thumbnails are untouched');
  await ctx.close();
}

for (const [name, size, theme] of [['phone-light', { width: 390, height: 780 }, 'light'], ['phone-dark', { width: 390, height: 780 }, 'dark'], ['desktop-dark', { width: 1280, height: 900 }, 'dark']]) {
  console.log(`Layout: ${name}`);
  const api = makeApi({ mode: 'hindi' });
  const { ctx, page } = await open(api, { ...size, theme });
  await click(page, 'copy-write');
  await sleep(900);
  const m = await page.evaluate(() => {
    const btns = [...document.querySelectorAll('app-thumbnail-copy button')].map((b) => b.getBoundingClientRect().height);
    return { minH: Math.min(...btns), w: innerWidth, sw: document.documentElement.scrollWidth, count: btns.length };
  });
  check(m.minH >= 44, `every button is at least 44 px tall (${Math.round(m.minH)} over ${m.count})`);
  check(m.sw <= m.w, 'nothing overflows sideways');
  await page.evaluate(() => document.querySelector('app-thumbnail-copy').scrollIntoView());
  await page.screenshot({ path: path.join(outDir, `layout-${name}.png`) });
  await ctx.close();
}

await browser.close();
console.log(failures.length ? `\n${failures.length} FAILED` : '\nAll checks passed');
process.exit(failures.length ? 1 : 0);
