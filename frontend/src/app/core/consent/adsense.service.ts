import { DOCUMENT } from '@angular/common';
import { Injectable, inject } from '@angular/core';
import { PERSONALIZED_ADS_ENABLED } from './consent.model';

export const ADSENSE_SRC = 'https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client=ca-pub-4438011184531485';

type AdsenseQueue = unknown[] & { pauseAdRequests?: number; requestNonPersonalizedAds?: number };
type AdsenseWindow = Window & { adsbygoogle?: AdsenseQueue };

/**
 * Google AdSense (#321). The script is added to the page only after the visitor
 * allows Advertising, and not at all before. If they withdraw, the script cannot
 * be taken back out of a page that already ran it, but it is told to pause ad
 * requests, and the next page load does not add it again.
 */
@Injectable({ providedIn: 'root' })
export class AdsenseService {
  private readonly document = inject(DOCUMENT);
  private injected = false;

  apply(advertising: boolean): void {
    const win = this.document.defaultView as AdsenseWindow | null;
    if (!win) return;
    if (!advertising) {
      // Never loaded: leave the page exactly as it is.
      if (this.injected && win.adsbygoogle) win.adsbygoogle.pauseAdRequests = 1;
      return;
    }
    const queue = (win.adsbygoogle = win.adsbygoogle ?? ([] as unknown as AdsenseQueue));
    queue.pauseAdRequests = 0;
    if (!PERSONALIZED_ADS_ENABLED) queue.requestNonPersonalizedAds = 1;
    if (this.injected) return;
    this.injected = true;
    const script = this.document.createElement('script');
    script.async = true;
    script.src = ADSENSE_SRC;
    script.crossOrigin = 'anonymous';
    this.document.head.appendChild(script);
  }
}
