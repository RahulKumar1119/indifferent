import { Routes } from '@angular/router';
import { LoginComponent, AuthCallbackComponent, authGuard } from './auth';
import { LandingComponent } from './pages/landing';
import { NotFoundComponent } from './pages/not-found';

export const routes: Routes = [
  { path: '', component: LandingComponent, pathMatch: 'full' },
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
  {
    path: 'about',
    loadComponent: () =>
      import('./pages/about/about.component').then((m) => m.AboutComponent),
  },
  {
    path: 'contact',
    loadComponent: () =>
      import('./pages/contact/contact.component').then((m) => m.ContactComponent),
  },
  {
    path: 'privacy',
    loadComponent: () =>
      import('./pages/privacy/privacy.component').then((m) => m.PrivacyComponent),
  },
  {
    path: 'terms',
    loadComponent: () =>
      import('./pages/terms/terms.component').then((m) => m.TermsComponent),
  },
  {
    path: 'tools',
    loadComponent: () =>
      import('./pages/tools/tools.component').then((m) => m.ToolsComponent),
  },
  {
    path: 'tools/add-watermark',
    loadComponent: () =>
      import('./pages/tools/watermark/watermark.component').then((m) => m.WatermarkComponent),
  },
  {
    path: 'blog',
    loadComponent: () =>
      import('./pages/blog/blog.component').then((m) => m.BlogComponent),
  },
  {
    path: 'blog/quiz-videos-youtube',
    loadComponent: () =>
      import('./pages/blog/articles/quiz-videos-youtube.component').then(
        (m) => m.QuizVideosYoutubeComponent,
      ),
  },
  {
    path: 'blog/educational-video-best-practices',
    loadComponent: () =>
      import('./pages/blog/articles/educational-video-best-practices.component').then(
        (m) => m.EducationalVideoBestPracticesComponent,
      ),
  },
  {
    path: 'blog/quiz-file-format-guide',
    loadComponent: () =>
      import('./pages/blog/articles/quiz-file-format-guide.component').then(
        (m) => m.QuizFileFormatGuideComponent,
      ),
  },
  {
    path: 'blog/video-template-comparison',
    loadComponent: () =>
      import('./pages/blog/articles/video-template-comparison.component').then(
        (m) => m.VideoTemplateComparisonComponent,
      ),
  },
  {
    path: 'blog/ai-narration-guide',
    loadComponent: () =>
      import('./pages/blog/articles/ai-narration-guide.component').then(
        (m) => m.AiNarrationGuideComponent,
      ),
  },
  { path: '**', component: NotFoundComponent },
];
