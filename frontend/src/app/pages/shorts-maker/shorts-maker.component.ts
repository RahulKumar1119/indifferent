import { Component, OnInit, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { Meta, Title } from '@angular/platform-browser';
import { RouterLink } from '@angular/router';
import { PublicHeaderComponent } from '../../shared/components/public-header/public-header.component';
import { SiteFooterComponent } from '../../shared/components/site-footer/site-footer.component';
import { RevealDirective } from '../../shared/directives/reveal.directive';
import { LucideDynamicIcon } from '@lucide/angular';
import { setCanonical, setJsonLd } from '../../shared/seo';

interface Feature {
  icon: string;
  title: string;
  text: string;
}

interface UseCase {
  slug: string;
  title: string;
  text: string;
}


interface Faq {
  q: string;
  a: string;
}

interface UseCaseLink {
  slug: string;
  title: string;
}

@Component({
  selector: 'app-shorts-maker',
  standalone: true,
  imports: [RouterLink, LucideDynamicIcon, PublicHeaderComponent, SiteFooterComponent, RevealDirective],
  styles: [`
    :host { display: block; }
    .page { background: var(--color-paper); color: var(--color-ink); font-family: var(--font-body); font-size: var(--type-md); line-height: 1.55; -webkit-font-smoothing: antialiased; }
    .wrap { width: min(var(--container), 100%); margin-inline: auto; padding-inline: var(--gutter); }
    h1, h2, h3, p, ul, ol { margin: 0; }
    h1 { font-family: var(--font-display); font-weight: 500; font-size: var(--type-display); line-height: 1.02; letter-spacing: var(--tracking-display); overflow-wrap: break-word; min-width: 0; text-wrap: balance; max-width: 16ch; }
    h2 { font-family: var(--font-display); font-weight: 500; font-size: var(--type-section); line-height: 1.04; letter-spacing: var(--tracking-display); overflow-wrap: break-word; min-width: 0; text-wrap: balance; max-width: 22ch; }
    h3 { font-size: var(--type-md); font-weight: 600; }
    .lead { margin-top: var(--space-lg); font-size: var(--type-lead); line-height: 1.45; color: var(--color-ink-2); max-width: 52ch; }
    .body { margin-top: var(--space-md); color: var(--color-ink-2); max-width: 60ch; }
    .muted { color: var(--color-muted); }
    .fine { margin-top: var(--space-xs); font-size: var(--type-sm); color: var(--color-muted); }
    a.inline { color: var(--color-accent-text); text-decoration: underline; text-underline-offset: 0.2em; }
    a.inline:hover { color: var(--color-ink); }
    a:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: 2px; }

    .hero { background: var(--color-paper); color: var(--color-ink); padding: calc(56px + var(--space-3xl)) 0 var(--space-3xl); }
    .actions { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-md); margin-top: var(--space-xl); }
    .block { padding: var(--space-3xl) 0; border-top: 1px solid var(--color-rule); scroll-margin-top: var(--space-3xl); }
    .block.first { border-top: 0; }
    .gap-top { margin-top: var(--space-3xl); }
    .block.tint { background: var(--color-paper-2); }

    .proof { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-xl); align-items: center; margin-top: var(--space-2xl); }
    @media (min-width: 48rem) { .proof { grid-template-columns: minmax(0, 20rem) minmax(0, 1fr); } }
    .proof.flush { margin-top: 0; }
    .proof img { display: block; width: 100%; height: auto; border: 1px solid var(--color-rule); border-radius: var(--radius-surface); }
    .proof .label { font-size: var(--type-sm); color: var(--color-muted); }
    .proof p + .label { margin-top: var(--space-lg); }
    .proof p.text { margin-top: var(--space-2xs); color: var(--color-ink-2); }

    ul.captions { list-style: none; padding: 0; margin: var(--space-xl) 0 0; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-md); }
    @media (min-width: 40rem) { ul.captions { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
    figure { margin: 0; }
    .frame { position: relative; aspect-ratio: 9 / 16; width: 100%; overflow: hidden; background: var(--color-paper-2); border: 1px solid var(--color-rule); border-radius: var(--radius-surface); }
    .frame .cap { position: absolute; inset-inline: 0; bottom: var(--space-sm); padding-inline: var(--space-xs); text-align: center; }
    figcaption { margin-top: var(--space-sm); }
    figcaption .name { display: block; font-size: var(--type-sm); font-weight: 600; }
    figcaption .note { display: block; font-size: var(--type-xs); color: var(--color-muted); }

    ul.items { list-style: none; padding: 0; margin: var(--space-xl) 0 0; display: grid; grid-template-columns: minmax(0, 1fr); border-top: 1px solid var(--color-rule); }
    @media (min-width: 64rem) { ul.items { grid-template-columns: repeat(2, minmax(0, 1fr)); column-gap: var(--space-2xl); } }
    ul.items > li { display: grid; grid-template-columns: 40px minmax(0, 1fr); gap: var(--space-md); padding: var(--space-lg) 0; border-bottom: 1px solid var(--color-rule); }
    .icon { color: var(--color-accent-text); padding-top: 2px; }
    ul.items p { margin-top: var(--space-2xs); color: var(--color-ink-2); }

    ol.steps { list-style: none; padding: 0; margin: var(--space-xl) 0 0; display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-xl); }
    @media (min-width: 48rem) { ol.steps { grid-template-columns: repeat(3, minmax(0, 1fr)); gap: var(--space-xl); } }
    ol.steps li { padding-top: var(--space-md); border-top: 1px solid var(--color-rule); }
    ol.steps .step { font-size: var(--type-sm); font-weight: 600; color: var(--color-accent-text); }
    ol.steps h3 { margin-top: var(--space-xs); font-size: var(--type-lg); }
    ol.steps p { margin-top: var(--space-xs); color: var(--color-ink-2); }

    ul.rows { list-style: none; padding: 0; margin: var(--space-xl) 0 0; border-top: 1px solid var(--color-rule); }
    ul.rows > li { border-bottom: 1px solid var(--color-rule); }
    .row-link { display: flex; align-items: center; justify-content: space-between; gap: var(--space-lg); padding: var(--space-lg) var(--space-xs); color: inherit; text-decoration: none; transition: background-color var(--dur-short) var(--ease-out); }
    .row-link:hover { background: var(--color-paper-2); }
    .row-title { display: block; font-family: var(--font-display); font-size: var(--type-2xl); line-height: 1.15; font-weight: 500; }
    .row-text { display: block; margin-top: var(--space-2xs); color: var(--color-muted); max-width: 56ch; }
    .row-cta { color: var(--color-accent-text); font-weight: 600; flex-shrink: 0; }

    .facts { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-lg); margin: 0; }
    .facts dt { font-size: var(--type-sm); color: var(--color-muted); }
    .facts dd { margin: var(--space-3xs) 0 0; font-family: var(--font-display); font-size: var(--type-3xl); font-weight: 500; font-variant-numeric: lining-nums tabular-nums; }

    .faq-list { display: grid; gap: var(--space-sm); margin-top: var(--space-xl); max-width: 52rem; }
    .closing { text-align: left; }
    .more { display: flex; flex-wrap: wrap; gap: var(--space-xs) var(--space-lg); margin-top: var(--space-xl); }
    .more a { display: inline-flex; align-items: center; min-height: 44px; }
  `],
  template: `
    <div class="page hm hm-on-light">
      <app-public-header />

      <section class="hero hm-on-dark" aria-labelledby="shorts-title">
        <div class="wrap">
          <h1 id="shorts-title">Turn long videos into ranked vertical shorts</h1>
          <p class="lead">
            Upload a video or audio file.
            AI transcribes it, picks the strongest moments from the transcript, and renders up to 3 ranked 9:16 clips
            with burned-in captions.
          </p>
          <div class="actions">
            <a routerLink="/shorts" class="ui-btn ui-btn--primary">Upload a video</a>
            <a href="#how" class="ui-btn ui-btn--secondary">See how it works</a>
          </div>
          <div>
            <p class="fine">MP4, MOV, MP3 or WAV · 30 seconds to 1 hour · 2 GB max</p>
            <p class="fine">
              Free during beta, no credit card.
              <a routerLink="/terms" class="inline">Beta terms</a>
            </p>
          </div>
        </div>
      </section>

      <main>
        <!-- Product proof: the real upload screen plus the real caption presets -->
        <section class="block first" aria-label="The upload screen and caption presets">
          <div class="wrap">
            <div appReveal class="reveal proof flush">
              <img
                src="/screenshots/shorts-hero-still.png"
                alt="The Indifferent AI Shorts upload screen showing the file format and duration limits"
                loading="lazy"
                width="480"
                height="350"
              />
              <div>
                <p class="label">The upload screen</p>
                <p class="text">Drop a file and the limits are stated up front: MP4, MOV, MP3 or WAV, 1 hour, 2 GB.</p>
                <p class="label">Pipeline</p>
                <p class="text">Transcribe, rank the moments, center the speaker, burn in captions, download.</p>
              </div>
            </div>

            <div appReveal class="reveal gap-top">
              <h2>Caption presets</h2>
              <p class="body">
                Every clip is captioned word by word from the transcript. The gallery has these four presets. Bold is the default, and a preset is used when a job's clips render, so a finished job keeps the look it was rendered with.
              </p>
              <ul class="captions">
                @for (p of captionPreviews; track p.id) {
                  <li>
                    <figure>
                      <div class="frame hm-on-dark">
                        <div class="cap">
                          <span [style.color]="p.color" [style.font-size]="p.fontSize" [style.font-weight]="p.weight" [style.text-shadow]="p.shadow" [style.background]="p.background" [style.padding]="p.padding" [style.border-radius]="p.radius">{{ p.sample }}</span>
                        </div>
                      </div>
                      <figcaption>
                        <span class="name">{{ p.label }}</span>
                        <span class="note">{{ p.blurb }}</span>
                      </figcaption>
                    </figure>
                  </li>
                }
              </ul>
              <p class="fine">Frames show each caption style over a still. Face-centered framing and blur-fill are applied when the clip renders.</p>
            </div>
          </div>
        </section>

        <!-- Features -->
        <section id="features" class="block tint" aria-labelledby="features-heading">
          <div class="wrap">
            <h2 id="features-heading" appReveal class="reveal">What the pipeline does to your file. Five things it handles for you.</h2>
            <ul class="items">
              @for (f of features; track f.title) {
                <li appReveal class="reveal">
                  <span class="icon"><svg lucideIcon="{{ f.icon }}" [size]="22"></svg></span>
                  <div>
                    <h3>{{ f.title }}</h3>
                    <p>{{ f.text }}</p>
                  </div>
                </li>
              }
            </ul>
          </div>
        </section>

        <!-- How it works -->
        <section id="how" class="block" aria-labelledby="how-heading">
          <div class="wrap">
            <h2 id="how-heading" appReveal class="reveal">Turn long videos into shorts. Simple to use, built for speed.</h2>
            <ol class="steps">
              <li appReveal class="reveal">
                <p class="step">Step 1</p>
                <h3>Upload a video or audio file</h3>
                <p>MP4, MOV, MP3 or WAV of 30 seconds to 1 hour and up to 2 GB. Podcasts, interviews or phone footage.</p>
              </li>
              <li appReveal class="reveal">
                <p class="step">Step 2</p>
                <h3>Let AI edit for you</h3>
                <p>AI transcribes the file, picks the strongest moments from the transcript, centers the 9:16 window on the speaker, and burns in short caption lines.</p>
              </li>
              <li appReveal class="reveal">
                <p class="step">Step 3</p>
                <h3>Preview and download</h3>
                <p>Ranked clips land in your gallery with previews. Download full-HD 9:16 MP4s, ready for Shorts, Reels and TikTok.</p>
              </li>
            </ol>
            <div appReveal class="reveal actions">
              <a routerLink="/shorts" class="ui-btn ui-btn--secondary">Upload your first video</a>
            </div>
          </div>
        </section>

        <!-- Use cases -->
        <section id="use-cases" class="block tint" aria-labelledby="use-cases-heading">
          <div class="wrap">
            <h2 id="use-cases-heading" appReveal class="reveal">Six ways to use ranked clips.</h2>
            <ul class="rows">
              @for (u of useCases; track u.slug) {
                <li appReveal class="reveal">
                  <a [routerLink]="['/use-cases', u.slug]" class="row-link">
                    <span>
                      <span class="row-title">{{ u.title }}</span>
                      <span class="row-text">{{ u.text }}</span>
                    </span>
                    <span class="row-cta">Learn more</span>
                  </a>
                </li>
              }
            </ul>
          </div>
        </section>

        <!-- Facts -->
        <section class="block" aria-label="Output at a glance">
          <div class="wrap">
            <dl appReveal class="reveal facts">
              <div><dt>ranked clips per upload</dt><dd>Up to 3</dd></div>
              <div><dt>full-HD vertical export</dt><dd>1080×1920</dd></div>
            </dl>
          </div>
        </section>

        <!-- FAQ -->
        <section id="faq" class="block tint" aria-labelledby="faq-heading">
          <div class="wrap">
            <h2 id="faq-heading" appReveal class="reveal">Before you upload.</h2>
            <div class="faq-list">
              @for (f of faqs; track f.q) {
                <details class="faq ui-disclosure">
                  <summary>{{ f.q }}</summary>
                  <p>{{ f.a }}</p>
                </details>
              }
            </div>
          </div>
        </section>

        <!-- Final CTA -->
        <section class="block closing" aria-labelledby="closing-heading">
          <div class="wrap">
            <h2 id="closing-heading" appReveal class="reveal">Start creating shorts from your long videos.</h2>
            <div appReveal class="reveal actions">
              <a routerLink="/shorts" class="ui-btn ui-btn--primary">Turn your next video into shorts</a>
            </div>
            <nav class="more" aria-label="More from the studio">
              <a routerLink="/projects" class="inline">TXT to video</a>
              <a routerLink="/tools" class="inline">Free tools</a>
              <a routerLink="/tools/add-watermark" class="inline">Watermark tool</a>
              <a routerLink="/blog/quiz-videos-youtube" class="inline">Quiz videos for YouTube</a>
              <a routerLink="/blog/ai-narration-guide" class="inline">AI narration guide</a>
              <a routerLink="/blog/educational-video-best-practices" class="inline">Educational video tips</a>
            </nav>
          </div>
        </section>
      </main>
      <app-site-footer />
    </div>
  `,
})
export class ShortsMakerComponent implements OnInit {
  private readonly title = inject(Title);
  private readonly meta = inject(Meta);
  private readonly document = inject(DOCUMENT);

  currentYear = new Date().getFullYear();

  /**
   * The four caption presets the shorts editor actually renders, with the
   * styling the product applies to each. Kept in step with the editor
   * presets so the page shows the real output style, not a mock-up.
   */
  captionPreviews: {
    id: string;
    label: string;
    blurb: string;
    sample: string;
    color: string;
    fontSize: string;
    weight: string;
    shadow: string;
    background: string;
    padding: string;
    radius: string;
  }[] = [
    {
      id: 'minimal',
      label: 'Minimal',
      blurb: 'Small, subtle, no box',
      sample: 'sample caption',
      color: '#FFFFFF',
      fontSize: '13px',
      weight: '500',
      shadow: '0 1px 3px rgba(0,0,0,0.8)',
      background: 'transparent',
      padding: '0',
      radius: '0',
    },
    {
      id: 'bold',
      label: 'Bold',
      blurb: 'Large white on dark box',
      sample: 'Sample caption',
      color: '#FFFFFF',
      fontSize: '19px',
      weight: '800',
      shadow: 'none',
      background: 'rgba(0,0,0,0.55)',
      padding: '2px 8px',
      radius: '6px',
    },
    {
      id: 'karaoke',
      label: 'Karaoke Highlight',
      blurb: 'Word-by-word sweep',
      sample: 'Sample',
      color: '#FFD60A',
      fontSize: '18px',
      weight: '800',
      shadow: '0 2px 4px rgba(0,0,0,0.9)',
      background: 'transparent',
      padding: '0',
      radius: '0',
    },
    {
      id: 'clean',
      label: 'Clean Subtitles',
      blurb: 'Classic outlined text',
      sample: 'Sample caption',
      color: '#FFFFFF',
      fontSize: '16px',
      weight: '600',
      shadow: '-1px 0 0 #000, 1px 0 0 #000, 0 -1px 0 #000, 0 1px 0 #000',
      background: 'transparent',
      padding: '0',
      radius: '0',
    },
  ];


  features: Feature[] = [
    { icon: 'scissors', title: 'A language model picks the moments', text: 'The transcript is cut into 10 second blocks, and a language model on Amazon Bedrock picks up to 3 moments of 30 to 90 seconds, each with a score. You review, not scrub timelines.' },
    { icon: 'focus', title: 'Face detection frames the speaker', text: 'Faces are found in up to 8 frames of each clip, and the 9:16 window is centered on the largest one instead of blindly cropping the middle. The window stays in one place for the whole clip.' },
    { icon: 'frame', title: 'Blur-fill fit never cuts content', text: 'Cartoons, text cards and screen recordings scale to fit over a blurred background. Nothing gets chopped.' },
    { icon: 'gauge', title: 'One output format', text: '1080×1920 MP4, H.264 video, AAC audio at 48kHz, 30fps, a format YouTube Shorts, Reels and TikTok accept.' },
    { icon: 'download', title: 'Upload, wait, download', text: 'Transcribe, score, render, then download the clips one by one or as a zip.' },
  ];

  useCases: UseCase[] = [
    { slug: 'content-creators', title: 'Content Creators', text: 'Repurpose podcasts, interviews and videos into ranked vertical shorts.' },
    { slug: 'marketing-teams', title: 'Marketing Teams', text: 'Turn webinars and launches into a batch of vertical clips without adding editing headcount.' },
    { slug: 'agencies', title: 'Agencies', text: 'Produce on-brand client content with consistent quality, fast.' },
    { slug: 'coaches', title: 'Coaches', text: 'Turn long lessons and calls into clips you can post the same day.' },
    { slug: 'media-companies', title: 'Media Companies', text: 'Turn long-form content into vertical clips at volume.' },
    { slug: 'educators', title: 'Educators', text: 'Clip the key explanation from every lecture, automatically.' },
  ];

  useCaseLinks: UseCaseLink[] = [
    { slug: 'content-creators', title: 'Content Creators' },
    { slug: 'marketing-teams', title: 'Marketing Teams' },
    { slug: 'agencies', title: 'Agencies' },
    { slug: 'coaches', title: 'Coaches' },
    { slug: 'media-companies', title: 'Media Companies' },
    { slug: 'educators', title: 'Educators' },
  ];

  faqs: Faq[] = [
    { q: 'How do I create a short?', a: 'Upload a video or audio file. It gets transcribed, a language model picks the strongest moments from the transcript, and the top ones are captioned and rendered as ranked 9:16 clips, usually within minutes for a short file.' },
    { q: 'Can I turn a long video into multiple shorts?', a: 'Yes. Every upload produces up to 3 ranked clips from the strongest moments in the transcript, each downloadable as its own 9:16 MP4.' },
    { q: 'Are captions added automatically?', a: 'Yes. Word-level timestamps become short burned-in caption lines styled for readability without sound.' },
    { q: 'What formats and quality do I get?', a: 'MP4 with H.264 video and AAC audio at 1080×1920 and 30fps, a standard format for YouTube Shorts, Instagram Reels and TikTok.' },
    { q: 'What if my video has no people in it?', a: 'When no face is found in a clip, it renders in blur-fill fit: the whole frame stays visible over a blurred background, so text and diagrams are never cut.' },
    { q: 'Which files can I upload?', a: 'MP4, MOV, MP3 and WAV of 30 seconds to 1 hour and up to 2 GB. A file shorter than 30 seconds is refused before upload, because each clip is at least 30 seconds.' },
    { q: 'How are the best moments picked?', a: 'Your file is transcribed, and the transcript is sent to a language model in 10 second blocks. The model is asked for up to 3 self-contained moments of 30 to 90 seconds with a strong hook or a complete thought, and returns a score for each. The clips are rendered in score order, and the score shows in the gallery. The model reads text only, so it cannot see the picture or hear tone of voice.' },
    { q: 'What happens if no segment scores well?', a: 'You still get clips, up to 3. If the source is mostly silence or a static frame, the chosen moments will not be compelling, and the tool cannot fix that. Trim the file to the interesting part and try again. A file with no speech at all fails with a no speech detected message, and one with no usable moment of 30 to 90 seconds fails with a message that says so.' },
    { q: 'Is my source file uploaded and kept?', a: 'Your file is uploaded to private storage so it can be transcribed and rendered. It stays in your account until you delete the job, and uploaded sources move to archive storage after 30 days. Deleting a Short removes its files and its record, so download your clips first.' },
    { q: 'How long does processing take?', a: 'Most files finish in a few minutes, and a one-hour file takes longer than a short one. The progress page shows the live stage you are on, so you can leave it open while it works.' },
    { q: 'Can I change the captions after the clips are made?', a: 'No. The captions are burned into the video, and a finished job is not rendered again. Animated captions and the other effects can be changed in the step between upload and processing, and on a failed job before you press Retry. To try a different look on a finished job, upload the file again and set the effects first.' },
    { q: 'Where are the clips stored, and what are they called?', a: 'Each job keeps its clips in the Shorts gallery, ranked by score, so the first clip is the one the model scored highest. You can download them one by one or grab the selected ones as a zip.' },
    { q: 'Can I publish straight to TikTok or Reels?', a: 'Not yet. You download the 9:16 MP4s and upload them to the platform yourself. The clips are exported at 1080x1920, 30fps, H.264 with AAC, which is the format Shorts and Reels expect.' },
  ];

  ngOnInit(): void {
    const pageUrl = 'https://indifferent.fun/shorts-maker';
    const description = 'Upload a video or audio file and let AI transcribe, rank and render top 9:16 shorts with burned-in captions. Free during beta.';
    this.title.setTitle('AI Shorts Maker: Long Video to Ranked Clips | Indifferent');
    this.meta.updateTag({ name: 'description', content: description });
    this.meta.updateTag({ property: 'og:type', content: 'website' });
    this.meta.updateTag({ property: 'og:title', content: 'AI Shorts Maker: Long Video to Ranked Clips | Indifferent' });
    this.meta.updateTag({ property: 'og:description', content: description });
    this.meta.updateTag({ property: 'og:url', content: pageUrl });
    setCanonical(this.document, pageUrl);
    this.meta.updateTag({ name: 'twitter:card', content: 'summary' });
    this.meta.updateTag({ name: 'twitter:title', content: 'AI Shorts Maker: Long Video to Ranked Clips | Indifferent' });
    this.meta.updateTag({ name: 'twitter:description', content: description });
    this.addJsonLd({
      '@context': 'https://schema.org',
      '@type': 'SoftwareApplication',
      name: 'Indifferent AI Shorts Maker',
      url: pageUrl,
      applicationCategory: 'MultimediaApplication',
      operatingSystem: 'Web',
      description,
      offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD', description: 'Free during beta' },
    });
    this.addJsonLd({
      '@context': 'https://schema.org',
      '@type': 'FAQPage',
      mainEntity: this.faqs.map((f) => ({ '@type': 'Question', name: f.q, acceptedAnswer: { '@type': 'Answer', text: f.a } })),
    });
  }

  private addJsonLd(schema: Record<string, unknown>): void {
    // One block per schema type, so revisiting the page never stacks duplicates.
    setJsonLd(this.document, String(schema['@type']).toLowerCase(), schema);
  }

}
