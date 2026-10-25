import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { USE_CASE_DATA, UseCaseComponent } from './use-case.component';

async function open(slug: string): Promise<HTMLElement> {
  const harness = await RouterTestingHarness.create();
  await harness.navigateByUrl(`/use-cases/${slug}`, UseCaseComponent);
  harness.detectChanges();
  return harness.routeNativeElement as HTMLElement;
}

describe('UseCaseComponent', () => {
  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideRouter([{ path: 'use-cases/:slug', component: UseCaseComponent }])],
    });
  });

  afterEach(() => {
    document.head.querySelectorAll('meta[name="robots"], link[rel="canonical"]').forEach((el) => el.remove());
  });

  const slugs = Object.keys(USE_CASE_DATA);

  it('shows the new page when a reader follows an "Other use cases" link', async () => {
    const harness = await RouterTestingHarness.create();
    await harness.navigateByUrl('/use-cases/coaches', UseCaseComponent);
    harness.detectChanges();
    const first = harness.routeNativeElement as HTMLElement;
    expect(first.querySelector('h1')?.textContent?.trim()).toBe(USE_CASE_DATA['coaches'].title);

    (first.querySelector('a[href="/use-cases/agencies"]') as HTMLAnchorElement).click();
    await harness.fixture.whenStable();
    harness.detectChanges();

    const page = harness.routeNativeElement as HTMLElement;
    expect(page.querySelector('h1')?.textContent?.trim()).toBe(USE_CASE_DATA['agencies'].title);
    expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('https://indifferent.fun/use-cases/agencies');
    expect(page.querySelector('a[href="/use-cases/agencies"]')).toBeNull();
    expect(page.querySelector('a[href="/use-cases/coaches"]')).not.toBeNull();
  });

  it('has the six persona pages', () => {
    expect(slugs.sort()).toEqual(['agencies', 'coaches', 'content-creators', 'educators', 'marketing-teams', 'media-companies']);
  });

  for (const slug of slugs) {
    describe(slug, () => {
      it('stays out of search results until it has material of its own', async () => {
        await open(slug);

        expect(document.head.querySelector('meta[name="robots"]')?.getAttribute('content')).toBe('noindex, follow');
        expect(document.head.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe(`https://indifferent.fun/use-cases/${slug}`);
      });

      it('makes only claims the product supports', async () => {
        const text = (await open(slug)).textContent!;

        expect(text).not.toMatch(/speaker[- ]track|face tracking|tracking keeps|broadcast-safe|accepted everywhere|predicted engagement|students rewatch|high-performing|authority-building|at scale|subjective/i);
        expect(text).not.toContain('\u2014');
      });

      it('lists benefits, a call to action and the shared footer', async () => {
        const page = await open(slug);

        expect(page.querySelectorAll('main ul li').length).toBe(USE_CASE_DATA[slug].benefits.length);
        expect(page.querySelector('a[href="/shorts"]')).not.toBeNull();
        expect(page.querySelector('app-site-footer a[href="/privacy"]')).not.toBeNull();
      });
    });
  }
});
