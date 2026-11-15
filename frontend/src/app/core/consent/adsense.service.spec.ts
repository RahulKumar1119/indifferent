import { DOCUMENT } from '@angular/common';
import { TestBed } from '@angular/core/testing';
import { ADSENSE_SRC, AdsenseService } from './adsense.service';

function setup() {
  const scripts: { src: string }[] = [];
  const win: Record<string, unknown> = {};
  const doc = { defaultView: win, createElement: () => ({ src: '' }), head: { appendChild: (s: { src: string }) => scripts.push(s) } };
  TestBed.configureTestingModule({ providers: [{ provide: DOCUMENT, useValue: doc }] });
  return { svc: TestBed.inject(AdsenseService), win, scripts };
}

describe('AdsenseService', () => {
  it('adds nothing without Advertising', () => {
    const { svc, win, scripts } = setup();
    svc.apply(false);
    expect(scripts.length).toBe(0);
    expect(win['adsbygoogle']).toBeUndefined();
  });

  it('adds the script once after Advertising, asking for non-personalised ads', () => {
    const { svc, win, scripts } = setup();
    svc.apply(true);
    svc.apply(true);
    expect(scripts.map((s) => s.src)).toEqual([ADSENSE_SRC]);
    const q = win['adsbygoogle'] as { pauseAdRequests: number; requestNonPersonalizedAds: number };
    expect(q.pauseAdRequests).toBe(0);
    expect(q.requestNonPersonalizedAds).toBe(1);
  });

  it('pauses ad requests on withdrawal and resumes them on a new allow, without a second script', () => {
    const { svc, win, scripts } = setup();
    svc.apply(true);
    svc.apply(false);
    expect((win['adsbygoogle'] as { pauseAdRequests: number }).pauseAdRequests).toBe(1);
    svc.apply(true);
    expect((win['adsbygoogle'] as { pauseAdRequests: number }).pauseAdRequests).toBe(0);
    expect(scripts.length).toBe(1);
  });
});
