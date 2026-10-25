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

  it('describes all three tools, not only the quiz generator', () => {
    const text = render().textContent!;

    expect(text).toContain('the AI Shorts generator');
    expect(text).toContain('the TXT to quiz video generator');
    expect(text).toContain('a watermark tool that runs in your browser');
    expect(text).toContain('in beta and free to use');
  });

  it('covers content rights, automatic output, deletion and liability', () => {
    const page = render();
    const headings = Array.from(page.querySelectorAll('section h2')).map((h) => h.textContent!.replace(/^\s*\d+\s*/, '').trim());

    expect(headings).toEqual([
      'Acceptance of these terms',
      'The service',
      'Accounts',
      'Your content',
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
