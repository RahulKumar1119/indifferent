import { ChangeDetectorRef, Component, DestroyRef, OnInit, inject } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { DOCUMENT } from '@angular/common';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { PublicHeaderComponent } from '../../../shared/components/public-header/public-header.component';
import { SiteFooterComponent } from '../../../shared/components/site-footer/site-footer.component';
import { ROBOTS_NOINDEX, setPageSeo, setRobots } from '../../../shared/seo';
import { ARTICLE_LOADERS } from './article-content';
import { InlineTextComponent } from './article-inline.component';
import { ArticleShellComponent } from './article-shell.component';
import { slugifyHeading } from './article-inline';
import { setArticleSeo } from './article-seo';
import { ArticleBlock } from './article-types';
import {
  ArticleMeta,
  PRODUCT_PAGES,
  articleByline,
  articlePath,
  findArticle,
  relatedArticles,
} from './registry';

interface TocEntry {
  id: string;
  text: string;
}

/** Renders any data-driven journal article from the registry and its content
 * module. The older articles keep their own components but share the SEO,
 * byline and footer helpers. */
@Component({
  selector: 'app-article-page',
  standalone: true,
  imports: [RouterLink, PublicHeaderComponent, SiteFooterComponent, InlineTextComponent, ArticleShellComponent],
  styles: [`
    :host { display: block; }
    .page { background: var(--color-paper); color: var(--color-ink); font-family: var(--font-body); line-height: 1.55; min-height: 100dvh; }
    .wrap { width: min(var(--container), 100%); margin-inline: auto; padding-inline: var(--gutter); }
    .missing { padding: calc(56px + var(--space-3xl)) 0 var(--space-3xl); }
    .missing h1 { margin: 0; font-family: var(--font-display); font-weight: 500; font-size: var(--type-display-s); line-height: 1.04; letter-spacing: var(--tracking-display); text-wrap: balance; }
    .missing p { margin: var(--space-md) 0 var(--space-xl); color: var(--color-ink-2); max-width: 52ch; }
    figure.cover-fig { margin: 0; }
    figure.cover-fig img { display: block; width: 100%; height: auto; border: 1px solid var(--color-rule); border-radius: var(--radius-surface); background: var(--color-paper-2); }
    nav.toc-box { padding: var(--space-lg); background: var(--color-paper-2); border: 1px solid var(--color-rule); border-radius: var(--radius-surface); }
    nav.toc-box p { margin: 0; font-size: var(--type-sm); font-weight: 600; }
    nav.toc-box ol { margin: var(--space-xs) 0 0; padding-left: var(--space-lg); list-style: decimal; }
    nav.toc-box a { display: inline-flex; align-items: center; min-height: 36px; color: var(--color-ink-2); text-decoration: underline; text-underline-offset: 0.2em; }
    nav.toc-box a:hover { color: var(--color-accent-text); }
    nav.toc-box a:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: 2px; }
    .state { margin: 0 0 1.25rem; color: var(--color-muted); }
  `],
  template: `
    @if (meta) {
      <app-article-shell [meta]="meta">
        <figure articleCover class="cover-fig">
          <img [src]="meta.cover" [alt]="meta.coverAlt" width="1200" height="630" fetchpriority="high">
        </figure>
        @if (toc.length >= 3) {
          <nav articleToc class="toc-box" aria-label="On this page">
            <p>On this page</p>
            <ol>
              @for (t of toc; track t.id) {
                <li><a [href]="'#' + t.id">{{ t.text }}</a></li>
              }
            </ol>
          </nav>
        }
        @if (loading) {
          <p class="state">Loading the article…</p>
        }
        @if (failed) {
          <p class="ui-note">This article could not be loaded. Reload the page, or <a routerLink="/blog">go back to the journal</a>.</p>
        }
        @for (b of blocks; track $index) {
          @switch (b.type) {
            @case ('h2') { <h2 [id]="idFor(b.text)">{{ b.text }}</h2> }
            @case ('h3') { <h3>{{ b.text }}</h3> }
            @case ('p') { <p><app-inline [text]="b.text" /></p> }
            @case ('ul') { <ul>@for (item of b.items; track $index) { <li><app-inline [text]="item" /></li> }</ul> }
            @case ('ol') { <ol>@for (item of b.items; track $index) { <li><app-inline [text]="item" /></li> }</ol> }
            @case ('code') {
              @if (b.label) { <p class="code-label">{{ b.label }}</p> }
              <pre><code>{{ b.text }}</code></pre>
            }
            @case ('note') {
              <aside class="ui-note">
                @if (b.title) { <p class="note-title">{{ b.title }}</p> }
                <p><app-inline [text]="b.text" /></p>
              </aside>
            }
            @case ('table') {
              <div class="table-wrap" tabindex="0" role="region" [attr.aria-label]="b.caption || 'Table'">
                <table>
                  @if (b.caption) { <caption class="sr-only">{{ b.caption }}</caption> }
                  <thead><tr>@for (h of b.head; track $index) { <th scope="col">{{ h }}</th> }</tr></thead>
                  <tbody>@for (row of b.rows; track $index) { <tr>@for (cell of row; track $index) { @if ($first) { <th scope="row">{{ cell }}</th> } @else { <td>{{ cell }}</td> } }</tr> }</tbody>
                </table>
              </div>
            }
            @case ('figure') {
              <figure>
                <img [src]="b.src" [alt]="b.alt" [attr.width]="b.width" [attr.height]="b.height" loading="lazy">
                <figcaption>{{ b.caption }}</figcaption>
              </figure>
            }
          }
        }
      </app-article-shell>
    } @else {
      <div class="page hm hm-on-light">
        <app-public-header />
        <main class="missing">
          <div class="wrap">
            <h1>That article does not exist.</h1>
            <p>It may have moved. The journal lists everything we have published.</p>
            <a routerLink="/blog" class="ui-btn ui-btn--primary">Open the journal</a>
          </div>
        </main>
        <app-site-footer />
      </div>
    }
  `,
})
export class ArticlePageComponent implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly document = inject(DOCUMENT);
  private readonly cdr = inject(ChangeDetectorRef);
  private readonly destroyRef = inject(DestroyRef);

  meta: ArticleMeta | undefined;
  byline = '';
  blocks: ArticleBlock[] = [];
  toc: TocEntry[] = [];
  related: ArticleMeta[] = [];
  loading = false;
  failed = false;

  get product() {
    return PRODUCT_PAGES[this.meta!.product];
  }

  ngOnInit(): void {
    // The router keeps this component alive when only :slug changes, which is what
    // happens when a reader follows a "Keep reading" link. Follow the param map so
    // the next article replaces the current one instead of the URL changing alone.
    this.route.paramMap
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((params) => this.show(params.get('slug') ?? ''));
  }

  private show(slug: string): void {
    this.meta = undefined;
    this.blocks = [];
    this.toc = [];
    this.related = [];
    this.loading = false;
    this.failed = false;
    const meta = findArticle(slug);
    const loader = ARTICLE_LOADERS[slug];
    if (!meta || !loader) {
      // Unknown slug: the app answers 200 for every path, so say so with noindex.
      setPageSeo(this.document, {
        title: 'Article not found | Indifferent',
        description: 'This article does not exist. Browse the Indifferent journal for guides on AI Shorts, quiz videos and watermarking.',
        canonical: 'https://indifferent.fun/blog',
      });
      setRobots(this.document, ROBOTS_NOINDEX);
      return;
    }
    this.meta = meta;
    this.byline = articleByline(meta);
    this.related = relatedArticles(slug, 3);
    setArticleSeo(this.document, meta);
    this.loading = true;
    loader()
      .then((mod) => {
        // A slow import for an article the reader already left must not overwrite the current one.
        if (this.meta !== meta) return;
        this.blocks = mod.blocks;
        this.toc = mod.blocks
          .filter((b): b is Extract<ArticleBlock, { type: 'h2' }> => b.type === 'h2')
          .map((b) => ({ id: slugifyHeading(b.text), text: b.text }));
      })
      .catch(() => {
        if (this.meta === meta) this.failed = true;
      })
      .finally(() => {
        if (this.meta === meta) this.loading = false;
        this.cdr.markForCheck();
      });
  }

  idFor(text: string): string {
    return slugifyHeading(text);
  }

  path(slug: string): string {
    return articlePath(slug);
  }
}
