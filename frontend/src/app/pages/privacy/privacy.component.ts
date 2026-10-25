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
          <p class="inline-flex items-center gap-2 rounded-full px-3 py-1 text-[10.5px] uppercase tracking-[0.1em] border border-white/15 text-white/70">Legal · Privacy</p>
          <h1 class="serif mt-5 font-medium leading-[0.95] text-[clamp(2.6rem,6vw,4.6rem)]">Privacy Policy.</h1>
          <p class="mt-4 text-white/60">Last updated: {{ updated }}</p>
        </div>
      </section>

      <main class="w-[min(1240px,100%)] mx-auto px-5 md:px-10 py-14 md:py-20 grid grid-cols-1 lg:grid-cols-[240px_1fr] gap-10">
        <aside class="hidden lg:block">
          <nav class="sticky top-24 rounded-[20px] bg-white border border-black/[0.07] p-6">
            <p class="text-[11px] uppercase tracking-[0.08em] text-[#6B6560] mb-4">Contents</p>
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
              @for (p of s.body ?? []; track p) { <p class="mt-4 text-[15px] text-[#3E3A33] leading-relaxed">{{ p }}</p> }
              @if (s.items) {
                <ul class="mt-4 space-y-2.5">
                  @for (it of s.items; track it) {
                    <li class="flex gap-2.5 text-[14.5px] text-[#3E3A33]"><span class="mt-[7px] w-1.5 h-1.5 rounded-full bg-[#1E3A2A] shrink-0"></span>{{ it }}</li>
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
  readonly updated = legalUpdatedLabel();
  sections: DocSection[] = [
    {
      id: 'who',
      n: '01',
      title: 'Who we are',
      body: [
        'Indifferent (indifferent.fun) is three tools: the AI Shorts generator, the TXT to quiz video generator and a watermark tool. It is built and run by one developer and is in beta.',
        'This policy says what the tools collect, where it goes, how long it stays and how you remove it. If something is missing, write to us at the address in section 12.',
      ],
    },
    {
      id: 'collect',
      n: '02',
      title: 'What we collect',
      body: ['The watermark tool collects nothing: your images and videos stay in your browser. The other two tools collect the following.'],
      items: [
        'Account details from Google sign-in: your name, email address and profile picture.',
        'Files you upload: TXT quiz files, and video or audio files for AI Shorts.',
        'What the tools make from them: transcripts, the moments chosen and their scores, rendered clips, quiz slides and videos, and thumbnails.',
        'Project records: names, the options you chose (voice, design, language, caption style, effects), status and timestamps.',
        'Your notification preferences from Settings.',
        'Usage and technical data from Google Analytics and Amazon CloudWatch RUM: pages visited, errors and performance. Section 07 describes them.',
      ],
    },
    {
      id: 'tools',
      n: '03',
      title: 'How each tool handles your files',
      items: [
        'TXT to quiz video: the text file is uploaded and read on our servers. Amazon Polly turns each question and answer into speech, and the slides and video are rendered on AWS. The file and the result are stored with your project so you can render again.',
        'AI Shorts: the video or audio file is uploaded to private storage. Amazon Transcribe produces the transcript. The transcript is sent to a language model on Amazon Bedrock, which picks the moments. Frames sampled from the video are sent to Amazon Rekognition to find faces. FFmpeg renders the clips. The thumbnail feature sends frames to Amazon Rekognition and Amazon Bedrock.',
        'Watermark tool: images and video are processed inside your browser tab and are not uploaded. The first video export downloads a processing engine of about 30 MB from the jsDelivr CDN. That request does not include your file.',
      ],
      body: ['We do not sell your content, and we do not use it to train any model.'],
    },
    {
      id: 'storage',
      n: '04',
      title: 'Where data is kept, and for how long',
      items: [
        'Files are kept in a private Amazon S3 bucket in the AWS Asia Pacific (Mumbai) region, with public access blocked.',
        'Project and job records are kept in Amazon DynamoDB, in the same region.',
        'AWS encrypts S3 objects and DynamoDB tables at rest, and the site and API use HTTPS.',
        'Uploaded source files move to archive storage after 30 days. Temporary working files are deleted after 7 days.',
        'Nothing else is deleted automatically. Projects, Shorts, clips and their records stay until you delete them.',
        'Deleting a project or a Short removes its stored files and its database record.',
      ],
    },
    {
      id: 'third-party',
      n: '05',
      title: 'Services that receive your data',
      items: [
        'Amazon Web Services: hosting (Amplify), storage (S3), database (DynamoDB), compute (Lambda and Fargate), workflows (Step Functions), speech (Polly and Transcribe), AI models (Bedrock), face detection (Rekognition), email (SES) and monitoring (CloudWatch).',
        'Google: sign-in with Google, Google Analytics, Google AdSense and Google Fonts, which serves the site typefaces.',
        'ImprovMX: forwards email sent to support@indifferent.fun to the developer.',
        'jsDelivr: serves the video engine for the watermark tool.',
        'GitHub: hosts the public issue tracker linked from the contact page. If you post there, GitHub’s policy applies.',
      ],
    },
    {
      id: 'email',
      n: '06',
      title: 'Email',
      body: [
        'We send an email through Amazon SES when a quiz video or a Shorts job finishes or fails. You can turn these off in Settings.',
        'We send no marketing email, and the site has no newsletter.',
      ],
    },
    {
      id: 'cookies',
      n: '07',
      title: 'Cookies, local storage and advertising',
      items: [
        'Sign-in: your browser keeps a refresh token in local storage, not in a cookie, until you sign out.',
        'Preferences: local storage also holds your theme, the progress of an upload in flight, an upload speed estimate used for time estimates, and some profile settings.',
        'Google Analytics sets cookies to measure how the site is used. It loads after your first scroll, click, touch or key press, or after three seconds.',
        'Amazon CloudWatch RUM sets cookies to record sessions, errors and page performance.',
        'Google AdSense: the site includes the AdSense script. Where Google shows ads, Google and its partners can use cookies and similar technologies to show and measure them.',
        'There is no cookie banner at the moment. You can block or delete cookies in your browser settings. The tools keep working without them.',
      ],
      links: [
        { label: 'How Google uses data from sites that use its services', href: 'https://policies.google.com/technologies/partner-sites' },
        { label: 'Google Ads Settings', href: 'https://myadcenter.google.com/' },
      ],
    },
    {
      id: 'rights',
      n: '08',
      title: 'Your choices',
      items: [
        'Delete any project or Short from your library whenever you like.',
        'Download your finished videos and clips at any time.',
        'Turn off completion emails in Settings, and sign out to remove the refresh token from your browser.',
        'To get a copy of your data, correct it, or have your account deleted, email support@indifferent.fun from the address on the account. These requests are handled by hand, because the app has no delete-account button yet.',
      ],
    },
    {
      id: 'children',
      n: '09',
      title: 'Children',
      body: ['Indifferent is not directed at children, and the terms require users to be at least 13. We do not knowingly collect data from younger children.'],
    },
    {
      id: 'transfers',
      n: '10',
      title: 'Where processing happens',
      body: ['Your files and records are stored and processed in AWS Mumbai. Some services named above, such as Google, process data in other countries under their own terms.'],
    },
    {
      id: 'changes',
      n: '11',
      title: 'Changes to this policy',
      body: ['When this policy changes, the date at the top changes with it. Using the service after a change means you accept the new version.'],
    },
    { id: 'contact', n: '12', title: 'Contact' },
  ];
}
