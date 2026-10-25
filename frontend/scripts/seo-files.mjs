/**
 * Generates public/sitemap.xml and public/feed.xml from the route list and the
 * journal registry, and checks the SEO files against the app.
 *
 *   node scripts/seo-files.mjs          write sitemap.xml and feed.xml
 *   node scripts/seo-files.mjs --check  fail when a file is stale or a rule is broken
 *
 * npm run seo:generate and npm run verify:seo wrap the two modes. The check
 * needs Node 22.18 or newer: it reads product-facts.ts directly.
 */
import { existsSync, readdirSync, readFileSync, statSync, writeFileSync } from 'node:fs';
import { join, relative } from 'node:path';
import { pathToFileURL } from 'node:url';
import { FRONTEND_DIR, INDEXED_ROUTES, NOINDEX_ROUTES, SITE_URL, loadArticles, sitemapRoutes } from './site-routes.mjs';

const PUBLIC = join(FRONTEND_DIR, 'public');
const SRC = join(FRONTEND_DIR, 'src');
const APP = join(SRC, 'app');
const read = (p) => readFileSync(p, 'utf8');
const rel = (p) => relative(FRONTEND_DIR, p);

const xmlEscape = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const rfc822 = (iso) => new Date(`${iso}T00:00:00Z`).toUTCString();

// ---------------------------------------------------------------- generation

/** Last-modified dates the code knows for certain. Pages without an entry get no lastmod. */
function lastModified() {
  const articles = loadArticles();
  const map = new Map();
  for (const a of articles) map.set(`/blog/${a.slug}`, a.updated);
  map.set('/blog', articles.reduce((max, a) => (a.updated > max ? a.updated : max), '0000-00-00'));
  const legal = /LEGAL_LAST_UPDATED\s*=\s*'(\d{4}-\d{2}-\d{2})'/.exec(read(join(APP, 'core/config/legal.ts')));
  if (legal) {
    map.set('/privacy', legal[1]);
    map.set('/terms', legal[1]);
  }
  return map;
}

export function buildSitemap() {
  const dates = lastModified();
  const urls = sitemapRoutes().map((route) => {
    const loc = route === '/' ? `${SITE_URL}/` : `${SITE_URL}${route}`;
    const last = dates.get(route);
    return `  <url>\n    <loc>${xmlEscape(loc)}</loc>${last ? `\n    <lastmod>${last}</lastmod>` : ''}\n  </url>`;
  });
  return `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${urls.join('\n')}\n</urlset>\n`;
}

export function buildFeed() {
  const articles = loadArticles();
  const built = articles.reduce((max, a) => (a.updated > max ? a.updated : max), '0000-00-00');
  const items = articles.map((a) => {
    const url = `${SITE_URL}/blog/${a.slug}`;
    return [
      '    <item>',
      `      <title>${xmlEscape(a.title)}</title>`,
      `      <link>${url}</link>`,
      `      <guid isPermaLink="true">${url}</guid>`,
      `      <pubDate>${rfc822(a.published)}</pubDate>`,
      `      <category>${xmlEscape(a.category)}</category>`,
      `      <description>${xmlEscape(a.description)}</description>`,
      '    </item>',
    ].join('\n');
  });
  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">',
    '  <channel>',
    '    <title>Indifferent journal</title>',
    `    <link>${SITE_URL}/blog</link>`,
    '    <description>Guides to the AI Shorts generator, the TXT to quiz video generator and the watermark tool.</description>',
    '    <language>en</language>',
    `    <lastBuildDate>${rfc822(built)}</lastBuildDate>`,
    `    <atom:link href="${SITE_URL}/feed.xml" rel="self" type="application/rss+xml"/>`,
    ...items,
    '  </channel>',
    '</rss>',
    '',
  ].join('\n');
}

// ------------------------------------------------------------------- checking

