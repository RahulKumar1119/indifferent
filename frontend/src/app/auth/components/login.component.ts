import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { environment } from '../../../environments/environment';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [RouterLink, LucideDynamicIcon],
  template: `
    <div class="flex min-h-screen">
      <!-- Left Panel - Branding -->
      <div class="hidden lg:flex lg:w-1/2 relative overflow-hidden bg-gradient-to-br from-violet-600 via-purple-600 to-indigo-700 p-12 flex-col justify-between">
        <!-- Decorative circles -->
        <div class="absolute -top-24 -left-24 w-96 h-96 bg-white/10 rounded-full blur-3xl"></div>
        <div class="absolute -bottom-32 -right-32 w-[500px] h-[500px] bg-white/5 rounded-full blur-3xl"></div>
        <div class="absolute top-1/2 left-1/3 w-64 h-64 bg-purple-400/10 rounded-full blur-2xl"></div>

        <!-- Logo & Nav -->
        <div class="relative z-10">
          <a routerLink="/" class="flex items-center gap-3 text-white">
            <div class="w-10 h-10 bg-white/20 backdrop-blur-sm rounded-lg flex items-center justify-center">
              <svg lucideIcon="video" [size]="22" class="text-white"></svg>
            </div>
            <span class="text-xl font-bold">Indifferent</span>
          </a>
        </div>

        <!-- Main Content -->
        <div class="relative z-10 space-y-8">
          <div>
            <h1 class="text-4xl font-bold text-white leading-tight mb-4">
              Turn your quiz files into<br>
              <span class="text-purple-200">stunning videos</span>
            </h1>
            <p class="text-lg text-purple-100/80 max-w-md">
              Upload a TXT file and get a professional MP4 video with narration, 
              animations, and countdown timers — all automatically.
            </p>
          </div>

          <!-- Testimonial Card -->
          <div class="bg-white/10 backdrop-blur-md rounded-2xl p-6 border border-white/10 max-w-md">
            <div class="flex items-center gap-1 mb-3">
              @for (star of [1,2,3,4,5]; track star) {
                <svg lucideIcon="star" [size]="16" class="text-yellow-400 fill-yellow-400"></svg>
              }
            </div>
            <p class="text-white/90 text-sm leading-relaxed mb-4">
              "Incredible tool! I used to spend hours editing quiz videos manually. 
              Now I just upload my text file and get a professional video in minutes. 
              Game changer for my YouTube channel."
            </p>
            <div class="flex items-center gap-3">
              <div class="w-9 h-9 rounded-full bg-purple-300/30 flex items-center justify-center">
                <span class="text-white text-sm font-semibold">AK</span>
              </div>
              <div>
                <p class="text-white text-sm font-medium">Aarav Kumar</p>
                <p class="text-purple-200/70 text-xs">Education Content Creator</p>
              </div>
            </div>
          </div>
        </div>

        <!-- Footer -->
        <div class="relative z-10">
          <p class="text-purple-200/50 text-sm">
            © {{ currentYear }} Indifferent. All rights reserved.
          </p>
        </div>
      </div>

      <!-- Right Panel - Sign In -->
      <div class="flex-1 flex items-center justify-center p-8 bg-white">
        <div class="w-full max-w-md space-y-8">
          <!-- Mobile Logo (shown on small screens) -->
          <div class="lg:hidden flex items-center gap-3 justify-center mb-8">
            <div class="w-10 h-10 bg-violet-100 rounded-lg flex items-center justify-center">
              <svg lucideIcon="video" [size]="22" class="text-violet-600"></svg>
            </div>
            <span class="text-xl font-bold text-gray-900">Indifferent</span>
          </div>

          <!-- Header -->
          <div class="text-center">
            <h2 class="text-2xl font-bold text-gray-900 mb-2">Welcome back</h2>
            <p class="text-gray-500 text-sm">
              Sign in to your account to continue
            </p>
          </div>

          <!-- Sign In Button -->
          <div class="space-y-4">
            <button
              (click)="signInWithGoogle()"
              class="w-full flex items-center justify-center gap-3 px-6 py-3.5 border border-gray-200 rounded-xl hover:bg-gray-50 hover:border-gray-300 transition-all duration-200 shadow-sm hover:shadow group"
            >
              <svg class="w-5 h-5" viewBox="0 0 24 24">
                <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4"/>
                <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/>
                <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/>
                <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/>
              </svg>
              <span class="text-gray-700 font-medium text-sm group-hover:text-gray-900">Continue with Google</span>
            </button>
          </div>

          <!-- Divider -->
          <div class="relative">
            <div class="absolute inset-0 flex items-center">
              <div class="w-full border-t border-gray-200"></div>
            </div>
            <div class="relative flex justify-center text-xs">
              <span class="bg-white px-4 text-gray-400">Secure sign-in powered by Google</span>
            </div>
          </div>

          <!-- Features -->
          <div class="grid grid-cols-2 gap-4">
            <div class="flex items-center gap-2 text-sm text-gray-500">
              <svg lucideIcon="check" [size]="16" class="text-green-500"></svg>
              <span>Free to use</span>
            </div>
            <div class="flex items-center gap-2 text-sm text-gray-500">
              <svg lucideIcon="check" [size]="16" class="text-green-500"></svg>
              <span>No credit card</span>
            </div>
            <div class="flex items-center gap-2 text-sm text-gray-500">
              <svg lucideIcon="check" [size]="16" class="text-green-500"></svg>
              <span>AI narration</span>
            </div>
            <div class="flex items-center gap-2 text-sm text-gray-500">
              <svg lucideIcon="check" [size]="16" class="text-green-500"></svg>
              <span>HD videos</span>
            </div>
          </div>

          <!-- Terms -->
          <p class="text-center text-xs text-gray-400">
            By continuing, you agree to our
            <a href="#" class="text-violet-600 hover:underline">Terms of Service</a>
            and
            <a href="#" class="text-violet-600 hover:underline">Privacy Policy</a>
          </p>

          <!-- Back to Home -->
          <div class="text-center">
            <a routerLink="/" class="text-sm text-gray-500 hover:text-violet-600 transition-colors">
              ← Back to home
            </a>
          </div>
        </div>
      </div>
    </div>
  `,
})
export class LoginComponent {
  currentYear = new Date().getFullYear();

  signInWithGoogle(): void {
    const params = new URLSearchParams({
      client_id: environment.googleClientId,
      redirect_uri: environment.googleRedirectUri,
      response_type: 'code',
      scope: 'openid email profile',
    });

    window.location.href = `https://accounts.google.com/o/oauth2/v2/auth?${params.toString()}`;
  }
}
