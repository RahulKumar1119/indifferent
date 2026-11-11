import { Component, OnInit, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { RouterLink } from '@angular/router';
import { PublicHeaderComponent } from '../../shared/components/public-header/public-header.component';
import { SiteFooterComponent } from '../../shared/components/site-footer/site-footer.component';
import { setPageSeo } from '../../shared/seo';
import { PRODUCT_FACTS, joinOr, shortsLengthLabel } from '../../core/config/product-facts';

export interface UsageLimit {
  what: string;
  allowance: string;
  resets: string;
}

const L = PRODUCT_FACTS.limits;
const S = PRODUCT_FACTS.story;
const MIDNIGHT = 'Midnight UTC';

/** The limits table, built only from PRODUCT_FACTS so it cannot drift from the API. */
export const USAGE_LIMITS: UsageLimit[] = [
  { what: 'Quiz projects', allowance: `${L.quizProjectsPerDay} a day`, resets: MIDNIGHT },
  { what: 'Question files and logos', allowance: `${L.fileUploadsPerDay} a day`, resets: MIDNIGHT },
  { what: 'Shorts uploads', allowance: `${L.shortsUploadsPerDay} a day`, resets: MIDNIGHT },
  { what: 'Shorts retries', allowance: `${L.shortsRetriesPerDay} a day`, resets: MIDNIGHT },
  { what: 'Thumbnail sets', allowance: `${L.thumbnailSetsPerDay} a day`, resets: MIDNIGHT },
  { what: 'Story plans', allowance: `${L.storyPlansPerHour} an hour`, resets: 'Any 60 minutes' },
  { what: 'Story videos', allowance: `${S.perDay} a day, ${S.maxActive} at a time`, resets: 'Any 24 hours' },
  { what: 'Stock searches', allowance: `${L.stockSearchesPerHour} an hour`, resets: 'Hourly' },
];

/**
 * "Free during beta" (#255). There is no paid plan, so the page states what is
 * free, the limits the API enforces, what beta means, and how far ahead people
 * hear about any change. It names no price and no tier.
 */
@Component({
  selector: 'app-pricing',
  standalone: true,
  imports: [RouterLink, PublicHeaderComponent, SiteFooterComponent],
  styles: [`
    :host { display: block; }
    .page { background: var(--color-paper); color: var(--color-ink); font-family: var(--font-body); font-size: var(--type-md); line-height: 1.55; -webkit-font-smoothing: antialiased; }
    .wrap { width: min(var(--container), 100%); margin-inline: auto; padding-inline: var(--gutter); }
    h1, h2, h3, p, ul { margin: 0; }
    h1 { font-family: var(--font-display); font-weight: 500; font-size: var(--type-display); line-height: 1.02; letter-spacing: var(--tracking-display); text-wrap: balance; max-width: 12ch; }
    h2 { font-family: var(--font-display); font-weight: 500; font-size: var(--type-section); line-height: 1.04; letter-spacing: var(--tracking-display); text-wrap: balance; }
    h3 { font-size: var(--type-md); font-weight: 600; }
    .hero { padding: calc(56px + var(--space-3xl)) 0 var(--space-3xl); }
    .lead { margin-top: var(--space-lg); max-width: 50ch; font-size: var(--type-lead); line-height: 1.45; color: var(--color-ink-2); }
    .actions { display: flex; flex-wrap: wrap; gap: var(--space-sm); margin-top: var(--space-xl); }
    .block { padding: var(--space-3xl) 0; border-top: 1px solid var(--color-rule); }
    .tint { background: var(--color-paper-2); }
    .body { margin-top: var(--space-md); max-width: 62ch; color: var(--color-ink-2); }
    .rows { list-style: none; padding: 0; margin-top: var(--space-xl); border-top: 1px solid var(--color-rule); display: grid; }
    @media (min-width: 48rem) { .rows { grid-template-columns: repeat(2, minmax(0, 1fr)); column-gap: var(--space-2xl); } }
    .rows li { padding: var(--space-md) 0; border-bottom: 1px solid var(--color-rule); }
    .rows h3 a { display: inline-flex; align-items: center; min-height: 44px; color: var(--color-ink); text-underline-offset: 0.2em; }
    .rows h3 a:hover { color: var(--color-accent-text); }
    .rows p { color: var(--color-ink-2); }
    .table-wrap { margin-top: var(--space-xl); overflow-x: auto; }
    .table-wrap:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; }
    table { width: 100%; border-collapse: collapse; }
    th, td { padding: var(--space-sm) var(--space-md) var(--space-sm) 0; text-align: left; vertical-align: top; border-bottom: 1px solid var(--color-rule); }
    thead th { font-size: var(--type-sm); font-weight: 600; color: var(--color-ink-2); }
    tbody th { font-weight: 500; }
    td.n { font-weight: 600; font-variant-numeric: tabular-nums; }
    @media (min-width: 40rem) { td.n { white-space: nowrap; } }
    @media (max-width: 39.99rem) { th, td { padding-right: var(--space-sm); font-size: var(--type-sm); } }
    td.r { color: var(--color-ink-2); }
    .fine { margin-top: var(--space-md); max-width: 62ch; font-size: var(--type-sm); color: var(--color-ink-2); }
    .ticks { list-style: none; padding: 0; margin-top: var(--space-lg); display: grid; gap: var(--space-sm); max-width: 62ch; color: var(--color-ink-2); }
    .ticks li { position: relative; padding-left: var(--space-lg); }
    .ticks li::before { content: ''; position: absolute; left: 0; top: 0.7em; width: 10px; height: 2px; background: var(--color-accent); }
    .promise { margin-top: var(--space-xl); max-width: 46rem; padding: var(--space-lg); border-left: 3px solid var(--color-accent); background: var(--color-paper-2); border-radius: 0 var(--radius-surface) var(--radius-surface) 0; font-size: var(--type-lg); line-height: 1.45; }
    a.inline { display: inline-flex; align-items: center; min-height: 44px; font-weight: 600; color: var(--color-accent-text); text-underline-offset: 0.2em; }
    a.inline:hover { color: var(--color-ink); }
    a:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: 2px; }
    .links { display: flex; flex-wrap: wrap; gap: 0 var(--space-lg); margin-top: var(--space-sm); }
  `],
  template: `
    <div class="page hm">
      <app-public-header />

      <section class="hero" aria-labelledby="pricing-title">
        <div class="wrap">
          <h1 id="pricing-title">Free during beta</h1>
          <p class="lead">Every tool on Indifferent is free while the service is in beta. No credit card, no paid plan and nothing to upgrade to.</p>
          <div class="actions">
            <a routerLink="/login" class="ui-btn ui-btn--primary">Start creating</a>
            <a routerLink="/terms" fragment="service" class="ui-btn ui-btn--secondary">Read the beta terms</a>
          </div>
        </div>
      </section>

      <main>
        <section class="block tint" aria-labelledby="free-heading">
          <div class="wrap">
            <h2 id="free-heading">What is free</h2>
            <ul class="rows" data-testid="free-tools">
              @for (t of tools; track t.path) {
                <li>
                  <h3><a [routerLink]="t.path">{{ t.title }}</a></h3>
                  <p>{{ t.text }}</p>
                </li>
              }
            </ul>
          </div>
        </section>

        <section class="block" aria-labelledby="limits-heading">
          <div class="wrap">
            <h2 id="limits-heading">Usage limits</h2>
            <div class="table-wrap" tabindex="0" role="region" aria-labelledby="limits-heading">
              <table data-testid="limits">
                <thead>
                  <tr><th scope="col">Per account</th><th scope="col">Limit</th><th scope="col">Resets</th></tr>
                </thead>
                <tbody>
                  @for (l of limits; track l.what) {
                    <tr><th scope="row">{{ l.what }}</th><td class="n">{{ l.allowance }}</td><td class="r">{{ l.resets }}</td></tr>
                  }
                </tbody>
              </table>
            </div>
            <p class="fine">At a limit, the app says when it resets. Failed attempts count too.</p>
          </div>
        </section>


        <section class="block" aria-labelledby="beta-heading">
          <div class="wrap">
            <h2 id="beta-heading">What beta means</h2>
            <ul class="ticks">
              <li>Features, limits and availability can change.</li>
              <li>Free during beta is not a promise to stay free for good.</li>
              <li>We do not promise the service is always available or free of errors.</li>
            </ul>
            <p class="promise" data-testid="notice">
              Before we introduce paid plans or materially reduce these free limits, we will tell every registered account by email
              and with a notice in the app, at least {{ noticeDays }} days ahead.
            </p>
            <p class="links">
              <a routerLink="/terms" fragment="limits" class="inline">Limits in the terms</a>
              <a routerLink="/contact" class="inline">Ask a question</a>
            </p>
          </div>
        </section>
      </main>
      <app-site-footer />
    </div>
  `,
})
export class PricingComponent implements OnInit {
  private readonly document = inject(DOCUMENT);

  readonly limits = USAGE_LIMITS;
  readonly noticeDays = PRODUCT_FACTS.changeNoticeDays;
  /** Each tool with the limit on one file, from PRODUCT_FACTS. */
  readonly tools = [
    { title: 'Story to video', path: '/features/story-video', text: `A story of ${S.minStoryChars} to ${S.maxStoryChars.toLocaleString('en-US')} characters becomes ${S.minScenes} to ${S.maxScenes} scenes, up to ${S.maxTotalSeconds} seconds.` },
    { title: 'AI Shorts generator', path: '/features/ai-shorts', text: `One ${joinOr(PRODUCT_FACTS.shorts.inputFormats)} file from ${shortsLengthLabel()}, up to ${PRODUCT_FACTS.shorts.maxUploadGb} GB.` },
    { title: 'TXT to quiz video', path: '/features/txt-to-video-quiz', text: `One ${PRODUCT_FACTS.quiz.inputFormat} file of questions, up to ${PRODUCT_FACTS.quiz.maxFileMb} MB.` },
    { title: 'Watermark tool', path: '/features/watermark', text: 'Runs in your browser and needs no account.' },
  ];

  ngOnInit(): void {
    setPageSeo(this.document, {
      title: 'Pricing: free during beta | Indifferent',
      description:
        'Every Indifferent tool is free during the beta, with no credit card and no paid plan. See the daily and hourly limits for each account and what beta means.',
      canonical: 'https://indifferent.fun/pricing',
    });
  }
}
