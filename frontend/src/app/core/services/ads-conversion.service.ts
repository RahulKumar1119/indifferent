import { DOCUMENT } from '@angular/common';
import { Injectable, inject } from '@angular/core';
import { CookieConsentService } from '../consent/consent.service';
import { GoogleTagService } from '../consent/google-tag.service';

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


/**
 * Reports Google Ads conversions through the Google tag (GoogleTagService).
 * Nothing is reported, and the tag is not loaded, unless the visitor allowed
 * Advertising (#321); the sign-in itself never waits on that.
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
  private readonly consent = inject(CookieConsentService);
  private readonly tag = inject(GoogleTagService);

  /**
   * Starts loading the Google tag now. Call it when a page with a conversion
   * opens: the tag otherwise waits for a first interaction or three seconds,
   * and a click that is the visitor's first interaction would be reported
   * before the Ads part of the tag has loaded, and be lost. Safe to call twice.
   */
  prepare(): void {
    if (!this.consent.allowed('advertising')) return;
    try {
      this.tag.loadNow();
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

    // No Advertising consent, or the tag was never started: nothing to report, so carry on at once.
    const gtag = this.consent.allowed('advertising') ? this.tag.gtag() : null;
    if (!gtag || !this.document.defaultView) {
      finish();
      return;
    }
    try {
      // The tag normally loads on the first interaction or after three seconds.
      // Load it now so the event is not queued behind that wait, and so the
      // queue holds the defaults, the choice, js, config, then this event, in that order.
      this.tag.loadNow();
      timer = setTimeout(finish, CONVERSION_TIMEOUT_MS);
      gtag('event', 'conversion', {
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
