import { Routes } from '@angular/router';
import { LoginComponent, AuthCallbackComponent, authGuard } from './auth';

export const routes: Routes = [
  { path: 'login', component: LoginComponent },
  { path: 'auth/callback', component: AuthCallbackComponent },
  {
    path: 'dashboard',
    loadComponent: () =>
      import('./auth/components/login.component').then((m) => m.LoginComponent), // placeholder until dashboard module is implemented
    canActivate: [authGuard],
  },
  { path: '', redirectTo: '/dashboard', pathMatch: 'full' },
];
