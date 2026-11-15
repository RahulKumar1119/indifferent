// Real-browser check of the cookie consent engine (#321). Not part of `ng test`.
//   cd frontend && npx ng build && node e2e/consent-verification.mjs [outDir]
// Serves dist/frontend/browser, records and aborts every request that leaves localhost, and checks what the page
// asks Google, AdSense and CloudWatch RUM for in each situation: first visit, reject, accept, custom choice,
// withdrawal, a lazy-loaded route, a direct visit to an inner page and a visitor with data from before the banner.
// Writes screenshots to outDir (default ~/shots303/consent), never into the repo. Exit 1 on any failure.
import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '..');
const require = createRequire(path.join(root, '/'));
const puppeteer = require('puppeteer');
const outDir = path.resolve(process.argv[2] ?? path.join(process.env.HOME, 'shots303/consent'));
fs.mkdirSync(outDir, { recursive: true });
const dist = path.join(root, 'dist/frontend/browser');
if (!fs.existsSync(path.join(dist, 'index.html'))) throw new Error('Build first: npx ng build');

const mime = { '.html': 'text/html', '.js': 'text/javascript', '.mjs': 'text/javascript', '.css': 'text/css', '.woff2': 'font/woff2', '.json': 'application/json', '.svg': 'image/svg+xml', '.png': 'image/png', '.ico': 'image/x-icon', '.webp': 'image/webp', '.jpg': 'image/jpeg' };
const server = http.createServer((req, res) => {
  let p = '/';
  try {
    p = decodeURIComponent(req.url.split('?')[0]);
  } catch {
    /* a malformed escape gets the app page */
  }
  let f = path.resolve(dist, '.' + path.posix.normalize('/' + p));
  const inside = f === dist || f.startsWith(dist + path.sep);
  if (inside && fs.existsSync(f) && fs.statSync(f).isDirectory() && fs.existsSync(path.join(f, 'index.html'))) f = path.join(f, 'index.html');
  if (p.includes('\0') || !inside || !fs.existsSync(f) || fs.statSync(f).isDirectory()) f = path.join(dist, 'index.html');
  res.writeHead(200, { 'content-type': mime[path.extname(f)] ?? 'application/octet-stream' });
  fs.createReadStream(f).pipe(res);
});
await new Promise((r) => server.listen(0, '127.0.0.1', r));
const base = `http://127.0.0.1:${server.address().port}`;

const failures = [];
const check = (ok, what) => {
  console.log(`${ok ? '  ok  ' : '  FAIL'} ${what}`);
  if (!ok) failures.push(what);
};
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const browser = await puppeteer.launch({ executablePath: '/usr/bin/google-chrome', headless: true, args: ['--no-sandbox'] });

/** What a request is, for the purpose of consent. */
function kind(url) {
  if (/googletagmanager\.com/.test(url)) return 'gtag';
  if (/googlesyndication\.com|doubleclick\.net|googleadservices\.com/.test(url)) return 'ads';
  if (/google-analytics\.com/.test(url)) return 'ga';
  if (/rum\.[\w-]+\.amazonaws\.com|cognito-identity/.test(url)) return 'rum';
  if (/fonts\.googleapis\.com|fonts\.gstatic\.com/.test(url)) return 'fonts';
  return 'other';
}

