import { Component, AfterViewInit, ElementRef, OnInit, PLATFORM_ID, ViewChild, inject } from '@angular/core';
import { DOCUMENT, isPlatformBrowser } from '@angular/common';
import { RouterLink } from '@angular/router';
import { NavMenuComponent } from '../../shared/components/nav-menu/nav-menu.component';
import { NavMobileMenuComponent } from '../../shared/components/nav-mobile-menu/nav-mobile-menu.component';
import { setPageSeo } from '../../shared/seo';
import { FlipbookComponent } from './flipbook.component';
import { HeroSceneComponent } from './hero-scene.component';

@Component({
  selector: 'app-landing',
  standalone: true,
  imports: [RouterLink, NavMenuComponent, NavMobileMenuComponent, FlipbookComponent, HeroSceneComponent],
  styles: [`
    /* Shape rule: pills for buttons, 20px cards, 12px inputs. Serif display only here. */
    .serif { font-family: 'Cormorant Garamond', 'Playfair Display', Georgia, serif; }
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
    .reveal { opacity: 0; transform: translateY(28px); transition: opacity .8s cubic-bezier(0.23,1,0.32,1), transform .8s cubic-bezier(0.23,1,0.32,1); }
    .reveal.is-visible { opacity: 1; transform: none; }
    .card-lift { transition: transform .35s cubic-bezier(0.23,1,0.32,1), box-shadow .35s; }
    .card-lift:hover { transform: translateY(-6px); box-shadow: 0 4px 6px -1px rgba(0,0,0,0.1); }
    .card-lift:active { transform: translateY(-1px) scale(.99); }
    .btn-primary { transition: transform .2s cubic-bezier(0.23,1,0.32,1), background-color .2s; }
    .btn-primary:hover { transform: translateY(-1px); }
    .btn-primary:active { transform: translateY(1px) scale(.98); }
    details.faq summary::-webkit-details-marker { display: none; }
    details.faq[open] .faq-chevron { transform: rotate(180deg); }
    .faq-chevron { transition: transform .25s; }
    .grain::before {
      content: ''; position: fixed; inset: 0; z-index: 60; pointer-events: none; opacity: .05;
      background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='2'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='0.5'/%3E%3C/svg%3E");
    }
    @media (prefers-reduced-motion: reduce) {
      .reveal { opacity: 1; transform: none; transition: none; }
      .card-lift, .btn-primary { transition: none; }
      .faq-chevron { transition: none; }
    }
  `],
  template: `
    <div class="grain sans bg-[#FAF7F2] text-[#1A1714] antialiased">

      <!-- Floating island nav -->
      <header class="fixed top-4 left-1/2 -translate-x-1/2 z-50 w-[min(1120px,calc(100%-2rem))]">
        <nav class="flex items-center justify-between h-14 pl-5 pr-2 rounded-full bg-[#0F0E0B]/90 backdrop-blur-xl border border-white/10 shadow-[0_8px_30px_rgba(20,15,10,0.25)]">
          <a routerLink="/" class="flex items-center gap-2 min-h-[44px] text-[#FAF7F2]">
            <span class="font-semibold tracking-tight text-[17px]">Indifferent<span class="text-[#D96C3D]">.</span></span>
          </a>
          <div class="hidden md:flex items-center gap-6 text-[13.5px] text-white/70">
            <app-nav-menu label="Features" [width]="340">
              <a routerLink="/features/txt-to-video-quiz" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                <span class="block text-white text-[13.5px] font-medium">TXT to narrated video</span>
                <span class="block text-white/60 text-[12px] mt-0.5">Text-based quiz files (.txt) into narrated video content with answer reveals, ready for YouTube or any platform.</span>
              </a>
              <a routerLink="/features/watermark" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                <span class="block text-white text-[13.5px] font-medium">Watermark tool</span>
                <span class="block text-white/60 text-[12px] mt-0.5">Draggable text and logo watermarks on images and video. Free, private, 100% in-browser.</span>
              </a>
              <a routerLink="/features/ai-shorts" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                <span class="block text-white text-[13.5px] font-medium">AI Shorts generator</span>
                <span class="block text-white/60 text-[12px] mt-0.5">Turn a long video or audio file into ranked 9:16 vertical clips with burned-in captions. AI finds the most engaging moments automatically.</span>
              </a>
            </app-nav-menu>
            <app-nav-menu label="Use cases" [width]="270">
              <a routerLink="/blog/quiz-videos-youtube" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                <span class="block text-white text-[13.5px] font-medium">YouTube creators</span>
                <span class="block text-white/60 text-[12px] mt-0.5">Faceless quiz channels</span>
              </a>
              <a routerLink="/blog/educational-video-best-practices" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                <span class="block text-white text-[13.5px] font-medium">Teachers &amp; educators</span>
                <span class="block text-white/60 text-[12px] mt-0.5">Lesson recaps &amp; revision</span>
              </a>
              <a routerLink="/blog/ai-narration-guide" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                <span class="block text-white text-[13.5px] font-medium">AI narration</span>
                <span class="block text-white/60 text-[12px] mt-0.5">5 voices to choose from</span>
              </a>
            </app-nav-menu>
            <app-nav-menu label="Resources" [width]="270">
              <a routerLink="/blog" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                <span class="block text-white text-[13.5px] font-medium">Journal</span>
                <span class="block text-white/60 text-[12px] mt-0.5">Essays &amp; notes</span>
              </a>
              <a routerLink="/blog/quiz-file-format-guide" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                <span class="block text-white text-[13.5px] font-medium">Format guide</span>
                <span class="block text-white/60 text-[12px] mt-0.5">Write parseable TXT</span>
              </a>
              <a routerLink="/about" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                <span class="block text-white text-[13.5px] font-medium">About the studio</span>
                <span class="block text-white/60 text-[12px] mt-0.5">What we build</span>
              </a>
            </app-nav-menu>
          </div>
          <div class="flex items-center gap-1">
            <app-nav-mobile-menu>
              <p class="px-4 pt-3 pb-1 text-[12.5px] font-medium text-white/60">Features</p>
              <a routerLink="/features/txt-to-video-quiz" class="block rounded-xl px-4 py-3 text-[14px] text-white hover:bg-white/[0.06] transition-colors">TXT to narrated video</a>
              <a routerLink="/features/watermark" class="block rounded-xl px-4 py-3 text-[14px] text-white hover:bg-white/[0.06] transition-colors">Watermark tool</a>
              <a routerLink="/features/ai-shorts" class="block rounded-xl px-4 py-3 text-[14px] text-white hover:bg-white/[0.06] transition-colors">AI Shorts generator</a>
              <p class="px-4 pt-3 pb-1 text-[12.5px] font-medium text-white/60">Use cases</p>
              <a routerLink="/blog/quiz-videos-youtube" class="block rounded-xl px-4 py-3 text-[14px] text-white hover:bg-white/[0.06] transition-colors">YouTube creators</a>
              <a routerLink="/blog/educational-video-best-practices" class="block rounded-xl px-4 py-3 text-[14px] text-white hover:bg-white/[0.06] transition-colors">Teachers &amp; educators</a>
              <a routerLink="/blog/ai-narration-guide" class="block rounded-xl px-4 py-3 text-[14px] text-white hover:bg-white/[0.06] transition-colors">AI narration</a>
              <p class="px-4 pt-3 pb-1 text-[12.5px] font-medium text-white/60">Resources</p>
              <a routerLink="/blog" class="block rounded-xl px-4 py-3 text-[14px] text-white hover:bg-white/[0.06] transition-colors">Journal</a>
              <a routerLink="/blog/quiz-file-format-guide" class="block rounded-xl px-4 py-3 text-[14px] text-white hover:bg-white/[0.06] transition-colors">Format guide</a>
              <a routerLink="/about" class="block rounded-xl px-4 py-3 text-[14px] text-white hover:bg-white/[0.06] transition-colors">About the studio</a>
            </app-nav-mobile-menu>
            <a routerLink="/login" class="btn-primary inline-flex items-center gap-2 h-11 pl-5 pr-5 sm:pr-1.5 rounded-full bg-[#FAF7F2] text-[#141310] text-[13.5px] font-semibold">
              Get started
              <span class="hidden sm:flex w-7 h-7 rounded-full bg-[#1A1714] text-white items-center justify-center text-sm leading-none" aria-hidden="true">↗</span>
            </a>
          </div>
        </nav>
      </header>

      <!-- HERO · dark opening block -->
      <section #heroSection class="relative overflow-hidden bg-[#0F0E0B] text-[#F4EFE6] pt-24 pb-14 md:pb-20 min-h-[100dvh] flex items-center">
        <div class="absolute inset-0" style="background: radial-gradient(ellipse at 75% 20%, rgba(217,108,61,0.22), transparent 55%), linear-gradient(to bottom, rgba(15,14,11,0.55), rgba(15,14,11,0.92));"></div>

        <div class="relative w-[min(1240px,100%)] mx-auto px-5 md:px-10 grid grid-cols-1 lg:grid-cols-[1.05fr_0.95fr] gap-12 items-center">
          <div>
            <h1 #heroTitle class="serif font-medium leading-[1.02] tracking-[-0.02em] text-[clamp(2.6rem,6vw,4.6rem)] text-balance">
              Turn a quiz text file into a <em class="font-light italic">narrated video.</em>
            </h1>
            <p #heroSubtitle class="mt-6 text-[17px] leading-relaxed text-white/70 max-w-[46ch]">
              Write your questions in a plain text file. Get back a 1080p MP4 where a voice reads each question aloud and then reveals the answer.
            </p>
            <div #heroCta class="mt-8 flex flex-wrap items-center gap-4">
              <a routerLink="/login" class="btn-primary inline-flex items-center gap-3 rounded-full bg-[#E8E0D2] text-[#141310] pl-7 pr-2 py-2 font-semibold">
                Turn my TXT into a video
                <span class="w-9 h-9 rounded-full bg-[#141310] text-white flex items-center justify-center">→</span>
              </a>
              <a href="#format" class="inline-flex items-center min-h-[44px] text-[15px] text-white/80 underline underline-offset-8 decoration-white/30 hover:decoration-white transition">See the file format</a>
            </div>
            <dl class="mt-8 flex flex-wrap gap-x-8 gap-y-4 text-[13px] text-white/60">
              <div><dt class="text-[13px] text-white/60">Input</dt><dd class="serif text-2xl text-white">TXT</dd></div>
              <div class="border-l border-white/10 pl-8"><dt class="text-[13px] text-white/60">Output</dt><dd class="serif text-2xl text-white">MP4</dd></div>
              <div class="border-l border-white/10 pl-8"><dt class="text-[13px] text-white/60">Narration voices</dt><dd class="serif text-2xl text-white">5</dd></div>
              <div class="border-l border-white/10 pl-8"><dt class="text-[13px] text-white/60">Max file size</dt><dd class="serif text-2xl text-white">5 MB</dd></div>
            </dl>
            <p class="mt-6 text-[13.5px] text-white/60">
              Free during beta, no credit card.
              <a routerLink="/terms" class="underline underline-offset-4 hover:text-white">Beta terms</a>
            </p>
            <p class="mt-3 text-[13px] text-white/50">
              Also in the studio:
              <a routerLink="/shorts-maker" class="underline underline-offset-4 hover:text-white">AI Shorts generator</a>
              and
              <a routerLink="/tools/add-watermark" class="underline underline-offset-4 hover:text-white">watermark tool</a>
            </p>
          </div>

          <div class="relative lg:justify-self-end w-full max-w-[540px]">
            <app-hero-scene />
          </div>
        </div>
      </section>

      <main class="bg-[#FAF7F2]">
        <!-- FORMAT · what the input actually looks like, above the fold -->
        <section id="format" class="scroll-mt-24 w-[min(1240px,100%)] mx-auto px-5 md:px-10 pt-14 md:pt-20">
          <div class="grid grid-cols-1 lg:grid-cols-[0.85fr_1.15fr] gap-10 items-center">
            <div>
              <h2 class="reveal serif font-medium tracking-[-0.02em] text-[clamp(1.6rem,3vw,2.4rem)] leading-tight">Your questions, in a plain text file.</h2>
              <p class="reveal mt-4 text-[16px] leading-relaxed text-[#6B6560] max-w-[46ch]">
                No special editor, no spreadsheet, no formatting to clean up. Number the questions, list the options, and mark the answer.
                Numbering, bulleted options and tab-indented answers are all detected.
              </p>
              <p class="reveal mt-2 text-[14px] leading-relaxed text-[#6B6560] max-w-[46ch]">
                The answer can be marked on its own line, or on the option itself with a trailing <span class="font-mono">*</span>.
              </p>
              <ul class="reveal mt-6 space-y-2 text-[14.5px] text-[#6B6560]">
                <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#BC5227] shrink-0"></span>Up to 5 MB, plain text only</li>
                <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#BC5227] shrink-0"></span>Multiple correct answers supported</li>
                <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#BC5227] shrink-0"></span>One question per slide, with an answer reveal</li>
              </ul>
              <a routerLink="/blog/quiz-file-format-guide" class="reveal mt-6 inline-flex items-center min-h-[44px] text-[14px] font-semibold text-[#A8481F] underline underline-offset-4">Read the format guide</a>
            </div>
            <div class="reveal rounded-[20px] border border-black/10 bg-[#1A1714] p-6 md:p-8">
              <pre class="overflow-x-auto text-[13px] leading-[1.7] text-[#E8E0D2] font-mono"><code>1. What is the capital of France?
A. London
B. Paris
C. Berlin
D. Madrid
Answer: B

2. Which planet is known as the Red Planet?
A. Venus
B. Jupiter
C. Mars
D. Saturn
Answer: C</code></pre>
            </div>
          </div>
        </section>

        <!-- TOOLS · the rest of the studio -->
        <section aria-labelledby="tools-heading" class="w-[min(1240px,100%)] mx-auto px-5 md:px-10 pt-16 md:pt-20">
          <h2 id="tools-heading" class="reveal serif font-medium tracking-[-0.02em] text-[clamp(1.6rem,3vw,2.4rem)]">Two more tools in the studio.</h2>
          <p class="reveal mt-4 max-w-[62ch] text-[16px] leading-relaxed text-[#6B6560]">
            The quiz renderer is one of three pipelines here. The other two work on footage you already have, and the second one never touches a server.
          </p>
          <div class="mt-8 grid grid-cols-1 lg:grid-cols-2 gap-4">
            <!-- AI Shorts -->
            <article class="reveal rounded-[20px] bg-[#1A1714] text-[#F4EFE6] p-6 md:p-8 flex flex-col">
              <p class="text-[11px] uppercase tracking-[0.06em] text-white/50">AI Shorts generator</p>
              <h3 class="serif mt-2 text-[26px] leading-tight">A long video, cut into ranked vertical clips.</h3>
              <p class="mt-3 text-[14.5px] leading-relaxed text-white/70">
                Drop in a podcast, an interview or a lecture. It is transcribed word by word, every segment is
                scored, and the best ones are rendered as 9:16 clips with captions burned in.
              </p>
              <ul class="mt-5 space-y-2.5 text-[14px] leading-relaxed text-white/75">
                <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#D96C3D] shrink-0"></span>Video or audio in: MP4, MOV, MP3, WAV, up to 1 hour</li>
                <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#D96C3D] shrink-0"></span>Up to 3 clips per upload, ranked by score</li>
                <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#D96C3D] shrink-0"></span>Face tracking pans the frame onto the speaker</li>
                <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#D96C3D] shrink-0"></span>Blur-fill keeps text cards and screen recordings whole</li>
                <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#D96C3D] shrink-0"></span>Four caption presets, restyled after the fact</li>
              </ul>
              <dl class="mt-6 pt-5 border-t border-white/10 grid grid-cols-2 gap-4 text-[13px]">
                <div><dt class="text-white/50">Output</dt><dd class="mt-0.5 font-semibold">1080x1920 MP4</dd></div>
                <div><dt class="text-white/50">Codec</dt><dd class="mt-0.5 font-semibold">H.264 + AAC, 30fps</dd></div>
              </dl>
              <a routerLink="/shorts-maker" class="btn-interactive mt-6 inline-flex items-center gap-2 self-start rounded-full bg-[#F4EFE6] text-[#141310] px-6 py-3 font-semibold text-[14px]">Open the Shorts generator</a>
            </article>

            <!-- Watermark -->
            <article class="reveal rounded-[20px] bg-[#1E3A2A] text-[#EDE8DB] p-6 md:p-8 flex flex-col">
              <p class="text-[11px] uppercase tracking-[0.06em] text-white/50">Watermark tool</p>
              <h3 class="serif mt-2 text-[26px] leading-tight">Your mark on any image or video, in your browser.</h3>
              <p class="mt-3 text-[14.5px] leading-relaxed text-[#EDE8DB]/80">
                Stamp a text or logo watermark, drag it where you want it, and export. The file never
                leaves your device, so there is no upload and no account.
              </p>
              <ul class="mt-5 space-y-2.5 text-[14px] leading-relaxed text-[#EDE8DB]/85">
                <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#D96C3D] shrink-0"></span>Text in 10 typefaces, or your own logo image</li>
                <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#D96C3D] shrink-0"></span>Size, opacity, colour and rotation, all adjustable</li>
                <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#D96C3D] shrink-0"></span>Original, 16:9, 9:16 or 1:1 framing</li>
                <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#D96C3D] shrink-0"></span>Images out at full resolution as PNG or JPEG</li>
                <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#D96C3D] shrink-0"></span>Video out as MP4, rendered on your own machine</li>
              </ul>
              <dl class="mt-6 pt-5 border-t border-white/10 grid grid-cols-2 gap-4 text-[13px]">
                <div><dt class="text-white/50">Accepts</dt><dd class="mt-0.5 font-semibold">JPG, PNG, WebP, MP4</dd></div>
                <div><dt class="text-white/50">Uploads</dt><dd class="mt-0.5 font-semibold">Nothing, ever</dd></div>
              </dl>
              <a routerLink="/tools/add-watermark" class="btn-interactive mt-6 inline-flex items-center gap-2 self-start rounded-full bg-[#EDE8DB] text-[#1E3A2A] px-6 py-3 font-semibold text-[14px]">Open the watermark tool</a>
            </article>
          </div>
        </section>
        <!-- PROCESS · how it works -->
        <section id="process" class="scroll-mt-24 mt-16 md:mt-24 border-y border-black/[0.07] bg-[#F3EEE3]">
          <div class="w-[min(1240px,100%)] mx-auto px-5 md:px-10 py-20 md:py-28 grid grid-cols-1 lg:grid-cols-[0.9fr_1.1fr] gap-12">
            <div class="reveal order-2 lg:order-1">
              <app-flipbook />
              <a routerLink="/blog/quiz-file-format-guide" class="mt-2 inline-flex items-center min-h-[44px] text-[14px] font-semibold text-[#A8481F] underline underline-offset-4">Read the format guide <span class="ml-1" aria-hidden="true">→</span></a>
            </div>
            <div class="order-1 lg:order-2">
              <h2 class="reveal serif font-medium tracking-[-0.02em] leading-[1.02] text-[clamp(2rem,4.5vw,3.4rem)]">How it works.</h2>
              <p class="reveal mt-4 max-w-[52ch] leading-relaxed text-[#6B6560]">Three steps from a text file to a finished video.</p>
              <ol class="mt-10">
                @for (s of steps; track s.n) {
                  <li class="reveal grid grid-cols-[56px_1fr] gap-5 py-7 border-t border-black/10 last:border-b">
                    <span class="serif italic text-[30px] text-[#A8481F] leading-none">{{ s.n }}</span>
                    <div>
                      <h3 class="text-[19px] font-semibold">{{ s.title }}</h3>
                      <p class="mt-1.5 text-[15px] text-[#6B6560] leading-relaxed max-w-[52ch]">{{ s.body }}</p>
                    </div>
                  </li>
                }
              </ol>
            </div>
          </div>
        </section>
        <!-- WHO · where a quiz video fits -->
        <section id="who" class="scroll-mt-24 w-[min(1240px,100%)] mx-auto px-5 md:px-10 py-20 md:py-28">
          <h2 class="reveal serif font-medium tracking-[-0.02em] leading-[1.02] text-[clamp(2rem,4.5vw,3.4rem)]">Where a quiz video fits.</h2>
          <ul class="mt-10 border-t border-[#1A1714]/15">
            @for (u of useCases; track u.title) {
              <li class="reveal border-b border-[#1A1714]/15">
                <a [routerLink]="u.href" class="flex items-center justify-between gap-6 py-6 transition-colors hover:bg-black/[0.02]">
                  <span>
                    <span class="serif block text-[26px] md:text-[30px] leading-tight">{{ u.title }}</span>
                    <span class="mt-1 block max-w-[56ch] text-[15px] text-[#6B6560]">{{ u.text }}</span>
                  </span>
                  <span class="shrink-0 text-[14px] font-semibold text-[#A8481F]"><span class="hidden sm:inline">{{ u.cta }}</span></span>
                </a>
              </li>
            }
          </ul>
        </section>
      </main>

      <!-- FAQ -->
      <section id="faq" class="border-t border-black/[0.07] bg-white">
        <div class="w-[min(900px,100%)] mx-auto px-5 md:px-10 py-16 md:py-24">
          <p class="reveal text-center text-[11.5px] uppercase tracking-[0.1em] text-[#6B6560]">FAQ</p>
          <h2 class="reveal mt-3 text-center serif font-medium tracking-[-0.02em] text-[clamp(1.7rem,3.2vw,2.6rem)]">Before you <em class="font-light">upload.</em></h2>
          <div class="mt-8 space-y-3">
            @for (f of faqs; track f.q) {
              <details class="faq reveal rounded-[16px] border border-black/10 bg-[#FAF7F2] px-5 py-4">
                <summary class="flex items-center justify-between gap-4 cursor-pointer list-none font-semibold text-[15px]">
                  {{ f.q }}
                  <svg class="shrink-0 text-[#6B6560] faq-chevron" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m6 9 6 6 6-6"/></svg>
                </summary>
                <p class="mt-3 text-[14px] leading-relaxed text-[#6B6560]">{{ f.a }}</p>
              </details>
            }
          </div>
        </div>
      </section>

      <!-- CLOSING dark block: trust + newsletter + footer -->
      <section class="relative overflow-hidden bg-[#0F0E0B] text-[#F4EFE6]">
        <div class="absolute inset-0" style="background: linear-gradient(to bottom, #0F0E0B, rgba(15,14,11,0.75) 45%, #0F0E0B);"></div>
        <div class="relative w-[min(1240px,100%)] mx-auto px-5 md:px-10 pt-20 md:pt-28 pb-10">
          <!-- Trust + newsletter -->
          <div class="grid grid-cols-1 lg:grid-cols-2 gap-10 items-start">
            <div>
              <h2 class="reveal serif font-medium tracking-[-0.02em] text-[clamp(1.6rem,3vw,2.4rem)]">Get the next essay first.</h2>
              <p class="reveal mt-3 text-[15px] leading-relaxed text-white/70 max-w-[46ch]">
                Notes on pacing, narration and publishing quiz videos. No newsletter exists yet, so this only confirms your address was read.
              </p>
              @if (newsletterSubscribed) {
                <p class="reveal is-visible mt-5 text-[14px] text-[#EDE8DB]">Thanks. Newsletter signup is coming soon, and we will not share your address.</p>
              } @else {
                <form (submit)="subscribeNewsletter($event)" class="reveal mt-5 max-w-[420px]">
                  <label for="landing-newsletter-email" class="sr-only">Email address</label>
                  <div class="rounded-[20px] p-2 bg-white/[0.06] border border-white/10 backdrop-blur flex flex-col sm:flex-row gap-2">
                    <input id="landing-newsletter-email" type="email" required placeholder="you@studio.com"
                      class="flex-1 rounded-[12px] bg-transparent px-5 h-12 text-[15px] placeholder:text-white/60 outline-none border border-transparent focus:border-white/40">
                    <button type="submit" class="btn-primary rounded-full bg-[#E8E0D2] text-[#141310] px-7 h-12 font-semibold text-[14.5px]">Subscribe</button>
                  </div>
                </form>
              }
            </div>
            <div>
              <h2 class="reveal serif font-medium tracking-[-0.02em] text-[clamp(1.6rem,3vw,2.4rem)]">Who runs this.</h2>
              <p class="reveal mt-3 text-[15px] leading-relaxed text-white/70 max-w-[46ch]">
                Indifferent is built and maintained by one developer. It is in beta, free to use, and there is no sales team behind it.
                If something breaks, you can reach the person who wrote it.
              </p>
              <p class="reveal mt-4 text-[15px]">
                <a href="mailto:support@indifferent.fun" class="underline underline-offset-4 decoration-white/30 hover:decoration-white">support&#64;indifferent.fun</a>
              </p>
            </div>
          </div>

          <footer class="mt-16 border-t border-white/10 pt-10">
            <p class="serif text-center leading-none tracking-[-0.03em] text-[clamp(3.5rem,14vw,11rem)] text-[#F4EFE6]/95 select-none">INDIFFERENT</p>
            <div class="mt-10 grid grid-cols-2 md:grid-cols-4 gap-8 text-[14px]">
              <div>
                <p class="text-white/60 text-[12px] uppercase tracking-[0.06em] mb-3">Product</p>
                <ul class="text-white/75">
                  <li><a href="#process" class="inline-flex items-center min-h-[44px] min-w-[44px] hover:text-white">How it works</a></li>
                  <li><a routerLink="/blog" class="inline-flex items-center min-h-[44px] min-w-[44px] hover:text-white">Journal</a></li>
                  <li><a routerLink="/features/watermark" class="inline-flex items-center min-h-[44px] min-w-[44px] hover:text-white">Watermark tool</a></li>
                  <li><a routerLink="/features/ai-shorts" class="inline-flex items-center min-h-[44px] min-w-[44px] hover:text-white">AI Shorts generator</a></li>
                  <li><a routerLink="/features/txt-to-video-quiz" class="inline-flex items-center min-h-[44px] min-w-[44px] hover:text-white">TXT to quiz video</a></li>
                </ul>
              </div>
              <div>
                <p class="text-white/60 text-[12px] uppercase tracking-[0.06em] mb-3">Studio</p>
                <ul class="text-white/75">
                  <li><a routerLink="/about" class="inline-flex items-center min-h-[44px] min-w-[44px] hover:text-white">About</a></li>
                  <li><a routerLink="/contact" class="inline-flex items-center min-h-[44px] min-w-[44px] hover:text-white">Contact</a></li>
                  <li><a routerLink="/blog/quiz-file-format-guide" class="inline-flex items-center min-h-[44px] min-w-[44px] hover:text-white">Format guide</a></li>
                </ul>
              </div>
              <div>
                <p class="text-white/60 text-[12px] uppercase tracking-[0.06em] mb-3">Legal</p>
                <ul class="text-white/75">
                  <li><a routerLink="/privacy" class="inline-flex items-center min-h-[44px] min-w-[44px] hover:text-white">Privacy</a></li>
                  <li><a routerLink="/terms" class="inline-flex items-center min-h-[44px] min-w-[44px] hover:text-white">Terms</a></li>
                </ul>
              </div>
              <div>
                <p class="text-white/60 text-[12px] uppercase tracking-[0.06em] mb-3">Start</p>
                <a routerLink="/login" class="btn-primary inline-flex items-center gap-2 rounded-full bg-[#BC5227] text-white px-6 py-3 font-semibold text-[14px]">Start creating</a>
                <p class="mt-4 text-white/60 text-[13px]">TXT in. MP4 out.</p>
              </div>
            </div>
            <div class="mt-10 pt-6 border-t border-white/10 flex flex-col sm:flex-row justify-between items-center gap-3 text-[12.5px] text-white/60">
              <span>© {{ currentYear }} Indifferent. All rights reserved.</span>
              <a href="mailto:support@indifferent.fun" class="underline underline-offset-4 decoration-white/25 hover:decoration-white">support&#64;indifferent.fun</a>
            </div>
          </footer>
        </div>
      </section>
    </div>
  `,
})
export class LandingComponent implements OnInit, AfterViewInit {
  private readonly isBrowser = isPlatformBrowser(inject(PLATFORM_ID));
  private readonly document = inject(DOCUMENT);

