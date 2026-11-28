// Real-browser check of the hook line and "Render this clip again" on the AI Shorts clips page (#329). Not part
// of `ng test`.
//   cd frontend && npx ng build && node e2e/clip-hook-verification.mjs [outDir]
// Serves dist/frontend/browser as https://indifferent.fun, a fake API as https://api.indifferent.fun and a real
// test video as https://media.indifferent.test through request interception. The fake API keeps the backend's
// rules: 80 characters, a request id makes a retry free, one render at a time per clip, and a quota.
// Checks:
//   - the preview over a playing video: hidden at 0 s, about half 0.1 s in, full at 1 s, gone after the hook's
//     seconds; with reduced motion, full at once with no fade; it is labelled approximate;
//   - Hindi in the preview and the counter (conjuncts count once);
//   - suggestions and the ranking line; the save; render again with the card badge, the poll, and the new file
//     loading when it finishes;
//   - a dropped connection retried with the same id; used-up renders and suggestions said plainly;
//   - layout on a phone and a desktop in both themes; no page errors.
// The test video is made with ffmpeg in outDir. Screenshots go to outDir (default ~/shots303/329/e2e), never into
// the repo. Exit 1 on any failure.
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '..');
const require = createRequire(path.join(root, '/'));
const puppeteer = require('puppeteer');
const outDir = path.resolve(process.argv[2] ?? path.join(process.env.HOME, 'shots303/329/e2e'));
fs.mkdirSync(outDir, { recursive: true });
const dist = path.join(root, 'dist/frontend/browser');
if (!fs.existsSync(path.join(dist, 'index.html'))) throw new Error('Build first: npx ng build');
const videoPath = path.join(outDir, 'clip.mp4');
if (!fs.existsSync(videoPath)) {
  execFileSync('ffmpeg', ['-nostdin', '-loglevel', 'error', '-y', '-f', 'lavfi', '-i', 'testsrc2=s=540x960:r=30:d=6', '-f', 'lavfi', '-i', 'sine=f=440:d=6',
    '-c:v', 'libx264', '-pix_fmt', 'yuv420p', '-c:a', 'aac', '-shortest', '-movflags', '+faststart', videoPath]);
}
const VIDEO = fs.readFileSync(videoPath);

const SITE = 'https://indifferent.fun';
const API = 'https://api.indifferent.fun';
const MEDIA = 'https://media.indifferent.test';
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.mjs': 'text/javascript', '.css': 'text/css', '.woff2': 'font/woff2', '.woff': 'font/woff', '.json': 'application/json', '.svg': 'image/svg+xml', '.png': 'image/png', '.ico': 'image/x-icon', '.webp': 'image/webp', '.jpg': 'image/jpeg', '.wasm': 'application/wasm' };
const CORS = { 'access-control-allow-origin': SITE, 'access-control-allow-credentials': 'true', 'access-control-allow-headers': 'Content-Type,Authorization', 'access-control-allow-methods': 'GET,POST,PUT,DELETE,OPTIONS' };
const failures = [];
const check = (ok, what) => {
  console.log(`${ok ? '  ok  ' : '  FAIL'} ${what}`);
  if (!ok) failures.push(what);
};
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const json = (req, status, data) => req.respond({ status, headers: { ...CORS, 'content-type': 'application/json' }, body: JSON.stringify(data) });
const CLIPS = [1, 2].map((r) => ({ clipId: 'c' + r, s3Key: 'k', rank: r, score: 0.9, duration: 6, hookText: r === 1 ? 'I lost three clients in one week' : 'यह गलती मत करना', start: r * 60, end: r * 60 + 6 }));

// Counts like the server (models.HookTextLength), for the fake's 80 character rule.
function chars(s) {
  let n = 0, prev = -1, first = true;
  for (const ch of s) {
    const cp = ch.codePointAt(0);
    const joins = !first && (/\p{M}/u.test(ch) || cp === 0x200c || cp === 0x200d || (prev === 0x94d && cp >= 0x915 && cp <= 0x939));
    first = false;
    if (!joins) n++;
    prev = cp;
  }
  return n;
}

