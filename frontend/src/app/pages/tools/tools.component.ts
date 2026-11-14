import { Component, OnInit, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { Meta, Title } from '@angular/platform-browser';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { PRODUCT_FACTS } from '../../core/config/product-facts';
import { setCanonical, setJsonLd } from '../../shared/seo';
import { PublicHeaderComponent } from '../../shared/components/public-header/public-header.component';
import { SiteFooterComponent } from '../../shared/components/site-footer/site-footer.component';

const I = PRODUCT_FACTS.infographic;

interface Tool {
  id: string;
  icon: string;
  title: string;
  text: string;
  points: string[];
  /** A real screenshot of the tool, or null for the drawn illustration. */
  image: { src: string; alt: string; w: number; h: number } | null;
  /** True for the tool that needs no account. */
  free: boolean;
  infoLink: string | null;
  ctaLink: string;
  ctaLabel: string;
}

@Component({
  selector: 'app-tools',
  standalone: true,
  imports: [RouterLink, LucideDynamicIcon, PublicHeaderComponent, SiteFooterComponent],
  styles: [`
    :host { display: block; }
    .page { background: var(--color-paper-2); color: var(--color-ink); font-family: var(--font-body); min-height: 100dvh; -webkit-font-smoothing: antialiased; }
    .wrap { width: min(80rem, 100%); margin-inline: auto; padding-inline: var(--space-md); }
    @media (min-width: 48rem) { .wrap { padding-inline: var(--space-xl); } }
    .hero { background: var(--color-paper); padding: calc(56px + var(--space-2xl)) 0 var(--space-2xl); border-bottom: 1px solid var(--color-rule); }
    h1 { margin: 0; font-family: var(--font-display); font-weight: 500; font-size: var(--type-display-s); line-height: 1; letter-spacing: var(--tracking-display); }
    h1 em { color: var(--color-accent-text); }
    .lead { margin: var(--space-sm) 0 0; max-width: 56ch; font-size: var(--type-lead); line-height: 1.45; color: var(--color-ink-2); }
    main { padding: var(--space-2xl) 0 var(--space-3xl); }
    .grid { margin: 0; padding: 0; list-style: none; display: grid; gap: var(--space-md); }
    @media (min-width: 48rem) { .grid { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-lg); } }
    @media (min-width: 72rem) { .grid { grid-template-columns: repeat(6, minmax(0, 1fr)); } .grid > li { grid-column: span 2; } .grid > li.wide { grid-column: span 3; } }
    .card { height: 100%; display: flex; flex-direction: column; overflow: hidden; background: var(--color-paper); border: 1px solid var(--color-rule); border-radius: var(--radius-surface); box-shadow: 0 1px 2px color-mix(in oklab, var(--color-ink) 5%, transparent), 0 12px 32px -18px color-mix(in oklab, var(--color-ink) 28%, transparent); }
    .media { aspect-ratio: 16 / 10; background: var(--color-paper-3); border-bottom: 1px solid var(--color-rule); overflow: hidden; }
    .media img { width: 100%; height: 100%; object-fit: cover; object-position: top left; display: block; }
    .media svg { width: 100%; height: 100%; display: block; }
    .body { flex: 1; display: flex; flex-direction: column; gap: var(--space-xs); padding: var(--space-lg); }
    .top { display: flex; align-items: center; justify-content: space-between; gap: var(--space-sm); }
    .chip { width: 2.5rem; height: 2.5rem; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; background: color-mix(in oklab, var(--color-accent) 12%, var(--color-paper)); color: var(--color-accent-text); }
    .pill { display: inline-flex; align-items: center; min-height: 1.75rem; padding: 0 var(--space-sm); border-radius: var(--radius-pill); font-size: var(--type-xs); font-weight: 600; border: 1px solid var(--color-rule); color: var(--color-ink-2); }
    .pill.free { background: var(--color-accent); border-color: var(--color-accent); color: var(--color-accent-ink); }
    h2 { margin: var(--space-xs) 0 0; font-size: var(--type-xl); font-weight: 600; line-height: 1.2; }
    .text { margin: 0; color: var(--color-ink-2); line-height: 1.55; }
    .points { margin: var(--space-2xs) 0 0; padding: 0; list-style: none; display: flex; flex-direction: column; gap: var(--space-2xs); font-size: var(--type-sm); color: var(--color-ink-2); }
    .points li { display: flex; gap: var(--space-xs); align-items: flex-start; }
    .points svg { flex: none; margin-top: 2px; color: var(--color-accent-text); }
    .actions { margin-top: auto; padding-top: var(--space-md); display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-xs) var(--space-md); }
    .actions .ui-link { display: inline-flex; align-items: center; min-height: 44px; font-size: var(--type-sm); }
    .closing { margin-top: var(--space-2xl); padding: var(--space-xl) var(--space-lg); text-align: center; border-radius: var(--radius-surface); background: var(--color-paper); border: 1px solid var(--color-rule); }
    .closing p { margin: 0; }
    .closing .big { font-family: var(--font-display); font-size: var(--type-2xl); line-height: 1.1; }
    .closing .sub { margin: var(--space-2xs) auto var(--space-md); max-width: 52ch; color: var(--color-ink-2); }
  `],
  template: `
    <div class="page hm hm-on-light">
      <app-public-header />
      <header class="hero">
        <div class="wrap">
          <h1>Six tools, <em>one studio.</em></h1>
          <p class="lead">Make infographics, quiz videos and shorts, turn a story into a video, pick a thumbnail and watermark your files. The watermark tool and the infographic maker are free and run in your browser with no account.</p>
        </div>
      </header>
      <main id="main">
        <div class="wrap">
          <ul class="grid">
            @for (tool of tools; track tool.id; let i = $index) {
              <li [class.wide]="i < 2" [attr.data-testid]="'tool-' + tool.id">
                <article class="card">
                  <div class="media">
                    @if (tool.image) {
                      <img [src]="tool.image.src" [alt]="tool.image.alt" [width]="tool.image.w" [height]="tool.image.h" loading="lazy" />
                    } @else {
                      <svg viewBox="0 0 320 200" role="img" aria-label="Illustration: one video frame, with three thumbnail options under it and the chosen one outlined">
                        <rect x="0" y="0" width="320" height="200" fill="var(--color-paper-3)" />
                        <rect x="104" y="18" width="112" height="100" rx="8" fill="var(--color-ink)" />
                        <circle cx="160" cy="62" r="16" fill="var(--color-accent)" />
                        <path d="M155 54 L168 62 L155 70 Z" fill="var(--color-accent-ink)" />
                        <rect x="118" y="92" width="84" height="8" rx="4" fill="var(--color-paper)" opacity="0.8" />
                        @for (x of [62, 128, 194]; track x; let j = $index) {
                          <rect [attr.x]="x" y="134" width="64" height="48" rx="6" fill="var(--color-paper)" [attr.stroke]="j === 1 ? 'var(--color-accent)' : 'var(--color-rule)'" [attr.stroke-width]="j === 1 ? 3 : 1" />
                          <rect [attr.x]="x + 8" y="144" width="30" height="5" rx="2.5" fill="var(--color-ink)" opacity="0.7" />
                          <rect [attr.x]="x + 8" y="154" width="44" height="18" rx="3" fill="var(--color-accent)" [attr.opacity]="j === 1 ? 0.9 : 0.35" />
                        }
                      </svg>
                    }
                  </div>
                  <div class="body">
                    <div class="top">
                      <span class="chip"><svg [lucideIcon]="tool.icon" [size]="20" aria-hidden="true"></svg></span>
                      <span class="pill" [class.free]="tool.free">{{ tool.free ? 'Free, no sign-in' : 'Sign in to use' }}</span>
                    </div>
                    <h2>{{ tool.title }}</h2>
                    <p class="text">{{ tool.text }}</p>
                    <ul class="points">
                      @for (point of tool.points; track point) {
                        <li><svg lucideIcon="check" [size]="16" aria-hidden="true"></svg>{{ point }}</li>
                      }
                    </ul>
                    <div class="actions">
                      <a [routerLink]="tool.ctaLink" class="ui-btn" [class.ui-btn--primary]="tool.free" [class.ui-btn--secondary]="!tool.free">{{ tool.ctaLabel }}</a>
                      @if (tool.infoLink) {
                        <a [routerLink]="tool.infoLink" class="ui-link">Learn more<span class="sr-only"> about {{ tool.title }}</span></a>
                      }
                    </div>
                  </div>
                </article>
              </li>
            }
          </ul>
          <section class="closing" aria-label="Get started">
            <p class="big">Need full videos, not just tools?</p>
            <p class="sub">Turn text or long footage into finished MP4s with AI narration, ranking and captions.</p>
            <a routerLink="/login" class="ui-btn ui-btn--primary">Try for free</a>
          </section>
        </div>
      </main>
      <app-site-footer />
    </div>
  `,
})
export class ToolsComponent implements OnInit {
  private readonly title = inject(Title);
  private readonly meta = inject(Meta);
  private readonly document = inject(DOCUMENT);
  tools: Tool[] = [
    {
      id: 'watermark',
      icon: 'stamp',
      title: 'Watermark tool',
      text: 'Stamp images and video with draggable text or a logo. Files never leave your device.',
      points: ['JPG, PNG, WebP, MP4, WebM and MOV', 'Size, opacity, colour and rotation', 'Full-resolution PNG, JPEG and MP4 export'],
      image: { src: '/screenshots/tools-watermark.webp', alt: 'The watermark editor: a quiz slide with a text watermark in the preview, and the watermark settings beside it', w: 1280, h: 800 },
      free: true,
      infoLink: '/features/watermark',
      ctaLink: '/tools/add-watermark',
      ctaLabel: 'Open the watermark tool',
    },
    {
      id: 'infographic',
      icon: 'chart-column',
      title: 'Infographic maker',
      text: `${I.templates} templates, charts from your own numbers and icons you can drag. Your text never leaves your device.`,
      points: ['Key points, steps, timelines, charts, flow charts and mind maps', `English, Hindi, Gujarati and ${I.languages.length - 3} more languages`, `${I.exports.join(', ')} download`],
      image: { src: '/screenshots/tools-infographic.webp', alt: 'The infographic maker: a key points infographic in the preview, and the template, size and colour choices beside it', w: 1280, h: 800 },
      free: true,
      infoLink: '/features/infographic-maker',
      ctaLink: '/tools/infographic-maker',
      ctaLabel: 'Open the infographic maker',
    },
    {
      id: 'quiz',
      icon: 'file-text',
      title: 'Quiz videos',
      text: 'Plain-text questions in, narrated quiz MP4 out. Five voices to choose from.',
      points: ['Question and answer slides', 'AI narration in five voices', '1080p MP4 download'],
      image: { src: '/screenshots/tools-quiz.webp', alt: 'The new quiz video page: what the TXT file of questions needs, then the project name and the design picker', w: 1280, h: 800 },
      free: false,
      infoLink: '/features/txt-to-video-quiz',
      ctaLink: '/projects/new',
      ctaLabel: 'Create a quiz video',
    },
    {
      id: 'shorts',
      icon: 'scissors',
      title: 'AI Shorts',
      text: 'Long video or audio into ranked 9:16 clips with burned-in captions.',
      points: ['Moments picked from the transcript', 'Face-centred framing and blur-fill', '1080 by 1920 exports'],
      image: { src: '/screenshots/tools-shorts.webp', alt: 'The AI Shorts page, where you drop in a long video or audio file', w: 1280, h: 800 },
      free: false,
      infoLink: '/features/ai-shorts',
      ctaLink: '/shorts',
      ctaLabel: 'Make shorts',
    },
    {
      id: 'story',
      icon: 'clapperboard',
      title: 'Story video',
      text: 'A short story into a narrated vertical video with matching stock footage.',
      points: ['Scenes planned from your story', 'Stock photos and clips from Pexels and Pixabay', '1080 by 1920 exports, up to 90 seconds'],
      image: { src: '/landing/story-editor.webp', alt: 'The story editor, with the planned scenes and their stock footage', w: 1280, h: 860 },
      free: false,
      infoLink: '/features/story-video',
      ctaLink: '/story',
      ctaLabel: 'Make a story video',
    },
    {
      id: 'thumbnails',
      icon: 'image',
      title: 'AI thumbnails',
      text: 'Thumbnail options made from a video you uploaded for shorts. Pick the one you want.',
      points: ['Frames picked from your video', 'Several options to choose from', 'Opened from your shorts history'],
      image: null,
      free: false,
      infoLink: null,
      ctaLink: '/shorts/history',
      ctaLabel: 'Open your shorts',
    },
  ];

  ngOnInit(): void {
    const url = 'https://indifferent.fun/tools';
    const description = 'Free creator tools: an in-browser infographic maker and image and video watermarking, plus TXT to quiz video, AI shorts, story to video and AI thumbnails.';
    this.title.setTitle('Free Creator Tools | Indifferent');
    this.meta.updateTag({ name: 'description', content: description });
    this.meta.updateTag({ property: 'og:title', content: 'Free Creator Tools | Indifferent' });
    this.meta.updateTag({ property: 'og:description', content: description });
    this.meta.updateTag({ property: 'og:url', content: url });
    this.meta.updateTag({ property: 'og:image', content: 'https://indifferent.fun/og-cover.jpg' });
    setCanonical(this.document, url);
    setJsonLd(this.document, 'webpage', {
      '@context': 'https://schema.org',
      '@type': 'WebPage',
      name: 'Free Creator Tools | Indifferent',
      url,
      description,
    });
  }
}
