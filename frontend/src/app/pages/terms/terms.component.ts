import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';

@Component({
  selector: 'app-terms',
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
        <h1 class="text-4xl font-bold mb-4 about-heading">Terms of Service</h1>
        <p class="text-lg about-muted">
          Last updated: January 2025
        </p>
      </header>

      <!-- Acceptance of Terms -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Acceptance of Terms</h2>
        </div>
        <p class="leading-relaxed about-muted">
          By using Indifferent you agree to these terms. If you do not agree, please do not use the service.
        </p>
      </section>

      <!-- Service Description -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Service Description</h2>
        </div>
        <p class="leading-relaxed about-muted">
          Indifferent converts text-based quiz files into video content using automated processing.
        </p>
      </section>

      <!-- User Accounts -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">User Accounts</h2>
        </div>
        <ul class="grid gap-3 list-none p-0 m-0">
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Must be 13+ to use the service</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Responsible for account security</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">One account per person</span>
          </li>
        </ul>
      </section>

      <!-- Acceptable Use -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Acceptable Use</h2>
        </div>
        <ul class="grid gap-3 list-none p-0 m-0">
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Upload only content you own or have rights to</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Do not upload harmful or illegal content</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Do not attempt to reverse engineer the service</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Do not use for spam or abuse</span>
          </li>
        </ul>
      </section>

      <!-- Content Ownership -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Content Ownership</h2>
        </div>
        <ul class="grid gap-3 list-none p-0 m-0">
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">You retain ownership of uploaded quiz files</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">You retain ownership of generated videos</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">We do not claim rights to your content</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">We may use anonymized usage data to improve the service</span>
          </li>
        </ul>
      </section>

      <!-- Service Availability -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Service Availability</h2>
        </div>
        <ul class="grid gap-3 list-none p-0 m-0">
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">We aim for high availability but do not guarantee 100% uptime</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">We may modify or discontinue features with notice</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Scheduled maintenance will be communicated in advance</span>
          </li>
        </ul>
      </section>

      <!-- Limitation of Liability -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Limitation of Liability</h2>
        </div>
        <ul class="grid gap-3 list-none p-0 m-0">
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Service provided "as is"</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Not liable for content accuracy in generated videos</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Not liable for indirect damages</span>
          </li>
        </ul>
      </section>

      <!-- Termination -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Termination</h2>
        </div>
        <ul class="grid gap-3 list-none p-0 m-0">
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">We may suspend accounts violating these terms</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">You may delete your account at any time</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Upon termination your data will be deleted within 30 days</span>
          </li>
        </ul>
      </section>

      <!-- Changes to Terms -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Changes to Terms</h2>
        </div>
        <p class="leading-relaxed about-muted">
          We may update terms with 30 days notice. Continued use constitutes acceptance.
        </p>
      </section>

      <!-- Contact -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Contact</h2>
        </div>
        <p class="about-muted">
          For questions about terms, contact
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
export class TermsComponent {}
