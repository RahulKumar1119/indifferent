import { routes } from '../../../app.routes';
import { ARTICLE_LOADERS } from './article-content';
import { stripInline } from './article-inline';
import { ArticleBlock } from './article-types';
import {
  ARTICLES,
  ARTICLE_CATEGORIES,
  PRODUCT_PAGES,
  articleByline,
  articleUrl,
  findArticle,
  formatLongDate,
  formatShortDate,
  relatedArticles,
} from './registry';

/** All text a reader sees in a block, markers removed. */
function blockText(block: ArticleBlock): string {
  switch (block.type) {
    case 'h2':
    case 'h3':
    case 'p':
      return stripInline(block.text);
    case 'ul':
    case 'ol':
      return block.items.map(stripInline).join(' ');
    case 'note':
      return stripInline(`${block.title ?? ''} ${block.text}`);
    case 'table':
      return [...block.head, ...block.rows.flat()].join(' ');
    case 'figure':
      return `${block.alt} ${block.caption}`;
    case 'code':
      return '';
  }
}

function wordCount(blocks: ArticleBlock[]): number {
  return blocks
    .map(blockText)
    .join(' ')
    .split(/\s+/)
    .filter(Boolean).length;
}

/** Internal link targets inside a block's inline text. */
function linksIn(blocks: ArticleBlock[]): string[] {
  const texts: string[] = [];
  for (const b of blocks) {
    if (b.type === 'p' || b.type === 'note') texts.push(b.text);
    if (b.type === 'ul' || b.type === 'ol') texts.push(...b.items);
  }
  return texts.flatMap((t) => [...t.matchAll(/\]\(([^)\s]+)\)/g)].map((m) => m[1]));
}

