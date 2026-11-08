import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { provideLucideIcons, LucideCheck, LucideClapperboard, LucideFileText, LucideImage, LucideScissors, LucideStamp } from '@lucide/angular';
import { ToolsComponent } from './tools.component';

describe('ToolsComponent', () => {
  function setup(): HTMLElement {
    TestBed.configureTestingModule({
      imports: [ToolsComponent],
      providers: [provideRouter([]), provideLucideIcons(LucideCheck, LucideClapperboard, LucideFileText, LucideImage, LucideScissors, LucideStamp)],
    });
    const fixture = TestBed.createComponent(ToolsComponent);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('shows the five tools in order, each linking to its existing page', () => {
    const el = setup();
    const cards = Array.from(el.querySelectorAll('[data-testid^="tool-"]'));
    expect(cards.map((c) => c.querySelector('h2')!.textContent!.trim())).toEqual([
      'Watermark tool',
      'Quiz videos',
      'AI Shorts',
      'Story video',
      'AI thumbnails',
    ]);
    expect(cards.map((c) => c.querySelector('.actions a')!.getAttribute('href'))).toEqual([
      '/tools/add-watermark',
      '/projects/new',
      '/shorts',
      '/story',
      '/shorts/history',
    ]);
  });

  it('uses real screenshots where they exist and a labelled illustration for thumbnails', () => {
    const el = setup();
    const imgs = Array.from(el.querySelectorAll('[data-testid^="tool-"] img'));
    expect(imgs.length).toBe(4);
    for (const img of imgs) expect(img.getAttribute('alt')!.length).toBeGreaterThan(10);
    const ill = el.querySelector('[data-testid="tool-thumbnails"] svg[role="img"]');
    expect(ill?.getAttribute('aria-label')).toContain('Illustration');
  });

  it('marks only the watermark tool as free with no sign-in, and shows no usage numbers', () => {
    const el = setup();
    const free = Array.from(el.querySelectorAll('.pill.free')).map((p) => p.closest('[data-testid]')!.getAttribute('data-testid'));
    expect(free).toEqual(['tool-watermark']);
    expect(el.textContent).not.toMatch(/\b\d[\d,.]*\+?\s*(users|creators|videos made|downloads)\b/i);
  });

  it('uses the shared public header and footer, with one h1', () => {
    const el = setup();
    expect(el.querySelector('app-public-header')).toBeTruthy();
    expect(el.querySelector('app-site-footer')).toBeTruthy();
    expect(el.querySelectorAll('h1').length).toBe(1);
  });
});
