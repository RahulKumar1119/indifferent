// Real-browser check of the post copy for each platform (#330), on the AI Shorts clips page and on a finished
// Story video. Not part of `ng test`.
//   cd frontend && npx ng build && node e2e/post-copy-verification.mjs [outDir]
// Serves dist/frontend/browser as https://indifferent.fun and a fake API as https://api.indifferent.fun through
// request interception. The fake API keeps the same rules the backend has: a request id makes a retry free, a
// copy over a limit is refused on save. Checks the requests the page makes, the real clipboard (read back from
// Chrome), live counts in each platform's unit, Copy and Save turning off at a limit, a retry after a dropped
// connection reusing its id, Hindi, keyboard use, the button being hidden while the feature is off, and the layout
// on a phone and a desktop in both themes.
// Screenshots go to outDir (default ~/shots303/post-copy), never into the repo. Exit 1 on any failure.
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '..');
const require = createRequire(path.join(root, '/'));
const puppeteer = require('puppeteer');
const outDir = path.resolve(process.argv[2] ?? path.join(process.env.HOME, 'shots303/post-copy'));
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

const SPECS = [
  { id: 'youtube', label: 'YouTube', hasTitle: true, titleMax: 100, titleUnit: 'characters', textLabel: 'Description', textMax: 5000, textUnit: 'bytes', aim: 700, hashtagsMax: 5, hashtagsMin: 3 },
  { id: 'tiktok', label: 'TikTok', hasTitle: false, textLabel: 'Caption', textMax: 2200, textUnit: 'utf16', aim: 220, hashtagsMax: 5, hashtagsMin: 3 },
  { id: 'instagram', label: 'Instagram Reels', hasTitle: false, textLabel: 'Caption', textMax: 2200, textUnit: 'utf16', aim: 400, hashtagsMax: 5, hashtagsMin: 3 },
];
const EN = {
  youtube: { title: 'Restoring My Grandfather’s Brass Lamp with New Wiring', text: 'This video shows the restoration of a vintage brass lamp. I replaced the cracked cable and fitted a safe modern socket.\n\nFirst the corroded parts soaked in warm water with vinegar for an hour.', hashtags: ['#BrassLamp', '#LampRestoration', '#ClothCoveredWire'] },
  tiktok: { text: 'Giving new life to my grandfather’s brass lamp\n\nSoaked, scrubbed and rewired with a cloth-covered wire', hashtags: ['#LampRestoration', '#DIYRepair', '#BrassLamp'] },
  instagram: { text: 'Restoring my grandfather’s brass lamp step by step\nA new cloth wire and a safe socket', hashtags: ['#LampRestoration', '#VintageLamp', '#BrassLamp'] },
};
const HI = {
  youtube: { title: 'दादाजी के पुराने पीतल के दीपक का टूटा तार बदला', text: 'नमस्कार दोस्तों, आज मैंने अपने दादाजी के पुराने पीतल के दीपक का टूटा तार बदला।\n\nजंग लगे पुर्ज़ों को गुनगुने पानी और थोड़े सिरके में एक घंटे तक भिगोया।', hashtags: ['#पीतलकादीपक', '#तारबदला', '#घरपरमरम्मत'] },
  tiktok: { text: 'दादाजी के पुराने पीतल के दीपक का टूटा तार बदला।', hashtags: ['#पीतलकादीपक', '#तारबदला', '#घरपरमरम्मत'] },
  instagram: { text: 'दादाजी का पीतल का दीपक फिर से रोशन।', hashtags: ['#पीतलकादीपक', '#दादाजीकादीपक', '#घरपरमरम्मत'] },
};
const CLIPS = [1, 2].map((r) => ({ clipId: 'c' + r, s3Key: 'k', rank: r, score: 0.9, duration: 30, hookText: 'Hook line for clip ' + r, start: r * 60, end: r * 60 + 30 }));
const VIDEO_ID = '01939f6c-8a40-7e21-b2c4-aa11bb22cc33';

const len = { bytes: (s) => Buffer.byteLength(s, 'utf8'), utf16: (s) => s.length, characters: (s) => Array.from(s).length };
const full = (c) => (c.hashtags.length ? `${c.text}\n\n${c.hashtags.join(' ')}` : c.text);

