import { Component, OnInit, inject } from '@angular/core';
import { CommonModule, DOCUMENT } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { environment } from '../../../environments/environment';
import { AdsConversionService } from '../../core/services/ads-conversion.service';
import { AuthService, DEFAULT_POST_AUTH_PATH, sanitizeNextPath } from '../../core/services/auth.service';
import { ROBOTS_NOINDEX, setPageSeo, setRobots } from '../../shared/seo';
import { OAuthFlow, createOAuthFlow, rememberOAuthFlow } from '../oauth-flow';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [CommonModule, FormsModule, RouterLink, LucideDynamicIcon],
  styles: [`
    :host { display: block; }
    .auth { min-height: 100dvh; display: grid; grid-template-columns: minmax(0, 1fr); font-family: var(--font-body); font-size: var(--type-md); line-height: 1.55; -webkit-font-smoothing: antialiased; }
    @media (min-width: 64rem) { .auth { grid-template-columns: minmax(0, 52fr) minmax(0, 48fr); } }
    h1, h2, p, ul { margin: 0; }

    /* Left: a solid dark brand panel. Under 64rem it is a short band above the form. */
    .brand-panel { background: var(--color-paper); color: var(--color-ink); padding: var(--space-xl) var(--gutter); display: flex; flex-direction: column; gap: var(--space-xl); }
    @media (min-width: 64rem) { .brand-panel { padding: var(--space-2xl); justify-content: space-between; min-height: 100dvh; } }
    /* On very wide screens the two halves keep to a 1440px frame instead of reaching the edges (#256). */
    @media (min-width: 90rem) { .brand-panel { padding-inline-start: max(var(--space-2xl), calc((100vw - 1440px) / 2)); } .form-panel { padding-inline-end: max(var(--gutter), calc((100vw - 1440px) / 2)); } }
    .brand { display: inline-flex; align-items: center; min-height: 44px; width: max-content; font-size: var(--type-lg); font-weight: 600; letter-spacing: -0.01em; color: var(--color-ink); text-decoration: none; }
    .brand .dot { color: var(--color-accent-text); }
    .brand:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: 2px; }
    h1 { font-family: var(--font-display); font-weight: 500; font-size: var(--type-display-s); line-height: 1.04; letter-spacing: var(--tracking-display); overflow-wrap: break-word; min-width: 0; text-wrap: balance; max-width: 14ch; }
    .lede { margin-top: var(--space-md); font-size: var(--type-lead); line-height: 1.45; color: var(--color-ink-2); max-width: 36ch; }
    .legal-note { font-size: var(--type-sm); color: var(--color-muted); }

    /* Right: one form surface on paper */
    .form-panel { background: var(--color-paper); color: var(--color-ink); padding: var(--space-2xl) var(--gutter); display: flex; align-items: center; justify-content: center; }
    .form-inner { width: min(28rem, 100%); }
    h2 { font-family: var(--font-display); font-weight: 500; font-size: var(--type-2xl); line-height: 1.1; letter-spacing: var(--tracking-display); }
    .sub { margin-top: var(--space-xs); color: var(--color-ink-2); }
    .btn-block { width: 100%; white-space: nowrap; }
    .google-btn { margin-top: var(--space-lg); }
    .google-btn svg { flex-shrink: 0; }
    .divider { display: flex; align-items: center; gap: var(--space-md); margin: var(--space-lg) 0; font-size: var(--type-sm); color: var(--color-muted); }
    .divider::before, .divider::after { content: ''; flex: 1; height: 1px; background: var(--color-rule); }
    form { display: grid; gap: var(--space-md); }
    .field { display: grid; gap: var(--space-2xs); }
    .hint { font-size: var(--type-xs); color: var(--color-muted); }
    .alert { margin-bottom: var(--space-md); padding: var(--space-sm) var(--space-md); border: 1px solid color-mix(in oklab, var(--color-negative) 45%, transparent); border-radius: var(--radius-control); background: color-mix(in oklab, var(--color-negative) 8%, var(--color-paper)); color: var(--color-ink); font-size: var(--type-sm); }
    .toggle { margin-top: var(--space-md); text-align: center; font-size: var(--type-sm); color: var(--color-ink-2); }
    .toggle a, .terms a { color: var(--color-accent-text); text-decoration: underline; text-underline-offset: 0.2em; font-weight: 600; }
    .toggle a:hover, .terms a:hover { color: var(--color-ink); }
    .toggle a, .back, .terms a { display: inline-flex; align-items: center; min-height: 44px; }
    .toggle a:focus-visible, .terms a:focus-visible, .back:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: 2px; }
    .perks { list-style: none; padding: 0; margin-top: var(--space-lg); display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-xs) var(--space-md); font-size: var(--type-sm); color: var(--color-muted); }
    .perks li { padding-left: var(--space-md); position: relative; }
    .perks li::before { content: ''; position: absolute; left: 0; top: 0.6em; width: 8px; height: 2px; background: var(--color-accent); }
    .terms { margin-top: var(--space-xl); font-size: var(--type-sm); color: var(--color-muted); }
    /* In-sentence links stay inline; the padding makes the tap target 44px tall without moving the text (#256). */
    .terms a { font-weight: 400; min-height: 0; display: inline; padding-block: 11px; }
    .back { margin-top: var(--space-xs); font-size: var(--type-sm); color: var(--color-muted); text-decoration: none; }
    .back:hover { color: var(--color-ink); text-decoration: underline; }
  `],
  template: `
    <div class="auth hm">
      <!-- Brand panel -->
      <div class="brand-panel hm-on-dark">
        <a routerLink="/" class="brand">Indifferent<span class="dot">.</span></a>
        <div>
          <h1>Enter the cutting room.</h1>
          <p class="lede">Sign in to turn stories, quizzes and long videos into short videos.</p>
        </div>
        <p class="legal-note">© {{ currentYear }} Indifferent. Architecture of video.</p>
      </div>

      <!-- Form -->
      <main class="form-panel hm-on-light">
        <div class="form-inner">
          <h2>{{ isSignupMode ? 'Create your account.' : 'Sign in to your studio.' }}</h2>
          <p class="sub">Continue with Google, or use your email. No credit card.</p>

          <button type="button" (click)="signInWithGoogle()" class="ui-btn ui-btn--secondary btn-block google-btn">
            <svg width="20" height="20" viewBox="0 0 24 24" aria-hidden="true">
              <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4"/>
              <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/>
              <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/>
              <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/>
            </svg>
            Continue with Google
          </button>

          <p class="divider">or continue with email</p>

          <div id="auth-error" role="alert">
            @if (formError) {
              <p class="alert">{{ formError }}</p>
            }
          </div>

          <form (ngSubmit)="submitPasswordForm()" novalidate>
            @if (isSignupMode) {
              <div class="field">
                <label for="auth-name" class="ui-label">Your name</label>
                <input id="auth-name" [(ngModel)]="name" name="name" type="text" class="ui-field" placeholder="Your name" autocomplete="name">
              </div>
            }
            <div class="field">
              <label for="auth-email" class="ui-label">Email address</label>
              <input id="auth-email" [(ngModel)]="email" name="email" type="email" required class="ui-field" placeholder="Email address" autocomplete="email"
                [attr.aria-describedby]="formError ? 'auth-error' : null">
            </div>
            <div class="field">
              <label for="auth-password" class="ui-label">Password</label>
              <input id="auth-password" [(ngModel)]="password" name="password" type="password" required minlength="8" class="ui-field" placeholder="Password (8+ characters)"
                [attr.autocomplete]="isSignupMode ? 'new-password' : 'current-password'"
                [attr.aria-describedby]="formError ? 'auth-error auth-password-hint' : 'auth-password-hint'">
              <span id="auth-password-hint" class="hint">8 characters or more.</span>
            </div>
            <button type="submit" [disabled]="submitting" [attr.aria-busy]="submitting" class="ui-btn ui-btn--primary btn-block">
              @if (submitting) {
                <svg lucideIcon="loader-2" [size]="16" class="animate-spin" aria-hidden="true"></svg>
              }
              {{ isSignupMode ? 'Create account' : 'Sign in' }}
            </button>
          </form>

          <p class="toggle">
            @if (isSignupMode) {
              Already have an account?
              <a [routerLink]="[]" [queryParams]="{ mode: null }" queryParamsHandling="merge">Sign in</a>
            } @else {
              New here?
              <a [routerLink]="[]" [queryParams]="{ mode: 'signup' }">Create an account</a>
            }
          </p>

          <ul class="perks">
            <li>Free to use</li>
            <li>No credit card</li>
            <li>5 AI voices</li>
            <li>HD 1080p MP4</li>
          </ul>

          <p class="terms">
            By continuing, you agree to our
            <a routerLink="/terms">Terms of Service</a>
            and
            <a routerLink="/privacy">Privacy Policy</a>
          </p>
          <a routerLink="/" class="back">Back to home</a>
        </div>
      </main>
    </div>
  `,
})
export class LoginComponent implements OnInit {
  currentYear = new Date().getFullYear();

