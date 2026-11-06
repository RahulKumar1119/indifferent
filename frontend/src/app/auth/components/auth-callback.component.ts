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
  template: `
    <div class="flex min-h-screen items-center justify-center bg-gray-50">
      <mat-spinner diameter="48"></mat-spinner>
    </div>
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
