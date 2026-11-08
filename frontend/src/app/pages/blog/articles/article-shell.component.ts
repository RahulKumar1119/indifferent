import { Component, computed, input } from '@angular/core';
import { RouterLink } from '@angular/router';
import { PublicHeaderComponent } from '../../../shared/components/public-header/public-header.component';
import { SiteFooterComponent } from '../../../shared/components/site-footer/site-footer.component';
import { ArticleClosingComponent } from './article-closing.component';
import { ArticleMeta, articleByline } from './registry';

/**
 * The frame every journal article sits in: the public header, a breadcrumb, the
 * title block, the article body on a reading measure, the shared ending and the
 * footer. The body is projected, so the data-driven page and the four older
 * hand-written articles share one layout and their text stays where it is.
 *
 * Slots: `[articleCover]` and `[articleToc]` go between the title block and the
 * body; anything else is the body.
 */
@Component({
  selector: 'app-article-shell',
  standalone: true,
  imports: [RouterLink, PublicHeaderComponent, SiteFooterComponent, ArticleClosingComponent],
  styles: [`
    :host { display: block; }
    .page { background: var(--color-paper); color: var(--color-ink); font-family: var(--font-body); font-size: var(--type-md); line-height: 1.55; -webkit-font-smoothing: antialiased; min-height: 100dvh; }
    .wrap { width: min(var(--container), 100%); margin-inline: auto; padding-inline: var(--gutter); }
    .col { width: min(46rem, 100%); }
    header.head { padding: calc(56px + var(--space-2xl)) 0 var(--space-lg); }
    nav.crumbs { font-size: var(--type-sm); color: var(--color-muted); }
    nav.crumbs a { display: inline-flex; align-items: center; min-height: 44px; color: inherit; text-underline-offset: 0.2em; }
    nav.crumbs a:hover { color: var(--color-ink); text-decoration: underline; }
    nav.crumbs a:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: 2px; }
    .category { margin: var(--space-md) 0 0; font-size: var(--type-sm); font-weight: 500; color: var(--color-accent-text); }
    h1 { margin: var(--space-xs) 0 0; font-family: var(--font-display); font-weight: 500; font-size: var(--type-display-s); line-height: 1.04; letter-spacing: var(--tracking-display); overflow-wrap: break-word; min-width: 0; text-wrap: balance; }
    .byline { margin: var(--space-md) 0 0; font-size: var(--type-sm); color: var(--color-muted); }
    .dek { margin: var(--space-md) 0 0; font-size: var(--type-lead); line-height: 1.5; color: var(--color-ink-2); }
    .body { padding-bottom: var(--space-3xl); }
    .cover, .toc { margin-bottom: var(--space-xl); }
  `],
  template: `
    <div class="page hm hm-on-light">
      <app-public-header />
      <main>
        <article>
          <header class="head">
            <div class="wrap"><div class="col">
              <nav class="crumbs" aria-label="Breadcrumb">
                <a routerLink="/">Home</a>
                <span aria-hidden="true"> / </span>
                <a routerLink="/blog">Journal</a>
              </nav>
              <p class="category">{{ meta().category }}</p>
              <h1>{{ meta().title }}</h1>
              <p class="byline">{{ byline() }}</p>
              <p class="dek">{{ meta().description }}</p>
            </div></div>
          </header>
          <div class="body">
            <div class="wrap"><div class="col">
              <div class="cover"><ng-content select="[articleCover]" /></div>
              <div class="toc"><ng-content select="[articleToc]" /></div>
              <div class="ui-article"><ng-content /></div>
            </div></div>
          </div>
        </article>
      </main>
      <app-article-closing [slug]="meta().slug" />
      <app-site-footer />
    </div>
  `,
})
export class ArticleShellComponent {
  readonly meta = input.required<ArticleMeta>();
  protected readonly byline = computed(() => articleByline(this.meta()));
}
