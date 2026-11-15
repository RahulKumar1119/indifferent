import { DOCUMENT } from '@angular/common';
import { TestBed } from '@angular/core/testing';
import { fakeAsync, tick } from '@angular/core/testing';
import { ADS_ACCOUNT_ID, GA_MEASUREMENT_ID, GTAG_IDLE_MS, GoogleTagService } from './google-tag.service';

function fakeDoc() {
  const scripts: { src: string }[] = [];
  const listeners = new Map<string, () => void>();
  const win: Record<string, unknown> = {
    addEventListener: (e: string, f: () => void) => listeners.set(e, f),
    removeEventListener: (e: string) => listeners.delete(e),
  };
  const doc = {
    defaultView: win,
    createElement: () => ({ src: '' }),
    head: { appendChild: (s: { src: string }) => scripts.push(s) },
  };
  return { doc, win, scripts, listeners };
}

/** The commands pushed to the queue, as plain arrays. */
function commands(win: Record<string, unknown>): unknown[][] {
  return ((win['dataLayer'] as ArrayLike<unknown>[]) ?? []).map((a) => Array.from(a));
}
const names = (win: Record<string, unknown>) => commands(win).map((c) => (c[0] === 'config' ? `config:${c[1]}` : c[0] === 'consent' ? `consent:${c[1]}` : String(c[0])));

describe('GoogleTagService', () => {
  function setup() {
    const f = fakeDoc();
    TestBed.configureTestingModule({ providers: [{ provide: DOCUMENT, useValue: f.doc }] });
    return { ...f, svc: TestBed.inject(GoogleTagService) };
  }
  const none = { analytics: false, advertising: false, functional: false };

  it('defines and requests nothing before the visitor allows something', () => {
    const { svc, win, scripts } = setup();
    svc.apply(none, true);
    expect(win['dataLayer']).toBeUndefined();
    expect(win['gtag']).toBeUndefined();
    expect(scripts.length).toBe(0);
    expect(svc.gtag()).toBeNull();
    svc.loadNow();
    expect(scripts.length).toBe(0);
  });

  it('queues default, update, js, config in that order, then loads the library once', () => {
    const { svc, win, scripts } = setup();
    svc.apply({ analytics: true, advertising: true, functional: false }, true);
    svc.apply({ analytics: true, advertising: true, functional: false }, true);
    svc.loadNow();
    expect(names(win)).toEqual(['consent:default', 'consent:update', 'js', `config:${GA_MEASUREMENT_ID}`, `config:${ADS_ACCOUNT_ID}`, 'consent:update']);
    const def = commands(win)[0][2] as Record<string, string>;
    expect(def['analytics_storage']).toBe('denied');
    expect(def['ad_storage']).toBe('denied');
    expect(def['security_storage']).toBe('granted');
    expect(scripts.length).toBe(1);
  });

  it('configures only the ID whose category is allowed', () => {
    const a = setup();
    a.svc.apply({ ...none, analytics: true }, true);
    expect(names(a.win)).toContain(`config:${GA_MEASUREMENT_ID}`);
    expect(names(a.win)).not.toContain(`config:${ADS_ACCOUNT_ID}`);
    TestBed.resetTestingModule();
    const b = setup();
    b.svc.apply({ ...none, advertising: true }, true);
    expect(names(b.win)).toContain(`config:${ADS_ACCOUNT_ID}`);
    expect(names(b.win)).not.toContain(`config:${GA_MEASUREMENT_ID}`);
    expect(b.scripts[0].src).toContain(ADS_ACCOUNT_ID);
  });

  it('adds the second ID later without a second library or a repeated config', () => {
    const { svc, win, scripts } = setup();
    svc.apply({ ...none, analytics: true }, true);
    svc.apply({ ...none, analytics: true, advertising: true }, true);
    expect(names(win).filter((n) => n.startsWith('config:')).length).toBe(2);
    expect(names(win).filter((n) => n === 'js').length).toBe(1);
    expect(scripts.length).toBe(1);
  });

  it('on withdrawal denies the signals, stops analytics and hands out no tag', () => {
    const { svc, win } = setup();
    svc.apply({ analytics: true, advertising: true, functional: true }, true);
    expect(win[`ga-disable-${GA_MEASUREMENT_ID}`]).toBeFalse();
    svc.apply(none, true);
    expect(win[`ga-disable-${GA_MEASUREMENT_ID}`]).toBeTrue();
    const last = commands(win).at(-1)!;
    expect(last.slice(0, 2)).toEqual(['consent', 'update']);
    expect((last[2] as Record<string, string>)['analytics_storage']).toBe('denied');
    expect((last[2] as Record<string, string>)['ad_storage']).toBe('denied');
    expect(svc.gtag()).toBeNull();
  });

  it('keeps ad_personalization denied while personalised ads are off', () => {
    const { svc, win } = setup();
    svc.apply({ ...none, advertising: true }, true);
    const update = commands(win).find((c) => c[1] === 'update')![2] as Record<string, string>;
    expect(update['ad_personalization']).toBe('denied');
    expect(update['ad_user_data']).toBe('granted');
  });

  it('for a returning visitor waits for an interaction or the idle time', fakeAsync(() => {
    const a = setup();
    a.svc.apply({ ...none, analytics: true }, false);
    expect(a.scripts.length).toBe(0);
    a.listeners.get('click')!();
    expect(a.scripts.length).toBe(1);
    tick(GTAG_IDLE_MS);
    expect(a.scripts.length).toBe(1);
    TestBed.resetTestingModule();
    const b = setup();
    b.svc.apply({ ...none, analytics: true }, false);
    tick(GTAG_IDLE_MS - 1);
    expect(b.scripts.length).toBe(0);
    tick(1);
    expect(b.scripts.length).toBe(1);
  }));
});
