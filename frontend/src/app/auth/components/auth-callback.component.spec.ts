import { TestBed } from '@angular/core/testing';
import { AuthCallbackComponent } from './auth-callback.component';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { provideRouter, Router } from '@angular/router';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { AuthService } from '../../core/services/auth.service';
import { environment } from '../../../environments/environment';
import { AuthTokens } from '../../shared';
import { OAUTH_FLOW_MAX_AGE_MS, rememberOAuthFlow } from '../oauth-flow';

const STATE = 'S'.repeat(43);
const VERIFIER = 'v'.repeat(64);

describe('AuthCallbackComponent', () => {
  let httpTesting: HttpTestingController;
  let router: Router;
  let authService: AuthService;

  /** Opens the callback page as Google would send the browser back to it. */
  function createComponent(
    queryParams: Record<string, string> = {},
    flow: { state: string; verifier: string } | null = { state: STATE, verifier: VERIFIER },
    rememberedAt?: number,
  ) {
    sessionStorage.clear();
    if (flow) rememberOAuthFlow({ ...flow, challenge: '' }, rememberedAt);
    TestBed.configureTestingModule({
      imports: [AuthCallbackComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
        {
          provide: ActivatedRoute,
          useValue: { snapshot: { queryParamMap: convertToParamMap(queryParams) } },
        },
      ],
    });
    httpTesting = TestBed.inject(HttpTestingController);
    router = TestBed.inject(Router);
    authService = TestBed.inject(AuthService);
    spyOn(router, 'navigate');
    spyOn(router, 'navigateByUrl');
    const fixture = TestBed.createComponent(AuthCallbackComponent);
    fixture.detectChanges();
    return fixture;
  }

  afterEach(() => {
    sessionStorage.clear();
    TestBed.resetTestingModule();
  });

  describe('a sign-in this browser started', () => {
    it('POSTs the code and the PKCE verifier to the API', () => {
      createComponent({ code: 'test-auth-code', state: STATE });
      const req = httpTesting.expectOne(`${environment.apiUrl}/auth/google/callback`);
      expect(req.request.method).toBe('POST');
      expect(req.request.body).toEqual({ code: 'test-auth-code', codeVerifier: VERIFIER });
      req.flush({ accessToken: 'at', refreshToken: 'rt', expiresIn: 3600 });
    });

    it('sends no verifier when the browser could not make one (state only)', () => {
      createComponent({ code: 'test-auth-code', state: STATE }, { state: STATE, verifier: '' });
      const req = httpTesting.expectOne(`${environment.apiUrl}/auth/google/callback`);
      expect(req.request.body).toEqual({ code: 'test-auth-code' });
      req.flush({ accessToken: 'at', refreshToken: 'rt', expiresIn: 3600 });
    });

    it('should call authService.setTokens and navigate to the post-login destination on success', () => {
      createComponent({ code: 'test-auth-code', state: STATE });
      spyOn(authService, 'setTokens');
      const mockTokens: AuthTokens = { accessToken: 'at', refreshToken: 'rt', expiresIn: 3600 };
      const req = httpTesting.expectOne(`${environment.apiUrl}/auth/google/callback`);
      req.flush(mockTokens);
      expect(authService.setTokens).toHaveBeenCalledWith(mockTokens);
      expect(router.navigateByUrl).toHaveBeenCalledWith('/new');
    });

    it('should navigate to /login with error param on API error', () => {
      createComponent({ code: 'test-auth-code', state: STATE });
      const req = httpTesting.expectOne(`${environment.apiUrl}/auth/google/callback`);
      req.flush(null, { status: 500, statusText: 'Server Error' });
      expect(router.navigate).toHaveBeenCalledWith(['/login'], { queryParams: { error: 'auth_failed' } });
    });

    it('forgets the flow, so the same callback link cannot be used twice', () => {
      createComponent({ code: 'test-auth-code', state: STATE });
      httpTesting.expectOne(`${environment.apiUrl}/auth/google/callback`).flush({ accessToken: 'a', refreshToken: 'r', expiresIn: 1 });
      expect(sessionStorage.getItem('indifferent.oauthFlow')).toBeNull();
    });
  });

  describe('a link this browser did not start (login CSRF)', () => {
    function expectRefused(): void {
      httpTesting.expectNone(`${environment.apiUrl}/auth/google/callback`);
      expect(router.navigate).toHaveBeenCalledWith(['/login'], { queryParams: { error: 'invalid_state' } });
    }

    it('refuses a code with no state at all', () => {
      createComponent({ code: 'attackers-code' });
      expectRefused();
    });

    it('refuses a code whose state is not the one this browser stored', () => {
      createComponent({ code: 'attackers-code', state: 'X'.repeat(43) });
      expectRefused();
    });

    it('refuses a code when this browser started no sign-in (the victim opens the attacker\'s link)', () => {
      createComponent({ code: 'attackers-code', state: STATE }, null);
      expectRefused();
    });

    it('refuses a sign-in that was started too long ago', () => {
      createComponent({ code: 'c', state: STATE }, { state: STATE, verifier: VERIFIER }, Date.now() - OAUTH_FLOW_MAX_AGE_MS - 1000);
      expectRefused();
    });

    it('does not let the failed attempt be retried with the right state afterwards', () => {
      createComponent({ code: 'attackers-code', state: 'X'.repeat(43) });
      expect(sessionStorage.getItem('indifferent.oauthFlow')).toBeNull();
    });
  });

  it('should redirect to /login with missing_code error when no code param', () => {
    createComponent({});
    httpTesting.expectNone(`${environment.apiUrl}/auth/google/callback`);
    expect(router.navigate).toHaveBeenCalledWith(['/login'], { queryParams: { error: 'missing_code' } });
  });

  it('still forgets the flow when Google came back with no code (the user said no)', () => {
    createComponent({ error: 'access_denied', state: STATE });
    expect(sessionStorage.getItem('indifferent.oauthFlow')).toBeNull();
  });
});
