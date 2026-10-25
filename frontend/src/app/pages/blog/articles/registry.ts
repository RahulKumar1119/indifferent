import articlesJson from './articles.json';

export const SITE_URL = 'https://indifferent.fun';

export type ArticleProduct = 'ai-shorts' | 'watermark' | 'txt-to-video-quiz';
export type ArticleCategory = 'Shorts' | 'Watermark' | 'Video basics' | 'Quiz videos';

export interface ArticleMeta {
  slug: string;
  title: string;
  description: string;
  category: ArticleCategory;
  /** ISO date (YYYY-MM-DD) the article first went live. */
  published: string;
  /** ISO date of the last change to the article's content. */
  updated: string;
  readMinutes: number;
  /** Site-relative path of a local image or diagram. */
  cover: string;
  coverAlt: string;
  product: ArticleProduct;
  /** true for the older articles that keep their own component and route. */
  legacy: boolean;
}

export const ARTICLE_CATEGORIES: readonly ArticleCategory[] = ['Shorts', 'Quiz videos', 'Watermark', 'Video basics'];

/** Newest first. The sort is stable, so the file order breaks ties. */
export const ARTICLES: readonly ArticleMeta[] = [...(articlesJson as unknown as ArticleMeta[])].sort((a, b) =>
  a.published < b.published ? 1 : a.published > b.published ? -1 : 0,
);

export function findArticle(slug: string): ArticleMeta | undefined {
  return ARTICLES.find((a) => a.slug === slug);
}

export function articlePath(slug: string): string {
  return `/blog/${slug}`;
}

export function articleUrl(slug: string): string {
  return `${SITE_URL}${articlePath(slug)}`;
}

/** "October 25, 2026" from an ISO date, independent of the visitor's timezone. */
export function formatLongDate(iso: string): string {
  return new Intl.DateTimeFormat('en-US', { year: 'numeric', month: 'long', day: 'numeric', timeZone: 'UTC' }).format(
    new Date(`${iso}T00:00:00Z`),
  );
}

/** "Oct 25, 2026" */
export function formatShortDate(iso: string): string {
  return new Intl.DateTimeFormat('en-US', { year: 'numeric', month: 'short', day: 'numeric', timeZone: 'UTC' }).format(
    new Date(`${iso}T00:00:00Z`),
  );
}

/** "Published October 25, 2026 · Updated October 26, 2026 · 7 min read".
 * The updated part only appears when the article changed after publication. */
export function articleByline(meta: ArticleMeta): string {
  const parts = [`Published ${formatLongDate(meta.published)}`];
  if (meta.updated > meta.published) parts.push(`Updated ${formatLongDate(meta.updated)}`);
  parts.push(`${meta.readMinutes} min read`);
  return parts.join(' · ');
}

/** Articles to suggest after `slug`: same product first, then same category. */
export function relatedArticles(slug: string, limit = 3): ArticleMeta[] {
  const self = findArticle(slug);
  if (!self) return [];
  const others = ARTICLES.filter((a) => a.slug !== slug);
  const score = (a: ArticleMeta) => (a.product === self.product ? 2 : 0) + (a.category === self.category ? 1 : 0);
  return others
    .map((a, i) => ({ a, i, s: score(a) }))
    .sort((x, y) => y.s - x.s || x.i - y.i)
    .slice(0, limit)
    .map((x) => x.a);
}

/** Product landing page that each article's call to action points to. */
export const PRODUCT_PAGES: Record<ArticleProduct, { label: string; path: string; cta: string; button: string; tryPath: string }> = {
  'ai-shorts': {
    label: 'AI Shorts generator',
    path: '/features/ai-shorts',
    cta: 'Turn a long video into ranked clips',
    button: 'Open the Shorts generator',
    tryPath: '/shorts',
  },
  watermark: {
    label: 'Free watermark tool',
    path: '/features/watermark',
    cta: 'Watermark an image or video in your browser',
    button: 'Open the watermark tool',
    tryPath: '/tools/add-watermark',
  },
  'txt-to-video-quiz': {
    label: 'TXT to quiz video',
    path: '/features/txt-to-video-quiz',
    cta: 'Turn a text file into a narrated quiz video',
    button: 'Create a quiz video',
    tryPath: '/projects/new',
  },
};
