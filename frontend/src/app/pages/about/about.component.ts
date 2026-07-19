import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';

@Component({
  selector: 'app-about',
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
        <h1 class="text-4xl font-bold mb-4 about-heading">About Indifferent</h1>
        <p class="text-lg about-muted">
          Transforming text-based quizzes into professional video content
        </p>
      </header>

      <!-- Mission Section -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Our Mission</h2>
        </div>
        <p class="leading-relaxed about-muted">
          Indifferent makes it effortless to convert your multiple-choice quiz files into engaging,
          YouTube-ready video content. Whether you're an educator, content creator, or training
          professional, our platform automates the entire video production process — from parsing
          your questions to generating narrated, animated videos with countdown timers and answer reveals.
        </p>
      </section>

      <!-- How It Works -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">How It Works</h2>
        </div>
        <ol class="grid gap-6 list-none p-0 m-0">
          <li class="flex gap-4 items-start">
            <span class="w-8 h-8 rounded-full flex items-center justify-center font-bold text-sm shrink-0 about-accent-solid">1</span>
            <div>
              <h3 class="font-medium mb-1 about-heading">Upload Your Quiz File</h3>
              <p class="text-sm about-muted">Upload a plain text (.txt) file with your multiple-choice questions. We support numbered, bulleted, and tab-indented formats.</p>
            </div>
          </li>
          <li class="flex gap-4 items-start">
            <span class="w-8 h-8 rounded-full flex items-center justify-center font-bold text-sm shrink-0 about-accent-solid">2</span>
            <div>
              <h3 class="font-medium mb-1 about-heading">Choose Template &amp; Voice</h3>
              <p class="text-sm about-muted">Select a visual template for your slides and pick from 5 professional AI voices powered by Amazon Polly.</p>
            </div>
          </li>
          <li class="flex gap-4 items-start">
            <span class="w-8 h-8 rounded-full flex items-center justify-center font-bold text-sm shrink-0 about-accent-solid">3</span>
            <div>
              <h3 class="font-medium mb-1 about-heading">Automatic Processing</h3>
              <p class="text-sm about-muted">Our serverless pipeline parses your questions, generates animated slides, creates narration, and renders the final video.</p>
            </div>
          </li>
          <li class="flex gap-4 items-start">
            <span class="w-8 h-8 rounded-full flex items-center justify-center font-bold text-sm shrink-0 about-accent-solid">4</span>
            <div>
              <h3 class="font-medium mb-1 about-heading">Download &amp; Share</h3>
              <p class="text-sm about-muted">Preview your video in-browser and download the MP4 file ready for YouTube, social media, or any platform.</p>
            </div>
          </li>
        </ol>
      </section>

      <!-- Supported Formats -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Supported Formats</h2>
        </div>
        <ul class="grid gap-3 list-none p-0 m-0">
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Numbered questions (1. Question text)</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Bulleted questions (• or - Question text)</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Tab-indented answers</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Multiple correct answers supported</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Maximum file size: 5MB</span>
          </li>
        </ul>
      </section>

      <!-- Video Templates -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Video Templates</h2>
        </div>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4" role="list">
          <div class="about-card p-4 flex items-start gap-3" role="listitem">
            <div aria-hidden="true" class="w-5 h-5 shrink-0 mt-0.5 rounded about-accent-bg about-marker"></div>
            <article>
              <p class="font-medium text-sm">Classic</p>
              <p class="text-xs mt-0.5 about-muted">Clean blue theme, professional look</p>
            </article>
          </div>
          <div class="about-card p-4 flex items-start gap-3" role="listitem">
            <div aria-hidden="true" class="w-5 h-5 shrink-0 mt-0.5 rounded about-accent-bg about-marker"></div>
            <article>
              <p class="font-medium text-sm">Modern</p>
              <p class="text-xs mt-0.5 about-muted">Gradient backgrounds, contemporary design</p>
            </article>
          </div>
          <div class="about-card p-4 flex items-start gap-3" role="listitem">
            <div aria-hidden="true" class="w-5 h-5 shrink-0 mt-0.5 rounded about-accent-bg about-marker"></div>
            <article>
              <p class="font-medium text-sm">Education</p>
              <p class="text-xs mt-0.5 about-muted">Warm colors, classroom-friendly</p>
            </article>
          </div>
          <div class="about-card p-4 flex items-start gap-3" role="listitem">
            <div aria-hidden="true" class="w-5 h-5 shrink-0 mt-0.5 rounded about-accent-bg about-marker"></div>
            <article>
              <p class="font-medium text-sm">Dark</p>
              <p class="text-xs mt-0.5 about-muted">Dark mode, high contrast for readability</p>
            </article>
          </div>
          <div class="about-card p-4 flex items-start gap-3" role="listitem">
            <div aria-hidden="true" class="w-5 h-5 shrink-0 mt-0.5 rounded about-accent-bg about-marker"></div>
            <article>
              <p class="font-medium text-sm">Minimal</p>
              <p class="text-xs mt-0.5 about-muted">Simple white, distraction-free</p>
            </article>
          </div>
          <div class="about-card p-4 flex items-start gap-3" role="listitem">
            <div aria-hidden="true" class="w-5 h-5 shrink-0 mt-0.5 rounded about-accent-bg about-marker"></div>
            <article>
              <p class="font-medium text-sm">Neon</p>
              <p class="text-xs mt-0.5 about-muted">Vibrant colors, energetic style</p>
            </article>
          </div>
        </div>
      </section>

      <!-- AI Voices -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">AI Voices</h2>
        </div>
        <ul class="grid gap-4 list-none p-0 m-0">
          <li class="flex items-center gap-4">
            <div aria-hidden="true" class="w-8 h-8 rounded-full flex items-center justify-center shrink-0 about-accent-bg about-marker"></div>
            <article>
              <p class="font-medium text-sm">Joanna</p>
              <p class="text-xs about-muted">US English, female, clear and professional</p>
            </article>
          </li>
          <li class="flex items-center gap-4">
            <div aria-hidden="true" class="w-8 h-8 rounded-full flex items-center justify-center shrink-0 about-accent-bg about-marker"></div>
            <article>
              <p class="font-medium text-sm">Matthew</p>
              <p class="text-xs about-muted">US English, male, warm and authoritative</p>
            </article>
          </li>
          <li class="flex items-center gap-4">
            <div aria-hidden="true" class="w-8 h-8 rounded-full flex items-center justify-center shrink-0 about-accent-bg about-marker"></div>
            <article>
              <p class="font-medium text-sm">Amy</p>
              <p class="text-xs about-muted">British English, female, polished</p>
            </article>
          </li>
          <li class="flex items-center gap-4">
            <div aria-hidden="true" class="w-8 h-8 rounded-full flex items-center justify-center shrink-0 about-accent-bg about-marker"></div>
            <article>
              <p class="font-medium text-sm">Brian</p>
              <p class="text-xs about-muted">British English, male, natural</p>
            </article>
          </li>
          <li class="flex items-center gap-4">
            <div aria-hidden="true" class="w-8 h-8 rounded-full flex items-center justify-center shrink-0 about-accent-bg about-marker"></div>
            <article>
              <p class="font-medium text-sm">Aditi</p>
              <p class="text-xs about-muted">Indian English, female, approachable</p>
            </article>
          </li>
        </ul>
      </section>

      <!-- Video Specifications -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Video Specifications</h2>
        </div>
        <ul class="grid grid-cols-1 sm:grid-cols-2 gap-4 list-none p-0 m-0">
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Resolution: 1920×1080 (Full HD)</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Format: MP4 (H.264)</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Audio: AAC</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Includes: Question slides, answer reveals, AI narration</span>
          </li>
          <li class="flex items-center gap-3">
            <div aria-hidden="true" class="w-4 h-4 shrink-0 rounded-full about-accent-bg about-marker"></div>
            <span class="text-sm about-muted">Automatic thumbnail generation</span>
          </li>
        </ul>
      </section>

      <!-- Use Cases -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Use Cases</h2>
        </div>
        <ul class="grid gap-4 list-none p-0 m-0">
          <li class="flex items-start gap-4">
            <div aria-hidden="true" class="w-10 h-10 rounded-lg flex items-center justify-center shrink-0 about-accent-bg about-marker"></div>
            <article>
              <p class="font-medium text-sm">Teachers &amp; Educators</p>
              <p class="text-xs mt-0.5 about-muted">Create revision videos for students</p>
            </article>
          </li>
          <li class="flex items-start gap-4">
            <div aria-hidden="true" class="w-10 h-10 rounded-lg flex items-center justify-center shrink-0 about-accent-bg about-marker"></div>
            <article>
              <p class="font-medium text-sm">YouTube Creators</p>
              <p class="text-xs mt-0.5 about-muted">Scale quiz/trivia content production</p>
            </article>
          </li>
          <li class="flex items-start gap-4">
            <div aria-hidden="true" class="w-10 h-10 rounded-lg flex items-center justify-center shrink-0 about-accent-bg about-marker"></div>
            <article>
              <p class="font-medium text-sm">E-learning Platforms</p>
              <p class="text-xs mt-0.5 about-muted">Automate video course creation</p>
            </article>
          </li>
          <li class="flex items-start gap-4">
            <div aria-hidden="true" class="w-10 h-10 rounded-lg flex items-center justify-center shrink-0 about-accent-bg about-marker"></div>
            <article>
              <p class="font-medium text-sm">Corporate Trainers</p>
              <p class="text-xs mt-0.5 about-muted">Turn compliance quizzes into video assessments</p>
            </article>
          </li>
          <li class="flex items-start gap-4">
            <div aria-hidden="true" class="w-10 h-10 rounded-lg flex items-center justify-center shrink-0 about-accent-bg about-marker"></div>
            <article>
              <p class="font-medium text-sm">Students</p>
              <p class="text-xs mt-0.5 about-muted">Make study materials more engaging</p>
            </article>
          </li>
        </ul>
      </section>

      <!-- Technology Section -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Built With</h2>
        </div>
        <ul class="grid grid-cols-2 sm:grid-cols-3 gap-4 list-none p-0 m-0">
          <li class="about-card p-4 text-center">
            <p class="font-medium text-sm">Angular 20</p>
            <p class="text-xs mt-1 about-muted">Frontend</p>
          </li>
          <li class="about-card p-4 text-center">
            <p class="font-medium text-sm">Go</p>
            <p class="text-xs mt-1 about-muted">Backend</p>
          </li>
          <li class="about-card p-4 text-center">
            <p class="font-medium text-sm">AWS Lambda</p>
            <p class="text-xs mt-1 about-muted">Compute</p>
          </li>
          <li class="about-card p-4 text-center">
            <p class="font-medium text-sm">Amazon Polly</p>
            <p class="text-xs mt-1 about-muted">Narration</p>
          </li>
          <li class="about-card p-4 text-center">
            <p class="font-medium text-sm">FFmpeg</p>
            <p class="text-xs mt-1 about-muted">Video</p>
          </li>
          <li class="about-card p-4 text-center">
            <p class="font-medium text-sm">DynamoDB</p>
            <p class="text-xs mt-1 about-muted">Database</p>
          </li>
          <li class="about-card p-4 text-center">
            <p class="font-medium text-sm">Amplify Hosting</p>
            <p class="text-xs mt-1 about-muted">CDN</p>
          </li>
          <li class="about-card p-4 text-center">
            <p class="font-medium text-sm">Step Functions</p>
            <p class="text-xs mt-1 about-muted">Orchestration</p>
          </li>
          <li class="about-card p-4 text-center">
            <p class="font-medium text-sm">GSAP</p>
            <p class="text-xs mt-1 about-muted">Animations</p>
          </li>
        </ul>
      </section>

      <!-- Team / Creator Section -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Creator</h2>
        </div>
        <p class="leading-relaxed about-muted">
          Indifferent is built and maintained by a passionate developer focused on making
          content creation accessible to everyone. We believe that creating engaging video
          content shouldn't require expensive software or video editing skills.
        </p>
      </section>

      <!-- Contact Section -->
      <section class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div aria-hidden="true" class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg about-marker"></div>
          <h2 class="text-2xl font-semibold about-heading">Get In Touch</h2>
        </div>
        <p class="mb-4 about-muted">
          Have questions, feedback, or feature requests? We'd love to hear from you.
        </p>
        <div class="flex flex-wrap gap-4">
          <a href="https://github.com/RahulKumar1119/indifferent/issues" target="_blank" rel="noopener" class="about-cta-btn">
            <div aria-hidden="true" class="w-4 h-4 rounded-full about-marker"></div>
            Open an Issue on GitHub
          </a>
        </div>
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
export class AboutComponent {}