function makeApi({ renderLimit = 15, suggestLimit = 20, dropFirstRender = false, renderTakesPolls = 1 } = {}) {
  const s = { hooks: {}, saves: [], renders: [], suggests: [], answered: new Map(), renderUsed: 0, suggestUsed: 0, state: {}, polls: 0, dropped: 0, urlCalls: [] };
  const hookView = (c) => ({
    hook: s.hooks[c] ?? { enabled: false, text: '', style: 'box', position: 'top', seconds: 2.5 }, saved: !!s.hooks[c],
    ranker: CLIPS.find((x) => x.clipId === c)?.hookText, suggestionsEnabled: true, renderEnabled: true,
    suggestRemainingToday: suggestLimit - s.suggestUsed, suggestDailyLimit: suggestLimit, renderRemainingToday: renderLimit - s.renderUsed, renderDailyLimit: renderLimit, maxChars: 80,
    ...(s.state[c] ? { render: s.state[c].view } : {}),
  });
  s.handle = (req) => {
    const p = new URL(req.url()).pathname;
    const m = req.method();
    if (m === 'OPTIONS') return req.respond({ status: 204, headers: CORS });
    if (p === '/auth/refresh') return json(req, 200, { accessToken: 'e2e', expiresIn: 900 });
    if (p === '/auth/me') return json(req, 200, { id: 'u1', email: 'e2e@example.com', name: 'E2E', emailVerified: true });
    if (p === '/shorts/job1/clips') return json(req, 200, { clips: CLIPS });
    if (p === '/shorts/job1/post-copy') return json(req, 200, { enabled: false, platforms: [], saved: {}, dailyLimit: 20 });
    if (p === '/shorts/job1') {
      s.polls++;
      const clipRenders = {};
      for (const [c, st] of Object.entries(s.state)) {
        if (st.view.state === 'rendering' && s.polls >= st.donePoll) st.view = { state: 'done', renders: (st.view.renders ?? 0) + 1, renderedAt: new Date().toISOString() };
        clipRenders[c] = st.view;
      }
      return json(req, 200, { jobId: 'job1', status: 'completed', fileType: 'mp4', sourceDuration: 600, clipEdits: {}, clipRenders });
    }
    const url = /^\/shorts\/job1\/clips\/([^/]+)\/url$/.exec(p);
    if (url) {
      s.urlCalls.push(url[1]);
      return json(req, 200, { url: `${MEDIA}/${url[1]}-${s.urlCalls.length}.mp4` });
    }
    const a = /^\/shorts\/job1\/clips\/([^/]+)\/(hook|hook-suggestions|render)$/.exec(p);
    if (a) {
      const [, c, action] = a;
      const body = JSON.parse(req.postData() || '{}');
      if (action === 'hook' && m === 'GET') return json(req, 200, hookView(c));
      if (action === 'hook' && m === 'PUT') {
        s.saves.push(body);
        if (body.reset) {
          delete s.hooks[c];
          return json(req, 200, { reset: true, hook: { enabled: false, text: '', style: 'box', position: 'top', seconds: 2.5 } });
        }
        if (chars(body.hook.text) > 80) return json(req, 400, { code: 'VALIDATION_ERROR', message: 'the hook text is too long' });
        s.hooks[c] = { ...body.hook, updatedAt: new Date().toISOString() };
        return json(req, 200, { hook: s.hooks[c] });
      }
      if (action === 'hook-suggestions') {
        s.suggests.push(body);
        if (s.answered.has(body.requestId)) return json(req, 200, { ...s.answered.get(body.requestId), replayed: true });
        if (s.suggestUsed >= suggestLimit) return json(req, 429, { code: 'QUOTA_EXCEEDED', message: 'limit', details: { retryAfter: 5400 } });
        s.suggestUsed++;
        const out = { suggestions: ['Nobody tells you this about clients', 'Write it down after every call'], ranker: CLIPS[0].hookText, dropped: 1, remainingToday: suggestLimit - s.suggestUsed, dailyLimit: suggestLimit, replayed: false };
        s.answered.set(body.requestId, out);
        return setTimeout(() => json(req, 200, out), 300);
      }
      if (action === 'render') {
        s.renders.push(body);
        if (s.answered.has(body.requestId)) return json(req, 202, { ...s.answered.get(body.requestId), replayed: true });
        if (s.state[c]?.view.state === 'rendering') return json(req, 409, { code: 'RENDER_IN_PROGRESS', message: 'busy' });
        if (s.renderUsed >= renderLimit) return json(req, 429, { code: 'QUOTA_EXCEEDED', message: 'limit' });
        s.renderUsed++;
        s.state[c] = { view: { state: 'rendering', requestedAt: new Date().toISOString(), renders: s.state[c]?.view.renders ?? 0 }, donePoll: s.polls + renderTakesPolls };
        const out = { render: s.state[c].view, replayed: false, remainingToday: renderLimit - s.renderUsed, dailyLimit: renderLimit };
        s.answered.set(body.requestId, out);
        if (dropFirstRender && s.dropped === 0) {
          s.dropped++;
          return req.abort('connectionreset');
        }
        return json(req, 202, out);
      }
    }
    return json(req, 404, { code: 'NOT_FOUND' });
  };
  return s;
}

