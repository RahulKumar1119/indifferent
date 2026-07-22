import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';

@Component({
  selector: 'app-privacy',
  standalone: true,
  imports: [RouterLink],
  styles: [`
    :host {
      display: block;
      background-color: #F8FAFC;
    }
    .about-card {
      background-color: #FFFFFF;
      border: 1px solid #E2E8F0;
      border-radius: 0.75rem;
    }
    .about-heading {
      color: #0F172A;
    }
    .about-muted {
      color: #475569;
    }
    .about-accent {
      color: #0D9488;
    }
    .about-accent-bg {
      background-color: #CCFBF1;
    }
    .about-accent-solid {
      background-color: #0D9488;
      color: #FFFFFF;
    }
    .about-marker {
      display: block;
    }
    .about-nav {
      background-color: rgba(255, 255, 255, 0.85);
      border-bottom: 1px solid #E2E8F0;
      backdrop-filter: blur(12px);
    }
    .about-signin-btn {
      background-color: #0D9488;
      color: #FFFFFF;
      padding: 0.5rem 1.25rem;
      border-radius: 0.5rem;
      font-weight: 600;
      font-size: 0.875rem;
      text-decoration: none;
      transition: background-color 0.2s;
    }
    .about-signin-btn:hover {
      background-color: #0F766E;
    }
    .about-cta-btn {
      background-color: #0D9488;
      color: #FFFFFF;
      display: inline-flex;
      align-items: center;
      gap: 0.5rem;
      padding: 0.5rem 1.25rem;
      border-radius: 0.5rem;
      font-weight: 600;
      font-size: 0.875rem;
      text-decoration: none;
      transition: background-color 0.2s;
    }
    .about-cta-btn:hover {
      background-color: #0F766E;
    }
  `],
  template: `
    <!-- Navbar -->
    <nav
      class="fixed top-0 left-0 right-0 z-50 flex items-center justify-between px-6 py-4 about-nav"
    >
      <a routerLink="/" class="flex items-center gap-2 text-xl font-bold">
        <img src="logo.svg" alt="Indifferent" class="h-8">
      </a>
      <a routerLink="/login" class="about-signin-btn">Sign In</a>
    </nav>

    <main class="max-w-4xl mx-auto px-6 py-12 pt-24">
      <!-- Header -->
      <header class="text-center mb-12">
        <h1 class="text-4xl font-bold mb-4 about-heading">Privacy Policy</h1>
        <p class="text-lg about-muted">
          Last updated: January 2025
        </p>
      </header>

      <!-- Information We Collect -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Information We Collect</h2>
        </div>
        <ul class="grid gap-3 list-none p-0 m-0">
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Account information (email, name via Google OAuth)</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Uploaded quiz files (.txt)</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Generated video content</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Usage data (pages visited, features used)</span>
          </li>
        </ul>
      </section>

      <!-- How We Use Your Information -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">How We Use Your Information</h2>
        </div>
        <ul class="grid gap-3 list-none p-0 m-0">
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">To provide the video generation service</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">To improve our platform</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">To communicate service updates</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">To provide technical support</span>
          </li>
        </ul>
      </section>

      <!-- Data Storage & Security -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Data Storage &amp; Security</h2>
        </div>
        <ul class="grid gap-3 list-none p-0 m-0">
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Files stored securely on AWS S3</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Data encrypted in transit and at rest</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">We do not sell your personal information</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">You can request data deletion at any time</span>
          </li>
        </ul>
      </section>

      <!-- Third-Party Services -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Third-Party Services</h2>
        </div>
        <ul class="grid gap-3 list-none p-0 m-0">
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">AWS (hosting, storage, compute)</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Google (authentication)</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Amazon Polly (text-to-speech)</span>
          </li>
        </ul>
      </section>

      <!-- Your Rights -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Your Rights</h2>
        </div>
        <ul class="grid gap-3 list-none p-0 m-0">
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Access your personal data</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Request data deletion</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Export your content</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Opt out of communications</span>
          </li>
        </ul>
      </section>

      <!-- Cookies -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Cookies</h2>
        </div>
        <p class="leading-relaxed about-muted">
          We use essential cookies for authentication and session management only.
        </p>
      </section>

      <!-- Changes to This Policy -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Changes to This Policy</h2>
        </div>
        <p class="leading-relaxed about-muted">
          We may update this policy periodically. Changes will be posted on this page.
        </p>
      </section>

      <!-- Contact -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Contact</h2>
        </div>
        <p class="about-muted">
          For privacy questions, contact
          <a href="mailto:support@indifferent.fun" class="about-accent hover:underline">support&#64;indifferent.fun</a>
        </p>
      </section>

      <!-- Back to Home -->
      <div class="text-center">
        <a routerLink="/" class="hover:underline text-sm about-accent">
          &larr; Back to Home
        </a>
      </div>
    </main>
  `,
})
export class PrivacyComponent {}
