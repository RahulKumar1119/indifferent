import { Component, OnInit, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { RouterLink } from '@angular/router';
import { setPageSeo } from '../../shared/seo';

interface DocSection {
  id: string;
  n: string;
  title: string;
  body?: string;
  items?: string[];
}

@Component({
  selector: 'app-privacy',
  standalone: true,
  imports: [RouterLink],
  styles: [`
    .serif { font-family: 'Cormorant Garamond', 'Playfair Display', Georgia, serif; }
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
    .btn-primary { transition: transform .2s cubic-bezier(0.23,1,0.32,1), background-color .2s; }
    .btn-primary:hover { transform: translateY(-1px); }
    .grain::before {
      content: ''; position: fixed; inset: 0; z-index: 60; pointer-events: none; opacity: .05;
      background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='2'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='0.5'/%3E%3C/svg%3E");
    }
  `],
  template: `
    <div class="grain sans bg-[#FAF7F2] text-[#1A1714] antialiased">
      <header class="fixed top-4 left-1/2 -translate-x-1/2 z-50 w-[min(1120px,calc(100%-2rem))]">
        <nav class="flex items-center justify-between h-14 pl-5 pr-2 rounded-full bg-[#0F0E0B]/90 backdrop-blur-xl border border-white/10 shadow-[0_8px_30px_rgba(20,15,10,0.25)]">
          <a routerLink="/" class="flex items-center gap-2 min-h-[44px] text-[#FAF7F2]">
            <span class="font-semibold tracking-tight text-[17px]">Indifferent<span class="text-[#D96C3D]">.</span></span>
          </a>
          <a routerLink="/login" class="btn-primary inline-flex items-center h-11 px-5 rounded-full bg-[#FAF7F2] text-[#141310] text-[13.5px] font-semibold">Sign in</a>
        </nav>
      </header>

      <section class="relative overflow-hidden bg-[#0F0E0B] text-[#F4EFE6] pt-32 pb-14">
        <div class="absolute inset-0" style="background: radial-gradient(ellipse at 30% 10%, rgba(217,108,61,0.18), transparent 55%), linear-gradient(to bottom, #141310, #0F0E0B);"></div>
        <div class="relative w-[min(1240px,100%)] mx-auto px-5 md:px-10">
          <p class="inline-flex items-center gap-2 rounded-full px-3 py-1 text-[10.5px] uppercase tracking-[0.22em] border border-white/15 text-white/70">Legal · Privacy</p>
          <h1 class="serif mt-5 font-medium leading-[0.95] text-[clamp(2.6rem,6vw,4.6rem)]">Privacy Policy.</h1>
          <p class="mt-4 text-white/60">Last updated: January 2025</p>
        </div>
      </section>

      <main class="w-[min(1240px,100%)] mx-auto px-5 md:px-10 py-14 md:py-20 grid grid-cols-1 lg:grid-cols-[240px_1fr] gap-10">
        <aside class="hidden lg:block">
          <nav class="sticky top-24 rounded-[20px] bg-white border border-black/[0.07] p-6">
            <p class="text-[11px] uppercase tracking-[0.2em] text-[#6B6560] mb-4">Contents</p>
            <ol class="space-y-2.5 text-[13.5px]">
              @for (s of sections; track s.id) {
                <li><a [href]="'#' + s.id" class="text-[#3E3A33] hover:text-[#1E3A2A] transition-colors"><span class="serif italic text-[#1E3A2A] mr-2">{{ s.n }}</span>{{ s.title }}</a></li>
              }
            </ol>
          </nav>
        </aside>
        <div>
          @for (s of sections; track s.id) {
            <section [id]="s.id" class="scroll-mt-28 rounded-[20px] bg-white border border-black/[0.07] p-8 md:p-10 mb-5">
              <h2 class="flex items-baseline gap-4"><span class="serif italic text-[28px] text-[#1E3A2A]">{{ s.n }}</span><span class="serif text-[30px] leading-tight">{{ s.title }}</span></h2>
              @if (s.body) { <p class="mt-4 text-[15px] text-[#3E3A33] leading-relaxed">{{ s.body }}</p> }
              @if (s.items) {
                <ul class="mt-4 space-y-2.5">
                  @for (it of s.items; track it) {
                    <li class="flex gap-2.5 text-[14.5px] text-[#3E3A33]"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#1E3A2A] shrink-0"></span>{{ it }}</li>
                  }
                </ul>
              }
              @if (s.id === 'contact') {
                <p class="mt-4 text-[15px] text-[#3E3A33]">For privacy questions, contact <a href="mailto:support@indifferent.fun" class="font-semibold underline underline-offset-4 decoration-black/25 hover:decoration-black">support&#64;indifferent.fun</a></p>
              }
            </section>
          }
        </div>
      </main>

      <section class="bg-[#0F0E0B] text-[#F4EFE6]">
        <div class="w-[min(1240px,100%)] mx-auto px-5 md:px-10 py-12 flex flex-col sm:flex-row justify-between gap-3 text-[12.5px] text-white/60">
          <span>© {{ currentYear }} Indifferent. All rights reserved.</span>
          <span class="flex flex-wrap">
            <a routerLink="/" class="inline-flex items-center min-h-[44px] px-3 hover:text-white">Home</a>
            <a routerLink="/about" class="inline-flex items-center min-h-[44px] px-3 hover:text-white">Studio</a>
            <a routerLink="/contact" class="inline-flex items-center min-h-[44px] px-3 hover:text-white">Contact</a>
            <a routerLink="/terms" class="inline-flex items-center min-h-[44px] px-3 hover:text-white">Terms</a>
          </span>
        </div>
      </section>
    </div>
  `,
})
export class PrivacyComponent implements OnInit {
  private readonly document = inject(DOCUMENT);

  ngOnInit(): void {
    setPageSeo(this.document, {
      title: 'Privacy Policy | Indifferent',
      description:
        'How Indifferent handles your data: what we collect, how uploads and accounts are stored, cookies, and your choices.',
      canonical: 'https://indifferent.fun/privacy',
    });
  }

  currentYear = new Date().getFullYear();
  sections: DocSection[] = [
    { id: 'collect', n: '01', title: 'Information We Collect', items: ['Account information (email, name via Google OAuth)', 'Uploaded quiz files (.txt)', 'Generated video content', 'Usage data (pages visited, features used)'] },
    { id: 'use', n: '02', title: 'How We Use It', items: ['To provide the video generation service', 'To improve our platform', 'To communicate service updates', 'To provide technical support'] },
    { id: 'storage', n: '03', title: 'Data Storage & Security', items: ['Files stored securely on AWS S3', 'Data encrypted in transit and at rest', 'We do not sell your personal information', 'You can request data deletion at any time'] },
    { id: 'third-party', n: '04', title: 'Third-Party Services', items: ['AWS (hosting, storage, compute)', 'Google (authentication)', 'Amazon Polly (text-to-speech)'] },
    { id: 'rights', n: '05', title: 'Your Rights', items: ['Access your personal data', 'Request data deletion', 'Export your content', 'Opt out of communications'] },
    { id: 'cookies', n: '06', title: 'Cookies', body: 'We use essential cookies for authentication and session management only.' },
    { id: 'changes', n: '07', title: 'Changes to This Policy', body: 'We may update this policy periodically. Changes will be posted on this page.' },
    { id: 'contact', n: '08', title: 'Contact' },
  ];
}
