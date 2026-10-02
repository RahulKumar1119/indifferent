import { RenderMode, ServerRoute } from '@angular/ssr';

const USE_CASE_SLUGS = [
  'content-creators',
  'marketing-teams',
  'agencies',
  'coaches',
  'media-companies',
  'educators',
];

export const serverRoutes: ServerRoute[] = [
  // Auth-gated + parameterized app routes render on demand, never prerendered
  // (they need a live session and would leak noindex shells to crawlers).
  { path: 'dashboard', renderMode: RenderMode.Server },
  { path: 'projects', renderMode: RenderMode.Server },
  { path: 'projects/new', renderMode: RenderMode.Server },
  { path: 'projects/:id/upload', renderMode: RenderMode.Server },
  { path: 'projects/:id/progress', renderMode: RenderMode.Server },
  { path: 'projects/:id/preview', renderMode: RenderMode.Server },
  { path: 'shorts', renderMode: RenderMode.Server },
  { path: 'shorts/:id/progress', renderMode: RenderMode.Server },
  { path: 'shorts/:id/clips', renderMode: RenderMode.Server },
  { path: 'login', renderMode: RenderMode.Server },
  { path: 'auth/callback', renderMode: RenderMode.Server },
  // Persona SEO pages prerender to static HTML with per-slug meta.
  {
    path: 'use-cases/:slug',
    renderMode: RenderMode.Prerender,
    getPrerenderParams: async () => USE_CASE_SLUGS.map((slug) => ({ slug })),
  },
  // All other marketing pages prerender statically.
  {
    path: '**',
    renderMode: RenderMode.Prerender,
  },
];
