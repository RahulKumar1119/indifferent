import { DOCUMENT } from '@angular/common';
import { TestBed } from '@angular/core/testing';
import { AdsenseService } from './adsense.service';
import { CONSENT_STORAGE, CookieConsentService } from './consent.service';
import { ConsentIntegrations, isPrerendering } from './consent-integrations';
import { cookieNames } from './cookies';
import { FunctionalStorage } from './functional-storage.service';
import { GoogleTagService } from './google-tag.service';
import { RumService } from './rum.service';
import { MemoryStorage } from './testing';
import { CONSENT_KEY, newRecord } from './consent.model';

function setup(opts: { saved?: { analytics: boolean; advertising: boolean; functional: boolean }; prerender?: boolean } = {}) {
  const storage = new MemoryStorage();
  if (opts.saved) storage.setItem(CONSENT_KEY, JSON.stringify(newRecord(opts.saved, Date.now())));
  const google = { apply: jasmine.createSpy('google') };
  const adsense = { apply: jasmine.createSpy('adsense') };
  const rum = { apply: jasmine.createSpy('rum') };
  const doc = opts.prerender ? ({ defaultView: { __PRERENDER__: true } } as unknown as Document) : document;
  TestBed.configureTestingModule({
    providers: [
      { provide: CONSENT_STORAGE, useValue: storage },
      { provide: GoogleTagService, useValue: google },
      { provide: AdsenseService, useValue: adsense },
      { provide: RumService, useValue: rum },
      { provide: DOCUMENT, useValue: doc },
    ],
  });
  return { storage, google, adsense, rum, consent: TestBed.inject(CookieConsentService), fs: TestBed.inject(FunctionalStorage), integ: TestBed.inject(ConsentIntegrations) };
}

describe('ConsentIntegrations', () => {
  afterEach(() => {
    for (const n of cookieNames(document)) document.cookie = `${n}=; max-age=0; path=/`;
  });

  it('starts nothing for a visitor who has not chosen, and leaves old storage alone', () => {
    const { integ, google, adsense, rum, storage } = setup();
    storage.setItem('app-theme', 'dark');
    integ.start();
    expect(google.apply).not.toHaveBeenCalled();
    expect(adsense.apply).not.toHaveBeenCalled();
    expect(rum.apply).not.toHaveBeenCalled();
    expect(storage.getItem('app-theme')).toBe('dark');
  });

  it('applies a saved choice at startup, as a returning visitor', () => {
    const { integ, google, adsense, rum } = setup({ saved: { analytics: true, advertising: false, functional: true } });
    integ.start();
    expect(google.apply).toHaveBeenCalledWith({ analytics: true, advertising: false, functional: true }, false);
    expect(adsense.apply).toHaveBeenCalledWith(false);
    expect(rum.apply).toHaveBeenCalledWith(true);
  });

  it('applies a new choice at once and loads immediately', () => {
    const { integ, google, adsense, rum, consent } = setup();
    integ.start();
    integ.start(); // a second start does not subscribe twice
    consent.acceptAll();
    expect(google.apply).toHaveBeenCalledTimes(1);
    expect(google.apply.calls.mostRecent().args[1]).toBeTrue();
    expect(adsense.apply).toHaveBeenCalledWith(true);
    expect(rum.apply).toHaveBeenCalledWith(true);
  });

  it('on rejection removes the cookies this site can reach and the Functional storage', () => {
    const { integ, consent, storage } = setup({ saved: { analytics: true, advertising: true, functional: true } });
    integ.start();
    document.cookie = '_ga=1; path=/';
    document.cookie = '_gcl_au=2; path=/';
    document.cookie = 'other=3; path=/';
    storage.setItem('app-theme', 'dark');
    storage.setItem('indifferent.infographic.draft.v1', '{}');
    storage.setItem('refresh-token', 'x');
    consent.rejectAll();
    expect(cookieNames(document)).toEqual(['other']);
    expect(storage.getItem('app-theme')).toBeNull();
    expect(storage.getItem('indifferent.infographic.draft.v1')).toBeNull();
    expect(storage.getItem('refresh-token')).toBe('x');
  });

  it('keeps cookies of a category that stays allowed', () => {
    const { integ, consent } = setup({ saved: { analytics: true, advertising: true, functional: false } });
    integ.start();
    document.cookie = '_ga=1; path=/';
    document.cookie = '_gcl_au=2; path=/';
    consent.save({ analytics: true, advertising: false, functional: false });
    expect(cookieNames(document)).toEqual(['_ga']);
  });

  it('does nothing at all while prerendering', () => {
    const { integ, google, consent } = setup({ prerender: true });
    integ.start();
    consent.acceptAll();
    expect(google.apply).not.toHaveBeenCalled();
    expect(isPrerendering({ __PRERENDER__: true } as unknown as Window)).toBeTrue();
    expect(isPrerendering(null)).toBeFalse();
  });
});
