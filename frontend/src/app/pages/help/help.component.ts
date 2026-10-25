import { Component, OnInit, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { RouterLink } from '@angular/router';
import { SiteFooterComponent } from '../../shared/components/site-footer/site-footer.component';
import { setPageSeo } from '../../shared/seo';
import { FEATURE_DATA, FeatureData, FeatureFaq } from '../features/feature-page.component';

interface HelpSection {
  id: string;
  title: string;
  intro?: string;
  faqs: FeatureFaq[];
  /** Link to the full product page, when the section is about one tool. */
  more?: { label: string; path: string };
}

/** Questions that apply to the whole site. Anything about a single tool comes
 * from FEATURE_DATA, so the help page and the product pages cannot disagree. */
export const GENERAL_FAQ: FeatureFaq[] = [
  {
    q: 'Is Indifferent free?',
    a: 'Yes, during the beta. There is no credit card and no paid plan yet.',
  },
  {
    q: 'Do I need an account?',
    a: 'The AI Shorts generator and the TXT to quiz video generator need you to sign in with Google. The watermark tool needs no account.',
  },
  {
    q: 'How long are my files kept?',
    a: 'Until you delete them. Uploaded source files move to archive storage after 30 days, and temporary working files are removed after 7 days. Nothing else is deleted automatically. The privacy policy has the details.',
  },
  {
    q: 'How do I delete my files?',
    a: 'Delete the project or the Short from your library. That removes its stored files and its record.',
  },
  {
    q: 'How do I delete my account?',
    a: 'Email support@indifferent.fun from the address on the account and ask for it to be deleted. There is no delete button in the app yet, and requests are handled by hand.',
  },
  {
    q: 'Which browsers work?',
    a: 'Use a current version of Chrome, Edge, Firefox or Safari.',
  },
  {
    q: 'Where do I report a problem or ask for a feature?',
    a: 'Open an issue on GitHub, or email support@indifferent.fun. The contact page lists both.',
  },
];

@Component({
  selector: 'app-help',
  standalone: true,
  imports: [RouterLink, SiteFooterComponent],
  styles: [`
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
    .serif { font-family: 'Cormorant Garamond', 'Playfair Display', Georgia, serif; }
  `],
  template: `
    <div class="sans bg-[#FAF7F2] text-[#1A1714] antialiased min-h-[100dvh]">
      <header class="max-w-3xl mx-auto px-4 pt-24 pb-6">
        <a routerLink="/" class="inline-flex items-center min-h-[44px] text-[13px] text-[#6B6560] hover:text-[#1A1714]">← Indifferent</a>
        <h1 class="serif mt-4 font-medium tracking-[-0.02em] leading-[1.02] text-[clamp(2.2rem,5vw,3.4rem)]">Help and FAQ</h1>
        <p class="mt-4 text-[16.5px] leading-relaxed text-[#3E3A33]">
          Answers about the three tools, your files and your account. Each tool also has a page with its exact limits and a step-by-step guide.
        </p>
        <nav aria-label="Sections" class="mt-6 flex flex-wrap gap-2">
          @for (s of sections; track s.id) {
            <a [href]="'#' + s.id" class="inline-flex items-center min-h-[44px] rounded-full border border-black/15 px-4 text-[13px] hover:bg-black/5 transition-colors">{{ s.title }}</a>
          }
        </nav>
      </header>

      <main class="max-w-3xl mx-auto px-4 pb-12">
        @for (s of sections; track s.id) {
          <section [id]="s.id" class="mt-10 scroll-mt-24" [attr.aria-labelledby]="s.id + '-heading'">
            <h2 [id]="s.id + '-heading'" class="text-[22px] font-semibold">{{ s.title }}</h2>
            @if (s.intro) {
              <p class="mt-2 text-[14.5px] leading-relaxed text-[#3E3A33]">{{ s.intro }}</p>
            }
            <div class="mt-4 space-y-3">
              @for (f of s.faqs; track f.q) {
                <details class="rounded-[14px] border border-black/10 bg-white px-5 py-4">
                  <summary class="cursor-pointer font-semibold text-[14.5px]">{{ f.q }}</summary>
                  <p class="mt-3 text-[14px] leading-relaxed text-[#3E3A33]">{{ f.a }}</p>
                </details>
              }
            </div>
            @if (s.more) {
              <p class="mt-4"><a [routerLink]="s.more.path" class="inline-flex items-center min-h-[44px] text-[14px] font-semibold underline underline-offset-4">{{ s.more.label }}</a></p>
            }
          </section>
        }

        <section class="mt-12 rounded-[20px] bg-[#1A1714] text-white px-6 py-8" aria-labelledby="contact-heading">
          <h2 id="contact-heading" class="text-[20px] font-semibold">Still stuck?</h2>
          <p class="mt-2 text-[14.5px] leading-relaxed text-white/75">
            Email <a href="mailto:support@indifferent.fun" class="underline underline-offset-4">support&#64;indifferent.fun</a> with the file type, its length and what the page showed. The <a routerLink="/contact" class="underline underline-offset-4">contact page</a> lists the other ways to reach us.
          </p>
        </section>
      </main>
      <app-site-footer />
    </div>
  `,
})
export class HelpComponent implements OnInit {
  private readonly document = inject(DOCUMENT);

  readonly sections: HelpSection[] = [
    { id: 'general', title: 'Accounts, files and pricing', faqs: GENERAL_FAQ },
    ...(['ai-shorts', 'txt-to-video-quiz', 'watermark'] as const).map((slug) => this.toolSection(FEATURE_DATA[slug])),
  ];

  ngOnInit(): void {
    setPageSeo(this.document, {
      title: 'Help and FAQ | Indifferent',
      description:
        'Answers about the AI Shorts generator, the TXT to quiz video generator and the watermark tool: limits, file formats, fixes, privacy and your account.',
      canonical: 'https://indifferent.fun/help',
    });
  }

  private toolSection(f: FeatureData): HelpSection {
    return {
      id: f.slug,
      title: f.title,
      intro: f.hook,
      faqs: f.troubleshooting,
      more: { label: `${f.title}: limits and step-by-step guide`, path: `/features/${f.slug}` },
    };
  }
}
