import { Type } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { AiNarrationGuideComponent } from './ai-narration-guide.component';
import { EducationalVideoBestPracticesComponent } from './educational-video-best-practices.component';
import { QuizFileFormatGuideComponent } from './quiz-file-format-guide.component';
import { QuizVideosYoutubeComponent } from './quiz-videos-youtube.component';
import { articleByline, articleUrl, findArticle } from './registry';

const LEGACY: { slug: string; component: Type<unknown> }[] = [
  { slug: 'quiz-videos-youtube', component: QuizVideosYoutubeComponent },
  { slug: 'educational-video-best-practices', component: EducationalVideoBestPracticesComponent },
  { slug: 'quiz-file-format-guide', component: QuizFileFormatGuideComponent },
  { slug: 'ai-narration-guide', component: AiNarrationGuideComponent },
];

describe('older journal articles', () => {
  beforeEach(() => {
    TestBed.configureTestingModule({ providers: [provideRouter([])] });
  });

  afterEach(() => {
    document.head
      .querySelectorAll('meta[name="robots"], script[data-seo-id], link[rel="canonical"]')
      .forEach((el) => el.remove());
  });

  function render(component: Type<unknown>): HTMLElement {
    const fixture = TestBed.createComponent(component);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  for (const { slug, component } of LEGACY) {
    describe(slug, () => {
      const meta = findArticle(slug)!;

      it('is in the registry as a legacy article', () => {
        expect(meta).toBeDefined();
        expect(meta.legacy).toBeTrue();
      });

      it('shows the real publication date from the registry, not an invented one', () => {
        const page = render(component);

        expect(page.textContent).toContain(articleByline(meta));
        expect(page.textContent).not.toMatch(/January \d+, 2025|December \d+, 2024|© 2025/);
      });

      it('writes the same title, canonical URL and structured data as every other article', () => {
        render(component);

        expect(document.title).toBe(`${meta.title} | Indifferent`);
        expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(articleUrl(slug));
        const ld = JSON.parse(document.head.querySelector('script[data-seo-id="article"]')!.textContent!);
        expect(ld.datePublished).toBe(meta.published);
        expect(ld.dateModified).toBe(meta.updated);
      });

      it('states a reading time that matches the length of the article', () => {
        const body = render(component).querySelector('article, main')!;
        const words = (body.textContent ?? '').split(/\s+/).filter((w) => /[A-Za-z0-9]/.test(w)).length;

        expect(words).withContext('words in the article body').toBeGreaterThan(800);
        expect(Math.abs(meta.readMinutes - words / 200)).toBeLessThanOrEqual(1);
      });

      it('ends with the shared footer so the legal pages are one click away', () => {
        const hrefs = Array.from(render(component).querySelectorAll('app-site-footer a')).map((a) => a.getAttribute('href'));

        for (const path of ['/privacy', '/terms', '/contact', '/about']) expect(hrefs).withContext(path).toContain(path);
      });
    });
  }

  it('the quiz format guide calls the 100-question figure a guideline, because the parser has no cap', () => {
    const text = render(QuizFileFormatGuideComponent).textContent!;

    expect(text).not.toContain('Maximum: 100 questions');
    expect(text).toContain('There is no hard cap on the number of questions');
    expect(text).toContain('The 5 MB file size is the only limit it enforces');
  });

  it('the narration guide says Indifferent uses the standard Polly engine, not neural voices', () => {
    const text = render(AiNarrationGuideComponent).textContent!;

    expect(text).toContain('standard engine, which Indifferent uses for all five of its voices');
    expect(text).toContain('concatenative synthesis');
    expect(text).not.toContain('use neural network models trained on thousands of hours');
    expect(text).not.toContain('regenerating the narration takes seconds');
    expect(text).toContain('Indifferent does not offer any of these today');
  });
});
