import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';

@Component({
  selector: 'app-about',
  standalone: true,
  imports: [RouterLink, LucideDynamicIcon],
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

    <div class="max-w-4xl mx-auto px-6 py-12 pt-24">
      <!-- Header -->
      <div class="text-center mb-12">
        <h1 class="text-4xl font-bold mb-4 about-heading">About Indifferent</h1>
        <p class="text-lg about-muted">
          Transforming text-based quizzes into professional video content
        </p>
      </div>

      <!-- Mission Section -->
      <div class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg">
            <svg lucideIcon="video" [size]="20" class="about-accent"></svg>
          </div>
          <h2 class="text-2xl font-semibold about-heading">Our Mission</h2>
        </div>
        <p class="leading-relaxed about-muted">
          Indifferent makes it effortless to convert your multiple-choice quiz files into engaging,
          YouTube-ready video content. Whether you're an educator, content creator, or training
          professional, our platform automates the entire video production process — from parsing
          your questions to generating narrated, animated videos with countdown timers and answer reveals.
        </p>
      </div>

      <!-- How It Works -->
      <div class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg">
            <svg lucideIcon="zap" [size]="20" class="about-accent"></svg>
          </div>
          <h2 class="text-2xl font-semibold about-heading">How It Works</h2>
        </div>
        <div class="grid gap-6">
          @for (step of steps; track step.number) {
            <div class="flex gap-4 items-start">
              <div class="w-8 h-8 rounded-full flex items-center justify-center font-bold text-sm shrink-0 about-accent-solid">
                {{ step.number }}
              </div>
              <div>
                <h3 class="font-medium mb-1 about-heading">{{ step.title }}</h3>
                <p class="text-sm about-muted">{{ step.description }}</p>
              </div>
            </div>
          }
        </div>
      </div>

      <!-- Supported Formats -->
      <div class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg">
            <svg lucideIcon="file-text" [size]="20" class="about-accent"></svg>
          </div>
          <h2 class="text-2xl font-semibold about-heading">Supported Formats</h2>
        </div>
        <div class="grid gap-3">
          @for (format of supportedFormats; track format.label) {
            <div class="flex items-center gap-3">
              <svg lucideIcon="check" [size]="16" class="shrink-0 about-accent"></svg>
              <span class="text-sm about-muted">{{ format.label }}</span>
            </div>
          }
        </div>
      </div>

      <!-- Video Templates -->
      <div class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg">
            <svg lucideIcon="palette" [size]="20" class="about-accent"></svg>
          </div>
          <h2 class="text-2xl font-semibold about-heading">Video Templates</h2>
        </div>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          @for (tpl of videoTemplates; track tpl.name) {
            <div class="about-card p-4 flex items-start gap-3">
              <svg [lucideIcon]="tpl.icon" [size]="18" class="shrink-0 mt-0.5 about-accent"></svg>
              <div>
                <p class="font-medium text-sm">{{ tpl.name }}</p>
                <p class="text-xs mt-0.5 about-muted">{{ tpl.description }}</p>
              </div>
            </div>
          }
        </div>
      </div>

      <!-- AI Voices -->
      <div class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg">
            <svg lucideIcon="mic" [size]="20" class="about-accent"></svg>
          </div>
          <h2 class="text-2xl font-semibold about-heading">AI Voices</h2>
        </div>
        <div class="grid gap-4">
          @for (voice of aiVoices; track voice.name) {
            <div class="flex items-center gap-4">
              <div class="w-8 h-8 rounded-full flex items-center justify-center shrink-0 about-accent-bg">
                <svg lucideIcon="volume-2" [size]="14" class="about-accent"></svg>
              </div>
              <div>
                <p class="font-medium text-sm">{{ voice.name }}</p>
                <p class="text-xs about-muted">{{ voice.description }}</p>
              </div>
            </div>
          }
        </div>
      </div>

      <!-- Video Specifications -->
      <div class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg">
            <svg lucideIcon="monitor" [size]="20" class="about-accent"></svg>
          </div>
          <h2 class="text-2xl font-semibold about-heading">Video Specifications</h2>
        </div>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          @for (spec of videoSpecs; track spec.label) {
            <div class="flex items-center gap-3">
              <svg [lucideIcon]="spec.icon" [size]="16" class="shrink-0 about-accent"></svg>
              <span class="text-sm about-muted">{{ spec.label }}</span>
            </div>
          }
        </div>
      </div>

      <!-- Use Cases -->
      <div class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg">
            <svg lucideIcon="users" [size]="20" class="about-accent"></svg>
          </div>
          <h2 class="text-2xl font-semibold about-heading">Use Cases</h2>
        </div>
        <div class="grid gap-4">
          @for (useCase of useCases; track useCase.title) {
            <div class="flex items-start gap-4">
              <div class="w-10 h-10 rounded-lg flex items-center justify-center shrink-0 about-accent-bg">
                <svg [lucideIcon]="useCase.icon" [size]="18" class="about-accent"></svg>
              </div>
              <div>
                <p class="font-medium text-sm">{{ useCase.title }}</p>
                <p class="text-xs mt-0.5 about-muted">{{ useCase.description }}</p>
              </div>
            </div>
          }
        </div>
      </div>

      <!-- Technology Section -->
      <div class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg">
            <svg lucideIcon="settings" [size]="20" class="about-accent"></svg>
          </div>
          <h2 class="text-2xl font-semibold about-heading">Built With</h2>
        </div>
        <div class="grid grid-cols-2 sm:grid-cols-3 gap-4">
          @for (tech of technologies; track tech.name) {
            <div class="about-card p-4 text-center">
              <p class="font-medium text-sm">{{ tech.name }}</p>
              <p class="text-xs mt-1 about-muted">{{ tech.role }}</p>
            </div>
          }
        </div>
      </div>

      <!-- Team / Creator Section -->
      <div class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg">
            <svg lucideIcon="user" [size]="20" class="about-accent"></svg>
          </div>
          <h2 class="text-2xl font-semibold about-heading">Creator</h2>
        </div>
        <p class="leading-relaxed about-muted">
          Indifferent is built and maintained by a passionate developer focused on making
          content creation accessible to everyone. We believe that creating engaging video
          content shouldn't require expensive software or video editing skills.
        </p>
      </div>

      <!-- Contact Section -->
      <div class="about-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div class="w-10 h-10 rounded-full flex items-center justify-center about-accent-bg">
            <svg lucideIcon="message-circle" [size]="20" class="about-accent"></svg>
          </div>
          <h2 class="text-2xl font-semibold about-heading">Get In Touch</h2>
        </div>
        <p class="mb-4 about-muted">
          Have questions, feedback, or feature requests? We'd love to hear from you.
        </p>
        <div class="flex flex-wrap gap-4">
          <a href="https://github.com/RahulKumar1119/indifferent/issues" target="_blank" rel="noopener" class="about-cta-btn">
            <svg lucideIcon="github" [size]="16"></svg>
            Open an Issue on GitHub
          </a>
        </div>
      </div>

      <!-- Back to Home -->
      <div class="text-center">
        <a routerLink="/" class="hover:underline text-sm about-accent">
          &larr; Back to Home
        </a>
      </div>
    </div>
  `,
})
export class AboutComponent {
  steps = [
    {
      number: 1,
      title: 'Upload Your Quiz File',
      description:
        'Upload a plain text (.txt) file with your multiple-choice questions. We support numbered, bulleted, and tab-indented formats.',
    },
    {
      number: 2,
      title: 'Choose Template & Voice',
      description:
        'Select a visual template for your slides and pick from 5 professional AI voices powered by Amazon Polly.',
    },
    {
      number: 3,
      title: 'Automatic Processing',
      description:
        'Our serverless pipeline parses your questions, generates animated slides, creates narration, and renders the final video.',
    },
    {
      number: 4,
      title: 'Download & Share',
      description:
        'Preview your video in-browser and download the MP4 file ready for YouTube, social media, or any platform.',
    },
  ];

  supportedFormats = [
    { label: 'Numbered questions (1. Question text)' },
    { label: 'Bulleted questions (• or - Question text)' },
    { label: 'Tab-indented answers' },
    { label: 'Multiple correct answers supported' },
    { label: 'Maximum file size: 5MB' },
  ];

  videoTemplates = [
    { name: 'Classic', icon: 'layout-template', description: 'Clean blue theme, professional look' },
    { name: 'Modern', icon: 'sparkles', description: 'Gradient backgrounds, contemporary design' },
    { name: 'Education', icon: 'graduation-cap', description: 'Warm colors, classroom-friendly' },
    { name: 'Dark', icon: 'moon', description: 'Dark mode, high contrast for readability' },
    { name: 'Minimal', icon: 'minus-square', description: 'Simple white, distraction-free' },
    { name: 'Neon', icon: 'lightbulb', description: 'Vibrant colors, energetic style' },
  ];

  aiVoices = [
    { name: 'Joanna', description: 'US English, female, clear and professional' },
    { name: 'Matthew', description: 'US English, male, warm and authoritative' },
    { name: 'Amy', description: 'British English, female, polished' },
    { name: 'Brian', description: 'British English, male, natural' },
    { name: 'Aditi', description: 'Indian English, female, approachable' },
  ];

  videoSpecs = [
    { icon: 'monitor', label: 'Resolution: 1920×1080 (Full HD)' },
    { icon: 'film', label: 'Format: MP4 (H.264)' },
    { icon: 'volume-2', label: 'Audio: AAC' },
    { icon: 'layers', label: 'Includes: Question slides, answer reveals, AI narration' },
    { icon: 'image', label: 'Automatic thumbnail generation' },
  ];

  useCases = [
    { icon: 'book-open', title: 'Teachers & Educators', description: 'Create revision videos for students' },
    { icon: 'youtube', title: 'YouTube Creators', description: 'Scale quiz/trivia content production' },
    { icon: 'laptop', title: 'E-learning Platforms', description: 'Automate video course creation' },
    { icon: 'briefcase', title: 'Corporate Trainers', description: 'Turn compliance quizzes into video assessments' },
    { icon: 'pencil', title: 'Students', description: 'Make study materials more engaging' },
  ];

  technologies = [
    { name: 'Angular 20', role: 'Frontend' },
    { name: 'Go', role: 'Backend' },
    { name: 'AWS Lambda', role: 'Compute' },
    { name: 'Amazon Polly', role: 'Narration' },
    { name: 'FFmpeg', role: 'Video' },
    { name: 'DynamoDB', role: 'Database' },
    { name: 'Amplify Hosting', role: 'CDN' },
    { name: 'Step Functions', role: 'Orchestration' },
    { name: 'GSAP', role: 'Animations' },
  ];
}
