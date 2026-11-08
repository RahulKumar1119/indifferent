import { Type } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { AiNarrationGuideComponent } from './articles/ai-narration-guide.component';
import { ArticlePageComponent } from './articles/article-page.component';
import { EducationalVideoBestPracticesComponent } from './articles/educational-video-best-practices.component';
import { QuizFileFormatGuideComponent } from './articles/quiz-file-format-guide.component';
import { QuizVideosYoutubeComponent } from './articles/quiz-videos-youtube.component';
import { ARTICLES, PRODUCT_PAGES, findArticle, relatedArticles } from './articles/registry';
import { BlogComponent } from './blog.component';

/**
 * SHA-256 of each older article's body text (whitespace collapsed), taken before
 * the journal redesign. The redesign moved these articles into a shared layout
 * and must not have changed a word, so any edit to the text fails here.
 */
const LEGACY_BODY_SHA256: Record<string, { sha: string; chars: number }> = {
  "quiz-videos-youtube": {
    "sha": "ee7d6091e9cca0362c2b22c119f4e1632d76a08a7999b85e1951b8bdcaa3c06e",
    "chars": 7857
  },
  "educational-video-best-practices": {
    "sha": "fec3509e40aa37631f546d020a4c7388791ea21fb9f97f917f527350bd26b010",
    "chars": 9437
  },
  "quiz-file-format-guide": {
    "sha": "4359866a3e673e0b2e7cc02c5a3384f83c1a972ee04ac610bb5d8af86b226f89",
    "chars": 8847
  },
  "ai-narration-guide": {
    "sha": "24ac4c9154d1c5a74702badd07128d978c6aa1f0387d60c155b5e784fc99d8ad",
    "chars": 9531
  }
};

const LEGACY: [string, Type<unknown>][] = [
  ['quiz-videos-youtube', QuizVideosYoutubeComponent],
  ['educational-video-best-practices', EducationalVideoBestPracticesComponent],
  ['quiz-file-format-guide', QuizFileFormatGuideComponent],
  ['ai-narration-guide', AiNarrationGuideComponent],
];

async function sha256(text: string): Promise<string> {
  const buf = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(text));
  return Array.from(new Uint8Array(buf)).map((b) => b.toString(16).padStart(2, '0')).join('');
}

describe('journal: one publication', () => {
  afterEach(() => {
    document.head
      .querySelectorAll('meta[name="robots"], script[data-seo-id], link[rel="canonical"]')
      .forEach((el) => el.remove());
  });

  describe('the four older articles', () => {
    beforeEach(() => TestBed.configureTestingModule({ providers: [provideRouter([])] }));
    function render(type: Type<unknown>): HTMLElement {
      const fixture = TestBed.createComponent(type);
      fixture.detectChanges();
      return fixture.nativeElement as HTMLElement;
    }

    for (const [slug, type] of LEGACY) {
      it(`${slug}: the body text is word for word what it was before the redesign`, async () => {
        const body = render(type).querySelector('.ui-article')!;
        const text = (body.textContent ?? '').replace(/\s+/g, ' ').trim();
        expect(text.length).toBe(LEGACY_BODY_SHA256[slug].chars);
        expect(await sha256(text)).toBe(LEGACY_BODY_SHA256[slug].sha);
      });

      it(`${slug}: uses the shared header, breadcrumb, title block, ending and footer`, () => {
        const meta = findArticle(slug)!;
        const page = render(type);
        expect(page.querySelectorAll('app-public-header').length).toBe(1);
        expect(page.querySelectorAll('app-site-footer').length).toBe(1);
        expect(page.querySelectorAll('h1').length).toBe(1);
        expect(page.querySelector('h1')!.textContent!.trim()).toBe(meta.title);
        expect(page.querySelector('nav[aria-label="Breadcrumb"] a[href="/blog"]')).not.toBeNull();
        const cta = page.querySelector('a[data-cta="try"]');
        expect(cta?.getAttribute('href')).toBe(PRODUCT_PAGES[meta.product].tryPath);
        const related = Array.from(page.querySelectorAll('section[aria-labelledby="related-heading"] a')).map((a) => a.getAttribute('href'));
        expect(related).toEqual(relatedArticles(slug, 3).map((a) => `/blog/${a.slug}`));
      });
    }
  });

  describe('the data-driven articles', () => {
    for (const meta of ARTICLES.filter((a) => !a.legacy)) {
      it(`${meta.slug}: shared header and footer, one h1, a breadcrumb and the ending`, async () => {
        TestBed.configureTestingModule({ providers: [provideRouter([{ path: 'blog/:slug', component: ArticlePageComponent }])] });
        const harness = await RouterTestingHarness.create();
        await harness.navigateByUrl(`/blog/${meta.slug}`, ArticlePageComponent);
        await harness.fixture.whenStable();
        harness.detectChanges();
        const page = harness.routeNativeElement as HTMLElement;
        expect(page.querySelectorAll('app-public-header').length).toBe(1);
        expect(page.querySelectorAll('app-site-footer').length).toBe(1);
        expect(page.querySelectorAll('h1').length).toBe(1);
        expect(page.querySelector('nav[aria-label="Breadcrumb"]')).not.toBeNull();
        expect(page.querySelector('a[data-cta="try"]')?.getAttribute('href')).toBe(PRODUCT_PAGES[meta.product].tryPath);
        expect(page.innerHTML).not.toMatch(/uppercase|tracking-\[/);
      });
    }
  });

  describe('the index', () => {
    beforeEach(() => TestBed.configureTestingModule({ imports: [BlogComponent], providers: [provideRouter([])] }));

    it('uses the shared header and footer, has one h1, and the search and filter controls are labelled', () => {
      const f = TestBed.createComponent(BlogComponent);
      f.detectChanges();
      const page = f.nativeElement as HTMLElement;
      expect(page.querySelectorAll('app-public-header').length).toBe(1);
      expect(page.querySelectorAll('app-site-footer').length).toBe(1);
      expect(page.querySelectorAll('h1').length).toBe(1);
      expect(page.querySelector('label[for="journal-search"]')).not.toBeNull();
      expect(page.querySelector('[role="group"][aria-label="Filter by category"]')).not.toBeNull();
    });

    it('marks the active category with aria-pressed and follows it when it changes', () => {
      const f = TestBed.createComponent(BlogComponent);
      f.detectChanges();
      const page = f.nativeElement as HTMLElement;
      const pressed = () => Array.from(page.querySelectorAll('button[aria-pressed="true"]')).map((b) => b.textContent!.trim());
      expect(pressed()).toEqual(['All']);
      f.componentInstance.setCategory('Watermark');
      f.detectChanges();
      expect(pressed()).toEqual(['Watermark']);
    });

    it('has a light masthead: no dark hero, no gradient, no tracked capitals', () => {
      const f = TestBed.createComponent(BlogComponent);
      f.detectChanges();
      const page = f.nativeElement as HTMLElement;
      expect(page.innerHTML).not.toMatch(/gradient\(|uppercase|tracking-\[/);
      // The shared pill header is the one dark element; the masthead and the page are light.
      const mast = page.querySelector('header.mast')!;
      expect(mast.innerHTML).not.toContain('hm-on-dark');
      expect(page.querySelector('.page')!.classList).toContain('hm-on-light');
    });
  });
});
