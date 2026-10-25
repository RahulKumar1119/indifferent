import { CUSTOM_ELEMENTS_SCHEMA } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { PRODUCT_FACTS, durationLabel } from '../../core/config/product-facts';
import { FlipbookComponent } from './flipbook.component';
import { HeroSceneComponent } from './hero-scene.component';
import { LandingComponent } from './landing.component';

describe('LandingComponent', () => {
  beforeEach(async () => {
    // The 3D hero and the flipbook load three.js and page-flip on demand and are pure
    // decoration. The copy, links and markup are what these tests check, so both are
    // swapped for plain custom elements.
    TestBed.configureTestingModule({ imports: [LandingComponent], providers: [provideRouter([])] });
    TestBed.overrideComponent(LandingComponent, {
      remove: { imports: [FlipbookComponent, HeroSceneComponent] },
      add: { schemas: [CUSTOM_ELEMENTS_SCHEMA] },
    });
    await TestBed.compileComponents();
  });

  afterEach(() => {
    document.head
      .querySelectorAll('meta[name="robots"], script[data-seo-id], link[rel="canonical"]')
      .forEach((el) => el.remove());
  });

  function render(): HTMLElement {
    const fixture = TestBed.createComponent(LandingComponent);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('has no newsletter form: no form, no email field, no subscribe button', () => {
    const page = render();

    expect(page.querySelector('form')).toBeNull();
    expect(page.querySelector('input')).toBeNull();
    expect(page.textContent).not.toMatch(/subscribe|newsletter|coming soon/i);
  });

  it('points readers at the journal and the RSS feed, and says there is no email list', () => {
    const page = render();

    expect(page.querySelector('a[href="/blog"]')).not.toBeNull();
    expect(page.querySelector('a[href="/feed.xml"]')?.textContent).toContain('RSS');
    expect(page.textContent).toContain('There is no email list');
  });

  it('says who runs the site and gives a real contact address', () => {
    const page = render();

    expect(page.textContent).toContain('built and maintained by one developer');
    expect(page.querySelector('a[href="mailto:support@indifferent.fun"]')).not.toBeNull();
    expect(page.querySelector('a[href="/contact"]')).not.toBeNull();
    expect(page.querySelector('a[href="/about"]')).not.toBeNull();
  });

  it('links to the legal pages and to all three tools', () => {
    const page = render();

    for (const href of ['/privacy', '/terms', '/features/txt-to-video-quiz', '/features/ai-shorts', '/features/watermark', '/shorts-maker', '/tools/add-watermark']) {
      expect(page.querySelector(`a[href="${href}"]`)).withContext(href).not.toBeNull();
    }
  });

  it('quotes the AI Shorts limits from the shared product facts', () => {
    const text = render().textContent!;
    const s = PRODUCT_FACTS.shorts;

    expect(text).toContain(`${s.inputFormats.join(', ')}, up to ${durationLabel(s.maxDurationMinutes)}`);
    expect(text).toContain(`Up to ${s.maxClips} clips per upload, ${s.clipMinSeconds} to ${s.clipMaxSeconds} seconds each`);
    expect(text).toContain(`${s.outputWidth}×${s.outputHeight} MP4`);
    expect(text).toContain(`H.264 + AAC, ${s.frameRate}fps`);
  });

  it('quotes the quiz and watermark specs from the shared product facts', () => {
    const text = render().textContent!;
    const q = PRODUCT_FACTS.quiz;
    const w = PRODUCT_FACTS.watermark;

    expect(text).toContain(`Up to ${q.maxFileMb} MB`);
    for (const voice of q.voices) expect(text).withContext(voice).toContain(voice);
    expect(text).toContain(`Text in ${w.fontChoices} typefaces`);
    expect(text).toContain([...w.imageFormats, ...w.videoFormats].join(', '));
  });

  it('does not repeat claims the product cannot back up', () => {
    const text = render().textContent!;

    expect(text).not.toMatch(/restyled after|speech density|engagement signals|pans the|tracks the speaker|follows the speaker|viral/i);
    expect(text).not.toMatch(/neural voice|unlimited|10 min|500 MB upload/i);
    expect(text).not.toContain('Maximum: 100 questions');
  });

  it('describes the caption options the product has: burned-in, optionally animated', () => {
    const text = render().textContent!;

    expect(text).toContain('Burned-in captions, with an optional animated style');
  });

  it('keeps every FAQ answer visible to crawlers and matches them in structured data', () => {
    const page = render();
    const items = Array.from(page.querySelectorAll('details.faq'));

    expect(items.length).toBe(8);
    for (const item of items) {
      expect(item.querySelector('summary')?.textContent?.trim().length).toBeGreaterThan(8);
      expect(item.querySelector('p')?.textContent?.trim().length).toBeGreaterThan(60);
    }
    const faq = JSON.parse(document.head.querySelector('script[data-seo-id="faq"]')!.textContent!);
    expect(faq['@type']).toBe('FAQPage');
    expect(faq.mainEntity.length).toBe(items.length);
    expect(faq.mainEntity[0].name).toBe(items[0].querySelector('summary')!.textContent!.trim().replace(/\s+/g, ' '));
  });

  it('answers the privacy and licensing questions without overpromising', () => {
    const text = render().textContent!;

    expect(text).toContain('Yes, the text file has to reach the server to be parsed and narrated');
    expect(text).toContain('The watermark tool is the opposite: it never uploads anything.');
    expect(text).toContain('You keep ownership of the videos you generate.');
    expect(text).not.toMatch(/deleted (automatically )?after \d+ days|30 days after|essential cookies only/i);
  });

  it('sets a unique title, description and canonical URL for the home page', () => {
    render();

    expect(document.title).toBe('Indifferent: AI Shorts, Quiz Videos and a Free Watermark Tool');
    expect(document.head.querySelector('meta[name="description"]')?.getAttribute('content')).toContain('Free during beta');
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('https://indifferent.fun/');
  });

  it('writes one structured-data block per type, with no invented ratings or reviews', () => {
    render();
    render();

    for (const id of ['webpage', 'software-quiz', 'software-shorts', 'software-watermark', 'faq']) {
      expect(document.head.querySelectorAll(`script[data-seo-id="${id}"]`).length).withContext(id).toBe(1);
    }
    const raw = Array.from(document.head.querySelectorAll('script[data-seo-id]'))
      .map((s) => s.textContent)
      .join('');
    expect(raw).not.toMatch(/aggregateRating|ratingValue|reviewCount|"review"/);
    const shorts = JSON.parse(document.head.querySelector('script[data-seo-id="software-shorts"]')!.textContent!);
    expect(shorts.description).toContain('up to 1 hour');
    expect(shorts.offers.price).toBe('0');
  });

  it('uses no em dashes in the copy', () => {
    expect(render().textContent).not.toContain('\u2014');
  });

  it('has a single h1 and no placeholder text', () => {
    const page = render();

    expect(page.querySelectorAll('h1').length).toBe(1);
    expect(page.textContent).not.toMatch(/lorem ipsum|TODO|placeholder|your text here/i);
    expect(page.innerHTML).not.toContain('picsum.photos');
  });
});