function media(req) {
  const range = req.headers().range;
  const headers = { 'access-control-allow-origin': SITE, 'content-type': 'video/mp4', 'accept-ranges': 'bytes' };
  if (range) {
    const [, a, b] = /bytes=(\d+)-(\d*)/.exec(range);
    const start = Number(a);
    const end = b ? Number(b) : VIDEO.length - 1;
    return req.respond({ status: 206, headers: { ...headers, 'content-range': `bytes ${start}-${end}/${VIDEO.length}`, 'content-length': String(end - start + 1) }, body: VIDEO.subarray(start, end + 1) });
  }
  return req.respond({ status: 200, headers: { ...headers, 'content-length': String(VIDEO.length) }, body: VIDEO });
}

const browser = await puppeteer.launch({ executablePath: '/usr/bin/google-chrome', headless: true, args: ['--no-sandbox', '--autoplay-policy=no-user-gesture-required'] });
const consent = JSON.stringify({ version: 1, savedAt: new Date().toISOString(), expiresAt: new Date(Date.now() + 86400000 * 100).toISOString(), choices: { analytics: false, advertising: false, functional: true } });

async function open(api, { width = 1280, height = 1100, theme = 'light', reducedMotion = false } = {}) {
  const ctx = await browser.createBrowserContext();
  const page = await ctx.newPage();
  await page.setViewport({ width, height });
  await page.emulateMediaFeatures([{ name: 'prefers-color-scheme', value: theme }, { name: 'prefers-reduced-motion', value: reducedMotion ? 'reduce' : 'no-preference' }]);
  await page.setRequestInterception(true);
  page.on('request', (req) => {
    const u = new URL(req.url());
    if (u.origin === API) return api.handle(req);
    if (u.origin === MEDIA) return media(req);
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
  await page.goto(SITE + '/shorts/job1/clips', { waitUntil: 'networkidle2', timeout: 30000 });
  await sleep(500);
  return { ctx, page, errors };
}

const T = (id) => `[data-testid="${id}"]`;
const C = (n, sel) => `[data-clip="c${n}"] ${sel}`;
const txt = (page, sel) => page.$eval(sel, (e) => e.textContent.replace(/\s+/g, ' ').trim()).catch(() => null);
const exists = (page, sel) => page.evaluate((s) => !!document.querySelector(s), sel);
async function setText(page, sel, value) {
  await page.$eval(sel, (e, v) => { e.focus(); e.value = v; e.dispatchEvent(new Event('input', { bubbles: true })); }, value);
  await sleep(120);
}
async function clickText(page, scope, label) {
  return page.evaluate((s, l) => {
    const b = Array.from(document.querySelectorAll(`${s} button`)).find((x) => x.textContent.replace(/\s+/g, ' ').trim().includes(l));
    if (b) b.click();
    return !!b;
  }, scope, label);
}
async function openHook(page, n) {
  await page.click(C(n, T('hook-toggle')));
  await sleep(400);
}
/** Starts the card's video from a time and samples the preview's opacity a moment later. */
async function opacityAt(page, n, t) {
  return page.evaluate(async (sel, t) => {
    let v = document.querySelector(`${sel} video`);
    if (!v) {
      document.querySelector(`${sel} .cover`)?.click();
      for (let i = 0; i < 50 && !(v = document.querySelector(`${sel} video`)); i++) await new Promise((r) => setTimeout(r, 100));
    }
    if (!v) return -1;
    v.muted = true;
    if (v.readyState < 2) await new Promise((r) => v.addEventListener('loadeddata', r, { once: true }));
    v.pause();
    v.currentTime = t;
    await new Promise((r) => v.addEventListener('seeked', r, { once: true }));
    await v.play();
    await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
    const o = Number(getComputedStyle(document.querySelector(`${sel} .hook-ov`)).opacity);
    const at = v.currentTime;
    v.pause();
    return { o, at };
  }, `[data-clip="c${n}"]`, t);
}
async function layout(page, label) {
  const r = await page.evaluate(() => {
    const host = document.querySelector('app-clip-hook');
    const over = document.documentElement.scrollWidth - document.documentElement.clientWidth;
    const small = Array.from(host?.querySelectorAll('button, input[type=text]') ?? []).filter((e) => e.getBoundingClientRect().height < 44 && e.offsetParent).map((e) => e.outerHTML.slice(0, 70));
    const box = host?.getBoundingClientRect();
    const out = Array.from(host?.querySelectorAll('*') ?? []).filter((e) => { const b = e.getBoundingClientRect(); return b.width > 0 && box && (b.right > box.right + 1 || b.left < box.left - 1); }).length;
    const ov = document.querySelector('.hook-ov')?.getBoundingClientRect();
    const player = document.querySelector('.hook-ov')?.closest('.player')?.getBoundingClientRect();
    const inside = !ov || !player || (ov.left >= player.left - 1 && ov.right <= player.right + 1 && ov.top >= player.top - 1 && ov.bottom <= player.bottom + 1);
    const el = document.querySelector('.hook-ov');
    let lines = 0, fontPx = 0, widthPx = 0;
    if (el) {
      const cs = getComputedStyle(el);
      const pad = parseFloat(cs.paddingTop) + parseFloat(cs.paddingBottom);
      lines = Math.round((el.getBoundingClientRect().height - pad) / parseFloat(cs.lineHeight));
      fontPx = parseFloat(cs.fontSize);
      widthPx = el.closest('.player').getBoundingClientRect().width;
    }
    return { over, small, out, inside, lines, fontPx, widthPx };
  });
  check(r.over <= 0, `${label}: no sideways scroll (${r.over}px)`);
  check(r.small.length === 0, `${label}: every button and box is at least 44 px tall ${r.small.length ? JSON.stringify(r.small) : ''}`);
  check(r.out === 0, `${label}: nothing spills out of the panel (${r.out})`);
  check(r.inside, `${label}: the preview stays inside the video`);
  if (r.lines) {
    check(r.lines <= 2, `${label}: the preview takes at most two lines (${r.lines})`);
    check(r.fontPx >= (r.widthPx * 40) / 1080 - 0.5, `${label}: and is no smaller than the render's floor (${r.fontPx.toFixed(1)}px on a ${r.widthPx.toFixed(0)}px player)`);
  }
}

console.log('1. Panel, preview, Hindi, desktop light');
{
  const api = makeApi();
  const { ctx, page, errors } = await open(api);
  check((await page.$$(T('hook-toggle'))).length === 2, 'each clip has a Hook button');
  check((await page.$eval(C(1, T('hook-toggle')), (b) => b.getAttribute('aria-label'))) === 'Edit the hook for clip 1', 'the button says what it is for');
  await openHook(page, 1);
  check(await exists(page, C(1, T('clip-hook'))), 'the panel opens under clip 1');
  check(!(await exists(page, C(1, T('hook-preview')))), 'off by default: no preview over the clip');
  check((await txt(page, C(1, T('hook-ideas'))))?.includes('From the ranking'), 'the ranking line is offered');
  await page.click(C(1, T('hook-enabled')));
  await setText(page, C(1, T('hook-text')), 'क्षत्रिय ज्ञान कि पहले जानिए');
  // क्ष त्रि य, ज्ञा न, कि, प ह ले, जा नि ए: 12 letters with their signs and conjuncts, and 4 spaces.
  check((await txt(page, C(1, '.count')))?.startsWith('16 / 80'), `Hindi conjuncts and vowel signs count with their letter (${await txt(page, C(1, '.count'))})`);
  check((await page.$eval(C(1, T('hook-preview')), (e) => e.getAttribute('lang'))) === 'hi', 'the preview is marked Hindi');
  check((await txt(page, C(1, 'app-clip-hook-preview .tag'))) === 'Hook preview, approximate', 'the preview is labelled approximate');
  check((await txt(page, C(1, T('clip-hook'))))?.includes('The preview on the clip is approximate'), 'the panel says the preview is approximate');
  const paused = await page.$eval(C(1, '.hook-ov'), (e) => Number(getComputedStyle(e).opacity));
  check(paused === 1, `before playing, the preview is fully shown (${paused})`);
  await page.screenshot({ path: path.join(outDir, 'hook-desktop-light.png'), fullPage: true });

  const at0 = await opacityAt(page, 1, 0);
  const at01 = await opacityAt(page, 1, 0.08);
  const at1 = await opacityAt(page, 1, 1);
  const at3 = await opacityAt(page, 1, 3);
  check(at0.o < 0.4, `starting at 0 s the preview is fading in (${JSON.stringify(at0)})`);
  check(at01.o > 0.2 && at01.o < 0.95, `0.1 s in it is part way (${JSON.stringify(at01)})`);
  check(at1.o === 1, `at 1 s it is fully shown (${JSON.stringify(at1)})`);
  check(at3.o === 0, `after 2.5 s it is gone (${JSON.stringify(at3)})`);
  await page.evaluate(() => document.querySelector('[data-clip="c1"] input[type=radio][value="band"]').click());
  await clickText(page, C(1, T('clip-hook')), 'Middle');
  await sleep(150);
  const cls = await page.$eval(C(1, T('hook-preview')), (e) => e.className);
  check(cls.includes('band') && cls.includes('middle'), `the look and position follow the choices (${cls})`);
  const bg = await page.$eval(C(1, T('hook-preview')), (e) => getComputedStyle(e).backgroundColor);
  check(bg === 'rgb(255, 214, 10)', `the band is the accent yellow (${bg})`);
  await page.evaluate(() => { const v = document.querySelector('[data-clip="c1"] video'); v.currentTime = 1; });
  await sleep(300);
  await page.screenshot({ path: path.join(outDir, 'hook-band-middle-desktop-light.png'), clip: await page.$eval('[data-clip="c1"]', (e) => { const b = e.getBoundingClientRect(); return { x: b.x, y: b.y + window.scrollY, width: b.width, height: Math.min(b.height, 900) }; }) });
  await layout(page, 'desktop light');

  await setText(page, C(1, T('hook-text')), 'क्ष'.repeat(81));
  check(await page.$eval(C(1, T('hook-save')), (b) => b.disabled), 'over 80 characters cannot be saved');
  check((await txt(page, C(1, '.why')))?.includes('The limit is 80'), 'the limit is said');
  check(errors.length === 0, `no page errors ${errors.join(' | ')}`);
  await ctx.close();
}

console.log('2. Suggestions, save, render again, the badge and the new file');
{
  const api = makeApi({ renderTakesPolls: 2 });
  const { ctx, page, errors } = await open(api);
  await openHook(page, 1);
  await page.click(C(1, T('hook-suggest')));
  await sleep(700);
  check(api.suggests.length === 1 && /^[A-Za-z0-9_-]{16,64}$/.test(api.suggests[0].requestId), 'suggestions send only a request id');
  check((await txt(page, C(1, T('hook-ideas'))))?.includes('Write it down after every call'), 'the suggestions are listed');
  check((await txt(page, C(1, T('clip-hook'))))?.includes('19 of 20 left today'), 'the writing allowance left is shown');
  check((await txt(page, C(1, T('clip-hook'))))?.includes('1 more line was left out'), 'what was left out is said');
  check(!(await page.$eval(C(1, T('hook-render-again')), (b) => b.disabled)), 'with nothing unsaved, render is available (trim and caption fixes need no hook)');
  await clickText(page, C(1, T('hook-ideas')), 'Use');
  await sleep(150);
  check((await page.$eval(C(1, T('hook-text')), (e) => e.value)) === CLIPS[0].hookText, 'Use puts the line in the box');
  check((await txt(page, C(1, T('clip-hook'))))?.includes('Save the hook first'), 'it asks for a save before rendering');
  await page.click(C(1, T('hook-save')));
  await sleep(300);
  check(api.saves.length === 1 && api.saves[0].hook.enabled === true && api.saves[0].hook.text === CLIPS[0].hookText, `the hook is saved (${JSON.stringify(api.saves[0])})`);
  const urlsBefore = api.urlCalls.filter((c) => c === 'c1').length;
  await page.click(C(1, T('hook-render-again')));
  await sleep(400);
  check(api.renders.length === 1, 'one render request');
  check(await exists(page, C(1, T('clip-rendering'))), 'the card shows Rendering again');
  check((await txt(page, C(1, T('hook-render'))))?.includes('14 of 15 left today'), 'the render allowance left is shown');
  check(await page.$eval(C(1, T('hook-render-again')), (b) => b.disabled), 'a second render cannot start while one runs');
  await page.screenshot({ path: path.join(outDir, 'render-again-rendering.png'), fullPage: true });
  await sleep(21500);
  check(!(await exists(page, C(1, T('clip-rendering')))), 'the badge goes when the render is done');
  check((await txt(page, C(1, T('hook-render'))))?.includes('Rendered again'), 'the panel says it was rendered again');
  check(api.urlCalls.filter((c) => c === 'c1').length > urlsBefore, 'the new file is loaded');
  const pollsAfter = api.polls;
  await sleep(11000);
  check(api.polls === pollsAfter, 'nothing rendering: the page stops asking');
  check(errors.length === 0, `no page errors ${errors.join(' | ')}`);
  await ctx.close();
}

console.log('3. A dropped connection, and used-up allowances');
{
  const api = makeApi({ dropFirstRender: true, renderLimit: 1, suggestLimit: 0 });
  api.hooks.c1 = { enabled: true, text: 'Saved', style: 'box', position: 'top', seconds: 2.5 };
  const { ctx, page } = await open(api);
  await openHook(page, 1);
  await page.click(C(1, T('hook-render-again')));
  await sleep(400);
  check((await txt(page, C(1, T('hook-render'))))?.includes("You're offline"), 'a dropped connection is said');
  await clickText(page, C(1, T('hook-render')), 'Try again');
  await sleep(400);
  check(api.renders.length === 2 && api.renders[0].requestId === api.renders[1].requestId, 'the retry reuses the request id');
  check(api.renderUsed === 1, 'the render was charged once');
  check(await exists(page, C(1, T('clip-rendering'))), 'and it is rendering');
  await openHook(page, 1);
  await openHook(page, 2);
  await page.click(C(2, T('hook-render-again')));
  await sleep(400);
  const used = await txt(page, C(2, T('hook-render')));
  check(used?.includes("You've used today's renders") && used.includes('stays as it is'), `used-up renders are said plainly (${used})`);
  await page.click(C(2, T('hook-suggest')));
  await sleep(400);
  const sug = await txt(page, C(2, T('clip-hook')));
  check(sug?.includes("You've used today's suggestions") && sug.includes('in 1 hour 30 minutes'), 'used-up suggestions are said, with the wait');
  check(sug?.includes('यह गलती मत करना'), 'the ranking line stays available');
  await page.screenshot({ path: path.join(outDir, 'quota-desktop-light.png'), fullPage: true });
  await ctx.close();
}

console.log('4. Reduced motion: no fade');
{
  const api = makeApi();
  const { ctx, page } = await open(api, { reducedMotion: true });
  await openHook(page, 1);
  await page.click(C(1, T('hook-enabled')));
  await setText(page, C(1, T('hook-text')), 'Watch this');
  const at = await opacityAt(page, 1, 0.02);
  const after = await opacityAt(page, 1, 3);
  check(at.o === 1, `with reduced motion it shows at once (${JSON.stringify(at)})`);
  check(after.o === 0, `and is gone after its seconds (${JSON.stringify(after)})`);
  await ctx.close();
}

console.log('5. Layout: phone and desktop, light and dark');
for (const [label, opts] of [['phone light', { width: 390, height: 844 }], ['phone dark', { width: 390, height: 844, theme: 'dark' }], ['desktop dark', { width: 1280, height: 1100, theme: 'dark' }]]) {
  const api = makeApi();
  const { ctx, page, errors } = await open(api, opts);
  await openHook(page, 2);
  await page.click(C(2, T('hook-enabled')));
  await setText(page, C(2, T('hook-text')), 'यह बात कोई नहीं बताता कि पहले साल में सबसे बड़ी गलती क्या होती है');
  await page.click(C(2, T('hook-suggest')));
  await sleep(600);
  await layout(page, label);
  await page.screenshot({ path: path.join(outDir, `hook-${label.replace(' ', '-')}.png`), fullPage: true });
  check(errors.length === 0, `${label}: no page errors ${errors.join(' | ')}`);
  await ctx.close();
}

await browser.close();
console.log(failures.length ? `\n${failures.length} FAILED` : '\nall checks passed');
console.log(`screenshots: ${outDir}`);
process.exit(failures.length ? 1 : 0);
