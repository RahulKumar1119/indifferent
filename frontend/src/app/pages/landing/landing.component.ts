import { Component, OnInit, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { RouterLink } from '@angular/router';
import { PublicHeaderComponent } from '../../shared/components/public-header/public-header.component';
import { RevealDirective } from '../../shared/directives/reveal.directive';
import { setJsonLd, setPageSeo } from '../../shared/seo';
import { FlipbookComponent } from './flipbook.component';
import { HeroSceneComponent } from './hero-scene.component';

@Component({
  selector: 'app-landing',
  standalone: true,
  imports: [RouterLink, PublicHeaderComponent, RevealDirective, FlipbookComponent, HeroSceneComponent],
  styles: [`
    :host { display: block; }
    .page { background: var(--color-paper); color: var(--color-ink); font-family: var(--font-body); font-size: var(--type-md); line-height: 1.55; -webkit-font-smoothing: antialiased; }
    .wrap { width: min(var(--container), 100%); margin-inline: auto; padding-inline: var(--gutter); }
    .serif { font-family: var(--font-display); font-weight: 500; letter-spacing: var(--tracking-display); line-height: 1.04; overflow-wrap: break-word; min-width: 0; text-wrap: balance; margin: 0; }
    h1.serif { font-size: var(--type-display); max-width: 14ch; }
    h2.serif { font-size: var(--type-section); max-width: 22ch; }
    h3.serif { font-size: var(--type-title); line-height: 1.2; }
    .lead { font-size: var(--type-lead); line-height: 1.45; color: var(--color-ink-2); max-width: 46ch; margin: var(--space-lg) 0 0; }
    p { margin: 0; }
    .muted { color: var(--color-muted); }
    .body { color: var(--color-ink-2); max-width: 56ch; margin-top: var(--space-md); }

    /* Hero: a solid dark surface, the real slide on it, nothing else competing */
    .hero { background: var(--color-paper); color: var(--color-ink); padding: calc(56px + var(--space-3xl)) 0 var(--space-2xl); }
    .hero-grid { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-2xl); align-items: center; }
    @media (min-width: 64rem) { .hero-grid { grid-template-columns: minmax(0, 1.05fr) minmax(0, 0.95fr); } }
    .actions { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-md); margin-top: var(--space-xl); }
    .hero-visual { width: 100%; max-width: 540px; justify-self: end; }
    .hero-visual :is(p) { color: var(--color-muted); }

    /* Facts row under the hero: what the flagship tool does, in figures */
    .facts { background: var(--color-paper-2); color: var(--color-ink); border-top: 1px solid var(--color-rule); }
    .facts dl { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-lg); margin: 0; padding: var(--space-xl) 0; }
    @media (min-width: 48rem) { .facts dl { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
    .facts dt { font-size: var(--type-sm); color: var(--color-muted); }
    .facts dd { margin: var(--space-3xs) 0 0; font-family: var(--font-display); font-size: var(--type-2xl); font-weight: 500; font-variant-numeric: tabular-nums; }

    .section { padding: var(--space-3xl) 0; }
    .section + .section { border-top: 1px solid var(--color-rule); }
    .section.tint { background: var(--color-paper-2); }
    .split { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-2xl); align-items: start; }
    @media (min-width: 64rem) { .split { grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr); } }

    ul.ticks { list-style: none; margin: var(--space-lg) 0 0; padding: 0; display: grid; gap: var(--space-sm); color: var(--color-ink-2); }
    ul.ticks li { padding-left: var(--space-lg); position: relative; }
    ul.ticks li::before { content: ''; position: absolute; left: 0; top: 0.7em; width: 10px; height: 2px; background: var(--color-accent); }
    dl.spec { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-md); margin: var(--space-xl) 0 0; padding-top: var(--space-lg); border-top: 1px solid var(--color-rule); }
    dl.spec dt { font-size: var(--type-sm); color: var(--color-muted); }
    dl.spec dd { margin: var(--space-3xs) 0 0; font-weight: 600; }

    .text-link { display: inline-flex; align-items: center; min-height: 44px; margin-top: var(--space-md); font-weight: 600; color: var(--color-accent-text); text-underline-offset: 0.2em; }
    .text-link:hover { color: var(--color-ink); }

    .code { background: var(--color-paper-3); border: 1px solid var(--color-rule); border-radius: var(--radius-surface); padding: var(--space-lg); }
    .code pre { margin: 0; overflow-x: auto; font-size: var(--type-sm); line-height: 1.7; color: var(--color-ink); }

    .rows { list-style: none; margin: var(--space-xl) 0 0; padding: 0; border-top: 1px solid var(--color-rule); }
    .rows > li { border-bottom: 1px solid var(--color-rule); }
    .row-link { display: flex; align-items: center; justify-content: space-between; gap: var(--space-lg); padding: var(--space-lg) var(--space-xs); color: inherit; text-decoration: none; transition: background-color var(--dur-short) var(--ease-out); }
    .row-link:hover { background: var(--color-paper-2); }
    .row-link:focus-visible { outline: 2px solid var(--color-focus); outline-offset: -2px; }
    .row-title { display: block; font-family: var(--font-display); font-size: var(--type-2xl); line-height: 1.15; font-weight: 500; }
    .row-text { display: block; margin-top: var(--space-2xs); color: var(--color-muted); max-width: 56ch; }
    .row-cta { color: var(--color-accent-text); font-weight: 600; flex-shrink: 0; }

    ol.steps { list-style: none; margin: var(--space-xl) 0 0; padding: 0; }
    ol.steps > li { display: grid; grid-template-columns: 56px minmax(0, 1fr); gap: var(--space-lg); padding: var(--space-lg) 0; border-top: 1px solid var(--color-rule); }
    ol.steps > li:last-child { border-bottom: 1px solid var(--color-rule); }
    .step-n { font-family: var(--font-display); font-size: var(--type-2xl); color: var(--color-accent-text); line-height: 1; font-variant-numeric: tabular-nums; }
    .step-title { margin: 0; font-size: var(--type-lg); font-weight: 600; }

    .faq-list { display: grid; gap: var(--space-sm); margin-top: var(--space-xl); }
    details.faq { background: var(--color-paper-2); border: 1px solid var(--color-rule); border-radius: var(--radius-surface); padding: 0 var(--space-lg); }
    details.faq summary { display: flex; align-items: center; justify-content: space-between; gap: var(--space-md); min-height: 56px; cursor: pointer; list-style: none; font-weight: 600; }
    details.faq summary::-webkit-details-marker { display: none; }
    details.faq summary:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: var(--radius-control); }
    details.faq .faq-chevron { flex-shrink: 0; color: var(--color-muted); transition: transform var(--dur-short) var(--ease-out); }
    details.faq[open] .faq-chevron { transform: rotate(180deg); }
    details.faq p { padding: 0 0 var(--space-lg); color: var(--color-ink-2); }

    /* Closing block */
    .closing { background: var(--color-paper); color: var(--color-ink); padding: var(--space-3xl) 0 var(--space-xl); }
    .closing a { color: inherit; }
    .closing .ui-btn--primary { color: var(--color-accent-ink); }
    .pair { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-2xl); }
    @media (min-width: 64rem) { .pair { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
    .wordmark { margin: var(--space-3xl) 0 0; text-align: center; font-family: var(--font-display); font-weight: 500; line-height: 1; letter-spacing: -0.03em; font-size: clamp(2.5rem, 12vw, 9rem); color: var(--color-ink-2); user-select: none; overflow-wrap: anywhere; }
    .foot-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-xl); margin-top: var(--space-2xl); padding-top: var(--space-xl); border-top: 1px solid var(--color-rule); }
    @media (min-width: 48rem) { .foot-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
    .foot-title { margin: 0 0 var(--space-xs); font-size: var(--type-sm); color: var(--color-muted); }
    .foot-grid ul { list-style: none; margin: 0; padding: 0; }
    .foot-link { display: inline-flex; align-items: center; min-height: 44px; min-width: 44px; font-size: var(--type-sm); color: var(--color-ink-2); text-underline-offset: 0.2em; }
    .foot-link:hover { color: var(--color-ink); text-decoration: underline; }
    .legal { display: flex; flex-wrap: wrap; justify-content: space-between; gap: var(--space-sm); margin-top: var(--space-xl); padding-top: var(--space-lg); border-top: 1px solid var(--color-rule); font-size: var(--type-xs); color: var(--color-muted); }
    .closing a:focus-visible, .foot-link:focus-visible, .text-link:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: 2px; }
  `],
  template: `
    <div class="page hm hm-on-light">
      <app-public-header />

      <!-- Hero: the headline, one sentence, the actions, and the real output -->
      <section class="hero hm-on-dark" aria-labelledby="hero-title">
        <div class="wrap hero-grid">
          <div>
            <h1 id="hero-title" class="serif">Turn your stories into captivating videos.</h1>
            <p class="lead">Transform written stories into narrated videos with automatically generated scenes.</p>
            <div class="actions">
              <a routerLink="/story" class="ui-btn ui-btn--primary">Create my first video</a>
              <a href="#story" class="ui-btn ui-btn--secondary">See how it works</a>
            </div>
          </div>
          <div class="hero-visual">
            <app-hero-scene />
          </div>
        </div>
      </section>

      <!-- Facts: stated once, in figures -->
      <section class="facts" aria-label="Story to video at a glance">
        <div class="wrap">
          <dl>
            <div><dt>Scenes per story</dt><dd>2 to 10</dd></div>
            <div><dt>Video length</dt><dd>Up to 90 seconds</dd></div>
            <div><dt>Output</dt><dd>1080×1920 MP4</dd></div>
            <div><dt>Price</dt><dd>Free during beta</dd></div>
          </dl>
        </div>
      </section>

      <main>
        <!-- STORY: the flagship -->
        <section id="story" class="section scroll-mt-24" aria-labelledby="story-heading">
          <div class="wrap split reveal" appReveal>
            <div>
              <h2 id="story-heading" class="serif">A written story, turned into a narrated video.</h2>
              <p class="body">
                Paste a short story. A language model breaks it into scenes, writes the narration and searches for a
                matching photo or clip for each one. You review every scene, swap any picture, then render the video.
              </p>
              <a routerLink="/features/story-video" class="text-link">See how story to video works</a>
              <a href="#format" class="text-link" style="display:flex;margin-top:0">Making a quiz video instead? See the file format</a>
            </div>
            <div>
              <ul class="ticks" style="margin-top:0">
                <li>A story of 30 to 5000 characters, planned into 2 to 10 scenes</li>
                <li>Stock photos and clips from Pexels and Pixabay, yours to swap</li>
                <li>Narration by Amazon Polly in the story's own language</li>
                <li>You confirm every scene and picture before anything renders</li>
              </ul>
              <dl class="spec">
                <div><dt>Output</dt><dd>1080×1920 MP4</dd></div>
                <div><dt>Length</dt><dd>Up to 90 seconds</dd></div>
              </dl>
            </div>
          </div>
        </section>

        <!-- QUIZ: the plain text file -->
        <section id="format" class="section tint scroll-mt-24" aria-labelledby="format-heading">
          <div class="wrap split reveal" appReveal>
            <div>
              <h2 id="format-heading" class="serif">Quiz videos start as a plain text file.</h2>
              <p class="body">
                No special editor, no spreadsheet, no formatting to clean up. Number the questions, list the options, and mark the answer.
                Numbering, bulleted options and tab-indented answers are all detected.
              </p>
              <p class="body" style="margin-top:var(--space-xs)">
                The answer can be marked on its own line, or on the option itself with a trailing <span>*</span>.
              </p>
              <ul class="ticks">
                <li>Up to 5 MB, plain text only</li>
                <li>Multiple correct answers supported</li>
                <li>One question per slide, with an answer reveal</li>
              </ul>
              <div class="actions" style="margin-top:var(--space-lg)">
                <a routerLink="/features/txt-to-video-quiz" class="ui-btn ui-btn--secondary">See the quiz video feature</a>
                <a routerLink="/blog/quiz-file-format-guide" class="text-link" style="margin-top:0">Read the format guide</a>
              </div>
            </div>
            <div class="code">
              <pre><code>1. What is the capital of France?
A. London
B. Paris
C. Berlin
D. Madrid
Answer: B

2. Which planet is known as the Red Planet?
A. Venus
B. Jupiter
C. Mars
D. Saturn
Answer: C</code></pre>
            </div>
          </div>
        </section>

        <!-- THE REST OF THE STUDIO -->
        <section class="section" aria-labelledby="tools-heading">
          <div class="wrap reveal" appReveal>
            <h2 id="tools-heading" class="serif">Two more tools in the studio.</h2>
            <p class="body">
              One turns a long video into clips, and the other watermarks a file without ever touching a server.
            </p>
            <div class="split" style="margin-top:var(--space-2xl)">
              <article>
                <h3 class="serif">AI Shorts generator: a long video, cut into ranked vertical clips.</h3>
                <p class="body">
                  Drop in a podcast, an interview or a lecture. It is transcribed word by word, a language model picks
                  the strongest moments from the transcript, and the top ones are rendered as 9:16 clips with captions burned in.
                </p>
                <ul class="ticks">
                  <li>Video or audio in: MP4, MOV, MP3, WAV, up to 1 hour</li>
                  <li>Up to 3 clips per upload, 30 to 90 seconds each, ranked by score</li>
                  <li>Face detection centers the 9:16 window on the speaker</li>
                  <li>Blur-fill keeps text cards and screen recordings whole</li>
                  <li>Burned-in captions, with an optional animated style</li>
                </ul>
                <dl class="spec">
                  <div><dt>Output</dt><dd>1080×1920 MP4</dd></div>
                  <div><dt>Codec</dt><dd>H.264 + AAC, 30fps</dd></div>
                </dl>
                <a routerLink="/shorts-maker" class="ui-btn ui-btn--secondary" style="margin-top:var(--space-lg)">Open the Shorts generator</a>
              </article>
              <article>
                <h3 class="serif">Watermark tool: your mark on any image or video, in your browser.</h3>
                <p class="body">
                  Stamp a text or logo watermark, drag it where you want it, and export. The file never
                  leaves your device, so there is no upload and no account.
                </p>
                <ul class="ticks">
                  <li>Text in 10 typefaces, or your own logo image</li>
                  <li>Size, opacity, colour and rotation, all adjustable</li>
                  <li>Crop guides for 16:9, 9:16 and 1:1 on the preview</li>
                  <li>Images out at full resolution as PNG or JPEG</li>
                  <li>Video out as MP4, rendered on your own machine</li>
                </ul>
                <dl class="spec">
                  <div><dt>Accepts</dt><dd>JPG, PNG, WebP, MP4, WebM, MOV</dd></div>
                  <div><dt>Uploads</dt><dd>Nothing, ever</dd></div>
                </dl>
                <a routerLink="/tools/add-watermark" class="ui-btn ui-btn--secondary" style="margin-top:var(--space-lg)">Open the watermark tool</a>
              </article>
            </div>
          </div>
        </section>

        <!-- PROCESS -->
        <section id="process" class="section tint scroll-mt-24" aria-labelledby="process-heading">
          <div class="wrap split">
            <div class="reveal" appReveal style="order:2">
              <app-flipbook />
              <a routerLink="/blog/quiz-file-format-guide" class="text-link">Read the format guide</a>
            </div>
            <div class="reveal" appReveal>
              <h2 id="process-heading" class="serif">How a quiz video works.</h2>
              <p class="body">Three steps from a text file to a finished video.</p>
              <ol class="steps">
                @for (s of steps; track s.n) {
                  <li>
                    <span class="step-n">{{ s.n }}</span>
                    <div>
                      <h3 class="step-title">{{ s.title }}</h3>
                      <p class="body" style="margin-top:var(--space-2xs)">{{ s.body }}</p>
                    </div>
                  </li>
                }
              </ol>
            </div>
          </div>
        </section>

        <!-- WHO -->
        <section id="who" class="section scroll-mt-24" aria-labelledby="who-heading">
          <div class="wrap reveal" appReveal>
            <h2 id="who-heading" class="serif">Where a quiz video fits.</h2>
            <ul class="rows">
              @for (u of useCases; track u.title) {
                <li>
                  <a [routerLink]="u.href" class="row-link">
                    <span>
                      <span class="row-title">{{ u.title }}</span>
                      <span class="row-text">{{ u.text }}</span>
                    </span>
                    <span class="row-cta">{{ u.cta }}</span>
                  </a>
                </li>
              }
            </ul>
          </div>
        </section>
      </main>

      <!-- FAQ -->
      <section id="faq" class="section tint" aria-labelledby="faq-heading">
        <div class="wrap reveal" appReveal style="max-width:900px">
          <h2 id="faq-heading" class="serif">Before you upload.</h2>
          <div class="faq-list">
            @for (f of faqs; track f.q) {
              <details class="faq">
                <summary>
                  {{ f.q }}
                  <svg class="faq-chevron" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m6 9 6 6 6-6"/></svg>
                </summary>
                <p>{{ f.a }}</p>
              </details>
            }
          </div>
        </div>
      </section>

      <!-- Closing: journal, who runs this, footer -->
      <section class="closing hm-on-dark">
        <div class="wrap">
          <div class="pair">
            <div>
              <h2 class="serif">Read the journal.</h2>
              <p class="body">
                Guides to the tools: how AI clipping works, how to format a quiz file, how to watermark in your browser, and which video format to export.
                There is no email list. New articles show up in the journal and in an RSS feed.
              </p>
              <p class="actions">
                <a routerLink="/blog" class="ui-btn ui-btn--primary">Open the journal</a>
                <a href="/feed.xml" class="ui-btn ui-btn--secondary">RSS feed</a>
              </p>
            </div>
            <div>
              <h2 class="serif">Who runs this.</h2>
              <p class="body">
                Indifferent is built and maintained by one developer. It is in beta, free to use, and there is no sales team behind it.
                If something breaks, you can reach the person who wrote it.
              </p>
              <p class="body">
                <a href="mailto:support@indifferent.fun" style="text-underline-offset:0.2em">support&#64;indifferent.fun</a>
              </p>
            </div>
          </div>

          <footer>
            <p class="wordmark" aria-hidden="true">Indifferent</p>
            <div class="foot-grid">
              <div>
                <p class="foot-title">Product</p>
                <ul>
                  <li><a href="#process" class="foot-link">How it works</a></li>
                  <li><a routerLink="/blog" class="foot-link">Journal</a></li>
                  <li><a routerLink="/features/watermark" class="foot-link">Watermark tool</a></li>
                  <li><a routerLink="/features/ai-shorts" class="foot-link">AI Shorts generator</a></li>
                  <li><a routerLink="/features/story-video" class="foot-link">Story to video</a></li>
                  <li><a routerLink="/features/txt-to-video-quiz" class="foot-link">TXT to quiz video</a></li>
                </ul>
              </div>
              <div>
                <p class="foot-title">Studio</p>
                <ul>
                  <li><a routerLink="/about" class="foot-link">About</a></li>
                  <li><a routerLink="/contact" class="foot-link">Contact</a></li>
                  <li><a routerLink="/blog/quiz-file-format-guide" class="foot-link">Format guide</a></li>
                </ul>
              </div>
              <div>
                <p class="foot-title">Legal</p>
                <ul>
                  <li><a routerLink="/privacy" class="foot-link">Privacy</a></li>
                  <li><a routerLink="/terms" class="foot-link">Terms</a></li>
                </ul>
              </div>
              <div>
                <p class="foot-title">Start</p>
                <a routerLink="/login" class="ui-btn ui-btn--primary">Start creating</a>
                <p class="muted" style="margin-top:var(--space-md);font-size:var(--type-sm)">Stories, quizzes and clips in, video out.</p>
              </div>
            </div>
            <div class="legal">
              <span>© {{ currentYear }} Indifferent. All rights reserved.</span>
              <a href="mailto:support@indifferent.fun" style="text-underline-offset:0.2em">support&#64;indifferent.fun</a>
            </div>
          </footer>
        </div>
      </section>
    </div>
  `,
})
export class LandingComponent implements OnInit {
  private readonly document = inject(DOCUMENT);


  currentYear = new Date().getFullYear();

  faqs: { q: string; a: string }[] = [
    { q: 'What file do I need?', a: 'A plain text (.txt) file containing your multiple-choice questions. Number each question, list the options, and mark the answer. Numbered, bulleted and tab-indented formats are all detected, so you do not have to clean up your formatting first.' },
    { q: 'How large can the file be?', a: 'Up to 5 MB, which is enough for several hundred questions. The video length follows from how many questions you write.' },
    { q: 'Which voices can I use?', a: 'Five, all real Amazon Polly voices: Joanna (US English, clear and professional), Matthew (US English, warm and authoritative), Amy (British English, polished), Brian (British English, natural) and Aditi (Indian English, approachable).' },
    { q: 'How long does rendering take?', a: 'A short quiz usually renders in a few minutes, and a longer one takes longer. The project page shows the live stage, so you can leave it open and come back.' },
    { q: 'Is it free?', a: 'Yes, during the beta. No credit card, no quota to track, no paid tier to upgrade to yet. The beta terms say what that means.' },
    { q: 'Is my question file uploaded?', a: 'Yes, the text file has to reach the server to be parsed and narrated, and it is stored with your project so you can re-render it. The privacy page explains exactly what is kept and for how long. The watermark tool is the opposite: it never uploads anything.' },
    { q: 'Can I use the video commercially?', a: 'You keep ownership of the videos you generate. You are responsible for having the rights to the questions you upload, and the narration is produced by Amazon Polly, so the AWS terms apply to it too. The terms page covers the details.' },
    { q: 'What happens to my file afterwards?', a: 'Your project and its text file stay in your account so you can re-render with a different voice or design, and you can delete them at any time from the project page.' },
  ];

  ngOnInit(): void {
    const title = 'Indifferent: AI Shorts, Quiz Videos and Watermark Tool';
    const description =
      'Cut long videos into captioned vertical clips, turn quiz text files into narrated MP4s, and watermark images in your browser. Free during beta.';
    setPageSeo(this.document, { title, description, canonical: 'https://indifferent.fun/' });
    setJsonLd(this.document, 'webpage', {
      '@context': 'https://schema.org',
      '@type': 'WebPage',
      name: title,
      url: 'https://indifferent.fun/',
      description,
    });
    const free = { '@type': 'Offer', price: '0', priceCurrency: 'USD', description: 'Free during beta' };
    setJsonLd(this.document, 'software-quiz', {
      '@context': 'https://schema.org',
      '@type': 'SoftwareApplication',
      name: 'Indifferent Quiz Video Generator',
      url: 'https://indifferent.fun/features/txt-to-video-quiz',
      applicationCategory: 'MultimediaApplication',
      operatingSystem: 'Web',
      description: 'Turns a plain text file of multiple-choice questions into a narrated 1080p MP4 quiz video.',
      offers: free,
      featureList: ['TXT quiz file input', '5 narration voices', '1080p MP4 output', 'Question and answer slides'],
    });
    setJsonLd(this.document, 'software-shorts', {
      '@context': 'https://schema.org',
      '@type': 'SoftwareApplication',
      name: 'Indifferent AI Shorts Generator',
      url: 'https://indifferent.fun/features/ai-shorts',
      applicationCategory: 'MultimediaApplication',
      operatingSystem: 'Web',
      description: 'Turns a video or audio file of up to 1 hour into up to 3 captioned 9:16 clips.',
      offers: free,
    });
    setJsonLd(this.document, 'software-story', {
      '@context': 'https://schema.org',
      '@type': 'SoftwareApplication',
      name: 'Indifferent Story to Video Generator',
      url: 'https://indifferent.fun/features/story-video',
      applicationCategory: 'MultimediaApplication',
      operatingSystem: 'Web',
      description: 'Turns a written story into a narrated 1080x1920 video with matching stock footage.',
      offers: free,
    });
    setJsonLd(this.document, 'software-watermark', {
      '@context': 'https://schema.org',
      '@type': 'SoftwareApplication',
      name: 'Indifferent Watermark Tool',
      url: 'https://indifferent.fun/features/watermark',
      applicationCategory: 'MultimediaApplication',
      operatingSystem: 'Web',
      description: 'Adds text or logo watermarks to images and video entirely in the browser.',
      offers: free,
    });
    setJsonLd(this.document, 'faq', {
      '@context': 'https://schema.org',
      '@type': 'FAQPage',
      mainEntity: this.faqs.map((f) => ({
        '@type': 'Question',
        name: f.q,
        acceptedAnswer: { '@type': 'Answer', text: f.a },
      })),
    });
  }

  steps = [
    {
      n: '01',
      title: 'Upload',
      body: 'Drop a TXT file. We detect numbering, options, and answer keys automatically. No formatting cleanup.',
    },
    {
      n: '02',
      title: 'Choose a voice',
      body: 'Pick one of five AI narrators. Each question and each answer is read aloud in that voice.',
    },
    {
      n: '03',
      title: 'Render and download',
      body: 'We render the MP4 for you. Download it, watermark it if you like, and post it to YouTube.',
    },
  ];
  useCases = [
    { title: 'YouTube quiz shows', text: 'Faceless quiz channels: upload a file, publish the video.', href: '/blog/quiz-videos-youtube', cta: 'Read the guide' },
    { title: 'Lesson recaps', text: 'Turn revision questions into a video students can replay.', href: '/blog/educational-video-best-practices', cta: 'Read the guide' },
    { title: 'Trivia nights', text: 'Run a round straight from a text file of questions.', href: '/blog/quiz-file-format-guide', cta: 'See the format' },
    { title: 'Staff training', text: 'Check understanding with multiple-choice questions.', href: '/features/txt-to-video-quiz', cta: 'See the feature' },
  ];
}
