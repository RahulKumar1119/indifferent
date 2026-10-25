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

    expect(page.textContent).toContain('Page Not Found');
    expect(page.querySelector('a[href="/"]')).not.toBeNull();
  });
});
