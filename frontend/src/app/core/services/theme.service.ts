import { Injectable, PLATFORM_ID, inject, signal } from '@angular/core';
import { isPlatformBrowser } from '@angular/common';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { CookieConsentService } from '../consent/consent.service';
import { FunctionalStorage } from '../consent/functional-storage.service';

export type Theme = 'dark' | 'light';

const THEME_STORAGE_KEY = 'app-theme';

/**
 * The colour theme. It is saved in the browser only while the visitor allows
 * Functional (#321): until then the choice lasts for the visit, and the page
 * opens in the default theme every time. A theme saved on an earlier visit is
 * used again as soon as Functional is allowed.
 */
@Injectable({ providedIn: 'root' })
export class ThemeService {
  private readonly isBrowser = isPlatformBrowser(inject(PLATFORM_ID));
  private readonly storage = inject(FunctionalStorage);
  private readonly consent = inject(CookieConsentService);
  private toggled = false;
  readonly currentTheme = signal<Theme>(this.loadTheme());

  constructor() {
    this.applyTheme(this.currentTheme());
    this.consent.changes$.pipe(takeUntilDestroyed()).subscribe(({ choices, previous }) => {
      if (!choices.functional || previous.functional) return;
      if (this.toggled) {
        // They picked a theme this visit before allowing storage: keep that and save it.
        this.storage.setItem(THEME_STORAGE_KEY, this.currentTheme());
        return;
      }
      const stored = this.loadTheme();
      if (stored !== this.currentTheme()) {
        this.currentTheme.set(stored);
        this.applyTheme(stored);
      }
    });
  }

  toggleTheme(): void {
    const next: Theme = this.currentTheme() === 'dark' ? 'light' : 'dark';
    this.toggled = true;
    this.currentTheme.set(next);
    this.applyTheme(next);
    if (this.isBrowser) {
      this.storage.setItem(THEME_STORAGE_KEY, next);
    }
  }

  private loadTheme(): Theme {
    const stored = this.storage.getItem(THEME_STORAGE_KEY);
    if (stored === 'dark' || stored === 'light') {
      return stored;
    }
    // Video tools feel premium in dark: default dark on first visit.
    return 'dark';
  }

  private applyTheme(theme: Theme): void {
    if (!this.isBrowser) {
      return;
    }
    const root = document.documentElement;
    if (theme === 'dark') {
      root.classList.add('dark');
    } else {
      root.classList.remove('dark');
    }
    root.setAttribute('data-theme', theme);
    root.style.colorScheme = theme;
  }
}
