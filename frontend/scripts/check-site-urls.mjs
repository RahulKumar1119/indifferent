/**
 * Build-time check that every page has one address (#361). Run after the
 * prerender, against the files Amplify will serve:
 *
 *   node scripts/check-site-urls.mjs [dist/frontend/browser]
 *
 * Amplify answers 200 for a path only when it is a file, and answers a path to a
 * folder with a 301 to the same path plus a slash. So for every page in the
 * sitemap this checks, on disk:
 *   - the sitemap address ends in a slash and `<route>/index.html` exists, so the
 *     address answers 200 without a redirect;
 *   - the page's canonical tag and og:url are that same address;
 *   - the page is not marked noindex;
 *   - every structured-data URL on the site that names a page uses the slash form;
 *   - every link in the prerendered HTML to a page on this site uses the slash
 *     form (a link without it would make a crawler follow a redirect).
 * The pages that are prerendered but kept out of search (sign-in, persona pages)
 * must carry noindex and a self-referencing canonical too.
 *
 * Plain JavaScript with no dependencies, so it runs on the Amplify build image.
 * It exits 1 and lists every problem when any check fails.
 */
import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { NOINDEX_ROUTES, SITE_URL, pagePath, pageUrl, prerenderRoutes, sitemapRoutes } from './site-routes.mjs';

const DIST = resolve(process.argv[2] ?? 'dist/frontend/browser');
const problems = [];
const fail = (m) => problems.push(m);

if (!existsSync(join(DIST, 'index.html'))) {
  console.error(`dist not found at ${DIST}: run ng build and the prerender first`);
  process.exit(1);
}

const fileFor = (route) => (route === '/' ? join(DIST, 'index.html') : join(DIST, route, 'index.html'));
const attr = (html, re) => re.exec(html)?.[1]?.replace(/&amp;/g, '&');
const canonicalOf = (html) => attr(html, /<link[^>]*rel="canonical"[^>]*href="([^"]*)"/i) ?? attr(html, /<link[^>]*href="([^"]*)"[^>]*rel="canonical"/i);
const ogUrlOf = (html) => attr(html, /<meta[^>]*property="og:url"[^>]*content="([^"]*)"/i);
const robotsOf = (html) => attr(html, /<meta[^>]*name="robots"[^>]*content="([^"]*)"/i) ?? '';

/** Pages the build writes as files: every prerendered route. */
const pageRoutes = new Set(prerenderRoutes());
/** True when path (no query, no fragment) is one of the prerendered pages, in either form. */
const isPage = (path) => pageRoutes.has(path === '/' ? '/' : path.replace(/\/+$/, ''));

// 1. The sitemap file itself.
const sitemapFile = join(DIST, 'sitemap.xml');
if (!existsSync(sitemapFile)) fail('sitemap.xml is not in the build.');
const locs = existsSync(sitemapFile) ? [...readFileSync(sitemapFile, 'utf8').matchAll(/<loc>([^<]+)<\/loc>/g)].map((m) => m[1]) : [];
const expected = sitemapRoutes().map(pageUrl);
if (JSON.stringify(locs) !== JSON.stringify(expected)) {
  fail(`sitemap.xml does not list the expected ${expected.length} addresses (it has ${locs.length}). Run npm run seo:generate.`);
}

// 2. Every sitemap page: served without a redirect, self-referencing, indexable.
for (const loc of locs) {
  if (!loc.startsWith(`${SITE_URL}/`)) {
    fail(`${loc}: not on ${SITE_URL}.`);
    continue;
  }
  const route = loc.slice(SITE_URL.length);
  if (route !== pagePath(route)) fail(`${loc}: does not end in a slash, so Amplify would redirect it.`);
  const file = fileFor(route.replace(/\/$/, '') || '/');
  if (!existsSync(file)) {
    fail(`${loc}: no prerendered file at ${file.slice(DIST.length + 1)}, so it would not answer 200.`);
    continue;
  }
  const html = readFileSync(file, 'utf8');
  const canonical = canonicalOf(html);
  if (canonical !== loc) fail(`${loc}: canonical is ${canonical ?? 'missing'}.`);
  const og = ogUrlOf(html);
  if (og !== loc) fail(`${loc}: og:url is ${og ?? 'missing'}.`);
  if (/noindex/i.test(robotsOf(html))) fail(`${loc}: is in the sitemap but marked ${robotsOf(html)}.`);
}

// 3. Prerendered pages kept out of search: noindex, and a canonical that is their own address.
for (const route of NOINDEX_ROUTES) {
  const file = fileFor(route);
  if (!existsSync(file)) {
    fail(`${route}: not prerendered.`);
    continue;
  }
  const html = readFileSync(file, 'utf8');
  if (!/noindex/i.test(robotsOf(html))) fail(`${route}: should be noindex, robots is "${robotsOf(html)}".`);
  const canonical = canonicalOf(html);
  if (canonical && canonical !== pageUrl(route)) fail(`${route}: canonical is ${canonical}, not ${pageUrl(route)}.`);
}

// 4. Links and structured data in every prerendered page.
function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) walk(full, out);
    else if (name === 'index.html') out.push(full);
  }
  return out;
}
const pathOf = (href) => href.split(/[?#]/)[0];
for (const file of walk(DIST)) {
  const where = file.slice(DIST.length + 1);
  const html = readFileSync(file, 'utf8');
  const bad = new Set();
  for (const m of html.matchAll(/<a\b[^>]*\bhref="([^"]*)"/gi)) {
    let href = m[1].replace(/&amp;/g, '&');
    if (href.startsWith(SITE_URL)) href = href.slice(SITE_URL.length) || '/';
    if (!href.startsWith('/') || href.startsWith('//')) continue;
    const path = pathOf(href);
    if (path === '/' || !isPage(path)) continue; // files, signed-in routes and the home page
    if (!path.endsWith('/')) bad.add(href);
  }
  for (const href of bad) fail(`${where}: links to ${href} without the trailing slash, which redirects.`);
  for (const m of html.matchAll(/<script[^>]*type="application\/ld\+json"[^>]*>([\s\S]*?)<\/script>/gi)) {
    for (const u of m[1].matchAll(/"(https:\/\/indifferent\.fun[^"]*)"/g)) {
      const path = pathOf(u[1].slice(SITE_URL.length)) || '/';
      if (path !== '/' && isPage(path) && !path.endsWith('/')) fail(`${where}: structured data names ${u[1]} without the trailing slash.`);
    }
  }
}

if (problems.length > 0) {
  console.error(`site URL check failed with ${problems.length} problem${problems.length === 1 ? '' : 's'}:`);
  for (const p of problems) console.error(`  - ${p}`);
  process.exit(1);
}
console.log(`site URL check passed: ${locs.length} sitemap addresses answer from a file with a self-referencing canonical and og:url, ${NOINDEX_ROUTES.length} noindex pages are self-referencing, and no prerendered link or structured-data URL needs a redirect`);
