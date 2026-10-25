import { Component, OnInit, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { ROBOTS_NOINDEX, setPageSeo, setRobots } from '../../shared/seo';

@Component({
  selector: 'app-not-found',
  standalone: true,
  imports: [RouterLink, LucideDynamicIcon],
  template: `
    <div class="min-h-screen flex flex-col items-center justify-center px-6">
      <h1 class="text-8xl sm:text-9xl font-extrabold text-[hsl(var(--primary))]">
        404
      </h1>
      <h2 class="mt-4 text-2xl sm:text-3xl font-semibold">
        Page Not Found
      </h2>
      <p class="mt-2 text-[hsl(var(--muted-foreground))] text-center max-w-md">
        The page you're looking for doesn't exist or has been moved.
      </p>
      <div class="mt-8 flex gap-4">
        <a routerLink="/" class="glow-btn">
          <svg lucideIcon="home" [size]="18"></svg>
          Go Home
        </a>
        <a
          routerLink="/dashboard"
          class="inline-flex items-center gap-2 px-5 py-3 rounded-lg border border-[hsl(var(--border))] hover:bg-white/5 transition-colors font-medium"
        >
          Dashboard
        </a>
      </div>
    </div>
  `,
})
export class NotFoundComponent implements OnInit {
  private readonly document = inject(DOCUMENT);

  ngOnInit(): void {
    // The host answers every path with the app shell and a 200, so a missing
    // page has to say it is missing itself: noindex keeps it out of search.
    setPageSeo(this.document, {
      title: 'Page not found | Indifferent',
      description: 'This page does not exist. Go back to the Indifferent home page or the journal.',
      canonical: 'https://indifferent.fun/',
    });
    setRobots(this.document, ROBOTS_NOINDEX);
  }
}
