import { AfterViewInit, Component, ElementRef, OnInit, PLATFORM_ID, inject } from '@angular/core';
import { CommonModule, DOCUMENT, isPlatformBrowser } from '@angular/common';
import { Meta, Title } from '@angular/platform-browser';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { setCanonical } from '../../shared/seo';

interface Feature {
  icon: string;
  title: string;
  text: string;
}

interface UseCase {
  slug: string;
  title: string;
  text: string;
}


interface Faq {
  q: string;
  a: string;
}

interface UseCaseLink {
  slug: string;
  title: string;
}

@Component({
  selector: 'app-shorts-maker',
  standalone: true,
  imports: [CommonModule, RouterLink, LucideDynamicIcon],
  styles: [`
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
    .reveal { opacity: 0; transform: translateY(28px); transition: opacity .8s cubic-bezier(0.23,1,0.32,1), transform .8s cubic-bezier(0.23,1,0.32,1); }
    .reveal.is-visible { opacity: 1; transform: none; }
    .card-lift { transition: transform .35s cubic-bezier(0.23,1,0.32,1), box-shadow .35s; }
    .card-lift:hover { transform: translateY(-6px); }
    .btn-primary { transition: transform .2s cubic-bezier(0.23,1,0.32,1), background-color .2s; }
    .btn-primary:hover { transform: translateY(-1px); }
    .btn-primary:active { transform: translateY(1px) scale(.99); }
    details.faq summary::-webkit-details-marker { display: none; }
    details.faq[open] .faq-chevron { transform: rotate(180deg); }
    .faq-chevron { transition: transform .25s; }
    @media (prefers-reduced-motion: reduce) {
      .reveal { opacity: 1; transform: none; transition: none; }
      .card-lift, .btn-primary { transition: none; }
    }
  `],
  template: `
    <div class="sans bg-[#FAF7F2] text-[#1A1714] antialiased">

      <!-- Sticky nav -->
      <header class="fixed top-4 left-1/2 -translate-x-1/2 z-50 w-[min(1120px,calc(100%-2rem))]">
        <nav class="flex items-center justify-between h-14 pl-5 pr-2 rounded-full bg-[#0F0E0B]/90 backdrop-blur-xl border border-white/10 shadow-[0_8px_30px_rgba(20,15,10,0.25)]">
          <a routerLink="/shorts-maker" class="flex items-center gap-2 text-[#FAF7F2]">
            <span class="font-semibold tracking-tight text-[17px]">Indifferent<span class="text-[#D96C3D]">.</span></span>
            <span class="hidden sm:inline text-[11px] uppercase tracking-[0.18em] text-white/50 border border-white/15 rounded-full px-2 py-0.5">Shorts</span>
          </a>
          <div class="hidden md:flex items-center gap-6 text-[13.5px] text-white/70">
            <!-- Product mega-menu -->
            <div class="relative group">
              <button type="button" class="flex items-center gap-1.5 bg-transparent border-none text-[13.5px] text-white/70 group-hover:text-white transition-colors cursor-pointer p-0">
                Product
                <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="transition-transform duration-200 group-hover:rotate-180"><path d="m6 9 6 6 6-6"/></svg>
              </button>
              <div class="absolute top-full left-1/2 -translate-x-1/2 pt-3 opacity-0 invisible translate-y-1 group-hover:opacity-100 group-hover:visible group-hover:translate-y-0 transition-all duration-200">
                <div class="w-[340px] rounded-2xl border border-white/10 bg-[#17150F] shadow-[0_20px_60px_rgba(0,0,0,0.5)] p-2">
                  <a routerLink="/shorts-maker" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                    <span class="block text-white text-[13.5px] font-medium">AI Shorts generator</span>
                    <span class="block text-white/50 text-[12px] mt-0.5">Long video or audio into ranked 9:16 clips with burned-in captions.</span>
                  </a>
                  <a routerLink="/projects" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                    <span class="block text-white text-[13.5px] font-medium">TXT to narrated video</span>
                    <span class="block text-white/50 text-[12px] mt-0.5">Quiz files into narrated video content with answer reveals.</span>
                  </a>
                  <a routerLink="/tools" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                    <span class="block text-white text-[13.5px] font-medium">Free tools</span>
                    <span class="block text-white/50 text-[12px] mt-0.5">Watermarks and more: private, in-browser utilities.</span>
                  </a>
                </div>
              </div>
            </div>
            <!-- Use cases mega-menu -->
            <div class="relative group">
              <button type="button" class="flex items-center gap-1.5 bg-transparent border-none text-[13.5px] text-white/70 group-hover:text-white transition-colors cursor-pointer p-0">
                Use Cases
                <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="transition-transform duration-200 group-hover:rotate-180"><path d="m6 9 6 6 6-6"/></svg>
              </button>
              <div class="absolute top-full left-1/2 -translate-x-1/2 pt-3 opacity-0 invisible translate-y-1 group-hover:opacity-100 group-hover:visible group-hover:translate-y-0 transition-all duration-200">
                <div class="w-[300px] rounded-2xl border border-white/10 bg-[#17150F] shadow-[0_20px_60px_rgba(0,0,0,0.5)] p-2">
                  @for (u of useCaseLinks; track u.slug) {
                    <a [routerLink]="['/use-cases', u.slug]" class="block rounded-xl px-4 py-2.5 hover:bg-white/[0.06] transition-colors">
                      <span class="block text-white text-[13.5px] font-medium">{{ u.title }}</span>
                    </a>
                  }
                </div>
              </div>
            </div>
            <!-- Resources mega-menu -->
            <div class="relative group">
              <button type="button" class="flex items-center gap-1.5 bg-transparent border-none text-[13.5px] text-white/70 group-hover:text-white transition-colors cursor-pointer p-0">
                Resources
                <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="transition-transform duration-200 group-hover:rotate-180"><path d="m6 9 6 6 6-6"/></svg>
              </button>
              <div class="absolute top-full left-1/2 -translate-x-1/2 pt-3 opacity-0 invisible translate-y-1 group-hover:opacity-100 group-hover:visible group-hover:translate-y-0 transition-all duration-200">
                <div class="w-[300px] rounded-2xl border border-white/10 bg-[#17150F] shadow-[0_20px_60px_rgba(0,0,0,0.5)] p-2">
                  <a routerLink="/blog" class="block rounded-xl px-4 py-2.5 hover:bg-white/[0.06] transition-colors"><span class="block text-white text-[13.5px] font-medium">Blog</span></a>
                  <a routerLink="/blog/quiz-videos-youtube" class="block rounded-xl px-4 py-2.5 hover:bg-white/[0.06] transition-colors"><span class="block text-white text-[13.5px] font-medium">Quiz videos for YouTube</span></a>
                  <a routerLink="/blog/ai-narration-guide" class="block rounded-xl px-4 py-2.5 hover:bg-white/[0.06] transition-colors"><span class="block text-white text-[13.5px] font-medium">AI narration guide</span></a>
                  <a routerLink="/about" class="block rounded-xl px-4 py-2.5 hover:bg-white/[0.06] transition-colors"><span class="block text-white text-[13.5px] font-medium">About</span></a>
                  <a href="#faq" class="block rounded-xl px-4 py-2.5 hover:bg-white/[0.06] transition-colors"><span class="block text-white text-[13.5px] font-medium">FAQ</span></a>
                </div>
              </div>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <a routerLink="/login" class="hidden sm:inline px-4 py-2 rounded-full text-[13.5px] text-white/80 hover:text-white transition-colors">Login</a>
            <a routerLink="/shorts" class="btn-primary px-4 py-2 rounded-full bg-[#D96C3D] hover:bg-[#BC5227] text-white text-[13.5px] font-semibold">Try For Free</a>
          </div>
        </nav>
      </header>

      <!-- Hero -->
      <section class="pt-32 md:pt-40 pb-14 px-4">
        <div class="max-w-5xl mx-auto text-center">
          <h1 class="reveal font-medium tracking-[-0.03em] leading-[0.98] text-[clamp(2.6rem,6vw,4.8rem)]">
            Turn long videos into <em class="italic font-light">ranked vertical shorts</em>
          </h1>
          <p class="reveal mt-5 text-[16.5px] leading-relaxed text-[#6B6560] max-w-[52ch] mx-auto">
            Upload a video or audio file.
            AI transcribes it, finds the best moments, and renders ranked 9:16 clips
            with burned-in captions.
          </p>
          <div class="reveal mt-8 flex flex-col sm:flex-row items-center justify-center gap-3">
            <a routerLink="/shorts" class="btn-primary w-full sm:w-auto px-7 py-3.5 rounded-full bg-[#1A1714] text-white font-semibold text-[15px] hover:bg-[#2A2620]">Get Started Now</a>
            <a routerLink="/login" class="btn-primary w-full sm:w-auto px-7 py-3.5 rounded-full border border-black/15 hover:bg-black/5 transition-colors font-semibold text-[15px]">Try for free</a>
          </div>
          <div class="reveal mt-6 flex flex-col items-center gap-2">
            <p class="text-[15px] tracking-[0.08em] text-[#D96C3D]" aria-label="Rated 5 out of 5">★★★★★</p>
            <p class="text-[13px] text-[#6B6560]">Free during beta · No credit card · 1080×1920 full-HD · H.264 + AAC · 30fps</p>
          </div>

          <!-- Product visual: CSS phone mock -->
          <div class="reveal mt-12 mx-auto w-[min(880px,100%)] rounded-[20px] border border-black/10 bg-white p-3 shadow-[0_30px_80px_rgba(20,15,10,0.12)]">
            <div class="rounded-[12px] bg-[#0F0E0B] px-4 py-3 md:px-8 md:py-6 flex flex-col md:flex-row items-center gap-6">
              <div class="relative w-[150px] md:w-[170px] shrink-0 rounded-[18px] border border-white/15 overflow-hidden bg-black" style="aspect-ratio: 9 / 16;">
                <div class="absolute inset-0" style="background: linear-gradient(160deg, #2A2620 0%, #0F0E0B 60%, #1A1714 100%);"></div>
                <span class="absolute top-2 left-2 text-[9px] font-semibold px-2 py-0.5 rounded-full bg-[#D96C3D]/90 text-white">Rank #1</span>
                <div class="absolute bottom-8 left-2 right-2 text-center">
                  <span class="inline-block text-white text-[11px] font-semibold px-1" style="text-shadow: 0 1px 3px #000, 0 0 6px #000;">never miss the moment</span>
                </div>
                <div class="absolute bottom-2 left-2 right-2 h-1 rounded-full bg-white/15 overflow-hidden">
                  <div class="h-full w-2/3 rounded-full bg-[#D96C3D]"></div>
                </div>
              </div>
              <div class="text-left text-white flex-1">
                <p class="text-[11px] uppercase tracking-[0.22em] text-white/50">AI pipeline</p>
                <p class="mt-2 text-[15px] leading-relaxed text-white/80">Transcribe → rank the most engaging moments → track the speaker → burn captions → publish.</p>
                <div class="mt-4 flex flex-wrap gap-2 text-[11.5px]">
                  <span class="rounded-full border border-white/15 px-3 py-1 text-white/70">Face tracking</span>
                  <span class="rounded-full border border-white/15 px-3 py-1 text-white/70">Blur-fill fit</span>
                  <span class="rounded-full border border-white/15 px-3 py-1 text-white/70">Word captions</span>
                  <span class="rounded-full border border-white/15 px-3 py-1 text-white/70">MP4 · AAC</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- Features -->
      <section id="features" class="py-16 px-4 scroll-mt-24">
        <div class="max-w-6xl mx-auto">
          <p class="reveal text-center text-[11.5px] uppercase tracking-[0.22em] text-[#6B6560]">Features</p>
          <h2 class="reveal mt-3 text-center font-medium tracking-[-0.02em] text-[clamp(1.8rem,3.5vw,2.8rem)] leading-tight">The new way to edit videos.<br><span class="font-light italic">Edit faster than ever before.</span></h2>
          <div class="mt-10 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
            @for (f of features; track f.title) {
              <div class="reveal card-lift rounded-[20px] border border-black/10 bg-white p-6">
                <span class="inline-flex w-10 h-10 items-center justify-center rounded-xl bg-[#D96C3D]/12 text-[#BC5227]">
                  <svg lucideIcon="{{ f.icon }}" [size]="20"></svg>
                </span>
                <h3 class="mt-4 font-semibold text-[16.5px]">{{ f.title }}</h3>
                <p class="mt-2 text-[14px] leading-relaxed text-[#6B6560]">{{ f.text }}</p>
              </div>
            }
          </div>
        </div>
      </section>

      <!-- How it works -->
      <section id="how" class="py-16 px-4 scroll-mt-24 bg-[#0F0E0B] text-[#F4EFE6]">
        <div class="max-w-6xl mx-auto">
          <p class="reveal text-center text-[11.5px] uppercase tracking-[0.22em] text-white/50">How it works</p>
          <h2 class="reveal mt-3 text-center font-medium tracking-[-0.02em] text-[clamp(1.8rem,3.5vw,2.8rem)]">Streamline your video workflow.<br><span class="font-light italic text-white/70">Simple to use, built for speed.</span></h2>
          <div class="mt-10 grid grid-cols-1 md:grid-cols-3 gap-5">
            <div class="reveal card-lift rounded-[20px] border border-white/10 bg-white/[0.05] p-6">
              <p class="text-[12px] font-semibold tracking-[0.18em] text-[#D96C3D]">STEP 1</p>
              <h3 class="mt-2 font-semibold text-[17px]">Upload a video or audio file</h3>
              <p class="mt-2 text-[14px] leading-relaxed text-white/60">MP4, MOV, MP3 or WAV up to 10 minutes. Podcasts, interviews, or phone footage. All formats work.</p>
            </div>
            <div class="reveal card-lift rounded-[20px] border border-white/10 bg-white/[0.05] p-6">
              <p class="text-[12px] font-semibold tracking-[0.18em] text-[#D96C3D]">STEP 2</p>
              <h3 class="mt-2 font-semibold text-[17px]">Let AI edit for you</h3>
              <p class="mt-2 text-[14px] leading-relaxed text-white/60">AI transcribes, ranks the most engaging moments, tracks the speaker, and burns in short caption lines automatically.</p>
            </div>
            <div class="reveal card-lift rounded-[20px] border border-white/10 bg-white/[0.05] p-6">
              <p class="text-[12px] font-semibold tracking-[0.18em] text-[#D96C3D]">STEP 3</p>
              <h3 class="mt-2 font-semibold text-[17px]">Preview and download</h3>
              <p class="mt-2 text-[14px] leading-relaxed text-white/60">Ranked clips land in your gallery with previews. Download full-HD 9:16 MP4s, ready for Shorts, Reels and TikTok.</p>
            </div>
          </div>
          <div class="reveal mt-8 text-center">
            <a routerLink="/shorts" class="btn-primary inline-block px-7 py-3.5 rounded-full bg-[#D96C3D] hover:bg-[#BC5227] text-white font-semibold text-[15px]">Get started free</a>
          </div>
        </div>
      </section>

      <!-- Use cases -->
      <section id="use-cases" class="py-16 px-4 scroll-mt-24">
        <div class="max-w-6xl mx-auto">
          <p class="reveal text-center text-[11.5px] uppercase tracking-[0.22em] text-[#6B6560]">Use cases</p>
          <h2 class="reveal mt-3 text-center font-medium tracking-[-0.02em] text-[clamp(1.8rem,3.5vw,2.8rem)]">Grow with high-performing <span class="font-light italic">short-form content.</span></h2>
          <div class="mt-10 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
            @for (u of useCases; track u.slug) {
              <a [routerLink]="['/use-cases', u.slug]" class="reveal card-lift rounded-[20px] border border-black/10 bg-white p-6 block hover:border-black/25">
                <h3 class="font-semibold text-[16.5px]">{{ u.title }}</h3>
                <p class="mt-2 text-[14px] leading-relaxed text-[#6B6560]">{{ u.text }}</p>
                <span class="mt-3 inline-block text-[13.5px] font-semibold text-[#BC5227]">Learn more →</span>
              </a>
            }
          </div>
        </div>
      </section>

      <!-- Stats band -->
      <section class="px-4 pb-4">
        <div class="reveal max-w-6xl mx-auto rounded-[20px] bg-[#1A1714] text-white px-6 py-12 md:py-14 grid grid-cols-1 sm:grid-cols-2 gap-8 text-center">
          <div><p class="text-[clamp(2.2rem,4vw,3.2rem)] font-medium tracking-tight">3</p><p class="mt-1 text-[13.5px] text-white/60">ranked clips per upload</p></div>
          <div><p class="text-[clamp(2.2rem,4vw,3.2rem)] font-medium tracking-tight">1080×1920</p><p class="mt-1 text-[13.5px] text-white/60">full-HD vertical export</p></div>
        </div>
      </section>

      <!-- FAQ -->
      <section id="faq" class="py-16 px-4 scroll-mt-24 bg-white border-y border-black/10">
        <div class="max-w-3xl mx-auto">
          <p class="reveal text-center text-[11.5px] uppercase tracking-[0.22em] text-[#6B6560]">FAQ</p>
          <h2 class="reveal mt-3 text-center font-medium tracking-[-0.02em] text-[clamp(1.8rem,3.5vw,2.8rem)]">Do you have <span class="font-light italic">any questions?</span></h2>
          <div class="mt-8 space-y-3">
            @for (f of faqs; track f.q) {
              <details class="faq reveal group rounded-[16px] border border-black/10 bg-[#FAF7F2] px-5 py-4">
                <summary class="flex items-center justify-between gap-4 cursor-pointer list-none font-semibold text-[15px]">
                  {{ f.q }}
                  <svg class="faq-chevron shrink-0 text-[#6B6560]" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m6 9 6 6 6-6"/></svg>
                </summary>
                <p class="mt-3 text-[14px] leading-relaxed text-[#6B6560]">{{ f.a }}</p>
              </details>
            }
          </div>
        </div>
      </section>

      <!-- Final CTA -->
      <section class="py-20 px-4 text-center">
        <h2 class="reveal font-medium tracking-[-0.02em] text-[clamp(2rem,4.5vw,3.4rem)] leading-tight">Start creating shorts<br><span class="font-light italic">from your long videos.</span></h2>
        <div class="reveal mt-8">
          <a routerLink="/shorts" class="btn-primary inline-block px-8 py-4 rounded-full bg-[#1A1714] text-white font-semibold text-[15px] hover:bg-[#2A2620]">Get started free</a>
        </div>
      </section>

      <!-- Footer -->
      <footer class="border-t border-black/10 px-4 py-12">
        <div class="max-w-6xl mx-auto grid grid-cols-2 md:grid-cols-5 gap-8 text-[13.5px]">
          <div class="col-span-2 md:col-span-1">
            <span class="font-semibold tracking-tight text-[16px]">Indifferent<span class="text-[#D96C3D]">.</span></span>
            <p class="mt-3 text-[#6B6560] leading-relaxed">Long video and audio to ranked 9:16 clips automatically.</p>
          </div>
          <div>
            <p class="font-semibold mb-3">Product</p>
            <ul class="space-y-2 text-[#6B6560]">
              <li><a routerLink="/shorts-maker" class="hover:text-[#1A1714]">AI Shorts generator</a></li>
              <li><a routerLink="/projects" class="hover:text-[#1A1714]">TXT to video</a></li>
              <li><a routerLink="/tools" class="hover:text-[#1A1714]">Free tools</a></li>
              <li><a routerLink="/tools/add-watermark" class="hover:text-[#1A1714]">Watermark tool</a></li>
            </ul>
          </div>
          <div>
            <p class="font-semibold mb-3">Use Cases</p>
            <ul class="space-y-2 text-[#6B6560]">
              @for (u of useCaseLinks; track u.slug) {
                <li><a [routerLink]="['/use-cases', u.slug]" class="hover:text-[#1A1714]">{{ u.title }}</a></li>
              }
            </ul>
          </div>
          <div>
            <p class="font-semibold mb-3">Resources</p>
            <ul class="space-y-2 text-[#6B6560]">
              <li><a routerLink="/blog" class="hover:text-[#1A1714]">Blog</a></li>
              <li><a routerLink="/blog/quiz-videos-youtube" class="hover:text-[#1A1714]">Quiz videos for YouTube</a></li>
              <li><a routerLink="/blog/ai-narration-guide" class="hover:text-[#1A1714]">AI narration guide</a></li>
              <li><a routerLink="/blog/educational-video-best-practices" class="hover:text-[#1A1714]">Educational video tips</a></li>
              <li><a routerLink="/about" class="hover:text-[#1A1714]">About</a></li>
              <li><a routerLink="/contact" class="hover:text-[#1A1714]">Contact</a></li>
            </ul>
          </div>
          <div>
            <p class="font-semibold mb-3">Legal</p>
            <ul class="space-y-2 text-[#6B6560]">
              <li><a routerLink="/terms" class="hover:text-[#1A1714]">Terms of Service</a></li>
              <li><a routerLink="/privacy" class="hover:text-[#1A1714]">Privacy Policy</a></li>
              <li><a routerLink="/login" class="hover:text-[#1A1714]">Sign in</a></li>
            </ul>
          </div>
        </div>
        <p class="mt-10 text-center text-[12.5px] text-[#6B6560]">© {{ currentYear }} Indifferent · Architecture of video</p>
      </footer>
    </div>
  `,
})
export class ShortsMakerComponent implements OnInit, AfterViewInit {
  private readonly title = inject(Title);
  private readonly meta = inject(Meta);
  private readonly document = inject(DOCUMENT);
  private readonly host = inject(ElementRef);
  private readonly isBrowser = isPlatformBrowser(inject(PLATFORM_ID));

