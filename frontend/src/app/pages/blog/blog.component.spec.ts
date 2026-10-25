import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { BlogComponent } from './blog.component';
import { ARTICLES, ARTICLE_CATEGORIES } from './articles/registry';

describe('BlogComponent (journal index)', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [BlogComponent],
      providers: [provideRouter([])],
    }).compileComponents();
  });

  afterEach(() => {
    document.head.querySelectorAll('meta[name="robots"], link[rel="canonical"]').forEach((el) => el.remove());
  });

  function render(): { fixture: ReturnType<typeof TestBed.createComponent<BlogComponent>>; page: HTMLElement } {
    const fixture = TestBed.createComponent(BlogComponent);
    fixture.detectChanges();
    return { fixture, page: fixture.nativeElement as HTMLElement };
  }

  /** Hrefs of the article cards only; the format-guide strip also links to an article. */
  const articleLinks = (page: HTMLElement) =>
    Array.from(page.querySelectorAll('main a[data-article]')).map((a) => a.getAttribute('href'));

  it('lists every article in the registry, newest first, each linking to its page', () => {
    const { page } = render();

    expect(articleLinks(page)).toEqual(ARTICLES.map((a) => `/blog/${a.slug}`));
  });

  it('also links to the file format guide from its own strip', () => {
    const { page } = render();

    const strip = page.querySelector('main a[href="/blog/quiz-file-format-guide"]:not([data-article])');
    expect(strip?.textContent).toContain('format guide');
  });

  it('shows each article with a local cover image, a date and a reading time', () => {
    const { page } = render();

    const images = Array.from(page.querySelectorAll('main img')) as HTMLImageElement[];
    expect(images.length).toBe(ARTICLES.length);
    for (const img of images) {
      expect(img.getAttribute('src')!.startsWith('/')).withContext(img.outerHTML).toBeTrue();
      expect(img.getAttribute('alt')!.length).withContext(img.outerHTML).toBeGreaterThan(15);
      expect(img.getAttribute('width')).toBeTruthy();
    }
    expect(page.textContent).toMatch(/Oct 25, 2026 · \d+ min read/);
    expect(page.textContent).toContain('Aug 8, 2026');
    expect(page.textContent).not.toMatch(/2024|January 2025|Jan \d+, 2025/);
  });

  it('has no email form, no subscribe button and no third-party placeholder images', () => {
    const { page } = render();

    expect(page.querySelector('input[type="email"]')).toBeNull();
    expect(page.querySelector('form')).toBeNull();
    expect(page.textContent).not.toMatch(/subscribe|newsletter signup|coming soon/i);
    expect(page.innerHTML).not.toContain('picsum.photos');
  });

  it('offers the RSS feed and a way to suggest a topic instead of a newsletter', () => {
    const { page } = render();

    const feed = page.querySelector('a[href="/feed.xml"]');
    expect(feed?.textContent).toContain('RSS');
    expect(page.textContent).toContain('There is no email list');
    expect(page.querySelector('a[href="/contact"]')).not.toBeNull();
  });

  it('filters by category and clears the filter', () => {
    const { fixture, page } = render();
    const component = fixture.componentInstance;

    component.setCategory('Watermark');
    fixture.detectChanges();
    const watermark = ARTICLES.filter((a) => a.category === 'Watermark').map((a) => `/blog/${a.slug}`);
    expect(watermark.length).toBeGreaterThan(0);
    expect(articleLinks(page)).toEqual(watermark);

    component.resetFilters();
    fixture.detectChanges();
    expect(articleLinks(page).length).toBe(ARTICLES.length);
  });

  it('offers a category button for every category that has an article', () => {
    const { fixture } = render();

    expect(fixture.componentInstance.categories).toEqual(['All', ...ARTICLE_CATEGORIES]);
    for (const category of ARTICLE_CATEGORIES) {
      expect(ARTICLES.some((a) => a.category === category)).withContext(category).toBeTrue();
    }
  });

  it('searches titles and descriptions and explains an empty result', () => {
    const { fixture, page } = render();
    const input = page.querySelector('#journal-search') as HTMLInputElement;

    input.value = 'webm';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(page.textContent).toContain('MP4 vs WebM');

    input.value = 'zzzz no such article';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(page.textContent).toContain('No essays found.');
  });

  it('sets a descriptive title, description and canonical URL', () => {
    render();

    expect(document.title).toContain('Journal');
    expect(document.head.querySelector('meta[name="description"]')?.getAttribute('content')).toContain('AI Shorts');
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('https://indifferent.fun/blog');
  });
});
