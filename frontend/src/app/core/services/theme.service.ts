import { Injectable, PLATFORM_ID, inject, signal } from '@angular/core';
import { isPlatformBrowser } from '@angular/common';

export type Theme = 'dark' | 'light';

const THEME_STORAGE_KEY = 'app-theme';

@Injectable({ providedIn: 'root' })
export class ThemeService {
  private readonly isBrowser = isPlatformBrowser(inject(PLATFORM_ID));

  readonly currentTheme = signal<Theme>(this.loadTheme());

  constructor() {
    this.applyTheme(this.currentTheme());
  }

  toggleTheme(): void {
    const next: Theme = this.currentTheme() === 'dark' ? 'light' : 'dark';
    this.currentTheme.set(next);
    this.applyTheme(next);
    if (this.isBrowser) {
      localStorage.setItem(THEME_STORAGE_KEY, next);
    }
  }

  private loadTheme(): Theme {
    // Server prerender has no localStorage: fall back to the default theme.
    if (!this.isBrowser) {
      return 'light';
    }
    const stored = localStorage.getItem(THEME_STORAGE_KEY);
    if (stored === 'dark' || stored === 'light') {
      return stored;
    }
    // Default to light if no preference stored
    return 'light';
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
  }
}
