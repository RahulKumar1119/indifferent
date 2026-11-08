import { Component, OnInit, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { RouterLink } from '@angular/router';
import { PublicHeaderComponent } from '../../shared/components/public-header/public-header.component';
import { SiteFooterComponent } from '../../shared/components/site-footer/site-footer.component';
import { RevealDirective } from '../../shared/directives/reveal.directive';
import { setPageSeo } from '../../shared/seo';
import { ARTICLES, ARTICLE_CATEGORIES, articlePath, formatShortDate } from './articles/registry';

interface Post {
  route: string;
  title: string;
  excerpt: string;
  date: string;
  read: string;
  category: string;
  cover: string;
  coverAlt: string;
}

@Component({
  selector: 'app-blog',
  standalone: true,
  imports: [RouterLink, PublicHeaderComponent, SiteFooterComponent, RevealDirective],
  styles: [`
    :host { display: block; }
    .page { background: var(--color-paper); color: var(--color-ink); font-family: var(--font-body); font-size: var(--type-md); line-height: 1.55; -webkit-font-smoothing: antialiased; }
    .wrap { width: min(var(--container), 100%); margin-inline: auto; padding-inline: var(--gutter); }
    h1, h2, h3, p, ul { margin: 0; }
    h1 { font-family: var(--font-display); font-weight: 500; font-size: var(--type-display); line-height: 1.02; letter-spacing: var(--tracking-display); text-wrap: balance; }
    h2, h3 { font-family: var(--font-display); font-weight: 500; letter-spacing: var(--tracking-display); line-height: 1.1; text-wrap: balance; }
    .lead { margin-top: var(--space-lg); font-size: var(--type-lead); line-height: 1.45; color: var(--color-ink-2); max-width: 46ch; }
    .muted { color: var(--color-muted); }

    .mast { padding: calc(56px + var(--space-3xl)) 0 var(--space-xl); }
    .mast-grid { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-xl); align-items: end; }
    @media (min-width: 64rem) { .mast-grid { grid-template-columns: minmax(0, 1.1fr) minmax(0, 0.9fr); } }
    .controls { display: grid; gap: var(--space-md); justify-self: end; width: min(26rem, 100%); }
    .filters { display: flex; flex-wrap: wrap; gap: var(--space-xs); }
    .chip[aria-pressed='true'] { background: var(--color-ink); color: var(--color-paper); border-color: var(--color-ink); }

    main { padding: var(--space-xl) 0 var(--space-3xl); border-top: 1px solid var(--color-rule); }
    .featured { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-xl); align-items: center; padding: var(--space-lg) 0 var(--space-2xl); color: inherit; text-decoration: none; border-bottom: 1px solid var(--color-rule); }
    @media (min-width: 64rem) { .featured { grid-template-columns: minmax(0, 1.1fr) minmax(0, 0.9fr); gap: var(--space-2xl); } }
    .featured h2 { margin-top: var(--space-sm); font-size: var(--type-section); }
    .featured .excerpt { margin-top: var(--space-md); color: var(--color-ink-2); max-width: 52ch; }
    .cover { display: block; width: 100%; height: auto; aspect-ratio: 1200 / 630; object-fit: cover; background: var(--color-paper-3); border: 1px solid var(--color-rule); border-radius: var(--radius-surface); }
    .meta { font-size: var(--type-sm); color: var(--color-muted); }
    .more { display: inline-flex; align-items: center; min-height: 44px; margin-top: var(--space-md); font-weight: 600; color: var(--color-accent-text); text-decoration: underline; text-underline-offset: 0.2em; }
    a.entry:hover .more { color: var(--color-ink); }
    a.entry:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 4px; border-radius: var(--radius-surface); }

    .grid { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-2xl) var(--space-xl); margin-top: var(--space-2xl); }
    @media (min-width: 48rem) { .grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
    @media (min-width: 64rem) { .grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
    a.item { display: flex; flex-direction: column; gap: var(--space-sm); color: inherit; text-decoration: none; height: 100%; }
    .item h3 { font-size: var(--type-xl); }
    .item .excerpt { color: var(--color-ink-2); flex: 1; }

    .empty { padding: var(--space-3xl) 0; text-align: center; }
    .empty h2 { font-size: var(--type-section); }
    .empty p { margin: var(--space-sm) 0 var(--space-lg); color: var(--color-muted); }

    .strip { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-lg); margin-top: var(--space-3xl); padding: var(--space-xl); background: var(--color-paper-2); border: 1px solid var(--color-rule); border-radius: var(--radius-surface); }
    .strip h2 { margin-top: var(--space-2xs); font-size: var(--type-2xl); }
    .tools { list-style: none; padding: 0; display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-lg); margin-top: var(--space-2xl); padding-top: var(--space-lg); border-top: 1px solid var(--color-rule); }
    @media (min-width: 48rem) { .tools { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
    .tools a { display: block; color: inherit; text-decoration: none; }
    .tools strong { display: block; font-weight: 600; text-decoration: underline; text-underline-offset: 0.2em; }
    .tools a:hover strong { color: var(--color-accent-text); }
    .tools a:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 4px; border-radius: 2px; }
    .tools span { display: block; margin-top: var(--space-3xs); font-size: var(--type-sm); color: var(--color-muted); }

    .follow { padding: var(--space-3xl) 0; background: var(--color-paper-2); border-top: 1px solid var(--color-rule); }
    .follow h2 { font-size: var(--type-section); }
    .follow p { margin-top: var(--space-md); color: var(--color-ink-2); max-width: 52ch; }
    .actions { display: flex; flex-wrap: wrap; gap: var(--space-md); margin-top: var(--space-xl); }
  `],
  template: `
    <div class="page hm hm-on-light">
      <app-public-header />

      <header class="mast">
        <div class="wrap mast-grid">
          <div>
            <h1>Notes on video.</h1>
            <p class="lead">
              Guides to the AI Shorts generator, quiz videos and the watermark tool: how they work, their limits and how to fix problems.
            </p>
          </div>
          <div class="controls">
            <label for="journal-search" class="sr-only">Search essays</label>
            <input id="journal-search" type="search" placeholder="Search essays…" class="ui-field" (input)="onSearch($event)">
            <div class="filters" role="group" aria-label="Filter by category">
              @for (c of categories; track c) {
                <button type="button" class="ui-btn ui-btn--secondary chip" [attr.aria-pressed]="activeCategory === c" (click)="setCategory(c)">{{ c }}</button>
              }
            </div>
          </div>
        </div>
      </header>

      <main>
        <div class="wrap">
          @if (filteredPosts.length === 0) {
            <div class="empty">
              <h2>No essays found.</h2>
              <p>Try a different search or category.</p>
              <button type="button" class="ui-btn ui-btn--primary" (click)="resetFilters()">Clear filters</button>
            </div>
          } @else {
            @if (showFeatured()) {
              <a data-article [routerLink]="filteredPosts[0].route" class="entry featured">
                <img [src]="filteredPosts[0].cover" [alt]="filteredPosts[0].coverAlt" class="cover" width="1200" height="630" fetchpriority="high">
                <div>
                  <p class="meta">{{ filteredPosts[0].date }} · {{ filteredPosts[0].read }}</p>
                  <h2>{{ filteredPosts[0].title }}</h2>
                  <p class="excerpt">{{ filteredPosts[0].excerpt }}</p>
                  <span class="more">Read essay</span>
                </div>
              </a>
            }
            <div class="grid">
              @for (p of gridPosts(); track p.route) {
                <a data-article [routerLink]="p.route" class="entry item reveal" appReveal>
                  <img [src]="p.cover" [alt]="p.coverAlt" class="cover" width="1200" height="630" loading="lazy">
                  <p class="meta">{{ p.date }} · {{ p.read }}</p>
                  <h3>{{ p.title }}</h3>
                  <p class="excerpt">{{ p.excerpt }}</p>
                  <span class="more">Read essay</span>
                </a>
              }
            </div>
          }

          <div class="strip">
            <div>
              <p class="meta">New to the format?</p>
              <h2>Format your TXT file once. Render forever.</h2>
            </div>
            <a routerLink="/blog/quiz-file-format-guide" class="ui-btn ui-btn--secondary">Open format guide</a>
          </div>

          <ul class="tools">
            <li><a routerLink="/features/ai-shorts"><strong>AI Shorts generator</strong><span>Long video or audio to up to 3 captioned 9:16 clips.</span></a></li>
            <li><a routerLink="/features/txt-to-video-quiz"><strong>TXT to quiz video</strong><span>Questions to narrated MP4.</span></a></li>
            <li><a routerLink="/features/watermark"><strong>Watermark tool</strong><span>Text or logo marks for images and video, in your browser.</span></a></li>
          </ul>
        </div>
      </main>

      <section class="follow" aria-labelledby="follow-heading">
        <div class="wrap">
          <h2 id="follow-heading">Follow the journal.</h2>
          <p>New articles appear on this page first. To get them in a feed reader, add the RSS feed. There is no email list.</p>
          <div class="actions">
            <a href="/feed.xml" class="ui-btn ui-btn--primary">RSS feed</a>
            <a routerLink="/contact" class="ui-btn ui-btn--secondary">Suggest a topic</a>
          </div>
        </div>
      </section>
      <app-site-footer />
    </div>
  `,
})
export class BlogComponent implements OnInit {
  private readonly document = inject(DOCUMENT);

  ngOnInit(): void {
    setPageSeo(this.document, {
      title: 'Journal: Guides & Notes | Indifferent',
      description:
        'Guides from the Indifferent studio on AI Shorts, quiz videos and watermarking: how the tools work, their limits, file formats and fixes.',
      canonical: 'https://indifferent.fun/blog',
    });
  }

  currentYear = new Date().getFullYear();

  categories: string[] = ['All', ...ARTICLE_CATEGORIES];

  activeCategory = 'All';
  query = '';

  posts: Post[] = ARTICLES.map((a) => ({
    route: articlePath(a.slug),
    title: a.title,
    excerpt: a.description,
    date: formatShortDate(a.published),
    read: `${a.readMinutes} min read`,
    category: a.category,
    cover: a.cover,
    coverAlt: a.coverAlt,
  }));

  get filteredPosts(): Post[] {
    const q = this.query.trim().toLowerCase();
    return this.posts.filter((p) => {
      const inCategory = this.activeCategory === 'All' || p.category === this.activeCategory;
      const inQuery = !q || (p.title + ' ' + p.excerpt).toLowerCase().includes(q);
      return inCategory && inQuery;
    });
  }

  showFeatured(): boolean {
    return this.activeCategory === 'All' && !this.query.trim() && this.filteredPosts.length > 0;
  }

  gridPosts(): Post[] {
    return this.showFeatured() ? this.filteredPosts.slice(1) : this.filteredPosts;
  }

  setCategory(c: string): void {
    this.activeCategory = c;
  }

  onSearch(e: Event): void {
    this.query = (e.target as HTMLInputElement).value;
  }

  resetFilters(): void {
    this.activeCategory = 'All';
    this.query = '';
    const input = document.getElementById('journal-search') as HTMLInputElement | null;
    if (input) input.value = '';
  }
}
