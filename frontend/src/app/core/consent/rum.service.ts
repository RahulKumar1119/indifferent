import { Injectable, InjectionToken, inject } from '@angular/core';

/** The part of the CloudWatch RUM client this site uses. */
export interface RumHandle {
  enable(): void;
  disable(): void;
  allowCookies(allow: boolean): void;
}

/**
 * Starts the RUM client. The library is imported here and nowhere else, so its
 * code is not even downloaded for a visitor who has not allowed Analytics.
 */
export const RUM_FACTORY = new InjectionToken<() => Promise<RumHandle>>('rum-factory', {
  providedIn: 'root',
  factory: () => async () => {
    const { AwsRum } = await import('aws-rum-web');
    return new AwsRum('15e33481-cc75-479b-aff9-6781ccead8f6', '1.0.0', 'ap-south-1', {
      sessionSampleRate: 1,
      identityPoolId: 'ap-south-1:6a7f63ba-3e57-4ff8-a7d7-296f9a49b880',
      endpoint: 'https://dataplane.rum.ap-south-1.amazonaws.com',
      telemetries: ['performance', 'errors', 'http'],
      allowCookies: true,
      enableXRay: true,
      signing: true,
    }) as RumHandle;
  },
});

/**
 * Amazon CloudWatch RUM, under the Analytics choice (#321). Not created, and its
 * code not loaded, until Analytics is allowed. On withdrawal it is switched off
 * and told to stop using cookies; a later allow switches it back on.
 */
@Injectable({ providedIn: 'root' })
export class RumService {
  private readonly create = inject(RUM_FACTORY);
  private handle: RumHandle | null = null;
  private starting: Promise<void> | null = null;
  private wanted = false;

  apply(analytics: boolean): void {
    this.wanted = analytics;
    if (analytics) {
      if (this.handle) {
        this.handle.enable();
        this.handle.allowCookies(true);
      } else {
        this.start();
      }
    } else if (this.handle) {
      this.handle.disable();
      this.handle.allowCookies(false);
    }
  }

  private start(): void {
    if (this.starting) return;
    this.starting = this.create()
      .then((handle) => {
        this.handle = handle;
        // The visitor may have changed their mind while the library was loading.
        if (!this.wanted) {
          handle.disable();
          handle.allowCookies(false);
        }
      })
      .catch(() => {
        // Measuring must never break the page.
        this.starting = null;
      });
  }
}
