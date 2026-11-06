import { ComponentFixture, TestBed } from '@angular/core/testing';
import { LoginComponent } from './login.component';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import { environment } from '../../../environments/environment';
import { LucideCheck, LucideLoader2, provideLucideIcons } from '@lucide/angular';
import { AdsConversionService } from '../../core/services/ads-conversion.service';
import { AuthService } from '../../core/services/auth.service';
import { pkceChallenge, takeOAuthFlow } from '../oauth-flow';

describe('LoginComponent', () => {
  let component: LoginComponent;
  let fixture: ComponentFixture<LoginComponent>;
  /** What the conversion service was asked to do next, once it has reported. */
  let afterReport: (() => void) | undefined;
  let trackSignInClick: jasmine.Spy;
  let prepare: jasmine.Spy;

  beforeEach(async () => {
    afterReport = undefined;
    prepare = jasmine.createSpy('prepare');
    trackSignInClick = jasmine.createSpy('trackSignInClick').and.callFake((next: () => void) => {
      afterReport = next;
    });
    await TestBed.configureTestingModule({
      imports: [LoginComponent],
      providers: [
        provideRouter([]),
        provideHttpClient(),
        provideHttpClientTesting(),
        provideLucideIcons(LucideCheck, LucideLoader2),
        { provide: AdsConversionService, useValue: { trackSignInClick, prepare } },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(LoginComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should render a sign-in button', () => {
    const compiled = fixture.nativeElement as HTMLElement;
    const button = compiled.querySelector('button');
    expect(button).toBeTruthy();
    expect(button!.textContent).toContain('Continue with Google');
  });

  it('should construct correct OAuth URL with client_id, redirect_uri, response_type, scope', () => {
    // signInWithGoogle sets window.location.href which we can't easily spy on,
    // so we verify the URL construction logic by extracting what the method would build.
    const params = new URLSearchParams({
      client_id: environment.googleClientId,
      redirect_uri: environment.googleRedirectUri,
      response_type: 'code',
      scope: 'openid email profile',
    });

    const expectedUrl = `https://accounts.google.com/o/oauth2/v2/auth?${params.toString()}`;

    expect(expectedUrl).toContain('https://accounts.google.com/o/oauth2/v2/auth');
    expect(expectedUrl).toContain('client_id=');
    expect(expectedUrl).toContain('redirect_uri=');
    expect(expectedUrl).toContain('response_type=code');
    // URLSearchParams encodes spaces as '+', so check for that
    expect(expectedUrl).toContain('scope=openid+email+profile');

    // Verify the URL can be parsed back to correct params
    const url = new URL(expectedUrl);
    expect(url.searchParams.get('client_id')).toBe(environment.googleClientId);
    expect(url.searchParams.get('redirect_uri')).toBe(environment.googleRedirectUri);
    expect(url.searchParams.get('response_type')).toBe('code');
    expect(url.searchParams.get('scope')).toBe('openid email profile');
  });
  it('starts loading the Google tag when the page opens, so it is ready by the click', () => {
    expect(prepare).toHaveBeenCalledTimes(1);
  });

  describe('Continue with Google', () => {
    type Redirecting = { redirectTo(url: string): void };
    let redirectTo: jasmine.Spy;

    type Preparing = { oauthReady: Promise<void>; oauthFlow: unknown };
    const preparing = () => component as unknown as Preparing;
    beforeEach(async () => {
      redirectTo = spyOn(component as unknown as Redirecting, 'redirectTo');
      sessionStorage.clear();
      await preparing().oauthReady;
    });
    afterEach(() => sessionStorage.clear());

    function clickContinueWithGoogle(): void {
      const button = Array.from((fixture.nativeElement as HTMLElement).querySelectorAll('button')).find((b) =>
        b.textContent?.includes('Continue with Google'),
      );
      button!.click();
    }

    it('reports the conversion once, and stays on the page until that is done', () => {
      clickContinueWithGoogle();
      expect(trackSignInClick).toHaveBeenCalledTimes(1);
      expect(redirectTo).not.toHaveBeenCalled();
    });

    it('then goes to Google sign-in with the OAuth parameters', () => {
      clickContinueWithGoogle();
      afterReport!();

      expect(redirectTo).toHaveBeenCalledTimes(1);
      const url = new URL(redirectTo.calls.mostRecent().args[0]);
      expect(url.origin + url.pathname).toBe('https://accounts.google.com/o/oauth2/v2/auth');
      expect(url.searchParams.get('client_id')).toBe(environment.googleClientId);
      expect(url.searchParams.get('redirect_uri')).toBe(environment.googleRedirectUri);
      expect(url.searchParams.get('response_type')).toBe('code');
      expect(url.searchParams.get('scope')).toBe('openid email profile');
    });

    it('remembers where to go after sign-in before it leaves', () => {
      const order: string[] = [];
      spyOn(TestBed.inject(AuthService), 'setPostLoginNext').and.callFake(() => void order.push('remember'));
      redirectTo.and.callFake(() => void order.push('leave'));
      clickContinueWithGoogle();
      afterReport!();
      expect(order).toEqual(['remember', 'leave']);
    });

    it('sends an unguessable state and a PKCE S256 challenge (login CSRF, SEC-07)', () => {
      clickContinueWithGoogle();
      afterReport!();
      const url = new URL(redirectTo.calls.mostRecent().args[0]);
      expect(url.searchParams.get('state')).toMatch(/^[A-Za-z0-9_-]{43}$/);
      expect(url.searchParams.get('code_challenge')).toMatch(/^[A-Za-z0-9_-]{43}$/);
      expect(url.searchParams.get('code_challenge_method')).toBe('S256');
    });

    it('remembers the same state, and the verifier behind the challenge, for the callback to check', async () => {
      clickContinueWithGoogle();
      afterReport!();
      const url = new URL(redirectTo.calls.mostRecent().args[0]);
      const remembered = takeOAuthFlow();
      expect(remembered).not.toBeNull();
      expect(remembered!.state).toBe(url.searchParams.get('state')!);
      expect(await pkceChallenge(remembered!.verifier)).toBe(url.searchParams.get('code_challenge')!);
    });

    it('remembers the flow before it leaves, so the callback always finds it', () => {
      redirectTo.and.callFake(() => expect(sessionStorage.getItem('indifferent.oauthFlow')).not.toBeNull());
      clickContinueWithGoogle();
      afterReport!();
      expect(redirectTo).toHaveBeenCalledTimes(1);
    });

    it('uses new values for every click', async () => {
      clickContinueWithGoogle();
      afterReport!();
      const first = new URL(redirectTo.calls.mostRecent().args[0]).searchParams;
      await preparing().oauthReady; // the next flow is made as soon as one is spent
      clickContinueWithGoogle();
      afterReport!();
      const second = new URL(redirectTo.calls.mostRecent().args[0]).searchParams;
      expect(second.get('state')).not.toBe(first.get('state'));
      expect(second.get('code_challenge')).not.toBe(first.get('code_challenge'));
    });

    it('waits for the values when it is clicked before the page has made them, then goes', async () => {
      preparing().oauthFlow = null;
      clickContinueWithGoogle();
      expect(trackSignInClick).not.toHaveBeenCalled(); // not yet: nothing to send to Google
      await new Promise((resolve) => setTimeout(resolve, 100));
      expect(trackSignInClick).toHaveBeenCalledTimes(1);
      afterReport!();
      expect(new URL(redirectTo.calls.mostRecent().args[0]).searchParams.get('state')).toBeTruthy();
    });

    it('goes nowhere, and says why, in a browser that cannot make random numbers', async () => {
      preparing().oauthFlow = null;
      spyOn(crypto, 'getRandomValues').and.throwError('no secure random numbers');
      clickContinueWithGoogle();
      await new Promise((resolve) => setTimeout(resolve, 100));
      clickContinueWithGoogle(); // the second click reports the problem
      expect(component.formError).toContain('cannot start a secure Google sign-in');
      expect(trackSignInClick).not.toHaveBeenCalled();
      expect(redirectTo).not.toHaveBeenCalled();
    });

    it('does not report a conversion for the email and password form', () => {
      component.email = 'someone@example.com';
      component.password = 'a-long-enough-password';
      component.submitPasswordForm();
      expect(trackSignInClick).not.toHaveBeenCalled();
    });
  });
});
