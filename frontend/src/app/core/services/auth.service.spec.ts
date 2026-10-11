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
    const tokens: AuthTokens = { accessToken: 'test-token', refreshToken: 'rt', expiresIn: 3600 };
    service.setTokens(tokens);
    expect(service.getAccessToken()).toBe('test-token');
  });

  it('should return the stored token from getAccessToken', () => {
    expect(service.getAccessToken()).toBeNull();

    service.setTokens({ accessToken: 'my-token', refreshToken: 'rt', expiresIn: 3600 });
    expect(service.getAccessToken()).toBe('my-token');
  });

  it('should return true from isAuthenticated when token exists', () => {
    service.setTokens({ accessToken: 'token', refreshToken: 'rt', expiresIn: 3600 });
    expect(service.isAuthenticated()).toBeTrue();
  });

  it('should return false from isAuthenticated when no token', () => {
    expect(service.isAuthenticated()).toBeFalse();
  });

  it('should persist the refresh token to storage via setTokens', () => {
    service.setTokens({ accessToken: 'at', refreshToken: 'stored-rt', expiresIn: 3600 });
    expect(service.getRefreshToken()).toBe('stored-rt');
  });

  it('should send the stored refresh token in the POST /auth/refresh body', () => {
    service.setTokens({ accessToken: 'old-at', refreshToken: 'stored-rt', expiresIn: 3600 });
    const mockTokens: AuthTokens = { accessToken: 'new-at', refreshToken: 'new-rt', expiresIn: 3600 };

    service.refreshToken().subscribe((tokens) => {
      expect(tokens).toEqual(mockTokens);
      expect(service.getAccessToken()).toBe('new-at');
      expect(service.getRefreshToken()).toBe('new-rt');
    });

    const req = httpTesting.expectOne(`${environment.apiUrl}/auth/refresh`);
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual({ refreshToken: 'stored-rt' });
    expect(req.request.withCredentials).toBeTrue();
    req.flush(mockTokens);
  });

  it('should fail fast without an HTTP call when no refresh token is stored', () => {
    let failed = false;
    service.refreshToken().subscribe({ error: () => (failed = true) });
    expect(failed).toBeTrue();
    httpTesting.expectNone(`${environment.apiUrl}/auth/refresh`);
  });

  it('should restore the session on initialize when a refresh token is stored', () => {
    localStorage.setItem(
      'indifferent.refreshToken',
      'stored-rt',
    );
    const mockTokens: AuthTokens = { accessToken: 'new-at', refreshToken: 'new-rt', expiresIn: 3600 };

    let done = false;
    service.initialize().subscribe(() => (done = true));

    const req = httpTesting.expectOne(`${environment.apiUrl}/auth/refresh`);
    expect(req.request.body).toEqual({ refreshToken: 'stored-rt' });
    req.flush(mockTokens);

    expect(done).toBeTrue();
    expect(service.getAccessToken()).toBe('new-at');
  });

  it('should resolve null on initialize without an HTTP call when no session exists', () => {
    let result: AuthTokens | null | undefined;
    service.initialize().subscribe((tokens) => (result = tokens));
    expect(result).toBeNull();
    httpTesting.expectNone(`${environment.apiUrl}/auth/refresh`);
  });

  it('should clear token and navigate to /login on logout', () => {
    spyOn(router, 'navigate');
    service.setTokens({ accessToken: 'token', refreshToken: 'rt', expiresIn: 3600 });

    service.logout();

    const req = httpTesting.expectOne(`${environment.apiUrl}/auth/logout`);
    expect(req.request.body).toEqual({ refreshToken: 'rt' });
    req.flush(null);

    expect(service.getAccessToken()).toBeNull();
    expect(service.getRefreshToken()).toBeNull();
    expect(router.navigate).toHaveBeenCalledWith(['/login']);
  });

  it('should share one in-flight refresh between concurrent callers (singleflight)', () => {
    service.setTokens({ accessToken: 'old-at', refreshToken: 'stored-rt', expiresIn: 3600 });
    const mockTokens: AuthTokens = { accessToken: 'new-at', refreshToken: 'new-rt', expiresIn: 3600 };
    const seen: AuthTokens[] = [];

    service.refreshToken().subscribe((t) => seen.push(t));
    service.refreshToken().subscribe((t) => seen.push(t));

    // Exactly one HTTP request for both subscribers.
    const req = httpTesting.expectOne(`${environment.apiUrl}/auth/refresh`);
    req.flush(mockTokens);

    expect(seen).toEqual([mockTokens, mockTokens]);
  });

  it('should retry once with a fresher stored token after a conflict (cross-tab recovery)', () => {
    service.setTokens({ accessToken: 'old-at', refreshToken: 'stale-rt', expiresIn: 3600 });
    const mockTokens: AuthTokens = { accessToken: 'new-at', refreshToken: 'new-rt', expiresIn: 3600 };
    let resolved: unknown = null;

    service.refreshToken().subscribe((t) => (resolved = t));

    // First attempt fails; meanwhile another tab rotated and stored new-rt.
    const first = httpTesting.expectOne(`${environment.apiUrl}/auth/refresh`);
    expect(first.request.body).toEqual({ refreshToken: 'stale-rt' });
    localStorage.setItem('indifferent.refreshToken', 'fresh-rt');
    first.flush('conflict', { status: 401, statusText: 'Unauthorized' });

    // Single retry with the fresher token, then success.
    const second = httpTesting.expectOne(`${environment.apiUrl}/auth/refresh`);
    expect(second.request.body).toEqual({ refreshToken: 'fresh-rt' });
    second.flush(mockTokens);

    expect(resolved).toEqual(mockTokens);
    expect(service.getRefreshToken()).toBe('new-rt');
  });
});
