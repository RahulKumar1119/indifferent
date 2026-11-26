// Real-browser check of one address per page (#361). Not part of `ng test`.
//   cd frontend && npx ng build && node scripts/prerender.mjs && node e2e/site-urls-verification.mjs [outDir]
// Serves dist/frontend/browser the way Amplify does: a file answers 200, a folder without a trailing slash answers
// 301 to the slash form, anything else gets the app shell with 200 (the 404-200 rewrite in amplify.yml). Then it
// crawls every sitemap address like a search engine (no JavaScript), and uses the app like a person: following links,
// reloading, going back, opening a signed-in page and the Google sign-in return address. Exit 1 on any failure.
import fs from 'node:fs';
import http from 'node:http';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '..');
const require = createRequire(path.join(root, '/'));
const puppeteer = require('puppeteer');
const outDir = path.resolve(process.argv[2] ?? path.join(process.env.HOME, 'shots303/site-urls'));
fs.mkdirSync(outDir, { recursive: true });
const dist = path.join(root, 'dist/frontend/browser');
if (!fs.existsSync(path.join(dist, 'about', 'index.html'))) throw new Error('Build and prerender first: npx ng build && node scripts/prerender.mjs');

const PORT = 4391;
const ORIGIN = `http://localhost:${PORT}`;
const SITE = 'https://indifferent.fun';
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.woff2': 'font/woff2', '.json': 'application/json', '.svg': 'image/svg+xml', '.png': 'image/png', '.xml': 'application/xml', '.txt': 'text/plain', '.ico': 'image/x-icon', '.jpg': 'image/jpeg', '.webp': 'image/webp' };
const failures = [];
const check = (ok, what) => {
  console.log(`${ok ? '  ok  ' : '  FAIL'} ${what}`);
  if (!ok) failures.push(what);
};
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// Amplify's static hosting, as it behaves for this app.
const served = [];
const server = http.createServer((req, res) => {
  const u = new URL(req.url, ORIGIN);
  const p = decodeURIComponent(u.pathname);
  const file = path.join(dist, p);
  let status = 200;
  let body;
  let type = 'text/html';
  if (!file.startsWith(dist)) status = 400;
  else if (p !== '/' && !p.endsWith('/') && fs.existsSync(file) && fs.statSync(file).isDirectory()) {
    status = 301;
    res.writeHead(301, { location: `${p}/${u.search}` });
    res.end();
    served.push([req.url, 301]);
    return;
  } else if (fs.existsSync(file) && fs.statSync(file).isFile()) {
    body = fs.readFileSync(file);
    type = mime[path.extname(file)] ?? 'application/octet-stream';
  } else if (fs.existsSync(path.join(file, 'index.html'))) {
    body = fs.readFileSync(path.join(file, 'index.html'));
  } else {
    body = fs.readFileSync(path.join(dist, 'index.html')); // 404-200 rewrite
  }
  served.push([req.url, status]);
  res.writeHead(status, { 'content-type': type });
  res.end(body);
});
await new Promise((r) => server.listen(PORT, r));

const raw = (p) => new Promise((resolve) => {
  http.get(`${ORIGIN}${p}`, (res) => {
    let d = '';
    res.on('data', (c) => (d += c));
    res.on('end', () => resolve({ status: res.statusCode, location: res.headers.location, body: d }));
  });
});

console.log('1. Every sitemap address, as a crawler meets it');
const sitemap = fs.readFileSync(path.join(dist, 'sitemap.xml'), 'utf8');
const locs = [...sitemap.matchAll(/<loc>([^<]+)<\/loc>/g)].map((m) => m[1]);
let good = 0;
for (const loc of locs) {
  const p = loc.slice(SITE.length);
  const r = await raw(p);
  const canonical = /rel="canonical" href="([^"]+)"/.exec(r.body)?.[1];
  const og = /property="og:url" content="([^"]+)"/.exec(r.body)?.[1];
  const ok = r.status === 200 && canonical === loc && og === loc;
  if (ok) good++;
  else check(false, `${loc}: ${r.status} canonical=${canonical} og=${og}`);
}
check(good === locs.length && locs.length > 20, `${good} of ${locs.length} sitemap addresses answer 200 without a redirect, with a self-referencing canonical and og:url`);
const bare = await raw('/about');
check(bare.status === 301 && bare.location === '/about/', `the old form /about still lands: 301 to ${bare.location}`);

