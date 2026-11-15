import { isPlatformBrowser } from '@angular/common';
import { Injectable, PLATFORM_ID, inject } from '@angular/core';
import { FUNCTIONAL_STORAGE_KEYS } from './consent.model';
import { CONSENT_STORAGE, CookieConsentService } from './consent.service';

/**
 * Browser storage for things that only make the site nicer (#321): the theme,
 * the profile details, the upload speed estimate. It writes to local storage
 * only while the visitor allows Functional. Until then a value lives in memory
 * for the visit, so the page still behaves, and nothing is saved.
 *
 * Removing is always allowed: deleting what is stored needs no consent.
 */
@Injectable({ providedIn: 'root' })
export class FunctionalStorage {
  private readonly consent = inject(CookieConsentService);
  private readonly storage = inject(CONSENT_STORAGE);
  private readonly browser = isPlatformBrowser(inject(PLATFORM_ID));
  private readonly memory = new Map<string, string>();

  /** True while Functional is allowed. Reads a signal, so a computed or template that uses it updates. */
  get permitted(): boolean {
    return this.consent.allowed('functional');
  }

  getItem(key: string): string | null {
    if (!this.browser) return null;
    if (this.permitted) {
      try {
        const stored = this.storage?.getItem(key) ?? null;
        if (stored !== null) return stored;
      } catch {
        // Storage unreadable: fall through to what this visit holds.
      }
    }
    return this.memory.get(key) ?? null;
  }

  /** True when the value was saved in the browser; false when it is held for this visit only. */
  setItem(key: string, value: string): boolean {
    if (!this.browser) return false;
    if (this.permitted && this.storage) {
      try {
        this.storage.setItem(key, value);
        this.memory.delete(key);
        return true;
      } catch {
        // Quota or blocked: keep it for this visit.
      }
    }
    this.memory.set(key, value);
    return false;
  }

  removeItem(key: string): void {
    this.memory.delete(key);
    try {
      this.storage?.removeItem(key);
    } catch {
      // Nothing stored.
    }
  }

  /**
   * After Functional is turned on: keep what this visit held, but only where
   * storage has nothing for the key. A value saved on an earlier visit is the
   * person's data and is never overwritten by something typed before they chose.
   */
  flushMemory(): void {
    if (!this.permitted || !this.storage) return;
    for (const [key, value] of this.memory) {
      try {
        if (this.storage.getItem(key) === null) this.storage.setItem(key, value);
      } catch {
        // Keep it in memory.
        continue;
      }
      this.memory.delete(key);
    }
  }

  /** When Functional is turned off: delete everything it covers, here and in memory. */
  forget(): void {
    this.memory.clear();
    for (const key of FUNCTIONAL_STORAGE_KEYS) {
      try {
        this.storage?.removeItem(key);
      } catch {
        // Nothing stored.
      }
    }
  }
}