/** A fresh browser profile with the network closed to everything but this server. */
async function newVisitor({ width = 1280, height = 900, theme = 'light', local = {} } = {}) {
  const ctx = await browser.createBrowserContext();
  const page = await ctx.newPage();
  await page.setViewport({ width, height });
  await page.emulateMediaFeatures([{ name: 'prefers-color-scheme', value: theme }]);
  const seen = [];
  await page.setRequestInterception(true);
  page.on('request', (req) => {
    const url = req.url();
    if (url.startsWith(base) || url.startsWith('data:') || url.startsWith('blob:')) return req.continue();
    seen.push({ url, kind: kind(url) });
    return req.abort();
  });
  // Seed storage once, before the first script of the first page, like an earlier visit would have left it.
  await page.evaluateOnNewDocument((entries) => {
    if (sessionStorage.getItem('__seeded')) return;
    sessionStorage.setItem('__seeded', '1');
    for (const [k, v] of Object.entries(entries)) localStorage.setItem(k, v);
  }, local);
  const kinds = () => new Set(seen.filter((s) => s.kind !== 'fonts').map((s) => s.kind));
  return { ctx, page, seen, kinds };
}
const go = async (page, route = '/') => {
  await page.goto(`${base}${route}`, { waitUntil: 'networkidle2', timeout: 30000 });
  await sleep(400);
};
const bannerShown = (page) => page.evaluate(() => !!document.querySelector('[data-cookie-banner]:not([hidden])'));
const clickText = (page, text) =>
  page.evaluate((t) => {
    const b = [...document.querySelectorAll('button')].find((x) => x.textContent.trim() === t && x.offsetParent !== null);
    if (!b) return false;
    b.click();
    return true;
  }, text);
const layer = (page) =>
  page.evaluate(() => (window.dataLayer ?? []).map((a) => Array.from(a)).map((c) => (c[0] === 'config' ? `config:${c[1]}` : c[0] === 'consent' ? `consent:${c[1]}` : String(c[0]))));
const store = (page) => page.evaluate(() => Object.fromEntries(Object.entries(localStorage)));
const cookies = (page) => page.evaluate(() => document.cookie);
const scripts = (page) => page.evaluate(() => [...document.scripts].map((s) => s.src).filter(Boolean));
const rumLoaded = (page) => page.evaluate(() => performance.getEntriesByType('resource').some((e) => /rum|aws/i.test(e.name) && e.name.includes('chunk')));

// --- 1. first visit
console.log('First visit');
{
  const v = await newVisitor({ local: { 'app-theme': 'light', 'indifferent.infographic.draft.v1': '{"version":1}', 'indifferent.creatorProfile.v1': '{"name":"Old"}' } });
  await go(v.page);
  check(await bannerShown(v.page), 'the banner is shown');
  check((await v.page.evaluate(() => document.body.innerText)).includes('We value your privacy'), 'it says "We value your privacy"');
  for (const t of ['Accept All', 'Reject All', 'Customize Preferences']) check(await v.page.evaluate((x) => [...document.querySelectorAll('button')].some((b) => b.textContent.trim() === x), t), `button: ${t}`);
  await v.page.mouse.move(100, 100);
  await v.page.evaluate(() => window.scrollBy(0, 400));
  await v.page.keyboard.press('Tab');
  await sleep(3600); // longer than the idle wait a returning visitor would have
  check(v.kinds().size === 0, `no request to Google, AdSense or RUM (saw: ${[...v.kinds()].join(', ') || 'none'})`);
  check((await layer(v.page)).length === 0, 'no dataLayer, no consent default');
  check(!(await scripts(v.page)).some((s) => /googletagmanager|googlesyndication/.test(s)), 'no Google script element');
  const s = await store(v.page);
  check(!('indifferent.consent.v1' in s), 'no consent record is written before a choice');
  check(s['app-theme'] === 'light' && 'indifferent.infographic.draft.v1' in s && 'indifferent.creatorProfile.v1' in s, 'old Functional values are left untouched before a choice');
  check(await v.page.evaluate(() => document.documentElement.classList.contains('dark')), 'the default theme is used, not the saved one');
  check((await cookies(v.page)) === '', 'no cookie is set');
  await v.page.screenshot({ path: path.join(outDir, 'banner-desktop-light.png') });
  // lazy-loaded route and a direct visit while undecided
  await v.page.evaluate(() => document.querySelector('a[href="/blog"]')?.click());
  await sleep(1200);
  await go(v.page, '/blog/ai-narration-guide');
  check(v.kinds().size === 0 && (await bannerShown(v.page)), 'an inner page opened directly still asks, and still loads nothing');
  await v.ctx.close();
}

