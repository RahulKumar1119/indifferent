import { TestBed } from '@angular/core/testing';
import { ActivatedRouteSnapshot, Router, RouterStateSnapshot, UrlTree, provideRouter } from '@angular/router';
import { Observable, firstValueFrom, isObservable, of, throwError } from 'rxjs';
import { routes } from '../../app.routes';
import { authGuard } from '../../auth';
import { AuthService } from '../../core/services/auth.service';

describe('account routes', () => {
  it('guards /account with the existing authGuard', () => {
    const route = routes.find((r) => r.path === 'account')!;
    expect(route).toBeTruthy();
    expect(route.canActivate).toContain(authGuard);
  });

  it('still guards /profile and keeps /profile/settings', () => {
    const route = routes.find((r) => r.path === 'profile')!;
    expect(route.canActivate).toContain(authGuard);
    expect(route.loadChildren).toBeDefined();
  });

  async function run(auth: Partial<AuthService>): Promise<boolean | UrlTree> {
    TestBed.configureTestingModule({ providers: [provideRouter([]), { provide: AuthService, useValue: auth }] });
    const out = TestBed.runInInjectionContext(() =>
      authGuard({} as ActivatedRouteSnapshot, { url: '/account' } as RouterStateSnapshot),
    );
    return isObservable(out) ? firstValueFrom(out as Observable<boolean | UrlTree>) : (out as boolean | UrlTree);
  }

  it('sends a signed-out visitor to the existing sign-in page', async () => {
    const result = await run({
      isAuthenticated: () => false,
      authReady$: of(true),
      refreshToken: () => throwError(() => new Error('no session')),
    } as unknown as Partial<AuthService>);
    const router = TestBed.inject(Router);
    expect(result instanceof UrlTree).toBeTrue();
    expect(router.serializeUrl(result as UrlTree)).toBe('/login');
  });

  it('lets a signed-in user through', async () => {
    expect(await run({ isAuthenticated: () => true } as unknown as Partial<AuthService>)).toBeTrue();
  });
});
