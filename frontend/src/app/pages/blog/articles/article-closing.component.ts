import { Component, computed, input } from '@angular/core';
import { RouterLink } from '@angular/router';
import { PRODUCT_PAGES, articlePath, findArticle, relatedArticles } from './registry';

/**
 * How every journal article ends: a path into the tool the article is about
 * (named in articles.json), then up to three related articles. Shared by the
 * data-driven page and the four older hand-written articles.
 */
@Component({
  selector: 'app-article-closing',
  standalone: true,
  imports: [RouterLink],
  styles: [`
    :host { display: block; }
    .wrap { width: min(var(--container), 100%); margin-inline: auto; padding-inline: var(--gutter); }
    .cta { background: var(--color-paper-2); border-top: 1px solid var(--color-rule); padding: var(--space-2xl) 0; }
    .cta .wrap { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-lg); }
    .cta p { margin: 0; }
    .cta .lead { font-family: var(--font-display); font-size: var(--type-xl); font-weight: 500; line-height: 1.2; max-width: 28ch; }
    .cta .fine { margin-top: var(--space-xs); font-size: var(--type-sm); color: var(--color-muted); }
    .cta .more { display: inline-flex; align-items: center; min-height: 44px; margin-top: var(--space-xs); font-size: var(--type-sm); color: var(--color-accent-text); text-decoration: underline; text-underline-offset: 0.2em; }
    .cta .more:hover { color: var(--color-ink); }
    .cta .more:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: 2px; }
    .related { padding: var(--space-2xl) 0; border-top: 1px solid var(--color-rule); }
    h2 { margin: 0; font-family: var(--font-display); font-weight: 500; font-size: var(--type-2xl); line-height: 1.1; letter-spacing: var(--tracking-display); }
    ul { list-style: none; margin: var(--space-lg) 0 0; padding: 0; display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-md); }
    @media (min-width: 48rem) { ul { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
    a.card { display: block; height: 100%; padding: var(--space-md) var(--space-lg); background: var(--color-paper-2); border: 1px solid var(--color-rule); border-radius: var(--radius-surface); color: inherit; text-decoration: none; transition: border-color var(--dur-short) var(--ease-out); }
    a.card:hover { border-color: var(--color-muted); }
    a.card:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; }
    .card small { display: block; font-size: var(--type-sm); color: var(--color-muted); }
    .card strong { display: block; margin-top: var(--space-3xs); font-weight: 600; line-height: 1.35; }
    @media (prefers-reduced-motion: reduce) { a.card { transition: none; } }
  `],
  template: `
    @if (product(); as p) {
      <section class="cta hm hm-on-light" aria-label="Try the tool">
        <div class="wrap">
          <div>
            <p class="lead">{{ p.cta }}</p>
            <p class="fine">Free during beta. No credit card.</p>
            <a [routerLink]="p.path" class="more">Steps, limits and file handling for the {{ p.label }}</a>
          </div>
          <a data-cta="try" [routerLink]="p.tryPath" class="ui-btn ui-btn--primary">{{ p.button }}</a>
        </div>
      </section>
    }
    @if (related().length > 0) {
      <section class="related hm hm-on-light" aria-labelledby="related-heading">
        <div class="wrap">
          <h2 id="related-heading">Keep reading</h2>
          <ul>
            @for (r of related(); track r.slug) {
              <li>
                <a [routerLink]="path(r.slug)" class="card">
                  <small>{{ r.category }}</small>
                  <strong>{{ r.title }}</strong>
                </a>
              </li>
            }
          </ul>
        </div>
      </section>
    }
  `,
})
export class ArticleClosingComponent {
  readonly slug = input.required<string>();
  protected readonly product = computed(() => {
    const meta = findArticle(this.slug());
    return meta ? PRODUCT_PAGES[meta.product] : undefined;
  });
  protected readonly related = computed(() => relatedArticles(this.slug(), 3));
  protected path(slug: string): string {
    return articlePath(slug);
  }
}
