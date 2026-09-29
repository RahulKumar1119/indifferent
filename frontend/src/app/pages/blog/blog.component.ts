import { Component, AfterViewInit } from '@angular/core';
import { RouterLink } from '@angular/router';

interface Post {
  route: string;
  title: string;
  excerpt: string;
  date: string;
  read: string;
  category: string;
  image: string;
}

@Component({
  selector: 'app-blog',
  standalone: true,
  imports: [RouterLink],
  styles: [`
    .serif { font-family: 'Cormorant Garamond', 'Playfair Display', Georgia, serif; }
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
    .reveal { opacity: 0; transform: translateY(28px); transition: opacity .8s cubic-bezier(0.23,1,0.32,1), transform .8s cubic-bezier(0.23,1,0.32,1); }
    .reveal.is-visible { opacity: 1; transform: none; }
    .card-lift { transition: transform .35s cubic-bezier(0.23,1,0.32,1), box-shadow .35s; }
    .card-lift:hover { transform: translateY(-6px); }
    .card-lift:active { transform: translateY(-1px) scale(.99); }
    .btn-primary { transition: transform .2s cubic-bezier(0.23,1,0.32,1), background-color .2s; }
    .btn-primary:hover { transform: translateY(-1px); }
    .btn-primary:active { transform: translateY(1px) scale(.98); }
    .grain::before {
      content: ''; position: fixed; inset: 0; z-index: 60; pointer-events: none; opacity: .05;
      background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='2'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='0.5'/%3E%3C/svg%3E");
    }
    @media (prefers-reduced-motion: reduce) {
      .reveal { opacity: 1; transform: none; transition: none; }
      .card-lift, .btn-primary { transition: none; }
    }
  `],
  template: `
    <div class="grain sans bg-[#FAF7F2] text-[#1A1714] antialiased">

      <!-- Floating island nav (matches landing) -->
      <header class="fixed top-4 left-1/2 -translate-x-1/2 z-50 w-[min(1120px,calc(100%-2rem))]">
        <nav class="flex items-center justify-between h-14 pl-5 pr-2 rounded-full bg-[#0F0E0B]/90 backdrop-blur-xl border border-white/10 shadow-[0_8px_30px_rgba(20,15,10,0.25)]">
          <a routerLink="/" class="flex items-center gap-2 text-[#FAF7F2]">
            <span class="font-semibold tracking-tight text-[17px]">Indifferent<span class="text-[#D96C3D]">.</span></span>
          </a>
          <div class="hidden md:flex items-center gap-7 text-[13.5px] text-white/70">
            <a routerLink="/login" class="hover:text-white transition-colors">Templates</a>
            <a routerLink="/" fragment="process" class="hover:text-white transition-colors">Process</a>
            <a routerLink="/" fragment="archive" class="hover:text-white transition-colors">Archive</a>
            <a routerLink="/blog" class="text-white font-medium">Journal</a>
          </div>
          <a routerLink="/login" class="btn-primary inline-flex items-center gap-2 h-10 pl-5 pr-1.5 rounded-full bg-[#FAF7F2] text-[#141310] text-[13.5px] font-semibold">
            Start creating
            <span class="w-7 h-7 rounded-full bg-[#1A1714] text-white flex items-center justify-center text-sm leading-none">↗</span>
          </a>
        </nav>
      </header>

      <!-- HERO · dark opening block -->
      <section class="relative overflow-hidden bg-[#0F0E0B] text-[#F4EFE6] pt-32 pb-16 md:pb-20">
        <img src="https://picsum.photos/seed/indifferent-journal-hero/1800/900" alt="" aria-hidden="true"
          class="absolute inset-0 w-full h-full object-cover opacity-30">
        <div class="absolute inset-0" style="background: radial-gradient(ellipse at 20% 15%, rgba(217,108,61,0.20), transparent 55%), linear-gradient(to bottom, rgba(15,14,11,0.55), rgba(15,14,11,0.93));"></div>
        <div class="relative w-[min(1240px,100%)] mx-auto px-5 md:px-10 grid grid-cols-1 lg:grid-cols-[1.1fr_0.9fr] gap-10 items-end">
          <div>
            <p class="inline-flex items-center gap-2 rounded-full px-3 py-1 text-[10.5px] uppercase tracking-[0.22em] border border-white/15 text-white/70">
              <span class="w-1.5 h-1.5 rounded-full bg-[#D96C3D]"></span> Journal &amp; Notes
            </p>
            <h1 class="serif mt-5 font-medium leading-[0.95] tracking-[-0.02em] text-[clamp(3rem,8vw,5.6rem)] text-balance">
              Notes on<br><em class="font-light italic">video.</em>
            </h1>
            <p class="mt-6 text-[17px] leading-relaxed text-white/70 max-w-[46ch]">
              Essays on pacing, narration, and publishing quiz films.
            </p>
          </div>
          <div class="lg:justify-self-end w-full max-w-[420px]">
            <label for="journal-search" class="sr-only">Search essays</label>
            <div class="rounded-[20px] p-2 bg-white/[0.06] border border-white/12 backdrop-blur flex items-center gap-2">
              <span aria-hidden="true" class="pl-3 text-white/50">⌕</span>
              <input id="journal-search" type="search" placeholder="Search essays…"
                (input)="onSearch($event)"
                class="flex-1 bg-transparent h-11 text-[15px] placeholder:text-white/35 outline-none">
            </div>
            <div class="mt-4 flex flex-wrap gap-2">
              @for (c of categories; track c) {
                <button type="button" (click)="setCategory(c)"
                  [class]="activeCategory === c
                    ? 'rounded-full bg-[#E8E0D2] text-[#141310] px-4 py-1.5 text-[13px] font-semibold'
                    : 'rounded-full border border-white/15 text-white/70 px-4 py-1.5 text-[13px] hover:border-white/40 hover:text-white transition-colors'">
                  {{ c }}
                </button>
              }
            </div>
          </div>
        </div>
      </section>

      <main class="bg-[#FAF7F2]">
        <div class="w-[min(1240px,100%)] mx-auto px-5 md:px-10 py-16 md:py-24">

          @if (filteredPosts.length === 0) {
            <div class="reveal is-visible rounded-[20px] border border-black/10 bg-white p-12 text-center">
              <p class="serif text-3xl">No essays found.</p>
              <p class="mt-2 text-[#6B6560]">Try a different search or category.</p>
              <button type="button" (click)="resetFilters()"
                class="btn-primary mt-6 inline-flex items-center rounded-full bg-[#1A1714] text-white px-6 py-3 text-[14px] font-semibold">Clear filters</button>
            </div>
          } @else {
            <!-- Featured essay -->
            @if (showFeatured()) {
              <a [routerLink]="filteredPosts[0].route"
                class="reveal card-lift group grid grid-cols-1 lg:grid-cols-2 overflow-hidden rounded-[20px] bg-white border border-black/[0.07]">
                <div class="relative min-h-[280px] lg:min-h-[380px] overflow-hidden">
                  <img [src]="filteredPosts[0].image" [alt]="filteredPosts[0].title" class="absolute inset-0 w-full h-full object-cover group-hover:scale-[1.03] transition-transform duration-700">
                  <span class="absolute top-4 left-4 rounded-full bg-black/55 backdrop-blur text-white text-[12px] px-3 py-1">Featured · {{ filteredPosts[0].category }}</span>
                </div>
                <div class="p-8 md:p-12 flex flex-col justify-center">
                  <p class="text-[12px] uppercase tracking-[0.2em] text-[#6B6560]">{{ filteredPosts[0].date }} · {{ filteredPosts[0].read }}</p>
                  <h2 class="serif mt-3 font-medium leading-[1.02] tracking-[-0.01em] text-[clamp(1.8rem,3.5vw,2.8rem)]">{{ filteredPosts[0].title }}</h2>
                  <p class="mt-4 text-[15.5px] text-[#6B6560] leading-relaxed">{{ filteredPosts[0].excerpt }}</p>
                  <span class="mt-6 inline-flex w-max items-center gap-2 rounded-full bg-[#1A1714] text-white px-6 py-3 text-[14px] font-semibold">Read essay <span aria-hidden="true">→</span></span>
                </div>
              </a>
            }

            <!-- Archive grid -->
            <div class="mt-8 grid grid-cols-1 md:grid-cols-2 gap-5">
              @for (p of gridPosts(); track p.route) {
                <a [routerLink]="p.route" class="reveal card-lift group rounded-[20px] overflow-hidden bg-white border border-black/[0.07] flex flex-col">
                  <div class="relative aspect-[16/9] overflow-hidden">
                    <img [src]="p.image" [alt]="p.title" loading="lazy" class="w-full h-full object-cover group-hover:scale-[1.03] transition-transform duration-700">
                    <span class="absolute top-3 left-3 rounded-full bg-black/55 backdrop-blur text-white text-[11.5px] px-3 py-1">{{ p.category }}</span>
                  </div>
                  <div class="p-6 md:p-7 flex flex-col flex-1">
                    <p class="text-[12px] uppercase tracking-[0.18em] text-[#6B6560]">{{ p.date }} · {{ p.read }}</p>
                    <h3 class="serif mt-2 text-[26px] leading-[1.05]">{{ p.title }}</h3>
                    <p class="mt-2 text-[14.5px] text-[#6B6560] leading-relaxed flex-1">{{ p.excerpt }}</p>
                    <span class="mt-4 text-[14px] font-semibold underline underline-offset-8 decoration-[#1A1714]/25 group-hover:decoration-[#1A1714] transition">Read essay →</span>
                  </div>
                </a>
              }
            </div>
          }

          <!-- Format strip -->
          <div class="reveal mt-10 rounded-[20px] p-8 md:p-10 bg-[#1E3A2A] text-[#EDE8DB] flex flex-col md:flex-row md:items-center gap-6 justify-between">
            <div>
              <p class="text-[12px] uppercase tracking-[0.2em] text-white/60">New to the format?</p>
              <p class="serif mt-2 text-[28px] leading-tight">Format your TXT file once.<br>Render forever.</p>
            </div>
            <a routerLink="/blog/quiz-file-format-guide" class="btn-primary inline-flex w-max items-center gap-2 rounded-full bg-[#E8E0D2] text-[#141310] px-7 py-3.5 font-semibold text-[14.5px]">Open format guide →</a>
          </div>
        </div>
      </main>

      <!-- CLOSING dark block -->
      <section class="relative overflow-hidden bg-[#0F0E0B] text-[#F4EFE6]">
        <div class="relative w-[min(1240px,100%)] mx-auto px-5 md:px-10 pt-20 pb-10">
          <h2 class="reveal serif font-medium text-center leading-[1.0] text-[clamp(2rem,5vw,3.6rem)] max-w-[22ch] mx-auto text-balance">Get the next essay first.</h2>
          <form (submit)="$event.preventDefault()" class="reveal mt-8 mx-auto max-w-[520px] rounded-[20px] p-2 bg-white/[0.06] border border-white/10 backdrop-blur flex flex-col sm:flex-row gap-2">
            <label for="blog-newsletter-email" class="sr-only">Email address</label>
            <input id="blog-newsletter-email" type="email" required placeholder="you@studio.com"
              class="flex-1 rounded-[12px] bg-transparent px-5 h-12 text-[15px] placeholder:text-white/35 outline-none border border-transparent focus:border-white/30">
            <button type="submit" class="btn-primary rounded-full bg-[#E8E0D2] text-[#141310] px-7 h-12 font-semibold text-[14.5px]">Subscribe</button>
          </form>
          <footer class="mt-16 border-t border-white/10 pt-10">
            <p class="serif text-center leading-none tracking-[-0.03em] text-[clamp(3rem,13vw,10rem)] text-[#F4EFE6]/95 select-none">INDIFFERENT</p>
            <div class="mt-10 flex flex-col sm:flex-row justify-between gap-3 text-[12.5px] text-white/40">
              <span>© {{ currentYear }} Indifferent. All rights reserved.</span>
              <span class="flex gap-5">
                <a routerLink="/" class="hover:text-white">Home</a>
                <a routerLink="/about" class="hover:text-white">About</a>
                <a routerLink="/privacy" class="hover:text-white">Privacy</a>
                <a routerLink="/terms" class="hover:text-white">Terms</a>
              </span>
            </div>
          </footer>
        </div>
      </section>
    </div>
  `,
})
export class BlogComponent implements AfterViewInit {
  currentYear = new Date().getFullYear();

