import { Component, OnInit, inject, signal } from '@angular/core';
import { HttpContext } from '@angular/common/http';
import { ApiService } from '../../../core/services/api.service';
import { AuthService } from '../../../core/services/auth.service';
import { SUPPRESS_GLOBAL_ERROR } from '../../../core/interceptors/error.interceptor';
import { UserProfile } from '../../models/user-profile.model';

type SendState = 'idle' | 'sending' | 'sent' | 'failed';

/**
 * Shown to a signed-in password account whose email address has not been
 * confirmed. Until it is, creating projects and Shorts is refused, so the
 * banner says so and offers to send the link again. It shows nothing for
 * everyone else, and nothing when the profile cannot be read.
 */
@Component({
  selector: 'app-email-confirm-banner',
  standalone: true,
  styles: [`
    :host { display: block; }
    .banner { background: color-mix(in oklab, var(--color-accent) 8%, var(--color-paper)); color: var(--color-ink); border: 1px solid color-mix(in oklab, var(--color-accent) 40%, transparent); border-radius: var(--radius-surface); padding: var(--space-md) var(--space-lg); font-family: var(--font-body); }
    .title { margin: 0; font-size: var(--type-md); font-weight: 500; }
    .body, .note { margin: var(--space-2xs) 0 0; font-size: var(--type-sm); color: var(--color-ink-2); }
    .note { margin: 0; }
    .problem { font-size: var(--type-sm); color: var(--color-negative); }
  `],
  template: `
    @if (unconfirmed()) {
      <section class="banner hm hm-on-light" aria-label="Confirm your email address">
        <p class="title">Confirm your email address</p>
        @if (required()) {
          <p class="body">
            We sent a link to {{ email() }}. Until you open it, you cannot create projects or Shorts.
          </p>
        } @else {
          <p class="body">
            Confirm {{ email() }} to get emails about your videos. Nothing else changes if you do not.
          </p>
        }
        <div class="mt-3 flex flex-wrap items-center gap-3">
          <button
            type="button"
            class="ui-btn ui-btn--primary"
            [disabled]="send() === 'sending' || send() === 'sent'"
            (click)="resend()"
          >
            {{ send() === 'sending' ? 'Sending' : send() === 'sent' ? 'Email sent' : 'Send the email again' }}
          </button>
          @if (send() === 'sent') {
            <span class="note" role="status">Check your inbox, and your spam folder.</span>
          }
          @if (send() === 'failed') {
            <span class="problem" role="alert">{{ error() }}</span>
          }
        </div>
      </section>
    }
  `,
})
export class EmailConfirmBannerComponent implements OnInit {
  private readonly api = inject(ApiService);
  private readonly auth = inject(AuthService);

  readonly unconfirmed = signal(false);
  readonly required = signal(false);
  readonly email = signal('');
  readonly send = signal<SendState>('idle');
  readonly error = signal('');

  ngOnInit(): void {
    this.api
      .get<UserProfile>('/auth/me', undefined, true, new HttpContext().set(SUPPRESS_GLOBAL_ERROR, true))
      .subscribe({
        next: (profile) => {
          // Only on an explicit false, and only when the API says confirmation is on:
          // an older API that sends neither field must not show this.
          this.unconfirmed.set(profile.emailVerified === false && profile.canConfirmEmail === true);
          this.required.set(profile.emailConfirmationRequired === true);
          this.email.set(profile.email);
        },
        error: () => undefined,
      });
  }

  resend(): void {
    if (this.send() === 'sending' || this.send() === 'sent') return;
    this.send.set('sending');
    this.error.set('');
    this.auth.resendVerification().subscribe({
      next: () => this.send.set('sent'),
      error: (err) => {
        this.error.set(err?.error?.message || 'Could not send the email. Please try again in a few minutes.');
        this.send.set('failed');
      },
    });
  }
}
