import { TestBed } from '@angular/core/testing';
import { CONSENT_STORAGE, CookieConsentService } from '../core/consent/consent.service';
import { MemoryStorage } from '../core/consent/testing';
import { FUNCTIONAL_STORAGE_KEYS } from '../core/consent/consent.model';
import { FunctionalStorage } from '../core/consent/functional-storage.service';
import { CAPTION_PREFERENCE_KEY, CaptionPreference } from './caption-preference.service';
import { NEW_JOB_CAPTION_STYLE } from './caption-style';

function setup(functional: boolean) {
  const storage = new MemoryStorage();
  TestBed.configureTestingModule({ providers: [{ provide: CONSENT_STORAGE, useValue: storage }] });
  const consent = TestBed.inject(CookieConsentService);
  if (functional) consent.save({ analytics: false, advertising: false, functional: true });
  return { storage, consent, pref: TestBed.inject(CaptionPreference) };
}
const karaoke = { ...NEW_JOB_CAPTION_STYLE, preset: 'karaoke' as const, font: 'serif' as const, aboveControls: false };

describe('CaptionPreference', () => {
  it('is covered by the consent withdrawal list', () => {
    expect(FUNCTIONAL_STORAGE_KEYS).toContain(CAPTION_PREFERENCE_KEY);
  });

  it('writes nothing, and holds nothing in memory, without Functional', () => {
    const { pref, storage } = setup(false);
    expect(pref.remembering).toBeFalse();
    expect(pref.save(karaoke)).toBeFalse();
    expect(storage.getItem(CAPTION_PREFERENCE_KEY)).toBeNull();
    expect(pref.load()).toBeNull();
    expect(pref.startingStyle()).toEqual(NEW_JOB_CAPTION_STYLE);
  });

  it('remembers the last style with Functional, as the start for a new job', () => {
    const { pref, storage } = setup(true);
    expect(pref.save(karaoke)).toBeTrue();
    expect(storage.getItem(CAPTION_PREFERENCE_KEY)).toContain('"karaoke"');
    expect(pref.startingStyle()).toEqual(karaoke);
  });

  it('ignores a value that was tampered with', () => {
    const { pref, storage } = setup(true);
    storage.setItem(CAPTION_PREFERENCE_KEY, '{"preset":"neon","fontSize":9999,"font":"comic"}');
    expect(pref.startingStyle()).toEqual({ ...NEW_JOB_CAPTION_STYLE, fontSize: 72 });
    storage.setItem(CAPTION_PREFERENCE_KEY, 'not json');
    expect(pref.startingStyle()).toEqual(NEW_JOB_CAPTION_STYLE);
  });

  it('stops using the remembered style the moment Functional is withdrawn, and removes it', () => {
    const { pref, storage, consent } = setup(true);
    pref.save(karaoke);
    consent.save({ analytics: false, advertising: false, functional: false });
    TestBed.inject(FunctionalStorage).forget();
    expect(pref.remembering).toBeFalse();
    expect(pref.load()).toBeNull();
    expect(storage.getItem(CAPTION_PREFERENCE_KEY)).toBeNull();
  });
});
