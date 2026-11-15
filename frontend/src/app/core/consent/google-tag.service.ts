import { DOCUMENT } from '@angular/common';
import { Injectable, inject } from '@angular/core';
import { ConsentChoices, googleConsentSignals } from './consent.model';

export const GA_MEASUREMENT_ID = 'G-D2GB04M42D';
/** The Google Ads conversion account. */
export const ADS_ACCOUNT_ID = 'AW-17587685388';
export const GTAG_SRC = 'https://www.googletagmanager.com/gtag/js';
/** How long the tag waits for a first interaction before it loads anyway. */
export const GTAG_IDLE_MS = 3000;
const INTERACTION_EVENTS = ['scroll', 'click', 'touchstart', 'keydown'] as const;

type GoogleWindow = Window & {
  dataLayer?: unknown[];
  gtag?: (...args: unknown[]) => void;
  [key: `ga-disable-${string}`]: boolean | undefined;
};

/**
 * The Google tag, in Google's Consent Mode v2 "basic" form (#321): nothing is
 * defined on the page and nothing is requested from Google until the visitor
 * allows Analytics or Advertising. Then the defaults (everything denied, the
 * security signal granted), the visitor's choice and the tag's own commands are
 * queued in that order, and gtag.js is loaded once. Each product ID is
 * configured only while its own category is allowed. This replaces the loader
 * that used to sit in index.html, so there is one place and no duplicate tag.
 */
@Injectable({ providedIn: 'root' })
export class GoogleTagService {
  private readonly document = inject(DOCUMENT);
  private defaulted = false;
  private started = false;
  private injected = false;
  private waiting = false;
  private readonly configured = new Set<string>();
  private allowedNow: Pick<ConsentChoices, 'analytics' | 'advertising'> = { analytics: false, advertising: false };

  private get win(): GoogleWindow | null {
    return this.document.defaultView as GoogleWindow | null;
  }

  /**
   * Applies the visitor's choices. `immediate` loads the library now (the visitor
   * just chose); otherwise it waits for a first interaction or a few seconds,
   * as the page always did for returning visitors.
   */
  apply(choices: ConsentChoices, immediate: boolean): void {
    const win = this.win;
    if (!win) return;
    this.allowedNow = { analytics: choices.analytics, advertising: choices.advertising };
    const wanted = choices.analytics || choices.advertising;
    // Never allowed and nothing loaded: there is nothing to tell Google.
    if (!wanted && !this.defaulted) return;
    this.ensureQueue(win);
    if (!this.defaulted) {
      this.defaulted = true;
      win.gtag!('consent', 'default', { ...googleConsentSignals({ analytics: false, advertising: false, functional: false }) });
    }
    win.gtag!('consent', 'update', googleConsentSignals(choices));
    // Google's own switch for stopping Analytics on a page that already loaded the tag.
    win[`ga-disable-${GA_MEASUREMENT_ID}`] = !choices.analytics;
    if (!wanted) return;
    if (!this.started) {
      this.started = true;
      win.gtag!('js', new Date());
    }
    if (choices.analytics && !this.configured.has(GA_MEASUREMENT_ID)) {
      this.configured.add(GA_MEASUREMENT_ID);
      win.gtag!('config', GA_MEASUREMENT_ID);
    }
    if (choices.advertising && !this.configured.has(ADS_ACCOUNT_ID)) {
      this.configured.add(ADS_ACCOUNT_ID);
      win.gtag!('config', ADS_ACCOUNT_ID);
    }
    if (immediate) this.inject();
    else this.injectWhenInteracted();
  }

  /** Loads the library now if the visitor allowed it. Safe to call twice. A visitor who did not allow it is left alone. */
  loadNow(): void {
    if (this.allowedNow.analytics || this.allowedNow.advertising) this.inject();
  }

  /** The tag's command queue, or null when it was never started (the visitor did not allow it). */
  gtag(): ((...args: unknown[]) => void) | null {
    return this.started && !this.isStopped() ? (this.win?.gtag ?? null) : null;
  }

  private isStopped(): boolean {
    return !this.allowedNow.analytics && !this.allowedNow.advertising;
  }

  private ensureQueue(win: GoogleWindow): void {
    win.dataLayer = win.dataLayer ?? [];
    if (!win.gtag) {
      // Google's tag reads Arguments objects from the queue, not arrays.
      // eslint-disable-next-line prefer-rest-params
      win.gtag = function () { (win.dataLayer as unknown[]).push(arguments); };
    }
  }

  private injectWhenInteracted(): void {
    if (this.injected || this.waiting) return;
    this.waiting = true;
    const win = this.win!;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const go = () => {
      if (timer !== undefined) clearTimeout(timer);
      for (const e of INTERACTION_EVENTS) win.removeEventListener(e, go);
      this.waiting = false;
      this.loadNow();
    };
    for (const e of INTERACTION_EVENTS) win.addEventListener(e, go, { once: true, passive: true });
    timer = setTimeout(go, GTAG_IDLE_MS);
  }

  private inject(): void {
    if (this.injected) return;
    this.injected = true;
    const script = this.document.createElement('script');
    // The library is loaded for the first ID the visitor allowed; it serves every ID configured on the page.
    const id = this.allowedNow.analytics ? GA_MEASUREMENT_ID : ADS_ACCOUNT_ID;
    script.src = `${GTAG_SRC}?id=${id}`;
    script.async = true;
    this.document.head.appendChild(script);
  }
}
