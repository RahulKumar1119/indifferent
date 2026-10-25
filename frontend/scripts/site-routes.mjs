/**
 * The public route list, shared by the post-build prerender (prerender.mjs),
 * the sitemap and RSS generator, and the SEO checks (seo-files.mjs).
 *
 * Plain JavaScript with no dependencies on purpose: the Amplify build runs
 * prerender.mjs, so this file has to work on whatever Node version the build
 * image ships.
 */
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));

export const SITE_URL = 'https://indifferent.fun';
export const FRONTEND_DIR = join(here, '..');
export const ARTICLES_JSON = join(FRONTEND_DIR, 'src/app/pages/blog/articles/articles.json');

/** Journal articles, newest first (same order the app shows). */
export function loadArticles() {
  const list = JSON.parse(readFileSync(ARTICLES_JSON, 'utf8'));
  return list
    .map((a, i) => ({ a, i }))
    .sort((x, y) => (x.a.published < y.a.published ? 1 : x.a.published > y.a.published ? -1 : x.i - y.i))
    .map((x) => x.a);
}

/** Pages that belong in search results and in the sitemap. */
export const INDEXED_ROUTES = [
  '/',
  '/about',
  '/contact',
  '/help',
  '/privacy',
  '/terms',
  '/tools',
  '/tools/add-watermark',
  '/shorts-maker',
  '/features/ai-shorts',
  '/features/txt-to-video-quiz',
  '/features/watermark',
  '/blog',
];

/** Public pages that render but carry a noindex tag: the sign-in form, and the
 * six persona pages that share one template and have no material of their own. */
export const NOINDEX_ROUTES = [
  '/login',
  '/use-cases/content-creators',
  '/use-cases/marketing-teams',
  '/use-cases/agencies',
  '/use-cases/coaches',
  '/use-cases/media-companies',
  '/use-cases/educators',
];

export const articleRoutes = () => loadArticles().map((a) => `/blog/${a.slug}`);
export const sitemapRoutes = () => [...INDEXED_ROUTES, ...articleRoutes()];
export const prerenderRoutes = () => [...INDEXED_ROUTES, ...NOINDEX_ROUTES, ...articleRoutes()];
