import { DOCUMENT } from '@angular/common';
import { Injectable, inject } from '@angular/core';

/**
 * The Google Ads conversion for a click on "Continue with Google". The
 * destination is the conversion label from Google Ads, not a secret: it is
 * visible in the page source of any site that uses it.
 */
export const SIGN_IN_CONVERSION = {
  sendTo: 'AW-17587685388/UgcLCITcm6kdEIyQu8JB',
  value: 1.0,
  currency: 'INR',
} as const;

/** How long to wait for Google to confirm before carrying on regardless. */
export const CONVERSION_TIMEOUT_MS = 1000;

interface GoogleTagWindow {
  /** Defined in index.html; queues commands for the Google tag. */
  gtag?: (...args: unknown[]) => void;
  /** Defined in index.html; loads the Google tag now instead of on the first interaction. Safe to call twice. */
  loadGtag?: () => void;
}

/**
 * Reports Google Ads conversions through the Google tag installed in
 * index.html.
 *
 * Signing in leaves the page for accounts.google.com, and a request still in
 * flight is cancelled by that navigation. So the caller hands over what to do
 * next, and it runs once Google has confirmed the event was sent. If Google
 * never confirms (an ad blocker, no network, the tag not loaded yet) it runs
 * after CONVERSION_TIMEOUT_MS, so the button is never left dead. This is the
 * purpose of the `event_callback` in Google's own snippet.
 */
@Injectable({ providedIn: 'root' })
export class AdsConversionService {
  private readonly document = inject(DOCUMENT);

  /**
   * Starts loading the Google tag now. Call it when a page with a conversion
   * opens: the tag otherwise waits for a first interaction or three seconds,
   * and a click that is the visitor's first interaction would be reported
   * before the Ads part of the tag has loaded, and be lost. Safe to call twice.
   */
  prepare(): void {
    try {
      (this.document.defaultView as (Window & GoogleTagWindow) | null)?.loadGtag?.();
    } catch {
      // Measuring must never break the page.
    }
  }

  /**
   * Reports the sign-in click, then calls `next` exactly once: after Google
   * confirms, after the timeout, or at once when there is no tag to report to.
   */
  trackSignInClick(next: () => void): void {
    let done = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const finish = () => {
      if (done) return;
      done = true;
      if (timer !== undefined) clearTimeout(timer);
      next();
    };

    const win = this.document.defaultView as (Window & GoogleTagWindow) | null;
    if (typeof win?.gtag !== 'function') {
      finish();
      return;
    }

    try {
      // The tag normally loads on the first interaction or after three seconds.
      // Load it now so the event is not queued behind that wait, and so the
      // queue holds js, config, config, then this event, in that order.
      win.loadGtag?.();
      timer = setTimeout(finish, CONVERSION_TIMEOUT_MS);
      win.gtag('event', 'conversion', {
        send_to: SIGN_IN_CONVERSION.sendTo,
        value: SIGN_IN_CONVERSION.value,
        currency: SIGN_IN_CONVERSION.currency,
        event_callback: finish,
      });
    } catch {
      finish();
    }
  }
}
