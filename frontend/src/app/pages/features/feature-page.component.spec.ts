import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { PRODUCT_FACTS, durationLabel, joinOr, shortsOutputLabel } from '../../core/config/product-facts';
import { findArticle } from '../blog/articles/registry';
import { FEATURE_DATA, FeaturePageComponent } from './feature-page.component';

/** One router harness per test; later calls navigate inside it, like a reader clicking a link. */
let harness: RouterTestingHarness | undefined;

async function open(slug: string): Promise<HTMLElement> {
  harness ??= await RouterTestingHarness.create();
  await harness.navigateByUrl(`/features/${slug}`, FeaturePageComponent);
  harness.detectChanges();
  return harness.routeNativeElement as HTMLElement;
}

describe('FeaturePageComponent', () => {
  beforeEach(() => {
    harness = undefined;
    TestBed.configureTestingModule({
      providers: [provideRouter([{ path: 'features/:slug', component: FeaturePageComponent }])],
    });
  });

  afterEach(() => {
    document.head
      .querySelectorAll('meta[name="robots"], script[data-seo-id], link[rel="canonical"]')
      .forEach((el) => el.remove());
  });

  const slugs = Object.keys(FEATURE_DATA);

  it('has a page for each of the three tools', () => {
    expect(slugs.sort()).toEqual(['ai-shorts', 'txt-to-video-quiz', 'watermark']);
  });

  for (const slug of slugs) {
    describe(slug, () => {
      it('documents the tool: steps, limits, file handling, troubleshooting and guides', async () => {
        const data = FEATURE_DATA[slug];
        const page = await open(slug);

        expect(page.querySelector('h1')?.textContent?.trim()).toBe(data.title);
        expect(Array.from(page.querySelectorAll('main h2')).map((h) => h.textContent?.trim())).toEqual([
          'Step by step',
          'Limits and formats',
          'How your files are handled',
          'Troubleshooting',
          'Guides',
        ]);
        expect(page.querySelectorAll('#steps-heading + ol > li').length).toBe(data.steps.length);
        expect(data.steps.length).toBeGreaterThanOrEqual(4);
        expect(page.querySelectorAll('dl > div').length).toBe(data.limits.length);
        expect(page.querySelectorAll('details').length).toBe(data.troubleshooting.length);
        expect(data.troubleshooting.length).toBeGreaterThanOrEqual(5);
      });

      it('links to guides that exist and to the privacy policy', async () => {
        const data = FEATURE_DATA[slug];
        const page = await open(slug);

        expect(data.guides.length).toBeGreaterThanOrEqual(2);
        for (const guide of data.guides) {
          expect(findArticle(guide)).withContext(guide).toBeDefined();
          expect(page.querySelector(`a[href="/blog/${guide}"]`)).withContext(guide).not.toBeNull();
        }
        expect(page.querySelector('#handling-heading ~ p a[href="/privacy"]')).not.toBeNull();
      });

      it('shows the date its facts were last checked', async () => {
        const page = await open(slug);

        expect(page.textContent).toMatch(/Last checked against the product on [A-Z][a-z]+ \d{1,2}, \d{4}\./);
      });

      it('has no stale figures, no unsupported tracking claims and no em dashes', async () => {
        const text = (await open(slug)).textContent!;

        expect(text).not.toMatch(/10 ?min|10-minute|500 ?MB per|up to 500 MB upload/i);
        expect(text).not.toMatch(/pans the|follows the speaker|tracking follows|speech density|viral/i);
        expect(text).not.toMatch(/restyle the captions|edit their boundaries|four caption presets/i);
        expect(text).not.toContain('\u2014');
      });
    });
  }

  it('AI Shorts page quotes the limits the backend enforces', async () => {
    const text = (await open('ai-shorts')).textContent!;
    const s = PRODUCT_FACTS.shorts;

    expect(text).toContain(durationLabel(s.maxDurationMinutes));
    expect(text).toContain(`${s.maxUploadGb} GB`);
    expect(text).toContain(joinOr(s.inputFormats));
    expect(text).toContain(shortsOutputLabel());
    expect(text).toContain(`Up to ${s.maxClips}, ${s.clipMinSeconds} to ${s.clipMaxSeconds} seconds each`);
    expect(text).toContain(s.languages.join(', '));
    expect(text).toContain('Face detection centers the 9:16 window on the speaker, with one window position per clip');
    expect(text).toContain('A finished job keeps the look it was rendered with, so set captions and effects before you press Start processing.');
  });

  it('Quiz page quotes the file limit, voices and designs', async () => {
    const text = (await open('txt-to-video-quiz')).textContent!;
    const q = PRODUCT_FACTS.quiz;

    expect(text).toContain(`up to ${q.maxFileMb} MB`);
    expect(text).toContain(q.voices.join(', '));
    expect(text).toContain(q.designs.join(', '));
    expect(text).toContain('No fixed cap');
  });

  it('Watermark page lists the same six input formats the file picker accepts, and says nothing is uploaded', async () => {
    const text = (await open('watermark')).textContent!;
    const w = PRODUCT_FACTS.watermark;

    for (const f of [...w.imageFormats, ...w.videoFormats]) expect(text).withContext(f).toContain(f);
    expect(text).toContain('Nothing is uploaded');
    expect(text).toContain('jsDelivr');
    expect(text).toContain('They draw a dashed guide on the preview');
  });

  it('keeps an unknown slug out of the index', async () => {
    await open('does-not-exist');

    expect(document.head.querySelector('meta[name="robots"]')?.getAttribute('content')).toBe('noindex, follow');
  });

  it('shows the new tool when a reader follows an "Other tools" link', async () => {
    const first = await open('watermark');
    expect(first.querySelector('h1')?.textContent?.trim()).toBe(FEATURE_DATA['watermark'].title);

    const link = first.querySelector('a[href="/features/ai-shorts"]') as HTMLAnchorElement;
    expect(link).not.toBeNull();
    link.click();
    await harness!.fixture.whenStable();
    harness!.detectChanges();

    const page = harness!.routeNativeElement as HTMLElement;
    expect(page.querySelector('h1')?.textContent?.trim()).toBe(FEATURE_DATA['ai-shorts'].title);
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('https://indifferent.fun/features/ai-shorts');
    expect(document.title).toBe(`${FEATURE_DATA['ai-shorts'].title} | Indifferent`);
    // The "Other tools" row now lists the tool the reader just left, and no longer the one on screen.
    const row = Array.from(page.querySelectorAll('p')).find((p) => p.textContent?.trim() === 'Other tools')!.nextElementSibling!;
    const hrefs = Array.from(row.querySelectorAll('a')).map((a) => a.getAttribute('href'));
    expect(hrefs).toContain('/features/watermark');
    expect(hrefs).not.toContain('/features/ai-shorts');
  });

  it('writes one canonical URL and structured data per tool page', async () => {
    await open('watermark');

    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('https://indifferent.fun/features/watermark');
    expect(document.head.querySelectorAll('script[data-seo-id="webpage"]').length).toBe(1);
    expect(document.head.querySelectorAll('script[data-seo-id="breadcrumbs"]').length).toBe(1);

    await open('ai-shorts');
    expect(document.head.querySelectorAll('script[data-seo-id="webpage"]').length).toBe(1);
    expect(JSON.parse(document.head.querySelector('script[data-seo-id="webpage"]')!.textContent!).url).toBe('https://indifferent.fun/features/ai-shorts');
  });
});
