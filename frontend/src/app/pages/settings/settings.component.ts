import { Component, DestroyRef, OnInit, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { NavbarComponent } from '../../shared/components/navbar/navbar.component';
import { AccountNavComponent } from '../account/account-nav.component';
import { ThemeService } from '../../core/services/theme.service';
import {
  NotificationPreference,
  NotificationSettings,
  NotificationSettingsService,
} from './notification-settings.service';

type LoadState = 'loading' | 'ready' | 'error';
type SaveState = 'idle' | 'saving' | 'saved' | 'error';

interface NotificationRow {
  preference: NotificationPreference;
  id: string;
  title: string;
  hint: string;
}

@Component({
  selector: 'app-settings',
  standalone: true,
  imports: [NavbarComponent, AccountNavComponent],
  styles: [`
    :host { display: block; }
    .row { display: flex; align-items: center; justify-content: space-between; gap: var(--space-md); padding: var(--space-sm) 0; }
    .rows { margin: var(--space-sm) 0 0; padding: 0; list-style: none; }
    .rows li + li { border-top: 1px solid var(--color-rule); }
    .row-title { margin: 0; font-weight: 500; }
    .row-hint { margin: var(--space-3xs) 0 0; font-size: var(--type-sm); color: var(--color-muted); }
    .ctl { display: flex; align-items: center; gap: var(--space-xs); flex: none; }
    .state { width: 1.75rem; text-align: right; font-size: var(--type-sm); color: var(--color-ink-2); }
    .note { margin: var(--space-sm) 0 0; padding: var(--space-sm) var(--space-md); border-radius: var(--radius-control); background: var(--color-paper-3); font-size: var(--type-sm); color: var(--color-ink-2); }
    .address { margin: var(--space-sm) 0 0; font-size: var(--type-sm); }
    .address strong { overflow-wrap: anywhere; }
    .status { margin: var(--space-sm) 0 0; min-height: 1.25rem; font-size: var(--type-sm); color: var(--color-ink-2); }
    .save-error { margin: var(--space-2xs) 0 0; font-size: var(--type-sm); font-weight: 500; color: var(--color-negative); }
    .inline { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-sm); margin-top: var(--space-sm); }
    .inline p { margin: 0; font-size: var(--type-sm); }
  `],
  template: `
    <div class="ui-app">
      <app-navbar />
      <main id="main" class="ui-app-main hm">
        <app-account-nav title="Settings" lead="How the app looks and when we email you. Changes save as soon as you make them." />

        <section class="ui-card" aria-labelledby="appearance-heading">
          <h2 id="appearance-heading" class="ui-card-title">Appearance</h2>
          <div class="row">
            <div>
              <p class="row-title" id="theme-label">Dark mode</p>
              <p class="row-hint" id="theme-hint">Use the dark theme on this device.</p>
            </div>
            <button
              type="button"
              class="ui-switch"
              role="switch"
              data-testid="theme-switch"
              [attr.aria-checked]="themeService.currentTheme() === 'dark'"
              aria-labelledby="theme-label"
              aria-describedby="theme-hint"
              (click)="themeService.toggleTheme()"
            ><span></span></button>
          </div>
        </section>

        <section class="ui-card" aria-labelledby="notifications-heading" data-testid="notifications">
          <h2 id="notifications-heading" class="ui-card-title">Email notifications</h2>
          <p class="ui-card-sub">We email you when a video or Shorts job finishes or fails.</p>
          @switch (loadState()) {
            @case ('loading') {
              <p class="status" role="status">Loading your settings</p>
            }
            @case ('error') {
              <div role="alert" class="inline">
                <p>We couldn't load your notification settings.</p>
                <button type="button" (click)="fetch()" class="ui-btn ui-btn--secondary">Try again</button>
              </div>
            }
            @case ('ready') {
              @if (settings(); as s) {
                <p class="address" data-testid="address">
                  @if (s.email) {
                    Emails go to <strong>{{ s.email }}</strong>.
                  } @else {
                    There is no email address saved on your account.
                  }
                </p>
                @if (!s.emailVerified) {
                  <p class="note" role="note" data-testid="unverified">
                    Email notifications are not available for this account. We only send to addresses we have
                    confirmed, and addresses from email and password sign-ups are not confirmed yet.
                  </p>
                }
                <ul class="rows">
                  @for (row of rows; track row.preference) {
                    <li class="row">
                      <div style="min-width: 0">
                        <p [id]="row.id + '-label'" class="row-title">{{ row.title }}</p>
                        <p [id]="row.id + '-hint'" class="row-hint">{{ row.hint }}</p>
                      </div>
                      <div class="ctl">
                        <span class="state" aria-hidden="true">{{ isOn(row.preference) ? 'On' : 'Off' }}</span>
                        <button
                          type="button"
                          role="switch"
                          class="ui-switch"
                          [attr.data-testid]="row.id"
                          [attr.aria-checked]="isOn(row.preference)"
                          [attr.aria-labelledby]="row.id + '-label'"
                          [attr.aria-describedby]="row.id + '-hint'"
                          [attr.aria-busy]="savingPreference() === row.preference"
                          [disabled]="!s.emailVerified"
                          (click)="toggle(row.preference)"
                        ><span></span></button>
                      </div>
                    </li>
                  }
                </ul>
                <p class="status" role="status" aria-live="polite" data-testid="save-status">
                  @switch (saveState()) {
                    @case ('saving') { Saving }
                    @case ('saved') { Saved. }
                  }
                </p>
                @if (saveState() === 'error') {
                  <p class="save-error" role="alert" data-testid="save-error">
                    We couldn't save that change. Your setting was not changed. Try again.
                  </p>
                }
              }
            }
          }
        </section>
      </main>
    </div>
  `,
})
export class SettingsComponent implements OnInit {
  readonly themeService = inject(ThemeService);
  private readonly notifications = inject(NotificationSettingsService);
  private readonly destroyRef = inject(DestroyRef);

  readonly loadState = signal<LoadState>('loading');
  readonly saveState = signal<SaveState>('idle');
  readonly settings = signal<NotificationSettings | null>(null);
  readonly savingPreference = signal<NotificationPreference | null>(null);

  readonly rows: NotificationRow[] = [
    {
      preference: 'notifyCompleted',
      id: 'notify-completed',
      title: 'When a job finishes',
      hint: 'Email me when a video or Shorts job is ready.',
    },
    {
      preference: 'notifyFailed',
      id: 'notify-failed',
      title: 'When a job fails',
      hint: 'Email me when a video or Shorts job does not complete.',
    },
  ];

  ngOnInit(): void {
    this.fetch();
  }

  fetch(): void {
    this.loadState.set('loading');
    this.notifications
      .load()
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (s) => {
          this.settings.set(s);
          this.loadState.set('ready');
        },
        error: () => this.loadState.set('error'),
      });
  }

  /** What the switch shows. An unconfirmed address never gets mail, so it reads Off. */
  isOn(preference: NotificationPreference): boolean {
    const s = this.settings();
    return !!s && s.emailVerified && s[preference];
  }

  toggle(preference: NotificationPreference): void {
    const before = this.settings();
    // Ignore clicks while a save is in flight (the button stays focusable).
    if (!before || !before.emailVerified || this.savingPreference() !== null) return;

    const value = !before[preference];
    this.settings.set({ ...before, [preference]: value });
    this.savingPreference.set(preference);
    this.saveState.set('saving');

    this.notifications
      .save(preference, value)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (saved) => {
          this.settings.set(saved);
          this.savingPreference.set(null);
          this.saveState.set('saved');
        },
        error: () => {
          // Put the switch back: what is shown must be what is stored.
          this.settings.set(before);
          this.savingPreference.set(null);
          this.saveState.set('error');
        },
      });
  }
}