function walk(dir, filter, out = []) {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) walk(full, filter, out);
    else if (filter(full)) out.push(full);
  }
  return out;
}

/** Route table from app.routes.ts: every `path: '...'` except the catch-all. */
function appRoutePatterns() {
  const source = read(join(APP, 'app.routes.ts'));
  const paths = [...source.matchAll(/path:\s*'([^']*)'/g)].map((m) => m[1]).filter((p) => p !== '**');
  return paths.map((p) => ({
    path: p,
    regex: new RegExp(`^/${p.split('/').map((seg) => (seg.startsWith(':') ? '[^/]+' : seg.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))).join('/')}$`),
    params: p.split('/').filter((seg) => seg.startsWith(':')),
  }));
}

function knownSlugs(articles) {
  const grab = (file, re) => new Set([...read(join(APP, file)).matchAll(re)].map((m) => m[1]));
  return {
    features: grab('pages/features/feature-page.component.ts', /slug:\s*'([a-z-]+)',\s*\n\s*title:/g),
    useCases: grab('pages/use-cases/use-case.component.ts', /slug:\s*'([a-z-]+)',\s*\n\s*title:/g),
    articles: new Set(articles.map((a) => a.slug)),
  };
}

function matchesRule(rule, path) {
  const anchored = rule.endsWith('$');
  const prefix = anchored ? rule.slice(0, -1) : rule;
  return anchored ? path === prefix : path.startsWith(prefix);
}

