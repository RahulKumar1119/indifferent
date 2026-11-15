// Real-browser check of the refresh-token cookie flow (SEC-04). Not part of `ng test`.
//   cd frontend && npx ng build && node e2e/session-cookie-verification.mjs
// The page is served as https://indifferent.fun and a small fake of the API answers as
// https://api.indifferent.fun, both through request interception, so Chrome applies its real cookie rules:
// the HttpOnly, Secure and SameSite=Strict flags, the /auth path, credentials on fetch. The fake follows what
// the real API does (backend/internal/api/refreshcookie.go): it sets indifferent_rt on sign-in and refresh,
// reads the cookie (or an older body token), rotates it, and clears it on logout. It cannot show that the real
// API Gateway behaves the same; the live check of that is a bogus-token request (see the pull request).
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '..');
const require = createRequire(path.join(root, '/'));
const puppeteer = require('puppeteer');
const dist = path.join(root, 'dist/frontend/browser');
if (!fs.existsSync(path.join(dist, 'index.html'))) throw new Error('Build first: npx ng build');
const SITE = 'https://indifferent.fun';
const API = 'https://api.indifferent.fun';
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.mjs': 'text/javascript', '.css': 'text/css', '.woff2': 'font/woff2', '.woff': 'font/woff', '.json': 'application/json', '.svg': 'image/svg+xml', '.png': 'image/png', '.ico': 'image/x-icon', '.webp': 'image/webp', '.jpg': 'image/jpeg' };
const failures = [];
const check = (ok, what) => {
  console.log(`${ok ? '  ok  ' : '  FAIL'} ${what}`);
  if (!ok) failures.push(what);
};
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const CORS = { 'access-control-allow-origin': SITE, 'access-control-allow-credentials': 'true', 'access-control-allow-headers': 'Content-Type,Authorization', 'access-control-allow-methods': 'GET,POST,PUT,DELETE,OPTIONS' };

/** One fake API per scenario, with the log of what the page sent. */
function makeApi() {
  const state = { n: 0, valid: new Set(), calls: [] };
  const issue = () => {
    const t = `rt-${++state.n}`;
    state.valid.add(t);
    return t;
  };
  const cookieOf = (req) => /(?:^|;\s*)indifferent_rt=([^;]*)/.exec(req.headers()['cookie'] ?? '')?.[1] ?? '';
  const body = (req) => {
    try {
      return JSON.parse(req.postData() || '{}');
    } catch {
      return {};
    }
  };
  const setCookie = (t, clear = false) => `indifferent_rt=${clear ? '' : t}; Path=/auth; ${clear ? 'Max-Age=0' : 'Max-Age=604800'}; HttpOnly; Secure; SameSite=Strict`;
  const json = (req, status, data, extra = {}) => req.respond({ status, headers: { ...CORS, 'content-type': 'application/json', ...extra }, body: JSON.stringify(data) });
  const tokens = (t) => ({ accessToken: 'at-' + t, refreshToken: t, expiresIn: 900 });
  state.handle = (req) => {
    const p = new URL(req.url()).pathname;
    if (req.method() === 'OPTIONS') return req.respond({ status: 204, headers: CORS });
    if (p === '/auth/login') {
      const t = issue();
      state.calls.push({ p, cookie: cookieOf(req), body: body(req) });
      return json(req, 200, tokens(t), { 'set-cookie': setCookie(t) });
    }
    if (p === '/auth/refresh') {
      const fromCookie = cookieOf(req);
      const b = body(req);
      const presented = fromCookie || b.refreshToken || '';
      state.calls.push({ p, cookie: fromCookie, body: b });
      if (!presented || !state.valid.has(presented)) return json(req, 401, { code: 'REFRESH_FAILED' }, fromCookie ? { 'set-cookie': setCookie('', true) } : {});
      state.valid.delete(presented);
      const t = issue();
      return json(req, 200, tokens(t), { 'set-cookie': setCookie(t) });
    }
    if (p === '/auth/logout') {
      const presented = cookieOf(req) || body(req).refreshToken || '';
      state.calls.push({ p, cookie: cookieOf(req), body: body(req) });
      state.valid.delete(presented);
      return req.respond({ status: 204, headers: { ...CORS, 'set-cookie': setCookie('', true) } });
    }
    if (p === '/auth/me') return json(req, 200, { id: 'u1', email: 'e2e@example.com', name: 'E2E', emailVerified: true });
    if (p === '/projects' || p === '/shorts' || p === '/thumbnails') return json(req, 200, p === '/projects' ? { projects: [] } : { jobs: [], items: [], hasMore: false });
    return json(req, 404, { code: 'NOT_FOUND' });
  };
  return state;
}