  private readonly authService = inject(AuthService);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);
  private readonly document = inject(DOCUMENT);
  private readonly ads = inject(AdsConversionService);

  isSignupMode = false;
  email = '';
  name = '';
  password = '';
  submitting = false;
  formError = '';
  private postAuthPath = '/new';
  /** The state and PKCE values for the next Google sign-in, made when the page opens. */
  private oauthFlow: OAuthFlow | null = null;
  /** True when this browser cannot make them at all (no secure random numbers). */
  private oauthUnavailable = false;
  /** Settles when the values for the next sign-in are ready. */
  protected oauthReady: Promise<void> = Promise.resolve();

  ngOnInit(): void {
    // Already signed in (back button, bookmark): don't show the form —
    // re-login would clobber valid tokens.
    if (this.authService.isAuthenticated()) {
      this.router.navigate([DEFAULT_POST_AUTH_PATH]);
      return;
    }
    setPageSeo(this.document, {
      title: 'Start Creating: Sign In | Indifferent',
      description:
        'Sign in to Indifferent to turn TXT quiz files into narrated videos and long footage into ranked vertical shorts.',
      canonical: 'https://indifferent.fun/login',
    });
    // The sign-in form has nothing to read; keep it out of search results.
    setRobots(this.document, ROBOTS_NOINDEX);
    // The click on "Continue with Google" is a Google Ads conversion. Load the
    // Google tag now so it is ready by then instead of after a first interaction.
    this.ads.prepare();
    // The Google button must stay synchronous (the conversion report has to go
    // out before the page leaves), and PKCE needs an asynchronous hash, so the
    // values for the first sign-in are made now, while the page loads.
    this.oauthReady = this.prepareOAuthFlow();
    this.route.queryParamMap.subscribe((params) => {
      this.isSignupMode = params.get('mode') === 'signup';
      this.postAuthPath = sanitizeNextPath(params.get('next'));
      const err = params.get('error');
      this.formError =
        err === 'auth_failed'
          ? 'Sign-in failed. Please try again.'
          : err === 'missing_code'
            ? 'Sign-in was interrupted before completion. Please try again.'
            : err === 'invalid_state'
              ? 'That sign-in could not be verified. Please start again from this page.'
              : '';
    });
  }

  signInWithGoogle(): void {
    const flow = this.oauthFlow;
    if (!flow) {
      if (this.oauthUnavailable) {
        this.formError = 'This browser cannot start a secure Google sign-in. Use the email form, or another browser.';
        return;
      }
      // Clicked in the first moments after the page opened, before the values for
      // the sign-in were ready. They take a few milliseconds; wait, then go.
      void this.prepareOAuthFlow().then(() => this.signInWithGoogle());
      return;
    }
    // One flow per sign-in. This one is spent; a fresh one is made for the next
    // click (the back button can bring the user back to this page as it was).
    this.oauthFlow = null;
    rememberOAuthFlow(flow);
    const params = new URLSearchParams({
      client_id: environment.googleClientId,
      redirect_uri: environment.googleRedirectUri,
      response_type: 'code',
      scope: 'openid email profile',
      state: flow.state,
    });
    if (flow.challenge) {
      params.set('code_challenge', flow.challenge);
      params.set('code_challenge_method', 'S256');
    }

    // Remembered across the Google round-trip; the callback consumes it.
    this.authService.setPostLoginNext(this.postAuthPath);
    const url = `https://accounts.google.com/o/oauth2/v2/auth?${params.toString()}`;
    // Leaving the page would cancel the conversion request, so the redirect
    // waits until Google has confirmed it (or a short timeout passes).
    this.ads.trackSignInClick(() => this.redirectTo(url));
    this.oauthReady = this.prepareOAuthFlow();
  }

  private async prepareOAuthFlow(): Promise<void> {
    try {
      this.oauthFlow = await createOAuthFlow();
    } catch {
      this.oauthFlow = null;
      this.oauthUnavailable = true;
    }
  }

  /** Leaves the app. A method of its own so tests can observe it. */
  protected redirectTo(url: string): void {
    window.location.href = url;
  }

  submitPasswordForm(): void {
    this.formError = '';
    if (!this.email.trim() || this.password.length < 8) {
      this.formError = 'Enter a valid email and a password of 8+ characters.';
      return;
    }
    this.submitting = true;
    const request = this.isSignupMode
      ? this.authService.signup(this.email.trim(), this.name.trim(), this.password)
      : this.authService.loginWithPassword(this.email.trim(), this.password);
    request.subscribe({
      next: () => {
        this.submitting = false;
        this.router.navigateByUrl(this.postAuthPath);
      },
      error: (err) => {
        this.submitting = false;
        this.formError = err?.error?.message || (this.isSignupMode ? 'Could not create your account.' : 'Invalid email or password.');
      },
    });
  }
}
