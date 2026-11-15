import { DOCUMENT } from '@angular/common';
import { Injector } from '@angular/core';
import { fakeAsync, tick } from '@angular/core/testing';
import { CookieConsentService } from '../consent/consent.service';
import { GoogleTagService } from '../consent/google-tag.service';
import { AdsConversionService, CONVERSION_TIMEOUT_MS, SIGN_IN_CONVERSION } from './ads-conversion.service';

/** A tag with the two calls the service uses, recording what happens in order. */
function fakePage(opts: { loadThrows?: boolean } = {}) {
  const log: string[] = [];
  const events: unknown[][] = [];
  const gtag = jasmine.createSpy('gtag').and.callFake((...args: unknown[]) => {
    log.push('event');
    events.push(args);
  });
  const tag = {
    gtag: () => gtag as unknown,
    loadNow: jasmine.createSpy('loadNow').and.callFake(() => {
      if (opts.loadThrows) throw new Error('blocked');
      log.push('load');
    }),
  };
  /** What Google calls once it has sent the event. */
  const confirm = () => (events[0][2] as { event_callback: () => void }).event_callback();
  return { tag, gtag, log, events, confirm };
}

type FakeTag = { gtag: () => unknown; loadNow: () => void };

function serviceFor(tag: FakeTag, advertising = true, hasWindow = true): AdsConversionService {
  return Injector.create({
    providers: [
      { provide: DOCUMENT, useValue: { defaultView: hasWindow ? {} : null } },
      { provide: CookieConsentService, useValue: { allowed: (c: string) => c === 'necessary' || (c === 'advertising' && advertising) } },
      { provide: GoogleTagService, useValue: tag },
      { provide: AdsConversionService, useClass: AdsConversionService, deps: [] },
    ],
  }).get(AdsConversionService);
}

describe('AdsConversionService', () => {
  it('reports the sign-in conversion with the Google Ads destination, value and currency', () => {
    const page = fakePage();
    serviceFor(page.tag).trackSignInClick(() => undefined);

    const [command, name, params] = page.events[0] as [string, string, Record<string, unknown>];
    expect(command).toBe('event');
    expect(name).toBe('conversion');
    expect(params['send_to']).toBe('AW-17587685388/UgcLCITcm6kdEIyQu8JB');
    expect(params['value']).toBe(1);
    expect(params['currency']).toBe('INR');
    expect(typeof params['event_callback']).toBe('function');
  });

  it('leaves out transaction_id: a click has no order to deduplicate on', () => {
    const page = fakePage();
    serviceFor(page.tag).trackSignInClick(() => undefined);
    expect(Object.keys(page.events[0][2] as object)).not.toContain('transaction_id');
  });

  it('keeps the destination in one place', () => {
    expect(SIGN_IN_CONVERSION.sendTo).toBe('AW-17587685388/UgcLCITcm6kdEIyQu8JB');
  });

  it('loads the Google tag before sending, so the event is not queued behind the wait for a first interaction', () => {
    const page = fakePage();
    serviceFor(page.tag).trackSignInClick(() => undefined);
    expect(page.log).toEqual(['load', 'event']);
  });

  it('does not carry on until Google confirms, then carries on exactly once', fakeAsync(() => {
    const page = fakePage();
    const next = jasmine.createSpy('next');
    serviceFor(page.tag).trackSignInClick(next);

    tick(CONVERSION_TIMEOUT_MS - 1);
    expect(next).not.toHaveBeenCalled();

    page.confirm();
    expect(next).toHaveBeenCalledTimes(1);

    // The timeout must not fire it a second time, nor a second confirmation.
    page.confirm();
    tick(CONVERSION_TIMEOUT_MS * 3);
    expect(next).toHaveBeenCalledTimes(1);
  }));

  it('carries on after the timeout when Google never answers (an ad blocker, no network)', fakeAsync(() => {
    const page = fakePage();
    const next = jasmine.createSpy('next');
    serviceFor(page.tag).trackSignInClick(next);

    tick(CONVERSION_TIMEOUT_MS - 1);
    expect(next).not.toHaveBeenCalled();
    tick(1);
    expect(next).toHaveBeenCalledTimes(1);

    // A confirmation that turns up late changes nothing.
    page.confirm();
    expect(next).toHaveBeenCalledTimes(1);
  }));

  it('carries on at once when there is no Google tag or no window', () => {
    for (const [tag, win] of [[{ gtag: () => null, loadNow: () => undefined }, true], [fakePage().tag, false]] as const) {
      const next = jasmine.createSpy('next');
      serviceFor(tag, true, win).trackSignInClick(next);
      expect(next).toHaveBeenCalledTimes(1);
    }
  });

  it('reports nothing and loads nothing without Advertising consent, and still carries on', () => {
    const page = fakePage();
    const next = jasmine.createSpy('next');
    serviceFor(page.tag, false).trackSignInClick(next);
    serviceFor(page.tag, false).prepare();
    expect(next).toHaveBeenCalledTimes(1);
    expect(page.log).toEqual([]);
  });

  it('carries on at once, once, when the tag throws', fakeAsync(() => {
    const next = jasmine.createSpy('next');
    const tag = {
      gtag: () => () => {
        throw new Error('blocked');
      },
      loadNow: () => undefined,
    };
    serviceFor(tag).trackSignInClick(next);
    expect(next).toHaveBeenCalledTimes(1);
    tick(CONVERSION_TIMEOUT_MS * 2); // the timer was cleared
    expect(next).toHaveBeenCalledTimes(1);
  }));

  describe('prepare', () => {
    it('loads the Google tag without sending anything', () => {
      const page = fakePage();
      serviceFor(page.tag).prepare();
      expect(page.log).toEqual(['load']);
      expect(page.events.length).toBe(0);
    });

    it('never breaks the page if loading the tag throws', () => {
      const page = fakePage({ loadThrows: true });
      expect(() => serviceFor(page.tag).prepare()).not.toThrow();
    });
  });

  it('does not leave a timer running after a confirmation', fakeAsync(() => {
    const page = fakePage();
    serviceFor(page.tag).trackSignInClick(() => undefined);
    page.confirm();
    // fakeAsync fails the test if a timer is still pending when it ends.
  }));
});