function makeApi({ enabled = true, copy = EN, dropFirstWrite = false } = {}) {
  const s = { writes: [], saves: [], answered: new Map(), saved: {}, used: 0, dropped: 0 };
  s.handle = (req) => {
    const u = new URL(req.url());
    const p = u.pathname;
    const m = req.method();
    if (m === 'OPTIONS') return req.respond({ status: 204, headers: CORS });
    if (p === '/auth/refresh') return json(req, 200, { accessToken: 'e2e', expiresIn: 900 });
    if (p === '/auth/me') return json(req, 200, { id: 'u1', email: 'e2e@example.com', name: 'E2E', emailVerified: true });
    if (p === '/shorts/job1/clips') return json(req, 200, { clips: CLIPS });
    if (p === '/shorts/job1') return json(req, 200, { jobId: 'job1', status: 'completed', fileType: 'mp4', sourceDuration: 600, clipEdits: {} });
    if (/\/clips\/[^/]+\/url$/.test(p)) return json(req, 200, {});
    if (p === '/shorts/job1/post-copy') {
      return json(req, 200, { enabled, platforms: SPECS, saved: Object.fromEntries(Object.entries(s.saved).filter(([k]) => k.startsWith('clip:')).map(([k, v]) => [k.slice(5), v])), remainingToday: 20 - s.used, dailyLimit: 20 });
    }
    const clip = /^\/shorts\/job1\/clips\/([^/]+)\/post-copy$/.exec(p);
    const story = new RegExp(`^/story-videos/${VIDEO_ID}/post-copy$`).exec(p);
    if (clip || story) {
      const key = clip ? 'clip:' + clip[1] : 'story';
      if (!enabled) return m === 'GET' ? json(req, 200, { enabled: false, platforms: SPECS, saved: null, dailyLimit: 20 }) : json(req, 503, { code: 'UNAVAILABLE', message: 'off' });
      if (m === 'GET') return json(req, 200, { enabled: true, platforms: SPECS, saved: s.saved[key] ?? null, remainingToday: 20 - s.used, dailyLimit: 20 });
      if (m === 'POST') {
        const body = JSON.parse(req.postData() || '{}');
        s.writes.push(body);
        if (s.answered.has(body.requestId)) return json(req, 200, { ...s.answered.get(body.requestId), replayed: true, remainingToday: 20 - s.used });
        s.used++;
        const saved = { copy, notes: ['TikTok: 1 hashtag left out because it was generic or not about the video.'], source: 'generated', generatedAt: new Date().toISOString(), updatedAt: new Date().toISOString() };
        s.saved[key] = saved;
        s.answered.set(body.requestId, { ...saved, dailyLimit: 20 });
        if (dropFirstWrite && s.dropped === 0) {
          s.dropped++;
          return req.abort('connectionreset');
        }
        return setTimeout(() => json(req, 200, { ...saved, remainingToday: 20 - s.used, dailyLimit: 20, replayed: false }), 300);
      }
      if (m === 'PUT') {
        const body = JSON.parse(req.postData() || '{}');
        s.saves.push(body);
        for (const sp of SPECS) {
          const c = body.copy?.[sp.id];
          if (!c) return json(req, 400, { code: 'VALIDATION_ERROR', message: `${sp.label} is missing` });
          if (sp.hasTitle && len[sp.titleUnit](c.title) > sp.titleMax) return json(req, 400, { code: 'VALIDATION_ERROR', message: `${sp.label} title is too long.` });
          const n = len[sp.textUnit](full(c));
          if (n > sp.textMax) return json(req, 400, { code: 'VALIDATION_ERROR', message: `${sp.label} ${sp.textLabel.toLowerCase()} with its hashtags is ${n} ${sp.textUnit === 'bytes' ? 'bytes' : 'characters'}; the limit is ${sp.textMax}.` });
        }
        const saved = { copy: body.copy, notes: [], source: 'edited', generatedAt: s.saved[key]?.generatedAt, updatedAt: new Date().toISOString() };
        s.saved[key] = saved;
        return json(req, 200, saved);
      }
    }
    if (p === '/story-plans/limits') return json(req, 200, { enabled: true, minStoryChars: 30, maxStoryChars: 5000, minScenes: 2, maxScenes: 10, maxTotalSeconds: 90, perHour: 20 });
    if (p === '/story-videos/limits') return json(req, 200, { enabled: true, maxActive: 2, perDay: 10, maxSeconds: 90, voices: { en: ['Joanna', 'Matthew'], hi: ['Aditi'] } });
    if (p === '/story-plans/plan-1') {
      const scenes = [1, 2].map((n) => ({ narration: ['The fisherman rows quietly across the dark water.', 'He lifts the net and finds it full of silver fish.'][n - 1], description: `Scene ${n}`, keywords: ['fisherman'], mediaType: 'photo', duration: 8, media: { provider: 'pixabay', type: 'photo', id: String(100 + n) } }));
      const PIXEL = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=';
      return json(req, 200, {
        planId: 'plan-1', status: 'ready', revision: 1, createdAt: '2026-11-20T10:00:00Z', updatedAt: '2026-11-20T10:00:00Z',
        plan: { language: 'en', hook: 'The fisherman rows.', totalSeconds: 16, scenes },
        media: scenes.map((sc, i) => ({ ...sc.media, asset: { provider: 'pixabay', type: 'photo', id: String(101 + i), previewUrl: PIXEL, pageUrl: 'https://pixabay.com/x', width: 1080, height: 1920, orientation: 'portrait', alt: 'x', author: 'Jo', credit: 'Photo by Jo on Pixabay' } })),
        mediaAttributions: [{ provider: 'pixabay', text: 'Photos provided by Pixabay', url: 'https://pixabay.com' }],
      });
    }
    if (p === '/story-videos' && m === 'GET') {
      return json(req, 200, { videos: [{ jobId: VIDEO_ID, planId: 'plan-1', planRevision: 1, status: 'completed', progress: 100, language: 'en', voice: 'Joanna', caption: { preset: 'bold' }, totalSeconds: 16, createdAt: '2026-11-20T10:00:00Z', updatedAt: '2026-11-20T10:05:00Z', completedAt: '2026-11-20T10:05:00Z' }], hasMore: false });
    }
    return json(req, 404, { code: 'NOT_FOUND' });
  };
  return s;
}

