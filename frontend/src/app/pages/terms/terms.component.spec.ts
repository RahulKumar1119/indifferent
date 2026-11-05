import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { legalUpdatedLabel } from '../../core/config/legal';
import { TermsComponent } from './terms.component';

describe('TermsComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [TermsComponent],
      providers: [provideRouter([])],
    }).compileComponents();
  });

  afterEach(() => {
    document.head.querySelectorAll('link[rel="canonical"]').forEach((el) => el.remove());
  });

  function render(): HTMLElement {
    const fixture = TestBed.createComponent(TermsComponent);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('is dated from the legal date constant', () => {
    const page = render();

    expect(page.textContent).toContain(`Last updated: ${legalUpdatedLabel()}`);
    expect(page.textContent).not.toContain('January 2025');
  });

  it('describes all four tools, not only the quiz generator', () => {
    const text = render().textContent!;

    expect(text).toContain('Indifferent offers four tools');
    expect(text).toContain('the AI Shorts generator');
    expect(text).toContain('the TXT to quiz video generator');
    expect(text).toContain('the Story to video generator');
    expect(text).toContain('a watermark tool that runs in your browser');
    expect(text).toContain('in beta and free to use');
  });

  it('describes the Story to video generator as a live tool, without a not-yet-available hedge', () => {
    const text = render().textContent!;

    expect(text).toContain('choose a photo or clip from Pexels or Pixabay for each scene');
    expect(text).not.toContain('being introduced');
    expect(text).not.toContain('may not be available to every account yet');
  });

  it('covers content rights, automatic output, deletion and liability', () => {
    const page = render();
    const headings = Array.from(page.querySelectorAll('section h2')).map((h) => h.textContent!.replace(/^\s*\d+\s*/, '').trim());

    expect(headings).toEqual([
      'Acceptance of these terms',
      'The service',
      'Accounts',
      'Your content',
      'Pictures and clips from other libraries',
      'Acceptable use',
      'Automatic output',
      'Limits and availability',
      'Storage and deletion',
      'Ending your use',
      'Disclaimer and liability',
      'Changes to these terms',
      'Contact',
    ]);
    expect(page.textContent).toContain('You keep ownership of what you upload');
    expect(page.textContent).toContain('You are responsible for having the rights to everything you upload');
    expect(page.textContent).toContain('can be wrong');
  });

  describe('pictures and clips from other libraries', () => {
    const section = () => render().querySelector<HTMLElement>('section#stock')!;

    it('is the section the picture picker links to (#stock), numbered after Your content', () => {
      const s = section();
      expect(s).not.toBeNull();
      const spans = s.querySelectorAll('h2 span');
      expect([spans[0].textContent!.trim(), spans[1].textContent!.trim()]).toEqual(['05', 'Pictures and clips from other libraries']);
    });

    it('says whose the pictures are, and that the download is the video only', () => {
      const t = section().textContent!.replace(/\s+/g, ' ');
      expect(t).toContain('They stay with their creators and under the licences and terms of Pexels and Pixabay');
      expect(t).toContain('gives you no more rights in it than those licences give');
      expect(t).toContain('The download is the finished video only');
      expect(t).toContain('to put on other stock or wallpaper sites');
    });

    it('says what the user must not do with logos, people and property', () => {
      const t = section().textContent!.replace(/\s+/g, ' ');
      expect(t).toContain('a visible logo, brand or product to sell goods or services');
      expect(t).toContain('suggests the brand backs you');
      expect(t).toContain('person who can be recognised in a bad or offensive light');
      expect(t).toContain('as if they support something');
      expect(t).toContain('do not use a picture in a misleading way');
      expect(t).toContain('private homes and other property may need the owner’s permission');
    });

    it('says no credit is required and where the credit lines are, and what happens if a picture is removed', () => {
      const t = section().textContent!.replace(/\s+/g, ' ');
      expect(t).toContain('Neither library requires a credit in your video');
      expect(t).toContain('shown beside it');
      expect(t).toContain('If a picture you chose is gone, you will need to choose another');
    });

    it('does not promise clearance, and links to both licences safely', () => {
      const s = section();
      expect(s.textContent).toContain('We cannot promise that a picture is free of other people’s rights');
      expect(s.textContent).not.toMatch(/\bcleared\b|\bcompliant\b|\bguarantee/i);
      const links = Array.from(s.querySelectorAll<HTMLAnchorElement>('a[href^="http"]'));
      expect(links.map((a) => a.getAttribute('href'))).toEqual([
        'https://www.pexels.com/license/',
        'https://pixabay.com/service/license-summary/',
      ]);
      for (const a of links) {
        expect(a.getAttribute('target')).toBe('_blank');
        expect(a.getAttribute('rel')?.split(/\s+/)).toContain('noopener');
      }
    });

    it('is on the same footing for stories: the story is the user\'s, the pictures inside are not', () => {
      const t = render().textContent!.replace(/\s+/g, ' ');
      expect(t).toContain('The stories you write for Story videos are yours');
      expect(t).toContain('The photos and clips inside a story video are not');
    });
  });

  it('numbers its sections in order with no gaps', () => {
    const numbers = Array.from(render().querySelectorAll('section h2 span:first-child')).map((s) => s.textContent!.trim());
    expect(numbers).toEqual(numbers.map((_, i) => String(i + 1).padStart(2, '0')));
    expect(numbers.length).toBe(13);
  });

  it('says plans cannot be deleted in the app, and that a story video can', () => {
    const text = render().textContent!.replace(/\s+/g, ' ');
    expect(text).toContain('A story video can be deleted the same way');
    expect(text).toContain('Story plans cannot be deleted from the app yet. To have one removed, email support@indifferent.fun');
  });

  it('says Story output is automatic and a picture may not match its scene', () => {
    expect(render().textContent!.replace(/\s+/g, ' ')).toContain('a picture may not show what its scene describes');
  });

  it('uses no em dash and no emoji in what it says', () => {
    const text = render().textContent!;
    expect(text).not.toContain('\u2014');
    // Real emoji only: the footer's © is a "pictographic" symbol and is fine.
    expect(text).not.toMatch(/[\u{1F300}-\u{1FAFF}\u{2600}-\u{27BF}]/u);
  });

  it('only promises deletion the product can do, and does not invent a deadline', () => {
    const text = render().textContent!;

    expect(text).not.toMatch(/within \d+ days/i);
    expect(text).not.toMatch(/you may delete your account at any time/i);
    expect(text).not.toMatch(/scheduled maintenance will be communicated/i);
    expect(text).not.toMatch(/30 days notice/i);
    expect(text).toContain('Deleting one removes its stored files and its record');
    expect(text).toContain('email support@indifferent.fun from the address on the account');
  });

  it('points to the privacy policy for retention and keeps a contact address', () => {
    const page = render();

    expect(page.textContent).toContain('The privacy policy lists what is kept');
    expect(page.querySelector('a[href="mailto:support@indifferent.fun"]')).not.toBeNull();
    expect(page.querySelector('a[href="/privacy"]')).not.toBeNull();
  });

  it('sets its own title and canonical URL', () => {
    render();

    expect(document.title).toBe('Terms of Service | Indifferent');
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('https://indifferent.fun/terms');
  });
});
