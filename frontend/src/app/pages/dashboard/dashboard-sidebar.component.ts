import { Component, inject } from '@angular/core';
import { RouterLink, RouterLinkActive } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { ThemeService } from '../../core/services/theme.service';

/** Desktop side navigation for the dashboard. Phones and tablets use the top navbar instead. */
@Component({
  selector: 'app-dashboard-sidebar',
  standalone: true,
  imports: [RouterLink, RouterLinkActive, LucideDynamicIcon],
  styles: [`
    :host { display: block; border-right: 1px solid var(--color-rule); background: var(--card); }
    aside { position: sticky; top: 0; height: 100dvh; overflow-y: auto; display: flex; flex-direction: column; gap: var(--space-lg); padding: var(--space-lg) var(--space-md); }
    .brand { display: inline-flex; align-items: center; min-height: 44px; padding-inline: var(--space-sm); font-weight: 600; font-size: var(--type-lg); color: var(--color-ink); text-decoration: none; }
    .brand span { color: var(--color-accent-text); }
    ul { display: flex; flex-direction: column; gap: var(--space-3xs); }
    a.link, button { display: flex; align-items: center; gap: var(--space-sm); width: 100%; min-height: 44px; padding-inline: var(--space-sm); border-radius: var(--radius-control); color: var(--color-ink-2); font-size: var(--type-sm); font-weight: 500; text-decoration: none; background: transparent; border: 0; cursor: pointer; transition: background-color var(--dur-short) var(--ease-out), color var(--dur-short) var(--ease-out); }
    a.link:hover, button:hover { background: var(--color-paper-3); color: var(--color-ink); }
    a.link.active { background: color-mix(in oklab, var(--color-accent) 12%, var(--card)); color: var(--color-ink); font-weight: 600; }
    a.link.active svg { color: var(--color-accent-text); }
    a:focus-visible, button:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; }
    .foot { margin-top: auto; border-top: 1px solid var(--color-rule); padding-top: var(--space-md); }
    @media (prefers-reduced-motion: reduce) { a.link, button { transition: none; } }
  `],
  template: `
    <aside>
      <a routerLink="/" class="brand">Indifferent<span>.</span></a>
      <nav aria-label="Studio">
        <ul>
          @for (l of links; track l.label) {
            <li>
              <a class="link" [routerLink]="l.link" routerLinkActive="active" [routerLinkActiveOptions]="{ exact: true }">
                <svg [lucideIcon]="l.icon" [size]="18" aria-hidden="true"></svg>
                {{ l.label }}
              </a>
            </li>
          }
        </ul>
      </nav>
      <div class="foot">
        <button
          type="button"
          (click)="theme.toggleTheme()"
          [attr.aria-label]="theme.currentTheme() === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'"
        >
          <svg [lucideIcon]="theme.currentTheme() === 'dark' ? 'sun' : 'moon'" [size]="18" aria-hidden="true"></svg>
          {{ theme.currentTheme() === 'dark' ? 'Light mode' : 'Dark mode' }}
        </button>
      </div>
    </aside>
  `,
})
export class DashboardSidebarComponent {
  readonly theme = inject(ThemeService);
  readonly links = [
    { label: 'Dashboard', icon: 'layout-dashboard', link: '/dashboard' },
    { label: 'Quiz videos', icon: 'film', link: '/projects' },
    { label: 'AI Shorts', icon: 'scissors', link: '/shorts' },
    { label: 'Story', icon: 'clapperboard', link: '/story' },
    { label: 'Thumbnails', icon: 'image', link: '/shorts/history' },
    { label: 'Watermark', icon: 'stamp', link: '/tools/add-watermark' },
    { label: 'Profile', icon: 'user', link: '/profile' },
  ];
}
