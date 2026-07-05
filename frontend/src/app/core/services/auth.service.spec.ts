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

  it('should call POST /auth/refresh for refreshToken', () => {
    const mockTokens: AuthTokens = { accessToken: 'new-at', refreshToken: 'new-rt', expiresIn: 3600 };

    service.refreshToken().subscribe((tokens) => {
      expect(tokens).toEqual(mockTokens);
      expect(service.getAccessToken()).toBe('new-at');
    });

    const req = httpTesting.expectOne(`${environment.apiUrl}/auth/refresh`);
    expect(req.request.method).toBe('POST');
    expect(req.request.withCredentials).toBeTrue();
    req.flush(mockTokens);
  });

  it('should clear token and navigate to /login on logout', () => {
    spyOn(router, 'navigate');
    service.setTokens({ accessToken: 'token', refreshToken: 'rt', expiresIn: 3600 });

    service.logout();

    const req = httpTesting.expectOne(`${environment.apiUrl}/auth/logout`);
    req.flush(null);

    expect(service.getAccessToken()).toBeNull();
    expect(router.navigate).toHaveBeenCalledWith(['/login']);
  });
});
