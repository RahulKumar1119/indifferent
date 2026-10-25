import { ChangeDetectorRef, Component, DestroyRef, OnInit, inject } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { DOCUMENT } from '@angular/common';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { SiteFooterComponent } from '../../../shared/components/site-footer/site-footer.component';
import { ROBOTS_NOINDEX, setPageSeo, setRobots } from '../../../shared/seo';
import { ARTICLE_LOADERS } from './article-content';
import { InlineTextComponent } from './article-inline.component';
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
  imports: [RouterLink, SiteFooterComponent, InlineTextComponent],
  styles: [`
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
    .serif { font-family: 'Cormorant Garamond', 'Playfair Display', Georgia, serif; }
    .article-body { color: #3E3A33; font-size: 1.0625rem; line-height: 1.8; }
    .article-body h2 { font-size: 1.5rem; font-weight: 700; color: #1A1714; margin: 2.5rem 0 1rem; scroll-margin-top: 5rem; }
    .article-body h3 { font-size: 1.25rem; font-weight: 600; color: #1A1714; margin: 2rem 0 0.75rem; }
    .article-body p { margin: 0 0 1.25rem; }
    .article-body ul, .article-body ol { margin: 0 0 1.25rem; padding-left: 1.5rem; }
    .article-body ul { list-style: disc; }
    .article-body ol { list-style: decimal; }
    .article-body li { margin-bottom: 0.5rem; }
    .article-body code { background: rgba(26,23,20,0.06); padding: 0.1rem 0.35rem; border-radius: 0.25rem; font-size: 0.875rem; overflow-wrap: anywhere; }
    .article-body pre { background: #0F0E0B; color: #EDE8DB; padding: 1.25rem; border-radius: 12px; overflow-x: auto; margin: 0 0 1.25rem; font-size: 0.875rem; line-height: 1.6; }
    .article-body pre code { background: none; padding: 0; overflow-wrap: normal; }
    .article-body strong { color: #1A1714; font-weight: 600; }
    .article-body a { color: #A8481F; text-decoration: underline; text-underline-offset: 3px; }
    .article-body table { width: 100%; border-collapse: collapse; font-size: 0.9375rem; margin: 0 0 1.25rem; }
    .article-body th, .article-body td { text-align: left; padding: 0.6rem 0.75rem; border-bottom: 1px solid rgba(26,23,20,0.12); vertical-align: top; }
    .article-body th { font-weight: 600; color: #1A1714; }
    .table-wrap { overflow-x: auto; margin: 0 0 1.25rem; }
    .btn-primary { transition: transform .2s cubic-bezier(0.23,1,0.32,1), background-color .2s; }
    .btn-primary:hover { transform: translateY(-1px); }
    @media (prefers-reduced-motion: reduce) { .btn-primary { transition: none; } }
  `],
  template: `
    <div class="sans bg-[#FAF7F2] text-[#1A1714] antialiased min-h-[100dvh]">
      @if (meta) {
        <header class="max-w-3xl mx-auto px-4 pt-20 pb-6">
          <nav aria-label="Breadcrumb" class="text-[13px] text-[#6B6560]">
            <a routerLink="/" class="inline-flex items-center min-h-[44px] hover:text-[#1A1714]">Home</a>
            <span aria-hidden="true"> / </span>
            <a routerLink="/blog" class="inline-flex items-center min-h-[44px] hover:text-[#1A1714]">Journal</a>
          </nav>
          <p class="mt-4 text-[11.5px] uppercase tracking-[0.1em] text-[#6B6560]">{{ meta.category }}</p>
          <h1 class="serif mt-3 font-medium tracking-[-0.02em] leading-[1.05] text-[clamp(2rem,5vw,3rem)]">{{ meta.title }}</h1>
          <p class="mt-3 text-[13px] text-[#6B6560]">{{ byline }}</p>
          <p class="mt-4 text-[17px] leading-relaxed text-[#3E3A33]">{{ meta.description }}</p>
        </header>

        <main class="max-w-3xl mx-auto px-4 pb-10">
          <figure class="mb-8 overflow-hidden rounded-[16px] border border-black/10 bg-white">
            <img [src]="meta.cover" [alt]="meta.coverAlt" class="block w-full h-auto" width="1200" height="630" fetchpriority="high">
          </figure>

          @if (toc.length >= 3) {
            <nav aria-label="On this page" class="mb-8 rounded-[16px] border border-black/10 bg-white p-5">
              <p class="text-[13px] font-semibold">On this page</p>
              <ol class="mt-2 list-decimal pl-5 text-[14.5px]">
                @for (t of toc; track t.id) {
                  <li><a [href]="'#' + t.id" class="inline-flex items-center min-h-[36px] underline underline-offset-4 decoration-black/25 hover:decoration-black">{{ t.text }}</a></li>
                }
              </ol>
            </nav>
          }

          @if (loading) {
            <p class="text-[#6B6560]">Loading the article…</p>
          }
          @if (failed) {
            <p class="rounded-[14px] border border-black/10 bg-white p-5 text-[14.5px]">This article could not be loaded. Reload the page, or <a routerLink="/blog" class="underline underline-offset-4">go back to the journal</a>.</p>
          }

          <article class="article-body">
            @for (b of blocks; track $index) {
              @switch (b.type) {
                @case ('h2') { <h2 [id]="idFor(b.text)">{{ b.text }}</h2> }
                @case ('h3') { <h3>{{ b.text }}</h3> }
                @case ('p') { <p><app-inline [text]="b.text" /></p> }
                @case ('ul') { <ul>@for (item of b.items; track $index) { <li><app-inline [text]="item" /></li> }</ul> }
                @case ('ol') { <ol>@for (item of b.items; track $index) { <li><app-inline [text]="item" /></li> }</ol> }
                @case ('code') {
                  @if (b.label) { <p class="!mb-1 text-[12.5px] text-[#6B6560]">{{ b.label }}</p> }
                  <pre><code>{{ b.text }}</code></pre>
                }
                @case ('note') {
                  <aside class="mb-5 rounded-[14px] border border-[#D96C3D]/30 bg-[#D96C3D]/[0.07] p-4 text-[15px] leading-relaxed">
                    @if (b.title) { <p class="!mb-1 font-semibold text-[#1A1714]">{{ b.title }}</p> }
                    <p class="!mb-0"><app-inline [text]="b.text" /></p>
                  </aside>
                }
                @case ('table') {
                  <div class="table-wrap" tabindex="0" role="region" [attr.aria-label]="b.caption || 'Table'">
                    <table>
                      @if (b.caption) { <caption class="sr-only">{{ b.caption }}</caption> }
                      <thead><tr>@for (h of b.head; track $index) { <th scope="col">{{ h }}</th> }</tr></thead>
                      <tbody>@for (row of b.rows; track $index) { <tr>@for (cell of row; track $index) { <td>{{ cell }}</td> }</tr> }</tbody>
                    </table>
                  </div>
                }
                @case ('figure') {
                  <figure class="mb-6 overflow-hidden rounded-[14px] border border-black/10 bg-white">
                    <img [src]="b.src" [alt]="b.alt" class="block w-full h-auto" [attr.width]="b.width" [attr.height]="b.height" loading="lazy">
                    <figcaption class="px-4 py-3 text-[13.5px] text-[#6B6560]">{{ b.caption }}</figcaption>
                  </figure>
                }
              }
            }
          </article>

          @if (meta) {
            <div class="mt-10 rounded-[20px] bg-[#1A1714] text-white px-6 py-8 text-center">
              <p class="font-medium text-[19px]">{{ product.cta }}</p>
              <p class="mt-2 text-[14px] text-white/70">Free during beta. No credit card.</p>
              <a data-cta="try" [routerLink]="product.tryPath" class="btn-primary mt-4 inline-flex items-center min-h-[44px] px-7 rounded-full bg-[#BC5227] hover:bg-[#A8481F] transition-colors text-white font-semibold text-[14.5px]">{{ product.button }}</a>
              <p class="mt-3 text-[13px]"><a [routerLink]="product.path" class="inline-flex items-center min-h-[44px] underline underline-offset-4 text-white/80 hover:text-white">Steps, limits and file handling for the {{ product.label }} →</a></p>
            </div>

            @if (related.length > 0) {
              <section class="mt-10" aria-labelledby="related-heading">
                <h2 id="related-heading" class="text-[16px] font-semibold">Keep reading</h2>
                <ul class="mt-3 grid grid-cols-1 sm:grid-cols-3 gap-3">
                  @for (r of related; track r.slug) {
                    <li>
                      <a [routerLink]="path(r.slug)" class="block h-full rounded-[14px] border border-black/10 bg-white p-4 hover:border-black/30 transition-colors">
                        <span class="block text-[11.5px] uppercase tracking-[0.08em] text-[#6B6560]">{{ r.category }}</span>
                        <span class="mt-1 block text-[14.5px] font-semibold leading-snug">{{ r.title }}</span>
                      </a>
                    </li>
                  }
                </ul>
              </section>
            }
          }
        </main>
      } @else {
        <main class="max-w-3xl mx-auto px-4 pt-28 pb-16 text-center">
          <h1 class="serif text-[clamp(2rem,5vw,3rem)] font-medium">That article does not exist.</h1>
          <p class="mt-3 text-[#6B6560]">It may have moved. The journal lists everything we have published.</p>
          <a routerLink="/blog" class="mt-6 inline-flex items-center min-h-[44px] px-7 rounded-full bg-[#1A1714] text-white font-semibold text-[14.5px]">Open the journal</a>
        </main>
      }
      <app-site-footer />
    </div>
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