  currentYear = new Date().getFullYear();


  features: Feature[] = [
    { icon: 'captions', title: 'Smart captions that keep people watching', text: 'Word-timed captions burned into every clip in short 4-word lines, styled for readability without sound.' },
    { icon: 'scissors', title: 'Magic ranking finds the best moments', text: 'AI scores every segment for engagement and keeps the top 3. You publish, not scrub timelines.' },
    { icon: 'focus', title: 'Subject tracking follows the speaker', text: 'Face detection pans the 9:16 window onto the speaker instead of blindly cropping the middle.' },
    { icon: 'frame', title: 'Blur-fill fit never cuts content', text: 'Cartoons, text cards and screen recordings scale to fit over a blurred background. Nothing gets chopped.' },
    { icon: 'gauge', title: 'Shorts-spec output, every time', text: '1080×1920, H.264 + AAC, 30fps, 48kHz, a standard format for YouTube Shorts, Reels and TikTok.' },
    { icon: 'zap', title: 'One-click pipeline', text: 'Transcribe → rank → render on auto-scaling infra. Upload, wait minutes, download ranked clips.' },
  ];

  useCases: UseCase[] = [
    { slug: 'content-creators', title: 'Content Creators', text: 'Repurpose podcasts, interviews and videos into ranked vertical shorts.' },
    { slug: 'marketing-teams', title: 'Marketing Teams', text: 'Create high-performing shorts that drive business growth.' },
    { slug: 'agencies', title: 'Agencies', text: 'Produce on-brand client content with consistent quality, fast.' },
    { slug: 'coaches', title: 'Coaches', text: 'Turn long lessons and calls into authority-building clips.' },
    { slug: 'media-companies', title: 'Media Companies', text: 'Turn long-form content into high-performing shorts at scale.' },
    { slug: 'educators', title: 'Educators', text: 'Clip the key explanation from every lecture, automatically.' },
  ];