  @ViewChild('heroTitle') heroTitle!: ElementRef;
  @ViewChild('heroSubtitle') heroSubtitle!: ElementRef;
  @ViewChild('heroCta') heroCta!: ElementRef;
  @ViewChild('heroSection') heroSection!: ElementRef;

  currentYear = new Date().getFullYear();
  newsletterSubscribed = false;

  faqs: { q: string; a: string }[] = [
    { q: 'What file do I need?', a: 'A plain text (.txt) file containing your multiple-choice questions. Number each question, list the options, and mark the answer. Numbered, bulleted and tab-indented formats are all detected, so you do not have to clean up your formatting first.' },
    { q: 'How large can the file be?', a: 'Up to 5 MB, which is enough for several hundred questions. The video length follows from how many questions you write.' },
    { q: 'Which voices can I use?', a: 'Five, all real Amazon Polly voices: Joanna (US English, clear and professional), Matthew (US English, warm and authoritative), Amy (British English, polished), Brian (British English, natural) and Aditi (Indian English, approachable).' },
    { q: 'How long does rendering take?', a: 'A short quiz renders in a couple of minutes. The project page shows the live stage, so you can leave it open and come back.' },
    { q: 'Is it free?', a: 'Yes, during the beta. No credit card, no quota to track, no paid tier to upgrade to yet. The beta terms say what that means.' },
    { q: 'Is my question file uploaded?', a: 'Yes, the text file has to reach the server to be parsed and narrated, and it is stored with your project so you can re-render it. The privacy page explains exactly what is kept and for how long. The watermark tool is the opposite: it never uploads anything.' },
    { q: 'Can I use the video commercially?', a: 'The narration and slides are generated for you, so what you can do with the result depends on the questions you wrote and on your account terms. The terms page covers this in full.' },
    { q: 'What happens to my file afterwards?', a: 'Your project and its text file stay in your account so you can re-render with a different voice or template, and you can delete them at any time from the project page.' },
  ];

