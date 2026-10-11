import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { catchError, filter, map, of, switchMap, take } from 'rxjs';
import { AuthService } from '../../core/services/auth.service';

export const authGuard: CanActivateFn = () => {
  const authService = inject(AuthService);
  const router = inject(Router);

  if (authService.isAuthenticated()) {
    return true;
  }

  // Wait for the boot-time session restore first: firing our own refresh
  // concurrently with APP_INITIALIZER's would race the same token and look
  // like reuse (family wipe). refreshToken() is also singleflighted.
  return authService.authReady$.pipe(
    filter((ready) => ready),
    take(1),
    switchMap(() => {
      if (authService.isAuthenticated()) {
        return of(true);
      }
      // In-memory token is empty after a page reload: try the stored
      // refresh token once before bouncing to /login.
      return authService.refreshToken().pipe(
        map(() => true),
        catchError(() => of(router.createUrlTree(['/login']))),
      );
    }),
  );
};