const browser = await puppeteer.launch({ executablePath: '/usr/bin/google-chrome', headless: true, args: ['--no-sandbox'] });
const consent = JSON.stringify({ version: 1, savedAt: new Date().toISOString(), expiresAt: new Date(Date.now() + 86400000 * 100).toISOString(), choices: { analytics: false, advertising: false, functional: true } });

async function open(api, route, { width = 1280, height = 1000, theme = 'light' } = {}) {
  const ctx = await browser.createBrowserContext();
  await ctx.overridePermissions(SITE, ['clipboard-read', 'clipboard-write', 'clipboard-sanitized-write']);
  const page = await ctx.newPage();
  await page.setViewport({ width, height });
  await page.emulateMediaFeatures([{ name: 'prefers-color-scheme', value: theme }]);
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
  const errors = [];
  page.on('pageerror', (e) => errors.push(String(e)));
  page.on('console', (m) => { if (m.type() === 'error' && !/Failed to load resource|net::ERR_/.test(m.text())) errors.push(m.text()); });
  await page.evaluateOnNewDocument((t, rec) => {
    if (sessionStorage.getItem('__seeded')) return;
    sessionStorage.setItem('__seeded', '1');
    localStorage.setItem('indifferent.session', 'e2e');
    localStorage.setItem('indifferent.consent.v1', rec);
    localStorage.setItem('app-theme', t);
  }, theme, consent);
  await page.goto(SITE + route, { waitUntil: 'networkidle2', timeout: 30000 });
  await sleep(400);
  return { ctx, page, errors };
}
const T = (id) => `[data-testid="${id}"]`;
const val = (page, id) => page.$eval(T(id), (e) => e.value);
const txt = (page, id) => page.$eval(T(id), (e) => e.textContent.replace(/\s+/g, ' ').trim()).catch(() => null);
const exists = (page, sel) => page.evaluate((s) => !!document.querySelector(s), sel);
const disabled = (page, id) => page.$eval(T(id), (e) => e.disabled);
const clip = (page) => page.evaluate(() => navigator.clipboard.readText());
async function setField(page, id, value) {
  await page.$eval(T(id), (e, v) => { e.focus(); e.value = v; e.dispatchEvent(new Event('input', { bubbles: true })); }, value);
  await sleep(80);
}
async function layout(page, label) {
  const r = await page.evaluate(() => {
    const host = document.querySelector('app-post-copy');
    const over = document.documentElement.scrollWidth - document.documentElement.clientWidth;
    const small = Array.from(host?.querySelectorAll('button, input, textarea') ?? []).filter((e) => e.getBoundingClientRect().height < 44 && e.offsetParent).map((e) => e.outerHTML.slice(0, 60));
    const box = host?.getBoundingClientRect();
    const out = Array.from(host?.querySelectorAll('*') ?? []).filter((e) => { const b = e.getBoundingClientRect(); return b.width > 0 && box && (b.right > box.right + 1 || b.left < box.left - 1); }).length;
    return { over, small, out };
  });
  check(r.over <= 0, `${label}: no sideways scroll (${r.over}px)`);
  check(r.small.length === 0, `${label}: every control is at least 44 px tall ${r.small.length ? JSON.stringify(r.small) : ''}`);
  check(r.out === 0, `${label}: nothing spills out of the panel (${r.out})`);
}