  useCaseLinks: UseCaseLink[] = [
    { slug: 'content-creators', title: 'Content Creators' },
    { slug: 'marketing-teams', title: 'Marketing Teams' },
    { slug: 'agencies', title: 'Agencies' },
    { slug: 'coaches', title: 'Coaches' },
    { slug: 'media-companies', title: 'Media Companies' },
    { slug: 'educators', title: 'Educators' },
  ];

  faqs: Faq[] = [
    { q: 'How do I create a short?', a: 'Upload a video or audio file and the AI transcribes it, detects highlights, adds captions and formats ranked clips for short-form platforms, ready in minutes.' },
    { q: 'Can I turn a long video into multiple shorts?', a: 'Yes. Every upload produces up to 3 ranked clips from the most engaging moments, each downloadable as its own 9:16 MP4.' },
    { q: 'Are captions added automatically?', a: 'Yes. Word-level timestamps become short burned-in caption lines styled for readability without sound.' },
    { q: 'What formats and quality do I get?', a: 'MP4 with H.264 video and AAC audio at 1080×1920 and 30fps, a standard format for YouTube Shorts, Instagram Reels and TikTok.' },
    { q: 'What if my video has no people in it?', a: 'Segments without a visible subject render in blur-fill fit: the whole frame stays visible over a blurred background, so text and diagrams are never cut.' },
    { q: 'Which files can I upload?', a: 'MP4, MOV, MP3 and WAV up to 10 minutes long.' },
  ];