  ngOnInit(): void {
    setPageSeo(this.document, {
      title: 'Indifferent: Turn a TXT Quiz File into a Narrated Video',
      description:
        'Write your quiz questions in a plain text file and get back a 1080p MP4 where a voice reads each question aloud and reveals the answer. Free during beta, no credit card.',
      canonical: 'https://indifferent.fun/',
    });
    this.addJsonLd({
      '@context': 'https://schema.org',
      '@type': 'WebPage',
      name: 'Indifferent: Turn a TXT Quiz File into a Narrated Video',
      url: 'https://indifferent.fun/',
      description:
        'Write your quiz questions in a plain text file and get back a 1080p MP4 where a voice reads each question aloud and reveals the answer.',
    });
    this.addJsonLd({
      '@context': 'https://schema.org',
      '@type': 'SoftwareApplication',
      name: 'Indifferent Quiz Video Generator',
      url: 'https://indifferent.fun/',
      applicationCategory: 'MultimediaApplication',
      operatingSystem: 'Web',
      description:
        'Turns a plain text file of multiple-choice questions into a narrated 1080p MP4 quiz video.',
      offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD', description: 'Free during beta' },
      featureList: [
        'TXT quiz file input',
        '5 narration voices',
        '1080p MP4 output',
        'Question and answer slides',
      ],
    });
    this.addJsonLd({
      '@context': 'https://schema.org',
      '@type': 'FAQPage',
      mainEntity: this.faqs.map((f) => ({
        '@type': 'Question',
        name: f.q,
        acceptedAnswer: { '@type': 'Answer', text: f.a },
      })),
    });
  }