console.log('1. Off: no Post copy button anywhere');
{
  const api = makeApi({ enabled: false });
  const { ctx, page } = await open(api, '/shorts/job1/clips');
  check(!(await exists(page, T('post-copy-toggle'))), 'the clips page shows no Post copy button');
  await page.evaluate(() => document.querySelector('[data-clip="c1"] button[aria-label="Copy caption"]').click());
  await sleep(200);
  check((await clip(page)) === 'Hook line for clip 1', `the quick copy is the plain hook line, no #fyp #viral #shorts (${JSON.stringify(await clip(page))})`);
  await ctx.close();
}

console.log('2. A clip, English, desktop light');
{
  const api = makeApi();
  const { ctx, page, errors } = await open(api, '/shorts/job1/clips');
  const toggles = await page.$$(T('post-copy-toggle'));
  check(toggles.length === 2, 'each clip has a Post copy button');
  check((await page.$eval(T('post-copy-toggle'), (b) => b.getAttribute('aria-label'))) === 'Write post copy for clip 1', 'the button says what it is for');
  await page.click(T('post-copy-toggle'));
  await sleep(300);
  check(await exists(page, '[data-clip="c1"] app-post-copy'), 'it opens under clip 1');
  check((await txt(page, 'pc-left'))?.startsWith('20 of 20'), 'uses left are shown');
  await page.screenshot({ path: path.join(outDir, 'clip-empty-desktop-light.png'), fullPage: true });
  await page.click(T('pc-write'));
  await sleep(100);
  check((await page.$eval(T('pc-write'), (b) => b.getAttribute('aria-busy'))) === 'true', 'the button is busy while it writes');
  await sleep(600);
  check(api.writes.length === 1 && /^[A-Za-z0-9_-]{16,64}$/.test(api.writes[0].requestId) && Object.keys(api.writes[0]).length === 1, `the request is only a request id (${JSON.stringify(api.writes[0])})`);
  check((await val(page, 'pc-youtube-title')) === EN.youtube.title, 'the YouTube title is shown');
  check((await val(page, 'pc-tiktok-tags')) === EN.tiktok.hashtags.join(' '), 'the TikTok hashtags are shown');
  check(await page.evaluate(() => document.activeElement?.tagName === 'H3'), 'focus moved to the heading');
  check((await txt(page, 'pc-notes'))?.includes('1 hashtag left out'), 'what was left out is said');
  check((await txt(page, 'pc-left'))?.startsWith('19 of 20'), 'one use was spent');
  const ytFull = full(EN.youtube);
  check((await txt(page, 'pc-youtube-text-count')) === `${Buffer.byteLength(ytFull)} of 5,000 bytes, hashtags included`, `the description is counted in bytes (${await txt(page, 'pc-youtube-text-count')})`);
  await page.screenshot({ path: path.join(outDir, 'clip-written-desktop-light.png'), fullPage: true });
  await layout(page, 'desktop light');

  await page.click(T('pc-youtube-copy-title'));
  await sleep(150);
  check((await clip(page)) === EN.youtube.title, 'the real clipboard holds the YouTube title');
  await page.click(T('pc-tiktok-copy-text'));
  await sleep(150);
  check((await clip(page)) === full(EN.tiktok), 'the real clipboard holds the TikTok caption, a blank line, then its hashtags');
  check((await txt(page, 'pc-tiktok-copy-text'))?.startsWith('Copied'), 'the button says Copied');
  check((await txt(page, 'pc-status')) === 'TikTok caption copied.', 'the copy is announced');
  await page.click(T('pc-instagram-copy-tags'));
  await sleep(150);
  check((await clip(page)) === EN.instagram.hashtags.join(' '), 'the hashtags alone copy');

  console.log('   limits');
  await setField(page, 'pc-youtube-title', 'a'.repeat(101));
  check(await disabled(page, 'pc-youtube-copy-title'), 'a 101 character title cannot be copied');
  check(await disabled(page, 'pc-save'), 'and nothing can be saved');
  check((await txt(page, 'pc-youtube-title-why'))?.includes('101 characters'), `the reason is shown (${await txt(page, 'pc-youtube-title-why')})`);
  check((await page.$eval(T('pc-youtube-title'), (e) => e.getAttribute('aria-invalid'))) === 'true', 'the field is marked invalid');
  check(!(await disabled(page, 'pc-tiktok-copy-text')), 'the other platforms still copy');
  await page.screenshot({ path: path.join(outDir, 'clip-over-limit-desktop-light.png'), fullPage: true });
  await setField(page, 'pc-youtube-title', 'My own title');
  check(!(await disabled(page, 'pc-youtube-copy-title')) && !(await disabled(page, 'pc-save')), 'fixing it turns Copy and Save back on');
  await setField(page, 'pc-youtube-text', 'द'.repeat(1700));
  check(await disabled(page, 'pc-youtube-copy-text') && (await txt(page, 'pc-youtube-text-why'))?.includes('bytes'), 'a Hindi description under 5,000 characters but over 5,000 bytes is stopped');
  await setField(page, 'pc-youtube-text', 'My own description with a link https://example.com/a kept whole.');
  await setField(page, 'pc-tiktok-tags', '#a1 #b2 #c3 #d4 #e5 #f6');
  check(await disabled(page, 'pc-tiktok-copy-text') && (await txt(page, 'pc-tiktok-tags-why'))?.includes('at most 5'), 'six hashtags are stopped');
  await setField(page, 'pc-tiktok-tags', 'lamp wiring,#Vintage');
  await page.$eval(T('pc-tiktok-tags'), (e) => e.dispatchEvent(new Event('blur')));
  await sleep(80);
  check((await val(page, 'pc-tiktok-tags')) === '#lamp #wiring #Vintage', 'the hashtag box is tidied on leaving it');

  console.log('   saving');
  await page.click(T('pc-save'));
  await sleep(300);
  const sent = api.saves[0]?.copy;
  check(sent?.youtube?.title === 'My own title' && sent?.youtube?.text.includes('https://example.com/a') && JSON.stringify(sent?.tiktok?.hashtags) === '["#lamp","#wiring","#Vintage"]' && !('title' in (sent?.tiktok ?? {})), `the save carries exactly what was typed (${JSON.stringify(sent?.tiktok)})`);
  check((await txt(page, 'pc-state'))?.startsWith('Saved'), 'it says Saved');
  check(await disabled(page, 'pc-save'), 'and has nothing more to save');
  await page.click('[data-clip="c1"] button[aria-label="Copy caption"]');
  await sleep(200);
  check((await clip(page)) === full({ text: EN.tiktok.text, hashtags: ['#lamp', '#wiring', '#Vintage'] }), 'the quick copy now gives the saved TikTok caption');

  console.log('   writing again');
  await setField(page, 'pc-tiktok-text', 'An edit I have not saved');
  await page.click(T('pc-write'));
  await sleep(150);
  check(await exists(page, T('pc-confirm')), 'writing again over unsaved changes asks first');
  check(await page.evaluate(() => document.activeElement?.getAttribute('data-testid') === 'pc-confirm-yes'), 'and puts focus on the answer');
  await page.keyboard.press('Escape');
  await sleep(100);
  check(!(await exists(page, T('pc-confirm'))) && (await val(page, 'pc-tiktok-text')) === 'An edit I have not saved', 'Escape keeps the edit');
  check(api.writes.length === 1, 'nothing was written');
  check(errors.length === 0, `no console errors (${errors.join(' | ')})`);
  await ctx.close();
}

