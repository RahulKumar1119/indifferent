// Real-browser check of splitting, merging and moving scenes in the story plan editor (#326). Not part of `ng test`.
//   cd frontend && npx ng build && node e2e/story-structure-verification.mjs [outDir]
// Serves dist/frontend/browser as https://indifferent.fun and a fake API as https://api.indifferent.fun through
// request interception. The fake PUT applies the operations the way the real API does (move, split, merge, then the
// per-scene edits), so this checks what the page sends and what it shows after the answer, not the Go code.
// Checks the request, the order on screen, the picture of each half of a split, keyboard use, focus after a move,
// the live region, the leave warning, a stale save, and the layout on a phone and a desktop in both themes.
// Screenshots go to outDir (default ~/shots303/story-structure), never into the repo. Exit 1 on any failure.
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '..');
const require = createRequire(path.join(root, '/'));
const puppeteer = require('puppeteer');
const outDir = path.resolve(process.argv[2] ?? path.join(process.env.HOME, 'shots303/story-structure'));
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

const texts = [
  'The lighthouse stood alone on the cliff.',
  'Every night the keeper climbed the stairs.',
  'One morning the sea was calm again.',
];
const asset = (id) => ({ provider: 'pixabay', type: 'photo', id, previewUrl: `https://cdn.example/${id}.jpg`, pageUrl: `https://pixabay.example/${id}`, width: 1280, height: 1920, orientation: 'portrait', alt: `Picture ${id}`, author: 'Jo', credit: `Photo ${id} by Jo on Pixabay` });

function makeApi() {
  const s = { revision: 1, puts: [], scenes: texts.map((narration, i) => ({ narration, description: `Scene ${i + 1} picture`, keywords: ['k' + i], mediaType: 'photo', duration: 5, media: { provider: 'pixabay', type: 'photo', id: String(101 + i) } })) };
  const record = () => ({
    planId: 'plan-1', status: 'ready', model: 'test', revision: s.revision, createdAt: '2026-11-15T10:00:00Z', updatedAt: '2026-11-15T10:00:00Z',
    plan: { language: 'en', hook: 'The lighthouse stood alone.', totalSeconds: s.scenes.reduce((t, x) => t + x.duration, 0), scenes: s.scenes },
    media: s.scenes.map((x) => ({ ...x.media, asset: asset(x.media.id) })),
    mediaAttributions: [],
  });
  s.handle = (req) => {
    const p = new URL(req.url()).pathname;
    if (req.method() === 'OPTIONS') return req.respond({ status: 204, headers: CORS });
    if (p === '/auth/refresh') return json(req, 200, { accessToken: 'e2e', expiresIn: 900 });
    if (p === '/auth/me') return json(req, 200, { id: 'u1', email: 'e2e@example.com', name: 'E2E', emailVerified: true });
    if (p === '/story-plans/limits') return json(req, 200, { enabled: true, minStoryChars: 30, maxStoryChars: 5000, minScenes: 2, maxScenes: 10, maxTotalSeconds: 90, perHour: 20 });
    if (p === '/story-videos/limits') return json(req, 200, { enabled: true, maxActive: 2, perDay: 10, maxSeconds: 90, voices: { en: ['Joanna'] } });
    if (p === '/story-videos') return json(req, 200, { videos: [], hasMore: false });
    if (p === '/story-plans/plan-1' && req.method() === 'GET') return json(req, 200, record());
    if (p === '/story-plans/plan-1' && req.method() === 'PUT') {
      const body = JSON.parse(req.postData() || '{}');
      s.puts.push(body);
      if (body.revision !== s.revision) return json(req, 409, { code: 'STALE_REVISION', message: 'This plan was changed somewhere else. Reload it and try again.' });
      let list = s.scenes.map((x) => ({ ...x }));
      const needs = list.map(() => false);
      for (const op of body.ops ?? []) {
        if (op.op === 'move') {
          const [x] = list.splice(op.from - 1, 1); const [f] = needs.splice(op.from - 1, 1);
          list.splice(op.to - 1, 0, x); needs.splice(op.to - 1, 0, f);
        } else if (op.op === 'split') {
          list.splice(op.scene, 0, { ...list[op.scene - 1] });
          needs[op.scene - 1] = true; needs.splice(op.scene, 0, true);
        } else if (op.op === 'merge') {
          list.splice(op.scene - 1, 2, { ...list[op.scene - 1], narration: `${list[op.scene - 1].narration} ${list[op.scene].narration}` });
          needs.splice(op.scene - 1, 2, true);
        } else return json(req, 400, { code: 'VALIDATION_ERROR', message: 'unknown change' });
      }
      if (body.scenes.length !== list.length) return json(req, 400, { code: 'VALIDATION_ERROR', message: 'the edit has the wrong number of scenes' });
      for (let i = 0; i < list.length; i++) {
        const e = body.scenes[i];
        if (needs[i] && e.narration === undefined) return json(req, 400, { code: 'VALIDATION_ERROR', message: `scene ${i + 1}: the narration must be given` });
        if (e.narration !== undefined) list[i] = { ...list[i], narration: e.narration.trim(), duration: 4 };
      }
      s.scenes = list;
      s.revision++;
      return json(req, 200, record());
    }
    return json(req, 404, { code: 'NOT_FOUND' });
  };
  return s;
}

