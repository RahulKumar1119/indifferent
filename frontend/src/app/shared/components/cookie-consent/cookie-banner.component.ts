import { ChangeDetectionStrategy, Component, afterNextRender, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { isPrerendering } from '../../../core/consent/consent-integrations';
import { CookieConsentService } from '../../../core/consent/consent.service';

/**
 * The first-visit consent banner (#321). It shows until the visitor chooses, on
 * every page and also for signed-in people, and not in the prerendered pages: it
 * appears only once the app is running in a browser. Accept All and Reject All
 * look and weigh the same; Customize Preferences opens the panel
 * (cookie-preferences.component.ts) and the banner steps aside while it is open.
 */
@Component({
  selector: 'app-cookie-banner',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterLink],
  styles: [`
    :host { display: contents; }
    .banner {
      position: fixed; z-index: 1000; inset-inline: var(--space-sm); inset-block-end: var(--space-sm);
      max-width: 46rem; margin-inline: auto; max-height: 85dvh; overflow-y: auto;
      padding: var(--space-md) var(--space-lg);
      background: var(--color-paper); color: var(--color-ink);
      border: 1px solid var(--color-rule); border-radius: var(--radius-surface);
      box-shadow: 0 12px 40px -12px color-mix(in oklab, var(--color-ink) 45%, transparent);
      font-family: var(--font-body);
    }
    .banner[hidden] { display: none; }
    h2 { margin: 0; font-size: var(--type-lg); font-weight: 600; }
    p { margin: var(--space-xs) 0 0; font-size: var(--type-sm); line-height: 1.5; color: var(--color-ink-2); }
    .actions { display: flex; flex-wrap: wrap; gap: var(--space-xs); margin-block-start: var(--space-md); }
    .actions .ui-btn { flex: 1 1 9rem; justify-content: center; }
    @media (max-width: 40rem) { .banner { inset-inline: 0; inset-block-end: 0; border-radius: var(--radius-surface) var(--radius-surface) 0 0; padding: var(--space-md); } }
  `],
  template: `
    @if (shown()) {
      <section class="banner" role="region" aria-labelledby="ck-banner-title" aria-describedby="ck-banner-text" data-cookie-banner [hidden]="consent.preferencesOpen()">
        <h2 id="ck-banner-title">We value your privacy</h2>
        <p id="ck-banner-text">
          Indifferent.fun uses essential cookies and browser storage to keep the website working. With your permission, we also use optional cookies and storage for
          analytics, advertising and functionality to improve your experience. You can accept all, reject the optional ones or choose, and change your mind at any time.
          <a routerLink="/privacy" class="ui-link">Privacy Policy</a>
        </p>
        <div class="actions">
          <button type="button" class="ui-btn ui-btn--primary" (click)="consent.acceptAll()">Accept All</button>
          <button type="button" class="ui-btn ui-btn--primary" (click)="consent.rejectAll()">Reject All</button>
          <button type="button" class="ui-btn ui-btn--secondary" (click)="customize($event)">Customize Preferences</button>
        </div>
      </section>
    }
  `,
})
export class CookieBannerComponent {
  protected readonly consent = inject(CookieConsentService);
  private readonly ready = signal(false);

  /** In the document from the moment the app runs in a browser until the visitor has chosen. */
  protected readonly shown = computed(() => this.ready() && this.consent.needsChoice());

  constructor() {
    // Runs in a browser only, after the first render. The prerender step sets __PRERENDER__ and gets no banner.
    afterNextRender(() => {
      if (!isPrerendering(window)) this.ready.set(true);
    });
  }

  protected customize(event: Event): void {
    this.consent.openPreferences(event.currentTarget as HTMLElement);
  }
}
