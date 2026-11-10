import { Component, DestroyRef, OnInit, inject } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { CommonModule, DOCUMENT } from '@angular/common';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { PublicHeaderComponent } from '../../shared/components/public-header/public-header.component';
import { SiteFooterComponent } from '../../shared/components/site-footer/site-footer.component';
import { ROBOTS_NOINDEX, setPageSeo, setRobots } from '../../shared/seo';
export interface UseCaseData {
  slug: string;
  title: string;
  hook: string;
  description: string;
  benefits: string[];
  cta: string;
}

/** Shared template for all persona SEO pages; content comes from route data. */
export const USE_CASE_DATA: Record<string, UseCaseData> = {
  'content-creators': {
    slug: 'content-creators',
    title: 'AI Shorts Maker for Content Creators',
    hook: 'Repurpose podcasts, interviews and videos into ranked vertical shorts.',
    description: 'Upload a long video or audio file and get up to 3 ranked 9:16 clips with burned-in captions, ready for Shorts, Reels and TikTok.',
    benefits: [
      'A language model picks up to three moments from the transcript, so you skip the timeline scrubbing',
      'Face detection frames the speaker, and blur-fill keeps on-screen text whole',
      'Short caption lines styled for silent viewing',
      'Full-HD 1080×1920 MP4 exports, free during beta',
    ],
    cta: 'Turn your next video into shorts',
  },
  'marketing-teams': {
    slug: 'marketing-teams',
    title: 'AI Shorts Maker for Marketing Teams',
    hook: 'Turn webinars, launches and interviews into vertical clips.',
    description: 'Turn webinars, launches and interviews into a pipeline of short-form content without adding editing headcount.',
    benefits: [
      'One upload becomes three ranked, ready-to-post clips',
      'Consistent captioned style across every video',
      'Vertical output: 1080×1920 MP4, H.264 + AAC, 30 fps',
      'No editing software to learn or license',
    ],
    cta: 'Scale your short-form output',
  },
  agencies: {
    slug: 'agencies',
    title: 'AI Shorts Maker for Agencies',
    hook: 'Produce on-brand client content with consistent quality, fast.',
    description: 'Repurpose client long-form into platform-ready shorts with predictable turnaround and consistent caption styling.',
    benefits: [
      'Ranked clips give the team a starting point when choosing moments',
      'Same caption and framing treatment on every deliverable',
      'Blur-fill fit keeps client graphics and text intact',
      'Downloadable MP4s you can upload to any scheduling tool',
    ],
    cta: 'Speed up client delivery',
  },
  coaches: {
    slug: 'coaches',
    title: 'AI Shorts Maker for Coaches',
    hook: 'Turn long lessons and calls into short clips.',
    description: 'Extract the key insight from every coaching call, lesson or live session and publish it as a captioned short.',
    benefits: [
      'Moments ranked by a language model that reads the transcript',
      'Face detection places the vertical window on you',
      'Burned-in captions, so clips read with the sound off',
      'No editing skills or software needed',
    ],
    cta: 'Clip your next lesson',
  },
  'media-companies': {
    slug: 'media-companies',
    title: 'AI Shorts Maker for Media Companies',
    hook: 'Turn long-form content into captioned vertical clips.',
    description: 'Convert shows, interviews and coverage into vertical clips, each a 1080×1920 MP4 with captions burned in.',
    benefits: [
      'Every upload returns up to three ranked clips',
      'With several people in frame the largest face sets the framing, so check each preview',
      'Blur-fill preserves graphics, lower-thirds and diagrams',
      'Standard H.264 + AAC MP4 exports',
    ],
    cta: 'Repurpose your archive',
  },
  educators: {
    slug: 'educators',
    title: 'AI Shorts Maker for Educators',
    hook: 'Clip the key explanation from every lecture, automatically.',
    description: 'Upload a lecture recording and get the most engaging explanations back as captioned vertical clips for revision and reach.',
    benefits: [
      'Ranking picks self-contained explanations from the transcript',
      'Short caption lines aid comprehension and retention',
      'Diagrams and slides stay fully visible with blur-fill fit',
      'MP3 and WAV lecture audio supported too',
    ],
    cta: 'Clip your next lecture',
  },
};

