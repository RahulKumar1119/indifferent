import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';

interface FooterLink {
  label: string;
  path: string;
}

/** Footer with the same link set on every content page, so the products, the
 * journal and the legal pages are always one click away. */
@Component({
  selector: 'app-site-footer',
  standalone: true,
  imports: [RouterLink],
  styles: [`
    :host { display: block; }
    .footer { background: var(--color-paper); color: var(--color-ink); border-top: 1px solid var(--color-rule); font-family: var(--font-body); padding: var(--space-xl) var(--space-md); }
    .inner { max-width: 48rem; margin-inline: auto; display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-xl); }
    @media (min-width: 40rem) { .inner { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
    .group-title { margin: 0; font-size: var(--type-sm); color: var(--color-muted); }
    ul { list-style: none; margin: var(--space-xs) 0 0; padding: 0; }
    .link { display: inline-flex; align-items: center; min-height: 44px; font-size: var(--type-sm); color: var(--color-ink); text-underline-offset: 0.2em; }
    .link:hover { color: var(--color-accent-text); text-decoration: underline; }
    .link:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: 2px; }
    .legal { max-width: 48rem; margin: var(--space-xl) auto 0; font-size: var(--type-xs); color: var(--color-muted); }
  `],
  template: `
    <footer class="footer hm-on-light" aria-label="Site footer">
      <div class="inner">
        <nav aria-label="Tools">
          <p class="group-title">Tools</p>
          <ul>
            @for (l of tools; track l.path) {
              <li><a [routerLink]="l.path" class="link min-h-[44px]">{{ l.label }}</a></li>
            }
          </ul>
        </nav>
        <nav aria-label="Learn">
          <p class="group-title">Learn</p>
          <ul>
            @for (l of learn; track l.path) {
              <li><a [routerLink]="l.path" class="link min-h-[44px]">{{ l.label }}</a></li>
            }
          </ul>
        </nav>
        <nav aria-label="Company">
          <p class="group-title">Company</p>
          <ul>
            @for (l of company; track l.path) {
              <li><a [routerLink]="l.path" class="link min-h-[44px]">{{ l.label }}</a></li>
            }
            <li><a href="mailto:support@indifferent.fun" class="link min-h-[44px]">support&#64;indifferent.fun</a></li>
          </ul>
        </nav>
      </div>
      <p class="legal">© {{ year }} Indifferent. Free during beta.</p>
    </footer>
  `,
})
export class SiteFooterComponent {
  readonly year = new Date().getFullYear();

  readonly tools: FooterLink[] = [
    { label: 'AI Shorts generator', path: '/features/ai-shorts' },
    { label: 'TXT to quiz video', path: '/features/txt-to-video-quiz' },
    { label: 'Story to video', path: '/features/story-video' },
    { label: 'Watermark tool', path: '/features/watermark' },
  ];

  readonly learn: FooterLink[] = [
    { label: 'Journal', path: '/blog' },
    { label: 'Help and FAQ', path: '/help' },
    { label: 'Quiz file format guide', path: '/blog/quiz-file-format-guide' },
  ];

  readonly company: FooterLink[] = [
    { label: 'Home', path: '/' },
    { label: 'About', path: '/about' },
    { label: 'Pricing', path: '/pricing' },
    { label: 'Contact', path: '/contact' },
    { label: 'Privacy', path: '/privacy' },
    { label: 'Terms', path: '/terms' },
  ];
}