const browser = await puppeteer.launch({ executablePath: '/usr/bin/google-chrome', headless: true, args: ['--no-sandbox'] });
async function open(p, { width = 1280, height = 900 } = {}) {
  const ctx = await browser.createBrowserContext();
  const page = await ctx.newPage();
  await page.setViewport({ width, height });
  const errors = [];
  page.on('pageerror', (e) => errors.push(String(e)));
  page.on('console', (m) => { if (m.type() === 'error' && !/Failed to load resource|net::ERR_|api\.indifferent\.fun|CORS/i.test(m.text())) errors.push(m.text()); });
  // The API is not here; answer as a signed-out visitor so nothing hangs.
  await page.setRequestInterception(true);
  page.on('request', (req) => {
    const u = new URL(req.url());
    if (u.origin === ORIGIN) return req.continue();
    if (u.hostname === 'api.indifferent.fun') return req.respond({ status: 401, headers: { 'access-control-allow-origin': ORIGIN, 'access-control-allow-credentials': 'true', 'content-type': 'application/json' }, body: '{"code":"UNAUTHORIZED"}' });
    return req.abort();
  });
  // A visitor who has already answered the cookie question, so the banner does not cover the page.
  await page.evaluateOnNewDocument(() => {
    if (location.protocol !== 'http:') return;
    localStorage.setItem('indifferent.consent.v1', JSON.stringify({ version: 1, savedAt: new Date().toISOString(), expiresAt: new Date(Date.now() + 864e7).toISOString(), choices: { analytics: false, advertising: false, functional: false } }));
  });
  await page.goto(`${ORIGIN}${p}`, { waitUntil: 'networkidle2', timeout: 30000 });
  await sleep(300);
  return { ctx, page, errors };
}
const here2 = (page) => page.evaluate(() => location.pathname + location.search + location.hash);

