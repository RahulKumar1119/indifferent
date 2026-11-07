import { Component, DOCUMENT, OnInit, inject, signal } from '@angular/core';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { HttpErrorResponse } from '@angular/common/http';
import { AuthService } from '../../core/services/auth.service';
import { ROBOTS_NOINDEX, setRobots } from '../../shared/seo';

type State = 'working' | 'done' | 'failed';

/**
 * The page the confirmation email links to (/verify-email?token=...). It spends
 * the single-use token with one POST and says what happened. The token is
 * removed from the address bar first thing, so it does not stay in the browser
 * history or leak through a screenshot or a shared tab.
 */
@Component({
  selector: 'app-verify-email',
  standalone: true,
  imports: [RouterLink],
  template: `
    <main class="min-h-[100dvh] bg-[#FAF7F2] text-[#1A1714] flex items-center justify-center px-5">
      <div class="w-full max-w-md rounded-2xl border border-black/10 bg-white p-8">
        <p class="text-[13px] tracking-wide text-[#6B6560]">Indifferent</p>
        @switch (state()) {
          @case ('working') {
            <h1 class="mt-3 text-2xl font-medium" role="status">Confirming your email address</h1>
            <p class="mt-3 text-[15px] text-[#3E3A33]">One moment.</p>
          }
          @case ('done') {
            <h1 class="mt-3 text-2xl font-medium">Your email address is confirmed</h1>
            <p class="mt-3 text-[15px] text-[#3E3A33]">You can create projects and Shorts now.</p>
            <a [routerLink]="signedIn ? '/dashboard' : '/login'" class="mt-6 inline-flex rounded-full bg-[#1A1714] px-6 py-3 text-[15px] font-semibold text-[#FAF7F2]">
              {{ signedIn ? 'Go to your dashboard' : 'Sign in' }}
            </a>
          }
          @case ('failed') {
            <h1 class="mt-3 text-2xl font-medium">This link did not work</h1>
            <p class="mt-3 text-[15px] text-[#3E3A33]" role="alert">{{ message() }}</p>
            <a [routerLink]="signedIn ? '/account' : '/login'" class="mt-6 inline-flex rounded-full bg-[#1A1714] px-6 py-3 text-[15px] font-semibold text-[#FAF7F2]">
              {{ signedIn ? 'Open your account page' : 'Sign in' }}
            </a>
          }
        }
      </div>
    </main>
  `,
})
export class VerifyEmailComponent implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly auth = inject(AuthService);
  private readonly document = inject(DOCUMENT);

  readonly state = signal<State>('working');
  readonly message = signal('');
  signedIn = false;

  ngOnInit(): void {
    this.document.title = 'Confirm your email | Indifferent';
    setRobots(this.document, ROBOTS_NOINDEX);
    this.signedIn = this.auth.isAuthenticated();

    const token = this.route.snapshot.queryParamMap.get('token');
    // Take the token out of the address first, whatever happens next.
    this.router.navigate([], { queryParams: {}, replaceUrl: true });
    if (!token) {
      this.fail('This link is incomplete. Open the whole link from the email, or ask for a new one on your account page.');
      return;
    }
    this.auth.verifyEmail(token).subscribe({
      next: () => this.state.set('done'),
      error: (err: HttpErrorResponse) =>
        this.fail(err?.error?.message || 'Could not confirm your email address. Please try again.'),
    });
  }

  private fail(message: string): void {
    this.message.set(message);
    this.state.set('failed');
  }
}
