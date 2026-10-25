import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { legalUpdatedLabel } from '../../core/config/legal';
import { PrivacyComponent } from './privacy.component';

describe('PrivacyComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [PrivacyComponent],
      providers: [provideRouter([])],
    }).compileComponents();
  });

  afterEach(() => {
    document.head.querySelectorAll('link[rel="canonical"]').forEach((el) => el.remove());
  });

  function render(): HTMLElement {
    const fixture = TestBed.createComponent(PrivacyComponent);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('is dated from the legal date constant and has one heading', () => {
    const page = render();

    expect(page.querySelectorAll('h1').length).toBe(1);
    expect(page.textContent).toContain(`Last updated: ${legalUpdatedLabel()}`);
    expect(page.textContent).not.toContain('January 2025');
  });

  it('covers each topic the AdSense and privacy review asks about', () => {
    const page = render();
    const headings = Array.from(page.querySelectorAll('section h2')).map((h) => h.textContent!.replace(/^\s*\d+\s*/, '').trim());

    expect(headings).toEqual([
      'Who we are',
      'What we collect',
      'How each tool handles your files',
      'Where data is kept, and for how long',
      'Services that receive your data',
      'Email',
      'Cookies, local storage and advertising',
      'Your choices',
      'Children',
      'Where processing happens',
      'Changes to this policy',
      'Contact',
    ]);
  });

  it('names every service that touches user data', () => {
    const text = render().textContent!;

    for (const service of [
      'Amazon S3',
      'Amazon DynamoDB',
      'Amazon Polly',
      'Amazon Transcribe',
      'Amazon Bedrock',
      'Amazon Rekognition',
      'Amazon SES',
      'CloudWatch RUM',
      'Google Analytics',
      'Google AdSense',
      'Google Fonts',
      'ImprovMX',
      'jsDelivr',
      'FFmpeg',
    ]) {
      expect(text).withContext(service).toContain(service);
    }
  });

  it('states retention exactly as the storage rules implement it', () => {
    const text = render().textContent!;

    expect(text).toContain('move to archive storage after 30 days');
    expect(text).toContain('Temporary working files are deleted after 7 days');
    expect(text).toContain('Nothing else is deleted automatically');
    expect(text).toContain('Deleting a project or a Short removes its stored files and its database record');
    expect(text).toContain('Asia Pacific (Mumbai)');
  });

  it('does not promise anything the product does not do', () => {
    const text = render().textContent!;

    // No claim of cookie-free operation, instant or automatic deletion, or a mailing list.
    expect(text).not.toMatch(/essential cookies/i);
    expect(text).not.toMatch(/deleted within \d+ days/i);
    expect(text).not.toMatch(/delete (your )?(account|data) (at any time|instantly|automatically)/i);
    expect(text).toContain('the app has no delete-account button yet');
    expect(text).toContain('the site has no newsletter');
    expect(text).toContain('We do not sell your content');
    expect(text).toContain('There is no cookie banner at the moment');
    expect(text).toContain('refresh token in local storage, not in a cookie');
  });

  it('says the watermark tool keeps files in the browser, and what its one request is', () => {
    const text = render().textContent!;

    expect(text).toContain('The watermark tool collects nothing');
    expect(text).toContain('about 30 MB from the jsDelivr CDN');
    expect(text).toContain('That request does not include your file');
  });

  it('gives a contact address and opens external links safely', () => {
    const page = render();

    expect(page.querySelector('a[href="mailto:support@indifferent.fun"]')).not.toBeNull();
    const external = Array.from(page.querySelectorAll('a[href^="http"]')) as HTMLAnchorElement[];
    expect(external.length).toBeGreaterThanOrEqual(2);
    for (const a of external) {
      expect(a.getAttribute('target')).toBe('_blank');
      expect(a.getAttribute('rel')?.split(/\s+/)).toContain('noopener');
      expect(a.textContent!.trim().length).toBeGreaterThan(5);
    }
  });

  it('sets its own title, description and canonical URL', () => {
    render();

    expect(document.title).toBe('Privacy Policy | Indifferent');
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('https://indifferent.fun/privacy');
  });
});
