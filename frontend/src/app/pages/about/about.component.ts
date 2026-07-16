import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';

@Component({
  selector: 'app-about',
  standalone: true,
  imports: [RouterLink, LucideDynamicIcon],
  template: `
    <div class="max-w-4xl mx-auto px-6 py-12">
      <!-- Header -->
      <div class="text-center mb-12">
        <h1 class="text-4xl font-bold mb-4 shimmer-text">About Indifferent</h1>
        <p class="text-lg text-[hsl(var(--muted-foreground))]">
          Transforming text-based quizzes into professional video content
        </p>
      </div>

      <!-- Mission Section -->
      <div class="glass-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div class="w-10 h-10 rounded-full bg-[hsl(var(--primary))]/10 flex items-center justify-center">
            <svg lucideIcon="video" [size]="20" class="text-[hsl(var(--primary))]"></svg>
          </div>
          <h2 class="text-2xl font-semibold">Our Mission</h2>
        </div>
        <p class="text-[hsl(var(--muted-foreground))] leading-relaxed">
          Indifferent makes it effortless to convert your multiple-choice quiz files into engaging,
          YouTube-ready video content. Whether you're an educator, content creator, or training
          professional, our platform automates the entire video production process — from parsing
          your questions to generating narrated, animated videos with countdown timers and answer reveals.
        </p>
      </div>

      <!-- How It Works -->
      <div class="glass-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div class="w-10 h-10 rounded-full bg-[hsl(var(--primary))]/10 flex items-center justify-center">
            <svg lucideIcon="zap" [size]="20" class="text-[hsl(var(--primary))]"></svg>
          </div>
          <h2 class="text-2xl font-semibold">How It Works</h2>
        </div>
        <div class="grid gap-6">
          @for (step of steps; track step.number) {
            <div class="flex gap-4 items-start">
              <div class="w-8 h-8 rounded-full bg-[hsl(var(--primary))] text-[hsl(var(--primary-foreground))] flex items-center justify-center font-bold text-sm shrink-0">
                {{ step.number }}
              </div>
              <div>
                <h3 class="font-medium mb-1">{{ step.title }}</h3>
                <p class="text-sm text-[hsl(var(--muted-foreground))]">{{ step.description }}</p>
              </div>
            </div>
          }
        </div>
      </div>

      <!-- Technology Section -->
      <div class="glass-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-6">
          <div class="w-10 h-10 rounded-full bg-[hsl(var(--primary))]/10 flex items-center justify-center">
            <svg lucideIcon="settings" [size]="20" class="text-[hsl(var(--primary))]"></svg>
          </div>
          <h2 class="text-2xl font-semibold">Built With</h2>
        </div>
        <div class="grid grid-cols-2 sm:grid-cols-3 gap-4">
          @for (tech of technologies; track tech.name) {
            <div class="spotlight-card glass-card p-4 text-center">
              <p class="font-medium text-sm">{{ tech.name }}</p>
              <p class="text-xs text-[hsl(var(--muted-foreground))] mt-1">{{ tech.role }}</p>
            </div>
          }
        </div>
      </div>

      <!-- Team / Creator Section -->
      <div class="glass-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div class="w-10 h-10 rounded-full bg-[hsl(var(--primary))]/10 flex items-center justify-center">
            <svg lucideIcon="user" [size]="20" class="text-[hsl(var(--primary))]"></svg>
          </div>
          <h2 class="text-2xl font-semibold">Creator</h2>
        </div>
        <p class="text-[hsl(var(--muted-foreground))] leading-relaxed">
          Indifferent is built and maintained by a passionate developer focused on making
          content creation accessible to everyone. We believe that creating engaging video
          content shouldn't require expensive software or video editing skills.
        </p>
      </div>

      <!-- Contact Section -->
      <div class="glass-card p-8 mb-8">
        <div class="flex items-center gap-3 mb-4">
          <div class="w-10 h-10 rounded-full bg-[hsl(var(--primary))]/10 flex items-center justify-center">
            <svg lucideIcon="help-circle" [size]="20" class="text-[hsl(var(--primary))]"></svg>
          </div>
          <h2 class="text-2xl font-semibold">Get In Touch</h2>
        </div>
        <p class="text-[hsl(var(--muted-foreground))] mb-4">
          Have questions, feedback, or feature requests? We'd love to hear from you.
        </p>
        <div class="flex flex-wrap gap-4">
          <a href="mailto:support@indifferent.fun" class="glow-btn text-sm">
            <svg lucideIcon="mail" [size]="16"></svg>
            support&#64;indifferent.fun
          </a>
          <a routerLink="/help" class="glow-btn text-sm !bg-[hsl(var(--secondary))]">
            <svg lucideIcon="help-circle" [size]="16"></svg>
            Help Center
          </a>
        </div>
      </div>

      <!-- Back to Home -->
      <div class="text-center">
        <a routerLink="/landing" class="text-[hsl(var(--primary))] hover:underline text-sm">
          ← Back to Home
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

  technologies = [
    { name: 'Angular 20', role: 'Frontend' },
    { name: 'Go', role: 'Backend' },
    { name: 'AWS Lambda', role: 'Compute' },
    { name: 'Amazon Polly', role: 'Narration' },
    { name: 'FFmpeg', role: 'Video' },
    { name: 'DynamoDB', role: 'Database' },
    { name: 'S3 + CloudFront', role: 'Storage & CDN' },
    { name: 'Step Functions', role: 'Orchestration' },
    { name: 'GSAP', role: 'Animations' },
  ];
}
