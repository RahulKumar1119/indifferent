import { Component, OnInit, inject } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { HttpClient } from '@angular/common/http';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { environment } from '../../../environments/environment';
import { AuthService } from '../../core/services/auth.service';
import { AuthTokens } from '../../shared';
import { takeOAuthFlow } from '../oauth-flow';

@Component({
  selector: 'app-auth-callback',
  standalone: true,
  imports: [MatProgressSpinnerModule],
  styles: [`
    :host { display: block; }
    main { min-height: 100dvh; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: var(--space-md); background: var(--color-paper); color: var(--color-ink); font-family: var(--font-body); }
    p { margin: 0; font-size: var(--type-lg); color: var(--color-ink-2); }
    mat-spinner { --mdc-circular-progress-active-indicator-color: var(--color-accent); --mat-progress-spinner-active-indicator-color: var(--color-accent); }
  `],
  template: `
    <main class="hm hm-on-light" role="status" aria-live="polite">
      <mat-spinner diameter="40" aria-hidden="true"></mat-spinner>
      <p>Signing you in</p>
    </main>
  `,
})
export class AuthCallbackComponent implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly http = inject(HttpClient);
  private readonly authService = inject(AuthService);

  ngOnInit(): void {
    const params = this.route.snapshot.queryParamMap;
    const code = params.get('code');
    const state = params.get('state');
    // Forget the flow whatever happens next: this sign-in is over, and it can
    // only ever be finished once.
    const flow = takeOAuthFlow();
    if (!code) {
      this.router.navigate(['/login'], { queryParams: { error: 'missing_code' } });
      return;
    }
    // Login CSRF (SEC-07): only finish a sign-in this browser started. A link
    // built by someone else carries a code but no state we remember, and must
    // not sign this browser in to their account.
    if (!flow || !state || state !== flow.state) {
      this.router.navigate(['/login'], { queryParams: { error: 'invalid_state' } });
      return;
    }
    const body: { code: string; codeVerifier?: string } = { code };
    if (flow.verifier) {
      body.codeVerifier = flow.verifier;
    }
    this.http
      .post<AuthTokens>(`${environment.apiUrl}/auth/google/callback`, body, { withCredentials: true })
      .subscribe({
        next: (tokens) => {
          this.authService.setTokens(tokens);
          this.router.navigateByUrl(this.authService.consumePostLoginNext());
        },
        error: () => {
          this.router.navigate(['/login'], { queryParams: { error: 'auth_failed' } });
        },
      });
  }
}
