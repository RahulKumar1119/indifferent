import { TestBed } from '@angular/core/testing';
import { CONSENT_KEY, CONSENT_VERSION, newRecord } from './consent.model';
import { CONSENT_CLOCK, CONSENT_STORAGE, ConsentChange, CookieConsentService } from './consent.service';
import { MemoryStorage } from './testing';

const NOW = Date.UTC(2026, 9, 29);

function make(storage: Storage | null, now = NOW): CookieConsentService {
  TestBed.configureTestingModule({
    providers: [{ provide: CONSENT_STORAGE, useValue: storage }, { provide: CONSENT_CLOCK, useValue: () => now }],
  });
  return TestBed.inject(CookieConsentService);
}

describe('CookieConsentService', () => {
  it('starts undecided with nothing optional allowed', () => {
    const s = make(new MemoryStorage());
    expect(s.decided()).toBeFalse();
    expect(s.needsChoice()).toBeTrue();
    for (const c of ['analytics', 'advertising', 'functional'] as const) expect(s.allowed(c)).toBeFalse();
    expect(s.allowed('necessary')).toBeTrue();
  });

  it('accept all, reject all and a custom choice are stored and published', () => {
    const storage = new MemoryStorage();
    const s = make(storage);
    const seen: ConsentChange[] = [];
    s.changes$.subscribe((c) => seen.push(c));
    s.acceptAll();
    expect(s.allowed('analytics') && s.allowed('advertising') && s.allowed('functional')).toBeTrue();
    expect(JSON.parse(storage.getItem(CONSENT_KEY)!).choices).toEqual({ analytics: true, advertising: true, functional: true });
    s.save({ analytics: false, advertising: true, functional: false });
    expect(s.allowed('analytics')).toBeFalse();
    expect(s.allowed('advertising')).toBeTrue();
    s.rejectAll();
    expect(s.allowed('advertising')).toBeFalse();
    expect(seen.length).toBe(3);
    expect(seen[2].previous.advertising).toBeTrue();
    expect(seen[2].choices.advertising).toBeFalse();
    expect(s.decided()).toBeTrue();
  });

  it('remembers the choice after a reload and forgets it after six months', () => {
    const storage = new MemoryStorage();
    storage.setItem(CONSENT_KEY, JSON.stringify(newRecord({ analytics: true, advertising: false, functional: true }, NOW)));
    expect(make(storage, NOW + 86400000).allowed('analytics')).toBeTrue();
    TestBed.resetTestingModule();
    const later = make(storage, Date.UTC(2027, 4, 1));
    expect(later.decided()).toBeFalse();
    expect(later.allowed('analytics')).toBeFalse();
  });

  it('asks again when the policy version changed, with every switch off', () => {
    const storage = new MemoryStorage();
    storage.setItem(CONSENT_KEY, JSON.stringify({ ...newRecord({ analytics: true, advertising: true, functional: true }, NOW), version: CONSENT_VERSION - 1 }));
    const s = make(storage);
    expect(s.needsChoice()).toBeTrue();
    expect(s.choices()).toEqual({ analytics: false, advertising: false, functional: false });
  });

  it('keeps the choice for this visit when storage is blocked', () => {
    const blocked = { getItem: () => { throw new Error('no'); }, setItem: () => { throw new Error('no'); } } as unknown as Storage;
    const s = make(blocked);
    expect(s.decided()).toBeFalse();
    s.acceptAll();
    expect(s.allowed('analytics')).toBeTrue();
    expect(s.persisted).toBeFalse();
    TestBed.resetTestingModule();
    expect(make(null).persisted).toBeFalse();
  });

  it('opens the panel for an opener, closes it and hands focus back once', () => {
    const s = make(new MemoryStorage());
    const el = document.createElement('button');
    s.openPreferences(el);
    expect(s.preferencesOpen()).toBeTrue();
    s.closePreferences();
    expect(s.preferencesOpen()).toBeFalse();
    expect(s.takeOpener()).toBe(el);
    expect(s.takeOpener()).toBeNull();
    s.openPreferences(el);
    s.rejectAll();
    expect(s.preferencesOpen()).toBeFalse();
  });
});
