import { ViewportScroller } from '@angular/common';
import { TestBed } from '@angular/core/testing';
import { ANCHOR_SCROLL_OFFSET, anchorOffsetInitializer } from './anchor-offset';

describe('anchor scroll offset', () => {
  it('gives the router the offset the pages already declare with scroll-mt-28', () => {
    // Tailwind's scroll-mt-28 is 7rem = 112px.
    expect(ANCHOR_SCROLL_OFFSET).toBe(7 * 16);
  });

  it('clears the floating header, which ends 72px from the top', () => {
    expect(ANCHOR_SCROLL_OFFSET).toBeGreaterThan(72);
  });

  it('sets it on the viewport scroller, horizontally none and vertically the offset, once started', () => {
    const scroller = jasmine.createSpyObj<ViewportScroller>('ViewportScroller', ['setOffset']);
    TestBed.configureTestingModule({ providers: [{ provide: ViewportScroller, useValue: scroller }] });
    const start = TestBed.runInInjectionContext(() => anchorOffsetInitializer());
    expect(scroller.setOffset).not.toHaveBeenCalled(); // nothing happens until the app starts
    start();
    expect(scroller.setOffset).toHaveBeenCalledOnceWith([0, ANCHOR_SCROLL_OFFSET]);
  });
});
