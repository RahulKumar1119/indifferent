import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

@Component({
  selector: 'app-not-found',
  standalone: true,
  imports: [RouterLink, MatButtonModule, MatIconModule],
  template: `
    <div
      class="min-h-screen flex flex-col items-center justify-center px-6 bg-gray-50 dark:bg-gray-950"
    >
      <mat-icon class="!text-8xl !w-24 !h-24 text-indigo-300 dark:text-indigo-700 mb-4">
        error_outline
      </mat-icon>
      <h1 class="text-7xl sm:text-9xl font-extrabold text-indigo-600 dark:text-indigo-400">
        404
      </h1>
      <h2 class="mt-4 text-2xl sm:text-3xl font-semibold text-gray-900 dark:text-white">
        Page Not Found
      </h2>
      <p class="mt-2 text-gray-600 dark:text-gray-400 text-center max-w-md">
        The page you're looking for doesn't exist or has been moved.
      </p>
      <div class="mt-8 flex gap-4">
        <a mat-flat-button routerLink="/landing" color="primary">
          Go Home
        </a>
        <a mat-stroked-button routerLink="/dashboard">
          Dashboard
        </a>
      </div>
    </div>
  `,
})
export class NotFoundComponent {}