  categories = ['All', 'YouTube', 'Teaching', 'Formatting', 'Templates', 'Narration'];
  activeCategory = 'All';
  query = '';

  posts: Post[] = [
    {
      route: '/blog/quiz-videos-youtube',
      title: 'How to Create Quiz Videos for YouTube',
      excerpt: 'The complete process behind quiz films that earn views, comments, and subscribers — pacing, reveals, and retention.',
      date: 'Jan 15, 2025',
      read: '6 min read',
      category: 'YouTube',
      image: 'https://picsum.photos/seed/indifferent-b-youtube/1200/700',
    },
    {
      route: '/blog/educational-video-best-practices',
      title: 'Best Practices for Educational Video Content',
      excerpt: 'Proven strategies for films that improve retention and keep learners watching to the final reveal.',
      date: 'Jan 10, 2025',
      read: '7 min read',
      category: 'Teaching',
      image: 'https://picsum.photos/seed/indifferent-b-teaching/800/600',
    },
    {
      route: '/blog/quiz-file-format-guide',
      title: 'Quiz File Format Guide: TXT Formatting Tips',
      excerpt: 'How to format questions, options, and answer keys so parsing succeeds on the first upload.',
      date: 'Jan 5, 2025',
      read: '6 min read',
      category: 'Formatting',
      image: 'https://picsum.photos/seed/indifferent-b-format/800/600',
    },
    {
      route: '/blog/video-template-comparison',
      title: 'Video Template Comparison: Which Style Fits?',
      excerpt: 'All six themes compared — which grade, type scale, and timer suits your audience and subject.',
      date: 'Dec 28, 2024',
      read: '7 min read',
      category: 'Templates',
      image: 'https://picsum.photos/seed/indifferent-b-templates/800/600',
    },
    {
      route: '/blog/ai-narration-guide',
      title: 'AI Narration for Videos: A Complete Guide',
      excerpt: 'Everything about AI voiceovers — choosing voices, scoring countdowns, and mixing narration beds.',
      date: 'Dec 20, 2024',
      read: '6 min read',
      category: 'Narration',
      image: 'https://picsum.photos/seed/indifferent-b-narration/800/600',
    },
  ];

