import { Provider } from '@angular/core';
import { CONSENT_CLOCK, CONSENT_STORAGE } from './consent.service';
import { ALL_OPTIONAL, CONSENT_KEY, ConsentChoices, NO_OPTIONAL, newRecord } from './consent.model';

/** A minimal in-memory Storage for tests. */
export class MemoryStorage implements Storage {
  private data = new Map<string, string>();
  get length(): number { return this.data.size; }
  clear(): void { this.data.clear(); }
  getItem(key: string): string | null { return this.data.get(key) ?? null; }
  key(i: number): string | null { return [...this.data.keys()][i] ?? null; }
  removeItem(key: string): void { this.data.delete(key); }
  setItem(key: string, value: string): void { this.data.set(key, value); }
}

/** Providers for a test where the visitor already chose. Defaults to everything allowed. */
export function consentProviders(choices: ConsentChoices = ALL_OPTIONAL, storage: Storage = new MemoryStorage()): Provider[] {
  storage.setItem(CONSENT_KEY, JSON.stringify(newRecord(choices, Date.now())));
  return [{ provide: CONSENT_STORAGE, useValue: storage }, { provide: CONSENT_CLOCK, useValue: () => Date.now() }];
}

export const noConsentProviders = (): Provider[] => consentProviders(NO_OPTIONAL);
