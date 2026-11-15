import { Injectable, InjectionToken, computed, inject, signal } from '@angular/core';
import { Subject } from 'rxjs';
import {
  ALL_OPTIONAL,
  CONSENT_KEY,
  ConsentCategory,
  ConsentChoices,
  ConsentRecord,
  NO_OPTIONAL,
  newRecord,
  parseRecord,
} from './consent.model';

/** Where the consent record is kept: this browser's local storage, or nothing when the browser blocks it. */
export const CONSENT_STORAGE = new InjectionToken<Storage | null>('consent-storage', {
  providedIn: 'root',
  factory: () => {
    try {
      return typeof localStorage === 'undefined' ? null : localStorage;
    } catch {
      return null; // storage switched off
    }
  },
});

/** The time source, so tests can move the clock. */
export const CONSENT_CLOCK = new InjectionToken<() => number>('consent-clock', { providedIn: 'root', factory: () => () => Date.now() });

export interface ConsentChange {
  choices: ConsentChoices;
  previous: ConsentChoices;
}

/**
 * The one place that knows what the visitor allowed (#321). It reads the saved
 * record when it is created, so the answer is known before any optional
 * integration starts, and publishes every change. Until a valid record exists
 * the visitor is treated as having allowed nothing optional.
 *
 * It never reads or writes anything but its own record. What each choice
 * switches on or off is done by the integrations (consent-integrations.ts).
 */
@Injectable({ providedIn: 'root' })
export class CookieConsentService {
  private readonly storage = inject(CONSENT_STORAGE);
  private readonly now = inject(CONSENT_CLOCK);

  private readonly _record = signal<ConsentRecord | null>(this.read());

  readonly record = this._record.asReadonly();
  /** What is allowed now. Nothing optional until the visitor has chosen. */
  readonly choices = computed<ConsentChoices>(() => this._record()?.choices ?? NO_OPTIONAL);
  /** False until a valid, current record exists: first visit, an expired choice, or a changed policy version. */
  readonly decided = computed(() => this._record() !== null);
  readonly needsChoice = computed(() => !this.decided());

  /** True while the preferences panel is open. */
  readonly preferencesOpen = signal(false);
  /** Where focus goes when the panel closes. */
  private opener: HTMLElement | null = null;

  /** Every saved choice, after it is stored. Subscribers act on it at once. */
  readonly changes$ = new Subject<ConsentChange>();

  /** True when the record could be stored. False when the browser blocks storage: the choice then lasts for this visit only. */
  persisted = this.storage !== null;

  allowed(category: ConsentCategory): boolean {
    return category === 'necessary' ? true : this.choices()[category];
  }

  acceptAll(): void {
    this.save(ALL_OPTIONAL);
  }

  rejectAll(): void {
    this.save(NO_OPTIONAL);
  }

  save(choices: ConsentChoices): void {
    const previous = this.choices();
    const record = newRecord(choices, this.now());
    this._record.set(record);
    this.persisted = this.write(record);
    this.preferencesOpen.set(false);
    this.changes$.next({ choices: record.choices, previous });
  }

  /** Opens the preferences panel. Pass the element that opened it, so focus can go back there. */
  openPreferences(opener?: HTMLElement | null): void {
    this.opener = opener ?? (typeof document !== 'undefined' ? (document.activeElement as HTMLElement | null) : null);
    this.preferencesOpen.set(true);
  }

  closePreferences(): void {
    this.preferencesOpen.set(false);
  }

  /** The element to give focus back to, once. */
  takeOpener(): HTMLElement | null {
    const el = this.opener;
    this.opener = null;
    return el;
  }

  private read(): ConsentRecord | null {
    try {
      return parseRecord(this.storage?.getItem(CONSENT_KEY) ?? null, this.now());
    } catch {
      return null;
    }
  }

  private write(record: ConsentRecord): boolean {
    if (!this.storage) return false;
    try {
      this.storage.setItem(CONSENT_KEY, JSON.stringify(record));
      return true;
    } catch {
      return false;
    }
  }
}
