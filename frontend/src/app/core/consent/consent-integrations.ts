import { DOCUMENT, isPlatformBrowser } from '@angular/common';
import { Injectable, PLATFORM_ID, inject } from '@angular/core';
import { AdsenseService } from './adsense.service';
import { ADVERTISING_COOKIE_PATTERNS, ANALYTICS_COOKIE_PATTERNS, ConsentChoices } from './consent.model';
import { CookieConsentService } from './consent.service';
import { clearCookies } from './cookies';
import { FunctionalStorage } from './functional-storage.service';
import { GoogleTagService } from './google-tag.service';
import { RumService } from './rum.service';

/** Set by the build's prerender step, which must produce a page with no optional integration and no banner. */
export function isPrerendering(win: Window | null | undefined): boolean {
  return !!(win as (Window & { __PRERENDER__?: boolean }) | null | undefined)?.__PRERENDER__;
}

/**
 * Connects the visitor's choices to everything optional (#321): the Google tag,
 * AdSense, CloudWatch RUM, the cookies on this site's own domain and the
 * Functional browser storage.
 *
 * `start()` runs once at startup, before any of them is created. It applies a
 * saved choice first (so a returning visitor's answer is known before anything
 * loads), then follows every later change. A visitor who has not chosen gets
 * nothing: no tag, no script, no RUM. Withdrawing a choice stops the integration
 * for the rest of the visit as far as its library allows, removes the cookies and
 * storage this site can reach, and does not reach into other domains.
 */
@Injectable({ providedIn: 'root' })
export class ConsentIntegrations {
  private readonly consent = inject(CookieConsentService);
  private readonly document = inject(DOCUMENT);
  private readonly browser = isPlatformBrowser(inject(PLATFORM_ID));
  private readonly google = inject(GoogleTagService);
  private readonly adsense = inject(AdsenseService);
  private readonly rum = inject(RumService);
  private readonly functional = inject(FunctionalStorage);
  private started = false;

  start(): void {
    if (this.started || !this.browser || isPrerendering(this.document.defaultView)) return;
    this.started = true;
    if (this.consent.decided()) this.apply(this.consent.choices(), false);
    this.consent.changes$.subscribe(({ choices }) => this.apply(choices, true));
  }

  private apply(choices: ConsentChoices, justChosen: boolean): void {
    this.google.apply(choices, justChosen);
    this.adsense.apply(choices.advertising);
    this.rum.apply(choices.analytics);
    if (!choices.analytics) clearCookies(this.document, ANALYTICS_COOKIE_PATTERNS);
    if (!choices.advertising) clearCookies(this.document, ADVERTISING_COOKIE_PATTERNS);
    if (choices.functional) {
      this.functional.flushMemory();
    } else {
      this.functional.forget();
    }
  }
}
