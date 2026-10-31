import { DOCUMENT } from '@angular/common';
import { Injector } from '@angular/core';
import { fakeAsync, tick } from '@angular/core/testing';
import { AdsConversionService, CONVERSION_TIMEOUT_MS, SIGN_IN_CONVERSION } from './ads-conversion.service';

/** A page with the two globals index.html defines, recording what happens in order. */
function fakePage() {
  const log: string[] = [];
  const events: unknown[][] = [];
  const win = {
    loadGtag: jasmine.createSpy('loadGtag').and.callFake(() => log.push('load')),
    gtag: jasmine.createSpy('gtag').and.callFake((...args: unknown[]) => {
      log.push('event');
      events.push(args);
    }),
  };
  /** What Google calls once it has sent the event. */
  const confirm = () => (events[0][2] as { event_callback: () => void }).event_callback();
  return { win, log, events, confirm };
}

function serviceFor(win: unknown): AdsConversionService {
  return Injector.create({
    providers: [
      { provide: DOCUMENT, useValue: { defaultView: win } },
      { provide: AdsConversionService, useClass: AdsConversionService, deps: [] },
    ],
  }).get(AdsConversionService);
}

describe('AdsConversionService', () => {
  it('reports the sign-in conversion with the Google Ads destination, value and currency', () => {
    const page = fakePage();
    serviceFor(page.win).trackSignInClick(() => undefined);

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
    serviceFor(page.win).trackSignInClick(() => undefined);
    expect(Object.keys(page.events[0][2] as object)).not.toContain('transaction_id');
  });

  it('keeps the destination in one place', () => {
    expect(SIGN_IN_CONVERSION.sendTo).toBe('AW-17587685388/UgcLCITcm6kdEIyQu8JB');
  });

  it('loads the Google tag before sending, so the event is not queued behind the wait for a first interaction', () => {
    const page = fakePage();
    serviceFor(page.win).trackSignInClick(() => undefined);
    expect(page.log).toEqual(['load', 'event']);
  });

  it('does not carry on until Google confirms, then carries on exactly once', fakeAsync(() => {
    const page = fakePage();
    const next = jasmine.createSpy('next');
    serviceFor(page.win).trackSignInClick(next);

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
    serviceFor(page.win).trackSignInClick(next);

    tick(CONVERSION_TIMEOUT_MS - 1);
    expect(next).not.toHaveBeenCalled();
    tick(1);
    expect(next).toHaveBeenCalledTimes(1);

    // A confirmation that turns up late changes nothing.
    page.confirm();
    expect(next).toHaveBeenCalledTimes(1);
  }));

  it('carries on at once when the page has no Google tag', () => {
    for (const win of [{}, { gtag: 'not a function' }, null]) {
      const next = jasmine.createSpy('next');
      serviceFor(win).trackSignInClick(next);
      expect(next).withContext(JSON.stringify(win)).toHaveBeenCalledTimes(1);
    }
  });

  it('works when the page defines gtag but not loadGtag', () => {
    const page = fakePage();
    const next = jasmine.createSpy('next');
    serviceFor({ gtag: page.win.gtag }).trackSignInClick(next);
    expect(page.events.length).toBe(1);
    page.confirm();
    expect(next).toHaveBeenCalledTimes(1);
  });

  it('carries on at once, once, when the tag throws', fakeAsync(() => {
    const next = jasmine.createSpy('next');
    serviceFor({
      gtag: () => {
        throw new Error('blocked');
      },
    }).trackSignInClick(next);
    expect(next).toHaveBeenCalledTimes(1);
    tick(CONVERSION_TIMEOUT_MS * 2); // the timer was cleared
    expect(next).toHaveBeenCalledTimes(1);
  }));

  describe('prepare', () => {
    it('loads the Google tag without sending anything', () => {
      const page = fakePage();
      serviceFor(page.win).prepare();
      expect(page.log).toEqual(['load']);
      expect(page.events.length).toBe(0);
    });

    it('does nothing, quietly, when there is no tag or the page has no window', () => {
      for (const win of [{}, { gtag: () => undefined }, null]) {
        expect(() => serviceFor(win).prepare()).withContext(JSON.stringify(win)).not.toThrow();
      }
    });

    it('never breaks the page if loading the tag throws', () => {
      const win = {
        loadGtag: () => {
          throw new Error('blocked');
        },
      };
      expect(() => serviceFor(win).prepare()).not.toThrow();
    });
  });

  it('does not leave a timer running after a confirmation', fakeAsync(() => {
    const page = fakePage();
    serviceFor(page.win).trackSignInClick(() => undefined);
    page.confirm();
    // fakeAsync fails the test if a timer is still pending when it ends.
  }));
});
