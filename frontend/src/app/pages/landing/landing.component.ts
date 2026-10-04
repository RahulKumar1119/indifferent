import { Component, AfterViewInit, ElementRef, PLATFORM_ID, ViewChild, inject } from '@angular/core';
import { isPlatformBrowser } from '@angular/common';
import { RouterLink } from '@angular/router';

@Component({
  selector: 'app-landing',
  standalone: true,
  imports: [RouterLink],
  styles: [`
    /* Shape rule: pills for buttons, 20px cards, 12px inputs. Serif display only here. */
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

      <!-- Floating island nav -->
      <header class="fixed top-4 left-1/2 -translate-x-1/2 z-50 w-[min(1120px,calc(100%-2rem))]">
        <nav class="flex items-center justify-between h-14 pl-5 pr-2 rounded-full bg-[#0F0E0B]/90 backdrop-blur-xl border border-white/10 shadow-[0_8px_30px_rgba(20,15,10,0.25)]">
          <a routerLink="/" class="flex items-center gap-2 text-[#FAF7F2]">
            <span class="font-semibold tracking-tight text-[17px]">Indifferent<span class="text-[#D96C3D]">.</span></span>
          </a>
          <div class="hidden md:flex items-center gap-6 text-[13.5px] text-white/70">
            <!-- Features -->
            <div class="relative group">
              <button type="button" class="flex items-center gap-1.5 bg-transparent border-none text-[13.5px] text-white/70 group-hover:text-white transition-colors cursor-pointer p-0">
                Features
                <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="transition-transform duration-200 group-hover:rotate-180"><path d="m6 9 6 6 6-6"/></svg>
              </button>
              <div class="absolute top-full left-1/2 -translate-x-1/2 pt-3 opacity-0 invisible translate-y-1 group-hover:opacity-100 group-hover:visible group-hover:translate-y-0 transition-all duration-200">
                <div class="w-[340px] rounded-2xl border border-white/10 bg-[#17150F] shadow-[0_20px_60px_rgba(0,0,0,0.5)] p-2">
                  <a routerLink="/features/txt-to-video-quiz" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                    <span class="block text-white text-[13.5px] font-medium">TXT to narrated video</span>
                    <span class="block text-white/50 text-[12px] mt-0.5">Text-based quiz files (.txt) into narrated video content with answer reveals, ready for YouTube or any platform.</span>
                  </a>
                  <a routerLink="/features/watermark" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                    <span class="block text-white text-[13.5px] font-medium">Watermark tool</span>
                    <span class="block text-white/50 text-[12px] mt-0.5">Draggable text watermarks on JPG, PNG, WebP — size, opacity, color, rotation. Free, private, 100% in-browser with full-resolution PNG export.</span>
                  </a>
                  <a routerLink="/features/ai-shorts" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                    <span class="block text-white text-[13.5px] font-medium">AI Shorts generator</span>
                    <span class="block text-white/50 text-[12px] mt-0.5">Turn a long video or audio file into ranked 9:16 vertical clips with burned-in captions — AI finds the most engaging moments automatically.</span>
                  </a>
                </div>
              </div>
            </div>
            <!-- Use cases -->
            <div class="relative group">
              <button type="button" class="flex items-center gap-1.5 bg-transparent border-none text-[13.5px] text-white/70 group-hover:text-white transition-colors cursor-pointer p-0">
                Use cases
                <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="transition-transform duration-200 group-hover:rotate-180"><path d="m6 9 6 6 6-6"/></svg>
              </button>
              <div class="absolute top-full left-1/2 -translate-x-1/2 pt-3 opacity-0 invisible translate-y-1 group-hover:opacity-100 group-hover:visible group-hover:translate-y-0 transition-all duration-200">
                <div class="w-[270px] rounded-2xl border border-white/10 bg-[#17150F] shadow-[0_20px_60px_rgba(0,0,0,0.5)] p-2">
                  <a routerLink="/blog/quiz-videos-youtube" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                    <span class="block text-white text-[13.5px] font-medium">YouTube creators</span>
                    <span class="block text-white/50 text-[12px] mt-0.5">Faceless quiz channels</span>
                  </a>
                  <a routerLink="/blog/educational-video-best-practices" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                    <span class="block text-white text-[13.5px] font-medium">Teachers &amp; educators</span>
                    <span class="block text-white/50 text-[12px] mt-0.5">Lesson recaps &amp; revision</span>
                  </a>
                  <a routerLink="/" fragment="use-events" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                    <span class="block text-white text-[13.5px] font-medium">Trivia hosts</span>
                    <span class="block text-white/50 text-[12px] mt-0.5">Nights &amp; live events</span>
                  </a>
                  <a routerLink="/" fragment="use-teams" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                    <span class="block text-white text-[13.5px] font-medium">Corporate trainers</span>
                    <span class="block text-white/50 text-[12px] mt-0.5">Training &amp; onboarding</span>
                  </a>
                </div>
              </div>
            </div>
            <!-- Resources -->
            <div class="relative group">
              <button type="button" class="flex items-center gap-1.5 bg-transparent border-none text-[13.5px] text-white/70 group-hover:text-white transition-colors cursor-pointer p-0">
                Resources
                <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="transition-transform duration-200 group-hover:rotate-180"><path d="m6 9 6 6 6-6"/></svg>
              </button>
              <div class="absolute top-full left-1/2 -translate-x-1/2 pt-3 opacity-0 invisible translate-y-1 group-hover:opacity-100 group-hover:visible group-hover:translate-y-0 transition-all duration-200">
                <div class="w-[270px] rounded-2xl border border-white/10 bg-[#17150F] shadow-[0_20px_60px_rgba(0,0,0,0.5)] p-2">
                  <a routerLink="/blog" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                    <span class="block text-white text-[13.5px] font-medium">Journal</span>
                    <span class="block text-white/50 text-[12px] mt-0.5">Essays &amp; notes</span>
                  </a>
                  <a routerLink="/blog/quiz-file-format-guide" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                    <span class="block text-white text-[13.5px] font-medium">Format guide</span>
                    <span class="block text-white/50 text-[12px] mt-0.5">Write parseable TXT</span>
                  </a>
                  <a routerLink="/about" class="block rounded-xl px-4 py-3 hover:bg-white/[0.06] transition-colors">
                    <span class="block text-white text-[13.5px] font-medium">About the studio</span>
                    <span class="block text-white/50 text-[12px] mt-0.5">What we build</span>
                  </a>
                </div>
              </div>
            </div>
          </div>
          <a routerLink="/login" class="btn-primary inline-flex items-center gap-2 h-10 pl-5 pr-1.5 rounded-full bg-[#FAF7F2] text-[#141310] text-[13.5px] font-semibold">
            Start creating
            <span class="w-7 h-7 rounded-full bg-[#1A1714] text-white flex items-center justify-center text-sm leading-none">↗</span>
          </a>
        </nav>
      </header>

      <!-- HERO · dark opening block -->
      <section #heroSection class="relative overflow-hidden bg-[#0F0E0B] text-[#F4EFE6] pt-24 pb-14 md:pb-20 min-h-[100dvh] flex items-center">
        <img src="https://picsum.photos/seed/indifferent-studio-dark/1800/1200" alt="" aria-hidden="true"
          class="absolute inset-0 w-full h-full object-cover opacity-35">
        <div class="absolute inset-0" style="background: radial-gradient(ellipse at 75% 20%, rgba(217,108,61,0.22), transparent 55%), linear-gradient(to bottom, rgba(15,14,11,0.55), rgba(15,14,11,0.92));"></div>

        <div class="relative w-[min(1240px,100%)] mx-auto px-5 md:px-10 grid grid-cols-1 lg:grid-cols-[1.05fr_0.95fr] gap-12 items-center">
          <div>
            <p class="inline-flex items-center gap-2 rounded-full px-3 py-1 text-[10.5px] uppercase tracking-[0.22em] border border-white/15 text-white/70">
              <span class="w-1.5 h-1.5 rounded-full bg-[#D96C3D]"></span> Studio &amp; Engine · TXT to Video
            </p>
            <h1 #heroTitle class="serif mt-5 font-medium leading-[0.95] tracking-[-0.02em] text-[clamp(3rem,8vw,6.2rem)] text-balance">
              Video<br><em class="font-light italic">Architecture.</em>
            </h1>
            <p #heroSubtitle class="mt-6 text-[17px] leading-relaxed text-white/70 max-w-[46ch]">
              Upload a quiz file. Receive a narrated, timed, YouTube-ready film.
            </p>
            <div #heroCta class="mt-8 flex flex-wrap items-center gap-4">
              <a routerLink="/login" class="btn-primary inline-flex items-center gap-3 rounded-full bg-[#E8E0D2] text-[#141310] pl-7 pr-2 py-2 font-semibold">
                Start creating
                <span class="w-9 h-9 rounded-full bg-[#141310] text-white flex items-center justify-center">→</span>
              </a>
              <a href="#templates" class="text-[15px] text-white/80 underline underline-offset-8 decoration-white/30 hover:decoration-white transition">View templates →</a>
            </div>
            <dl class="mt-10 flex gap-8 text-[13px] text-white/55">
              <div><dt class="sr-only">Render time</dt><dd class="serif text-2xl text-white">~4 min</dd><dd>file to MP4</dd></div>
              <div class="border-l border-white/10 pl-8"><dd class="serif text-2xl text-white">6</dd><dd>art-directed themes</dd></div>
              <div class="border-l border-white/10 pl-8"><dd class="serif text-2xl text-white">5</dd><dd>AI narrator voices</dd></div>
            </dl>
          </div>

          <!-- Right: layered preview stack -->
          <div class="relative lg:justify-self-end w-full max-w-[520px]">
            <div class="rounded-[2rem] p-2 bg-white/[0.06] border border-white/10 backdrop-blur-sm">
              <div class="rounded-[calc(2rem-0.5rem)] overflow-hidden bg-[#17150F] border border-white/10">
                <div class="flex items-center gap-1.5 px-4 h-10 border-b border-white/10">
                  <span class="w-2.5 h-2.5 rounded-full bg-white/15"></span>
                  <span class="w-2.5 h-2.5 rounded-full bg-white/15"></span>
                  <span class="w-2.5 h-2.5 rounded-full bg-[#D96C3D]/70"></span>
                  <span class="ml-3 text-[11px] tracking-widest uppercase text-white/40">world-history-quiz.mp4 · 1080p</span>
                </div>
                <div class="relative aspect-video">
                  <img src="https://picsum.photos/seed/indifferent-quiz-frame/960/540" alt="Generated quiz video frame showing a timed question card" class="absolute inset-0 w-full h-full object-cover">
                  <div class="absolute inset-0 flex flex-col items-center justify-center p-6 text-center" style="background: linear-gradient(to bottom, rgba(15,14,11,0.25), rgba(15,14,11,0.72));">
                    <p class="text-[11px] uppercase tracking-[0.25em] text-white/60">Question 07 / 20</p>
                    <p class="serif text-2xl md:text-3xl mt-2">Which empire built the Colosseum?</p>
                    <div class="grid grid-cols-2 gap-2 mt-5 w-full max-w-[340px] text-[13px]">
                      <span class="rounded-lg bg-white/10 border border-white/20 px-3 py-2">A · Greece</span>
                      <span class="rounded-lg bg-[#D96C3D] px-3 py-2 font-semibold">B · Rome</span>
                      <span class="rounded-lg bg-white/10 border border-white/20 px-3 py-2">C · Persia</span>
                      <span class="rounded-lg bg-white/10 border border-white/20 px-3 py-2">D · Egypt</span>
                    </div>
                  </div>
                  <span class="absolute bottom-3 right-3 rounded-md bg-black/70 text-white text-[11px] px-2 py-0.5">03:24</span>
                </div>
                <div class="flex items-center gap-3 px-4 h-12 border-t border-white/10 text-white/60 text-[12px]">
                  <span class="w-6 h-6 rounded-full bg-white text-black flex items-center justify-center text-[10px]">▶</span>
                  <div class="flex-1 h-1 rounded bg-white/15 overflow-hidden"><div class="h-full w-2/5 bg-[#D96C3D]"></div></div>
                  <span>AI · Aditi</span>
                </div>
              </div>
            </div>
            <div class="absolute -bottom-5 -left-4 md:-left-8 rounded-2xl bg-[#F4EFE6] text-[#1A1714] px-4 py-3 shadow-[0_20px_50px_rgba(0,0,0,0.4)] border border-black/10 rotate-[-2deg]">
              <p class="text-[11px] uppercase tracking-[0.18em] text-[#6B6560]">Narration</p>
              <p class="text-[14px] font-medium">“Answer: B — Rome…”</p>
            </div>
            <div class="absolute -top-4 -right-2 md:-right-4 rounded-full bg-[#1E3A2A] text-[#EDE8DB] px-4 py-2 text-[12.5px] border border-white/15 rotate-[3deg]">Auto timers · Reveals · Captions</div>
          </div>
        </div>
      </section>

      <main class="bg-[#FAF7F2]">
        <!-- TEMPLATES · curated assemblages -->
        <section id="templates" class="w-[min(1240px,100%)] mx-auto px-5 md:px-10 py-24 md:py-32">
          <div class="reveal flex flex-wrap items-end justify-between gap-6">
            <h2 class="serif font-medium tracking-[-0.02em] leading-[1.02] text-[clamp(2rem,4.5vw,3.4rem)] max-w-[14ch]">Curated templates, ready to render.</h2>
            <a routerLink="/login" class="rounded-full border border-[#1A1714]/20 px-6 py-3 text-[14px] font-semibold hover:bg-[#1A1714] hover:text-white transition-colors">View complete archive</a>
          </div>
          <p class="reveal mt-4 text-[#6B6560] max-w-[62ch] leading-relaxed">Living themes designed for watch time. Not slideshows — timed, narrated films with countdowns and reveals.</p>

          <div class="mt-12 grid grid-cols-1 md:grid-cols-3 gap-6">
            @for (t of templates; track t.name) {
              <article class="reveal card-lift rounded-[20px] overflow-hidden bg-white border border-black/[0.07]">
                <div class="relative aspect-[4/3] overflow-hidden">
                  <img [src]="t.image" [alt]="t.name + ' template preview'" loading="lazy" class="w-full h-full object-cover">
                  <span class="absolute top-3 left-3 rounded-full bg-black/55 backdrop-blur text-white text-[11.5px] px-3 py-1">{{ t.tag }}</span>
                </div>
                <div class="p-6">
                  <div class="flex items-baseline justify-between">
                    <h3 class="serif text-[26px] leading-none">{{ t.name }}</h3>
                    <span class="text-[13px] text-[#6B6560]">{{ t.meta }}</span>
                  </div>
                  <p class="mt-2 text-[14.5px] text-[#6B6560] leading-relaxed">{{ t.desc }}</p>
                  <a routerLink="/login" class="btn-primary mt-5 inline-flex w-full items-center justify-center gap-2 rounded-full bg-[#1A1714] text-white py-3 text-[14.5px] font-semibold hover:bg-[#2A2620]">
                    Use {{ t.name }} <span aria-hidden="true">→</span>
                  </a>
                </div>
              </article>
            }
          </div>
        </section>

        <!-- PROCESS · architecture of video -->
        <section id="process" class="border-y border-black/[0.07] bg-[#F3EEE3]">
          <div class="w-[min(1240px,100%)] mx-auto px-5 md:px-10 py-24 md:py-32 grid grid-cols-1 lg:grid-cols-[0.9fr_1.1fr] gap-12">
            <div class="reveal relative order-2 lg:order-1">
              <div class="rounded-[20px] overflow-hidden border border-black/10">
                <img src="https://picsum.photos/seed/indifferent-process-studio/900/1100" alt="Editorial view of the rendering studio workflow" loading="lazy" class="w-full h-[420px] lg:h-[560px] object-cover">
              </div>
              <div class="absolute bottom-5 left-5 right-5 rounded-2xl bg-[#0F0E0B]/85 backdrop-blur text-white p-5 border border-white/10">
                <p class="font-mono text-[11px] uppercase tracking-[0.2em] text-white/50">$ cat capitals-quiz.txt</p>
                <p class="mt-2 font-mono text-[13px] leading-relaxed text-white/85">1. Capital of Japan?<br>a) Kyoto&nbsp;&nbsp;b) Tokyo *&nbsp;&nbsp;c) Osaka<br>→ detected · 20 questions · 03:41 runtime</p>
              </div>
            </div>
            <div class="order-1 lg:order-2">
              <h2 class="reveal serif font-medium tracking-[-0.02em] leading-[1.02] text-[clamp(2rem,4.5vw,3.4rem)]">The architecture<br>of video.</h2>
              <p class="reveal mt-4 text-[#6B6560] max-w-[52ch] leading-relaxed">A deliberate rejection of screen-recording. Every upload is treated as a timed broadcast.</p>
              <ol class="mt-10">
                @for (s of steps; track s.n) {
                  <li class="reveal grid grid-cols-[56px_1fr] gap-5 py-7 border-t border-black/10 last:border-b">
                    <span class="serif italic text-[30px] text-[#BC5227] leading-none">{{ s.n }}</span>
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

        <!-- ARCHIVE · use-case bento -->
        <section id="archive" class="w-[min(1240px,100%)] mx-auto px-5 md:px-10 py-24 md:py-32">
          <h2 class="reveal serif font-medium tracking-[-0.02em] text-[clamp(2rem,4.5vw,3.4rem)]">The archive.</h2>
          <p class="reveal mt-3 text-[#6B6560] max-w-[58ch]">Five lanes our creators publish in weekly. Pick a lane — the timing, voices, and reveals adapt.</p>
          <div class="mt-10 grid grid-cols-1 md:grid-cols-6 gap-5">
            <a routerLink="/login" id="use-youtube" class="reveal card-lift group relative overflow-hidden rounded-[20px] md:col-span-4 min-h-[300px] flex items-end scroll-mt-24">
              <img src="https://picsum.photos/seed/indifferent-youtube/1200/700" alt="YouTube quiz channel setup" loading="lazy" class="absolute inset-0 w-full h-full object-cover group-hover:scale-[1.03] transition-transform duration-700">
              <span class="absolute inset-0" style="background: linear-gradient(to top, rgba(15,14,11,0.78), transparent 60%);"></span>
              <span class="relative p-7 text-white"><span class="block text-[12px] uppercase tracking-[0.2em] text-white/65">01 · Faceless channels</span><span class="serif block text-3xl mt-1">YouTube quiz shows</span></span>
            </a>
            <a routerLink="/login" id="use-lessons" class="reveal card-lift group relative overflow-hidden rounded-[20px] md:col-span-2 min-h-[300px] flex items-end scroll-mt-24">
              <img src="https://picsum.photos/seed/indifferent-classroom/800/700" alt="Classroom revision video" loading="lazy" class="absolute inset-0 w-full h-full object-cover group-hover:scale-[1.03] transition-transform duration-700">
              <span class="absolute inset-0" style="background: linear-gradient(to top, rgba(15,14,11,0.78), transparent 60%);"></span>
              <span class="relative p-7 text-white"><span class="block text-[12px] uppercase tracking-[0.2em] text-white/65">02 · Teaching</span><span class="serif block text-3xl mt-1">Lesson recaps</span></span>
            </a>
            <a routerLink="/login" id="use-events" class="reveal card-lift group relative overflow-hidden rounded-[20px] md:col-span-2 min-h-[260px] flex items-end scroll-mt-24">
              <img src="https://picsum.photos/seed/indifferent-trivia/800/600" alt="Pub trivia night visuals" loading="lazy" class="absolute inset-0 w-full h-full object-cover group-hover:scale-[1.03] transition-transform duration-700">
              <span class="absolute inset-0" style="background: linear-gradient(to top, rgba(15,14,11,0.78), transparent 60%);"></span>
              <span class="relative p-6 text-white"><span class="block text-[12px] uppercase tracking-[0.2em] text-white/65">03 · Events</span><span class="serif block text-[26px] mt-1">Trivia nights</span></span>
            </a>
            <a routerLink="/login" id="use-teams" class="reveal card-lift group relative overflow-hidden rounded-[20px] md:col-span-2 min-h-[260px] flex items-end scroll-mt-24">
              <img src="https://picsum.photos/seed/indifferent-training/800/600" alt="Corporate training still" loading="lazy" class="absolute inset-0 w-full h-full object-cover group-hover:scale-[1.03] transition-transform duration-700">
              <span class="absolute inset-0" style="background: linear-gradient(to top, rgba(15,14,11,0.78), transparent 60%);"></span>
              <span class="relative p-6 text-white"><span class="block text-[12px] uppercase tracking-[0.2em] text-white/65">04 · Teams</span><span class="serif block text-[26px] mt-1">Staff training</span></span>
            </a>
            <a routerLink="/tools/add-watermark" class="reveal card-lift relative overflow-hidden rounded-[20px] md:col-span-2 min-h-[260px] flex flex-col justify-between p-6 bg-[#1E3A2A] text-[#EDE8DB]">
              <span class="text-[12px] uppercase tracking-[0.2em] text-white/60">05 · Finishing</span>
              <span class="serif text-[26px] leading-tight">Brand it.<br>Watermark &amp; ship.</span>
              <span class="inline-flex w-max items-center gap-2 rounded-full bg-white/10 border border-white/15 px-4 py-2 text-[13.5px]">Open tool →</span>
            </a>
          </div>
        </section>

        <!-- PROOF · clarity wall -->
        <section class="border-t border-black/[0.07] bg-white">
          <div class="w-[min(1240px,100%)] mx-auto px-5 md:px-10 py-24 md:py-28 grid grid-cols-1 lg:grid-cols-[0.85fr_1.15fr] gap-12">
            <div class="reveal">
              <h2 class="serif font-medium text-[clamp(2rem,4vw,3rem)] leading-[1.02]">Clarity.</h2>
              <p class="mt-4 text-[#6B6560] leading-relaxed max-w-[40ch]">Creators stopped editing. They upload TXT files on Monday and publish all week.</p>
              <div class="mt-8 flex items-center gap-3">
                <div class="flex -space-x-2">
                  <span class="w-9 h-9 rounded-full border-2 border-white bg-[#BC5227] text-white flex items-center justify-center text-[12px] font-bold">PR</span>
                  <span class="w-9 h-9 rounded-full border-2 border-white bg-[#1E3A2A] text-white flex items-center justify-center text-[12px] font-bold">SK</span>
                  <span class="w-9 h-9 rounded-full border-2 border-white bg-[#8A7B5C] text-white flex items-center justify-center text-[12px] font-bold">DM</span>
                </div>
                <p class="text-[13.5px] text-[#6B6560]">2,400+ videos rendered<br>this quarter</p>
              </div>
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-5">
              @for (q of quotes; track q.name) {
                <figure class="reveal rounded-[20px] border border-black/[0.07] bg-[#FAF7F2] p-6 flex flex-col">
                  <blockquote class="serif text-[19px] leading-snug">“{{ q.text }}”</blockquote>
                  <figcaption class="mt-5 pt-4 border-t border-black/10 flex items-center gap-3">
                    <span class="w-9 h-9 rounded-full flex items-center justify-center text-[13px] font-bold text-white" [style.background]="q.color">{{ q.initials }}</span>
                    <span><span class="block text-[14px] font-semibold">{{ q.name }}</span><span class="block text-[12.5px] text-[#6B6560]">{{ q.role }}</span></span>
                  </figcaption>
                </figure>
              }
            </div>
          </div>
        </section>
      </main>

      <!-- CLOSING dark block: manifesto + newsletter + footer -->
      <section class="relative overflow-hidden bg-[#0F0E0B] text-[#F4EFE6]">
        <img src="https://picsum.photos/seed/indifferent-manifesto/1800/900" alt="" aria-hidden="true" class="absolute inset-0 w-full h-full object-cover opacity-25">
        <div class="absolute inset-0" style="background: linear-gradient(to bottom, #0F0E0B, rgba(15,14,11,0.75) 45%, #0F0E0B);"></div>
        <div class="relative w-[min(1240px,100%)] mx-auto px-5 md:px-10 pt-24 md:pt-32 pb-10">
          <h2 class="reveal serif font-medium text-center leading-[1.0] tracking-[-0.02em] text-[clamp(2.2rem,5.5vw,4.2rem)] max-w-[20ch] mx-auto text-balance">Never a generic slideshow again.</h2>
          <p class="reveal mt-5 text-center text-white/65 max-w-[58ch] mx-auto leading-relaxed">Timed questions, spoken narration, answer reveals scored to the frame. Indifferent directs each file like a small broadcast.</p>

          <form (submit)="$event.preventDefault()" class="reveal mt-10 mx-auto max-w-[520px] rounded-[20px] p-2 bg-white/[0.06] border border-white/12 backdrop-blur flex flex-col sm:flex-row gap-2">
            <label for="newsletter-email" class="sr-only">Email address</label>
            <input id="newsletter-email" type="email" required placeholder="you@studio.com"
              class="flex-1 rounded-[12px] bg-transparent px-5 h-12 text-[15px] placeholder:text-white/35 outline-none border border-transparent focus:border-white/30">
            <button type="submit" class="btn-primary rounded-full bg-[#E8E0D2] text-[#141310] px-7 h-12 font-semibold text-[14.5px]">Subscribe</button>
          </form>
          <p class="reveal mt-3 text-center text-[12.5px] text-white/45">Bi-weekly notes on pacing, voices, and retention. Nothing else.</p>

          <footer class="mt-20 border-t border-white/10 pt-10">
            <p class="serif text-center leading-none tracking-[-0.03em] text-[clamp(3.5rem,14vw,11rem)] text-[#F4EFE6]/95 select-none">INDIFFERENT</p>
            <div class="mt-10 grid grid-cols-2 md:grid-cols-4 gap-8 text-[14px]">
              <div>
                <p class="text-white/40 text-[12px] uppercase tracking-[0.18em] mb-3">Archive</p>
                <ul class="space-y-2 text-white/75">
                  <li><a routerLink="/login" class="hover:text-white">Templates</a></li>
                  <li><a routerLink="/blog" class="hover:text-white">Journal</a></li>
                  <li><a routerLink="/features/watermark" class="hover:text-white">Watermark tool</a></li>
                  <li><a routerLink="/features/ai-shorts" class="hover:text-white">AI Shorts generator</a></li>
                  <li><a routerLink="/features/txt-to-video-quiz" class="hover:text-white">TXT to quiz video</a></li>
                </ul>
              </div>
              <div>
                <p class="text-white/40 text-[12px] uppercase tracking-[0.18em] mb-3">Studio</p>
                <ul class="space-y-2 text-white/75">
                  <li><a routerLink="/about" class="hover:text-white">About</a></li>
                  <li><a routerLink="/contact" class="hover:text-white">Contact</a></li>
                  <li><a routerLink="/blog" class="hover:text-white">Format guide</a></li>
                </ul>
              </div>
              <div>
                <p class="text-white/40 text-[12px] uppercase tracking-[0.18em] mb-3">Legal</p>
                <ul class="space-y-2 text-white/75">
                  <li><a routerLink="/privacy" class="hover:text-white">Privacy</a></li>
                  <li><a routerLink="/terms" class="hover:text-white">Terms</a></li>
                </ul>
              </div>
              <div>
                <p class="text-white/40 text-[12px] uppercase tracking-[0.18em] mb-3">Start</p>
                <a routerLink="/login" class="btn-primary inline-flex items-center gap-2 rounded-full bg-[#D96C3D] text-white px-6 py-3 font-semibold text-[14px]">Start creating →</a>
                <p class="mt-4 text-white/50 text-[13px]">TXT in. MP4 out.<br>Built with Angular, Go &amp; AWS.</p>
              </div>
            </div>
            <div class="mt-10 pt-6 border-t border-white/10 flex flex-col sm:flex-row justify-between gap-2 text-[12.5px] text-white/40">
              <span>© {{ currentYear }} Indifferent. All rights reserved.</span>
              <span>Architecture of video</span>
            </div>
          </footer>
        </div>
      </section>
    </div>
  `,
})
export class LandingComponent implements AfterViewInit {
  private readonly isBrowser = isPlatformBrowser(inject(PLATFORM_ID));

  @ViewChild('heroTitle') heroTitle!: ElementRef;
  @ViewChild('heroSubtitle') heroSubtitle!: ElementRef;
  @ViewChild('heroCta') heroCta!: ElementRef;
  @ViewChild('heroSection') heroSection!: ElementRef;

  currentYear = new Date().getFullYear();

  templates = [
    {
      name: 'Midnight Lecture',
      tag: 'Most used',
      meta: '16:9 · 1080p',
      desc: 'High-contrast cards with calm narration bed. Built for retention.',
      image: 'https://picsum.photos/seed/indifferent-t-midnight/800/600',
    },
    {
      name: 'Blush Seminar',
      tag: 'Warm grade',
      meta: '16:9 · 1080p',
      desc: 'Paper-warm backgrounds with ink type. Feels like a printed handout, filmed.',
      image: 'https://picsum.photos/seed/indifferent-t-blush/800/600',
    },
    {
      name: 'Neon Recall',
      tag: 'High energy',
      meta: '9:16 ready',
      desc: 'Countdown timers and answer stings. Made for Shorts and Reels cuts.',
      image: 'https://picsum.photos/seed/indifferent-t-neon/800/600',
    },
  ];

  steps = [
    {
      n: '01',
      title: 'Upload & vision',
      body: 'Drop a TXT file. We detect numbering, options, and answer keys automatically — no formatting cleanup.',
    },
    {
      n: '02',
      title: 'Voice & timing',
      body: 'Pick from 5 Polly narrators. Questions, countdowns, and reveals are scored to the voice track.',
    },
    {
      n: '03',
      title: 'Render & publish',
      body: 'Serverless pipeline cuts the MP4 in minutes. Download, watermark, and post to YouTube.',
    },
  ];

  quotes = [
    { text: 'It stopped looking like slides. It plays like a real quiz show.', name: 'Priya Rao', role: 'Education creator', initials: 'PR', color: '#BC5227' },
    { text: 'I upload on Monday, publish all week. No timeline scrubbing.', name: 'Sam Keller', role: 'Trivia host', initials: 'SK', color: '#1E3A2A' },
    { text: 'The countdown pacing alone doubled our average view time.', name: 'Dana Mensah', role: 'Course producer', initials: 'DM', color: '#8A7B5C' },
    { text: 'Students ask for the videos by name now. That never happened.', name: 'Leo Fontaine', role: 'Language coach', initials: 'LF', color: '#3E4A3D' },
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
