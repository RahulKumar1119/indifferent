import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { catchError, map, of } from 'rxjs';
import { AuthService } from '../../core/services/auth.service';

export const authGuard: CanActivateFn = () => {
  const authService = inject(AuthService);
  const router = inject(Router);

  if (authService.isAuthenticated()) {
    return true;
  }

  // In-memory token is empty after a page reload — try the httpOnly
  // refresh cookie once before bouncing to /login.
  return authService.refreshToken().pipe(
    map(() => true),
    catchError(() => of(router.createUrlTree(['/login']))),
  );
};