const browser = await puppeteer.launch({ executablePath: '/usr/bin/google-chrome', headless: true, args: ['--no-sandbox', '--host-resolver-rules=MAP * ~NOTFOUND , EXCLUDE localhost'] });
async function visitor(api, { legacy = null } = {}) {
  const ctx = await browser.createBrowserContext();
  const page = await ctx.newPage();
  await page.setViewport({ width: 1280, height: 900 });
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
  // Skip the consent banner: this check is about sign-in.
  await page.evaluateOnNewDocument((legacyToken) => {
    if (sessionStorage.getItem('__seeded')) return;
    sessionStorage.setItem('__seeded', '1');
    localStorage.setItem('indifferent.consent.v1', JSON.stringify({ version: 1, savedAt: new Date().toISOString(), expiresAt: new Date(Date.now() + 86400000 * 100).toISOString(), choices: { analytics: false, advertising: false, functional: false } }));
    if (legacyToken) localStorage.setItem('indifferent.refreshToken', legacyToken);
  }, legacy);
  return { ctx, page };
}
// The cookie's path is /auth, so ask for it with a URL under /auth.
const cookiesFor = (page) => page.cookies(API + '/auth/refresh');
const storage = (page) => page.evaluate(() => JSON.stringify([Object.entries(localStorage), Object.entries(sessionStorage)]));

console.log('First visit, never signed in');
{
  const api = makeApi();
  const { ctx, page } = await visitor(api);
  await page.goto(SITE + '/', { waitUntil: 'networkidle2' });
  await sleep(500);
  check(api.calls.filter((c) => c.p === '/auth/refresh').length === 0, 'no refresh call for a visitor with no sign-in');
  await ctx.close();
}

console.log('Sign in with a password');
let signedIn;
{
  const api = makeApi();
  const { ctx, page } = await visitor(api);
  await page.goto(SITE + '/login', { waitUntil: 'networkidle2' });
  await sleep(500);
  const filled = await page.evaluate(() => {
    const email = document.querySelector('input[type=email]');
    const pw = document.querySelector('input[type=password]');
    if (!email || !pw) return false;
    for (const [el, v] of [[email, 'e2e@example.com'], [pw, 'a-long-enough-pass']]) {
      el.value = v;
      el.dispatchEvent(new Event('input', { bubbles: true }));
    }
    return true;
  });
  check(filled, 'the login form has an email and a password field');
  if (filled) {
    await page.evaluate(() => {
      const btn = [...document.querySelectorAll('button[type=submit], form button')].find((b) => /sign in|log in|continue/i.test(b.textContent));
      (btn ?? document.querySelector('form button'))?.click();
    });
    await sleep(1500);
  }
  const jar = await cookiesFor(page);
  const c = jar.find((x) => x.name === 'indifferent_rt');
  check(!!c, 'the browser stored the sign-in cookie');
  check(!!c && c.httpOnly && c.secure && c.sameSite === 'Strict' && c.path === '/auth', `cookie flags: HttpOnly, Secure, SameSite=Strict, Path=/auth (${c ? [c.httpOnly, c.secure, c.sameSite, c.path].join(' ') : 'none'})`);
  check(!(await page.evaluate(() => document.cookie.includes('indifferent_rt'))), 'a script on the page cannot read the cookie');
  const s = await storage(page);
  check(!/rt-\d/.test(s), 'the refresh token is in neither localStorage nor sessionStorage');
  check(!!(await page.evaluate(() => localStorage.getItem('indifferent.session'))), 'a non-secret marker records that a sign-in exists');
  signedIn = { api, ctx, page };
}

