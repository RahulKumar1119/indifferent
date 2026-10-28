import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { RevealDirective } from './reveal.directive';

@Component({
  standalone: true,
  imports: [RevealDirective],
  template: `<div class="reveal" appReveal data-testid="target">content</div>`,
})
class HostComponent {}

describe('RevealDirective', () => {
  let originalIO: typeof IntersectionObserver;
  let observed: Element[];
  let unobserved: Element[];
  let triggers: ((entries: Partial<IntersectionObserverEntry>[]) => void)[];

  beforeEach(() => {
    observed = [];
    unobserved = [];
    triggers = [];
    originalIO = window.IntersectionObserver;
    // Fake observer that records targets and lets the test fire intersection.
    class FakeIO {
      constructor(private cb: (entries: Partial<IntersectionObserverEntry>[]) => void) {
        triggers.push(cb);
      }
      observe(el: Element) { observed.push(el); }
      unobserve(el: Element) { unobserved.push(el); }
      disconnect() {}
    }
    (window as unknown as { IntersectionObserver: unknown }).IntersectionObserver = FakeIO as unknown;
    TestBed.configureTestingModule({ imports: [HostComponent] });
  });

  afterEach(() => {
    (window as unknown as { IntersectionObserver: unknown }).IntersectionObserver = originalIO;
  });

  function target(fixture: ReturnType<typeof TestBed.createComponent<HostComponent>>): HTMLElement {
    return (fixture.nativeElement as HTMLElement).querySelector('[data-testid="target"]')!;
  }

  it('observes the host and does not reveal until it intersects', () => {
    const fixture = TestBed.createComponent(HostComponent);
    fixture.detectChanges();
    expect(observed.length).toBe(1);
    expect(target(fixture).classList.contains('is-visible')).toBeFalse();
  });

  it('adds is-visible and stops observing once the element intersects', () => {
    const fixture = TestBed.createComponent(HostComponent);
    fixture.detectChanges();
    const el = target(fixture);

    triggers[0]([{ isIntersecting: true, target: el } as Partial<IntersectionObserverEntry>]);
    expect(el.classList.contains('is-visible')).toBeTrue();
    expect(unobserved).toContain(el);
  });

  it('ignores a non-intersecting entry', () => {
    const fixture = TestBed.createComponent(HostComponent);
    fixture.detectChanges();
    const el = target(fixture);

    triggers[0]([{ isIntersecting: false, target: el } as Partial<IntersectionObserverEntry>]);
    expect(el.classList.contains('is-visible')).toBeFalse();
    expect(unobserved).not.toContain(el);
  });

  it('reveals immediately when IntersectionObserver is unavailable', () => {
    (window as unknown as { IntersectionObserver: unknown }).IntersectionObserver = undefined;
    const fixture = TestBed.createComponent(HostComponent);
    fixture.detectChanges();

    expect(target(fixture).classList.contains('is-visible')).toBeTrue();
    expect(observed.length).toBe(0);
  });
});
