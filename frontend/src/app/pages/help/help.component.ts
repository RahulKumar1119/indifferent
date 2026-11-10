import { Component, OnInit, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { RouterLink } from '@angular/router';
import { PublicHeaderComponent } from '../../shared/components/public-header/public-header.component';
import { SiteFooterComponent } from '../../shared/components/site-footer/site-footer.component';
import { setPageSeo } from '../../shared/seo';
import { FREE_BETA_FAQ } from '../../core/config/free-beta';
import { FEATURE_DATA, FeatureData, FeatureFaq } from '../features/feature-page.component';

interface HelpSection {
  id: string;
  title: string;
  intro?: string;
  faqs: FeatureFaq[];
  /** Link to the full product page, when the section is about one tool. */
  more?: { label: string; path: string };
}

/** Questions that apply to the whole site. Anything about a single tool comes
 * from FEATURE_DATA, so the help page and the product pages cannot disagree. */
export const GENERAL_FAQ: FeatureFaq[] = [
  FREE_BETA_FAQ,
  {
    q: 'Do I need an account?',
    a: 'The AI Shorts generator, the TXT to quiz video generator and the Story to video generator need you to sign in with Google. The watermark tool needs no account.',
  },
  {
    q: 'How long are my files kept?',
    a: 'Until you delete them. Uploaded source files move to archive storage after 30 days, and temporary working files are removed after 7 days. Nothing else is deleted automatically. The privacy policy has the details.',
  },
  {
    q: 'How do I delete my files?',
    a: 'Delete the project or the Short from your library. That removes its stored files and its record.',
  },
  {
    q: 'How do I delete my account?',
    a: 'Email support@indifferent.fun from the address on the account and ask for it to be deleted. There is no delete button in the app yet, and requests are handled by hand.',
  },
  {
    q: 'Which browsers work?',
    a: 'Use a current version of Chrome, Edge, Firefox or Safari.',
  },
  {
    q: 'Where do I report a problem or ask for a feature?',
    a: 'Open an issue on GitHub, or email support@indifferent.fun. The contact page lists both.',
  },
];

@Component({
  selector: 'app-help',
  standalone: true,
  imports: [RouterLink, PublicHeaderComponent, SiteFooterComponent],
  styles: [`
    :host { display: block; }
    .page { background: var(--color-paper); color: var(--color-ink); font-family: var(--font-body); font-size: var(--type-md); line-height: 1.55; -webkit-font-smoothing: antialiased; min-height: 100dvh; }
    .wrap { width: min(var(--container), 100%); margin-inline: auto; padding-inline: var(--gutter); }
    .col { width: min(52rem, 100%); }
    h1, h2, p { margin: 0; }
    h1 { font-family: var(--font-display); font-weight: 500; font-size: var(--type-display-s); line-height: 1.04; letter-spacing: var(--tracking-display); overflow-wrap: break-word; min-width: 0; text-wrap: balance; }
    h2 { font-family: var(--font-display); font-weight: 500; font-size: var(--type-2xl); line-height: 1.1; letter-spacing: var(--tracking-display); }
    .hero { padding: calc(56px + var(--space-3xl)) 0 var(--space-xl); }
    .lead { margin-top: var(--space-lg); font-size: var(--type-lead); line-height: 1.45; color: var(--color-ink-2); max-width: 52ch; }
    .jump { display: flex; flex-wrap: wrap; gap: var(--space-xs); margin-top: var(--space-xl); }
    section.faq-section { padding: var(--space-xl) 0; border-top: 1px solid var(--color-rule); scroll-margin-top: var(--space-3xl); }
    .intro { margin-top: var(--space-xs); color: var(--color-ink-2); }
    .list { display: grid; gap: var(--space-sm); margin-top: var(--space-lg); }
    .more { margin-top: var(--space-md); }
    .link { display: inline-flex; align-items: center; min-height: 44px; font-weight: 600; color: var(--color-accent-text); text-underline-offset: 0.2em; }
    .link:hover { color: var(--color-ink); }
    .link:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: 2px; }
    .stuck { background: var(--color-paper-2); border-top: 1px solid var(--color-rule); padding: var(--space-2xl) 0; }
    .stuck p { margin-top: var(--space-sm); color: var(--color-ink-2); max-width: 60ch; }
    /* In-sentence links stay inline; the padding makes the tap target 44px tall without moving the text (#256). */
    .stuck a { color: var(--color-accent-text); text-decoration: underline; text-underline-offset: 0.2em; padding-block: 13px; }
    .stuck a:hover { color: var(--color-ink); }
    .stuck a:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: 2px; }
  `],
  template: `
    <div class="page hm hm-on-light">
      <app-public-header />
      <header class="hero">
        <div class="wrap"><div class="col">
          <h1>Help and FAQ</h1>
          <p class="lead">
            Answers about the four tools, your files and your account. Each tool also has a page with its exact limits and a step-by-step guide.
          </p>
          <nav aria-label="Sections" class="jump">
            @for (s of sections; track s.id) {
              <a [href]="'#' + s.id" class="ui-btn ui-btn--secondary">{{ s.title }}</a>
            }
          </nav>
        </div></div>
      </header>
      <main>
        @for (s of sections; track s.id) {
          <section [id]="s.id" class="faq-section" [attr.aria-labelledby]="s.id + '-heading'">
            <div class="wrap"><div class="col">
              <h2 [id]="s.id + '-heading'">{{ s.title }}</h2>
              @if (s.intro) {
                <p class="intro">{{ s.intro }}</p>
              }
              <div class="list">
                @for (f of s.faqs; track f.q) {
                  <details class="ui-disclosure">
                    <summary>{{ f.q }}</summary>
                    <p>{{ f.a }}</p>
                    @if (f.link) {
                      <p><a [routerLink]="f.link.path" class="link">{{ f.link.label }}</a></p>
                    }
                  </details>
                }
              </div>
              @if (s.more) {
                <p class="more"><a [routerLink]="s.more.path" class="link">{{ s.more.label }}</a></p>
              }
            </div></div>
          </section>
        }
        <section class="stuck" aria-labelledby="contact-heading">
          <div class="wrap"><div class="col">
            <h2 id="contact-heading">Still stuck?</h2>
            <p>
              Email <a href="mailto:support@indifferent.fun">support&#64;indifferent.fun</a> with the file type, its length and what the page showed. The <a routerLink="/contact">contact page</a> lists the other ways to reach us.
            </p>
          </div></div>
        </section>
      </main>
      <app-site-footer />
    </div>
  `,
})
export class HelpComponent implements OnInit {
  private readonly document = inject(DOCUMENT);

  readonly sections: HelpSection[] = [
    { id: 'general', title: 'Accounts, files and pricing', faqs: GENERAL_FAQ },
    ...(['ai-shorts', 'txt-to-video-quiz', 'watermark'] as const).map((slug) => this.toolSection(FEATURE_DATA[slug])),
  ];

  ngOnInit(): void {
    setPageSeo(this.document, {
      title: 'Help and FAQ | Indifferent',
      description:
        'Answers about the AI Shorts generator, the TXT to quiz video generator, the Story to video generator and the watermark tool: limits, file formats, fixes, privacy and your account.',
      canonical: 'https://indifferent.fun/help',
    });
  }

  private toolSection(f: FeatureData): HelpSection {
    return {
      id: f.slug,
      title: f.title,
      intro: f.hook,
      faqs: f.troubleshooting,
      more: { label: `${f.title}: limits and step-by-step guide`, path: `/features/${f.slug}` },
    };
  }
}
