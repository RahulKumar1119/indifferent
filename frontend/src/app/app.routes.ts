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
    path: 'projects/:id',
    loadComponent: () =>
      import('./pages/projects/project-detail.component').then((m) => m.ProjectDetailComponent),
    canActivate: [authGuard],
  },
  {
    path: 'new',
    loadComponent: () => import('./pages/new/new.component').then((m) => m.NewComponent),
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
    path: 'shorts',
    loadComponent: () =>
      import('./pages/shorts/shorts-upload.component').then((m) => m.ShortsUploadComponent),
    canActivate: [authGuard],
  },
  {
    path: 'shorts/history',
    loadComponent: () =>
      import('./pages/shorts/shorts-history.component').then((m) => m.ShortsHistoryComponent),
    canActivate: [authGuard],
  },
  {
    path: 'shorts/:id/progress',
    loadComponent: () =>
      import('./pages/shorts/shorts-progress.component').then((m) => m.ShortsProgressComponent),
    canActivate: [authGuard],
  },
  {
    path: 'shorts/:id/clips',
    loadComponent: () =>
      import('./pages/shorts/shorts-gallery.component').then((m) => m.ShortsGalleryComponent),
    canActivate: [authGuard],
  },
  {
    path: 'features/:slug',
    loadComponent: () =>
      import('./pages/features/feature-page.component').then((m) => m.FeaturePageComponent),
  },
  {
    path: 'shorts-maker',
    loadComponent: () =>
      import('./pages/shorts-maker/shorts-maker.component').then((m) => m.ShortsMakerComponent),
  },
  {
    path: 'use-cases/:slug',
    loadComponent: () =>
      import('./pages/use-cases/use-case.component').then((m) => m.UseCaseComponent),
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
