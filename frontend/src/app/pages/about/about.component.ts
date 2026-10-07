import { Component, OnInit, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { RouterLink } from '@angular/router';
import { setPageSeo } from '../../shared/seo';

@Component({
  selector: 'app-about',
  standalone: true,
  imports: [RouterLink],
  styles: [`
    .serif { font-family: 'Cormorant Garamond', 'Playfair Display', Georgia, serif; }
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
    .btn-primary { transition: transform .2s cubic-bezier(0.23,1,0.32,1), background-color .2s; }
    .btn-primary:hover { transform: translateY(-1px); }
    .btn-primary:active { transform: translateY(1px) scale(.98); }
    .grain::before {
      content: ''; position: fixed; inset: 0; z-index: 60; pointer-events: none; opacity: .05;
      background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='2'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='0.5'/%3E%3C/svg%3E");
    }
  `],
  template: `
    <div class="grain sans bg-[#FAF7F2] text-[#1A1714] antialiased">
      <header class="fixed top-4 left-1/2 -translate-x-1/2 z-50 w-[min(1120px,calc(100%-2rem))]">
        <nav class="flex items-center justify-between h-14 pl-5 pr-2 rounded-full bg-[#0F0E0B]/90 backdrop-blur-xl border border-white/10 shadow-[0_8px_30px_rgba(20,15,10,0.25)]">
          <a routerLink="/" class="flex items-center gap-2 text-[#FAF7F2]">
            <span class="font-semibold tracking-tight text-[17px]">Indifferent<span class="text-[#D96C3D]">.</span></span>
          </a>
          <div class="hidden md:flex items-center gap-7 text-[13.5px] text-white/70">
            <a routerLink="/login" class="hover:text-white transition-colors">Templates</a>
            <a routerLink="/blog" class="hover:text-white transition-colors">Journal</a>
            <a routerLink="/about" class="text-white font-medium">Studio</a>
            <a routerLink="/contact" class="hover:text-white transition-colors">Contact</a>
          </div>
          <a routerLink="/login" class="btn-primary inline-flex items-center gap-2 h-10 pl-5 pr-1.5 rounded-full bg-[#FAF7F2] text-[#141310] text-[13.5px] font-semibold">
            Start creating
            <span class="w-7 h-7 rounded-full bg-[#1A1714] text-white flex items-center justify-center text-sm leading-none">↗</span>
          </a>
        </nav>
      </header>

      <section class="relative overflow-hidden bg-[#0F0E0B] text-[#F4EFE6] pt-32 pb-16 md:pb-20">
        <img src="https://picsum.photos/seed/indifferent-about-hero/1800/900" alt="" aria-hidden="true" class="absolute inset-0 w-full h-full object-cover opacity-30">
        <div class="absolute inset-0" style="background: radial-gradient(ellipse at 75% 15%, rgba(217,108,61,0.20), transparent 55%), linear-gradient(to bottom, rgba(15,14,11,0.55), rgba(15,14,11,0.93));"></div>
        <div class="relative w-[min(1240px,100%)] mx-auto px-5 md:px-10">
          <p class="inline-flex items-center gap-2 rounded-full px-3 py-1 text-[10.5px] uppercase tracking-[0.22em] border border-white/15 text-white/70">
            <span class="w-1.5 h-1.5 rounded-full bg-[#D96C3D]"></span> The Studio
          </p>
          <h1 class="serif mt-5 font-medium leading-[0.95] tracking-[-0.02em] text-[clamp(2.8rem,7vw,5.2rem)] max-w-[16ch] text-balance">The studio behind the video.</h1>
          <p class="mt-6 text-[17px] leading-relaxed text-white/70 max-w-[52ch]">Indifferent turns multiple-choice quiz files into engaging, YouTube-ready films — no editing skills required.</p>
        </div>
      </section>

      <main class="w-[min(1240px,100%)] mx-auto px-5 md:px-10 py-16 md:py-24">
        <!-- Mission split -->
        <section class="grid grid-cols-1 lg:grid-cols-2 gap-10 items-center">
          <div>
            <h2 class="serif font-medium tracking-[-0.02em] text-[clamp(2rem,4vw,3rem)] leading-[1.02]">Our mission</h2>
            <p class="mt-4 text-[16px] text-[#3E3A33] leading-relaxed">Indifferent makes it effortless to convert your multiple-choice quiz files into engaging, YouTube-ready video content. Whether you're an educator, content creator, or training professional, our platform automates the entire production — from parsing your questions to generating narrated, animated videos with countdown timers and answer reveals.</p>
            <a routerLink="/login" class="btn-primary mt-6 inline-flex items-center gap-3 rounded-full bg-[#1A1714] text-white pl-6 pr-2 py-2 font-semibold text-[14.5px]">Start creating <span class="w-8 h-8 rounded-full bg-white/15 flex items-center justify-center">→</span></a>
          </div>
          <div class="rounded-[20px] overflow-hidden border border-black/10">
            <img src="https://picsum.photos/seed/indifferent-about-mission/900/700" alt="Studio desk with quiz production notes" loading="lazy" class="w-full h-[320px] lg:h-[400px] object-cover">
          </div>
        </section>

        <!-- How it works -->
        <section class="mt-20 rounded-[20px] bg-[#F3EEE3] border border-black/[0.07] p-8 md:p-12">
          <h2 class="serif font-medium text-[clamp(1.8rem,3.5vw,2.6rem)]">How it works</h2>
          <ol class="mt-6">
            <li class="grid grid-cols-[48px_1fr] gap-4 py-5 border-t border-black/10">
              <span class="serif italic text-[26px] text-[#BC5227] leading-none">01</span>
              <div><h3 class="font-semibold">Upload your quiz file</h3><p class="mt-1 text-[14.5px] text-[#6B6560]">Plain text (.txt) with multiple-choice questions. Numbered, bulleted, and tab-indented formats supported.</p></div>
            </li>
            <li class="grid grid-cols-[48px_1fr] gap-4 py-5 border-t border-black/10">
              <span class="serif italic text-[26px] text-[#BC5227] leading-none">02</span>
              <div><h3 class="font-semibold">Choose template &amp; voice</h3><p class="mt-1 text-[14.5px] text-[#6B6560]">A visual theme for your slides and one of 5 professional AI voices powered by Amazon Polly.</p></div>
            </li>
            <li class="grid grid-cols-[48px_1fr] gap-4 py-5 border-t border-black/10">
              <span class="serif italic text-[26px] text-[#BC5227] leading-none">03</span>
              <div><h3 class="font-semibold">Automatic processing</h3><p class="mt-1 text-[14.5px] text-[#6B6560]">Our serverless pipeline parses questions, builds animated slides, creates narration, and renders the video.</p></div>
            </li>
            <li class="grid grid-cols-[48px_1fr] gap-4 py-5 border-y border-black/10">
              <span class="serif italic text-[26px] text-[#BC5227] leading-none">04</span>
              <div><h3 class="font-semibold">Download &amp; share</h3><p class="mt-1 text-[14.5px] text-[#6B6560]">Preview in-browser and download the MP4 — ready for YouTube, social, or any platform.</p></div>
            </li>
          </ol>
        </section>

        <!-- Formats + templates -->
        <section class="mt-20 grid grid-cols-1 lg:grid-cols-2 gap-6">
          <div class="rounded-[20px] bg-white border border-black/[0.07] p-8">
            <h2 class="serif text-[28px]">Supported formats</h2>
            <ul class="mt-4 space-y-2.5 text-[14.5px] text-[#3E3A33]">
              <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#BC5227] shrink-0"></span>Numbered questions (1. Question text)</li>
              <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#BC5227] shrink-0"></span>Bulleted questions (• or - Question text)</li>
              <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#BC5227] shrink-0"></span>Tab-indented answers</li>
              <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#BC5227] shrink-0"></span>Multiple correct answers supported</li>
              <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#BC5227] shrink-0"></span>Maximum file size: 5MB</li>
            </ul>
          </div>
          <div class="rounded-[20px] bg-white border border-black/[0.07] p-8">
            <h2 class="serif text-[28px]">Video specifications</h2>
            <ul class="mt-4 space-y-2.5 text-[14.5px] text-[#3E3A33]">
              <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#1E3A2A] shrink-0"></span>Resolution: 1920×1080 (Full HD)</li>
              <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#1E3A2A] shrink-0"></span>Format: MP4 (H.264), Audio: AAC</li>
              <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#1E3A2A] shrink-0"></span>Question slides, answer reveals, AI narration</li>
              <li class="flex gap-2.5"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#1E3A2A] shrink-0"></span>Automatic thumbnail generation</li>
            </ul>
          </div>
        </section>

        <!-- Templates -->
        <section class="mt-16">
          <h2 class="serif font-medium text-[clamp(1.8rem,3.5vw,2.6rem)]">Six themes, one engine</h2>
          <div class="mt-6 grid grid-cols-2 md:grid-cols-3 gap-4">
            @for (t of templates; track t.name) {
              <div class="rounded-[20px] bg-white border border-black/[0.07] p-5">
                <p class="font-semibold text-[15px]">{{ t.name }}</p>
                <p class="text-[13px] text-[#6B6560] mt-0.5">{{ t.desc }}</p>
              </div>
            }
          </div>
        </section>

        <!-- Voices -->
        <section class="mt-16 grid grid-cols-1 lg:grid-cols-[0.9fr_1.1fr] gap-10 items-start">
          <div>
            <h2 class="serif font-medium text-[clamp(1.8rem,3.5vw,2.6rem)]">Five narrator voices</h2>
            <p class="mt-3 text-[#6B6560] leading-relaxed">Professional text-to-speech via Amazon Polly. Pick the register that fits your audience.</p>
          </div>
          <ul class="rounded-[20px] bg-white border border-black/[0.07] divide-y divide-black/[0.07]">
            @for (v of voices; track v.name) {
              <li class="flex items-center gap-4 p-4">
                <span class="w-10 h-10 rounded-full bg-[#1E3A2A] text-white flex items-center justify-center text-[13px] font-bold">{{ v.initials }}</span>
                <span><span class="block font-semibold text-[14.5px]">{{ v.name }}</span><span class="block text-[12.5px] text-[#6B6560]">{{ v.desc }}</span></span>
              </li>
            }
          </ul>
        </section>

        <!-- Use cases -->
        <section class="mt-16">
          <h2 class="serif font-medium text-[clamp(1.8rem,3.5vw,2.6rem)]">Who uses it</h2>
          <div class="mt-6 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4">
            @for (u of uses; track u.title) {
              <div class="rounded-[20px] bg-white border border-black/[0.07] p-5">
                <p class="font-semibold text-[14.5px]">{{ u.title }}</p>
                <p class="text-[12.5px] text-[#6B6560] mt-1">{{ u.desc }}</p>
              </div>
            }
          </div>
        </section>

        <!-- Stack -->
        <section class="mt-16 rounded-[20px] bg-[#1E3A2A] text-[#EDE8DB] p-8 md:p-12">
          <h2 class="serif text-[28px]">Built with</h2>
          <div class="mt-5 flex flex-wrap gap-2">
            @for (s of stack; track s) {
              <span class="rounded-full border border-white/20 bg-white/[0.07] px-4 py-1.5 text-[13px]">{{ s }}</span>
            }
          </div>
          <p class="mt-6 text-white/65 text-[14px] max-w-[60ch]">Indifferent is built and maintained by a developer focused on making content creation accessible — no expensive software, no editing skills.</p>
          <div class="mt-6 flex flex-wrap gap-3">
            <a href="https://github.com/RahulKumar1119/indifferent/issues" target="_blank" rel="noopener" class="btn-primary inline-flex items-center rounded-full bg-[#E8E0D2] text-[#141310] px-6 py-3 font-semibold text-[14px]">Open an issue →</a>
            <a routerLink="/contact" class="inline-flex items-center rounded-full border border-white/25 px-6 py-3 text-[14px] hover:border-white/60 transition-colors">Get in touch</a>
          </div>
        </section>
      </main>

      <section class="bg-[#0F0E0B] text-[#F4EFE6]">
        <div class="w-[min(1240px,100%)] mx-auto px-5 md:px-10 py-12 flex flex-col sm:flex-row justify-between gap-3 text-[12.5px] text-white/40">
          <span>© {{ currentYear }} Indifferent. All rights reserved.</span>
          <span class="flex gap-5">
            <a routerLink="/" class="hover:text-white">Home</a>
            <a routerLink="/blog" class="hover:text-white">Journal</a>
            <a routerLink="/privacy" class="hover:text-white">Privacy</a>
            <a routerLink="/terms" class="hover:text-white">Terms</a>
          </span>
        </div>
      </section>
    </div>
  `,
})
export class AboutComponent implements OnInit {
  private readonly document = inject(DOCUMENT);

  ngOnInit(): void {
    setPageSeo(this.document, {
      title: 'About the Studio | Indifferent',
      description:
        'Indifferent is an AI video studio: TXT quiz files become narrated videos, and long footage becomes ranked vertical Shorts. Built with Angular, Go and AWS.',
      canonical: 'https://indifferent.fun/about',
    });
  }

  currentYear = new Date().getFullYear();
  templates = [
    { name: 'Classic', desc: 'Clean blue theme, professional look' },
    { name: 'Modern', desc: 'Gradient backgrounds, contemporary design' },
    { name: 'Education', desc: 'Warm colors, classroom-friendly' },
    { name: 'Dark', desc: 'Dark mode, high contrast for readability' },
    { name: 'Minimal', desc: 'Simple white, distraction-free' },
    { name: 'Neon', desc: 'Vibrant colors, energetic style' },
  ];
  voices = [
    { name: 'Joanna', desc: 'US English, female, clear and professional', initials: 'JO' },
    { name: 'Matthew', desc: 'US English, male, warm and authoritative', initials: 'MA' },
    { name: 'Amy', desc: 'British English, female, polished', initials: 'AM' },
    { name: 'Brian', desc: 'British English, male, natural', initials: 'BR' },
    { name: 'Aditi', desc: 'Indian English, female, approachable', initials: 'AD' },
  ];
  uses = [
    { title: 'Teachers', desc: 'Revision videos for students' },
    { title: 'YouTube creators', desc: 'Scale quiz and trivia output' },
    { title: 'E-learning', desc: 'Automate course video creation' },
    { title: 'Corporate trainers', desc: 'Compliance quizzes as video' },
    { title: 'Students', desc: 'More engaging study material' },
  ];
  stack = ['Angular 20 · Frontend', 'Go · Backend', 'AWS Lambda · Compute', 'Amazon Polly · Narration', 'FFmpeg · Video', 'DynamoDB · Database', 'Amplify · CDN', 'Step Functions · Orchestration', 'GSAP · Animation'];
}
