// Real-browser check of "Hear this scene" in the story plan editor (#326). Not part of `ng test`.
//   cd frontend && npx ng build && node e2e/story-voice-preview-verification.mjs [outDir]
// Serves dist/frontend/browser as https://indifferent.fun and a fake API as https://api.indifferent.fun through
// request interception, with a real 1.5 s MP3 behind the signed link, so Chrome's own audio element plays it.
// Checks the request the page makes, the button states, a second play without a new request, the daily limit
// message, the keyboard, focus, and the layout on a phone and a desktop in both themes.
// Screenshots go to outDir (default ~/shots303/story-voice), never into the repo. Exit 1 on any failure.
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '..');
const require = createRequire(path.join(root, '/'));
const puppeteer = require('puppeteer');
const outDir = path.resolve(process.argv[2] ?? path.join(process.env.HOME, 'shots303/story-voice'));
fs.mkdirSync(outDir, { recursive: true });
const dist = path.join(root, 'dist/frontend/browser');
if (!fs.existsSync(path.join(dist, 'index.html'))) throw new Error('Build first: npx ng build');
const mp3 = fs.readFileSync('/tmp/take.mp3');
const SITE = 'https://indifferent.fun';
const API = 'https://api.indifferent.fun';
const AUDIO = 'https://audio.example';
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.mjs': 'text/javascript', '.css': 'text/css', '.woff2': 'font/woff2', '.woff': 'font/woff', '.json': 'application/json', '.svg': 'image/svg+xml', '.png': 'image/png', '.ico': 'image/x-icon', '.webp': 'image/webp', '.jpg': 'image/jpeg', '.wasm': 'application/wasm' };
const CORS = { 'access-control-allow-origin': SITE, 'access-control-allow-credentials': 'true', 'access-control-allow-headers': 'Content-Type,Authorization', 'access-control-allow-methods': 'GET,POST,PUT,DELETE,OPTIONS' };
const failures = [];
const check = (ok, what) => {
  console.log(`${ok ? '  ok  ' : '  FAIL'} ${what}`);
  if (!ok) failures.push(what);
};
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const json = (req, status, data, extra = {}) => req.respond({ status, headers: { ...CORS, 'content-type': 'application/json', ...extra }, body: JSON.stringify(data) });

const plan = () => ({
  planId: 'plan-1', status: 'ready', model: 'test', revision: 1, createdAt: '2026-11-15T10:00:00Z', updatedAt: '2026-11-15T10:00:00Z',
  plan: {
    language: 'en', hook: 'The lighthouse stood alone.', totalSeconds: 15.7,
    scenes: [
      ['The lighthouse stood alone on the cliff.', 'A white lighthouse on a stormy cliff', ['lighthouse', 'cliff']],
      ['Every night the keeper climbed the stairs.', 'A spiral staircase inside a lighthouse', ['staircase']],
      ['One morning the sea was calm again.', 'A calm sea at sunrise', ['calm sea']],
    ].map(([narration, description, keywords]) => ({ narration, description, keywords, mediaType: 'photo', duration: 5 })),
  },
});

function makeApi({ limitAfter = 99 } = {}) {
  const s = { previews: [], audio: 0, made: 0 };
  s.handle = (req) => {
    const u = new URL(req.url());
    const p = u.pathname;
    if (req.method() === 'OPTIONS') return req.respond({ status: 204, headers: CORS });
    if (p === '/auth/refresh') return json(req, 200, { accessToken: 'e2e', expiresIn: 900 });
    if (p === '/auth/me') return json(req, 200, { id: 'u1', email: 'e2e@example.com', name: 'E2E', emailVerified: true });
    if (p === '/story-plans/limits') return json(req, 200, { enabled: true, minStoryChars: 30, maxStoryChars: 5000, minScenes: 2, maxScenes: 10, maxTotalSeconds: 90, perHour: 20, voicePreview: true, voicePreviewsPerDay: 60 });
    if (p === '/story-plans/plan-1') return json(req, 200, plan());
    if (p === '/story-videos/limits') return json(req, 200, { enabled: true, maxActive: 2, perDay: 10, maxSeconds: 90, voices: { en: ['Joanna', 'Matthew'], hi: ['Aditi'] } });
    if (p === '/story-videos') return json(req, 200, { videos: [], hasMore: false });
    if (p === '/story-plans/plan-1/voice-preview') {
      const body = JSON.parse(req.postData() || '{}');
      s.previews.push(body);
      const fresh = new Set(s.previews.map((b) => b.narration + '|' + b.voice)).size;
      if (fresh > limitAfter) return json(req, 429, { code: 'QUOTA_EXCEEDED', message: 'limit', details: { retryAfter: 3600 } }, { 'retry-after': '3600' });
      s.made = fresh;
      return json(req, 200, { url: `${AUDIO}/take-${fresh}.mp3?X-Amz-Signature=e2e`, expiresInSeconds: 300, cached: false, remainingToday: 60 - fresh, dailyLimit: 60 });
    }
    return json(req, 404, { code: 'NOT_FOUND' });
  };
  return s;
}

