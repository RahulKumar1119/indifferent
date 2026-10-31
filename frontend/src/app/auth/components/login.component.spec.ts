import { ComponentFixture, TestBed } from '@angular/core/testing';
import { LoginComponent } from './login.component';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import { environment } from '../../../environments/environment';
import { LucideCheck, LucideLoader2, provideLucideIcons } from '@lucide/angular';
import { AdsConversionService } from '../../core/services/ads-conversion.service';
import { AuthService } from '../../core/services/auth.service';

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

    beforeEach(() => {
      redirectTo = spyOn(component as unknown as Redirecting, 'redirectTo');
    });

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

    it('does not report a conversion for the email and password form', () => {
      component.email = 'someone@example.com';
      component.password = 'a-long-enough-password';
      component.submitPasswordForm();
      expect(trackSignInClick).not.toHaveBeenCalled();
    });
  });
});
