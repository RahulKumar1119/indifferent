import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import {
  LucideDownload,
  LucideFocus,
  LucideFrame,
  LucideGauge,
  LucideScissors,
  provideLucideIcons,
} from '@lucide/angular';
import { PRODUCT_FACTS, durationLabel } from '../../core/config/product-facts';
import { ShortsMakerComponent } from './shorts-maker.component';

describe('ShortsMakerComponent (public landing page)', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ShortsMakerComponent],
      providers: [
        provideRouter([]),
        provideLucideIcons(LucideScissors, LucideFocus, LucideFrame, LucideGauge, LucideDownload),
      ],
    }).compileComponents();
  });

  afterEach(() => {
    document.head
      .querySelectorAll('meta[name="robots"], script[data-seo-id], link[rel="canonical"]')
      .forEach((el) => el.remove());
  });

  function render(): { fixture: ReturnType<typeof TestBed.createComponent<ShortsMakerComponent>>; page: HTMLElement } {
    const fixture = TestBed.createComponent(ShortsMakerComponent);
    fixture.detectChanges();
    return { fixture, page: fixture.nativeElement as HTMLElement };
  }

  it('states the same upload limits the backend enforces', () => {
    const { page } = render();
    const s = PRODUCT_FACTS.shorts;
    const text = page.textContent!;

    expect(text).toContain(`${s.inputFormats.slice(0, -1).join(', ')} or ${s.inputFormats.at(-1)} · ${durationLabel(s.maxDurationMinutes)} max · ${s.maxUploadGb} GB max`);
    expect(text).toContain(`${s.inputFormats.slice(0, -1).join(', ')} and ${s.inputFormats.at(-1)} up to ${durationLabel(s.maxDurationMinutes)} long and ${s.maxUploadGb} GB.`);
    expect(text).not.toMatch(/10 ?min|500 ?MB/i);
  });

  it('describes the ranking as a language model reading the transcript, not signals it never measures', () => {
    const text = render().page.textContent!;
    const s = PRODUCT_FACTS.shorts;

    expect(text).toContain(`${s.rankingBlockSeconds} second blocks`);
    expect(text).toContain('The model reads text only, so it cannot see the picture or hear tone of voice.');
    expect(text).not.toMatch(/speech density|speaker moves|engagement signals|scores? every segment|every segment is scored/i);
  });

  it('describes face framing as one centered window per clip, not tracking', () => {
    const text = render().page.textContent!;

    expect(text).toContain('The window stays in one place for the whole clip.');
    expect(text).toContain('Face-centered framing and blur-fill are applied when the clip renders.');
    expect(text).not.toMatch(/speaker tracking|subject tracking|tracks the speaker|track the speaker|pans the|follows the speaker/i);
  });

  it('does not say captions or clips can be changed after a job has finished', () => {
    const { page } = render();
    const text = page.textContent!;

    expect(text).toContain('Can I change the captions after the clips are made?');
    expect(text).toContain('No. The captions are burned into the video, and a finished job is not rendered again.');
    expect(text).toContain('a finished job keeps the look it was rendered with');
    expect(text).not.toMatch(/Saving a new style re-renders|restyle|Caption styles you can pick/i);
  });

  it('shows the four caption presets that exist in the product, and names Bold as the default', () => {
    const { page, fixture } = render();
    const labels = Array.from(page.querySelectorAll('figure figcaption span:first-child')).map((el) => el.textContent?.trim());

    expect(labels.length).toBe(PRODUCT_FACTS.shorts.captionPresets.length);
    expect(fixture.componentInstance.captionPreviews.map((p) => p.id)).toEqual(['minimal', 'bold', 'karaoke', 'clean']);
    expect(page.textContent).toContain('Bold is the default');
  });

  it('has a self-contained upload screenshot with alt text and dimensions', () => {
    const { page } = render();
    const img = page.querySelector('img[src*="/screenshots/"]') as HTMLImageElement;

    expect(img).not.toBeNull();
    expect(img.getAttribute('alt')!.length).toBeGreaterThan(30);
    expect(img.getAttribute('width')).toBeTruthy();
    expect(img.getAttribute('height')).toBeTruthy();
    expect(page.innerHTML).not.toContain('picsum.photos');
  });

  it('answers 13 questions in visible FAQ items that match the structured data', () => {
    const { page } = render();
    const items = Array.from(page.querySelectorAll('details.faq'));
    const faq = JSON.parse(document.head.querySelector('script[data-seo-id="faqpage"]')!.textContent!);

    expect(items.length).toBe(13);
    expect(faq['@type']).toBe('FAQPage');
    expect(faq.mainEntity.length).toBe(items.length);
    for (const item of items) expect(item.querySelector('p')?.textContent?.trim().length).toBeGreaterThan(30);
  });

  it('tells the truth about storage and deletion', () => {
    const text = render().page.textContent!;

    expect(text).toContain('uploaded sources move to archive storage after 30 days');
    expect(text).toContain('Deleting a Short removes its files and its record');
    expect(text).not.toMatch(/not kept as a copy|deleted automatically|we never store/i);
  });

  it('keeps one SoftwareApplication and one FAQPage block however often the page is opened', () => {
    render();
    render();

    expect(document.head.querySelectorAll('script[data-seo-id="softwareapplication"]').length).toBe(1);
    expect(document.head.querySelectorAll('script[data-seo-id="faqpage"]').length).toBe(1);
    const app = JSON.parse(document.head.querySelector('script[data-seo-id="softwareapplication"]')!.textContent!);
    expect(app.offers.description).toBe('Free during beta');
    expect(JSON.stringify(app)).not.toMatch(/aggregateRating|ratingValue/);
  });

  it('sets its own title, description and canonical URL', () => {
    render();

    expect(document.title).toBe('AI Shorts Maker: Long Video to Ranked Clips | Indifferent');
    expect(document.head.querySelector('meta[name="description"]')?.getAttribute('content')).toContain('Free during beta');
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('https://indifferent.fun/shorts-maker');
  });

  it('links to the legal pages, the feature guide and every persona page', () => {
    const { page } = render();

    for (const href of ['/privacy', '/terms', '/contact', '/about', '/blog', '/shorts', '/login']) {
      expect(page.querySelector(`a[href="${href}"]`)).withContext(href).not.toBeNull();
    }
    expect(page.querySelectorAll('a[href^="/use-cases/"]').length).toBeGreaterThanOrEqual(6);
  });

  it('uses no em dashes in the copy', () => {
    expect(render().page.textContent).not.toContain('\u2014');
  });
});
