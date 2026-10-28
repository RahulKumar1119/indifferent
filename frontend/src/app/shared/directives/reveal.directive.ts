import {
  AfterViewInit,
  Directive,
  ElementRef,
  OnDestroy,
  PLATFORM_ID,
  inject,
} from '@angular/core';
import { isPlatformBrowser } from '@angular/common';

/**
 * Scroll reveal for an element carrying the `.reveal` class (ui-animation skill,
 * issue #182). Adds `is-visible` once the element scrolls into view, then stops
 * observing: a reveal runs once and never re-animates on scroll up.
 *
 * Replaces the IntersectionObserver blocks hand-rolled in landing, shorts-maker
 * and blog, each with its own SSR guard. One place now owns:
 *   - the server/prerender case (no DOM): content must ship visible, so add
 *     `is-visible` is not possible there; the server renders the un-revealed
 *     state and the client reveals on hydration. To avoid a flash of hidden
 *     content when JS is off or the observer is missing, those cases reveal
 *     immediately.
 *   - reduced motion: the global CSS kill switch already neutralises the
 *     transition, so the element simply appears; we still add `is-visible` so
 *     its final (opacity:1) state is applied.
 *
 * Usage: `<section class="reveal" appReveal>`. A stagger delay can still be set
 * per element with an inline `transition-delay` or a utility class.
 */
@Directive({
  selector: '[appReveal]',
  standalone: true,
})
export class RevealDirective implements AfterViewInit, OnDestroy {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly isBrowser = isPlatformBrowser(inject(PLATFORM_ID));
  private observer?: IntersectionObserver;

  ngAfterViewInit(): void {
    const el = this.host.nativeElement;
    // Server prerender has no DOM APIs: leave the class as authored; the client
    // hydrates and reveals. On the client without IntersectionObserver, reveal
    // at once so content is never stuck hidden.
    if (!this.isBrowser || typeof IntersectionObserver === 'undefined') {
      el.classList.add('is-visible');
      return;
    }
    this.observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            entry.target.classList.add('is-visible');
            this.observer?.unobserve(entry.target);
          }
        }
      },
      { threshold: 0.15, rootMargin: '0px 0px -8% 0px' },
    );
    this.observer.observe(el);
  }

  ngOnDestroy(): void {
    this.observer?.disconnect();
  }
}
