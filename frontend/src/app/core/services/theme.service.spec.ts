import { TestBed } from '@angular/core/testing';
import { CONSENT_STORAGE, CookieConsentService } from '../consent/consent.service';
import { MemoryStorage } from '../consent/testing';
import { ThemeService } from './theme.service';

describe('ThemeService and consent', () => {
  let storage: MemoryStorage;
  function make(): { theme: ThemeService; consent: CookieConsentService } {
    TestBed.configureTestingModule({ providers: [{ provide: CONSENT_STORAGE, useValue: storage }] });
    return { theme: TestBed.inject(ThemeService), consent: TestBed.inject(CookieConsentService) };
  }
  beforeEach(() => (storage = new MemoryStorage()));
  afterEach(() => document.documentElement.classList.remove('dark'));

  it('ignores a theme saved before the banner until Functional is allowed, then uses it', () => {
    storage.setItem('app-theme', 'light');
    const { theme, consent } = make();
    expect(theme.currentTheme()).toBe('dark');
    consent.save({ analytics: false, advertising: false, functional: true });
    expect(theme.currentTheme()).toBe('light');
  });

  it('saves nothing when the visitor toggles without Functional', () => {
    const { theme } = make();
    theme.toggleTheme();
    expect(theme.currentTheme()).toBe('light');
    expect(storage.getItem('app-theme')).toBeNull();
  });

  it('keeps and saves a theme picked this visit once Functional is allowed', () => {
    const { theme, consent } = make();
    theme.toggleTheme();
    consent.save({ analytics: false, advertising: false, functional: true });
    expect(theme.currentTheme()).toBe('light');
    expect(storage.getItem('app-theme')).toBe('light');
  });

  it('saves a toggle while Functional is allowed', () => {
    const { theme, consent } = make();
    consent.save({ analytics: false, advertising: false, functional: true });
    theme.toggleTheme();
    expect(storage.getItem('app-theme')).toBe('light');
  });
});
