import { TestBed } from '@angular/core/testing';
import { CONSENT_STORAGE, CookieConsentService } from './consent.service';
import { FUNCTIONAL_STORAGE_KEYS } from './consent.model';
import { FunctionalStorage } from './functional-storage.service';
import { MemoryStorage } from './testing';

function setup() {
  const storage = new MemoryStorage();
  TestBed.configureTestingModule({ providers: [{ provide: CONSENT_STORAGE, useValue: storage }] });
  return { storage, consent: TestBed.inject(CookieConsentService), fs: TestBed.inject(FunctionalStorage) };
}

describe('FunctionalStorage', () => {
  it('writes nothing and ignores stored values without Functional', () => {
    const { fs, storage } = setup();
    storage.setItem('app-theme', 'dark');
    expect(fs.getItem('app-theme')).toBeNull();
    expect(fs.setItem('app-theme', 'light')).toBeFalse();
    expect(storage.getItem('app-theme')).toBe('dark');
    expect(fs.getItem('app-theme')).toBe('light'); // held for this visit
  });

  it('reads and writes storage with Functional', () => {
    const { fs, storage, consent } = setup();
    consent.save({ analytics: false, advertising: false, functional: true });
    storage.setItem('app-theme', 'dark');
    expect(fs.getItem('app-theme')).toBe('dark');
    expect(fs.setItem('app-theme', 'light')).toBeTrue();
    expect(storage.getItem('app-theme')).toBe('light');
  });

  it('flushes a visit value only where storage has nothing, never over older data', () => {
    const { fs, storage, consent } = setup();
    storage.setItem('app-theme', 'dark'); // legacy value from before the banner
    fs.setItem('app-theme', 'light');
    fs.setItem('shorts-upload-speed', '5');
    consent.save({ analytics: false, advertising: false, functional: true });
    fs.flushMemory();
    expect(storage.getItem('app-theme')).toBe('dark');
    expect(storage.getItem('shorts-upload-speed')).toBe('5');
  });

  it('forget removes every covered key and the visit values, and leaves other keys', () => {
    const { fs, storage, consent } = setup();
    for (const k of FUNCTIONAL_STORAGE_KEYS) storage.setItem(k, '1');
    storage.setItem('refresh-token', 'x');
    fs.setItem('held', 'v');
    consent.rejectAll();
    fs.forget();
    for (const k of FUNCTIONAL_STORAGE_KEYS) expect(storage.getItem(k)).withContext(k).toBeNull();
    expect(storage.getItem('refresh-token')).toBe('x');
    expect(fs.getItem('held')).toBeNull();
  });

  it('removeItem works without consent', () => {
    const { fs, storage } = setup();
    storage.setItem('app-theme', 'dark');
    fs.removeItem('app-theme');
    expect(storage.getItem('app-theme')).toBeNull();
  });
});
