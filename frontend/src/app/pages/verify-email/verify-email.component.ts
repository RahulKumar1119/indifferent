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
  styles: [`
    :host { display: block; }
    main { min-height: 100dvh; display: flex; align-items: center; justify-content: center; padding: var(--space-xl) var(--gutter); background: var(--color-paper); color: var(--color-ink); font-family: var(--font-body); }
    .panel { width: min(28rem, 100%); }
    .brand { margin: 0; font-size: var(--type-sm); color: var(--color-muted); }
    h1 { margin: var(--space-sm) 0 0; font-family: var(--font-display); font-weight: 500; font-size: var(--type-3xl); line-height: 1.08; letter-spacing: var(--tracking-display); text-wrap: balance; }
    .text { margin: var(--space-md) 0 0; color: var(--color-ink-2); }
    .action { margin-top: var(--space-xl); }
  `],
  template: `
    <main class="hm hm-on-light">
      <div class="panel">
        <p class="brand">Indifferent</p>
        @switch (state()) {
          @case ('working') {
            <h1 role="status">Confirming your email address</h1>
            <p class="text">One moment.</p>
          }
          @case ('done') {
            <h1>Your email address is confirmed</h1>
            <p class="text">You can create projects and Shorts now.</p>
            <a [routerLink]="signedIn ? '/dashboard' : '/login'" class="ui-btn ui-btn--primary action">
              {{ signedIn ? 'Go to your dashboard' : 'Sign in' }}
            </a>
          }
          @case ('failed') {
            <h1>This link did not work</h1>
            <p class="text" role="alert">{{ message() }}</p>
            <a [routerLink]="signedIn ? '/account' : '/login'" class="ui-btn ui-btn--primary action">
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
