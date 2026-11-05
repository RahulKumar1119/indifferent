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

  it('lists all four tools without a not-yet-available hedge on Story videos', () => {
    const text = render().textContent!;
    expect(text).toContain('is four tools: the AI Shorts generator, the TXT to quiz video generator, the Story to video generator and a watermark tool');
    expect(text).not.toContain('being introduced');
    expect(text).not.toContain('may not be available to every account yet');
  });

  describe('Story videos', () => {
    const text = () => render().textContent!.replace(/\s+/g, ' ');

    it('says the story is sent to Bedrock, is not kept, and that the plan made from it is', () => {
      const t = text();
      expect(t).toContain('the story you write is sent to a language model on Amazon Bedrock');
      expect(t).toContain('We keep the plan and not the story, and only the story’s length is recorded');
      expect(t).toContain('The plan’s narration lines can follow your own wording closely, and they are kept');
      expect(t).toContain('The story itself is not kept');
    });

    it('says what Pexels and Pixabay receive, and what the browser loads from them directly', () => {
      const t = text();
      expect(t).toContain('our server sends the search words (the scene’s keywords, or what you type) to Pexels and Pixabay, without your name, email or account');
      expect(t).toContain('Your browser then loads the preview pictures and clips directly from those services and the networks they use');
      expect(t).toContain('they can see your IP address and browser details');
      expect(t).toContain('Pexels and Pixabay: the photo and clip libraries behind Story videos');
    });

    it('says only the narration text goes to Polly and the pictures go to no recognition service', () => {
      const t = text();
      expect(t).toContain('Only the narration text goes to Polly');
      expect(t).toContain('The pictures and clips are not sent to any image or face recognition service');
    });

    it('states retention as the storage rules implement it, and admits plans have no delete button', () => {
      const t = text();
      expect(t).toContain('the narration audio are working files, so they go after 7 days');
      expect(t).toContain('The finished video stays until you delete it, and deleting it removes the video, its working files and its record');
      expect(t).toContain('Story plans stay until you ask us to remove them. The app has no button to delete a plan yet');
      expect(t).toContain('A story plan can be removed on request');
    });

    it('says the metrics, logs and cache hold no personal data, and no more than that', () => {
      const t = text();
      expect(t).toContain('They carry no name, email, account id or story text');
      expect(t).toContain('Technical logs record account and job ids and error details, not your story text');
      expect(t).toContain('holds what Pexels and Pixabay returned, not who searched');
      expect(t).not.toMatch(/no logs|logs nothing|anonymous/i);
    });

    it('includes story videos in the emails it describes', () => {
      expect(text()).toContain('when a quiz video, a Shorts job or a story video finishes or fails');
    });

    it('uses no em dash and no emoji', () => {
      const raw = render().textContent!;
      expect(raw).not.toContain('\u2014');
      expect(raw).not.toMatch(/[\u{1F300}-\u{1FAFF}\u{2600}-\u{27BF}]/u);
    });
  });

  it('keeps the section numbers that the text refers to (section 03, section 07, section 12)', () => {
    const page = render();
    const numberOf = (id: string) => page.querySelector(`section#${id} h2 span:first-child`)!.textContent!.trim();
    expect(numberOf('tools')).toBe('03');
    expect(numberOf('cookies')).toBe('07');
    expect(numberOf('contact')).toBe('12');
    const t = page.textContent!;
    expect(t).toContain('see section 03');
    expect(t).toContain('Section 07 describes them');
    expect(t).toContain('the address in section 12');
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
      'Pexels',
      'Pixabay',
      'CloudWatch RUM',
      'Google Analytics',
      'Google Ads',
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

  it('discloses the Google Ads conversion on the sign-in click, and what it does not send', () => {
    const text = render().textContent!;

    expect(text).toContain('Google Ads conversion measurement');
    expect(text).toContain('when you click “Continue with Google”');
    expect(text).toContain('We send no name or email with it');
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
