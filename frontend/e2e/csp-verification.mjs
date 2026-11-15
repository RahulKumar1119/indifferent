// Checks what the full Content-Security-Policy in customHttp.yml would block if it were enforced (SEC-05).
//   cd frontend && npx ng build && node scripts/prerender.mjs && node e2e/csp-verification.mjs [--accept]
// Serves dist/frontend/browser as https://indifferent.fun (request interception, so no network is needed), sends the
// policy from customHttp.yml as an ENFORCED header, and records every `securitypolicyviolation` event on each page,
// public and signed-in (a fake API answers as https://api.indifferent.fun). Anything outside the site and the hosts in the
// policy is aborted, so a violation here is the policy's doing, not the network's.
//   --accept   first click "Accept All" on the cookie banner, so Google tag, AdSense and RUM start and are checked too.
// Exit 1 when any page has a violation. Not part of `ng test`.
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '..');
const require = createRequire(path.join(root, '/'));
const puppeteer = require('puppeteer');
const dist = path.join(root, 'dist/frontend/browser');
const ACCEPT = process.argv.includes('--accept');
const SITE = 'https://indifferent.fun';
const API = 'https://api.indifferent.fun';

// The report-only policy from customHttp.yml, without report-uri, sent as an enforced header.
const yml = fs.readFileSync(path.resolve(root, '../customHttp.yml'), 'utf8');
const m = /key: Content-Security-Policy-Report-Only\s+value: >-\n((?:\s{10,}.*\n?)+)/.exec(yml);
if (!m) throw new Error('could not read the report-only policy from customHttp.yml');
const policy = m[1].split('\n').map((l) => l.trim()).filter(Boolean).join(' ').replace(/;?\s*report-uri [^;]*;?\s*$/, '');

const mime = { '.html': 'text/html', '.js': 'text/javascript', '.mjs': 'text/javascript', '.css': 'text/css', '.woff2': 'font/woff2', '.woff': 'font/woff', '.json': 'application/json', '.svg': 'image/svg+xml', '.png': 'image/png', '.ico': 'image/x-icon', '.webp': 'image/webp', '.jpg': 'image/jpeg', '.wasm': 'application/wasm', '.xml': 'application/xml' };
const CORS = { 'access-control-allow-origin': SITE, 'access-control-allow-credentials': 'true', 'access-control-allow-headers': 'Content-Type,Authorization', 'access-control-allow-methods': 'GET,POST,PUT,DELETE,OPTIONS' };
const json = (req, status, data) => req.respond({ status, headers: { ...CORS, 'content-type': 'application/json' }, body: JSON.stringify(data) });
function fakeApi(req) {
  const p = new URL(req.url()).pathname;
  if (req.method() === 'OPTIONS') return req.respond({ status: 204, headers: CORS });
  if (p === '/auth/refresh') return json(req, 200, { accessToken: 'e2e', expiresIn: 900 });
  if (p === '/auth/me') return json(req, 200, { id: 'u1', email: 'e2e@example.com', name: 'E2E', emailVerified: true });
  if (p === '/projects') return json(req, 200, { projects: [] });
  if (p === '/shorts' || p === '/thumbnails') return json(req, 200, { jobs: [], items: [], hasMore: false });
  if (p.endsWith('/limits')) return json(req, 200, { enabled: true });
  if (p === '/story-videos') return json(req, 200, { videos: [], hasMore: false });
  if (p === '/settings/notifications') return json(req, 200, { email: 'e2e@example.com', emailVerified: true, notifyCompleted: true, notifyFailed: true });
  return json(req, 404, { code: 'NOT_FOUND' });
}

const publicRoutes = ['/', '/about', '/contact', '/help', '/pricing', '/privacy', '/terms', '/tools', '/tools/add-watermark', '/tools/infographic-maker', '/shorts-maker', '/features/ai-shorts', '/features/txt-to-video-quiz', '/features/watermark', '/features/story-video', '/features/infographic-maker', '/blog', '/blog/podcast-to-youtube-shorts', '/blog/ai-narration-guide', '/login', '/signup'];
const signedInRoutes = ['/new', '/projects', '/shorts', '/account', '/settings', '/profile', '/thumbnails', '/story'];