const browser = await puppeteer.launch({ executablePath: '/usr/bin/google-chrome', headless: true, args: ['--no-sandbox'] });
async function open(api, { width = 1280, height = 900, theme = 'light' } = {}) {
  const ctx = await browser.createBrowserContext();
  const page = await ctx.newPage();
  await page.setViewport({ width, height });
  await page.emulateMediaFeatures([{ name: 'prefers-color-scheme', value: theme }]);
  await page.setRequestInterception(true);
  page.on('dialog', (d) => d.accept());
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
const narrations = (page) => page.evaluate(() => [...document.querySelectorAll('app-story-scene-card textarea')].map((t) => t.value));
const pics = (page) => page.evaluate(() => [...document.querySelectorAll('app-story-scene-card')].map((c) => c.querySelector('img')?.getAttribute('src')?.split('/').pop() ?? null));
const press = (page, n, id) => page.evaluate((i, t) => document.querySelectorAll('app-story-scene-card')[i - 1].querySelector(`[data-testid="${t}"]`).click(), n, id);
const status = (page) => page.evaluate(() => document.querySelector('[data-testid="structure-status"]').textContent.trim());
const saveStatus = (page) => page.evaluate(() => document.querySelector('[data-testid="save-status"]').textContent.replace(/\s+/g, ' ').trim());
const save = (page) => page.evaluate(() => [...document.querySelectorAll('button')].find((b) => b.textContent.includes('Save changes')).click());
const cursor = (page, n, at) => page.evaluate((i, a) => { const t = document.querySelectorAll('app-story-scene-card')[i - 1].querySelector('textarea'); t.focus(); t.setSelectionRange(a, a); }, n, at);

console.log('Move');
{
  const api = makeApi();
  const { ctx, page } = await open(api);
  await page.screenshot({ path: path.join(outDir, 'editor-before.png') });
  await press(page, 1, 'move-down');
  await sleep(200);
  const n = await narrations(page);
  check(n[0] === texts[1] && n[1] === texts[0], 'scene 1 moved below scene 2 on screen');
  check((await pics(page)).join() === '102.jpg,101.jpg,103.jpg', `each picture moved with its scene (${(await pics(page)).join()})`);
  check((await status(page)) === 'Scene 1 moved to position 2 of 3.', `a screen reader is told (${await status(page)})`);
  check(await page.evaluate(() => document.activeElement?.id === 'scene-2-move-down'), 'focus follows the moved scene');
  check((await saveStatus(page)).includes('The scene order has changes that are not saved'), `the save bar says the order is unsaved (${await saveStatus(page)})`);
  check(await page.evaluate(() => !document.querySelector('[data-testid="make-video"]')), '"Make the video" waits for the save');
  await page.screenshot({ path: path.join(outDir, 'editor-moved.png') });
  await save(page);
  await sleep(400);
  check(api.puts.length === 1 && JSON.stringify(api.puts[0].ops) === '[{"op":"move","from":1,"to":2}]' && api.puts[0].scenes.length === 3, `one save with the move (${JSON.stringify(api.puts[0])})`);
  check((await narrations(page))[0] === texts[1] && (await saveStatus(page)).includes('Saved'), 'after the save the new order stays and the bar says Saved');
  await page.reload({ waitUntil: 'networkidle2' });
  await sleep(500);
  check((await narrations(page))[0] === texts[1], 'after a reload the saved order is what loads');
  await ctx.close();
}

console.log('Split');
{
  const api = makeApi();
  const { ctx, page } = await open(api);
  await cursor(page, 1, 'The lighthouse stood alone'.length);
  await press(page, 1, 'split');
  await sleep(200);
  const n = await narrations(page);
  check(n.length === 4 && n[0] === 'The lighthouse stood alone' && n[1] === 'on the cliff.', `scene 1 is two scenes (${JSON.stringify(n.slice(0, 2))})`);
  const p = await pics(page);
  check(p[0] === '101.jpg' && p[1] === '101.jpg', 'the second half has the same picture');
  check(await page.evaluate(() => document.querySelectorAll('[data-testid="length-pending"]').length === 2), 'both halves say their length is set when saved');
  check((await status(page)) === 'Scene 1 split into scenes 1 and 2.', 'announced');
  await page.screenshot({ path: path.join(outDir, 'editor-split.png') });
  await save(page);
  await sleep(400);
  check(JSON.stringify(api.puts[0].ops) === '[{"op":"split","scene":1}]', 'the split is sent as one operation');
  check(api.puts[0].scenes.length === 4 && api.puts[0].scenes[0].narration === 'The lighthouse stood alone' && api.puts[0].scenes[1].narration === 'on the cliff.', 'with the narration of both halves');
  check((await narrations(page)).length === 4 && (await pics(page)).slice(0, 2).join() === '101.jpg,101.jpg', 'after the save there are four scenes, both with the picture');
  // a cursor at the end is explained
  await cursor(page, 4, 5000);
  await press(page, 4, 'split');
  await sleep(150);
  check(await page.evaluate(() => /Put the cursor/.test(document.querySelector('[data-testid="structure-problem"]')?.textContent ?? '')), 'a cursor at the end says where to click');
  await ctx.close();
}

console.log('Merge');
{
  const api = makeApi();
  const { ctx, page } = await open(api);
  await press(page, 1, 'merge');
  await sleep(200);
  const n = await narrations(page);
  check(n.length === 2 && n[0] === `${texts[0]} ${texts[1]}`, 'scenes 1 and 2 are one scene with both narrations');
  check((await pics(page))[0] === '101.jpg', 'the merged scene keeps the first picture');
  check(await page.evaluate(() => document.querySelectorAll('app-story-scene-card')[0].querySelector('[data-testid="merge"]').disabled), 'a video at the fewest scenes cannot be merged further');
  await save(page);
  await sleep(400);
  check(JSON.stringify(api.puts[0].ops) === '[{"op":"merge","scene":1}]', 'the merge is sent');
  await ctx.close();
}

console.log('Keyboard, leaving, stale save');
{
  const api = makeApi();
  const { ctx, page } = await open(api);
  await page.evaluate(() => document.querySelectorAll('app-story-scene-card')[0].querySelector('[data-testid="move-down"]').focus());
  await page.keyboard.press('Enter');
  await sleep(200);
  check((await narrations(page))[0] === texts[1], 'Enter on "Move down" moves the scene');
  await page.keyboard.press('Space');
  await sleep(200);
  check((await narrations(page))[1] === texts[2] || (await narrations(page))[2] === texts[0], 'Space on the same button, now on the moved scene, moves it again');
  const unload = await page.evaluate(() => { const e = new Event('beforeunload', { cancelable: true }); window.dispatchEvent(e); return e.defaultPrevented; });
  check(unload, 'leaving with an unsaved order asks first');
  api.revision = 5; // changed elsewhere
  await save(page);
  await sleep(400);
  check(await page.evaluate(() => /changed somewhere else|Reload/i.test(document.querySelector('[data-testid="problem"]')?.textContent ?? '')), 'a stale save says to reload');
  check((await narrations(page))[0] !== texts[0] || (await narrations(page)).length === 3, 'the unsaved order is still on screen');
  await ctx.close();
}

for (const [name, size, theme] of [['phone-light', { width: 390, height: 780 }, 'light'], ['phone-dark', { width: 390, height: 780 }, 'dark'], ['desktop-dark', { width: 1280, height: 900 }, 'dark']]) {
  console.log(`Layout: ${name}`);
  const api = makeApi();
  const { ctx, page } = await open(api, { ...size, theme });
  await press(page, 2, 'move-up');
  await sleep(200);
  const m = await page.evaluate(() => {
    const tools = [...document.querySelectorAll('[data-testid="tools"] button')].map((b) => b.getBoundingClientRect().height);
    return { minH: Math.min(...tools), w: innerWidth, sw: document.documentElement.scrollWidth };
  });
  check(m.minH >= 44, `every control is at least 44 px tall (${Math.round(m.minH)})`);
  check(m.sw <= m.w, 'nothing overflows sideways');
  await page.evaluate(() => document.querySelectorAll('app-story-scene-card')[1].scrollIntoView());
  await page.screenshot({ path: path.join(outDir, `layout-${name}.png`) });
  await ctx.close();
}

await browser.close();
console.log(failures.length ? `\n${failures.length} FAILED` : '\nAll checks passed');
process.exit(failures.length ? 1 : 0);