// --- 2. reject all, then a migration check
console.log('Reject All');
{
  const v = await newVisitor({ local: { 'app-theme': 'light', 'indifferent.infographic.draft.v1': '{"version":1}', 'indifferent.creatorProfile.v1': '{"name":"Old"}', 'refresh-token': 'keep' } });
  await go(v.page);
  await clickText(v.page, 'Reject All');
  await sleep(500);
  check(!(await bannerShown(v.page)), 'the banner goes away');
  const s = await store(v.page);
  const rec = JSON.parse(s['indifferent.consent.v1'] ?? 'null');
  check(rec && !rec.choices.analytics && !rec.choices.advertising && !rec.choices.functional, 'the record says nothing optional');
  check(!('app-theme' in s) && !('indifferent.infographic.draft.v1' in s) && !('indifferent.creatorProfile.v1' in s), 'old Functional values are deleted on rejection');
  check(s['refresh-token'] === 'keep', 'strictly necessary storage is kept');
  await go(v.page, '/about');
  await v.page.reload({ waitUntil: 'networkidle2' });
  await v.page.mouse.move(50, 50);
  await sleep(3600);
  check(v.kinds().size === 0, `after rejection and a reload, still no Google, AdSense or RUM request (saw: ${[...v.kinds()].join(', ') || 'none'})`);
  check(!(await bannerShown(v.page)), 'the banner does not return on reload');
  check((await layer(v.page)).length === 0, 'no dataLayer after rejection');
  await v.ctx.close();
}

// --- 3. accept all
console.log('Accept All');
{
  const v = await newVisitor({ local: { 'app-theme': 'light' } });
  await go(v.page);
  await clickText(v.page, 'Accept All');
  await sleep(2500);
  check(v.kinds().has('gtag'), 'the Google tag is requested after Accept All');
  check(v.seen.filter((s) => s.kind === 'gtag').length === 1, 'the Google tag library is requested once');
  check(v.kinds().has('ads'), 'AdSense is requested after Accept All');
  check(v.kinds().has('rum'), `CloudWatch RUM is started after Accept All (requests: ${v.seen.filter((s) => s.kind === 'rum').length})`);
  const l = await layer(v.page);
  check(l.slice(0, 4).join() === 'consent:default,consent:update,js,config:G-D2GB04M42D', `dataLayer order (${l.join(' > ')})`);
  check(l.includes('config:AW-17587685388'), 'the Ads ID is configured');
  check(l.filter((x) => x === 'js').length === 1 && new Set(l.filter((x) => x.startsWith('config:'))).size === l.filter((x) => x.startsWith('config:')).length, 'no duplicate js or config');
  const sig = await v.page.evaluate(() => Array.from(window.dataLayer.find((a) => a[0] === 'consent' && a[1] === 'update')[2] && [window.dataLayer.find((a) => a[0] === 'consent' && a[1] === 'update')[2]])[0]);
  check(sig.analytics_storage === 'granted' && sig.ad_storage === 'granted' && sig.ad_user_data === 'granted' && sig.functionality_storage === 'granted' && sig.security_storage === 'granted', 'consent update grants the allowed signals');
  check(sig.ad_personalization === 'denied', 'ad_personalization stays denied (no TCF platform yet)');
  check((await store(v.page))['app-theme'] === 'light', 'the old theme is used again with Functional');
  check(await v.page.evaluate(() => !document.documentElement.classList.contains('dark')), 'the saved light theme applies');
  // lazy route after consent
  await v.page.evaluate(() => document.querySelector('a[href="/blog"]')?.click());
  await sleep(1500);
  check(v.seen.filter((s) => s.kind === 'gtag').length === 1, 'a lazy-loaded route does not add a second Google tag');
  // returning visitor
  const gtagBefore = v.seen.filter((s) => s.kind === 'gtag').length;
  await v.page.reload({ waitUntil: 'networkidle2' });
  await sleep(300);
  const early = v.seen.filter((s) => s.kind === 'gtag').length;
  await v.page.mouse.move(60, 60);
  await v.page.keyboard.press('Tab');
  await sleep(1500);
  check(early === gtagBefore, 'a returning visitor does not load the tag before an interaction or the idle time');
  check(v.seen.filter((s) => s.kind === 'gtag').length === gtagBefore + 1, 'a returning visitor loads the tag after an interaction');
  const l2 = await layer(v.page);
  check(l2[0] === 'consent:default' && l2[1] === 'consent:update', 'the defaults come before anything else on a returning visit');
  check(!(await bannerShown(v.page)), 'no banner for a returning visitor');
  await v.ctx.close();
}