function routeExists(path: string): boolean {
  const clean = path.split(/[?#]/)[0].replace(/^\//, '');
  return routes.some((r) => {
    if (r.path === '**') return false;
    const pattern = new RegExp(`^${(r.path ?? '').split('/').map((s) => (s.startsWith(':') ? '[^/]+' : s)).join('/')}$`);
    return pattern.test(clean);
  });
}

describe('journal registry', () => {
  it('lists unique, well-formed articles', () => {
    expect(ARTICLES.length).toBeGreaterThanOrEqual(10);
    expect(new Set(ARTICLES.map((a) => a.slug)).size).toBe(ARTICLES.length);
    for (const a of ARTICLES) {
      expect(a.slug).withContext(a.slug).toMatch(/^[a-z0-9]+(-[a-z0-9]+)*$/);
      expect(a.title.length).withContext(`${a.slug} title`).toBeLessThanOrEqual(70);
      expect(a.description.length).withContext(`${a.slug} description`).toBeGreaterThanOrEqual(60);
      expect(a.description.length).withContext(`${a.slug} description`).toBeLessThanOrEqual(170);
      expect(ARTICLE_CATEGORIES).withContext(`${a.slug} category`).toContain(a.category);
      expect(Object.keys(PRODUCT_PAGES)).withContext(`${a.slug} product`).toContain(a.product);
      expect(a.cover.startsWith('/')).withContext(`${a.slug} cover`).toBeTrue();
      expect(a.coverAlt.length).withContext(`${a.slug} cover alt`).toBeGreaterThan(15);
    }
  });

  it('dates are real, ordered and never in the future', () => {
    const today = new Date().toISOString().slice(0, 10);
    for (const a of ARTICLES) {
      for (const d of [a.published, a.updated]) {
        expect(d).withContext(`${a.slug} ${d}`).toMatch(/^\d{4}-\d{2}-\d{2}$/);
        expect(Number.isNaN(Date.parse(d))).withContext(`${a.slug} ${d}`).toBeFalse();
        expect(d <= today).withContext(`${a.slug} ${d} is in the future`).toBeTrue();
      }
      expect(a.updated >= a.published).withContext(`${a.slug} updated before published`).toBeTrue();
    }
  });

  it('shows the newest article first', () => {
    for (let i = 1; i < ARTICLES.length; i++) {
      expect(ARTICLES[i - 1].published >= ARTICLES[i].published).toBeTrue();
    }
  });

  it('no article carries the dates it had before it was dated honestly', () => {
    for (const a of ARTICLES) {
      expect(a.published.startsWith('2024-') || a.published.startsWith('2025-')).withContext(a.slug).toBeFalse();
    }
  });

  it('formats dates in UTC so the day never shifts with the visitor timezone', () => {
    expect(formatLongDate('2026-10-25')).toBe('October 25, 2026');
    expect(formatShortDate('2026-01-05')).toBe('Jan 5, 2026');
  });

  it('writes the byline with an updated date only when the article changed', () => {
    const fresh = { ...ARTICLES[0], published: '2026-10-25', updated: '2026-10-25', readMinutes: 7 };
    const edited = { ...fresh, updated: '2026-11-02' };
    expect(articleByline(fresh)).toBe('Published October 25, 2026 · 7 min read');
    expect(articleByline(edited)).toBe('Published October 25, 2026 · Updated November 2, 2026 · 7 min read');
  });

  it('finds articles by slug and builds absolute URLs', () => {
    expect(findArticle('how-ai-video-clipping-works')?.product).toBe('ai-shorts');
    expect(findArticle('nope')).toBeUndefined();
    expect(articleUrl('mp4-vs-webm-formats')).toBe('https://indifferent.fun/blog/mp4-vs-webm-formats');
  });

  it('suggests articles about the same product first and never itself', () => {
    const related = relatedArticles('podcast-to-youtube-shorts', 3);
    expect(related.length).toBe(3);
    expect(related.map((a) => a.slug)).not.toContain('podcast-to-youtube-shorts');
    expect(related.every((a) => a.product === 'ai-shorts')).toBeTrue();
    expect(relatedArticles('missing')).toEqual([]);
  });

  it('every legacy article has an explicit route and every other article a content loader', () => {
    for (const a of ARTICLES) {
      if (a.legacy) {
        expect(routes.some((r) => r.path === `blog/${a.slug}`)).withContext(a.slug).toBeTrue();
        expect(ARTICLE_LOADERS[a.slug]).withContext(a.slug).toBeUndefined();
      } else {
        expect(ARTICLE_LOADERS[a.slug]).withContext(a.slug).toBeDefined();
      }
    }
    for (const slug of Object.keys(ARTICLE_LOADERS)) {
      expect(findArticle(slug)).withContext(`${slug} has a loader but no registry entry`).toBeDefined();
    }
  });
});

describe('journal article content', () => {
  const dataDriven = ARTICLES.filter((a) => !a.legacy);

  for (const meta of dataDriven) {
    describe(meta.slug, () => {
      let blocks: ArticleBlock[] = [];

      beforeAll(async () => {
        blocks = (await ARTICLE_LOADERS[meta.slug]()).blocks;
      });

      it('is long enough to be worth reading', () => {
        expect(wordCount(blocks)).toBeGreaterThanOrEqual(650);
      });

      it('has a readMinutes value close to its real length', () => {
        const minutes = wordCount(blocks) / 200;
        // 200 words a minute, rounded to the nearest minute, so a gap above 1 means the number is stale.
        expect(Math.abs(meta.readMinutes - minutes)).toBeLessThanOrEqual(1);
      });

      it('is organised under headings and has something concrete in it', () => {
        expect(blocks.filter((b) => b.type === 'h2').length).toBeGreaterThanOrEqual(4);
        expect(blocks.some((b) => b.type === 'table' || b.type === 'code' || b.type === 'figure')).toBeTrue();
        expect(blocks[0].type).toBe('h2');
      });

      it('links onward to the tool it is about and to another article', () => {
        const links = linksIn(blocks);
        const internal = links.filter((l) => l.startsWith('/'));
        expect(internal.some((l) => l.startsWith('/blog/'))).withContext('no link to another article').toBeTrue();
        expect(internal.length).withContext('too few internal links').toBeGreaterThanOrEqual(2);
      });

      it('only links to pages that exist', () => {
        for (const link of linksIn(blocks)) {
          if (link.startsWith('http')) {
            expect(link.startsWith('https://')).withContext(link).toBeTrue();
            continue;
          }
          expect(routeExists(link)).withContext(`${meta.slug} links to ${link}`).toBeTrue();
          if (link.startsWith('/blog/')) {
            expect(findArticle(link.replace('/blog/', ''))).withContext(`${meta.slug} links to ${link}`).toBeDefined();
          }
        }
      });

      it('has no placeholder text and no em dashes', () => {
        const raw = JSON.stringify(blocks);
        expect(raw).not.toMatch(/lorem ipsum|TODO|FIXME|coming soon|placeholder|picsum/i);
        expect(raw).not.toContain('\u2014');
        expect(raw).not.toContain('\u2013');
      });

      it('makes no claim the product contradicts', () => {
        const raw = JSON.stringify(blocks);
        // A finished Shorts job is read-only: styles, effects and clip edits are not rendered again.
        expect(raw).not.toMatch(/Saving re-renders|re-renders the clips|renders again with the corrected|for any finished job|restyle captions on a finished/i);
        expect(raw).not.toMatch(/Open Edit on a clip|use Edit in the gallery|in the clip editor/i);
        // The ranking model reads text only and the 9:16 window does not move within a clip.
        expect(raw).not.toMatch(/speech density|speaker moves|tracks the speaker|follows the speaker|pans the|scored for engagement/i);
      });

      it('gives every figure and table a description', () => {
        for (const b of blocks) {
          if (b.type === 'figure') {
            expect(b.alt.length).toBeGreaterThan(20);
            expect(b.caption.length).toBeGreaterThan(10);
            expect(b.src.startsWith('/')).toBeTrue();
          }
          if (b.type === 'table') {
            expect(b.caption).toBeTruthy();
            expect(b.rows.every((r) => r.length === b.head.length)).toBeTrue();
          }
        }
      });
    });
  }
});
