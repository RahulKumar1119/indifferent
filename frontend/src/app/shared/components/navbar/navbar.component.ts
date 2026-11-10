import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink, RouterLinkActive } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { ThemeService } from '../../../core/services/theme.service';

/**
 * Compact app navbar for the signed-in tool pages: brand, section links, theme
 * toggle. It follows the light or dark theme through the brand tokens.
 */
@Component({
  selector: 'app-navbar',
  standalone: true,
  imports: [CommonModule, RouterLink, RouterLinkActive, LucideDynamicIcon],
  styles: [`
    :host { display: block; }
    .bar { position: sticky; top: 0; z-index: var(--z-nav); background: var(--color-paper); color: var(--color-ink); border-bottom: 1px solid var(--color-rule); font-family: var(--font-body); }
    .row { max-width: var(--container); margin-inline: auto; padding-inline: var(--space-md); height: 56px; display: flex; align-items: center; justify-content: space-between; gap: var(--space-xs); }
    .brand { font-weight: 600; font-size: var(--type-md); letter-spacing: -0.01em; color: var(--color-ink); text-decoration: none; display: inline-flex; align-items: center; min-height: 44px; }
    .brand .dot { color: var(--color-accent-text); }
    nav { display: flex; align-items: center; gap: var(--space-2xs); font-size: var(--type-sm); }
    .link, .icon-btn { display: inline-flex; align-items: center; justify-content: center; min-height: 44px; border-radius: var(--radius-control); color: var(--color-ink-2); text-decoration: none; transition: background-color var(--dur-short) var(--ease-out), color var(--dur-short) var(--ease-out); }
    .link { padding-inline: var(--space-sm); }
    .icon-btn { min-width: 44px; background: transparent; border: 0; cursor: pointer; }
    .link:hover, .icon-btn:hover { background: var(--color-paper-3); color: var(--color-ink); }
    .link.active { color: var(--color-ink); font-weight: 600; }
    .link:focus-visible, .icon-btn:focus-visible, .brand:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; }
    .wide { display: none; }
    @media (min-width: 40rem) { .wide { display: inline-flex; } .narrow { display: none; } }
    .toggle { margin-left: var(--space-2xs); border: 1px solid var(--color-muted); }
    @media (prefers-reduced-motion: reduce) { .link, .icon-btn { transition: none; } }
  `],
  template: `
    <header class="bar">
      <div class="row">
        <a routerLink="/" class="brand">Indifferent<span class="dot">.</span></a>
        <nav aria-label="App sections">
          <a routerLink="/new" routerLinkActive="active" [routerLinkActiveOptions]="{ exact: true }" class="link">New</a>
          <a routerLink="/story" routerLinkActive="active" class="link">Story</a>
          <a routerLink="/shorts/history" routerLinkActive="active" class="link wide">History</a>
          <a routerLink="/dashboard" routerLinkActive="active" class="link wide">Dashboard</a>
          <a routerLink="/profile" routerLinkActive="active" class="link wide">Profile</a>
          <a routerLink="/profile" class="icon-btn narrow" aria-label="Profile">
            <svg lucideIcon="user" [size]="17"></svg>
          </a>
          <button
            type="button"
            class="icon-btn toggle"
            (click)="theme.toggleTheme()"
            [attr.aria-label]="theme.currentTheme() === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'"
            [title]="theme.currentTheme() === 'dark' ? 'Light mode' : 'Dark mode'"
          >
            @if (theme.currentTheme() === 'dark') {
              <svg lucideIcon="sun" [size]="17"></svg>
            } @else {
              <svg lucideIcon="moon" [size]="17"></svg>
            }
          </button>
        </nav>
      </div>
    </header>
  `,
})
export class NavbarComponent {
  readonly theme = inject(ThemeService);
}
