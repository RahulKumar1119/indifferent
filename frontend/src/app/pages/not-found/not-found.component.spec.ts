import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { LucideHome, provideLucideIcons } from '@lucide/angular';
import { NotFoundComponent } from './not-found.component';

describe('NotFoundComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [NotFoundComponent],
      providers: [provideRouter([]), provideLucideIcons(LucideHome)],
    }).compileComponents();
  });

  afterEach(() => {
    document.head.querySelectorAll('meta[name="robots"], link[rel="canonical"]').forEach((el) => el.remove());
  });

  it('tells search engines not to index a missing page, because the host answers it with a 200', () => {
    const fixture = TestBed.createComponent(NotFoundComponent);
    fixture.detectChanges();

    expect(document.head.querySelector('meta[name="robots"]')?.getAttribute('content')).toBe('noindex, follow');
    expect(document.title).toBe('Page not found | Indifferent');
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('https://indifferent.fun/');
  });

  it('says what happened and offers a way home', () => {
    const fixture = TestBed.createComponent(NotFoundComponent);
    fixture.detectChanges();
    const page = fixture.nativeElement as HTMLElement;

    // Sentence case, as everywhere else since #251.
    expect(page.querySelectorAll('h1').length).toBe(1);
    expect(page.querySelector('h1')!.textContent!.trim()).toBe('Page not found');
    expect(page.querySelector('a[href="/"]')).not.toBeNull();
  });

  it('offers the tool picker and help as well, on the shared buttons, with no legacy styling (#256)', () => {
    const fixture = TestBed.createComponent(NotFoundComponent);
    fixture.detectChanges();
    const page = fixture.nativeElement as HTMLElement;
    const actions = Array.from(page.querySelectorAll('main a.ui-btn')).map((a) => a.getAttribute('href'));
    expect(actions).toEqual(['/', '/new', '/help']);
    expect(page.innerHTML).not.toContain('glow-btn');
    expect(page.innerHTML).not.toContain('hsl(var(');
  });
});