// --- 4. a custom choice: Analytics only
console.log('Customize: Analytics only');
{
  const v = await newVisitor({ local: { 'app-theme': 'light', 'indifferent.infographic.draft.v1': '{"version":1}' } });
  await go(v.page);
  await clickText(v.page, 'Customize Preferences');
  await sleep(500);
  check(await v.page.evaluate(() => !!document.querySelector('[data-cookie-preferences]')), 'the panel opens');
  const sw = await v.page.evaluate(() => [...document.querySelectorAll('[role=switch]')].map((s) => s.getAttribute('aria-checked')));
  check(sw.length === 3 && sw.every((x) => x === 'false'), 'all three optional switches start off');
  check(await v.page.evaluate(() => document.activeElement?.id === 'ck-pref-title'), 'focus moves into the panel');
  await v.page.screenshot({ path: path.join(outDir, 'panel-desktop-light.png') });
  await v.page.evaluate(() => document.querySelectorAll('[role=switch]')[0].click());
  await clickText(v.page, 'Save preferences');
  await sleep(2500);
  check(v.kinds().has('gtag') && v.kinds().has('rum'), 'Analytics: Google tag and RUM start');
  check(!v.kinds().has('ads'), 'Analytics only: no AdSense request');
  const l = await layer(v.page);
  check(l.includes('config:G-D2GB04M42D') && !l.includes('config:AW-17587685388'), 'only the Analytics ID is configured');
  const s = await store(v.page);
  check(!('app-theme' in s) && !('indifferent.infographic.draft.v1' in s), 'Functional is off, so old Functional values are deleted');
  await v.ctx.close();
}

// --- 5. withdrawal
console.log('Withdrawal');
{
  const v = await newVisitor();
  await go(v.page);
  await clickText(v.page, 'Accept All');
  await sleep(2000);
  await v.page.evaluate(() => {
    document.cookie = '_ga=1; path=/';
    document.cookie = '_gcl_au=2; path=/';
    document.cookie = 'cwr_u=3; path=/';
    document.cookie = 'plain=4; path=/';
  });
  const before = v.seen.length;
  // footer link reopens the panel
  await v.page.evaluate(() => window.scrollTo(0, document.body.scrollHeight));
  check(await clickText(v.page, 'Cookie preferences'), 'the footer has a "Cookie preferences" button');
  await sleep(600);
  check(await v.page.evaluate(() => !!document.querySelector('[data-cookie-preferences]')), 'the footer button reopens the panel');
  const sw = await v.page.evaluate(() => [...document.querySelectorAll('[role=switch]')].map((s) => s.getAttribute('aria-checked')));
  check(sw.every((x) => x === 'true'), 'the panel shows the saved choice');
  // Escape returns focus to the opener
  await v.page.keyboard.press('Escape');
  await sleep(500);
  check(await v.page.evaluate(() => !document.querySelector('[data-cookie-preferences]')), 'Escape closes the panel');
  check(await v.page.evaluate(() => document.activeElement?.textContent.trim() === 'Cookie preferences'), 'focus returns to the footer button');
  await clickText(v.page, 'Cookie preferences');
  await sleep(400);
  await clickText(v.page, 'Reject All');
  await sleep(1200);
  const l = await layer(v.page);
  const last = await v.page.evaluate(() => Array.from(window.dataLayer.at(-1)));
  check(last[0] === 'consent' && last[1] === 'update' && last[2].analytics_storage === 'denied' && last[2].ad_storage === 'denied' && last[2].ad_user_data === 'denied', `a denied consent update is sent (${l.at(-1)})`);
  check((await v.page.evaluate(() => window['ga-disable-G-D2GB04M42D'])) === true, 'Google Analytics is switched off for this page');
  check((await v.page.evaluate(() => window.adsbygoogle?.pauseAdRequests)) === 1, 'AdSense ad requests are paused');
  const c = await cookies(v.page);
  check(c === 'plain=4', `cookies the site can reach are removed, others kept (left: ${c})`);
  await v.page.evaluate(() => document.querySelector('a[href="/blog"]')?.click());
  await sleep(2500);
  const after = v.seen.slice(before).filter((s) => ['gtag', 'ads', 'ga'].includes(s.kind));
  check(after.length === 0, `no new Google or AdSense request after withdrawal (${after.length})`);
  await v.page.reload({ waitUntil: 'networkidle2' });
  await v.page.mouse.move(40, 40);
  await sleep(3600);
  const total = v.seen.filter((s) => ['gtag', 'ads', 'ga', 'rum'].includes(s.kind)).length;
  const reloadNew = v.seen.slice(before).filter((s) => ['gtag', 'ads', 'ga', 'rum'].includes(s.kind)).length;
  check(reloadNew === 0, `after withdrawal and a reload nothing optional loads (${reloadNew} of ${total} since)`);
  await v.ctx.close();
}

