import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { ArticlePageComponent } from './article-page.component';
import { ARTICLE_LOADERS } from './article-content';
import { ArticleBlock } from './article-types';
import { PRODUCT_PAGES, articleByline, articleUrl, findArticle, relatedArticles } from './registry';

/** One router harness per test; later calls navigate inside it, like a reader clicking a link. */
let harness: RouterTestingHarness | undefined;

/** The body arrives through a dynamic import; poll until the page says it has finished loading. */
async function settle(h: RouterTestingHarness): Promise<void> {
  const page = h.routeDebugElement!.componentInstance as ArticlePageComponent;
  for (let i = 0; i < 100; i++) {
    h.detectChanges();
    if (!page.loading) break;
    await new Promise((resolve) => setTimeout(resolve, 20));
  }
  h.detectChanges();
}

async function openArticle(slug: string): Promise<HTMLElement> {
  harness ??= await RouterTestingHarness.create();
  await harness.navigateByUrl(`/blog/${slug}`, ArticlePageComponent);
  await settle(harness);
  return harness.routeNativeElement as HTMLElement;
}

describe('ArticlePageComponent', () => {
  beforeEach(() => {
    harness = undefined;
    TestBed.configureTestingModule({
      providers: [provideRouter([{ path: 'blog/:slug', component: ArticlePageComponent }])],
    });
  });

  afterEach(() => {
    document.head
      .querySelectorAll('meta[name="robots"], script[data-seo-id], link[rel="canonical"]')
      .forEach((el) => el.remove());
  });

  it('renders the title, byline, cover and body of a data-driven article', async () => {
    const meta = findArticle('mp4-vs-webm-formats')!;
    const page = await openArticle(meta.slug);

    expect(page.querySelector('h1')?.textContent?.trim()).toBe(meta.title);
    expect(page.textContent).toContain(articleByline(meta));
    const cover = page.querySelector('figure img') as HTMLImageElement;
    expect(cover.getAttribute('src')).toBe(meta.cover);
    expect(cover.getAttribute('alt')).toBe(meta.coverAlt);

    const { blocks } = await ARTICLE_LOADERS[meta.slug]();
    const h2s = blocks.filter((b): b is Extract<ArticleBlock, { type: 'h2' }> => b.type === 'h2');
    const rendered = Array.from(page.querySelectorAll('article h2')).map((h) => h.textContent?.trim());
    expect(rendered).toEqual(h2s.map((h) => h.text));
  });

  it('writes the canonical URL, social tags and Article structured data', async () => {
    const meta = findArticle('landscape-to-vertical-video')!;
    await openArticle(meta.slug);

    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(articleUrl(meta.slug));
    expect(document.title).toBe(`${meta.title} | Indifferent`);
    expect(document.head.querySelector('meta[property="og:type"]')?.getAttribute('content')).toBe('article');
    expect(document.head.querySelector('meta[property="og:image"]')?.getAttribute('content')).toMatch(/^https:\/\/indifferent\.fun\/.+\.(png|jpg|webp|jpeg)$/);

    const ld = JSON.parse(document.head.querySelector('script[data-seo-id="article"]')!.textContent!);
    expect(ld['@type']).toBe('Article');
    expect(ld.headline).toBe(meta.title);
    expect(ld.datePublished).toBe(meta.published);
    expect(ld.dateModified).toBe(meta.updated);
    expect(ld.author.name).toBe('Indifferent');
    const crumbs = JSON.parse(document.head.querySelector('script[data-seo-id="breadcrumbs"]')!.textContent!);
    expect(crumbs.itemListElement.map((i: { name: string }) => i.name)).toEqual(['Home', 'Journal', meta.title]);
  });

  it('links the table of contents to real headings', async () => {
    const page = await openArticle('how-ai-video-clipping-works');

    const toc = Array.from(page.querySelectorAll('nav[aria-label="On this page"] a')) as HTMLAnchorElement[];
    expect(toc.length).toBeGreaterThanOrEqual(5);
    for (const link of toc) {
      const id = link.getAttribute('href')!.slice(1);
      expect(page.querySelector(`#${id}`)).withContext(id).not.toBeNull();
    }
  });

  it('gives images fixed dimensions and tables an accessible region', async () => {
    const page = await openArticle('animated-captions-for-shorts');

    for (const img of Array.from(page.querySelectorAll('img'))) {
      expect(img.getAttribute('alt')).withContext(img.outerHTML).toBeTruthy();
      expect(img.getAttribute('width')).withContext(img.outerHTML).toBeTruthy();
      expect(img.getAttribute('height')).withContext(img.outerHTML).toBeTruthy();
    }
    const region = page.querySelector('.table-wrap') as HTMLElement;
    expect(region.getAttribute('role')).toBe('region');
    expect(region.getAttribute('aria-label')).toBeTruthy();
    expect(region.tabIndex).toBe(0);
  });

  it('opens external links safely and keeps internal links inside the app', async () => {
    const page = await openArticle('mp4-vs-webm-formats');

    const external = Array.from(page.querySelectorAll('article a[href^="http"]')) as HTMLAnchorElement[];
    expect(external.length).toBeGreaterThanOrEqual(2);
    for (const a of external) {
      expect(a.getAttribute('target')).toBe('_blank');
      expect(a.getAttribute('rel')?.split(/\s+/)).toContain('noopener');
    }
    const internal = Array.from(page.querySelectorAll('article a[href^="/"]'));
    expect(internal.length).toBeGreaterThanOrEqual(1);
  });

  it('ends with a call to action for the right tool and three related articles', async () => {
    const meta = findArticle('add-watermark-to-image-in-browser')!;
    const page = await openArticle(meta.slug);

    const product = PRODUCT_PAGES[meta.product];
    const cta = page.querySelector('a[data-cta="try"]');
    expect(cta?.getAttribute('href')).toBe(product.tryPath);
    expect(cta?.textContent?.trim()).toBe(product.button);
    expect(page.textContent).toContain(product.cta);
    expect(page.querySelector(`a[href="${product.path}"]`)).not.toBeNull();
    const related = Array.from(page.querySelectorAll('section[aria-labelledby="related-heading"] a'));
    expect(related.length).toBe(3);
    expect(related.map((a) => a.getAttribute('href'))).toEqual(
      relatedArticles(meta.slug, 3).map((a) => `/blog/${a.slug}`),
    );
  });

  it('shows the next article when a reader follows a "Keep reading" link', async () => {
    const first = findArticle('mp4-vs-webm-formats')!;
    const page = await openArticle(first.slug);
    expect(page.querySelector('h1')?.textContent?.trim()).toBe(first.title);

    const next = relatedArticles(first.slug, 3).find((a) => !a.legacy)!;
    const link = page.querySelector(`section[aria-labelledby="related-heading"] a[href="/blog/${next.slug}"]`) as HTMLAnchorElement;
    expect(link).withContext('related link to a data-driven article').not.toBeNull();
    link.click();
    await harness!.fixture.whenStable();
    await settle(harness!);

    const after = harness!.routeNativeElement as HTMLElement;
    const { blocks } = await ARTICLE_LOADERS[next.slug]();
    const h2s = blocks.filter((b): b is Extract<ArticleBlock, { type: 'h2' }> => b.type === 'h2').map((b) => b.text);
    expect(after.querySelector('h1')?.textContent?.trim()).toBe(next.title);
    expect(Array.from(after.querySelectorAll('article h2')).map((h) => h.textContent?.trim())).toEqual(h2s);
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(articleUrl(next.slug));
    expect(document.title).toBe(`${next.title} | Indifferent`);
    expect(JSON.parse(document.head.querySelector('script[data-seo-id="article"]')!.textContent!).headline).toBe(next.title);
  });

  it('shows the site footer with the legal pages', async () => {
    const page = await openArticle('podcast-to-youtube-shorts');
    const hrefs = Array.from(page.querySelectorAll('app-site-footer a')).map((a) => a.getAttribute('href'));
    for (const path of ['/privacy', '/terms', '/contact', '/about', '/help', '/blog']) {
      expect(hrefs).withContext(path).toContain(path);
    }
  });

  it('answers an unknown slug with a not-found message and a noindex tag', async () => {
    const harness = await RouterTestingHarness.create();
    await harness.navigateByUrl('/blog/this-article-does-not-exist', ArticlePageComponent);
    harness.detectChanges();
    const page = harness.routeNativeElement as HTMLElement;

    expect(page.querySelector('h1')?.textContent).toContain('does not exist');
    expect(page.querySelector('a[href="/blog"]')).not.toBeNull();
    expect(document.head.querySelector('meta[name="robots"]')?.getAttribute('content')).toBe('noindex, follow');
    expect(document.head.querySelector('script[data-seo-id="article"]')).toBeNull();
  });

  it('does not treat a legacy article slug as data-driven', async () => {
    const harness = await RouterTestingHarness.create();
    await harness.navigateByUrl('/blog/quiz-file-format-guide', ArticlePageComponent);
    harness.detectChanges();

    expect((harness.routeNativeElement as HTMLElement).querySelector('h1')?.textContent).toContain('does not exist');
  });
});