  ngOnInit(): void {
    const pageUrl = 'https://indifferent.fun/shorts-maker';
    const description = 'Upload a video or audio file and let AI transcribe, rank and render top 9:16 shorts with burned-in captions. Free to try.';
    this.title.setTitle('AI Shorts Maker: Long Video to Ranked Clips | Indifferent');
    this.meta.updateTag({ name: 'description', content: description });
    this.meta.updateTag({ property: 'og:type', content: 'website' });
    this.meta.updateTag({ property: 'og:title', content: 'AI Shorts Maker: Long Video to Ranked Clips | Indifferent' });
    this.meta.updateTag({ property: 'og:description', content: description });
    this.meta.updateTag({ property: 'og:url', content: pageUrl });
    setCanonical(this.document, pageUrl);
    this.meta.updateTag({ name: 'twitter:card', content: 'summary' });
    this.meta.updateTag({ name: 'twitter:title', content: 'AI Shorts Maker: Long Video to Ranked Clips | Indifferent' });
    this.meta.updateTag({ name: 'twitter:description', content: description });
    this.addJsonLd({
      '@context': 'https://schema.org',
      '@type': 'SoftwareApplication',
      name: 'Indifferent AI Shorts Maker',
      url: pageUrl,
      applicationCategory: 'MultimediaApplication',
      operatingSystem: 'Web',
      description,
      offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD', description: 'Free to try' },
    });
    this.addJsonLd({
      '@context': 'https://schema.org',
      '@type': 'VideoObject',
      name: 'AI Shorts Maker: Long Video to Ranked Clips',
      description,
      url: pageUrl,
      embedUrl: pageUrl,
      thumbnailUrl: 'https://indifferent.fun/logo.svg',
      uploadDate: '2026-10-03',
      duration: 'PT1M',
      contentRating: 'General',
    });
    this.addJsonLd({
      '@context': 'https://schema.org',
      '@type': 'FAQPage',
      mainEntity: this.faqs.map((f) => ({ '@type': 'Question', name: f.q, acceptedAnswer: { '@type': 'Answer', text: f.a } })),
    });
  }

  private addJsonLd(schema: Record<string, unknown>): void {
    const script = this.document.createElement('script');
    script.type = 'application/ld+json';
    script.text = JSON.stringify(schema);
    this.document.head.appendChild(script);
  }

  ngAfterViewInit(): void {
    // DOM-only scroll reveals: skip on the server (Domino nodelists lack
    // forEach and there is no IntersectionObserver). Content prerenders
    // un-revealed; the client animates after hydration.
    if (!this.isBrowser) {
      return;
    }
    const els: Element[] = Array.from(this.host.nativeElement.querySelectorAll('.reveal'));
    if (typeof IntersectionObserver === 'undefined') {
      els.forEach((el) => el.classList.add('is-visible'));
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => entries.forEach((e) => {
        if (e.isIntersecting) {
          e.target.classList.add('is-visible');
          observer.unobserve(e.target);
        }
      }),
      { threshold: 0.12 },
    );
    els.forEach((el) => observer.observe(el));
  }
}
