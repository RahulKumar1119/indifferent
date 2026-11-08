import { Type } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import {
  LucideDownload,
  LucideFocus,
  LucideFrame,
  LucideGauge,
  LucideScissors,
  provideLucideIcons,
} from '@lucide/angular';
import { AboutComponent } from './about/about.component';
import { ContactComponent } from './contact/contact.component';
import { FeaturePageComponent } from './features/feature-page.component';
import { HelpComponent } from './help/help.component';
import { PrivacyComponent } from './privacy/privacy.component';
import { ShortsMakerComponent } from './shorts-maker/shorts-maker.component';
import { TermsComponent } from './terms/terms.component';
import { UseCaseComponent } from './use-cases/use-case.component';

const PAGES: [string, Type<unknown>][] = [
  ['feature page', FeaturePageComponent],
  ['use case page', UseCaseComponent],
  ['shorts maker', ShortsMakerComponent],
  ['about', AboutComponent],
  ['contact', ContactComponent],
  ['help', HelpComponent],
  ['privacy', PrivacyComponent],
  ['terms', TermsComponent],
];

describe('public information pages share one header and one footer', () => {
  afterEach(() => {
    document.head
      .querySelectorAll('meta[name="robots"], script[data-seo-id], link[rel="canonical"]')
      .forEach((el) => el.remove());
  });

  function render(type: Type<unknown>): HTMLElement {
    TestBed.configureTestingModule({
      imports: [type],
      providers: [
        provideRouter([]),
        provideLucideIcons(LucideScissors, LucideFocus, LucideFrame, LucideGauge, LucideDownload),
      ],
    });
    const fixture = TestBed.createComponent(type);
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  for (const [name, type] of PAGES) {
    describe(name, () => {
      it('uses the shared public header and the shared footer, and no page-local header', () => {
        const page = render(type);
        expect(page.querySelectorAll('app-public-header').length).toBe(1);
        expect(page.querySelectorAll('app-site-footer').length).toBe(1);
        expect(page.querySelector('app-public-header a[href="/login"]')?.textContent?.trim()).toBe('Get started');
        expect(page.querySelectorAll('footer').length).toBe(1);
      });

      it('has exactly one h1 and a named main landmark region', () => {
        const page = render(type);
        expect(page.querySelectorAll('h1').length).toBe(1);
        expect(page.querySelector('h1')!.textContent!.trim().length).toBeGreaterThan(3);
        expect(page.querySelectorAll('main').length).toBe(1);
      });

      it('has no tracked-capital labels, gradients or glued arrows', () => {
        const page = render(type);
        expect(page.innerHTML).not.toMatch(/uppercase|tracking-\[/);
        expect(page.innerHTML).not.toMatch(/gradient\(/);
        expect(page.textContent).not.toMatch(/[\u2197\u2192]/);
      });
    });
  }

  describe('primary actions', () => {
    it('feature page: one primary action in the page body, to the tool', () => {
      const page = render(FeaturePageComponent);
      const primary = page.querySelectorAll('main a.ui-btn--primary');
      expect(primary.length).toBe(1);
      expect(primary[0].getAttribute('href')).toBeTruthy();
    });

    it('use case page: one primary action, to the Shorts tool', () => {
      const page = render(UseCaseComponent);
      const primary = page.querySelectorAll('main a.ui-btn--primary');
      expect(primary.length).toBe(1);
      expect(primary[0].getAttribute('href')).toBe('/shorts');
    });

    it('contact page: one primary action, to the issue tracker', () => {
      const page = render(ContactComponent);
      const primary = page.querySelectorAll('main a.ui-btn--primary');
      expect(primary.length).toBe(1);
      expect(primary[0].getAttribute('href')).toContain('github.com/RahulKumar1119/indifferent/issues');
    });
  });

  describe('legal pages', () => {
    for (const [name, type] of [['privacy', PrivacyComponent], ['terms', TermsComponent]] as [string, Type<unknown>][]) {
      it(`${name}: contents list links to every section and every section has a heading`, () => {
        const page = render(type);
        const sections = Array.from(page.querySelectorAll('main section.doc-section'));
        const links = Array.from(page.querySelectorAll('nav[aria-label="Contents"] a')).map((a) => a.getAttribute('href'));
        expect(sections.length).toBeGreaterThan(5);
        expect(links.length).toBe(sections.length);
        for (const s of sections) {
          expect(links).toContain('#' + s.id);
          expect(s.querySelector('h2')).not.toBeNull();
        }
      });
    }
  });
});
