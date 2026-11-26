import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router, RouterLink, UrlSerializer, provideRouter } from '@angular/router';
import { Location } from '@angular/common';
import { TrailingSlashUrlSerializer, canonicalForm, siteUrl, withTrailingSlash } from './site-url';
import { robotsForUrl } from './seo';

describe('site URLs (#361)', () => {
  describe('withTrailingSlash', () => {
    const cases: Record<string, string> = {
      '/about': '/about/',
      '/about/': '/about/',
      '/': '/',
      '/blog/mp4-vs-webm-formats': '/blog/mp4-vs-webm-formats/',
      '/terms#stock': '/terms/#stock',
      '/shorts/job1/clips?tab=2': '/shorts/job1/clips/?tab=2',
      '/sitemap.xml': '/sitemap.xml',
      '/og-cover.jpg': '/og-cover.jpg',
      '/feed.xml?x=1': '/feed.xml?x=1',
    };
    for (const [input, want] of Object.entries(cases)) {
      it(`${input} becomes ${want}`, () => expect(withTrailingSlash(input)).toBe(want));
    }
  });

  it('builds absolute addresses in the served form', () => {
    expect(siteUrl('/about')).toBe('https://indifferent.fun/about/');
    expect(siteUrl('about')).toBe('https://indifferent.fun/about/');
    expect(siteUrl('/')).toBe('https://indifferent.fun/');
  });

  it('puts this site URLs in the served form and leaves others alone', () => {
    expect(canonicalForm('https://indifferent.fun/help')).toBe('https://indifferent.fun/help/');
    expect(canonicalForm('https://indifferent.fun')).toBe('https://indifferent.fun/');
    expect(canonicalForm('https://indifferent.fun/og-cover.jpg')).toBe('https://indifferent.fun/og-cover.jpg');
    expect(canonicalForm('https://example.org/x')).toBe('https://example.org/x');
    expect(canonicalForm('https://indifferent.fun.evil.example/x')).toBe('https://indifferent.fun.evil.example/x');
  });

  describe('TrailingSlashUrlSerializer', () => {
    const s = new TrailingSlashUrlSerializer();
    it('reads an address with or without the slash as the same route', () => {
      for (const [a, b] of [['/about', '/about/'], ['/blog/x', '/blog/x/'], ['/shorts/j1/clips?tab=1', '/shorts/j1/clips/?tab=1'], ['/terms#stock', '/terms/#stock']]) {
        expect(s.serialize(s.parse(a))).toBe(s.serialize(s.parse(b)));
      }
      const segments = s.parse('/blog/x/').root.children['primary'].segments.map((x) => x.path);
      expect(segments).toEqual(['blog', 'x']);
    });
    it('writes every address with the slash, keeping the query and the fragment', () => {
      expect(s.serialize(s.parse('/about'))).toBe('/about/');
      expect(s.serialize(s.parse('/'))).toBe('/');
      expect(s.serialize(s.parse('/story?plan=1#top'))).toBe('/story/?plan=1#top');
      expect(s.serialize(s.parse('/a//'))).toBe('/a/');
    });
  });

  it('keeps signed-in pages noindex in either form', () => {
    for (const url of ['/new', '/new/', '/shorts', '/shorts/', '/shorts/j1/clips/', '/dashboard/', '/auth/callback/?code=x', '/profile/']) {
      expect(robotsForUrl(url)).withContext(url).toBe('noindex, nofollow');
    }
    for (const url of ['/', '/about/', '/shorts-maker/', '/blog/x/', '/features/ai-shorts/']) {
      expect(robotsForUrl(url)).withContext(url).toBe('index, follow');
    }
  });

  describe('in the router', () => {
    @Component({ standalone: true, template: 'page' })
    class Page {}
    @Component({
      standalone: true,
      imports: [RouterLink],
      template: `<a id="a" routerLink="/about">About</a><a id="b" routerLink="/blog/x" fragment="part">X</a><a id="h" routerLink="/">Home</a>`,
    })
    class Links {}

    beforeEach(() => {
      TestBed.configureTestingModule({
        providers: [
          provideRouter([
            { path: '', component: Page },
            { path: 'about', component: Page },
            { path: 'blog/:slug', component: Page },
          ]),
          { provide: UrlSerializer, useClass: TrailingSlashUrlSerializer },
        ],
      });
    });

    it('gives every routerLink the slash form', () => {
      const f = TestBed.createComponent(Links);
      f.detectChanges();
      const href = (id: string) => (f.nativeElement as HTMLElement).querySelector(`#${id}`)!.getAttribute('href');
      expect(href('a')).toBe('/about/');
      expect(href('b')).toBe('/blog/x/#part');
      expect(href('h')).toBe('/');
    });

    it('finds the page for an address that arrives with the slash, and shows the slash', async () => {
      const router = TestBed.inject(Router);
      expect(await router.navigateByUrl('/about/')).toBeTrue();
      expect(router.url).toBe('/about/');
      expect(await router.navigateByUrl('/blog/x')).toBeTrue();
      expect(router.url).toBe('/blog/x/');
      expect(TestBed.inject(Location).path()).toBe('/blog/x');
    });
  });
});
