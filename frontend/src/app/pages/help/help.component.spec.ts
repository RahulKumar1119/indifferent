import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { FEATURE_DATA } from '../features/feature-page.component';
import { GENERAL_FAQ, HelpComponent } from './help.component';

describe('HelpComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [HelpComponent],
      providers: [provideRouter([])],
    }).compileComponents();
  });

  afterEach(() => {
    document.head.querySelectorAll('link[rel="canonical"]').forEach((el) => el.remove());
  });

  function render(): HTMLElement {
    const fixture = TestBed.createComponent(HelpComponent);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('covers accounts and all three tools, not only the quiz generator', () => {
    const page = render();

    expect(page.querySelectorAll('h1').length).toBe(1);
    expect(Array.from(page.querySelectorAll('main section h2')).map((h) => h.textContent?.trim())).toEqual([
      'Accounts, files and pricing',
      'AI Shorts Generator',
      'TXT to Quiz Video Generator',
      'Free Watermark Tool',
      'Still stuck?',
    ]);
  });

  it('answers every general question and every tool troubleshooting question from the same data as the product pages', () => {
    const page = render();
    const questions = Array.from(page.querySelectorAll('summary')).map((s) => s.textContent?.trim());

    for (const f of GENERAL_FAQ) expect(questions).withContext(f.q).toContain(f.q);
    for (const slug of ['ai-shorts', 'txt-to-video-quiz', 'watermark']) {
      for (const t of FEATURE_DATA[slug].troubleshooting) expect(questions).withContext(t.q).toContain(t.q);
    }
  });

  it('links each tool section to its full page', () => {
    const page = render();

    for (const slug of ['ai-shorts', 'txt-to-video-quiz', 'watermark']) {
      expect(page.querySelector(`a[href="/features/${slug}"]`)).withContext(slug).not.toBeNull();
    }
  });

  it('states file retention and account deletion the way the product works', () => {
    const text = render().textContent!;

    expect(text).toContain('Uploaded source files move to archive storage after 30 days, and temporary working files are removed after 7 days');
    expect(text).toContain('There is no delete button in the app yet');
    expect(text).not.toMatch(/neural/i);
  });

  it('has a jump link for every section and a contact address', () => {
    const page = render();

    const jumps = Array.from(page.querySelectorAll('nav[aria-label="Sections"] a')).map((a) => a.getAttribute('href'));
    expect(jumps).toEqual(['#general', '#ai-shorts', '#txt-to-video-quiz', '#watermark']);
    for (const href of jumps) expect(page.querySelector(href!)).withContext(href!).not.toBeNull();
    expect(page.querySelector('a[href="mailto:support@indifferent.fun"]')).not.toBeNull();
  });

  it('sets a title, description and canonical URL', () => {
    render();

    expect(document.title).toBe('Help and FAQ | Indifferent');
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('https://indifferent.fun/help');
  });
});
