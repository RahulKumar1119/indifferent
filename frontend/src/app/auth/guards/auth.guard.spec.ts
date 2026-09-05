import { TestBed } from '@angular/core/testing';
import { Router, UrlTree } from '@angular/router';
import { provideRouter } from '@angular/router';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { of, throwError } from 'rxjs';
import { authGuard } from './auth.guard';
import { AuthService } from '../../core/services/auth.service';
import { provideHttpClient } from '@angular/common/http';
import { environment } from '../../../environments/environment';

describe('authGuard', () => {
  let authService: AuthService;
  let router: Router;
  let httpTesting: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([{ path: 'login', component: class {} as any }]),
      ],
    });

    authService = TestBed.inject(AuthService);
    router = TestBed.inject(Router);
    httpTesting = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpTesting.verify();
  });

  it('should allow authenticated users through', () => {
    spyOn(authService, 'isAuthenticated').and.returnValue(true);

    const result = TestBed.runInInjectionContext(() => authGuard({} as any, {} as any));

    expect(result).toBe(true);
  });

  it('should try refresh before redirecting unauthenticated users', () => {
    spyOn(authService, 'isAuthenticated').and.returnValue(false);
    spyOn(authService, 'refreshToken').and.returnValue(
      of({ accessToken: 'new', refreshToken: 'rt', expiresIn: 3600 }),
    );

    const result = TestBed.runInInjectionContext(() => authGuard({} as any, {} as any));

    expect(typeof result === 'boolean' || result instanceof UrlTree).toBeFalse();
    let resolved: unknown;
    (result as any).subscribe((v: unknown) => (resolved = v));
    expect(resolved).toBe(true);
  });

  it('should redirect to /login when refresh fails', () => {
    spyOn(authService, 'isAuthenticated').and.returnValue(false);
    spyOn(authService, 'refreshToken').and.returnValue(
      throwError(() => ({ status: 401 })),
    );

    const result = TestBed.runInInjectionContext(() => authGuard({} as any, {} as any));

    let resolved: unknown;
    (result as any).subscribe((v: unknown) => (resolved = v));
    expect(resolved).toBeInstanceOf(UrlTree);
    expect((resolved as UrlTree).toString()).toBe('/login');
  });
});
