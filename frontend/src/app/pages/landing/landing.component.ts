import { Component, AfterViewInit, ElementRef, ViewChild } from '@angular/core';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import gsap from 'gsap';

@Component({
  selector: 'app-landing',
  standalone: true,
  imports: [RouterLink, LucideDynamicIcon],
  template: `
    <!-- Navbar -->
    <nav
      class="fixed top-0 left-0 right-0 z-50 flex items-center justify-between px-6 py-4 glass-card !rounded-none !border-t-0 !border-x-0"
    >
      <a routerLink="/landing" class="flex items-center gap-2 text-xl font-bold">
        <img src="logo.svg" alt="Indifferent" class="h-8">
      </a>
      <a routerLink="/login" class="glow-btn !py-2 !px-5 !text-sm">Sign In</a>
    </nav>

    <!-- Hero Section -->
    <section
      #heroSection
      class="min-h-screen flex items-center justify-center px-6 pt-20 relative overflow-hidden"
    >
      <div class="max-w-4xl mx-auto text-center">
        <h1
          #heroTitle
          class="text-4xl sm:text-5xl md:text-7xl font-extrabold leading-tight opacity-0"
        >
          Turn Your TXT Quizzes Into
          <span class="shimmer-text block mt-2">YouTube-Ready Videos</span>
        </h1>
        <p
          #heroSubtitle
          class="mt-8 text-lg sm:text-xl text-[hsl(var(--muted-foreground))] max-w-2xl mx-auto opacity-0"
        >
          Upload a text file with multiple-choice questions and get a professional MP4 video
          complete with narration, animations, and countdown timers — all automatically.
        </p>
        <div #heroCta class="mt-10 opacity-0">
          <a routerLink="/login" class="glow-btn !text-lg !px-10 !py-4">
            Get Started Free
            <svg lucideIcon="arrow-right" [size]="20"></svg>
          </a>
        </div>
      </div>
    </section>

    <!-- Features Section -->
    <section class="py-24 px-6 relative">
      <div class="max-w-6xl mx-auto">
        <h2 class="text-3xl sm:text-4xl font-bold text-center mb-4">
          Everything You Need
        </h2>
        <p class="text-center text-[hsl(var(--muted-foreground))] mb-16 max-w-2xl mx-auto">
          From text file to finished video in minutes. No editing skills required.
        </p>
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
          @for (feature of features; track feature.title) {
            <div
              class="glass-card spotlight-card p-6 hover:border-[hsl(var(--primary))]/30 transition-all duration-300"
              (mousemove)="onSpotlightMove($event)"
            >
              <div class="w-12 h-12 rounded-lg bg-[hsl(var(--primary))]/10 flex items-center justify-center mb-4">
                <svg [lucideIcon]="feature.icon" [size]="24" class="text-[hsl(var(--primary))]"></svg>
              </div>
              <h3 class="font-semibold text-lg mb-2">{{ feature.title }}</h3>
              <p class="text-sm text-[hsl(var(--muted-foreground))]">{{ feature.description }}</p>
            </div>
          }
        </div>
      </div>
    </section>

    <!-- CTA Section -->
    <section class="py-24 px-6 relative">
      <div class="max-w-3xl mx-auto text-center glass-card p-12">
        <h2 class="text-3xl sm:text-4xl font-bold mb-4 shimmer-text inline-block">
          Ready to Create Your First Video?
        </h2>
        <p class="text-[hsl(var(--muted-foreground))] mb-8 text-lg">
          Sign in with Google and upload your first TXT quiz. It only takes a few minutes.
        </p>
        <a routerLink="/login" class="glow-btn !text-lg !px-10 !py-4">
          Start Now
          <svg lucideIcon="zap" [size]="20"></svg>
        </a>
      </div>
    </section>

    <!-- Footer -->
    <footer class="py-8 px-6 border-t border-[hsl(var(--border))]">
      <div class="max-w-6xl mx-auto text-center text-[hsl(var(--muted-foreground))] text-sm">
        &copy; {{ currentYear }} Indifferent. All rights reserved.
      </div>
    </footer>
  `,
})
export class LandingComponent implements AfterViewInit {
  @ViewChild('heroTitle') heroTitle!: ElementRef;
  @ViewChild('heroSubtitle') heroSubtitle!: ElementRef;
  @ViewChild('heroCta') heroCta!: ElementRef;

  currentYear = new Date().getFullYear();

  features = [
    {
      icon: 'wand-2',
      title: 'Auto Format Detection',
      description: 'Supports numbered, bulleted, and tabbed question formats automatically.',
    },
    {
      icon: 'palette',
      title: 'Multiple Templates',
      description: 'Choose from Classic, Modern, Education, Dark, Minimal, and Neon themes.',
    },
    {
      icon: 'mic',
      title: 'AI Narration',
      description: 'Professional voice-over narration powered by Amazon Polly with 5 voice options.',
    },
    {
      icon: 'zap',
      title: 'Fast Rendering',
      description: 'Serverless pipeline renders your video in minutes with countdown timers and transitions.',
    },
  ];

  ngAfterViewInit(): void {
    const tl = gsap.timeline({ defaults: { ease: 'power3.out' } });

    tl.fromTo(
      this.heroTitle.nativeElement,
      { opacity: 0, y: 40 },
      { opacity: 1, y: 0, duration: 1 }
    )
      .fromTo(
        this.heroSubtitle.nativeElement,
        { opacity: 0, y: 30 },
        { opacity: 1, y: 0, duration: 0.8 },
        '-=0.4'
      )
      .fromTo(
        this.heroCta.nativeElement,
        { opacity: 0, y: 20 },
        { opacity: 1, y: 0, duration: 0.6 },
        '-=0.3'
      );
  }

  onSpotlightMove(event: MouseEvent): void {
    const card = event.currentTarget as HTMLElement;
    const rect = card.getBoundingClientRect();
    const x = ((event.clientX - rect.left) / rect.width) * 100;
    const y = ((event.clientY - rect.top) / rect.height) * 100;
    card.style.setProperty('--mouse-x', `${x}%`);
    card.style.setProperty('--mouse-y', `${y}%`);
  }
}
