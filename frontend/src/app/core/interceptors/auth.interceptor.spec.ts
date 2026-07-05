import { TestBed } from '@angular/core/testing';
import { HTTP_INTERCEPTORS, HttpClient, provideHttpClient, withInterceptorsFromDi } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { provideRouter, Router } from '@angular/router';
import { AuthInterceptor } from './auth.interceptor';
import { AuthService } from '../services/auth.service';
import { environment } from '../../../environments/environment';
import { AuthTokens } from '../../shared';

describe('AuthInterceptor', () => {
  let httpClient: HttpClient;
  let httpTesting: HttpTestingController;
  let authService: AuthService;
  let router: Router;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(withInterceptorsFromDi()),
        provideHttpClientTesting(),
        provideRouter([]),
        {
          provide: HTTP_INTERCEPTORS,
          useClass: AuthInterceptor,
          multi: true,
        },
      ],
    });

    httpClient = TestBed.inject(HttpClient);
    httpTesting = TestBed.inject(HttpTestingController);
    authService = TestBed.inject(AuthService);
    router = TestBed.inject(Router);
  });

  afterEach(() => {
    httpTesting.verify();
  });

  it('should attach Authorization header when token exists', () => {
    authService.setTokens({ accessToken: 'my-token', refreshToken: 'rt', expiresIn: 3600 });

    httpClient.get('/api/data').subscribe();

    const req = httpTesting.expectOne('/api/data');
    expect(req.request.headers.get('Authorization')).toBe('Bearer my-token');
    req.flush({});
  });

  it('should NOT attach Authorization header to auth endpoints', () => {
    authService.setTokens({ accessToken: 'my-token', refreshToken: 'rt', expiresIn: 3600 });

    httpClient.post(`${environment.apiUrl}/auth/refresh`, {}).subscribe();

    const req = httpTesting.expectOne(`${environment.apiUrl}/auth/refresh`);
    expect(req.request.headers.has('Authorization')).toBeFalse();
    req.flush({});
  });

  it('should attempt token refresh on 401 and retry the request', () => {
    authService.setTokens({ accessToken: 'old-token', refreshToken: 'rt', expiresIn: 3600 });

    httpClient.get('/api/data').subscribe((res) => {
      expect(res).toEqual({ result: 'ok' });
    });

    // First request gets 401
    const firstReq = httpTesting.expectOne('/api/data');
    firstReq.flush(null, { status: 401, statusText: 'Unauthorized' });

    // Interceptor triggers refresh
    const refreshReq = httpTesting.expectOne(`${environment.apiUrl}/auth/refresh`);
    expect(refreshReq.request.method).toBe('POST');
    const newTokens: AuthTokens = { accessToken: 'new-token', refreshToken: 'new-rt', expiresIn: 3600 };
    refreshReq.flush(newTokens);

    // Retried request with new token
    const retryReq = httpTesting.expectOne('/api/data');
    expect(retryReq.request.headers.get('Authorization')).toBe('Bearer new-token');
    retryReq.flush({ result: 'ok' });
  });

  it('should call logout on refresh failure', () => {
    spyOn(authService, 'logout');
    authService.setTokens({ accessToken: 'old-token', refreshToken: 'rt', expiresIn: 3600 });

    httpClient.get('/api/data').subscribe({
      error: () => {
        // Expected error after refresh failure
      },
    });

    // First request gets 401
    const firstReq = httpTesting.expectOne('/api/data');
    firstReq.flush(null, { status: 401, statusText: 'Unauthorized' });

    // Refresh also fails
    const refreshReq = httpTesting.expectOne(`${environment.apiUrl}/auth/refresh`);
    refreshReq.flush(null, { status: 401, statusText: 'Unauthorized' });

    expect(authService.logout).toHaveBeenCalled();
  });
});
