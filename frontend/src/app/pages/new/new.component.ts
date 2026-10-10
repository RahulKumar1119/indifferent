import { AfterViewInit, Component, ElementRef, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import gsap from 'gsap';

const HERO_GIF = '/screenshots/shorts-hero.gif';
const HERO_STILL = '/screenshots/shorts-hero-still.png';

@Component({
  selector: 'app-new',
  standalone: true,
  imports: [CommonModule, RouterLink, LucideDynamicIcon],
  template: `
    <div class="sans bg-[#FAF7F2] text-[#1A1714] antialiased min-h-[100dvh]">
      <main class="max-w-6xl mx-auto px-4 pt-20 pb-16">
        <p class="text-center text-[11.5px] uppercase tracking-[0.22em] text-[#6B6560]">What do you want to create?</p>
        <h1 class="mt-3 text-center font-medium tracking-[-0.02em] leading-[1.02] text-[clamp(2.2rem,5vw,3.6rem)]">
          Turn your content <span class="font-light italic">into anything.</span>
        </h1>
        <p class="mt-4 text-center text-[15.5px] text-[#6B6560] max-w-[60ch] mx-auto leading-relaxed">
          One studio, three tools. Upload once and let AI do the tedious parts, or finish in your browser, free.
        </p>

        <div class="mt-10 grid grid-cols-1 md:grid-cols-4 gap-5">
          <!-- Hero: Shorts (2×1) -->
          <a routerLink="/shorts"
            class="picker-card card-lift rounded-[20px] border border-black/10 bg-[#0F0E0B] text-[#F4EFE6] p-0 overflow-hidden md:col-span-2 flex flex-col sm:flex-row hover:border-black/30 transition-colors"
            (mouseenter)="heroHover = canHover"
            (mouseleave)="heroHover = false">
            <img
              [src]="heroHover ? heroGif : heroStill"
              alt="Screen recording of the AI Shorts upload page scrolling"
              class="w-full sm:w-[46%] h-56 sm:h-auto object-cover object-top"
              loading="eager"
            />
            <div class="p-6 md:p-7 flex flex-col flex-1">
              <span class="inline-flex items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-white/50">
                <svg lucideIcon="scissors" [size]="14" class="picker-icon"></svg>
                Vertical clips
              </span>
              <h2 class="mt-1 font-semibold text-[22px]">AI Shorts Generator</h2>
              <p class="mt-2 text-[14.5px] leading-relaxed text-white/65 flex-1">Upload once → get ranked 9:16 clips.</p>
              <span class="cta-pulse mt-4 inline-flex items-center justify-center gap-1.5 h-14 md:h-auto w-full sm:w-auto px-6 md:py-2.5 rounded-full bg-[#BC5227] text-white text-[14.5px] font-semibold">
                Make shorts <span aria-hidden="true">→</span>
              </span>
            </div>
          </a>

          <!-- Quiz (1×1) -->
          <a routerLink="/projects/new"
            class="picker-card card-lift rounded-[20px] border border-black/10 bg-white p-0 overflow-hidden flex flex-col hover:border-black/25 transition-colors">
            <img
              src="/screenshots/quiz-create.png"
              alt="Quiz project creation form with template picker"
              class="w-full h-44 object-cover object-top"
              loading="lazy"
            />
            <div class="p-6 flex flex-col flex-1">
              <span class="inline-flex items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-[#6B6560]">
                <svg lucideIcon="file-text" [size]="14" class="picker-icon"></svg>
                Quiz films
              </span>
              <h2 class="mt-1 font-semibold text-[18px]">TXT to Quiz Video</h2>
              <p class="mt-2 text-[14px] leading-relaxed text-[#6B6560] flex-1">Paste a text file → get a video.</p>
              <span class="mt-4 inline-flex items-center justify-center gap-1.5 h-14 md:h-auto w-full sm:w-auto px-6 md:py-2.5 rounded-full bg-[#1A1714] text-white text-[14px] font-semibold">
                Create video <span aria-hidden="true">→</span>
              </span>
            </div>
          </a>

          <!-- Watermark (1×1) -->
          <a routerLink="/tools/add-watermark"
            class="picker-card card-lift rounded-[20px] border border-black/10 bg-white p-0 overflow-hidden flex flex-col hover:border-black/25 transition-colors">
            <img
              src="/screenshots/watermark-tool.png"
              alt="Watermark tool drop zone"
              class="w-full h-44 object-cover object-top"
              loading="lazy"
            />
            <div class="p-6 flex flex-col flex-1">
              <span class="inline-flex items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-[#6B6560]">
                <svg lucideIcon="stamp" [size]="14" class="picker-icon"></svg>
                Free tool
              </span>
              <h2 class="mt-1 font-semibold text-[18px]">Watermark Tool</h2>
              <p class="mt-2 text-[14px] leading-relaxed text-[#6B6560] flex-1">Drag, position, download. No signup.</p>
              <span class="mt-4 inline-flex items-center justify-center gap-1.5 h-14 md:h-auto w-full sm:w-auto px-6 md:py-2.5 rounded-full bg-[#1A1714] text-white text-[14px] font-semibold">
                Open tool <span aria-hidden="true">→</span>
              </span>
            </div>
          </a>
        </div>

        <p class="mt-8 text-center text-[13.5px] text-[#6B6560]">
          Not sure? <a routerLink="/dashboard" class="font-semibold text-[#1A1714] underline underline-offset-4">Go to your dashboard</a>
        </p>
      </main>
    </div>
  `,
  styles: [`
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
    .card-lift { transition: transform .35s cubic-bezier(0.23,1,0.32,1), box-shadow .35s; }
    /* Hover physics only where hover truly exists: no tap-hover on mobile. */
    @media (hover: hover) {
      .card-lift:hover {
        transform: scale(1.02);
        box-shadow: 0 10px 15px -3px rgba(0,0,0,0.1);
      }
      .picker-card:hover .picker-icon {
        transform: rotate(15deg);
      }
    }
    .picker-icon {
      display: inline-flex;
      transition: transform .2s ease;
    }
    /* Primary CTA: subtle 1.5s indigo glow pulse. */
    @keyframes cta-pulse {
      0%, 100% { box-shadow: 0 0 0 0 rgba(99,102,241,0); }
      50% { box-shadow: 0 0 18px 2px rgba(99,102,241,0.55); }
    }
    .cta-pulse { animation: cta-pulse 1.5s ease-in-out infinite; }
    @media (prefers-reduced-motion: reduce) {
      .cta-pulse { animation: none; }
    }
  `],
})
export class NewComponent implements AfterViewInit {
  private readonly host = inject(ElementRef);

  readonly heroGif = HERO_GIF;
  readonly heroStill = HERO_STILL;
  heroHover = false;
  readonly canHover =
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(hover: hover)').matches &&
    !window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  ngAfterViewInit(): void {
    if (typeof window === 'undefined') return;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const cards: HTMLElement[] = Array.from(
      this.host.nativeElement.querySelectorAll('.picker-card'),
    );
    if (cards.length === 0) return;
    gsap.from(cards, { y: 40, opacity: 0, stagger: 0.1, duration: 0.5, ease: 'power2.out', clearProps: 'all' });
  }
}
