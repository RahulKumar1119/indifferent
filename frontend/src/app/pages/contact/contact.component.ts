import { Component, OnInit, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { PublicHeaderComponent } from '../../shared/components/public-header/public-header.component';
import { SiteFooterComponent } from '../../shared/components/site-footer/site-footer.component';
import { setPageSeo } from '../../shared/seo';

@Component({
  selector: 'app-contact',
  standalone: true,
  imports: [PublicHeaderComponent, SiteFooterComponent],
  styles: [`
    :host { display: block; }
    .page { background: var(--color-paper); color: var(--color-ink); font-family: var(--font-body); font-size: var(--type-md); line-height: 1.55; -webkit-font-smoothing: antialiased; }
    .wrap { width: min(var(--container), 100%); margin-inline: auto; padding-inline: var(--gutter); }
    h1, h2, p { margin: 0; }
    h1 { font-family: var(--font-display); font-weight: 500; font-size: var(--type-display); line-height: 1.02; letter-spacing: var(--tracking-display); overflow-wrap: break-word; min-width: 0; text-wrap: balance; max-width: 12ch; }
    h2 { font-family: var(--font-display); font-weight: 500; font-size: var(--type-2xl); line-height: 1.1; letter-spacing: var(--tracking-display); }
    .lead { margin-top: var(--space-lg); font-size: var(--type-lead); line-height: 1.45; color: var(--color-ink-2); max-width: 48ch; }
    .hero { background: var(--color-paper); color: var(--color-ink); padding: calc(56px + var(--space-3xl)) 0 var(--space-3xl); }
    .grid { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-2xl); padding: var(--space-3xl) 0; }
    @media (min-width: 64rem) { .grid { grid-template-columns: minmax(0, 3fr) minmax(0, 2fr); } }
    .label { font-size: var(--type-sm); color: var(--color-muted); }
    .item { padding-top: var(--space-lg); border-top: 1px solid var(--color-rule); }
    .item h2 { margin-top: var(--space-2xs); }
    .item p.body { margin-top: var(--space-sm); color: var(--color-ink-2); max-width: 52ch; }
    .side { display: grid; gap: var(--space-xl); align-content: start; }
    .mail { display: inline-flex; align-items: center; min-height: 44px; margin-top: var(--space-sm); font-size: var(--type-lg); font-weight: 600; color: var(--color-accent-text); text-underline-offset: 0.2em; }
    .mail:hover { color: var(--color-ink); }
    .mail:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: 2px; }
    .actions { margin-top: var(--space-lg); }
  `],
  template: `
    <div class="page hm hm-on-light">
      <app-public-header />
      <section class="hero hm-on-dark" aria-labelledby="contact-title">
        <div class="wrap">
          <h1 id="contact-title">Write to the studio.</h1>
          <p class="lead">Questions, feedback, or feature requests. Reach out through any channel below.</p>
        </div>
      </section>
      <main>
        <div class="wrap grid">
          <section class="item" aria-labelledby="issues-heading">
            <p class="label">Bugs, features and questions</p>
            <h2 id="issues-heading">GitHub issues</h2>
            <p class="body">The fastest route. Report a bug, request a feature, or ask a question where the work happens.</p>
            <div class="actions">
              <a href="https://github.com/RahulKumar1119/indifferent/issues" target="_blank" rel="noopener" class="ui-btn ui-btn--primary">Open an issue</a>
            </div>
          </section>
          <div class="side">
            <section class="item" aria-labelledby="email-heading">
              <p class="label">Email</p>
              <h2 id="email-heading">General inquiries</h2>
              <a href="mailto:support@indifferent.fun" class="mail">support&#64;indifferent.fun</a>
            </section>
            <section class="item" aria-labelledby="time-heading">
              <p class="label">Response time</p>
              <h2 id="time-heading">Within 24–48 hours.</h2>
              <p class="body">Usually faster on weekdays.</p>
            </section>
          </div>
        </div>
      </main>
      <app-site-footer />
    </div>
  `,
})
export class ContactComponent implements OnInit {
  private readonly document = inject(DOCUMENT);

  ngOnInit(): void {
    setPageSeo(this.document, {
      title: 'Contact | Indifferent',
      description:
        'Get in touch with the Indifferent studio: support, feedback, and feature requests for the AI video platform.',
      canonical: 'https://indifferent.fun/contact',
    });
  }

  currentYear = new Date().getFullYear();
}
