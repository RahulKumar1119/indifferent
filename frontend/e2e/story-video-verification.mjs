// Clicks through the "Make the video" panel in a real browser against a fake
// API, and prints what happened to each control. Run with the dev server up:
//
//   npx ng serve --port 4300 --configuration development
//   STORY_URL=http://localhost:4300 STORY_VIDEO_FILE=/path/to/small.mp4 node e2e/story-video-verification.mjs
//
// STORY_VIDEO_FILE is a real MP4 the fake API serves as "the finished video", so
// the check that it downloads and plays is real. AXE_PATH, if set, is the path to
// axe.min.js; without it the accessibility scan is skipped and says so.
import assert from 'node:assert/strict';
import { existsSync, mkdtempSync, readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import puppeteer from 'puppeteer';

const BASE_URL = process.env.STORY_URL ?? 'http://localhost:4300';
const API = 'http://localhost:3000';
const VIDEO_FILE = process.env.STORY_VIDEO_FILE;
const AXE_PATH = process.env.AXE_PATH;
const SHOTS = process.env.STORY_SHOTS ?? '/tmp';
assert.ok(VIDEO_FILE && existsSync(VIDEO_FILE), 'set STORY_VIDEO_FILE to a small MP4');

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const evidence = [];
const note = (line) => {
  evidence.push(line);
  console.log('  ' + line);
};

const CORS = {
  'access-control-allow-origin': BASE_URL,
  'access-control-allow-credentials': 'true',
  'access-control-allow-headers': 'authorization,content-type',
  'access-control-allow-methods': 'GET,POST,PUT,DELETE,OPTIONS',
};
const json = (request, status, body, extra = {}) =>
  request.respond({ status, headers: { ...CORS, 'content-type': 'application/json', ...extra }, body: JSON.stringify(body) });

// A 1x1 picture, so no scene needs the network.
const PIXEL = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=';
const asset = (id, type = 'photo') => ({
  provider: 'pixabay', type, id, previewUrl: PIXEL, pageUrl: `https://pixabay.com/${id}`, width: 1080, height: 1920,
  orientation: 'portrait', alt: `Picture ${id}`, author: 'Jo Doe', credit: `Photo ${id} by Jo Doe on Pixabay`,
});

/** What the fake server knows. Each scenario resets it. */
let world;
function reset(over = {}) {
  world = {
    language: 'en',
    revision: 2,
    noPictureOnScene3: false,
    videos: [],
    // How a video advances each time the page asks about it.
    script: [
      { status: 'queued', progress: 0 },
      { status: 'preparing', progress: 12 },
      { status: 'narrating', progress: 40 },
      { status: 'rendering', progress: 70 },
      { status: 'rendering', progress: 92 },
      { status: 'completed', progress: 100 },
    ],
    failWith: null,
    requests: [],
    ...over,
  };
}

function planRecord() {
  const scenes = [1, 2, 3].map((n) => ({
    narration: ['The lighthouse stood alone on the cliff.', 'Every night the keeper climbed the stairs.', 'One morning the sea was calm again.'][n - 1],
    description: `Scene ${n}`, keywords: ['lighthouse'], mediaType: 'photo', duration: 5,
    ...(n === 3 && world.noPictureOnScene3 ? {} : { media: { provider: 'pixabay', type: 'photo', id: String(100 + n) } }),
  }));
  return {
    planId: 'plan-1', status: 'ready', revision: world.revision, createdAt: '2026-10-31T10:00:00Z', updatedAt: '2026-10-31T10:00:00Z',
    plan: { language: world.language, hook: 'The lighthouse stood alone.', totalSeconds: 15, scenes },
    media: scenes.map((s, i) => (s.media ? { ...s.media, asset: asset(String(101 + i)) } : null)),
    mediaAttributions: [{ provider: 'pixabay', text: 'Photos provided by Pixabay', url: 'https://pixabay.com' }],
  };
}

function view(v) {
  const { step, ...pub } = v;
  return pub;
}

function advance(v) {
  if (v.status === 'completed' || v.status === 'failed') return v;
  if (world.failWith && v.step >= 2) {
    Object.assign(v, { status: 'failed', failedStage: 'preparing', ...world.failWith, updatedAt: new Date().toISOString() });
    return v;
  }
  v.step = Math.min(v.step + 1, world.script.length - 1);
  const s = world.script[v.step];
  Object.assign(v, s, { updatedAt: new Date().toISOString() });
  if (s.status === 'completed') Object.assign(v, { totalSeconds: 14.6, completedAt: new Date().toISOString() });
  return v;
}

function fakeApi(request) {
  const url = new URL(request.url());
  const path = url.pathname;
  const method = request.method();
  if (method === 'OPTIONS') return request.respond({ status: 204, headers: CORS });
  world.requests.push(`${method} ${path}${url.search}`);

  if (path === '/auth/refresh') return json(request, 200, { accessToken: 'e2e', refreshToken: 'e2e', expiresIn: 3600 });
  if (path === '/auth/me') return json(request, 200, { id: 'u1', email: 'e2e@example.com', name: 'E2E' });
  if (path === '/story-plans/limits') return json(request, 200, { enabled: true, minStoryChars: 30, maxStoryChars: 5000, minScenes: 2, maxScenes: 10, maxTotalSeconds: 90, perHour: 20 });
  if (path === '/story-plans/plan-1' && method === 'GET') return json(request, 200, planRecord());

  if (path === '/story-videos/limits') {
    return json(request, 200, { enabled: true, maxActive: 2, perDay: 10, maxSeconds: 90, voices: { en: ['Joanna', 'Matthew', 'Amy', 'Brian', 'Aditi'], hi: ['Aditi'] } });
  }
  if (path === '/story-videos' && method === 'GET') {
    return json(request, 200, { videos: [...world.videos].reverse().map(view), hasMore: false });
  }
  if (path === '/story-videos' && method === 'POST') {
    const body = JSON.parse(request.postData() ?? '{}');
    world.lastCreate = body;
    if (world.videos.some((v) => v.status !== 'completed' && v.status !== 'failed')) {
      const running = world.videos.find((v) => v.status !== 'completed' && v.status !== 'failed');
      return json(request, 409, { code: 'ALREADY_RUNNING', message: 'A video is already being made from this plan.', details: { jobId: running.jobId } });
    }
    const now = new Date().toISOString();
    const v = {
      jobId: `job-${world.videos.length + 1}`, planId: body.planId, planRevision: body.revision, status: 'queued', progress: 0,
      language: world.language, voice: body.voice ?? 'Joanna', caption: body.caption ?? {}, createdAt: now, updatedAt: now, step: 0,
    };
    world.videos.push(v);
    return json(request, 202, view(v));
  }
  const m = /^\/story-videos\/([^/]+)(?:\/(cancel|download))?$/.exec(path);
  if (m) {
    const v = world.videos.find((x) => x.jobId === m[1]);
    if (!v) return json(request, 404, { code: 'NOT_FOUND', message: 'Video not found' });
    if (!m[2] && method === 'GET') return json(request, 200, view(advance(v)));
    if (m[2] === 'cancel' && method === 'POST') {
      if (v.status !== 'completed' && v.status !== 'failed') {
        Object.assign(v, { failedStage: v.status, status: 'failed', errorCode: 'CANCELLED', error: 'You cancelled this video.' });
      }
      return json(request, 200, view(v));
    }
    if (m[2] === 'download' && method === 'GET') {
      if (v.status !== 'completed') return json(request, 409, { code: 'NOT_READY', message: 'This video is not finished yet' });
      return json(request, 200, { url: `${API}/files/${v.jobId}.mp4`, filename: 'story-video.mp4', expiresInSeconds: 900 });
    }
    if (!m[2] && method === 'DELETE') {
      if (v.status !== 'completed' && v.status !== 'failed') return json(request, 409, { code: 'STILL_RUNNING', message: 'Cancel it first' });
      world.videos = world.videos.filter((x) => x !== v);
      return request.respond({ status: 204, headers: CORS });
    }
  }
  if (path.startsWith('/files/')) {
    return request.respond({
      status: 200,
      headers: { ...CORS, 'content-type': 'video/mp4', 'content-disposition': 'attachment; filename="story-video.mp4"', 'accept-ranges': 'none' },
      body: readFileSync(VIDEO_FILE),
    });
  }
  return json(request, 404, { code: 'NOT_FOUND', message: 'unknown ' + path });
}

const chrome = process.env.CHROME_PATH ?? ['/usr/bin/google-chrome', '/usr/bin/chromium', '/snap/bin/chromium'].find(existsSync);
const browser = await puppeteer.launch({ headless: true, ...(chrome ? { executablePath: chrome } : {}), args: ['--no-sandbox', '--disable-dev-shm-usage', '--autoplay-policy=no-user-gesture-required'] });

const problems = [];
async function openPage(theme = 'light', width = 1280) {
  const page = await browser.newPage();
  page.on('console', (msg) => {
    if (['error', 'warning'].includes(msg.type())) {
      const text = msg.text();
      if (/Failed to load resource.*(404|net::ERR)/.test(text)) return;
      problems.push(`${msg.type()}: ${text}`);
    }
  });
  page.on('pageerror', (e) => problems.push(`pageerror: ${e.message}`));
  await page.evaluateOnNewDocument((t) => {
    localStorage.setItem('indifferent.refreshToken', 'e2e-refresh-token');
    localStorage.setItem('app-theme', t);
  }, theme);
  await page.setViewport({ width, height: 900, deviceScaleFactor: 1 });
  await page.setRequestInterception(true);
  page.on('request', (request) => {
    const u = new URL(request.url());
    if (u.origin === API) return void fakeApi(request);
    if (u.hostname.includes('fonts.g')) return void request.abort(); // web fonts are not what is being checked
    return void request.continue();
  });
  return page;
}

const goto = async (page) => {
  await page.goto(`${BASE_URL}/story/plan-1`, { waitUntil: 'networkidle2' });
  await page.waitForSelector('[data-testid="video-panel"]', { timeout: 20000 });
  await page.waitForFunction(() => !document.querySelector('[data-testid="loading"]'), { timeout: 10000 });
};

const text = (page, sel) => page.$eval(sel, (e) => e.textContent.replace(/\s+/g, ' ').trim());
const click = (page, testid) => page.$eval(`[data-testid="${testid}"]`, (b) => b.click());
const exists = (page, sel) => page.$(sel).then(Boolean);
const active = (page) => page.evaluate(() => document.activeElement?.id || document.activeElement?.getAttribute('data-testid') || document.activeElement?.tagName);
async function waitFor(page, fn, what, timeout = 30000) {
  try {
    await page.waitForFunction(fn, { timeout });
  } catch {
    throw new Error('timed out waiting for: ' + what);
  }
}
const shot = (page, name) => page.screenshot({ path: join(SHOTS, `story-video-${name}.png`), fullPage: true });

async function axeScan(page, label) {
  if (!AXE_PATH) return;
  await page.evaluate(readFileSync(AXE_PATH, 'utf8'));
  const violations = await page.evaluate(async () => {
    const r = await axe.run(document.querySelector('[data-testid="video-panel"]'), { runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21aa'] } });
    return r.violations.map((v) => ({ id: v.id, nodes: v.nodes.map((n) => n.target.join(' ')).slice(0, 3), help: v.help }));
  });
  assert.deepEqual(violations, [], `axe (${label}): ${JSON.stringify(violations)}`);
  note(`axe, ${label}: no WCAG 2.1 A/AA violations in the panel`);
}

async function checkLayout(page, label) {
  const m = await page.evaluate(() => ({ sw: document.documentElement.scrollWidth, iw: window.innerWidth }));
  assert.ok(m.sw <= m.iw + 1, `${label}: page scrolls sideways (${m.sw} > ${m.iw})`);
  const short = await page.evaluate(() =>
    [...document.querySelectorAll('[data-testid="video-panel"] button, [data-testid="video-panel"] label:has(input), [data-testid="video-panel"] select')]
      .filter((e) => e.offsetParent !== null)
      .map((e) => ({ t: (e.textContent || e.id).trim().replace(/\s+/g, ' ').slice(0, 40), h: Math.round(e.getBoundingClientRect().height) }))
      .filter((x) => x.h < 44),
  );
  assert.deepEqual(short, [], `${label}: controls under 44px: ${JSON.stringify(short)}`);
  const clipped = await page.evaluate(() => {
    const panel = document.querySelector('[data-testid="video-panel"]').getBoundingClientRect();
    return [...document.querySelectorAll('[data-testid="video-panel"] *')]
      .filter((e) => e.offsetParent !== null && e.getBoundingClientRect().right > panel.right + 1 && !e.closest('video'))
      .map((e) => e.tagName + '.' + (e.className || '').toString().slice(0, 30));
  });
  assert.deepEqual(clipped, [], `${label}: content leaves the panel: ${clipped.join(', ')}`);
}

try {
  // ---------------------------------------------------------------- 1. layout and the idle form
  console.log('1. The form before any video, at four widths, in both themes');
  for (const theme of ['light', 'dark']) {
    for (const width of [320, 375, 768, 1280]) {
      reset();
      const page = await openPage(theme, width);
      await goto(page);
      await checkLayout(page, `${theme} ${width}px, form`);
      if (width === 375 || width === 1280) await shot(page, `form-${theme}-${width}`);
      if (width === 375) await axeScan(page, `form, ${theme}, 375px`);
      await page.close();
    }
    note(`form, ${theme}: no sideways scroll, no control under 44px, nothing leaves the panel at 320, 375, 768, 1280px`);
  }

  // ---------------------------------------------------------------- 2. make, with the keyboard only
  console.log('2. Making a video with the keyboard only');
  reset();
  let page = await openPage('light', 1280);
  await goto(page);
  assert.equal(await text(page, '#video-heading'), 'Make the video');
  assert.equal(await page.$eval('[data-testid="make"]', (b) => b.disabled), false);
  // The controls can be reached with Tab alone, in reading order: voice menu,
  // then the caption group (one stop), then the main button.
  await page.focus('#video-voice');
  await page.keyboard.press('Tab');
  assert.equal(await page.evaluate(() => document.activeElement?.name + ':' + document.activeElement?.checked), 'video-caption:true', 'Tab from the voice menu did not reach the checked caption look');
  await page.keyboard.press('Tab');
  assert.equal(await page.evaluate(() => document.activeElement?.getAttribute('data-testid')), 'make', 'Tab from the caption group did not reach the main button');
  note('Tab order: voice menu -> caption looks (one stop) -> "Make the video"');
  await page.focus('#video-voice');
  await page.keyboard.press('ArrowDown'); // Joanna -> Matthew
  assert.equal(await page.$eval('#video-voice', (s) => s.value), 'Matthew', 'ArrowDown did not change the voice');
  note('voice menu: ArrowDown changes the voice (Joanna -> Matthew)');
  // Move to the caption group and change with arrows.
  await page.focus('input[value="bold"]');
  await page.keyboard.press('ArrowDown'); // radios in one group move with arrows
  const look = await page.$eval('input[name="video-caption"]:checked', (r) => r.value);
  assert.notEqual(look, 'bold', 'arrow key did not move the caption look');
  note(`caption looks: ArrowDown moves the choice (bold -> ${look})`);
  // The main button shows a visible focus ring.
  await page.focus('[data-testid="make"]');
  const ring = await page.$eval('[data-testid="make"]', (b) => {
    const s = getComputedStyle(b);
    return { style: s.outlineStyle, width: parseFloat(s.outlineWidth), color: s.outlineColor };
  });
  assert.ok(ring.style !== 'none' && ring.width >= 2, `no visible focus ring on the main button: ${JSON.stringify(ring)}`);
  note(`main button: focus ring ${ring.width}px ${ring.style} ${ring.color}`);
  await page.keyboard.press('Enter');
  await waitFor(page, () => document.querySelector('[data-testid="active"]'), 'the progress view');
  assert.deepEqual(world.lastCreate, { planId: 'plan-1', revision: 2, voice: 'Matthew', caption: { preset: look } });
  note(`Enter on "Make the video" -> POST /story-videos with ${JSON.stringify(world.lastCreate)}`);
  await sleep(150);
  assert.equal(await text(page, '[data-state-heading]'), 'Making your video');
  assert.ok(await page.evaluate(() => document.activeElement?.hasAttribute('data-state-heading')), 'focus did not move to the progress heading');
  note('focus moved to the "Making your video" heading');

  // ---------------------------------------------------------------- 3. progress, for real
  console.log('3. Progress');
  await shot(page, 'active-light-1280');
  const seen = [];
  const bar = () => page.$eval('progress', (p) => p.value);
  for (let i = 0; i < 40 && !(await exists(page, '[data-testid="done"]')); i++) {
    seen.push(await bar().catch(() => null));
    await sleep(1500);
  }
  await waitFor(page, () => document.querySelector('[data-testid="done"]'), 'the finished view', 40000);
  const distinct = [...new Set(seen.filter((x) => x !== null))];
  assert.ok(distinct.length >= 3, `the bar did not move through several values: ${distinct}`);
  assert.deepEqual(distinct, [...distinct].sort((a, b) => a - b), 'the bar went backwards');
  note(`progress bar moved ${distinct.join(' -> ')} then the view became "Your video is ready"`);
  assert.equal(await text(page, '[data-testid="announcement"]'), 'Your video is ready.');
  note('completion announced in the status region: "Your video is ready."');

  // ---------------------------------------------------------------- 4. download and play
  console.log('4. The finished video');
  assert.match(await text(page, '[data-testid="finished"] p'), /^15 seconds · Matthew voice · \w[\w ]+ captions$/);
  note(`facts: ${await text(page, '[data-testid="finished"] p')}`);
  const dir = mkdtempSync(join(tmpdir(), 'story-dl-'));
  const cdp = await browser.target().createCDPSession();
  await cdp.send('Browser.setDownloadBehavior', { behavior: 'allow', downloadPath: dir });
  await click(page, 'download');
  for (let i = 0; i < 40 && !readdirSync(dir).some((f) => f.endsWith('.mp4')); i++) await sleep(250);
  const files = readdirSync(dir);
  assert.deepEqual(files, ['story-video.mp4'], `download folder holds ${files}`);
  const bytes = readFileSync(join(dir, files[0]));
  assert.ok(bytes.length > 1000 && bytes.subarray(4, 8).toString() === 'ftyp', 'the downloaded file is not an MP4');
  note(`"Download MP4" -> saved story-video.mp4 (${bytes.length} bytes, MP4 header "ftyp")`);

  await click(page, 'watch');
  await waitFor(page, () => document.querySelector('[data-testid="player"]'), 'the player');
  await waitFor(page, () => { const v = document.querySelector('video'); return v && v.readyState >= 1 && v.duration > 0; }, 'the video to load', 15000);
  const media = await page.$eval('video', (v) => ({ d: v.duration, w: v.videoWidth, h: v.videoHeight, err: v.error?.code ?? null, auto: v.autoplay, paused: v.paused }));
  assert.equal(media.err, null);
  assert.ok(media.d > 1.5 && media.w === 270 && media.h === 480, JSON.stringify(media));
  assert.equal(media.paused, true, 'the video started by itself');
  note(`"Watch it here" -> the browser decoded it: ${media.w}x${media.h}, ${media.d.toFixed(1)} s, not playing until pressed`);
  await page.$eval('video', (v) => v.play());
  await sleep(700);
  assert.ok(await page.$eval('video', (v) => v.currentTime > 0.3), 'the video did not play');
  note('pressing play plays it (currentTime advanced)');
  await shot(page, 'done-light-1280');
  await axeScan(page, 'finished, with the player, light 1280px');
  await click(page, 'watch');
  assert.equal(await exists(page, '[data-testid="player"]'), false);
  note('"Hide the video" removes the player');

  // ---------------------------------------------------------------- 5. another version, then delete
  console.log('5. Another version, and deleting');
  await click(page, 'another');
  await waitFor(page, () => document.querySelector('[data-testid="make-form"]'), 'the form for another version');
  const terracotta = await page.$$eval('button', (bs) =>
    bs.filter((b) => b.className.includes('bg-[#BC5227]') && b.offsetParent).map((b) => ({ label: b.textContent.trim(), disabled: b.disabled })),
  );
  const primaries = terracotta.filter((b) => !b.disabled).map((b) => b.label);
  assert.deepEqual(primaries, ['Make this version'], 'more than one main action on screen');
  note(`terracotta buttons seen: ${JSON.stringify(terracotta)} (a disabled one is not an action)`);
  note('with the form open, "Make this version" is the only main action');
  await page.$$eval('button', (bs) => bs.find((b) => b.textContent.trim() === 'Not now').click());
  await sleep(100);
  assert.equal(await exists(page, '[data-testid="make-form"]'), false);
  note('"Not now" closes the form');

  await click(page, 'delete');
  await waitFor(page, () => document.querySelector('[data-testid="confirm"]'), 'the confirmation');
  await sleep(100);
  assert.equal(await page.evaluate(() => document.activeElement?.getAttribute('data-testid')), 'keep', 'focus is not on the safe choice');
  note('"Delete" asks first and puts focus on "Keep it"');
  await axeScan(page, 'delete confirmation open');
  await page.keyboard.press('Escape');
  await sleep(150);
  assert.equal(await exists(page, '[data-testid="confirm"]'), false);
  assert.equal(await page.evaluate(() => document.activeElement?.getAttribute('data-testid')), 'delete', 'focus did not return to Delete');
  assert.equal(world.videos.length, 1);
  note('Escape closes it, focus returns to "Delete", nothing deleted');
  await click(page, 'delete');
  await waitFor(page, () => document.querySelector('[data-testid="confirm"]'), 'the confirmation again');
  await click(page, 'confirm-delete');
  await waitFor(page, () => !document.querySelector('[data-testid="done"]') && document.querySelector('[data-testid="make"]'), 'the form after deleting');
  assert.equal(world.videos.length, 0);
  assert.ok(world.requests.includes('DELETE /story-videos/job-1'));
  note('"Delete the video" -> DELETE /story-videos/job-1, the video is gone and the form is back');
  assert.equal(await text(page, '[data-testid="announcement"]'), 'The video was deleted.');
  await page.close();

  // ---------------------------------------------------------------- 6. leave and come back, then cancel
  console.log('6. Leaving and coming back, and cancelling');
  reset();
  page = await openPage('dark', 375);
  await goto(page);
  await click(page, 'make');
  await waitFor(page, () => document.querySelector('[data-testid="active"]'), 'progress');
  await page.goto(`${BASE_URL}/`, { waitUntil: 'networkidle2' }); // leave the page
  await goto(page); // come back
  assert.ok(await exists(page, '[data-testid="active"]'), 'the running video was not picked up');
  note('left the page and came back: the video in the making was picked up');
  await shot(page, 'active-dark-375');
  await checkLayout(page, 'dark 375px, in progress');
  await axeScan(page, 'in progress, dark, 375px');
  await click(page, 'cancel');
  await waitFor(page, () => document.querySelector('[data-testid="failed"]'), 'the cancelled view');
  await sleep(150);
  assert.equal(await text(page, '[data-state-heading]'), 'You cancelled this video');
  assert.equal(await exists(page, '[role="alert"]'), false, 'a cancel was announced as an alert');
  assert.equal(await text(page, '[data-testid="make"]'), 'Make the video again');
  note('"Cancel the video" -> POST .../cancel, "You cancelled this video", no alert, "Make the video again" offered');
  await shot(page, 'cancelled-dark-375');
  await axeScan(page, 'cancelled, dark, 375px');
  await page.close();

  // ---------------------------------------------------------------- 7. a failure about one picture
  console.log('7. A failure about one picture');
  reset({ failWith: { errorCode: 'MEDIA_UNAVAILABLE', error: 'Scene 2: the picture is no longer available from Pixabay. Choose another and make the video again.' } });
  page = await openPage('light', 375);
  await goto(page);
  await click(page, 'make');
  await waitFor(page, () => document.querySelector('[data-testid="failed"]'), 'the failure', 30000);
  assert.ok(await page.$eval('[data-testid="failed"] [data-testid="problem"]', (p) => p.getAttribute('role') === 'alert'));
  assert.match(await text(page, '[data-testid="failed"]'), /A picture can no longer be used.*Scene 2: the picture is no longer available/);
  note('a failure watched live is an alert, in the API\'s words, with the scene named');
  await shot(page, 'failed-light-375');
  await checkLayout(page, 'light 375px, failed');
  await axeScan(page, 'failed, light, 375px');
  await click(page, 'go-to-scene');
  await sleep(200);
  assert.equal(await page.evaluate(() => document.activeElement?.id), 'scene-2-choose');
  note('"Go to scene 2" -> focus is on Scene 2\'s "Replace picture" button');
  await page.close();

  // ---------------------------------------------------------------- 8. a plan that cannot be made yet
  console.log('8. A plan with a scene that has no picture');
  reset({ noPictureOnScene3: true });
  page = await openPage('light', 375);
  await goto(page);
  assert.equal(await text(page, '[data-testid="blocker"]'), 'Scene 3 has no picture. Choose one, then save.');
  assert.equal(await page.$eval('[data-testid="make"]', (b) => b.disabled), true);
  note('the button is off and says why: "Scene 3 has no picture. Choose one, then save."');
  await checkLayout(page, 'light 375px, blocked');
  await axeScan(page, 'blocked, light, 375px');
  // Type in a scene: the video must wait for the save.
  reset();
  await page.reload({ waitUntil: 'networkidle2' });
  await page.waitForSelector('#scene-1-narration');
  await page.type('#scene-1-narration', ' And so on.');
  await page.waitForFunction(() => document.querySelector('[data-testid="blocker"]')?.textContent.includes('Save your changes first'));
  assert.equal(await page.$eval('[data-testid="make"]', (b) => b.disabled), true);
  note('editing a scene holds the button back: "Save your changes first."');
  await page.close();

  // ---------------------------------------------------------------- 9. Hindi: one voice
  console.log('9. A story in Hindi');
  reset({ language: 'hi' });
  page = await openPage('light', 375);
  await goto(page);
  assert.equal(await exists(page, 'select'), false);
  assert.match(await text(page, '[data-testid="voice-note"]'), /Hindi narration has one voice/);
  note('Hindi shows the one voice as a fact, with no menu');
  await page.close();

  // ---------------------------------------------------------------- 10. console
  console.log('10. The browser console');
  assert.deepEqual(problems, [], 'console problems:\n' + problems.join('\n'));
  note('no console errors, warnings or uncaught exceptions during any of the above');

  console.log(`\nstory video verification passed (${evidence.length} checks recorded)`);
  writeFileSync(join(SHOTS, 'story-video-evidence.txt'), evidence.join('\n') + '\n');
} finally {
  await browser.close();
}