  get filteredPosts(): Post[] {
    const q = this.query.trim().toLowerCase();
    return this.posts.filter((p) => {
      const inCategory = this.activeCategory === 'All' || p.category === this.activeCategory;
      const inQuery = !q || (p.title + ' ' + p.excerpt).toLowerCase().includes(q);
      return inCategory && inQuery;
    });
  }

  showFeatured(): boolean {
    return this.activeCategory === 'All' && !this.query.trim() && this.filteredPosts.length > 0;
  }

  gridPosts(): Post[] {
    return this.showFeatured() ? this.filteredPosts.slice(1) : this.filteredPosts;
  }

  setCategory(c: string): void {
    this.activeCategory = c;
    this.revealAll();
  }

  onSearch(e: Event): void {
    this.query = (e.target as HTMLInputElement).value;
  }

  resetFilters(): void {
    this.activeCategory = 'All';
    this.query = '';
    const input = document.getElementById('journal-search') as HTMLInputElement | null;
    if (input) input.value = '';
    this.revealAll();
  }

  ngAfterViewInit(): void {
    this.observeReveals();
  }

  private observeReveals(): void {
    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    const els = Array.from(document.querySelectorAll('.reveal'));
    if (!('IntersectionObserver' in window) || reduce) {
      els.forEach((e) => e.classList.add('is-visible'));
      return;
    }
    const io = new IntersectionObserver(
      (entries) => entries.forEach((en) => {
        if (en.isIntersecting) { en.target.classList.add('is-visible'); io.unobserve(en.target); }
      }),
      { threshold: 0.12, rootMargin: '0px 0px -8% 0px' },
    );
    els.forEach((e) => io.observe(e));
  }

  private revealAll(): void {
    // New cards enter after filter change — observe them on next frame
    requestAnimationFrame(() => this.observeReveals());
  }
}