console.log('3. A dropped connection: the retry is the same request and uses nothing more');
{
  const api = makeApi({ dropFirstWrite: true });
  const { ctx, page } = await open(api, '/shorts/job1/clips');
  await page.click(T('post-copy-toggle'));
  await sleep(300);
  await page.click(T('pc-write'));
  await sleep(500);
  check((await page.$eval('[data-testid="problem"]', (e) => e.getAttribute('data-kind')).catch(() => null)) === 'offline', 'a dropped answer is shown as offline');
  await page.click('[data-testid="problem"] button');
  await sleep(700);
  check(api.writes.length === 2 && api.writes[0].requestId === api.writes[1].requestId, 'Try again sends the same request id');
  check(api.used === 1, `the day lost one use, not two (${api.used})`);
  check((await txt(page, 'pc-status'))?.includes('Nothing more was used'), 'and says so');
  check((await val(page, 'pc-youtube-title')) === EN.youtube.title, 'the copy is there');
  await ctx.close();
}

console.log('4. Hindi, phone dark, keyboard only');
{
  const api = makeApi({ copy: HI });
  const { ctx, page } = await open(api, '/shorts/job1/clips', { width: 390, height: 844, theme: 'dark' });
  await page.focus(T('post-copy-toggle'));
  await page.keyboard.press('Enter');
  await sleep(300);
  await page.focus(T('pc-write'));
  await page.keyboard.press('Enter');
  await sleep(700);
  check((await val(page, 'pc-tiktok-text')) === HI.tiktok.text, 'Hindi stays Hindi');
  check((await page.$eval(T('pc-tiktok-text'), (e) => e.getAttribute('lang'))) === 'hi', 'the field is marked as Hindi');
  check((await txt(page, 'pc-youtube-text-count')) === `${Buffer.byteLength(full(HI.youtube))} of 5,000 bytes, hashtags included`, 'Hindi is counted in bytes for YouTube');
  check((await txt(page, 'pc-tiktok-text-count')) === `${full(HI.tiktok).length} of 2,200 characters, hashtags included`, 'and in characters for TikTok');
  await page.focus(T('pc-tiktok-copy-text'));
  await page.keyboard.press('Enter');
  await sleep(150);
  check((await clip(page)) === full(HI.tiktok), 'Enter on Copy copies the Hindi caption with its hashtags');
  await layout(page, 'phone dark');
  await page.screenshot({ path: path.join(outDir, 'clip-hindi-phone-dark.png'), fullPage: true });
  await ctx.close();
}