const browser = await puppeteer.launch({ executablePath: '/usr/bin/google-chrome', headless: true, args: ['--no-sandbox'] });
const results = [];
async function visit(route, signedIn) {
  const ctx = await browser.createBrowserContext();
  const page = await ctx.newPage();
  await page.setViewport({ width: 1280, height: 900 });
  const violations = [];
  await page.exposeFunction('__csp', (v) => violations.push(v));
  await page.evaluateOnNewDocument((seed) => {
    document.addEventListener('securitypolicyviolation', (e) => window.__csp({ directive: e.effectiveDirective, blocked: e.blockedURI, source: e.sourceFile, line: e.lineNumber, sample: (e.sample || '').slice(0, 60), disposition: e.disposition }));
    if (sessionStorage.getItem('__seeded')) return;
    sessionStorage.setItem('__seeded', '1');
    if (seed.signedIn) localStorage.setItem('indifferent.session', 'e2e');
    if (seed.accepted) localStorage.setItem('indifferent.consent.v1', JSON.stringify({ version: 1, savedAt: new Date().toISOString(), expiresAt: new Date(Date.now() + 86400000 * 100).toISOString(), choices: { analytics: true, advertising: true, functional: true } }));
  }, { signedIn, accepted: ACCEPT });
  await page.setRequestInterception(true);
  page.on('request', (req) => {
    const u = new URL(req.url());
    if (u.origin === API) return fakeApi(req);
    if (u.origin === SITE) {
      let f = path.resolve(dist, '.' + path.posix.normalize('/' + decodeURIComponent(u.pathname)));
      if (!f.startsWith(dist) || !fs.existsSync(f) || fs.statSync(f).isDirectory()) {
        const d = path.join(f, 'index.html');
        f = f.startsWith(dist) && fs.existsSync(d) ? d : path.join(dist, 'index.html');
      }
      const headers = { 'content-type': mime[path.extname(f)] ?? 'application/octet-stream' };
      if (path.extname(f) === '.html') headers['content-security-policy'] = policy;
      return req.respond({ status: 200, headers, body: fs.readFileSync(f) });
    }
    return req.abort();
  });
  try {
    await page.goto(SITE + route, { waitUntil: 'networkidle2', timeout: 30000 });
    await new Promise((r) => setTimeout(r, ACCEPT ? 4500 : 1200)); // the Google tag waits for an interaction or 3 s
    await page.mouse.move(50, 50);
    await page.evaluate(() => window.scrollBy(0, 600));
    await new Promise((r) => setTimeout(r, 600));
  } catch (e) {
    violations.push({ directive: 'navigation', blocked: String(e).slice(0, 80) });
  }
  await ctx.close();
  results.push({ route, signedIn, violations });
}
for (const r of publicRoutes) await visit(r, false);
for (const r of signedInRoutes) await visit(r, true);
await browser.close();

let bad = 0;
const summary = new Map();
for (const { route, signedIn, violations } of results) {
  console.log(`${violations.length ? 'FAIL' : '  ok'} ${signedIn ? '[in] ' : '     '}${route}${violations.length ? `  (${violations.length})` : ''}`);
  for (const v of violations) {
    bad++;
    const k = `${v.directive} ${v.blocked} ${v.sample ?? ''}`;
    summary.set(k, (summary.get(k) ?? 0) + 1);
  }
}
console.log(`\nmode: ${ACCEPT ? 'after Accept All' : 'no consent given'}; policy length ${policy.length}`);
if (bad) {
  console.log('\nDistinct violations:');
  for (const [k, n] of [...summary].sort((a, b) => b[1] - a[1])) console.log(String(n).padStart(4), k);
}
console.log(bad ? `\n${bad} violation(s) on ${results.filter((r) => r.violations.length).length} page(s)` : '\nno violations');
process.exit(bad ? 1 : 0);
