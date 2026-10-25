import { Component, OnInit, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { RouterLink } from '@angular/router';
import { legalUpdatedLabel } from '../../core/config/legal';
import { setPageSeo } from '../../shared/seo';

interface DocSection {
  id: string;
  n: string;
  title: string;
  /** Paragraphs shown above the list. */
  body?: string[];
  items?: string[];
  links?: { label: string; href: string }[];
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
          <p class="inline-flex items-center gap-2 rounded-full px-3 py-1 text-[10.5px] uppercase tracking-[0.1em] border border-white/15 text-white/70">Legal · Terms</p>
          <h1 class="serif mt-5 font-medium leading-[0.95] text-[clamp(2.6rem,6vw,4.6rem)]">Terms of Service.</h1>
          <p class="mt-4 text-white/60">Last updated: {{ updated }}</p>
        </div>
      </section>

      <main class="w-[min(1240px,100%)] mx-auto px-5 md:px-10 py-14 md:py-20 grid grid-cols-1 lg:grid-cols-[240px_1fr] gap-10">
        <aside class="hidden lg:block">
          <nav class="sticky top-24 rounded-[20px] bg-white border border-black/[0.07] p-6">
            <p class="text-[11px] uppercase tracking-[0.08em] text-[#6B6560] mb-4">Contents</p>
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
              @for (p of s.body ?? []; track p) { <p class="mt-4 text-[15px] text-[#3E3A33] leading-relaxed">{{ p }}</p> }
              @if (s.items) {
                <ul class="mt-4 space-y-2.5">
                  @for (it of s.items; track it) {
                    <li class="flex gap-2.5 text-[14.5px] text-[#3E3A33]"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#BC5227] shrink-0"></span>{{ it }}</li>
                  }
                </ul>
              }
              @if (s.links) {
                <p class="mt-4 flex flex-wrap gap-x-5 text-[14.5px] text-[#3E3A33]">
                  @for (l of s.links; track l.href) {
                    <a [href]="l.href" target="_blank" rel="noopener noreferrer" class="inline-flex items-center min-h-[44px] underline underline-offset-4 decoration-black/30 hover:decoration-black">{{ l.label }}</a>
                  }
                </p>
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
  readonly updated = legalUpdatedLabel();
  sections: DocSection[] = [
    {
      id: 'acceptance',
      n: '01',
      title: 'Acceptance of these terms',
      body: ['By using Indifferent you agree to these terms and to the privacy policy. If you do not agree, please do not use the service.'],
    },
    {
      id: 'service',
      n: '02',
      title: 'The service',
      body: [
        'Indifferent offers three tools: the AI Shorts generator, which turns a long video or audio file into captioned vertical clips; the TXT to quiz video generator, which turns a text file of questions into a narrated video; and a watermark tool that runs in your browser.',
        'The service is in beta and free to use. Features, limits and availability can change.',
      ],
    },
    {
      id: 'accounts',
      n: '03',
      title: 'Accounts',
      items: [
        'You must be at least 13 years old.',
        'You sign in with Google. You are responsible for keeping your account secure and for what happens under it.',
        'One account per person.',
        'The watermark tool needs no account.',
      ],
    },
    {
      id: 'content',
      n: '04',
      title: 'Your content',
      items: [
        'You keep ownership of what you upload, and of the transcripts, clips and videos made from it.',
        'You give us permission to store and process your content, and to pass it to the service providers named in the privacy policy, only to run the tools for you.',
        'You are responsible for having the rights to everything you upload, including footage, music, scripts, and recordings of other people. Do not upload content you are not allowed to copy or process.',
      ],
    },
    {
      id: 'acceptable-use',
      n: '05',
      title: 'Acceptable use',
      items: [
        'Do not upload content that is illegal, harmful, or that infringes someone else’s rights.',
        'Do not upload recordings of people where the law requires their consent and you do not have it.',
        'Do not try to break, overload, or reverse engineer the service.',
        'Do not use the service to send spam or to harass anyone.',
      ],
    },
    {
      id: 'output',
      n: '06',
      title: 'Automatic output',
      body: [
        'Transcripts, the moments chosen, framing and narration are produced automatically and can be wrong. Captions may contain mistakes, and a clip may start or end in a place you would not choose.',
        'Check every clip, caption and narration before you publish it. You are responsible for what you publish.',
      ],
    },
    {
      id: 'limits',
      n: '07',
      title: 'Limits and availability',
      items: [
        'The current limits for file size, length and number of clips are listed on each tool’s page, and they can change.',
        'We do not promise that the service will always be available or free of errors.',
        'We may change or remove features during the beta.',
      ],
    },
    {
      id: 'storage',
      n: '08',
      title: 'Storage and deletion',
      body: ['Your projects and Shorts stay in your account until you delete them. Deleting one removes its stored files and its record. The privacy policy lists what is kept, where, and for how long.'],
    },
    {
      id: 'termination',
      n: '09',
      title: 'Ending your use',
      items: [
        'You can stop using the service at any time.',
        'We may suspend or remove an account that breaks these terms.',
        'To have your account and its data deleted, email support@indifferent.fun from the address on the account.',
      ],
    },
    {
      id: 'liability',
      n: '10',
      title: 'Disclaimer and liability',
      body: ['The service is provided as it is, without warranties. As far as the law allows, we are not liable for indirect or consequential losses, lost profits or lost data. Keep copies of anything important.'],
    },
    {
      id: 'changes',
      n: '11',
      title: 'Changes to these terms',
      body: ['We may update these terms. The date at the top shows the current version, and using the service after a change means you accept it.'],
    },
    { id: 'contact', n: '12', title: 'Contact' },
  ];
}
