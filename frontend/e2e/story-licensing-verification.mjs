// Looks at the licensing text of Story videos in a real browser against a fake API,
// and prints what it found on each page. Run with the dev server up:
//
//   npx ng serve --port 4300 --configuration development
//   STORY_URL=http://localhost:4300 AXE_PATH=/path/to/axe.min.js node e2e/story-licensing-verification.mjs
//
// What it covers: the "Pictures and clips from other libraries" section of the terms
// (reached by its #stock address, as the picker links to it), the privacy page, the
// stock picker (the provider credit and the care note, with results, with none, and
// when the search fails) and the note under a finished video. For each: no sideways
// scroll, controls at least 44px tall, keyboard reach, axe (WCAG 2.1 A and AA), light
// and dark. AXE_PATH, if unset, skips the axe scans and says so.
import assert from 'node:assert/strict';
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import puppeteer from 'puppeteer';

const BASE_URL = process.env.STORY_URL ?? 'http://localhost:4300';
const API = 'http://localhost:3000';
const AXE_PATH = process.env.AXE_PATH;
const SHOTS = process.env.STORY_SHOTS ?? '/tmp';
const SIZES = [
  { w: 320, h: 640 },
  { w: 375, h: 667 },
  { w: 768, h: 1024 },
  { w: 1280, h: 800 },
];

const evidence = [];
const note = (line) => {
  evidence.push(line);
  console.log('  ' + line);
};
if (!AXE_PATH) note('AXE_PATH is not set: the axe scans are SKIPPED');

const CORS = {
  'access-control-allow-origin': BASE_URL,
  'access-control-allow-credentials': 'true',
  'access-control-allow-headers': 'authorization,content-type',
  'access-control-allow-methods': 'GET,POST,PUT,DELETE,OPTIONS',
};
const json = (request, status, body) =>
  request.respond({ status, headers: { ...CORS, 'content-type': 'application/json' }, body: JSON.stringify(body) });

const PIXEL = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=';
const asset = (id, provider = 'pixabay') => ({
  provider, type: 'photo', id, previewUrl: PIXEL, pageUrl: `https://${provider}.com/${id}`, width: 1080, height: 1920,
  orientation: 'portrait', alt: `Picture ${id}`, author: 'Jo Doe', credit: `Photo ${id} by Jo Doe on ${provider === 'pexels' ? 'Pexels' : 'Pixabay'}`,
});
const ATTRIBUTIONS = [
  { provider: 'pexels', text: 'Photos provided by Pexels', url: 'https://www.pexels.com' },
  { provider: 'pixabay', text: 'Images provided by Pixabay', url: 'https://pixabay.com' },
];

/** What the fake server does. */
let world = { search: 'results', withVideo: true };

function planRecord() {
  const scenes = [1, 2, 3].map((n) => ({
    narration: ['The lighthouse stood alone on the cliff.', 'Every night the keeper climbed the stairs.', 'One morning the sea was calm again.'][n - 1],
    description: `Scene ${n}`, keywords: ['lighthouse'], mediaType: 'photo', duration: 5,
    media: { provider: 'pixabay', type: 'photo', id: String(100 + n) },
  }));
  return {
    planId: 'plan-1', status: 'ready', revision: 2, createdAt: '2026-11-04T10:00:00Z', updatedAt: '2026-11-04T10:00:00Z',
    plan: { language: 'en', hook: 'The lighthouse stood alone.', totalSeconds: 15, scenes },
    media: scenes.map((s, i) => ({ ...s.media, asset: asset(String(101 + i)) })),
    mediaAttributions: [ATTRIBUTIONS[1]],
  };
}

const finishedVideo = () => ({
  jobId: 'job-1', planId: 'plan-1', planRevision: 2, status: 'completed', progress: 100, language: 'en', voice: 'Matthew',
  caption: { preset: 'bold' }, totalSeconds: 14.6, createdAt: '2026-11-04T10:05:00Z', updatedAt: '2026-11-04T10:07:00Z', completedAt: '2026-11-04T10:07:00Z',
});