console.log('5. A finished Story video, desktop dark and phone light');
for (const [name, opts] of [['desktop-dark', { theme: 'dark' }], ['phone-light', { width: 390, height: 844 }]]) {
  const api = makeApi();
  const { ctx, page, errors } = await open(api, '/story/plan-1', opts);
  await page.waitForSelector(T('post-copy-toggle'), { timeout: 10000 }).catch(() => null);
  check(await exists(page, T('post-copy-toggle')), `${name}: the finished video has a Post copy button`);
  await page.click(T('post-copy-toggle'));
  await sleep(300);
  check((await page.$eval(T('post-copy-toggle'), (b) => b.getAttribute('aria-expanded'))) === 'true', `${name}: it says it is open`);
  check((await txt(page, 'post-copy'))?.includes('what this story says'), `${name}: it speaks of the story`);
  await page.click(T('pc-write'));
  await sleep(700);
  check((await val(page, 'pc-instagram-text')) === EN.instagram.text, `${name}: the copy for the video is shown`);
  await page.evaluate(() => document.querySelector('app-post-copy')?.scrollIntoView());
  await layout(page, `story ${name}`);
  await page.screenshot({ path: path.join(outDir, `story-${name}.png`), fullPage: true });
  check(errors.length === 0, `${name}: no console errors (${errors.join(' | ')})`);
  await ctx.close();
}

await browser.close();
if (failures.length) {
  console.log(`\n${failures.length} FAILED`);
  process.exit(1);
}
console.log('\nAll checks passed');