@Component({
  selector: 'app-use-case',
  standalone: true,
  imports: [CommonModule, RouterLink, PublicHeaderComponent, SiteFooterComponent],
  styles: [`
    :host { display: block; }
    .page { background: var(--color-paper); color: var(--color-ink); font-family: var(--font-body); font-size: var(--type-md); line-height: 1.55; -webkit-font-smoothing: antialiased; min-height: 100dvh; }
    .wrap { width: min(var(--container), 100%); margin-inline: auto; padding-inline: var(--gutter); }
    .col { width: min(52rem, 100%); display: grid; gap: var(--space-lg); }
    h1, p, ul { margin: 0; }
    h1.title { font-family: var(--font-display); font-weight: 500; font-size: var(--type-display-s); line-height: 1.04; letter-spacing: var(--tracking-display); overflow-wrap: break-word; min-width: 0; text-wrap: balance; max-width: 18ch; }
    .lead { margin-top: var(--space-lg); font-size: var(--type-lead); line-height: 1.45; color: var(--color-ink-2); max-width: 46ch; }
    .crumb { display: inline-flex; align-items: center; min-height: 44px; margin-bottom: var(--space-md); font-size: var(--type-sm); color: var(--color-muted); text-underline-offset: 0.2em; }
    .crumb:hover { color: var(--color-ink); }
    .crumb:focus-visible, .link:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: 2px; }
    .hero { background: var(--color-paper); color: var(--color-ink); padding: calc(56px + var(--space-3xl)) 0 var(--space-2xl); }
    .actions { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-md); margin-top: var(--space-xl); }
    .fine { margin-top: var(--space-sm); font-size: var(--type-sm); color: var(--color-muted); }
    .block { padding: var(--space-2xl) 0; border-top: 1px solid var(--color-rule); }
    .intro { font-size: var(--type-lead); line-height: 1.5; color: var(--color-ink-2); }
    ul.ticks { list-style: none; padding: 0; display: grid; gap: var(--space-sm); color: var(--color-ink-2); }
    ul.ticks li { padding-left: var(--space-lg); position: relative; }
    ul.ticks li::before { content: ''; position: absolute; left: 0; top: 0.7em; width: 10px; height: 2px; background: var(--color-accent); }
    .cta { background: var(--color-paper-2); }
    .cta .wrap { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-lg); }
    .cta p { font-family: var(--font-display); font-size: var(--type-xl); font-weight: 500; line-height: 1.2; max-width: 30ch; }
    .others-title { font-size: var(--type-sm); font-weight: 600; color: var(--color-muted); }
    .others { display: flex; flex-wrap: wrap; gap: var(--space-xs); margin-top: var(--space-sm); }
  `],
  template: `
    <div class="page hm hm-on-light">
      <app-public-header />
      <main>
        <section class="hero hm-on-dark" aria-labelledby="use-case-title">
          <div class="wrap">
            <a routerLink="/shorts-maker" class="crumb">AI Shorts Maker</a>
            <h1 id="use-case-title" class="title">{{ data.title }}</h1>
            <p class="lead">{{ data.hook }}</p>
            <div class="actions">
              <a routerLink="/shorts" class="ui-btn ui-btn--primary">Try for free</a>
            </div>
            <p class="fine">Free during beta. No credit card.</p>
          </div>
        </section>
        <section class="block">
          <div class="wrap"><div class="col">
            <p class="intro">{{ data.description }}</p>
            <ul class="ticks">
              @for (b of data.benefits; track $index) {
                <li>{{ b }}</li>
              }
            </ul>
          </div></div>
        </section>
        <section class="block cta" aria-label="Get started">
          <div class="wrap">
            <p>{{ data.cta }}</p>
            <a routerLink="/shorts" class="ui-btn ui-btn--secondary">Try for free</a>
          </div>
        </section>
        <section class="block">
          <div class="wrap">
            <p class="others-title">Other use cases</p>
            <div class="others">
              @for (u of others; track u.slug) {
                <a [routerLink]="['/use-cases', u.slug]" class="ui-btn ui-btn--secondary">{{ audience(u) }}</a>
              }
            </div>
          </div>
        </section>
      </main>
      <app-site-footer />
    </div>
  `,
})
export class UseCaseComponent implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly document = inject(DOCUMENT);
  private readonly destroyRef = inject(DestroyRef);
  data: UseCaseData = USE_CASE_DATA['content-creators'];
  others: UseCaseData[] = [];

  /** "Media companies" from "AI Shorts Maker for Media Companies": the heading already says what these are (#256). */
  audience(u: UseCaseData): string {
    const who = u.title.replace(/^AI Shorts Maker for\s+/i, '');
    return who.charAt(0).toUpperCase() + who.slice(1).toLowerCase();
  }
  currentYear = new Date().getFullYear();
  ngOnInit(): void {
    // The router reuses this component when only :slug changes, as it does for the
    // "Other use cases" links, so follow the param map instead of reading it once.
    this.route.paramMap
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((params) => this.show(params.get('slug') ?? 'content-creators'));
  }

  private show(slug: string): void {
    this.data = USE_CASE_DATA[slug] ?? USE_CASE_DATA['content-creators'];
    this.others = Object.values(USE_CASE_DATA).filter((u) => u.slug !== this.data.slug);
    setPageSeo(this.document, {
      title: `${this.data.title} | Indifferent`,
      description: this.data.description,
      canonical: `https://indifferent.fun/use-cases/${this.data.slug}`,
    });
    // These six pages share one template and about sixty words each. Until they
    // carry material of their own they stay out of search results; the AI Shorts
    // page and the journal hold the real documentation.
    setRobots(this.document, ROBOTS_NOINDEX);
  }
}
