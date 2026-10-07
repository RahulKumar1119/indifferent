import { Component, OnInit, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { RouterLink } from '@angular/router';
import { setPageSeo } from '../../shared/seo';

@Component({
  selector: 'app-contact',
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
            <a routerLink="/about" class="hover:text-white transition-colors">Studio</a>
            <a routerLink="/contact" class="text-white font-medium">Contact</a>
          </div>
          <a routerLink="/login" class="btn-primary inline-flex items-center gap-2 h-10 pl-5 pr-1.5 rounded-full bg-[#FAF7F2] text-[#141310] text-[13.5px] font-semibold">
            Start creating
            <span class="w-7 h-7 rounded-full bg-[#1A1714] text-white flex items-center justify-center text-sm leading-none">↗</span>
          </a>
        </nav>
      </header>

      <section class="relative overflow-hidden bg-[#0F0E0B] text-[#F4EFE6] pt-32 pb-16 md:pb-20">
        <img src="https://picsum.photos/seed/indifferent-contact-hero/1800/900" alt="" aria-hidden="true" class="absolute inset-0 w-full h-full object-cover opacity-30">
        <div class="absolute inset-0" style="background: radial-gradient(ellipse at 25% 15%, rgba(217,108,61,0.20), transparent 55%), linear-gradient(to bottom, rgba(15,14,11,0.55), rgba(15,14,11,0.93));"></div>
        <div class="relative w-[min(1240px,100%)] mx-auto px-5 md:px-10">
          <p class="inline-flex items-center gap-2 rounded-full px-3 py-1 text-[10.5px] uppercase tracking-[0.22em] border border-white/15 text-white/70">
            <span class="w-1.5 h-1.5 rounded-full bg-[#D96C3D]"></span> Correspondence
          </p>
          <h1 class="serif mt-5 font-medium leading-[0.95] tracking-[-0.02em] text-[clamp(2.8rem,7vw,5.2rem)] text-balance">Write to the studio.</h1>
          <p class="mt-6 text-[17px] leading-relaxed text-white/70 max-w-[48ch]">Questions, feedback, or feature requests — reach out through any channel below.</p>
        </div>
      </section>

      <main class="w-[min(1240px,100%)] mx-auto px-5 md:px-10 py-16 md:py-24">
        <div class="grid grid-cols-1 lg:grid-cols-5 gap-5">
          <div class="lg:col-span-3 rounded-[20px] bg-white border border-black/[0.07] p-8 md:p-10">
            <p class="text-[11.5px] uppercase tracking-[0.2em] text-[#6B6560]">Bugs · Features · Questions</p>
            <h2 class="serif mt-2 text-[32px] leading-tight">GitHub issues</h2>
            <p class="mt-3 text-[15px] text-[#6B6560] leading-relaxed">The fastest route. Report a bug, request a feature, or ask a question where the work happens.</p>
            <a href="https://github.com/RahulKumar1119/indifferent/issues" target="_blank" rel="noopener" class="btn-primary mt-6 inline-flex items-center gap-2 rounded-full bg-[#1A1714] text-white px-7 py-3.5 font-semibold text-[14.5px]">Open an issue →</a>
          </div>
          <div class="lg:col-span-2 flex flex-col gap-5">
            <div class="rounded-[20px] bg-white border border-black/[0.07] p-8 flex-1">
              <p class="text-[11.5px] uppercase tracking-[0.2em] text-[#6B6560]">Email</p>
              <h2 class="serif mt-2 text-[28px]">General inquiries</h2>
              <a href="mailto:support@indifferent.fun" class="mt-3 inline-block text-[16px] font-semibold underline underline-offset-8 decoration-[#1A1714]/25 hover:decoration-[#1A1714] transition">support&#64;indifferent.fun</a>
            </div>
            <div class="rounded-[20px] bg-[#1E3A2A] text-[#EDE8DB] p-8 flex-1">
              <p class="text-[11.5px] uppercase tracking-[0.2em] text-white/60">Response time</p>
              <p class="serif mt-2 text-[28px] leading-tight">Within 24–48 hours.</p>
              <p class="mt-2 text-[13.5px] text-white/65">Usually faster on weekdays.</p>
            </div>
          </div>
        </div>
      </main>

      <section class="bg-[#0F0E0B] text-[#F4EFE6]">
        <div class="w-[min(1240px,100%)] mx-auto px-5 md:px-10 py-12 flex flex-col sm:flex-row justify-between gap-3 text-[12.5px] text-white/40">
          <span>© {{ currentYear }} Indifferent. All rights reserved.</span>
          <span class="flex gap-5">
            <a routerLink="/" class="hover:text-white">Home</a>
            <a routerLink="/about" class="hover:text-white">Studio</a>
            <a routerLink="/privacy" class="hover:text-white">Privacy</a>
            <a routerLink="/terms" class="hover:text-white">Terms</a>
          </span>
        </div>
      </section>
    </div>
  `,
})
export class ContactComponent implements OnInit {
  private readonly document = inject(DOCUMENT);

  ngOnInit(): void {
    setPageSeo(this.document, {
      title: 'Contact | Indifferent',
      description:
        'Get in touch with the Indifferent studio: support, feedback, and feature requests for the AI video platform.',
      canonical: 'https://indifferent.fun/contact',
    });
  }

  currentYear = new Date().getFullYear();
}
