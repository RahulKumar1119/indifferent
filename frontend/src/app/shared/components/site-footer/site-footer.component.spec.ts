import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { SiteFooterComponent } from './site-footer.component';

describe('SiteFooterComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [SiteFooterComponent],
      providers: [provideRouter([])],
    }).compileComponents();
  });

  function render(): HTMLElement {
    const fixture = TestBed.createComponent(SiteFooterComponent);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('links the three tools, the journal, help, the company pages and the legal pages', () => {
    const hrefs = Array.from(render().querySelectorAll('a')).map((a) => a.getAttribute('href'));

    for (const path of [
      '/features/ai-shorts',
      '/features/txt-to-video-quiz',
      '/features/watermark',
      '/blog',
      '/help',
      '/',
      '/about',
      '/contact',
      '/privacy',
      '/terms',
      'mailto:support@indifferent.fun',
    ]) {
      expect(hrefs).withContext(path).toContain(path);
    }
  });

  it('names its link groups and makes every link reachable and large enough to tap', () => {
    const footer = render();

    expect(Array.from(footer.querySelectorAll('nav')).map((n) => n.getAttribute('aria-label'))).toEqual(['Tools', 'Learn', 'Company']);
    for (const a of Array.from(footer.querySelectorAll('a'))) {
      expect(a.textContent!.trim().length).withContext(a.outerHTML).toBeGreaterThan(0);
      expect(a.tabIndex).not.toBe(-1);
      expect(a.className).withContext(a.outerHTML).toContain('min-h-[44px]');
    }
  });

  it('shows the current year', () => {
    expect(render().textContent).toContain(`© ${new Date().getFullYear()} Indifferent`);
  });
});
