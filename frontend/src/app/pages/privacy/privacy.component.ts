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
  selector: 'app-privacy',
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
          <h1 id="doc-title">Privacy Policy.</h1>
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
                <p>For privacy questions, contact <a href="mailto:support@indifferent.fun">support&#64;indifferent.fun</a></p>
              }
            </section>
          }
        </div>
      </main>
      <app-site-footer />
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
        'Indifferent (indifferent.fun) is four tools: the AI Shorts generator, the TXT to quiz video generator, the Story to video generator and a watermark tool. It is built and run by one developer and is in beta.',
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
        'For Story videos: the plan made from your story (a hook, narration lines, scene descriptions, search words, and the picture you chose for each scene), and each video’s voice, caption style, status and timestamps. The story itself is not kept: see section 03.',
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
        'Story videos, the plan: the story you write is sent to a language model on Amazon Bedrock, which writes the plan. We keep the plan and not the story, and only the story’s length is recorded. The plan’s narration lines can follow your own wording closely, and they are kept.',
        'Story videos, the pictures: when you search, our server sends the search words (the scene’s keywords, or what you type) to Pexels and Pixabay, without your name, email or account. When you make a plan, our server may also search Pixabay with each scene’s keywords to suggest a picture, the same search the picker makes when you open it. A scene whose keywords look like contact details, a link or a long number is not searched. Your browser then loads the preview pictures and clips directly from those services and the networks they use, so they can see your IP address and browser details, as any site you visit can.',
        'Story videos, the video: the pictures and clips you chose are copied into our private storage, Amazon Polly speaks the narration, and FFmpeg renders the video on AWS. Only the narration text goes to Polly. The pictures and clips are not sent to any image or face recognition service.',
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
        'For Story videos, the pictures and clips copied from Pexels and Pixabay and the narration audio are working files, so they go after 7 days. The finished video stays until you delete it, and deleting it removes the video, its working files and its record.',
        'Story plans stay until you ask us to remove them. The app has no button to delete a plan yet.',
        'Counts of events, such as how many story plans were made or failed, are kept in Amazon CloudWatch to see how the tool is doing. They carry no name, email, account id or story text. Technical logs record account and job ids and error details, not your story text. A shared cache of picture search results holds what Pexels and Pixabay returned, not who searched.',
      ],
    },
    {
      id: 'third-party',
      n: '05',
      title: 'Services that receive your data',
      items: [
        'Amazon Web Services: hosting (Amplify), storage (S3), database (DynamoDB), compute (Lambda and Fargate), workflows (Step Functions), speech (Polly and Transcribe), AI models (Bedrock), face detection (Rekognition), email (SES) and monitoring (CloudWatch).',
        'Google: sign-in with Google, Google Analytics, Google Ads conversion measurement, Google AdSense and Google Fonts, which serves the site typefaces.',
        'Pexels and Pixabay: the photo and clip libraries behind Story videos. They receive the search words from our server, and your browser loads previews from them and the networks they use, as section 03 describes.',
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
        'We send an email through Amazon SES when a quiz video, a Shorts job or a story video finishes or fails. You can turn these off in Settings.',
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
        'Google Ads: when you click “Continue with Google”, the Google tag tells Google Ads that the click happened, so we can measure which ads lead to sign-ins. We send no name or email with it. The tag can set cookies for this, and it loads together with Google Analytics.',
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
        'Delete any project, Short or story video whenever you like. A story plan can be removed on request.',
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
