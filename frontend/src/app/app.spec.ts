import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { App } from './app';
import { setJsonLd, setRobots } from './shared/seo';

@Component({ selector: 'app-dummy', template: '' })
class DummyComponent {}

describe('App', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [App],
      providers: [
        provideRouter([
          { path: '', component: DummyComponent },
          { path: 'blog', component: DummyComponent },
          { path: 'shorts-maker', component: DummyComponent },
          { path: 'dashboard', component: DummyComponent },
        ]),
      ],
    }).compileComponents();
  });

  afterEach(() => {
    document.head.querySelectorAll('meta[name="robots"], script[data-seo-id]').forEach((el) => el.remove());
  });

  it('should create the app shell', () => {
    const fixture = TestBed.createComponent(App);
    expect(fixture.componentInstance).toBeTruthy();
  });

  it('renders the router outlet', () => {
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('router-outlet')).toBeTruthy();
  });

  describe('head reset on navigation', () => {
    const robots = () => document.head.querySelector('meta[name="robots"]')?.getAttribute('content');

    it('lifts a noindex left by the previous page before the next page runs', async () => {
      TestBed.createComponent(App);
      const router = TestBed.inject(Router);
      setRobots(document, 'noindex, follow');

      await router.navigateByUrl('/blog');

      expect(robots()).toBe('index, follow');
    });

    it('marks signed-in routes noindex and keeps /shorts-maker indexable', async () => {
      TestBed.createComponent(App);
      const router = TestBed.inject(Router);

      await router.navigateByUrl('/dashboard');
      expect(robots()).toBe('noindex, nofollow');

      await router.navigateByUrl('/shorts-maker');
      expect(robots()).toBe('index, follow');
    });

    it('drops structured data from the previous page', async () => {
      TestBed.createComponent(App);
      const router = TestBed.inject(Router);
      setJsonLd(document, 'article', { '@type': 'Article' });

      await router.navigateByUrl('/blog');

      expect(document.head.querySelectorAll('script[data-seo-id]').length).toBe(0);
    });
  });
});