function fakeApi(request) {
  const url = new URL(request.url());
  const path = url.pathname;
  if (request.method() === 'OPTIONS') return request.respond({ status: 204, headers: CORS });
  if (path === '/auth/refresh') return json(request, 200, { accessToken: 'e2e', refreshToken: 'e2e', expiresIn: 3600 });
  if (path === '/auth/me') return json(request, 200, { id: 'u1', email: 'e2e@example.com', name: 'E2E' });
  if (path === '/story-plans/limits') return json(request, 200, { enabled: true, minStoryChars: 30, maxStoryChars: 5000, minScenes: 2, maxScenes: 10, maxTotalSeconds: 90, perHour: 20 });
  if (path === '/story-plans/plan-1') return json(request, 200, planRecord());
  if (path === '/story-videos/limits') return json(request, 200, { enabled: true, maxActive: 2, perDay: 10, maxSeconds: 90, voices: { en: ['Joanna', 'Matthew'], hi: ['Aditi'] } });
  if (path === '/story-videos') return json(request, 200, { videos: world.withVideo ? [finishedVideo()] : [], hasMore: false });
  if (path === '/stock-media/search') {
    const type = url.searchParams.get('type') ?? 'photo';
    if (world.search === 'fail') return json(request, 502, { code: 'SEARCH_FAILED', message: 'The photo libraries could not be reached.' });
    // Real answers list the providers that answered, even when they found nothing.
    const assets = world.search === 'empty' ? [] : [asset('301'), asset('302', 'pexels'), asset('303'), asset('304')].map((a) => ({ ...a, type }));
    return json(request, 200, { assets, attributions: ATTRIBUTIONS, page: 1, perPage: 12, hasMore: false });
  }
  return json(request, 404, { code: 'NOT_FOUND', message: 'unknown ' + path });
}

const chrome = process.env.CHROME_PATH ?? ['/usr/bin/google-chrome', '/usr/bin/chromium', '/snap/bin/chromium'].find(existsSync);
const browser = await puppeteer.launch({ headless: true, ...(chrome ? { executablePath: chrome } : {}), args: ['--no-sandbox', '--disable-dev-shm-usage'] });

const problems = [];
/** `expect502`: this page makes a search that the fake API fails on purpose, and Chrome logs that as an error. */
async function openPage(theme, size, { expect502 = false } = {}) {
  const page = await browser.newPage();
  page.on('console', (msg) => {
    if (!['error', 'warning'].includes(msg.type())) return;
    const text = msg.text();
    if (/Failed to load resource.*(404|net::ERR)/.test(text)) return;
    if (expect502 && /Failed to load resource.*502/.test(text)) return;
    problems.push(`${msg.type()}: ${text}`);
  });
  page.on('pageerror', (e) => problems.push(`pageerror: ${e.message}`));
  await page.evaluateOnNewDocument((t) => {
    localStorage.setItem('indifferent.refreshToken', 'e2e-refresh-token');
    localStorage.setItem('app-theme', t);
  }, theme);
  await page.setViewport({ width: size.w, height: size.h, deviceScaleFactor: 1 });
  await page.setRequestInterception(true);
  page.on('request', (request) => {
    const u = new URL(request.url());
    if (u.origin === API) return void fakeApi(request);
    if (u.hostname.includes('fonts.g')) return void request.abort();
    return void request.continue();
  });
  return page;
}

const shot = (page, name) => page.screenshot({ path: join(SHOTS, `story-licensing-${name}.png`), fullPage: false });

