import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { provideRouter, Router } from '@angular/router';
import { AuthService } from './auth.service';
import { environment } from '../../../environments/environment';
import { AuthTokens } from '../../shared';

describe('AuthService', () => {
  let service: AuthService;
  let httpTesting: HttpTestingController;
  let router: Router;

  beforeEach(() => {
    localStorage.clear();
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
      ],
    });

    service = TestBed.inject(AuthService);
    httpTesting = TestBed.inject(HttpTestingController);
    router = TestBed.inject(Router);
  });

  afterEach(() => {
    httpTesting.verify();
    localStorage.clear();
  });

  it('should store access token via setTokens', () => {
    const tokens: AuthTokens = { accessToken: 'test-token', expiresIn: 3600 };
    service.setTokens(tokens);
    expect(service.getAccessToken()).toBe('test-token');
  });

  it('should return the stored token from getAccessToken', () => {
    expect(service.getAccessToken()).toBeNull();

    service.setTokens({ accessToken: 'my-token', expiresIn: 3600 });
    expect(service.getAccessToken()).toBe('my-token');
  });

  it('should return true from isAuthenticated when token exists', () => {
    service.setTokens({ accessToken: 'token', expiresIn: 3600 });
    expect(service.isAuthenticated()).toBeTrue();
  });

  it('should return false from isAuthenticated when no token', () => {
    expect(service.isAuthenticated()).toBeFalse();
  });

  const MARKER = 'indifferent.session';
  const LEGACY = 'indifferent.refreshToken';
  const tokens = (at = 'new-at'): AuthTokens => ({ accessToken: at, expiresIn: 3600 });

  it('never stores the refresh token: the cookie holds it', () => {
    service.setTokens({ accessToken: 'at', refreshToken: 'secret-rt', expiresIn: 3600 } as AuthTokens);
    const everything = JSON.stringify(Object.entries(localStorage)) + JSON.stringify(Object.entries(sessionStorage));
    expect(everything).not.toContain('secret-rt');
    expect(localStorage.getItem(LEGACY)).toBeNull();
  });

  it('keeps a changing marker, not a secret, to know a sign-in exists', () => {
    service.setTokens(tokens());
    const first = localStorage.getItem(MARKER);
    expect(first).toBeTruthy();
    service.setTokens(tokens());
    expect(localStorage.getItem(MARKER)).not.toBe(first);
  });

  it('refreshes with the cookie: an empty body and credentials', () => {
    service.setTokens(tokens('old-at'));
    const mockTokens = tokens('new-at');
    service.refreshToken().subscribe((t) => {
      expect(t).toEqual(mockTokens);
      expect(service.getAccessToken()).toBe('new-at');
    });
    const req = httpTesting.expectOne(`${environment.apiUrl}/auth/refresh`);
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual({});
    expect(req.request.withCredentials).toBeTrue();
    req.flush(mockTokens);
  });

  it('deletes a refresh token an older version left behind, and does not send it', () => {
    localStorage.setItem(LEGACY, 'old-version-rt');
    service.initialize().subscribe();
    expect(localStorage.getItem(LEGACY)).toBeNull();
    httpTesting.expectNone(`${environment.apiUrl}/auth/refresh`);
  });

  it('fails fast without an HTTP call when this browser has no sign-in', () => {
    let failed = false;
    service.refreshToken().subscribe({ error: () => (failed = true) });
    expect(failed).toBeTrue();
    httpTesting.expectNone(`${environment.apiUrl}/auth/refresh`);
  });

  it('restores the session on initialize when the marker says a sign-in exists', () => {
    localStorage.setItem(MARKER, '1');
    let done = false;
    service.initialize().subscribe(() => (done = true));
    const req = httpTesting.expectOne(`${environment.apiUrl}/auth/refresh`);
    expect(req.request.body).toEqual({});
    req.flush(tokens());
    expect(done).toBeTrue();
    expect(service.getAccessToken()).toBe('new-at');
  });

  it('resolves null on initialize without an HTTP call when no session exists', () => {
    let result: AuthTokens | null | undefined;
    service.initialize().subscribe((t) => (result = t));
    expect(result).toBeNull();
    httpTesting.expectNone(`${environment.apiUrl}/auth/refresh`);
  });

  it('drops the marker when the restore fails, so the next visit does not call again', () => {
    localStorage.setItem(MARKER, '1');
    service.initialize().subscribe();
    httpTesting.expectOne(`${environment.apiUrl}/auth/refresh`).flush('no', { status: 401, statusText: 'Unauthorized' });
    expect(localStorage.getItem(MARKER)).toBeNull();
    expect(service.getAccessToken()).toBeNull();
  });

  it('signup and password login send credentials so the cookie is set', () => {
    service.signup('a@b.c', 'N', 'pw-long-enough').subscribe();
    const s = httpTesting.expectOne(`${environment.apiUrl}/auth/signup`);
    expect(s.request.withCredentials).toBeTrue();
    s.flush(tokens());
    service.loginWithPassword('a@b.c', 'pw-long-enough').subscribe();
    const l = httpTesting.expectOne(`${environment.apiUrl}/auth/login`);
    expect(l.request.withCredentials).toBeTrue();
    l.flush(tokens());
  });

  it('clears the local session and navigates to /login on logout, asking the server to end the cookie session', () => {
    spyOn(router, 'navigate');
    service.setTokens(tokens('token'));
    service.logout();
    const req = httpTesting.expectOne(`${environment.apiUrl}/auth/logout`);
    expect(req.request.body).toEqual({});
    expect(req.request.withCredentials).toBeTrue();
    req.flush(null);
    expect(service.getAccessToken()).toBeNull();
    expect(localStorage.getItem(MARKER)).toBeNull();
    expect(router.navigate).toHaveBeenCalledWith(['/login']);
  });

  it('shares one in-flight refresh between concurrent callers (singleflight)', () => {
    service.setTokens(tokens('old-at'));
    const mockTokens = tokens('new-at');
    const seen: AuthTokens[] = [];
    service.refreshToken().subscribe((t) => seen.push(t));
    service.refreshToken().subscribe((t) => seen.push(t));
    httpTesting.expectOne(`${environment.apiUrl}/auth/refresh`).flush(mockTokens);
    expect(seen).toEqual([mockTokens, mockTokens]);
  });

  it('retries once after a conflict when another tab rotated meanwhile (marker changed)', () => {
    localStorage.setItem(MARKER, 'before');
    const mockTokens = tokens('new-at');
    let resolved: unknown = null;
    service.refreshToken().subscribe((t) => (resolved = t));
    const first = httpTesting.expectOne(`${environment.apiUrl}/auth/refresh`);
    // Another tab refreshed and stored its own marker while this request was out.
    localStorage.setItem(MARKER, 'after-other-tab');
    first.flush('conflict', { status: 401, statusText: 'Unauthorized' });
    const second = httpTesting.expectOne(`${environment.apiUrl}/auth/refresh`);
    second.flush(mockTokens);
    expect(resolved).toEqual(mockTokens);
  });

  it('does not retry a failure when nothing changed under it (a real expiry or reuse)', () => {
    localStorage.setItem(MARKER, 'same');
    let failed = false;
    service.refreshToken().subscribe({ error: () => (failed = true) });
    httpTesting.expectOne(`${environment.apiUrl}/auth/refresh`).flush('no', { status: 401, statusText: 'Unauthorized' });
    expect(failed).toBeTrue();
    httpTesting.expectNone(`${environment.apiUrl}/auth/refresh`);
  });

  describe('logoutAll', () => {
    it('asks the server, then clears the local session and goes to the login page', () => {
      service.setTokens({ accessToken: 'at', expiresIn: 3600 });
      const navigate = spyOn(router, 'navigate').and.resolveTo(true);
      service.logoutAll().subscribe();
      // Nothing is cleared until the server has answered.
      expect(service.getAccessToken()).toBe('at');
      httpTesting.expectOne(`${environment.apiUrl}/auth/logout-all`).flush(null, { status: 204, statusText: 'No Content' });
      expect(service.getAccessToken()).toBeNull();
      expect(localStorage.getItem(MARKER)).toBeNull();
      expect(navigate).toHaveBeenCalledWith(['/login']);
    });

    it('keeps the session when the server fails, so the user can try again', () => {
      service.setTokens({ accessToken: 'at', expiresIn: 3600 });
      const navigate = spyOn(router, 'navigate').and.resolveTo(true);
      let failed = false;
      service.logoutAll().subscribe({ error: () => (failed = true) });
      httpTesting.expectOne(`${environment.apiUrl}/auth/logout-all`).flush('boom', { status: 500, statusText: 'Server Error' });
      expect(failed).toBeTrue();
      expect(service.getAccessToken()).toBe('at');
      expect(navigate).not.toHaveBeenCalled();
    });
  });

  describe('email confirmation', () => {
    it('verifyEmail posts the token and nothing else', () => {
      let done = false;
      service.verifyEmail('tok').subscribe(() => (done = true));
      const req = httpTesting.expectOne(`${environment.apiUrl}/auth/verify-email`);
      expect(req.request.method).toBe('POST');
      expect(req.request.body).toEqual({ token: 'tok' });
      req.flush(null, { status: 204, statusText: 'No Content' });
      expect(done).toBeTrue();
    });

    it('resendVerification posts an empty body: the address is the account\'s own', () => {
      service.resendVerification().subscribe();
      const req = httpTesting.expectOne(`${environment.apiUrl}/auth/resend-verification`);
      expect(req.request.method).toBe('POST');
      expect(req.request.body).toEqual({});
      req.flush(null, { status: 204, statusText: 'No Content' });
    });
  });
});
