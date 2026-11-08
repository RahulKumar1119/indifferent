import { Component, OnInit, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { RouterLink } from '@angular/router';
import { PublicHeaderComponent } from '../../shared/components/public-header/public-header.component';
import { SiteFooterComponent } from '../../shared/components/site-footer/site-footer.component';
import { setPageSeo } from '../../shared/seo';
import { TEMPLATE_OPTIONS } from '../../shared/models';
import { PRODUCT_FACTS, durationLabel, joinAnd } from '../../core/config/product-facts';

@Component({
  selector: 'app-about',
  standalone: true,
  imports: [RouterLink, PublicHeaderComponent, SiteFooterComponent],
  styles: [`
    :host { display: block; }
    .page { background: var(--color-paper); color: var(--color-ink); font-family: var(--font-body); font-size: var(--type-md); line-height: 1.55; -webkit-font-smoothing: antialiased; }
    .wrap { width: min(var(--container), 100%); margin-inline: auto; padding-inline: var(--gutter); }
    h1, h2, h3, p, ul, ol { margin: 0; }
    h1 { font-family: var(--font-display); font-weight: 500; font-size: var(--type-display); line-height: 1.02; letter-spacing: var(--tracking-display); overflow-wrap: break-word; min-width: 0; text-wrap: balance; max-width: 14ch; }
    h2 { font-family: var(--font-display); font-weight: 500; font-size: var(--type-section); line-height: 1.04; letter-spacing: var(--tracking-display); overflow-wrap: break-word; min-width: 0; text-wrap: balance; }
    h3 { font-size: var(--type-md); font-weight: 600; }
    .lead { margin-top: var(--space-lg); font-size: var(--type-lead); line-height: 1.45; color: var(--color-ink-2); max-width: 52ch; }
    .body { color: var(--color-ink-2); max-width: 60ch; margin-top: var(--space-md); }
    .muted { color: var(--color-muted); }
    a.inline { color: var(--color-accent-text); text-decoration: underline; text-underline-offset: 0.2em; }
    a.inline:hover { color: var(--color-ink); }
    a:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: 2px; }

    .hero { background: var(--color-paper); color: var(--color-ink); padding: calc(56px + var(--space-3xl)) 0 var(--space-3xl); }
    .block { padding: var(--space-3xl) 0; border-top: 1px solid var(--color-rule); }
    .block.first { border-top: 0; }
    .block.tint { background: var(--color-paper-2); }
    .split { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-2xl); align-items: start; }
    @media (min-width: 64rem) { .split { grid-template-columns: minmax(0, 0.9fr) minmax(0, 1.1fr); } }
    .actions { display: flex; flex-wrap: wrap; gap: var(--space-md); margin-top: var(--space-xl); }

    ul.rows, ol.rows { list-style: none; padding: 0; margin-top: var(--space-xl); border-top: 1px solid var(--color-rule); }
    ul.rows > li, ol.rows > li { padding: var(--space-lg) 0; border-bottom: 1px solid var(--color-rule); }
    ul.rows h3 a { display: inline-flex; align-items: center; min-height: 44px; color: var(--color-ink); text-underline-offset: 0.2em; }
    ul.rows h3 a:hover { color: var(--color-accent-text); }
    ul.rows p { color: var(--color-ink-2); max-width: 60ch; }
    ol.rows.flush { margin-top: 0; }
    ol.rows > li { display: grid; grid-template-columns: 56px minmax(0, 1fr); gap: var(--space-lg); }
    .num { font-size: var(--type-xl); font-weight: 500; line-height: 1.1; color: var(--color-accent-text); font-variant-numeric: tabular-nums; }
    ol.rows p { margin-top: var(--space-2xs); color: var(--color-ink-2); }

    ul.ticks { list-style: none; padding: 0; margin-top: var(--space-lg); display: grid; gap: var(--space-sm); color: var(--color-ink-2); }
    ul.ticks li { padding-left: var(--space-lg); position: relative; }
    ul.ticks li::before { content: ''; position: absolute; left: 0; top: 0.7em; width: 10px; height: 2px; background: var(--color-accent); }
    .pair { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-2xl); }
    @media (min-width: 64rem) { .pair { grid-template-columns: repeat(2, minmax(0, 1fr)); } }

    ul.designs { list-style: none; padding: 0; margin-top: var(--space-xl); display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-lg); }
    @media (min-width: 64rem) { ul.designs { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
    ul.designs img { display: block; width: 100%; height: auto; border: 1px solid var(--color-rule); border-radius: var(--radius-control); }
    ul.designs h3 { margin-top: var(--space-sm); }
    ul.designs p { font-size: var(--type-sm); color: var(--color-muted); }

    ul.voices { list-style: none; padding: 0; border-top: 1px solid var(--color-rule); }
    ul.voices li { display: flex; align-items: center; gap: var(--space-md); padding: var(--space-md) 0; border-bottom: 1px solid var(--color-rule); }
    .initials { display: inline-flex; align-items: center; justify-content: center; width: 40px; height: 40px; flex-shrink: 0; border: 1px solid var(--color-muted); border-radius: var(--radius-control); font-size: var(--type-sm); font-weight: 600; }
    .name { display: block; font-weight: 600; }
    .voices small { display: block; font-size: var(--type-sm); color: var(--color-muted); }

    ul.uses { list-style: none; padding: 0; margin-top: var(--space-xl); display: grid; grid-template-columns: minmax(0, 1fr); border-top: 1px solid var(--color-rule); }
    @media (min-width: 40rem) { ul.uses { grid-template-columns: repeat(2, minmax(0, 1fr)); column-gap: var(--space-2xl); } }
    @media (min-width: 64rem) { ul.uses { grid-template-columns: repeat(5, minmax(0, 1fr)); column-gap: var(--space-lg); } }
    ul.uses li { padding: var(--space-md) 0; border-bottom: 1px solid var(--color-rule); }
    ul.uses small { display: block; margin-top: var(--space-3xs); font-size: var(--type-sm); color: var(--color-muted); }

    .stack { background: var(--color-paper); color: var(--color-ink); padding: var(--space-3xl) 0; }
    ul.stack-list { list-style: none; padding: 0; margin-top: var(--space-xl); display: flex; flex-wrap: wrap; gap: var(--space-xs); }
    ul.stack-list li { border: 1px solid var(--color-rule); border-radius: var(--radius-pill); padding: var(--space-xs) var(--space-md); font-size: var(--type-sm); }
    ul.stack-list small { color: var(--color-muted); margin-left: var(--space-xs); }
  `],
  template: `
    <div class="page hm hm-on-light">
      <app-public-header />

      <section class="hero hm-on-dark" aria-labelledby="about-title">
        <div class="wrap">
          <h1 id="about-title">The studio behind the video.</h1>
          <p class="lead">Indifferent is a small studio of four video tools: quiz films from a text file, vertical clips from long footage, narrated videos from a written story, and a watermark tool that runs in your browser. No editing skills required.</p>
        </div>
      </section>

      <main>
        <section class="block first">
          <div class="wrap">
            <h2>Our mission</h2>
            <p class="body">Indifferent makes it simple to turn a text file, a long recording or a written story into a finished video, without editing software or editing skills.</p>
            <div class="actions">
              <a routerLink="/login" class="ui-btn ui-btn--primary">Start creating</a>
            </div>
          </div>
        </section>

        <section class="block tint" aria-labelledby="tools-heading">
          <div class="wrap">
            <h2 id="tools-heading">Four tools, one studio</h2>
            <p class="body">They share one account. Each has its own page with the exact limits and a step-by-step guide.</p>
            <ul class="rows">
              @for (t of tools; track t.href) {
                <li>
                  <h3><a [routerLink]="t.href">{{ t.title }}</a></h3>
                  <p>{{ t.text }}</p>
                </li>
              }
            </ul>
          </div>
        </section>

        <section class="block">
          <div class="wrap split">
            <h2>How a quiz film is made</h2>
            <ol class="rows flush">
              <li><span class="num">01</span><div><h3>Upload your quiz file</h3><p>Plain text (.txt) with multiple-choice questions. Numbered, bulleted, and tab-indented formats supported.</p></div></li>
              <li><span class="num">02</span><div><h3>Choose design &amp; voice</h3><p>One of six slide designs and one of 5 professional AI voices powered by Amazon Polly.</p></div></li>
              <li><span class="num">03</span><div><h3>Automatic processing</h3><p>Our serverless pipeline parses questions, builds question and answer slides, creates narration, and renders the video.</p></div></li>
              <li><span class="num">04</span><div><h3>Download &amp; share</h3><p>Preview in-browser and download the MP4, ready for YouTube, social, or any platform.</p></div></li>
            </ol>
          </div>
        </section>

        <section class="block tint">
          <div class="wrap pair">
            <div>
              <h2>Quiz file formats</h2>
              <ul class="ticks">
                <li>Numbered questions (1. Question text)</li>
                <li>Bulleted questions (• or - Question text)</li>
                <li>Tab-indented answers</li>
                <li>Multiple correct answers supported</li>
                <li>Maximum file size: 5MB</li>
              </ul>
            </div>
            <div>
              <h2>Quiz film specifications</h2>
              <ul class="ticks">
                <li>Resolution: 1920×1080 (Full HD)</li>
                <li>Format: MP4 (H.264), Audio: AAC</li>
                <li>Question slides, answer reveals, AI narration</li>
                <li>Automatic thumbnail generation</li>
              </ul>
            </div>
          </div>
        </section>

        <section class="block">
          <div class="wrap">
            <h2>Six designs, six looks</h2>
            <p class="body">A design sets the palette, type scale and layout together: the background, the text, the shape of the answer cards and the highlight on the correct answer. Timing is the same in all six. Every image below is a question slide drawn by the same renderer that makes your video.</p>
            <ul class="designs">
              @for (t of designs; track t.value) {
                <li>
                  <img [src]="'/templates/' + t.value + '.webp'" [alt]="t.label + ' design preview: ' + t.look" width="480" height="270" loading="lazy" />
                  <h3>{{ t.label }}</h3>
                  <p>{{ t.look }}</p>
                </li>
              }
            </ul>
          </div>
        </section>

        <section class="block tint">
          <div class="wrap split">
            <div>
              <h2>Five narrator voices</h2>
              <p class="body">Professional text-to-speech via Amazon Polly. Pick the register that fits your audience.</p>
            </div>
            <ul class="voices">
              @for (v of voices; track v.name) {
                <li>
                  <span class="initials" aria-hidden="true">{{ v.initials }}</span>
                  <span><span class="font-semibold name">{{ v.name }}</span><small>{{ v.desc }}</small></span>
                </li>
              }
            </ul>
          </div>
        </section>

        <section class="block">
          <div class="wrap">
            <h2>Who uses it</h2>
            <ul class="uses">
              @for (u of uses; track u.title) {
                <li>
                  <strong>{{ u.title }}</strong>
                  <small>{{ u.desc }}</small>
                </li>
              }
            </ul>
          </div>
        </section>

        <section class="stack hm-on-dark">
          <div class="wrap">
            <h2>Built with</h2>
            <ul class="stack-list">
              @for (s of stack; track s) {
                <li>{{ s.split(' · ')[0] }}<small>{{ s.split(' · ')[1] }}</small></li>
              }
            </ul>
            <p class="body">Indifferent is built and maintained by a developer focused on making content creation accessible. No expensive software, no editing skills.</p>
            <div class="actions">
              <a href="https://github.com/RahulKumar1119/indifferent/issues" target="_blank" rel="noopener" class="ui-btn ui-btn--secondary">Open an issue</a>
              <a routerLink="/contact" class="ui-btn ui-btn--quiet">Get in touch</a>
            </div>
          </div>
        </section>
      </main>
      <app-site-footer />
    </div>
  `,
})
export class AboutComponent implements OnInit {
  private readonly document = inject(DOCUMENT);

  ngOnInit(): void {
    setPageSeo(this.document, {
      title: 'About the Studio | Indifferent',
      description:
        'An AI video studio: quiz films from TXT files, vertical Shorts from long footage, narrated videos from a story, and a watermark tool that runs in your browser.',
      canonical: 'https://indifferent.fun/about',
    });
  }

  currentYear = new Date().getFullYear();
  readonly designs = TEMPLATE_OPTIONS;
  readonly tools = [
    {
      title: 'TXT to quiz video',
      href: '/features/txt-to-video-quiz',
      text: `Upload a text file of multiple-choice questions and get a narrated ${PRODUCT_FACTS.quiz.outputLabel} with answer reveals. ${PRODUCT_FACTS.quiz.voices.length} voices and ${PRODUCT_FACTS.quiz.designs.length} designs to choose from.`,
    },
    {
      title: 'AI Shorts generator',
      href: '/features/ai-shorts',
      text: `Upload a video or audio file of up to ${durationLabel(PRODUCT_FACTS.shorts.maxDurationMinutes)} and get up to ${PRODUCT_FACTS.shorts.maxClips} ranked vertical clips with captions burned in.`,
    },
    {
      title: 'Story to video',
      href: '/features/story-video',
      text: `Paste a story and a model plans ${PRODUCT_FACTS.story.minScenes} to ${PRODUCT_FACTS.story.maxScenes} scenes. You confirm a stock photo or clip from ${joinAnd(PRODUCT_FACTS.story.stockLibraries)} for each, and it becomes a narrated vertical video of up to ${PRODUCT_FACTS.story.maxTotalSeconds} seconds.`,
    },
    {
      title: 'Watermark tool',
      href: '/features/watermark',
      text: 'Stamp a text or logo watermark on an image or a video. It runs in your browser, so nothing is uploaded.',
    },
  ];
  voices = [
    { name: 'Joanna', desc: 'US English, female, clear and professional', initials: 'JO' },
    { name: 'Matthew', desc: 'US English, male, warm and authoritative', initials: 'MA' },
    { name: 'Amy', desc: 'British English, female, polished', initials: 'AM' },
    { name: 'Brian', desc: 'British English, male, natural', initials: 'BR' },
    { name: 'Aditi', desc: 'Indian English, female, approachable', initials: 'AD' },
  ];
  uses = [
    { title: 'Teachers', desc: 'Revision videos for students' },
    { title: 'YouTube creators', desc: 'Scale quiz and trivia output' },
    { title: 'E-learning', desc: 'Automate course video creation' },
    { title: 'Corporate trainers', desc: 'Compliance quizzes as video' },
    { title: 'Students', desc: 'More engaging study material' },
  ];
  stack = ['Angular 20 · Frontend', 'Go · Backend', 'AWS Lambda · Compute', 'Amazon Polly · Narration', 'Amazon Bedrock · Planning and ranking', 'FFmpeg · Video', 'DynamoDB · Database', 'Amplify · CDN', 'Step Functions · Orchestration', 'GSAP · Animation'];
}