  private addJsonLd(schema: Record<string, unknown>): void {
    const script = this.document.createElement('script');
    script.type = 'application/ld+json';
    script.text = JSON.stringify(schema);
    this.document.head.appendChild(script);
  }

  subscribeNewsletter(event: Event): void {
    event.preventDefault();
    this.newsletterSubscribed = true;
  }

  steps = [
    {
      n: '01',
      title: 'Upload',
      body: 'Drop a TXT file. We detect numbering, options, and answer keys automatically. No formatting cleanup.',
    },
    {
      n: '02',
      title: 'Choose a voice',
      body: 'Pick one of five AI narrators. Each question and each answer is read aloud in that voice.',
    },
    {
      n: '03',
      title: 'Render and download',
      body: 'We render the MP4 for you. Download it, watermark it if you like, and post it to YouTube.',
    },
  ];
  useCases = [
    { title: 'YouTube quiz shows', text: 'Faceless quiz channels: upload a file, publish the video.', href: '/blog/quiz-videos-youtube', cta: 'Read the guide' },
    { title: 'Lesson recaps', text: 'Turn revision questions into a video students can replay.', href: '/blog/educational-video-best-practices', cta: 'Read the guide' },
    { title: 'Trivia nights', text: 'Run a round straight from a text file of questions.', href: '/blog/quiz-file-format-guide', cta: 'See the format' },
    { title: 'Staff training', text: 'Check understanding with multiple-choice questions.', href: '/features/txt-to-video-quiz', cta: 'See the feature' },
  ];

