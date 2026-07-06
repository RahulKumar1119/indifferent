import { Routes } from '@angular/router';
import { LoginComponent, AuthCallbackComponent, authGuard } from './auth';
import { LandingComponent } from './pages/landing';
import { NotFoundComponent } from './pages/not-found';

export const routes: Routes = [
  { path: 'landing', component: LandingComponent },
  { path: 'login', component: LoginComponent },
  { path: 'auth/callback', component: AuthCallbackComponent },
  {
    path: 'dashboard',
    loadComponent: () =>
      import('./pages/dashboard/dashboard.component').then((m) => m.DashboardComponent),
    canActivate: [authGuard],
  },
  {
    path: 'projects',
    loadComponent: () =>
      import('./pages/projects/projects.component').then((m) => m.ProjectsComponent),
    canActivate: [authGuard],
  },
  {
    path: 'projects/new',
    loadComponent: () =>
      import('./pages/create-project/create-project.component').then(
        (m) => m.CreateProjectComponent,
      ),
    canActivate: [authGuard],
  },
  {
    path: 'projects/:id/upload',
    loadComponent: () =>
      import('./pages/create-project/upload.component').then((m) => m.UploadComponent),
    canActivate: [authGuard],
  },
  {
    path: 'projects/:id/progress',
    loadComponent: () =>
      import('./pages/progress/progress.component').then((m) => m.ProgressComponent),
    canActivate: [authGuard],
  },
  {
    path: 'projects/:id/preview',
    loadComponent: () =>
      import('./pages/preview/preview.component').then((m) => m.PreviewComponent),
    canActivate: [authGuard],
  },
  { path: '', redirectTo: '/landing', pathMatch: 'full' },
  { path: '**', component: NotFoundComponent },
];
