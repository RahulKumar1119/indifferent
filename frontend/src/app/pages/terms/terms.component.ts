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
  selector: 'app-terms',
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
        <div class="absolute inset-0" style="background: radial-gradient(ellipse at 70% 10%, rgba(217,108,61,0.18), transparent 55%), linear-gradient(to bottom, #141310, #0F0E0B);"></div>
        <div class="relative w-[min(1240px,100%)] mx-auto px-5 md:px-10">
          <p class="inline-flex items-center gap-2 rounded-full px-3 py-1 text-[10.5px] uppercase tracking-[0.22em] border border-white/15 text-white/70">Legal · Terms</p>
          <h1 class="serif mt-5 font-medium leading-[0.95] text-[clamp(2.6rem,6vw,4.6rem)]">Terms of Service.</h1>
          <p class="mt-4 text-white/60">Last updated: January 2025</p>
        </div>
      </section>

      <main class="w-[min(1240px,100%)] mx-auto px-5 md:px-10 py-14 md:py-20 grid grid-cols-1 lg:grid-cols-[240px_1fr] gap-10">
        <aside class="hidden lg:block">
          <nav class="sticky top-24 rounded-[20px] bg-white border border-black/[0.07] p-6">
            <p class="text-[11px] uppercase tracking-[0.2em] text-[#6B6560] mb-4">Contents</p>
            <ol class="space-y-2.5 text-[13.5px]">
              @for (s of sections; track s.id) {
                <li><a [href]="'#' + s.id" class="text-[#3E3A33] hover:text-[#BC5227] transition-colors"><span class="serif italic text-[#BC5227] mr-2">{{ s.n }}</span>{{ s.title }}</a></li>
              }
            </ol>
          </nav>
        </aside>
        <div>
          @for (s of sections; track s.id) {
            <section [id]="s.id" class="scroll-mt-28 rounded-[20px] bg-white border border-black/[0.07] p-8 md:p-10 mb-5">
              <h2 class="flex items-baseline gap-4"><span class="serif italic text-[28px] text-[#BC5227]">{{ s.n }}</span><span class="serif text-[30px] leading-tight">{{ s.title }}</span></h2>
              @if (s.body) { <p class="mt-4 text-[15px] text-[#3E3A33] leading-relaxed">{{ s.body }}</p> }
              @if (s.items) {
                <ul class="mt-4 space-y-2.5">
                  @for (it of s.items; track it) {
                    <li class="flex gap-2.5 text-[14.5px] text-[#3E3A33]"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#BC5227] shrink-0"></span>{{ it }}</li>
                  }
                </ul>
              }
              @if (s.id === 'contact') {
                <p class="mt-4 text-[15px] text-[#3E3A33]">For questions about terms, contact <a href="mailto:support@indifferent.fun" class="font-semibold underline underline-offset-4 decoration-black/25 hover:decoration-black">support&#64;indifferent.fun</a></p>
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
            <a routerLink="/privacy" class="inline-flex items-center min-h-[44px] px-3 hover:text-white">Privacy</a>
          </span>
        </div>
      </section>
    </div>
  `,
})
export class TermsComponent implements OnInit {
  private readonly document = inject(DOCUMENT);

  ngOnInit(): void {
    setPageSeo(this.document, {
      title: 'Terms of Service | Indifferent',
      description:
        'The terms of service for Indifferent: acceptable use, content ownership, accounts, availability, and limitation of liability.',
      canonical: 'https://indifferent.fun/terms',
    });
  }

  currentYear = new Date().getFullYear();
  sections: DocSection[] = [
    { id: 'acceptance', n: '01', title: 'Acceptance of Terms', body: 'By using Indifferent you agree to these terms. If you do not agree, please do not use the service.' },
    { id: 'service', n: '02', title: 'Service Description', body: 'Indifferent converts text-based quiz files into video content using automated processing.' },
    { id: 'accounts', n: '03', title: 'User Accounts', items: ['Must be 13+ to use the service', 'Responsible for account security', 'One account per person'] },
    { id: 'acceptable-use', n: '04', title: 'Acceptable Use', items: ['Upload only content you own or have rights to', 'Do not upload harmful or illegal content', 'Do not attempt to reverse engineer the service', 'Do not use for spam or abuse'] },
    { id: 'ownership', n: '05', title: 'Content Ownership', items: ['You retain ownership of uploaded quiz files', 'You retain ownership of generated videos', 'We do not claim rights to your content', 'We may use anonymized usage data to improve the service'] },
    { id: 'availability', n: '06', title: 'Service Availability', items: ['We aim for high availability but do not guarantee 100% uptime', 'We may modify or discontinue features with notice', 'Scheduled maintenance will be communicated in advance'] },
    { id: 'liability', n: '07', title: 'Limitation of Liability', items: ['Service provided "as is"', 'Not liable for content accuracy in generated videos', 'Not liable for indirect damages'] },
    { id: 'termination', n: '08', title: 'Termination', items: ['We may suspend accounts violating these terms', 'You may delete your account at any time', 'Upon termination your data will be deleted within 30 days'] },
    { id: 'changes', n: '09', title: 'Changes to Terms', body: 'We may update terms with 30 days notice. Continued use constitutes acceptance.' },
    { id: 'contact', n: '10', title: 'Contact' },
  ];
}