// --- 6. expired choice and a changed policy version
console.log('Expiry and version');
{
  const rec = (over) => JSON.stringify({ version: 1, savedAt: '2026-01-01T00:00:00.000Z', expiresAt: '2099-01-01T00:00:00.000Z', choices: { analytics: true, advertising: true, functional: true }, ...over });
  for (const [name, over] of [['expired', { expiresAt: '2026-02-01T00:00:00.000Z' }], ['another version', { version: 2 }], ['damaged', null]]) {
    const v = await newVisitor({ local: { 'indifferent.consent.v1': over === null ? '{broken' : rec(over) } });
    await go(v.page);
    await v.page.mouse.move(10, 10);
    await sleep(3500);
    check((await bannerShown(v.page)) && v.kinds().size === 0, `${name} record: asks again and loads nothing`);
    await v.ctx.close();
  }
}

// --- 7. phone, dark theme
console.log('Phone and dark theme');
{
  const v = await newVisitor({ width: 390, height: 780, theme: 'dark' });
  await go(v.page);
  const box = await v.page.evaluate(() => {
    const r = document.querySelector('[data-cookie-banner]').getBoundingClientRect();
    return { l: r.left, r: r.right, t: r.top, b: r.bottom, w: innerWidth, h: innerHeight, sw: document.documentElement.scrollWidth };
  });
  check(box.l >= 0 && box.r <= box.w && box.b <= box.h + 1 && box.sw <= box.w, 'the banner fits a 390 px phone without sideways scrolling');
  await v.page.screenshot({ path: path.join(outDir, 'banner-phone-dark.png') });
  await clickText(v.page, 'Customize Preferences');
  await sleep(500);
  const p = await v.page.evaluate(() => {
    const r = document.querySelector('[data-cookie-preferences]').getBoundingClientRect();
    return { l: r.left, r: r.right, w: innerWidth };
  });
  check(p.l >= 0 && p.r <= p.w, 'the panel fits a 390 px phone');
  await v.page.screenshot({ path: path.join(outDir, 'panel-phone-dark.png') });
  await v.ctx.close();
}

// --- 8. the theme script in index.html matches the app's consent record
console.log('index.html');
{
  const html = fs.readFileSync(path.join(dist, 'index.html'), 'utf8');
  const model = fs.readFileSync(path.join(root, 'src/app/core/consent/consent.model.ts'), 'utf8');
  const key = /CONSENT_KEY = '([^']+)'/.exec(model)[1];
  const ver = /CONSENT_VERSION = (\d+)/.exec(model)[1];
  check(html.includes(`localStorage.getItem('${key}')`) && html.includes(`c.version === ${ver}`), 'the inline theme script reads the same key and version as the app');
  check(!/googletagmanager\.com\/gtag|pagead2\.googlesyndication/.test(html), 'index.html loads no Google script');
  check(/google-adsense-account/.test(html), 'index.html keeps the passive AdSense account meta tag');
}

await browser.close();
server.close();
console.log(failures.length ? `\n${failures.length} check(s) failed` : '\nall checks passed');
process.exit(failures.length ? 1 : 0);
