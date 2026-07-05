import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatIconModule } from '@angular/material/icon';

@Component({
  selector: 'app-landing',
  standalone: true,
  imports: [RouterLink, MatButtonModule, MatCardModule, MatIconModule],
  template: `
    <!-- Navbar -->
    <nav
      class="fixed top-0 left-0 right-0 z-50 flex items-center justify-between px-6 py-4 bg-white/80 dark:bg-gray-900/80 backdrop-blur-md border-b border-gray-200 dark:border-gray-800"
    >
      <a routerLink="/landing" class="flex items-center gap-2 text-xl font-bold text-gray-900 dark:text-white">
        <mat-icon class="text-indigo-600 dark:text-indigo-400">videocam</mat-icon>
        <span>TXT2Video</span>
      </a>
      <a mat-flat-button routerLink="/login" color="primary">Sign In</a>
    </nav>

    <!-- Hero Section -->
    <section
      class="min-h-screen flex items-center justify-center px-6 pt-20 bg-gradient-to-br from-indigo-50 via-white to-purple-50 dark:from-gray-900 dark:via-gray-950 dark:to-indigo-950"
    >
      <div class="max-w-4xl mx-auto text-center">
        <h1
          class="text-4xl sm:text-5xl md:text-6xl font-extrabold text-gray-900 dark:text-white leading-tight"
        >
          Turn Your TXT Quizzes Into
          <span class="text-indigo-600 dark:text-indigo-400">YouTube-Ready Videos</span>
        </h1>
        <p class="mt-6 text-lg sm:text-xl text-gray-600 dark:text-gray-300 max-w-2xl mx-auto">
          Upload a text file with multiple-choice questions and get a professional MP4 video
          complete with narration, animations, and countdown timers — all automatically.
        </p>
        <div class="mt-10">
          <a
            mat-flat-button
            routerLink="/login"
            color="primary"
            class="!text-lg !px-8 !py-3"
          >
            Get Started Free
          </a>
        </div>
      </div>
    </section>

    <!-- Features Section -->
    <section class="py-20 px-6 bg-white dark:bg-gray-950">
      <div class="max-w-6xl mx-auto">
        <h2 class="text-3xl sm:text-4xl font-bold text-center text-gray-900 dark:text-white mb-4">
          Everything You Need
        </h2>
        <p class="text-center text-gray-600 dark:text-gray-400 mb-12 max-w-2xl mx-auto">
          From text file to finished video in minutes. No editing skills required.
        </p>
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
          @for (feature of features; track feature.title) {
            <mat-card class="!bg-gray-50 dark:!bg-gray-900 !border !border-gray-200 dark:!border-gray-800">
              <mat-card-header>
                <mat-icon mat-card-avatar class="!text-indigo-600 dark:!text-indigo-400 !text-3xl !w-10 !h-10 flex items-center justify-center">
                  {{ feature.icon }}
                </mat-icon>
                <mat-card-title class="!text-gray-900 dark:!text-white">{{ feature.title }}</mat-card-title>
              </mat-card-header>
              <mat-card-content>
                <p class="text-gray-600 dark:text-gray-400 mt-2">{{ feature.description }}</p>
              </mat-card-content>
            </mat-card>
          }
        </div>
      </div>
    </section>

    <!-- CTA Section -->
    <section
      class="py-20 px-6 bg-indigo-600 dark:bg-indigo-900"
    >
      <div class="max-w-3xl mx-auto text-center">
        <h2 class="text-3xl sm:text-4xl font-bold text-white mb-4">
          Ready to Create Your First Video?
        </h2>
        <p class="text-indigo-100 mb-8 text-lg">
          Sign in with Google and upload your first TXT quiz. It only takes a few minutes.
        </p>
        <a
          mat-flat-button
          routerLink="/login"
          class="!bg-white !text-indigo-700 !text-lg !px-8 !py-3"
        >
          Start Now
        </a>
      </div>
    </section>

    <!-- Footer -->
    <footer class="py-8 px-6 bg-gray-100 dark:bg-gray-900 border-t border-gray-200 dark:border-gray-800">
      <div class="max-w-6xl mx-auto text-center text-gray-500 dark:text-gray-400 text-sm">
        &copy; {{ currentYear }} TXT2Video. All rights reserved.
      </div>
    </footer>
  `,
})
export class LandingComponent {
  currentYear = new Date().getFullYear();

  features = [
    {
      icon: 'auto_fix_high',
      title: 'Auto Format Detection',
      description: 'Supports numbered, bulleted, and tabbed question formats automatically.',
    },
    {
      icon: 'palette',
      title: 'Multiple Templates',
      description: 'Choose from Classic, Modern, Education, Dark, Minimal, and Neon themes.',
    },
    {
      icon: 'record_voice_over',
      title: 'AI Narration',
      description: 'Professional voice-over narration powered by Amazon Polly with 5 voice options.',
    },
    {
      icon: 'speed',
      title: 'Fast Rendering',
      description: 'Serverless pipeline renders your video in minutes with countdown timers and transitions.',
    },
  ];
}
