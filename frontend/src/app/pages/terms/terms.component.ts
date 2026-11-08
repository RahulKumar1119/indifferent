import { Component, OnInit, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { legalUpdatedLabel } from '../../core/config/legal';
import { PublicHeaderComponent } from '../../shared/components/public-header/public-header.component';
import { SiteFooterComponent } from '../../shared/components/site-footer/site-footer.component';
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
  imports: [PublicHeaderComponent, SiteFooterComponent],
  styles: [`
    :host { display: block; }
    .page { background: var(--color-paper); color: var(--color-ink); font-family: var(--font-body); font-size: var(--type-md); line-height: 1.55; -webkit-font-smoothing: antialiased; }
    .wrap { width: min(var(--container), 100%); margin-inline: auto; padding-inline: var(--gutter); }
    h1, p { margin: 0; }
    h1 { font-family: var(--font-display); font-weight: 500; font-size: var(--type-display-s); line-height: 1.04; letter-spacing: var(--tracking-display); overflow-wrap: break-word; min-width: 0; text-wrap: balance; }
    .hero { padding: calc(56px + var(--space-3xl)) 0 var(--space-xl); border-bottom: 1px solid var(--color-rule); }
    .updated { margin-top: var(--space-md); color: var(--color-muted); }
    .layout { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-2xl); padding-block: var(--space-2xl) var(--space-3xl); }
    @media (min-width: 64rem) { .layout { grid-template-columns: 16rem minmax(0, 1fr); } }
    aside { display: none; }
    @media (min-width: 64rem) { aside { display: block; align-self: start; position: sticky; top: var(--space-3xl); } }
    .toc-title { margin: 0 0 var(--space-sm); font-size: var(--type-sm); color: var(--color-muted); }
    .doc-section { scroll-margin-top: var(--space-3xl); }
    .doc-section + .doc-section { margin-top: var(--space-xl); padding-top: var(--space-xl); border-top: 1px solid var(--color-rule); }
    .doc-section h2 { margin: 0; display: flex; align-items: baseline; gap: var(--space-md); }
    .doc-section h2 .n { font-family: var(--font-body); font-size: var(--type-lg); font-weight: 500; color: var(--color-accent-text); font-variant-numeric: tabular-nums; }
    .doc-section ul { margin: var(--space-md) 0 0; }
    .doc-section p.links { display: flex; flex-wrap: wrap; column-gap: var(--space-lg); }
    .doc-section p.links a { display: inline-flex; align-items: center; min-height: 44px; }
  `],
  template: `
    <div class="page hm hm-on-light">
      <app-public-header />
      <section class="hero" aria-labelledby="doc-title">
        <div class="wrap">
          <h1 id="doc-title">Terms of Service.</h1>
          <p class="updated">Last updated: {{ updated }}</p>
        </div>
      </section>
      <main class="wrap layout">
        <aside>
          <nav aria-label="Contents">
            <p class="toc-title">Contents</p>
            <ol class="ui-toc">
              @for (s of sections; track s.id) {
                <li><a [href]="'#' + s.id"><span class="n">{{ s.n }}</span>{{ s.title }}</a></li>
              }
            </ol>
          </nav>
        </aside>
        <div class="ui-doc">
          @for (s of sections; track s.id) {
            <section [id]="s.id" class="doc-section">
              <h2><span class="n">{{ s.n }}</span><span>{{ s.title }}</span></h2>
              @for (p of s.body ?? []; track p) { <p>{{ p }}</p> }
              @if (s.items) {
                <ul>
                  @for (it of s.items; track it) {
                    <li>{{ it }}</li>
                  }
                </ul>
              }
              @if (s.links) {
                <p class="links">
                  @for (l of s.links; track l.href) {
                    <a [href]="l.href" target="_blank" rel="noopener noreferrer">{{ l.label }}</a>
                  }
                </p>
              }
              @if (s.id === 'contact') {
                <p>For questions about terms, contact <a href="mailto:support@indifferent.fun">support&#64;indifferent.fun</a></p>
              }
            </section>
          }
        </div>
      </main>
      <app-site-footer />
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
        'Indifferent offers four tools: the AI Shorts generator, which turns a long video or audio file into captioned vertical clips; the TXT to quiz video generator, which turns a text file of questions into a narrated video; the Story to video generator, described below; and a watermark tool that runs in your browser.',
        'With the Story to video generator you write a short story, check the plan the AI makes from it, choose a photo or clip from Pexels or Pixabay for each scene, and the service adds a narrator’s voice and captions to make a vertical video.',
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
        'The stories you write for Story videos are yours, and so are the plan, narration and video made from them. The photos and clips inside a story video are not: see “Pictures and clips from other libraries”.',
      ],
    },
    {
      id: 'stock',
      n: '05',
      title: 'Pictures and clips from other libraries',
      body: ['Story videos use photos and video clips from Pexels and Pixabay. You choose every one, and we show where each one comes from.'],
      items: [
        'They stay with their creators and under the licences and terms of Pexels and Pixabay. Using one in your video gives you no more rights in it than those licences give.',
        'The download is the finished video only. Do not take the pictures or clips out to sell, share or post on their own, or to put on other stock or wallpaper sites.',
        'Neither library requires a credit in your video. The credit line for each picture is shown beside it if you want to give one.',
        'You are responsible for the pictures you choose. Do not use a picture with a visible logo, brand or product to sell goods or services, or in a way that suggests the brand backs you.',
        'Do not show a person who can be recognised in a bad or offensive light, or as if they support something, and do not use a picture in a misleading way.',
        'Pictures of private homes and other property may need the owner’s permission. A picture being on Pexels or Pixabay does not mean that permission was given.',
        'A library can remove a picture or change its rules. If a picture you chose is gone, you will need to choose another before the video can be made.',
        'We cannot promise that a picture is free of other people’s rights. Check each one before you publish.',
      ],
      links: [
        { label: 'Pexels licence', href: 'https://www.pexels.com/license/' },
        { label: 'Pixabay content licence', href: 'https://pixabay.com/service/license-summary/' },
      ],
    },
    {
      id: 'acceptable-use',
      n: '06',
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
      n: '07',
      title: 'Automatic output',
      body: [
        'Transcripts, the moments chosen, framing and narration are produced automatically and can be wrong. Captions may contain mistakes, and a clip may start or end in a place you would not choose.',
        'In Story videos the plan, the scene wording and the voice are made automatically from your story, and a picture may not show what its scene describes.',
        'Check every clip, caption and narration before you publish it. You are responsible for what you publish.',
      ],
    },
    {
      id: 'limits',
      n: '08',
      title: 'Limits and availability',
      items: [
        'The current limits for file size, length and number of clips are listed on each tool’s page, and they can change.',
        'We do not promise that the service will always be available or free of errors.',
        'We may change or remove features during the beta.',
      ],
    },
    {
      id: 'storage',
      n: '09',
      title: 'Storage and deletion',
      body: [
        'Your projects and Shorts stay in your account until you delete them. Deleting one removes its stored files and its record. A story video can be deleted the same way. The privacy policy lists what is kept, where, and for how long.',
        'Story plans cannot be deleted from the app yet. To have one removed, email support@indifferent.fun.',
      ],
    },
    {
      id: 'termination',
      n: '10',
      title: 'Ending your use',
      items: [
        'You can stop using the service at any time.',
        'We may suspend or remove an account that breaks these terms.',
        'To have your account and its data deleted, email support@indifferent.fun from the address on the account.',
      ],
    },
    {
      id: 'liability',
      n: '11',
      title: 'Disclaimer and liability',
      body: ['The service is provided as it is, without warranties. As far as the law allows, we are not liable for indirect or consequential losses, lost profits or lost data. Keep copies of anything important.'],
    },
    {
      id: 'changes',
      n: '12',
      title: 'Changes to these terms',
      body: ['We may update these terms. The date at the top shows the current version, and using the service after a change means you accept it.'],
    },
    { id: 'contact', n: '13', title: 'Contact' },
  ];
}