const browser = await puppeteer.launch({ executablePath: '/usr/bin/google-chrome', headless: true, args: ['--no-sandbox', '--autoplay-policy=no-user-gesture-required'] });
async function open(api, { width = 1280, height = 900, theme = 'light' } = {}) {
  const ctx = await browser.createBrowserContext();
  const page = await ctx.newPage();
  await page.setViewport({ width, height });
  await page.emulateMediaFeatures([{ name: 'prefers-color-scheme', value: theme }]);
  await page.setRequestInterception(true);
  page.on('request', (req) => {
    const u = new URL(req.url());
    if (u.origin === API) return api.handle(req);
    if (u.origin === AUDIO) {
      api.audio++;
      return req.respond({ status: 200, headers: { 'content-type': 'audio/mpeg', 'access-control-allow-origin': '*', 'content-length': String(mp3.length) }, body: mp3 });
    }
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
  await page.goto(SITE + '/story/plan-1', { waitUntil: 'networkidle2', timeout: 30000 });
  await sleep(600);
  return { ctx, page };
}
const label = (page, n) => page.evaluate((i) => document.querySelectorAll('app-story-scene-card')[i - 1].querySelector('[data-testid="hear"]')?.textContent.replace(/\s+/g, ' ').trim(), n);
const click = (page, n) => page.evaluate((i) => document.querySelectorAll('app-story-scene-card')[i - 1].querySelector('[data-testid="hear"]').click(), n);
const alertText = (page, n) => page.evaluate((i) => document.querySelectorAll('app-story-scene-card')[i - 1].querySelector('[data-testid="voice-problem"]')?.textContent.replace(/\s+/g, ' ').trim() ?? null, n);

console.log('Desktop, light');
{
  const api = makeApi({ limitAfter: 2 });
  const { ctx, page } = await open(api);
  const n = await page.evaluate(() => document.querySelectorAll('[data-testid="hear"]').length);
  check(n === 3, `a "Hear this scene" button on each of the 3 scenes (${n})`);
  check((await label(page, 1)).startsWith('Hear this scene'), 'the button says "Hear this scene"');
  await page.screenshot({ path: path.join(outDir, 'editor-desktop-light.png') });

  await click(page, 1);
  await sleep(150);
  check(api.previews.length === 1, 'one voice request made');
  check(api.previews[0].scene === 1 && api.previews[0].revision === 1 && api.previews[0].narration === 'The lighthouse stood alone on the cliff.', `the request names scene 1, revision 1 and the text (${JSON.stringify(api.previews[0])})`);
  check(api.previews[0].voice === 'Joanna', 'it uses the voice chosen for the video');
  await sleep(500);
  const mid = await label(page, 1);
  check(mid.startsWith('Stop'), `while it plays the button says Stop (${mid})`);
  check(api.audio >= 1, 'the browser fetched the audio');
  await page.screenshot({ path: path.join(outDir, 'editor-desktop-playing.png') });
  await sleep(1500);
  check((await label(page, 1)).startsWith('Hear this scene'), 'when the take ends the button is back');

  await click(page, 1);
  await sleep(400);
  check(api.previews.length === 1, 'playing it again makes no new request');
  await click(page, 1); // stop
  await sleep(100);
  check((await label(page, 1)).startsWith('Hear this scene'), 'pressing Stop stops it');

  // unsaved text
  await page.evaluate(() => {
    const t = document.querySelectorAll('app-story-scene-card')[1].querySelector('textarea');
    t.value = 'A line typed a moment ago.';
    t.dispatchEvent(new Event('input', { bubbles: true }));
  });
  await sleep(100);
  await click(page, 2);
  await sleep(300);
  check(api.previews.at(-1).narration === 'A line typed a moment ago.' && api.previews.at(-1).scene === 2, 'unsaved typing is what gets spoken');
  await sleep(1800);

  // the daily limit
  await page.evaluate(() => {
    const t = document.querySelectorAll('app-story-scene-card')[2].querySelector('textarea');
    t.value = 'A third new line.';
    t.dispatchEvent(new Event('input', { bubbles: true }));
  });
  await sleep(100);
  await click(page, 3);
  await sleep(400);
  const msg = await alertText(page, 3);
  check(!!msg && msg.includes("today's 60 new voice previews") && msg.includes('already heard still play'), `the daily limit is said in words beside the scene (${msg})`);
  check((await page.evaluate(() => document.querySelectorAll('app-story-scene-card')[2].querySelector('[data-testid="voice-problem"]').getAttribute('role'))) === 'alert', 'the message is an alert');
  await page.screenshot({ path: path.join(outDir, 'editor-desktop-limit.png') });
  await click(page, 1);
  await sleep(300);
  check(api.previews.length === 3 && (await label(page, 1)).startsWith('Stop'), 'a scene already heard still plays after the limit');
  await ctx.close();
}

console.log('Keyboard and focus');
{
  const api = makeApi();
  const { ctx, page } = await open(api);
  await page.evaluate(() => document.querySelectorAll('app-story-scene-card')[0].querySelector('[data-testid="hear"]').focus());
  await page.keyboard.press('Enter');
  await sleep(300);
  check(api.previews.length === 1, 'Enter on the focused button asks for the voice');
  check(await page.evaluate(() => document.activeElement?.getAttribute('data-testid') === 'hear'), 'focus stays on the button');
  await page.keyboard.press('Space');
  await sleep(200);
  check((await label(page, 1)).startsWith('Hear this scene'), 'Space stops it');
  const status = await page.evaluate(() => document.querySelectorAll('app-story-scene-card')[0].querySelector('[data-testid="voice-status"]').getAttribute('role'));
  check(status === 'status', 'there is a status region for screen readers');
  await ctx.close();
}

for (const [name, size, theme] of [['phone-dark', { width: 390, height: 780 }, 'dark'], ['phone-light', { width: 390, height: 780 }, 'light'], ['desktop-dark', { width: 1280, height: 900 }, 'dark']]) {
  console.log(`Layout: ${name}`);
  const api = makeApi({ limitAfter: 0 });
  const { ctx, page } = await open(api, { ...size, theme });
  const m = await page.evaluate(() => {
    const b = document.querySelector('[data-testid="hear"]');
    const r = b.getBoundingClientRect();
    return { h: r.height, right: r.right, w: innerWidth, sw: document.documentElement.scrollWidth };
  });
  check(m.h >= 44, `the button is at least 44 px tall (${Math.round(m.h)})`);
  check(m.right <= m.w && m.sw <= m.w, 'nothing overflows sideways');
  await click(page, 1);
  await sleep(400);
  check(!!(await alertText(page, 1)), 'a problem is shown without breaking the layout');
  const m2 = await page.evaluate(() => ({ sw: document.documentElement.scrollWidth, w: innerWidth }));
  check(m2.sw <= m2.w, 'still no sideways scroll with the message');
  await page.evaluate(() => document.querySelector('[data-testid="hear"]').scrollIntoView({ block: 'center' }));
  await sleep(200);
  await page.screenshot({ path: path.join(outDir, `editor-${name}-problem.png`) });
  await ctx.close();
}

await browser.close();
console.log(failures.length ? `\n${failures.length} check(s) failed` : '\nall checks passed');
process.exit(failures.length ? 1 : 0);
