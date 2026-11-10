import { Component, OnInit, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { RouterLink } from '@angular/router';
import { PublicHeaderComponent } from '../../shared/components/public-header/public-header.component';
import { SiteFooterComponent } from '../../shared/components/site-footer/site-footer.component';
import { ROBOTS_NOINDEX, setPageSeo, setRobots } from '../../shared/seo';

/** The page for any path the app does not know (#256): the shared public chrome and three ways on. */
@Component({
  selector: 'app-not-found',
  standalone: true,
  imports: [RouterLink, PublicHeaderComponent, SiteFooterComponent],
  styles: [`
    :host { display: block; }
    .page { min-height: 100dvh; display: flex; flex-direction: column; background: var(--color-paper); color: var(--color-ink); font-family: var(--font-body); }
    main { flex: 1; }
    .wrap { width: min(var(--container), 100%); margin-inline: auto; padding: calc(56px + var(--space-3xl)) var(--gutter) var(--space-3xl); }
    .code { margin: 0; font-size: var(--type-sm); font-weight: 600; color: var(--color-accent-text); font-variant-numeric: tabular-nums; }
    h1 { margin: var(--space-xs) 0 0; max-width: 14ch; font-family: var(--font-display); font-weight: 500; font-size: var(--type-display); line-height: 1.02; letter-spacing: var(--tracking-display); text-wrap: balance; }
    .lead { margin: var(--space-lg) 0 0; max-width: 46ch; font-size: var(--type-lead); line-height: 1.45; color: var(--color-ink-2); }
    .actions { display: flex; flex-wrap: wrap; gap: var(--space-sm); margin-top: var(--space-xl); }
    .ui-btn { white-space: nowrap; }
  `],
  template: `
    <div class="page hm">
      <app-public-header />
      <main id="main">
        <div class="wrap">
          <p class="code">Error 404</p>
          <h1>Page not found</h1>
          <p class="lead">The page you're looking for doesn't exist or has been moved.</p>
          <div class="actions">
            <a routerLink="/" class="ui-btn ui-btn--primary">Go to the home page</a>
            <a routerLink="/new" class="ui-btn ui-btn--secondary">Make a video</a>
            <a routerLink="/help" class="ui-btn ui-btn--quiet">Help and FAQ</a>
          </div>
        </div>
      </main>
      <app-site-footer />
    </div>
  `,
})
export class NotFoundComponent implements OnInit {
  private readonly document = inject(DOCUMENT);

  ngOnInit(): void {
    // The host answers every path with the app shell and a 200, so a missing
    // page has to say it is missing itself: noindex keeps it out of search.
    setPageSeo(this.document, {
      title: 'Page not found | Indifferent',
      description: 'This page does not exist. Go back to the Indifferent home page or the journal.',
      canonical: 'https://indifferent.fun/',
    });
    setRobots(this.document, ROBOTS_NOINDEX);
  }
}
