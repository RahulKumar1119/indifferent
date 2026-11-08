import { Component, input } from '@angular/core';
import { RouterLink, RouterLinkActive } from '@angular/router';

/**
 * Page title plus the Profile / Settings / Account tabs shared by the three
 * account screens. Each tab is its own route, so this is navigation, not a tablist.
 */
@Component({
  selector: 'app-account-nav',
  standalone: true,
  imports: [RouterLink, RouterLinkActive],
  styles: [`
    :host { display: block; }
    h1 { margin: 0; font-size: var(--type-2xl); font-weight: 600; line-height: 1.15; letter-spacing: -0.01em; }
    .lead { margin: var(--space-2xs) 0 0; color: var(--color-ink-2); font-size: var(--type-sm); max-width: 60ch; }
    nav { margin-top: var(--space-md); border-bottom: 1px solid var(--color-rule); overflow-x: auto; }
    ul { display: flex; gap: var(--space-2xs); margin: 0; padding: 0; list-style: none; }
    a { display: inline-flex; align-items: center; min-height: 44px; padding: 0 var(--space-sm); border-bottom: 2px solid transparent; margin-bottom: -1px; color: var(--color-ink-2); font-size: var(--type-sm); font-weight: 500; text-decoration: none; white-space: nowrap; }
    a:hover { color: var(--color-ink); }
    a.active { color: var(--color-ink); font-weight: 600; border-bottom-color: var(--color-accent); }
    a:focus-visible { outline: 2px solid var(--color-focus); outline-offset: -2px; border-radius: var(--radius-control); }
  `],
  template: `
    <h1>{{ title() }}</h1>
    @if (lead()) {
      <p class="lead">{{ lead() }}</p>
    }
    <nav aria-label="Your account">
      <ul>
        <li><a routerLink="/profile" routerLinkActive="active" [routerLinkActiveOptions]="{ exact: true }" ariaCurrentWhenActive="page">Profile</a></li>
        <li><a routerLink="/profile/settings" routerLinkActive="active" ariaCurrentWhenActive="page">Settings</a></li>
        <li><a routerLink="/account" routerLinkActive="active" ariaCurrentWhenActive="page">Account</a></li>
      </ul>
    </nav>
  `,
})
export class AccountNavComponent {
  readonly title = input.required<string>();
  readonly lead = input('');
}