  ngAfterViewInit(): void {
    // Scroll reveals need window/document: browser only. Server prerender
    // ships the content un-revealed; the client animates after hydration.
    if (!this.isBrowser) {
      return;
    }
    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    const els = Array.from(document.querySelectorAll('.reveal'));
    if (!('IntersectionObserver' in window) || reduce) {
      els.forEach((e) => e.classList.add('is-visible'));
    } else {
      const io = new IntersectionObserver(
        (entries) => entries.forEach((en) => {
          if (en.isIntersecting) { en.target.classList.add('is-visible'); io.unobserve(en.target); }
        }),
        { threshold: 0.15, rootMargin: '0px 0px -8% 0px' },
      );
      els.forEach((e) => io.observe(e));
    }
    if (reduce) return;
    // Gentle hero entrance: transform + opacity only (GPU-safe)
    const items = [this.heroTitle?.nativeElement, this.heroSubtitle?.nativeElement, this.heroCta?.nativeElement].filter(Boolean);
    items.forEach((el: HTMLElement, i: number) => {
      el.style.opacity = '0';
      el.style.transform = 'translateY(26px)';
      el.style.transition = `opacity .9s cubic-bezier(0.23,1,0.32,1) ${i * 0.12}s, transform .9s cubic-bezier(0.23,1,0.32,1) ${i * 0.12}s`;
      requestAnimationFrame(() => requestAnimationFrame(() => {
        el.style.opacity = '1';
        el.style.transform = 'none';
      }));
    });
  }
}
