import { Component, AfterViewInit, ElementRef, ViewChild } from '@angular/core';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import gsap from 'gsap';

@Component({
  selector: 'app-landing',
  standalone: true,
  imports: [RouterLink, LucideDynamicIcon],
  styles: [`
    .thumbnail-card {
      transition: transform 0.3s ease, box-shadow 0.3s ease;
    }
    .thumbnail-card:hover {
      transform: translateY(-4px);
    }
    .thumbnail-card:hover .aspect-video {
      box-shadow: 0 20px 40px -12px rgba(0, 0, 0, 0.3);
    }
  `],
  template: `
    <!-- Navbar -->
    <nav
      class="fixed top-0 left-0 right-0 z-50 flex items-center justify-between px-6 py-4 glass-card !rounded-none !border-t-0 !border-x-0"
    >
      <a routerLink="/" class="flex items-center gap-2 text-xl font-bold">
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

    <!-- Sample Videos Section -->
    <section class="py-24 px-6 relative" #thumbnailSection>
      <div class="max-w-6xl mx-auto">
        <h2 class="text-3xl sm:text-4xl font-bold text-center mb-4">
          See What You Can Create
        </h2>
        <p class="text-center text-[hsl(var(--muted-foreground))] mb-16 max-w-2xl mx-auto">
          Professional quiz videos generated automatically from simple text files
        </p>
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-8">
          @for (thumb of thumbnails; track thumb.title) {
            <div class="thumbnail-card group cursor-pointer" (mousemove)="onSpotlightMove($event)">
              <!-- 16:9 Thumbnail Container -->
              <div class="relative aspect-video rounded-xl overflow-hidden mb-3" [style.background]="'linear-gradient(to bottom right, ' + thumb.colors[0] + ', ' + thumb.colors[1] + ')'">
                <!-- Animated background elements -->
                <div class="absolute inset-0 flex items-center justify-center">
                  <!-- Simulated quiz slide content -->
                  <div class="text-center px-4">
                    <div class="w-16 h-2 bg-white/30 rounded mx-auto mb-3 animate-pulse"></div>
                    <div class="w-24 h-2 bg-white/20 rounded mx-auto mb-2"></div>
                    <div class="w-20 h-2 bg-white/20 rounded mx-auto mb-4"></div>
                    <div class="grid grid-cols-2 gap-2 px-4">
                      <div class="h-6 bg-white/15 rounded animate-[pulse_2s_ease-in-out_infinite]"></div>
                      <div class="h-6 bg-white/15 rounded animate-[pulse_2s_ease-in-out_0.5s_infinite]"></div>
                      <div class="h-6 bg-white/15 rounded animate-[pulse_2s_ease-in-out_1s_infinite]"></div>
                      <div class="h-6 bg-white/15 rounded animate-[pulse_2s_ease-in-out_1.5s_infinite]"></div>
                    </div>
                  </div>
                </div>
                <!-- Play button overlay -->
                <div class="absolute inset-0 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity duration-300 bg-black/20">
                  <div class="w-14 h-14 rounded-full bg-white/90 flex items-center justify-center shadow-lg animate-[pulse_2s_ease-in-out_infinite]">
                    <div class="w-0 h-0 border-t-[8px] border-t-transparent border-l-[14px] border-l-[hsl(var(--primary))] border-b-[8px] border-b-transparent ml-1"></div>
                  </div>
                </div>
                <!-- Duration badge -->
                <div class="absolute bottom-2 right-2 bg-black/70 text-white text-xs font-medium px-2 py-0.5 rounded">
                  {{ thumb.duration }}
                </div>
                <!-- Category badge -->
                <div class="absolute top-2 left-2 bg-white/20 backdrop-blur-sm text-white text-xs font-medium px-2 py-0.5 rounded">
                  {{ thumb.category }}
                </div>
              </div>
              <!-- Title -->
              <h3 class="font-medium text-sm group-hover:text-[hsl(var(--primary))] transition-colors">{{ thumb.title }}</h3>
              <p class="text-xs text-[hsl(var(--muted-foreground))] mt-0.5">Auto-generated • HD 1080p</p>
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
    <footer class="py-12 px-6 border-t border-[hsl(var(--border))]">
      <div class="max-w-6xl mx-auto">
        <div class="grid grid-cols-1 md:grid-cols-4 gap-8 mb-8">
          <!-- Brand -->
          <div class="md:col-span-1">
            <a routerLink="/" class="flex items-center gap-2 text-lg font-bold mb-3">
              <img src="logo.svg" alt="Indifferent" class="h-7">
            </a>
            <p class="text-sm text-[hsl(var(--muted-foreground))] max-w-sm">
              Transform your text-based quizzes into professional YouTube-ready videos with AI narration, custom templates, and fast serverless rendering.
            </p>
          </div>
          <!-- Product Links -->
          <div>
            <h4 class="font-semibold text-sm mb-3">Product</h4>
            <ul class="space-y-2 text-sm text-[hsl(var(--muted-foreground))]">
              <li><a routerLink="/login" class="hover:text-[hsl(var(--primary))] transition-colors">Get Started</a></li>
              <li><a routerLink="/about" class="hover:text-[hsl(var(--primary))] transition-colors">About</a></li>
              <li><a routerLink="/contact" class="hover:text-[hsl(var(--primary))] transition-colors">Contact</a></li>
            </ul>
          </div>
          <!-- Legal -->
          <div>
            <h4 class="font-semibold text-sm mb-3">Legal</h4>
            <ul class="space-y-2 text-sm text-[hsl(var(--muted-foreground))]">
              <li><a routerLink="/privacy" class="hover:text-[hsl(var(--primary))] transition-colors">Privacy Policy</a></li>
              <li><a routerLink="/terms" class="hover:text-[hsl(var(--primary))] transition-colors">Terms of Service</a></li>
            </ul>
          </div>
          <!-- Connect -->
          <div>
            <h4 class="font-semibold text-sm mb-3">Connect</h4>
            <ul class="space-y-2 text-sm text-[hsl(var(--muted-foreground))]">
              <li><a href="https://github.com/RahulKumar1119/indifferent" target="_blank" rel="noopener" class="hover:text-[hsl(var(--primary))] transition-colors">GitHub</a></li>
            </ul>
          </div>
        </div>
        <!-- Bottom bar -->
        <div class="pt-8 border-t border-[hsl(var(--border))] flex flex-col sm:flex-row items-center justify-between gap-4 text-sm text-[hsl(var(--muted-foreground))]">
          <span>&copy; {{ currentYear }} Indifferent. All rights reserved.</span>
          <span>Built with Angular, Go & AWS</span>
        </div>
      </div>
    </footer>
  `,
})
export class LandingComponent implements AfterViewInit {
  @ViewChild('heroTitle') heroTitle!: ElementRef;
  @ViewChild('heroSubtitle') heroSubtitle!: ElementRef;
  @ViewChild('heroCta') heroCta!: ElementRef;
  @ViewChild('thumbnailSection') thumbnailSection!: ElementRef;

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

  thumbnails = [
    { title: 'World History Quiz', duration: '3:24', colors: ['#6366f1', '#9333ea'], category: 'Education' },
    { title: 'Science Trivia Challenge', duration: '4:12', colors: ['#10b981', '#0d9488'], category: 'Science' },
    { title: 'Math Practice Test', duration: '2:58', colors: ['#f97316', '#dc2626'], category: 'Mathematics' },
    { title: 'English Grammar Quiz', duration: '3:45', colors: ['#3b82f6', '#06b6d4'], category: 'Language' },
    { title: 'Geography Explorer', duration: '4:30', colors: ['#ec4899', '#e11d48'], category: 'Geography' },
    { title: 'Computer Science Basics', duration: '3:15', colors: ['#8b5cf6', '#d946ef'], category: 'Technology' },
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

    // Animate thumbnails with stagger
    gsap.fromTo(
      this.thumbnailSection.nativeElement.querySelectorAll('.thumbnail-card'),
      { opacity: 0, y: 40 },
      { opacity: 1, y: 0, duration: 0.6, stagger: 0.1, delay: 0.5, ease: 'power2.out' }
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
