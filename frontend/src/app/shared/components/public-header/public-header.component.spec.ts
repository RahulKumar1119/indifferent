import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { PublicHeaderComponent } from './public-header.component';

describe('PublicHeaderComponent', () => {
  function render(): HTMLElement {
    TestBed.configureTestingModule({ imports: [PublicHeaderComponent], providers: [provideRouter([])] });
    const fixture = TestBed.createComponent(PublicHeaderComponent);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('links every page the old landing header linked, in both the menus and the mobile menu', () => {
    const el = render();
    for (const href of [
      '/features/txt-to-video-quiz',
      '/features/watermark',
      '/features/infographic-maker',
      '/features/ai-shorts',
      '/features/story-video',
      '/blog/quiz-videos-youtube',
      '/blog/educational-video-best-practices',
      '/blog/ai-narration-guide',
      '/blog',
      '/blog/quiz-file-format-guide',
      '/about',
    ]) {
      expect(el.querySelectorAll(`a[href="${href}"]`).length).withContext(href).toBeGreaterThanOrEqual(1);
    }
  });

  it('has the brand link home, the sign-in action and a named navigation landmark', () => {
    const el = render();
    expect(el.querySelector('nav')?.getAttribute('aria-label')).toBe('Main');
    expect(el.querySelector('a.brand')?.getAttribute('href')).toBe('/');
    expect(el.querySelector('a[href="/login"]')?.textContent?.trim()).toBe('Get started');
  });

  it('has three desktop menus: Features, Use cases and Resources', () => {
    const labels = Array.from(render().querySelectorAll('app-nav-menu button')).map((b) => b.textContent?.trim());
    expect(labels).toEqual(['Features', 'Use cases', 'Resources']);
  });
});
