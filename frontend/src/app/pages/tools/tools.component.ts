import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';

@Component({
  selector: 'app-tools',
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
  `],
  template: `
    <!-- Navbar -->
    <nav class="fixed top-0 left-0 right-0 z-50 flex items-center justify-between px-6 py-4 about-nav">
      <a routerLink="/" class="flex items-center gap-2 text-xl font-bold">
        <img src="logo.svg" alt="Indifferent" class="h-8">
      </a>
      <a routerLink="/login" class="about-signin-btn">Sign In</a>
    </nav>

    <main class="max-w-4xl mx-auto px-6 py-12 pt-24">
      <header class="text-center mb-12">
        <h1 class="text-4xl font-bold mb-4 about-heading">Tools</h1>
        <p class="text-lg about-muted">
          Free tools to help you create better quiz content
        </p>
      </header>

      <!-- Text to Video -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg"></div>
          <h2 class="text-2xl font-semibold about-heading">TXT to Video Converter</h2>
        </div>
        <p class="leading-relaxed about-muted mb-4">
          Upload a plain text quiz file and convert it into a professional video with AI narration, custom templates, and countdown timers.
        </p>
        <a routerLink="/login" class="about-signin-btn">Get Started</a>
      </section>

      <!-- Quiz Format Validator -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg"></div>
          <h2 class="text-2xl font-semibold about-heading">Quiz Format Validator</h2>
        </div>
        <p class="leading-relaxed about-muted">
          Check if your quiz file is properly formatted before uploading. Supports numbered, bulleted, and tab-indented formats.
        </p>
      </section>

      <!-- Template Preview -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg"></div>
          <h2 class="text-2xl font-semibold about-heading">Template Gallery</h2>
        </div>
        <p class="leading-relaxed about-muted">
          Browse and preview all 6 video templates — Classic, Modern, Education, Dark, Minimal, and Neon — before creating your video.
        </p>
      </section>

      <!-- Voice Preview -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg"></div>
          <h2 class="text-2xl font-semibold about-heading">AI Voice Preview</h2>
        </div>
        <p class="leading-relaxed about-muted">
          Listen to samples of all 5 AI narration voices (Joanna, Matthew, Amy, Brian, Aditi) powered by Amazon Polly.
        </p>
      </section>

      <!-- Sample Quiz Files -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg"></div>
          <h2 class="text-2xl font-semibold about-heading">Sample Quiz Files</h2>
        </div>
        <p class="leading-relaxed about-muted">
          Download example .txt quiz files in all supported formats to use as templates for your own content.
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
export class ToolsComponent {}
