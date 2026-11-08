import { Component, inject, OnInit, signal } from '@angular/core';
import { DatePipe } from '@angular/common';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { ApiService } from '../../core/services/api.service';
import { AuthService } from '../../core/services/auth.service';
import { UserProfile } from '../../shared/models/user-profile.model';
import { EmailConfirmBannerComponent } from '../../shared/components/email-confirm-banner/email-confirm-banner.component';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { AccountNavComponent } from './account-nav.component';

/** Account and security: who is signed in, how to sign out, and how to delete the account. */
@Component({
  selector: 'app-account',
  standalone: true,
  imports: [DatePipe, RouterLink, LucideDynamicIcon, EmailConfirmBannerComponent, NavbarComponent, AccountNavComponent],
  styles: [`
    :host { display: block; }
    dl { margin: var(--space-md) 0 0; display: grid; gap: var(--space-sm); }
    @media (min-width: 40rem) { dl { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
    dl div { display: flex; flex-direction: column; gap: var(--space-3xs); min-width: 0; }
    dt { font-size: var(--type-xs); color: var(--color-muted); }
    dd { margin: 0; font-weight: 500; overflow-wrap: anywhere; }
    .who { display: flex; align-items: center; gap: var(--space-md); }
    .who img { width: 3.5rem; height: 3.5rem; border-radius: 50%; object-fit: cover; border: 1px solid var(--color-rule); flex: none; }
    .actions { margin-top: var(--space-md); display: flex; flex-wrap: wrap; gap: var(--space-xs); }
    .hint { margin: var(--space-sm) 0 0; font-size: var(--type-sm); color: var(--color-ink-2); }
    .err { margin: var(--space-sm) 0 0; font-size: var(--type-sm); font-weight: 500; color: var(--color-negative); }
    .danger-title { display: flex; align-items: center; gap: var(--space-xs); color: var(--color-negative); }
    .sub-title { margin: var(--space-md) 0 0; font-size: var(--type-md); font-weight: 600; }
    .danger p { margin: var(--space-xs) 0 0; color: var(--color-ink-2); max-width: 62ch; }
  `],
  template: `
    <div class="ui-app">
      <app-navbar />
      <main id="main" class="ui-app-main hm">
        <app-account-nav title="Account" lead="Your sign-in, your sessions and your data." />
        <app-email-confirm-banner />

        <section class="ui-card" aria-labelledby="acct-details">
          <div class="who">
            <img [src]="profile().avatarUrl || '/icons/icon-96x96.png'" [alt]="''" />
            <div>
              <h2 id="acct-details" class="ui-card-title">Account details</h2>
              <p class="ui-card-sub">From your sign-in. Edit your name, username and bio on <a routerLink="/profile" class="ui-link">Profile</a>.</p>
            </div>
          </div>
          <dl>
            <div><dt>Name</dt><dd>{{ profile().name || 'Not set' }}</dd></div>
            <div><dt>Email</dt><dd data-testid="email">{{ profile().email || 'Not set' }}</dd></div>
            <div><dt>Member since</dt><dd>{{ profile().createdAt ? (profile().createdAt | date: 'mediumDate') : 'Unknown' }}</dd></div>
          </dl>
        </section>

        <section class="ui-card" aria-labelledby="acct-sessions">
          <h2 id="acct-sessions" class="ui-card-title">Signing out</h2>
          <p class="ui-card-sub">Sign out here, or end every sign-in on this account at once.</p>
          <div class="actions">
            <button type="button" class="ui-btn ui-btn--secondary" (click)="signOut()">
              <svg lucideIcon="log-out" [size]="18" aria-hidden="true"></svg>
              Sign out
            </button>
            <button type="button" class="ui-btn ui-btn--secondary" [disabled]="signingOutAll()" [attr.aria-busy]="signingOutAll()" (click)="signOutEverywhere()">
              Sign out of all devices
            </button>
          </div>
          <p class="hint">
            Signing out of all devices ends every sign-in on this account. Other devices stop working within 15 minutes.
          </p>
          @if (signOutAllError()) {
            <p class="err" role="alert">{{ signOutAllError() }}</p>
          }
        </section>

        <section class="ui-card ui-danger danger" aria-labelledby="acct-danger" data-testid="danger-zone">
          <h2 id="acct-danger" class="ui-card-title danger-title">
            <svg lucideIcon="triangle-alert" [size]="20" aria-hidden="true"></svg>
            Danger zone
          </h2>
          <h3 class="sub-title">Delete account</h3>
          <p>
            Deleting your account removes your profile, your projects, your Shorts and their files. It cannot be undone.
          </p>
          <p>
            There is no delete button in the app yet. Account deletion is handled by a support request: send it from the address on this account and we delete everything by hand.
          </p>
          <div class="actions">
            <a routerLink="/contact" class="ui-btn ui-btn--danger">Request account deletion</a>
          </div>
        </section>
      </main>
    </div>
  `,
})
export class AccountComponent implements OnInit {
  private readonly apiService = inject(ApiService);
  private readonly authService = inject(AuthService);
  readonly profile = signal<UserProfile>({
    email: '',
    name: '',
    avatarUrl: '',
    createdAt: '',
  });
  ngOnInit(): void {
    this.apiService.get<UserProfile>('/auth/me').subscribe({
      next: (user) => this.profile.set(user),
      error: () => {
        // Fallback if endpoint not ready
      },
    });
  }
  readonly signingOutAll = signal(false);
  readonly signOutAllError = signal('');
  signOut(): void {
    this.authService.logout();
  }
  signOutEverywhere(): void {
    if (this.signingOutAll()) return;
    this.signingOutAll.set(true);
    this.signOutAllError.set('');
    this.authService.logoutAll().subscribe({
      error: () => {
        this.signingOutAll.set(false);
        this.signOutAllError.set('Could not sign you out of all devices. Please try again.');
      },
    });
  }
}
