/**
 * Post-build prerenderer (no SSR required).
 *
 * Boots headless Chrome against the production `dist` output, renders each
 * public route, and saves the resulting DOM as static `index.html` files.
 * Amplify then serves real files (HTTP 200 + full content) instead of relying
 * on the SPA fallback rewrite — which is what Googlebot needs to index us.
 *
 * Auth-gated routes are intentionally excluded: they need a live session.
 *
 * Usage: `node scripts/prerender.mjs [dist/frontend/browser]`
 */
import { createServer } from 'node:http';
import { readFile, stat } from 'node:fs/promises';
import { existsSync, mkdirSync, writeFileSync } from 'node:fs';
import { join, dirname, extname, resolve } from 'node:path';
import puppeteer from 'puppeteer';

const DIST = resolve(process.argv[2] ?? 'dist/frontend/browser');
const PORT = 4329;

// Public, indexable routes. Parameterized SEO pages are expanded explicitly.
const ROUTES = [
  '/',
  '/login',
  '/about',
  '/contact',
  '/privacy',
  '/terms',
  '/tools',
  '/tools/add-watermark',
  '/blog',
  '/blog/quiz-videos-youtube',
  '/blog/educational-video-best-practices',
  '/blog/quiz-file-format-guide',
  '/blog/video-template-comparison',
  '/blog/ai-narration-guide',
  '/features/ai-shorts',
  '/features/watermark',
  '/features/txt-to-video-quiz',
  '/shorts-maker',
  '/use-cases/content-creators',
  '/use-cases/marketing-teams',
  '/use-cases/agencies',
  '/use-cases/coaches',
  '/use-cases/media-companies',
  '/use-cases/educators',
];

const MIME = {
  '.html': 'text/html',
  '.js': 'text/javascript',
  '.mjs': 'text/javascript',
  '.css': 'text/css',
  '.json': 'application/json',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.ico': 'image/x-icon',
  '.webmanifest': 'application/manifest+json',
  '.txt': 'text/plain',
  '.xml': 'application/xml',
};

/** Minimal static server with SPA fallback, so every route boots the app. */
function serveStatic(root) {
  return new Promise((resolveServer) => {
    const server = createServer(async (req, res) => {
      try {
        const urlPath = decodeURIComponent(new URL(req.url, 'http://x').pathname);
        let file = join(root, urlPath);
        if (urlPath.endsWith('/')) file = join(file, 'index.html');
        // Anything that isn't a real file (routes, directories left by a
        // previous prerender run) falls back to the app shell.
        const st = await stat(file).catch(() => null);
        if (!st || !st.isFile()) file = join(root, 'index.html');
        const body = await readFile(file);
        res.writeHead(200, { 'Content-Type': MIME[extname(file)] ?? 'application/octet-stream' });
        res.end(body);
      } catch {
        res.writeHead(404);
        res.end('not found');
      }
    });
    server.listen(PORT, () => resolveServer(server));
  });
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function prerenderRoute(browser, base, route) {
  const page = await browser.newPage();
  const errors = [];
  page.on('pageerror', (err) => errors.push(`pageerror: ${String(err).slice(0, 160)}`));
  try {
    // Decorative subresources only slow the render and can flake offline builds.
    await page.setRequestInterception(true);
    page.on('request', (req) => {
      const type = req.resourceType();
      if (type === 'image' || type === 'media' || type === 'font') return req.abort();
      return req.continue();
    });
    await page.goto(`${base}${route === '/' ? '' : route}`, {
      waitUntil: 'networkidle2',
      timeout: 30000,
    });
    await sleep(2000); // let lazy chunks + bindings settle
    const html = await page.content();
    if (!html.includes('<app-root')) throw new Error('app-root missing from DOM');
    if (errors.length > 0) throw new Error(errors[0]);

    const outDir = route === '/' ? DIST : join(DIST, route);
    mkdirSync(outDir, { recursive: true });
    writeFileSync(join(outDir, 'index.html'), html);
    console.log(`  ✓ ${route} (${(html.length / 1024).toFixed(1)} KB)`);
  } finally {
    await page.close();
  }
}

async function main() {
  if (!existsSync(join(DIST, 'index.html'))) {
    console.error(`dist not found at ${DIST} — run ng build first`);
    process.exit(1);
  }
  const server = await serveStatic(DIST);
  const browser = await puppeteer.launch({
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-gpu'],
  });
  const base = `http://localhost:${PORT}`;
  const failures = [];
  for (const route of ROUTES) {
    for (let attempt = 1; attempt <= 2; attempt++) {
      try {
        await prerenderRoute(browser, base, route);
        break;
      } catch (err) {
        console.error(`  ✗ ${route} (attempt ${attempt}): ${err.message}`);
        if (attempt === 2) failures.push(route);
        await sleep(1000);
      }
    }
  }
  await browser.close();
  server.close();
  console.log(`\nprerendered ${ROUTES.length - failures.length}/${ROUTES.length} routes`);
  if (failures.length > 0) {
    console.error('failed routes:', failures.join(', '));
    process.exit(1);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
