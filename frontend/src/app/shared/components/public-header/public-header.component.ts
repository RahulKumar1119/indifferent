import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';
import { NavMenuComponent } from '../nav-menu/nav-menu.component';
import { NavMobileMenuComponent } from '../nav-mobile-menu/nav-mobile-menu.component';

interface MenuItem {
  label: string;
  note?: string;
  path: string;
}

interface Menu {
  label: string;
  width: number;
  items: MenuItem[];
}

/**
 * The floating pill header for public pages: brand, three dropdown menus on
 * wide screens, one menu button on narrow ones, and the sign-in action.
 * Always a dark pill whatever the theme, so it pins the dark token set.
 * Links are the ones the landing page has always had; nothing is routed anew.
 */
@Component({
  selector: 'app-public-header',
  standalone: true,
  imports: [RouterLink, NavMenuComponent, NavMobileMenuComponent],
  styles: [`
    :host { display: block; }
    .wrap { position: fixed; top: var(--space-md); left: 50%; transform: translateX(-50%); z-index: var(--z-nav); width: min(1120px, calc(100% - 2rem)); font-family: var(--font-body); }
    .pill { display: flex; align-items: center; justify-content: space-between; height: 56px; padding: 0 var(--space-xs) 0 var(--space-lg); border-radius: var(--radius-pill); background: var(--color-paper); border: 1px solid var(--color-rule); color: var(--color-ink); }
    .brand { display: inline-flex; align-items: center; min-height: 44px; font-size: var(--type-lg); font-weight: 600; letter-spacing: -0.01em; color: var(--color-ink); text-decoration: none; }
    .brand .dot { color: var(--color-accent-text); }
    .menus { display: none; align-items: center; gap: var(--space-lg); font-size: var(--type-sm); }
    @media (min-width: 48rem) { .menus { display: flex; } }
    .end .ui-btn { white-space: nowrap; padding: 0 var(--space-md); }
    .end { display: flex; align-items: center; gap: var(--space-2xs); }
    .item { display: block; padding: var(--space-sm) var(--space-md); border-radius: var(--radius-surface); text-decoration: none; color: var(--color-ink); transition: background-color var(--dur-short) var(--ease-out); }
    .item:hover { background: var(--color-paper-3); }
    .item:focus-visible { outline: 2px solid var(--color-focus); outline-offset: -2px; }
    .item-title { display: block; font-size: var(--type-sm); font-weight: 500; }
    .item-note { display: block; margin-top: var(--space-3xs); font-size: var(--type-xs); color: var(--color-muted); line-height: 1.4; }
    .group { margin: 0; padding: var(--space-sm) var(--space-md) var(--space-2xs); font-size: var(--type-xs); font-weight: 500; color: var(--color-muted); }
    .brand:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: 4px; }
    @media (prefers-reduced-motion: reduce) { .item { transition: none; } }
  `],
  template: `
    <header class="wrap hm hm-on-dark">
      <nav class="pill" aria-label="Main">
        <a routerLink="/" class="brand">Indifferent<span class="dot">.</span></a>
        <div class="menus">
          @for (m of menus; track m.label) {
            <app-nav-menu [label]="m.label" [width]="m.width">
              @for (i of m.items; track i.path) {
                <a [routerLink]="i.path" class="item">
                  <span class="item-title">{{ i.label }}</span>
                  @if (i.note) { <span class="item-note">{{ i.note }}</span> }
                </a>
              }
            </app-nav-menu>
          }
        </div>
        <div class="end">
          <app-nav-mobile-menu>
            @for (m of menus; track m.label) {
              <p class="group">{{ m.label }}</p>
              @for (i of m.items; track i.path) {
                <a [routerLink]="i.path" class="item"><span class="item-title">{{ i.label }}</span></a>
              }
            }
          </app-nav-mobile-menu>
          <a routerLink="/login" class="ui-btn ui-btn--primary">Get started</a>
        </div>
      </nav>
    </header>
  `,
})
export class PublicHeaderComponent {
  readonly menus: Menu[] = [
    {
      label: 'Features',
      width: 340,
      items: [
        { label: 'TXT to narrated video', path: '/features/txt-to-video-quiz', note: 'Text-based quiz files (.txt) into narrated video content with answer reveals, ready for YouTube or any platform.' },
        { label: 'Watermark tool', path: '/features/watermark', note: 'Draggable text and logo watermarks on images and video. Free, private, 100% in-browser.' },
        { label: 'Infographic maker', path: '/features/infographic-maker', note: 'Twenty templates, charts from your numbers, icons you can drag. PNG, JPG or PDF. Free, private, 100% in-browser.' },
        { label: 'AI Shorts generator', path: '/features/ai-shorts', note: 'Turn a long video or audio file into ranked 9:16 vertical clips with burned-in captions. AI finds the most engaging moments automatically.' },
        { label: 'Story to video', path: '/features/story-video', note: 'Paste a short story and get a narrated 9:16 video. A model plans the scenes and matches stock footage, and you confirm every pick.' },
      ],
    },
    {
      label: 'Use cases',
      width: 270,
      items: [
        { label: 'YouTube creators', path: '/blog/quiz-videos-youtube', note: 'Faceless quiz channels' },
        { label: 'Teachers & educators', path: '/blog/educational-video-best-practices', note: 'Lesson recaps & revision' },
        { label: 'AI narration', path: '/blog/ai-narration-guide', note: '5 voices to choose from' },
      ],
    },
    {
      label: 'Resources',
      width: 270,
      items: [
        { label: 'Journal', path: '/blog', note: 'Essays & notes' },
        { label: 'Format guide', path: '/blog/quiz-file-format-guide', note: 'Write parseable TXT' },
        { label: 'About the studio', path: '/about', note: 'What we build' },
      ],
    },
  ];
}
