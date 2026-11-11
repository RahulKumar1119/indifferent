import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { PRODUCT_FACTS } from '../../core/config/product-facts';
import { PricingComponent, USAGE_LIMITS } from './pricing.component';

describe('PricingComponent (#255)', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [PricingComponent], providers: [provideRouter([])] }).compileComponents();
  });
  afterEach(() => {
    document.head.querySelectorAll('link[rel="canonical"]').forEach((el) => el.remove());
  });

  function render(): HTMLElement {
    const fixture = TestBed.createComponent(PricingComponent);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }
  const text = (el: Element) => el.textContent!.replace(/\s+/g, ' ').trim();

  it('is titled "Free during beta", with one h1, and says no card and no paid plan', () => {
    const page = render();
    expect(page.querySelectorAll('h1').length).toBe(1);
    expect(text(page.querySelector('h1')!)).toBe('Free during beta');
    expect(text(page)).toContain('No credit card, no paid plan');
  });

  it('names no price, currency, tier or comparison table', () => {
    const page = text(render());
    expect(page).not.toMatch(/[$€£₹]|\bUSD\b|\bINR\b|per month|\/mo\b|\bPro\b|\bPremium\b|\bEnterprise\b|\bStarter\b/);
    expect(page).not.toMatch(/free forever|always be free|free for life/i);
    expect(render().querySelectorAll('table').length).toBe(1);
  });

  it('shows exactly the limits the API enforces, with when they reset', () => {
    const rows = Array.from(render().querySelectorAll('[data-testid="limits"] tbody tr')).map((tr) =>
      Array.from(tr.children).map(text).join(' '),
    );
    expect(rows).toEqual([
      'Quiz projects 40 a day Midnight UTC',
      'Question files and logos 120 a day Midnight UTC',
      'Shorts uploads 30 a day Midnight UTC',
      'Shorts retries 15 a day Midnight UTC',
      'Thumbnail sets 30 a day Midnight UTC',
      'Story plans 20 an hour Any 60 minutes',
      'Story videos 10 a day, 2 at a time Any 24 hours',
      'Stock searches 120 an hour Hourly',
    ]);
    expect(USAGE_LIMITS.length).toBe(rows.length);
  });

  it('reads every figure from the product facts, so the page and the limits cannot drift', () => {
    const L = PRODUCT_FACTS.limits;
    const all = USAGE_LIMITS.map((l) => l.allowance).join(' ');
    for (const n of [L.quizProjectsPerDay, L.fileUploadsPerDay, L.shortsUploadsPerDay, L.shortsRetriesPerDay, L.thumbnailSetsPerDay, L.storyPlansPerHour, L.stockSearchesPerHour, PRODUCT_FACTS.story.perDay, PRODUCT_FACTS.story.maxActive]) {
      expect(all).toContain(String(n));
    }
  });

  it('states the per-file limit of each tool from the product facts', () => {
    const items = Array.from(render().querySelectorAll('[data-testid="free-tools"] li p')).map(text);
    expect(items).toEqual([
      'A story of 30 to 5,000 characters becomes 2 to 10 scenes, up to 90 seconds.',
      'One MP4, MOV, MP3 or WAV file from 30 seconds to 1 hour, up to 2 GB.',
      'One TXT file of questions, up to 5 MB.',
      'Runs in your browser and needs no account.',
    ]);
  });

  it('promises email and in-app notice at least 30 days before paid plans or lower limits', () => {
    const notice = text(render().querySelector('[data-testid="notice"]')!);
    expect(notice).toContain('paid plans');
    expect(notice).toContain('materially reduce these free limits');
    expect(notice).toContain('by email');
    expect(notice).toContain('notice in the app');
    expect(notice).toContain('at least 30 days ahead');
  });

  it('says beta is not a promise to stay free, and links the beta terms', () => {
    const page = render();
    expect(text(page)).toContain('Free during beta is not a promise to stay free for good.');
    expect(page.querySelector('a[href="/terms#service"]')).not.toBeNull();
    expect(page.querySelector('a[href="/terms#limits"]')).not.toBeNull();
  });

  it('lists the four free tools with links to their pages', () => {
    const links = Array.from(render().querySelectorAll('[data-testid="free-tools"] a')).map((a) => a.getAttribute('href'));
    expect(links).toEqual(['/features/story-video', '/features/ai-shorts', '/features/txt-to-video-quiz', '/features/watermark']);
  });

  it('sets a title, description and canonical URL, and uses no em dashes', () => {
    const page = render();
    expect(document.title).toBe('Pricing: free during beta | Indifferent');
    expect(document.head.querySelector('link[rel="canonical"]')!.getAttribute('href')).toBe('https://indifferent.fun/pricing');
    expect(page.textContent).not.toContain('\u2014');
  });
});