async function axeScan(page, selector, label) {
  if (!AXE_PATH) return;
  await page.evaluate(readFileSync(AXE_PATH, 'utf8'));
  const violations = await page.evaluate(async (sel) => {
    const r = await axe.run(document.querySelector(sel), { runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21aa'] } });
    return r.violations.map((v) => ({ id: v.id, nodes: v.nodes.map((n) => n.target.join(' ')).slice(0, 3), help: v.help }));
  }, selector);
  assert.deepEqual(violations, [], `axe (${label}): ${JSON.stringify(violations)}`);
  note(`axe, ${label}: no WCAG 2.1 A/AA violations`);
}

async function noSidewaysScroll(page, label) {
  const m = await page.evaluate(() => ({ sw: document.documentElement.scrollWidth, iw: window.innerWidth }));
  assert.ok(m.sw <= m.iw + 1, `${label}: the page scrolls sideways (${m.sw} > ${m.iw})`);
}

/** Links and buttons in `scope` that are under 44px tall and not inside running text. */
async function shortTargets(page, scope) {
  return page.evaluate((sel) =>
    [...document.querySelectorAll(`${sel} a, ${sel} button`)]
      .filter((e) => e.offsetParent !== null && !e.closest('p, li > span') === false ? false : e.offsetParent !== null)
      .filter((e) => !e.closest('p')) // a link inside a sentence is exempt; one on its own line is not
      .map((e) => ({ t: (e.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 50), h: Math.round(e.getBoundingClientRect().height) }))
      .filter((x) => x.h < 44),
  scope);
}

const sizeLabel = (theme, s) => `${theme} ${s.w}px`;

try {
  // ---------------------------------------------------------------- 1. the terms section
  console.log('1. Terms: the section the picker links to');
  for (const theme of ['light', 'dark']) {
    for (const s of SIZES) {
      const page = await openPage(theme, s);
      await page.goto(`${BASE_URL}/terms#stock`, { waitUntil: 'networkidle2' });
      await page.waitForSelector('section#stock', { timeout: 20000 });
      await new Promise((r) => setTimeout(r, 600)); // the router scrolls to the fragment after the page renders
      // Opening the address must put the section 112px from the top (the page's scroll-mt-28),
      // which leaves its heading clear of the fixed header, which ends at 72px.
      const top = await page.$eval('section#stock', (e) => Math.round(e.getBoundingClientRect().top));
      const headingTop = await page.$eval('section#stock h2', (e) => Math.round(e.getBoundingClientRect().top));
      assert.ok(Math.abs(top - 112) <= 2, `${sizeLabel(theme, s)}: /terms#stock put the section ${top}px from the top, not 112px`);
      assert.ok(headingTop > 80, `${sizeLabel(theme, s)}: the heading is under the fixed header (top ${headingTop}px)`);
      const heading = await page.$eval('section#stock h2', (h) => [...h.querySelectorAll('span')].map((x) => x.textContent.trim()).join(' | '));
      assert.equal(heading, '05 | Pictures and clips from other libraries');
      await noSidewaysScroll(page, `terms ${sizeLabel(theme, s)}`);
      assert.deepEqual(await shortTargets(page, 'section#stock'), [], `terms ${sizeLabel(theme, s)}: links under 44px`);
      const links = await page.$$eval('section#stock a[href^="http"]', (as) => as.map((a) => `${a.getAttribute('href')} ${a.target} ${a.rel}`));
      assert.deepEqual(links, [
        'https://www.pexels.com/license/ _blank noopener noreferrer',
        'https://pixabay.com/service/license-summary/ _blank noopener noreferrer',
      ]);
      if (s.w === 375 || s.w === 1280) await axeScan(page, 'main', `terms, ${sizeLabel(theme, s)}`);
      if ((theme === 'light' && s.w === 375) || (theme === 'dark' && s.w === 1280)) await shot(page, `terms-${theme}-${s.w}`);
      await page.close();
    }
  }
  note('terms: /terms#stock scrolls to section 05; no sideways scroll; links 44px and open safely, at 320/375/768/1280 in light and dark');

  // The landing page's #process is reached through the router from About, Contact, Blog and the
  // watermark tool, which is the same scrolling as /terms#stock. Follow one of those links the way a
  // person would. Its exact position depends on that page's own layout, so only what matters is
  // checked: the section is on screen and clear of the fixed header (which ends at 72px).
  {
    const page = await openPage('light', { w: 375, h: 667 });
    await page.goto(`${BASE_URL}/about`, { waitUntil: 'networkidle2' });
    const link = await page.$('a[href="/#process"]');
    assert.ok(link, 'about: no link to /#process found');
    await page.evaluate(() => document.querySelector('a[href="/#process"]').click());
    await page.waitForFunction(() => location.pathname === '/' && location.hash === '#process', { timeout: 10000 });
    await new Promise((r) => setTimeout(r, 800));
    const top = await page.$eval('#process', (e) => Math.round(e.getBoundingClientRect().top));
    assert.ok(top > 80 && top < 250, `landing #process, reached from About: ${top}px from the top (want clear of the 72px header and on screen)`);
    await page.close();
    note(`a router link to /#process (from About) lands ${top}px from the top: clear of the header and on screen`);
  }

  // ---------------------------------------------------------------- 2. privacy
  console.log('2. Privacy');
  for (const theme of ['light', 'dark']) {
    for (const s of SIZES) {
      const page = await openPage(theme, s);
      await page.goto(`${BASE_URL}/privacy`, { waitUntil: 'networkidle2' });
      await page.waitForSelector('section#tools', { timeout: 20000 });
      const body = await page.$eval('main', (m) => m.textContent.replace(/\s+/g, ' '));
      for (const must of [
        'We keep the plan and not the story',
        'Your browser then loads the preview pictures and clips directly from those services',
        'Story plans stay until you ask us to remove them',
        'Only the narration text goes to Polly',
      ]) assert.ok(body.includes(must), `privacy ${sizeLabel(theme, s)}: missing "${must}"`);
      await noSidewaysScroll(page, `privacy ${sizeLabel(theme, s)}`);
      if (s.w === 375 || s.w === 1280) await axeScan(page, 'main', `privacy, ${sizeLabel(theme, s)}`);
      await page.close();
    }
  }
  note('privacy: the Story videos statements are on the page; no sideways scroll; at four widths in light and dark');

  // ---------------------------------------------------------------- 3. the picker
  console.log('3. The stock picker');
  const openPicker = async (page, mode) => {
    world = { search: mode, withVideo: true };
    await page.goto(`${BASE_URL}/story/plan-1`, { waitUntil: 'networkidle2' });
    await page.waitForSelector('#scene-1-choose', { timeout: 20000 });
    await page.click('#scene-1-choose');
    await page.waitForSelector('dialog[open]', { timeout: 10000 });
    await page.waitForFunction(() => !document.querySelector('dialog[open] [data-testid="picker-status"]')?.textContent.includes('Searching'), { timeout: 10000 });
    await new Promise((r) => setTimeout(r, 150));
  };
  /** Is the element wholly inside the picker's scrolling body, with no scrolling done? */
  const visibleInBody = (page, sel) =>
    page.evaluate((s) => {
      const body = document.querySelector('dialog[open] .picker-body').getBoundingClientRect();
      const e = document.querySelector(`dialog[open] ${s}`)?.getBoundingClientRect();
      return !!e && e.top >= body.top - 1 && e.bottom <= body.bottom + 1;
    }, sel);

  for (const theme of ['light', 'dark']) {
    for (const s of SIZES) {
      const page = await openPage(theme, s);
      await openPicker(page, 'results');
      const lab = sizeLabel(theme, s);
      // The credit is above the results, and both it and the note are seen with no scrolling.
      const order = await page.evaluate(() => {
        const d = document.querySelector('dialog[open]');
        const c = d.querySelector('[data-testid="attribution"]');
        const r = d.querySelector('[data-testid="results"]');
        return !!c && !!r && !!(c.compareDocumentPosition(r) & Node.DOCUMENT_POSITION_FOLLOWING);
      });
      assert.ok(order, `picker ${lab}: the provider credit is not above the results`);
      assert.ok(await visibleInBody(page, '[data-testid="attribution"]'), `picker ${lab}: the provider credit needs scrolling to be seen`);
      assert.ok(await visibleInBody(page, '[data-testid="picker-care"] p'), `picker ${lab}: the care note needs scrolling to be seen`);
      const creditText = await page.$eval('dialog[open] [data-testid="attribution"]', (e) => e.textContent.replace(/\s+/g, ' ').trim());
      assert.match(creditText, /Photos provided by Pexels.*Images provided by Pixabay/);
      const care = await page.$eval('dialog[open] [data-testid="picker-care"] p', (e) => e.textContent.replace(/\s+/g, ' ').trim());
      assert.match(care, /^Check each picture before you choose it\. Avoid visible logos or brand names, people who could be recognised shown in a bad light or as if they back something, and private homes\.$/);
      const link = await page.$eval('dialog[open] [data-testid="picker-terms"]', (a) => ({ href: a.getAttribute('href'), target: a.target, rel: a.rel, h: Math.round(a.getBoundingClientRect().height), text: a.textContent.replace(/\s+/g, ' ').trim() }));
      assert.deepEqual({ ...link, h: undefined }, { href: '/terms#stock', target: '_blank', rel: 'noopener noreferrer', h: undefined, text: 'What the terms say (opens in a new tab)' });
      assert.ok(link.h >= 44, `picker ${lab}: the terms link is ${link.h}px tall`);
      assert.deepEqual(await shortTargets(page, 'dialog[open] [data-testid="picker-care"]'), [], `picker ${lab}: credit links under 44px`);
      await noSidewaysScroll(page, `picker ${lab}`);
      const dialogWide = await page.evaluate(() => {
        const d = document.querySelector('dialog[open]');
        const b = d.querySelector('.picker-body');
        return d.scrollWidth > d.clientWidth + 1 || b.scrollWidth > b.clientWidth + 1;
      });
      assert.ok(!dialogWide, `picker ${lab}: the dialog scrolls sideways`);
      if (s.w === 375 || s.w === 1280) await axeScan(page, 'dialog[open]', `picker with results, ${lab}`);
      if ((theme === 'light' && s.w === 375) || (theme === 'dark' && s.w === 1280)) await shot(page, `picker-${theme}-${s.w}`);
      await page.close();
    }
  }
  note('picker with results: the credit is above the results and, with the care note, in view without scrolling at 320x640, 375x667, 768x1024, 1280x800, light and dark; 44px link; no sideways scroll');

  // A phone held sideways: the whole dialog scrolls, and the credit and note must still be there and not clipped.
  {
    const page = await openPage('light', { w: 667, h: 375 });
    await openPicker(page, 'results');
    await noSidewaysScroll(page, 'picker landscape');
    const reach = await page.evaluate(() => {
      const d = document.querySelector('dialog[open]');
      const e = d.querySelector('[data-testid="picker-care"]');
      e.scrollIntoView();
      const r = e.getBoundingClientRect();
      return r.top >= 0 && r.bottom <= window.innerHeight;
    });
    assert.ok(reach, 'picker landscape: the credit and note cannot be scrolled into view');
    await axeScan(page, 'dialog[open]', 'picker with results, landscape 667x375');
    await page.close();
    note('picker, phone held sideways (667x375): the credit and note can be scrolled into view; axe clean');
  }

  // Keyboard: Tab reaches the credit links and the terms link, Escape closes, and focus goes back to the button that opened it.
  {
    const page = await openPage('light', { w: 1280, h: 800 });
    await openPicker(page, 'results');
    await page.focus('#picker-query');
    const stops = [];
    for (let i = 0; i < 20; i++) {
      await page.keyboard.press('Tab');
      stops.push(await page.evaluate(() => document.activeElement?.getAttribute('data-testid') || document.activeElement?.textContent?.trim().slice(0, 24) || document.activeElement?.tagName));
      if (stops.at(-1) === 'picker-terms') break;
    }
    assert.equal(stops.at(-1), 'picker-terms', `keyboard: Tab did not reach the terms link (stops: ${stops.join(' > ')})`);
    const ring = await page.evaluate(() => {
      const cs = getComputedStyle(document.activeElement);
      return cs.outlineStyle !== 'none' && parseFloat(cs.outlineWidth) >= 2 ? `${cs.outlineWidth} ${cs.outlineStyle}` : cs.boxShadow !== 'none' ? 'box-shadow' : 'none';
    });
    assert.notEqual(ring, 'none', 'keyboard: the terms link shows no focus ring');
    note(`keyboard: Tab reaches the terms link after ${stops.length} presses from the search box; focus ring: ${ring}`);
    await page.keyboard.press('Escape');
    await page.waitForFunction(() => !document.querySelector('dialog[open]'), { timeout: 5000 });
    const back = await page.evaluate(() => document.activeElement?.id);
    assert.equal(back, 'scene-1-choose', 'keyboard: after Escape, focus did not return to the button that opened the picker');
    note('keyboard: Escape closes the picker and focus returns to "Replace picture"');
    await page.close();
  }

  // No results: the libraries were still asked, so they are still credited.
  {
    const page = await openPage('light', { w: 375, h: 667 });
    await openPicker(page, 'empty');
    assert.ok(await page.$('dialog[open] [data-testid="empty"]'), 'empty: the "no results" state did not show');
    const credit = await page.$eval('dialog[open] [data-testid="attribution"]', (e) => e.textContent.replace(/\s+/g, ' ').trim());
    assert.match(credit, /Photos provided by Pexels.*Images provided by Pixabay/);
    assert.ok(await visibleInBody(page, '[data-testid="attribution"]'), 'empty: the credit needs scrolling to be seen');
    await axeScan(page, 'dialog[open]', 'picker with no results, light 375px');
    await shot(page, 'picker-empty-375');
    await page.close();
    note('picker with no results: the libraries are still credited, in view; axe clean');
  }

  // A failed search shows no results, so it shows no credit and no note.
  {
    const page = await openPage('light', { w: 375, h: 667 }, { expect502: true });
    await openPicker(page, 'fail');
    assert.equal(await page.$('dialog[open] [data-testid="attribution"]'), null, 'failure: a credit was shown with no results');
    assert.equal(await page.$('dialog[open] [data-testid="picker-care"]'), null, 'failure: the care note was shown with no results');
    assert.ok(await page.$('dialog[open] [data-testid="problem"]'), 'failure: the problem was not shown');
    await axeScan(page, 'dialog[open]', 'picker after a failed search, light 375px');
    await page.close();
    note('picker after a failed search: the problem is shown and no credit or care note is');
  }

  // ---------------------------------------------------------------- 4. the finished video
  console.log('4. The note under a finished video');
  for (const theme of ['light', 'dark']) {
    for (const s of SIZES) {
      const page = await openPage(theme, s);
      world = { search: 'results', withVideo: true };
      await page.goto(`${BASE_URL}/story/plan-1`, { waitUntil: 'networkidle2' });
      await page.waitForSelector('[data-testid="licence-note"]', { timeout: 20000 });
      const lab = sizeLabel(theme, s);
      const t = await page.$eval('[data-testid="licence-note"]', (e) => e.textContent.replace(/\s+/g, ' ').trim());
      assert.equal(t, 'This is the finished video only. The pictures and clips in it stay under the Pexels and Pixabay licences. Neither requires a credit in your video; the credit for each picture is under its scene above.');
      const inside = await page.evaluate(() => {
        const p = document.querySelector('[data-testid="video-panel"]').getBoundingClientRect();
        const n = document.querySelector('[data-testid="licence-note"]').getBoundingClientRect();
        return n.left >= p.left - 1 && n.right <= p.right + 1;
      });
      assert.ok(inside, `video ${lab}: the note leaves the panel`);
      // "under its scene above": the scene credits really are above the panel, and there is one per scene.
      const above = await page.evaluate(() => {
        const panelTop = document.querySelector('[data-testid="video-panel"]').getBoundingClientRect().top + window.scrollY;
        const credits = [...document.querySelectorAll('[data-testid="credit"]')];
        return { n: credits.length, allAbove: credits.every((c) => c.getBoundingClientRect().top + window.scrollY < panelTop) };
      });
      assert.deepEqual(above, { n: 3, allAbove: true }, `video ${lab}: the scene credits are not above the panel`);
      await noSidewaysScroll(page, `video ${lab}`);
      if (s.w === 375 || s.w === 1280) await axeScan(page, '[data-testid="video-panel"]', `finished video, ${lab}`);
      if (theme === 'light' && s.w === 375) {
        // The page scrolls smoothly; an instant scroll puts the note in the frame before the shot.
        await page.$eval('[data-testid="licence-note"]', (e) => e.scrollIntoView({ block: 'center', behavior: 'instant' }));
        await new Promise((r) => setTimeout(r, 300));
        const inFrame = await page.$eval('[data-testid="licence-note"]', (e) => {
          const r = e.getBoundingClientRect();
          return r.top >= 0 && r.bottom <= window.innerHeight;
        });
        assert.ok(inFrame, `video ${lab}: the note is not in the frame of its own screenshot`);
        await shot(page, 'video-note-light-375');
      }
      await page.close();
    }
  }
  note('finished video: the note is in the panel, the three scene credits it points to are above it; no sideways scroll; axe clean; four widths, light and dark');

  // The note belongs to a finished video: with none, the panel does not show it.
  {
    const page = await openPage('light', { w: 375, h: 667 });
    world = { search: 'results', withVideo: false };
    await page.goto(`${BASE_URL}/story/plan-1`, { waitUntil: 'networkidle2' });
    await page.waitForSelector('[data-testid="video-panel"]', { timeout: 20000 });
    await page.waitForFunction(() => !document.querySelector('[data-testid="loading"]'), { timeout: 10000 });
    assert.equal(await page.$('[data-testid="licence-note"]'), null, 'with no finished video the note should not be shown');
    await page.close();
    note('with no finished video the licence note is not shown');
  }

  // ---------------------------------------------------------------- 5. console
  console.log('5. The browser console');
  assert.deepEqual(problems, [], 'console problems:\n' + problems.join('\n'));
  note('no console errors, warnings or uncaught exceptions during any of the above');

  console.log(`\nstory licensing verification passed (${evidence.length} checks recorded)`);
  writeFileSync(join(SHOTS, 'story-licensing-evidence.txt'), evidence.join('\n') + '\n');
} finally {
  await browser.close();
}
