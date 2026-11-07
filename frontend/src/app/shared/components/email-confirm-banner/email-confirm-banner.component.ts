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
  template: `
    @if (unconfirmed()) {
      <section class="rounded-xl border border-[#D96C3D]/40 bg-[#FFF4EC] px-5 py-4 text-[#1A1714]" aria-label="Confirm your email address">
        <p class="text-[15px] font-medium">Confirm your email address</p>
        <p class="mt-1 text-[14px] text-[#3E3A33]">
          We sent a link to {{ email() }}. Until you open it, you cannot create projects or Shorts.
        </p>
        <div class="mt-3 flex flex-wrap items-center gap-3">
          <button
            type="button"
            class="rounded-full bg-[#1A1714] px-5 py-2 text-[14px] font-semibold text-[#FAF7F2] disabled:opacity-60"
            [disabled]="send() === 'sending' || send() === 'sent'"
            (click)="resend()"
          >
            {{ send() === 'sending' ? 'Sending' : send() === 'sent' ? 'Email sent' : 'Send the email again' }}
          </button>
          @if (send() === 'sent') {
            <span class="text-[14px] text-[#3E3A33]" role="status">Check your inbox, and your spam folder.</span>
          }
          @if (send() === 'failed') {
            <span class="text-[14px] text-red-700" role="alert">{{ error() }}</span>
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
  readonly email = signal('');
  readonly send = signal<SendState>('idle');
  readonly error = signal('');

  ngOnInit(): void {
    this.api
      .get<UserProfile>('/auth/me', undefined, true, new HttpContext().set(SUPPRESS_GLOBAL_ERROR, true))
      .subscribe({
        next: (profile) => {
          // Only an explicit false: an older API that does not send the field must not show this.
          this.unconfirmed.set(profile.emailVerified === false);
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