console.log('Reload restores the session from the cookie');
{
  const { api, page } = signedIn;
  const before = api.calls.filter((c) => c.p === '/auth/refresh').length;
  await page.goto(SITE + '/', { waitUntil: 'networkidle2' });
  await sleep(800);
  const refreshes = api.calls.filter((c) => c.p === '/auth/refresh');
  check(refreshes.length === before + 1, 'one refresh call on reload');
  const last = refreshes.at(-1);
  check(/^rt-\d+$/.test(last.cookie), 'the browser sent the cookie');
  check(Object.keys(last.body).length === 0, 'the request body carries no token');
  const jar = await cookiesFor(page);
  check(jar.find((x) => x.name === 'indifferent_rt')?.value !== last.cookie, 'the cookie was replaced by the refresh (rotation)');
}

console.log('Sign out');
{
  const { api, page } = signedIn;
  await page.goto(SITE + '/account', { waitUntil: 'networkidle2' });
  await sleep(800);
  const clicked = await page.evaluate(() => {
    const b = [...document.querySelectorAll('button')].find((x) => x.textContent.trim() === 'Sign out');
    if (!b) return false;
    b.click();
    return true;
  });
  await sleep(1200);
  if (clicked) {
    check(api.calls.some((c) => c.p === '/auth/logout'), 'sign out calls /auth/logout');
    const jar = await cookiesFor(signedIn.page);
    check(!jar.find((x) => x.name === 'indifferent_rt'), 'the cookie is gone after sign out');
    check(!(await signedIn.page.evaluate(() => localStorage.getItem('indifferent.session'))), 'the marker is gone after sign out');
  } else {
    console.log('  skip no sign-out control found on this page (covered by the unit test)');
  }
  await signedIn.ctx.close();
}

console.log('A browser that still holds an older version\'s token');
{
  const api = makeApi();
  const legacy = 'legacy-rt';
  api.valid.add(legacy);
  const { ctx, page } = await visitor(api, { legacy });
  await page.goto(SITE + '/', { waitUntil: 'networkidle2' });
  await sleep(800);
  const first = api.calls.find((c) => c.p === '/auth/refresh');
  check(first && first.body.refreshToken === legacy, 'the old token is sent once, in the body');
  const jar = await cookiesFor(page);
  check(!!jar.find((x) => x.name === 'indifferent_rt'), 'the sign-in moved to the cookie');
  check(!(await page.evaluate(() => localStorage.getItem('indifferent.refreshToken'))), 'the old token is deleted from localStorage');
  await page.goto(SITE + '/', { waitUntil: 'networkidle2' });
  await sleep(600);
  const second = api.calls.filter((c) => c.p === '/auth/refresh').at(-1);
  check(/^rt-\d+$/.test(second.cookie) && !second.body.refreshToken, 'the next load uses the cookie only');
  await ctx.close();
}

console.log('A cookie the server no longer accepts');
{
  const api = makeApi();
  const { ctx, page } = await visitor(api);
  await page.setCookie({ name: 'indifferent_rt', value: 'dead', domain: 'api.indifferent.fun', path: '/auth', secure: true, httpOnly: true, sameSite: 'Strict' });
  await page.evaluateOnNewDocument(() => localStorage.setItem('indifferent.session', 'x'));
  await page.goto(SITE + '/', { waitUntil: 'networkidle2' });
  await sleep(800);
  const jar = await cookiesFor(page);
  check(!jar.find((x) => x.name === 'indifferent_rt'), 'a rejected cookie is cleared');
  check(!(await page.evaluate(() => localStorage.getItem('indifferent.session'))), 'and the marker with it, so the next visit makes no call');
  await ctx.close();
}

await browser.close();
console.log(failures.length ? `\n${failures.length} check(s) failed` : '\nall checks passed');
process.exit(failures.length ? 1 : 0);