export async function check() {
  const problems = [];
  const fail = (msg) => problems.push(msg);
  const articles = loadArticles();
  const routes = appRoutePatterns();
  const slugs = knownSlugs(articles);

  // 1. Generated files are current.
  for (const [file, build] of [['sitemap.xml', buildSitemap], ['feed.xml', buildFeed]]) {
    const path = join(PUBLIC, file);
    if (!existsSync(path)) fail(`public/${file} is missing. Run npm run seo:generate.`);
    else if (read(path) !== build()) fail(`public/${file} is out of date. Run npm run seo:generate.`);
  }

  // 2. Every sitemap, noindex and prerender route resolves to a real page.
  const resolves = (route) => {
    const hit = routes.find((r) => r.regex.test(route));
    if (!hit) return false;
    const last = route.split('/').pop();
    if (hit.path === 'features/:slug') return slugs.features.has(last);
    if (hit.path === 'use-cases/:slug') return slugs.useCases.has(last);
    if (hit.path === 'blog/:slug') return slugs.articles.has(last);
    return true;
  };
  for (const route of [...INDEXED_ROUTES, ...NOINDEX_ROUTES, ...sitemapRoutes()]) {
    if (!resolves(route)) fail(`Route ${route} is listed for the sitemap or prerender but the app has no page for it.`);
  }
  for (const route of NOINDEX_ROUTES) {
    if (INDEXED_ROUTES.includes(route)) fail(`${route} is both indexed and noindex.`);
  }

  // 3. robots.txt keeps private areas closed and public pages open.
  const robots = read(join(PUBLIC, 'robots.txt'));
  if (!robots.includes(`Sitemap: ${SITE_URL}/sitemap.xml`)) fail('robots.txt does not point at the sitemap.');
  const disallows = [...robots.matchAll(/^Disallow:\s*(\S+)/gm)].map((m) => m[1]);
  for (const route of sitemapRoutes()) {
    const blocked = disallows.find((rule) => matchesRule(rule, route));
    if (blocked) fail(`robots.txt rule "Disallow: ${blocked}" blocks the indexed page ${route}.`);
  }
  for (const privateRoute of ['/dashboard', '/projects/1', '/new', '/shorts', '/shorts/abc/clips', '/account', '/profile', '/settings', '/auth/callback']) {
    if (!disallows.some((rule) => matchesRule(rule, privateRoute))) fail(`robots.txt does not block the signed-in page ${privateRoute}.`);
  }

  // 4. ads.txt matches the AdSense publisher in index.html.
  const indexHtml = read(join(SRC, 'index.html'));
  const pub = /ca-pub-(\d+)/.exec(indexHtml)?.[1];
  const adsPath = join(PUBLIC, 'ads.txt');
  if (!pub) fail('index.html has no AdSense publisher id.');
  else if (!existsSync(adsPath)) fail('public/ads.txt is missing.');
  else if (!read(adsPath).split('\n').some((l) => l.trim() === `google.com, pub-${pub}, DIRECT, f08c47fec0942fa0`)) {
    fail(`public/ads.txt has no DIRECT line for pub-${pub}.`);
  }

  // 5. No ad units on tool or signed-in pages.
  for (const file of walk(join(APP, 'pages'), (f) => f.endsWith('.ts') && !f.endsWith('.spec.ts'))) {
    if (/class="adsbygoogle"|<ins[^>]*adsbygoogle/.test(read(file))) fail(`${rel(file)} contains an ad unit. Ads must not sit on tool or signed-in pages.`);
  }

  // 6. Journal registry.
  const seen = new Set();
  const legacyFiles = readdirSync(join(APP, 'pages/blog/articles'));
  const routesSource = read(join(APP, 'app.routes.ts'));
  for (const a of articles) {
    const where = `article ${a.slug}`;
    if (seen.has(a.slug)) fail(`${where}: duplicate slug.`);
    seen.add(a.slug);
    if (!/^[a-z0-9]+(-[a-z0-9]+)*$/.test(a.slug)) fail(`${where}: slug is not kebab-case.`);
    for (const key of ['published', 'updated']) {
      if (!/^\d{4}-\d{2}-\d{2}$/.test(a[key]) || Number.isNaN(Date.parse(a[key]))) fail(`${where}: ${key} is not a date.`);
    }
    if (a.updated < a.published) fail(`${where}: updated is before published.`);
    if (a.published > new Date().toISOString().slice(0, 10)) fail(`${where}: published date is in the future.`);
    if (a.description.length < 60 || a.description.length > 170) fail(`${where}: description is ${a.description.length} characters (want 60 to 170).`);
    if (a.title.length > 70) fail(`${where}: title is ${a.title.length} characters (want 70 or fewer).`);
    if (!existsSync(join(PUBLIC, a.cover))) fail(`${where}: cover ${a.cover} is not in public/.`);
    if (a.coverAlt.trim().length < 15) fail(`${where}: coverAlt is too short to describe the image.`);
    if (a.legacy) {
      if (!legacyFiles.includes(`${a.slug}.component.ts`)) fail(`${where}: legacy article has no component file.`);
      if (!routesSource.includes(`path: 'blog/${a.slug}'`)) fail(`${where}: legacy article has no explicit route.`);
    } else if (!legacyFiles.includes('content') || !existsSync(join(APP, 'pages/blog/articles/content', `${a.slug}.ts`))) {
      fail(`${where}: no content module in articles/content/.`);
    }
  }
  const contentDir = join(APP, 'pages/blog/articles/content');
  for (const file of existsSync(contentDir) ? readdirSync(contentDir) : []) {
    if (!seen.has(file.replace(/\.ts$/, ''))) fail(`articles/content/${file} has no entry in articles.json.`);
  }

  // 7. Internal links in pages and article content resolve.
  const sources = walk(APP, (f) => f.endsWith('.ts') && !f.endsWith('.spec.ts'));
  const linkPatterns = [/routerLink="(\/[^"#?]*)/g, /href="(\/[^"#?]*)"/g, /\]\((\/[^)\s#?]*)\)/g];
  for (const file of sources) {
    // Comments document syntax (for example the article inline-link format), so skip them.
    const text = read(file)
      .split('\n')
      .filter((line) => !/^\s*(\/\/|\*|\/\*)/.test(line))
      .join('\n');
    for (const re of linkPatterns) {
      for (const m of text.matchAll(re)) {
        const target = m[1] === '/' ? '/' : m[1].replace(/\/$/, '');
        if (target === '/') continue;
        if (existsSync(join(PUBLIC, target))) continue;
        if (!resolves(target)) fail(`${rel(file)} links to ${target}, which is neither a page nor a file in public/.`);
      }
    }
  }

  // 8. No placeholder text or hosts, and no em dashes in public copy.
  const publicCopy = [
    ...walk(join(APP, 'pages'), (f) => f.endsWith('.ts') && !f.endsWith('.spec.ts') && !/\/(profile|shorts|thumbnails|projects|create-project|dashboard|new|preview|progress|settings|account)\//.test(f)),
    join(APP, 'shared/components/site-footer/site-footer.component.ts'),
    join(SRC, 'index.html'),
    join(APP, 'pages/blog/articles/articles.json'),
  ];
  const banned = /lorem ipsum|picsum\.photos|via\.placeholder\.com|\bTODO\b|\bFIXME\b|coming soon|example\.com/i;
  for (const file of publicCopy) {
    const lines = read(file).split('\n');
    lines.forEach((line, i) => {
      const trimmed = line.trim();
      const isComment = /^(\/\/|\*|\/\*)/.test(trimmed) || trimmed.startsWith('<!--');
      if (!isComment && banned.test(line)) fail(`${rel(file)}:${i + 1} has placeholder text or a placeholder host.`);
      if (!isComment && line.includes('\u2014')) fail(`${rel(file)}:${i + 1} has an em dash in public copy.`);
    });
  }

  // 9. The static summary in index.html matches the product facts.
  try {
    const { PRODUCT_FACTS, durationLabel } = await import(pathToFileURL(join(APP, 'core/config/product-facts.ts')).href);
    const noscript = [...indexHtml.matchAll(/<noscript>([\s\S]*?)<\/noscript>/g)].map((m) => m[1]).join('\n');
    const s = PRODUCT_FACTS.shorts;
    const q = PRODUCT_FACTS.quiz;
    const expected = [
      `${durationLabel(s.maxDurationMinutes)} and ${s.maxUploadGb} GB`,
      `up to ${s.maxClips} moments of ${s.clipMinSeconds} to ${s.clipMaxSeconds} seconds`,
      `${s.outputWidth}x${s.outputHeight}`,
      `up to ${q.maxFileMb} MB`,
      `${q.designs.length} designs`,
      `${q.voices.length} narration voices`,
      `${PRODUCT_FACTS.watermark.imageFormats.join(', ').replace(/, ([^,]+)$/, ' or $1')} image`,
    ];
    for (const phrase of expected) {
      if (!noscript.includes(phrase)) fail(`index.html noscript summary does not contain "${phrase}", which product-facts.ts requires.`);
    }
  } catch (err) {
    fail(`Could not read product-facts.ts (Node 22.18 or newer is needed): ${err.message}`);
  }

  return problems;
}

// ---------------------------------------------------------------------- main

if (process.argv[1] && pathToFileURL(process.argv[1]).href === import.meta.url) {
  if (process.argv.includes('--check')) {
    const problems = await check();
    if (problems.length > 0) {
      console.error(`seo check failed with ${problems.length} problem${problems.length === 1 ? '' : 's'}:`);
      for (const p of problems) console.error(`  - ${p}`);
      process.exit(1);
    }
    console.log(`seo check passed: ${sitemapRoutes().length} sitemap URLs, ${loadArticles().length} articles, robots, ads.txt and links verified`);
  } else {
    writeFileSync(join(PUBLIC, 'sitemap.xml'), buildSitemap());
    writeFileSync(join(PUBLIC, 'feed.xml'), buildFeed());
    console.log(`wrote public/sitemap.xml (${sitemapRoutes().length} URLs) and public/feed.xml (${loadArticles().length} items)`);
  }
}
