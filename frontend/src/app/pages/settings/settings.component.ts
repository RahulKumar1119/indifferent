import { Component, DestroyRef, OnInit, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { LucideDynamicIcon } from '@lucide/angular';
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
  imports: [LucideDynamicIcon],
  template: `
    <div class="mx-auto max-w-2xl p-6">
      <h1 class="mb-6 text-2xl font-bold">Settings</h1>

      <div class="glass-card p-6 mb-6">
        <h2 class="mb-4 text-lg font-semibold flex items-center gap-2">
          <svg lucideIcon="sun" [size]="20" class="text-[hsl(var(--primary))]"></svg>
          Appearance
        </h2>
        <div class="flex items-center justify-between">
          <div>
            <p class="font-medium">Dark Mode</p>
            <p class="text-sm text-[hsl(var(--muted-foreground))]">
              Toggle between light and dark theme
            </p>
          </div>
          <!-- Custom toggle switch -->
          <button
            class="relative w-12 h-6 rounded-full transition-colors cursor-pointer"
            [class.bg-[hsl(var(--primary))]]="themeService.currentTheme() === 'dark'"
            [class.bg-[hsl(var(--muted))]]="themeService.currentTheme() === 'light'"
            (click)="themeService.toggleTheme()"
            role="switch"
            [attr.aria-checked]="themeService.currentTheme() === 'dark'"
            aria-label="Toggle dark mode"
          >
            <span
              class="absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-white shadow-sm transition-transform flex items-center justify-center"
              [class.translate-x-6]="themeService.currentTheme() === 'dark'"
            >
              @if (themeService.currentTheme() === 'dark') {
                <svg lucideIcon="moon" [size]="12" class="text-[hsl(var(--primary))]"></svg>
              } @else {
                <svg lucideIcon="sun" [size]="12" class="text-yellow-500"></svg>
              }
            </span>
          </button>
        </div>
      </div>

      <section class="glass-card p-6" aria-labelledby="notifications-heading" data-testid="notifications">
        <h2 id="notifications-heading" class="mb-1 text-lg font-semibold flex items-center gap-2">
          <svg lucideIcon="settings" [size]="20" class="text-[hsl(var(--primary))]"></svg>
          Notifications
        </h2>
        <p class="mb-4 text-sm text-[hsl(var(--muted-foreground))]">
          We email you when a video or Shorts job finishes or fails.
        </p>

        @switch (loadState()) {
          @case ('loading') {
            <p class="text-sm text-[hsl(var(--muted-foreground))]" role="status">Loading your settings…</p>
          }
          @case ('error') {
            <div role="alert" class="flex flex-wrap items-center gap-3">
              <p class="text-sm">We couldn't load your notification settings.</p>
              <button
                type="button"
                (click)="fetch()"
                class="min-h-[44px] rounded-full border border-[hsl(var(--border))] px-5 text-sm font-medium cursor-pointer"
              >
                Try again
              </button>
            </div>
          }
          @case ('ready') {
            @if (settings(); as s) {
              <p class="mb-4 text-sm" data-testid="address">
                @if (s.email) {
                  Emails go to <strong class="break-all font-semibold">{{ s.email }}</strong>.
                } @else {
                  There is no email address saved on your account.
                }
              </p>

              @if (!s.emailVerified) {
                <p
                  class="mb-4 rounded-xl border border-[hsl(var(--border))] bg-[hsl(var(--muted))] p-3 text-sm"
                  role="note"
                  data-testid="unverified"
                >
                  Email notifications are not available for this account. We only send to addresses we have
                  confirmed, and addresses from email and password sign-ups are not confirmed yet.
                </p>
              }

              <ul class="divide-y divide-[hsl(var(--border))]">
                @for (row of rows; track row.preference) {
                  <li class="flex items-center justify-between gap-4 py-3">
                    <div class="min-w-0">
                      <p [id]="row.id + '-label'" class="font-medium">{{ row.title }}</p>
                      <p [id]="row.id + '-hint'" class="text-sm text-[hsl(var(--muted-foreground))]">
                        {{ row.hint }}
                      </p>
                    </div>
                    <div class="flex shrink-0 items-center gap-2">
                      <span class="w-7 text-right text-sm" aria-hidden="true">
                        {{ isOn(row.preference) ? 'On' : 'Off' }}
                      </span>
                      <!-- 44 px hit area; the visible track is smaller. -->
                      <button
                        type="button"
                        role="switch"
                        class="inline-flex h-11 w-[60px] items-center justify-center rounded-full cursor-pointer disabled:cursor-not-allowed disabled:opacity-50"
                        [attr.data-testid]="row.id"
                        [attr.aria-checked]="isOn(row.preference)"
                        [attr.aria-labelledby]="row.id + '-label'"
                        [attr.aria-describedby]="row.id + '-hint'"
                        [attr.aria-busy]="savingPreference() === row.preference"
                        [disabled]="!s.emailVerified"
                        (click)="toggle(row.preference)"
                      >
                        <span
                          class="relative block h-7 w-12 rounded-full border transition-colors motion-reduce:transition-none"
                          [class.bg-[hsl(var(--primary))]]="isOn(row.preference)"
                          [class.bg-[hsl(var(--muted))]]="!isOn(row.preference)"
                          [style.border-color]="isOn(row.preference) ? 'hsl(var(--primary))' : 'hsl(var(--muted-foreground))'"
                        >
                          <span
                            class="absolute left-0.5 top-0.5 block h-5 w-5 rounded-full shadow-sm transition-transform motion-reduce:transition-none"
                            [class.bg-white]="isOn(row.preference)"
                            [class.bg-[hsl(var(--muted-foreground))]]="!isOn(row.preference)"
                            [style.transform]="isOn(row.preference) ? 'translateX(20px)' : 'translateX(0)'"
                          ></span>
                        </span>
                      </button>
                    </div>
                  </li>
                }
              </ul>

              <p class="mt-3 min-h-[20px] text-sm text-[hsl(var(--muted-foreground))]" role="status" aria-live="polite" data-testid="save-status">
                @switch (saveState()) {
                  @case ('saving') { Saving… }
                  @case ('saved') { Saved. }
                }
              </p>
              @if (saveState() === 'error') {
                <p class="save-error mt-1 text-sm font-medium" role="alert" data-testid="save-error">
                  We couldn't save that change. Your setting was not changed. Try again.
                </p>
              }
            }
          }
        }
      </section>
    </div>
  `,
  styles: [
    `
      /* The app's dark theme is a .dark class on <html>; Tailwind's dark: variant is not wired to it. */
      .save-error {
        color: #b91c1c;
      }
      :host-context(.dark) .save-error {
        color: #f87171;
      }
    `,
  ],
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