console.log('2. Using the site');
{
  const { ctx, page, errors } = await open('/');
  const hrefs = await page.$$eval('a[href^="/"]', (as) => as.map((a) => a.getAttribute('href')));
  const pages = hrefs.filter((h) => !/\.(xml|txt|png|jpg|svg)$/.test(h.split(/[?#]/)[0]) && h !== '/' && !h.startsWith('/#'));
  const noSlash = pages.filter((h) => !h.split(/[?#]/)[0].endsWith('/'));
  check(pages.length > 10 && noSlash.length === 0, `all ${pages.length} links to pages on the home page end in a slash${noSlash.length ? ` (not: ${noSlash.join(', ')})` : ''}`);
  served.length = 0;
  // The first About link may be inside a closed menu; use the one in the footer, which is always shown.
  const visible = await page.$$eval('a[href="/about/"]', (as) => as.findIndex((a) => a.offsetParent !== null && a.getBoundingClientRect().height > 0));
  check(visible >= 0, 'there is a visible About link');
  const links = await page.$$('a[href="/about/"]');
  await links[visible].evaluate((a) => {
    a.scrollIntoView();
    a.click();
  });
  await page.waitForFunction(() => location.pathname !== '/', { timeout: 5000 }).catch(() => null);
  await sleep(300);
  check((await here2(page)) === '/about/', `following the About link shows /about/ (${await here2(page)})`);
  check(served.every(([u]) => !/^\/about$/.test(u)), 'and the app never asks for /about without the slash');
  check((await page.$eval('h1', (h) => h.textContent.trim()).catch(() => '')).length > 0, 'the About page shows its heading');
  await page.goBack();
  await page.waitForFunction(() => location.pathname === '/', { timeout: 5000 }).catch(() => null);
  await sleep(300);
  check((await here2(page)) === '/', 'Back returns to the home page');
  await page.goto(`${ORIGIN}/blog/podcast-to-youtube-shorts/`, { waitUntil: 'networkidle2' });
  await sleep(300);
  check((await here2(page)) === '/blog/podcast-to-youtube-shorts/', 'reloading an article with the slash keeps the address');
  check((await page.$eval('link[rel="canonical"]', (l) => l.href)) === `${SITE}/blog/podcast-to-youtube-shorts/`, 'and its canonical after the app has run is the slash form');
  check((await page.$eval('meta[property="og:url"]', (l) => l.content)) === `${SITE}/blog/podcast-to-youtube-shorts/`, 'and so is og:url');
  await page.goto(`${ORIGIN}/blog/podcast-to-youtube-shorts`, { waitUntil: 'networkidle2' });
  await sleep(300);
  check((await here2(page)) === '/blog/podcast-to-youtube-shorts/', 'an old link without the slash lands on the slash form');
  await page.goto(`${ORIGIN}/terms/#stock`, { waitUntil: 'networkidle2' });
  await sleep(300);
  check((await here2(page)) === '/terms/#stock', `a link to a part of a page keeps its fragment (${await here2(page)})`);
  check(errors.length === 0, `no console errors (${errors.join(' | ')})`);
  await page.screenshot({ path: path.join(outDir, 'terms-stock.png') });
  await ctx.close();
}

console.log('3. Pages that are not prerendered still work');
{
  const { ctx, page, errors } = await open('/dashboard');
  const at = await here2(page);
  check(at.startsWith('/login/'), `a signed-in page sends a visitor to sign in (${at})`);
  check((await page.$eval('meta[name="robots"]', (m) => m.content)) === 'noindex, follow' || (await page.$eval('meta[name="robots"]', (m) => m.content)).includes('noindex'), 'and the sign-in page is noindex');
  await page.goto(`${ORIGIN}/dashboard/`, { waitUntil: 'networkidle2' });
  await sleep(300);
  check((await here2(page)).startsWith('/login/'), 'the slash form of a signed-in page does the same');
  await ctx.close();
}

console.log('3b. The Google sign-in return address, registered without a slash');
{
  const ctx = await browser.createBrowserContext();
  const page = await ctx.newPage();
  const errors = [];
  page.on('pageerror', (e) => errors.push(String(e)));
  let sent = null;
  const cors = { 'access-control-allow-origin': ORIGIN, 'access-control-allow-credentials': 'true', 'access-control-allow-headers': 'Content-Type,Authorization', 'access-control-allow-methods': 'GET,POST,OPTIONS' };
  await page.setRequestInterception(true);
  page.on('request', (req) => {
    const u = new URL(req.url());
    if (u.origin === ORIGIN) return req.continue();
    if (u.hostname !== 'api.indifferent.fun') return req.abort();
    if (req.method() === 'OPTIONS') return req.respond({ status: 204, headers: cors });
    const json = (status, body) => req.respond({ status, headers: { ...cors, 'content-type': 'application/json' }, body: JSON.stringify(body) });
    if (u.pathname === '/auth/google/callback') {
      sent = JSON.parse(req.postData() || '{}');
      return json(200, { accessToken: 'e2e', expiresIn: 900 });
    }
    if (u.pathname === '/auth/me') return json(200, { id: 'u1', email: 'e2e@example.com', name: 'E2E', emailVerified: true });
    if (u.pathname === '/auth/refresh') return json(401, { code: 'UNAUTHORIZED' });
    return json(200, {});
  });
  // The browser started a sign-in: the flow the app saves before it leaves for Google, and the page to return to.
  await page.goto(`${ORIGIN}/robots.txt`);
  await page.evaluate(() => {
    sessionStorage.setItem('indifferent.oauthFlow', JSON.stringify({ state: 'state-from-this-browser-01', verifier: 'v'.repeat(43), createdAt: Date.now() }));
    sessionStorage.setItem('indifferent.postLoginNext', '/dashboard');
    localStorage.setItem('indifferent.consent.v1', JSON.stringify({ version: 1, savedAt: new Date().toISOString(), expiresAt: new Date(Date.now() + 864e7).toISOString(), choices: { analytics: false, advertising: false, functional: false } }));
  });
  await page.goto(`${ORIGIN}/auth/callback?code=abc&state=state-from-this-browser-01`, { waitUntil: 'networkidle2' });
  await page.waitForFunction(() => !location.pathname.startsWith('/auth/'), { timeout: 8000 }).catch(() => null);
  await sleep(300);
  check(sent?.code === 'abc' && typeof sent?.codeVerifier === 'string', `the code from Google reaches the API (${JSON.stringify(sent)})`);
  const at = await page.evaluate(() => location.pathname);
  check(at === '/dashboard/', `and the person lands where they were going (${at})`);
  check(errors.length === 0, `no page errors (${errors.join(' | ')})`);
  await ctx.close();
}

console.log('3c. Unknown pages');
{
  const { ctx, page, errors } = await open('/no-such-page/');
  check(/not found|404|can.t find/i.test(await page.evaluate(() => document.body.innerText)), 'an unknown page shows the not-found page');
  check(errors.length === 0, `no console errors (${errors.join(' | ')})`);
  await ctx.close();
}

console.log('4. Phone layout, a feature page');
{
  const { ctx, page, errors } = await open('/features/story-video/', { width: 390, height: 844 });
  check((await here2(page)) === '/features/story-video/', 'the feature page opens at its slash address');
  check((await page.$eval('link[rel="canonical"]', (l) => l.href)) === `${SITE}/features/story-video/`, 'with its own canonical');
  const over = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  check(over <= 0, `no sideways scroll (${over}px)`);
  await page.screenshot({ path: path.join(outDir, 'feature-story-phone.png'), fullPage: false });
  check(errors.length === 0, `no console errors (${errors.join(' | ')})`);
  await ctx.close();
}

await browser.close();
server.close();
if (failures.length) {
  console.log(`\n${failures.length} FAILED`);
  process.exit(1);
}
console.log('\nAll checks passed');
