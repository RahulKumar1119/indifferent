import { ViewportScroller } from '@angular/common';
import { inject } from '@angular/core';

/**
 * How far below the top of the window a heading lands when the router scrolls to a
 * link's #fragment, for example /terms#stock or /#process.
 *
 * The pages already give each section `scroll-mt-28` (112px) so that it clears the
 * floating header (which ends at 72px). The browser honours that when it scrolls by
 * itself, but Angular's router does its own scrolling and ignores it, so a heading
 * reached through a link was left under the header. This is the same number, given
 * to the router.
 */
export const ANCHOR_SCROLL_OFFSET = 112;

/** An APP_INITIALIZER factory: sets the offset once, when the app starts. */
export function anchorOffsetInitializer(): () => void {
  const scroller = inject(ViewportScroller);
  return () => scroller.setOffset([0, ANCHOR_SCROLL_OFFSET]);
}
