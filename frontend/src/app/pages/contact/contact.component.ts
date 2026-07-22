import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';

@Component({
  selector: 'app-contact',
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
        <h1 class="text-4xl font-bold mb-4 about-heading">Contact Us</h1>
        <p class="text-lg about-muted">
          We'd love to hear from you
        </p>
      </header>

      <!-- Get In Touch -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Get In Touch</h2>
        </div>
        <p class="leading-relaxed about-muted">
          Have questions, feedback, or feature requests? Reach out through any of the channels below.
        </p>
      </section>

      <!-- GitHub -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">GitHub</h2>
        </div>
        <p class="mb-4 about-muted">
          Report bugs, request features, or ask questions
        </p>
        <a href="https://github.com/RahulKumar1119/indifferent/issues" target="_blank" rel="noopener" class="about-cta-btn">
          <div aria-hidden="true" class="w-4 h-4 rounded-full about-marker"></div>
          Open an Issue
        </a>
      </section>

      <!-- Email -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Email</h2>
        </div>
        <p class="mb-4 about-muted">
          For general inquiries and support
        </p>
        <a href="mailto:support@indifferent.fun" class="about-accent font-medium hover:underline">
          support&#64;indifferent.fun
        </a>
      </section>

      <!-- Response Time -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Response Time</h2>
        </div>
        <p class="leading-relaxed about-muted">
          We typically respond within 24-48 hours
        </p>
      </section>

      <!-- Footer -->
      <footer class="mt-12 pt-8 border-t border-gray-200">
        <div class="flex flex-wrap justify-center gap-6 text-sm about-muted mb-4">
          <a routerLink="/" class="hover:underline about-accent">Home</a>
          <a routerLink="/about" class="hover:underline about-accent">About</a>
          <a routerLink="/contact" class="hover:underline about-accent">Contact</a>
          <a routerLink="/privacy" class="hover:underline about-accent">Privacy Policy</a>
          <a routerLink="/terms" class="hover:underline about-accent">Terms of Service</a>
        </div>
        <p class="text-center text-xs about-muted">&copy; 2025 Indifferent. All rights reserved.</p>
      </footer>
    </main>
  `,
})
export class ContactComponent {}
