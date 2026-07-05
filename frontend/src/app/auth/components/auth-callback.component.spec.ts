import { ComponentFixture, TestBed, fakeAsync, tick } from '@angular/core/testing';
import { AuthCallbackComponent } from './auth-callback.component';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { provideRouter, Router } from '@angular/router';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { AuthService } from '../../core/services/auth.service';
import { environment } from '../../../environments/environment';
import { AuthTokens } from '../../shared';

describe('AuthCallbackComponent', () => {
  let httpTesting: HttpTestingController;
  let router: Router;
  let authService: AuthService;

  function createComponent(queryParams: Record<string, string> = {}) {
    TestBed.configureTestingModule({
      imports: [AuthCallbackComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
        {
          provide: ActivatedRoute,
          useValue: {
            snapshot: {
              queryParamMap: convertToParamMap(queryParams),
            },
          },
        },
      ],
    });

    httpTesting = TestBed.inject(HttpTestingController);
    router = TestBed.inject(Router);
    authService = TestBed.inject(AuthService);
    spyOn(router, 'navigate');

    const fixture = TestBed.createComponent(AuthCallbackComponent);
    fixture.detectChanges();
    return fixture;
  }

  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('should POST to API with the code query param', () => {
    createComponent({ code: 'test-auth-code' });

    const req = httpTesting.expectOne(`${environment.apiUrl}/auth/google/callback`);
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual({ code: 'test-auth-code' });
    req.flush({ accessToken: 'at', refreshToken: 'rt', expiresIn: 3600 });
  });

  it('should call authService.setTokens and navigate to /dashboard on success', () => {
    const fixture = createComponent({ code: 'test-auth-code' });
    spyOn(authService, 'setTokens');

    const mockTokens: AuthTokens = { accessToken: 'at', refreshToken: 'rt', expiresIn: 3600 };
    const req = httpTesting.expectOne(`${environment.apiUrl}/auth/google/callback`);
    req.flush(mockTokens);

    expect(authService.setTokens).toHaveBeenCalledWith(mockTokens);
    expect(router.navigate).toHaveBeenCalledWith(['/dashboard']);
  });

  it('should navigate to /login with error param on API error', () => {
    createComponent({ code: 'test-auth-code' });

    const req = httpTesting.expectOne(`${environment.apiUrl}/auth/google/callback`);
    req.flush(null, { status: 500, statusText: 'Server Error' });

    expect(router.navigate).toHaveBeenCalledWith(['/login'], { queryParams: { error: 'auth_failed' } });
  });

  it('should redirect to /login with missing_code error when no code param', () => {
    createComponent({});

    httpTesting.expectNone(`${environment.apiUrl}/auth/google/callback`);
    expect(router.navigate).toHaveBeenCalledWith(['/login'], { queryParams: { error: 'missing_code' } });
  });
});
