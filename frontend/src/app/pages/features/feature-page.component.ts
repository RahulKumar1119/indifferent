import { Component, DestroyRef, OnInit, inject } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { DOCUMENT } from '@angular/common';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { PRODUCT_FACTS, durationLabel, joinAnd, joinOr, shortsOutputLabel } from '../../core/config/product-facts';
import { SiteFooterComponent } from '../../shared/components/site-footer/site-footer.component';
import { ROBOTS_NOINDEX, setJsonLd, setPageSeo, setRobots } from '../../shared/seo';
import { ArticleMeta, articlePath, findArticle, formatLongDate } from '../blog/articles/registry';

export interface FeatureStep {
  title: string;
  text: string;
}

export interface FeatureLimit {
  label: string;
  value: string;
}

export interface FeatureFaq {
  q: string;
  a: string;
}

export interface FeatureData {
  slug: string;
  title: string;
  hook: string;
  description: string;
  bullets: string[];
  steps: FeatureStep[];
  cta: string;
  ctaLink: string;
  specs: string[];
  /** Exact limits and formats, read from PRODUCT_FACTS. */
  limits: FeatureLimit[];
  /** Where the user's files go and how they are removed. */
  handling: string;
  troubleshooting: FeatureFaq[];
  /** Slugs of journal articles that go deeper. */
  guides: string[];
  /** ISO date the facts on this page were last checked against the code. */
  reviewed: string;
}

const S = PRODUCT_FACTS.shorts;
const Q = PRODUCT_FACTS.quiz;
const W = PRODUCT_FACTS.watermark;

/** Shared template for product feature pages; content comes from FEATURE_DATA. */
export const FEATURE_DATA: Record<string, FeatureData> = {
  'ai-shorts': {
    slug: 'ai-shorts',
    title: 'AI Shorts Generator',
    hook: `Turn a long video or audio file into up to ${S.maxClips} captioned 9:16 clips.`,
    description:
      `Upload a video or audio file up to ${durationLabel(S.maxDurationMinutes)} long. Amazon Transcribe produces word-level timestamps, a language model picks up to ${S.maxClips} moments from the transcript, and each one is rendered as a ${S.outputWidth}×${S.outputHeight} clip with captions burned in.`,
    bullets: [
      `Captions timed to each word, in cues of up to ${S.captionWordsPerCue} words`,
      `A language model reads the transcript and picks up to ${S.maxClips} moments of ${S.clipMinSeconds} to ${S.clipMaxSeconds} seconds, each with a score`,
      'Face detection centers the 9:16 window on the speaker, with one window position per clip',
      'Blur-fill keeps slides, screen recordings and cartoons fully visible when no face is found',
      'Burned-in captions, plus optional animated captions (words light up as they are spoken), zoom and color grading',
    ],
    steps: [
      {
        title: 'Upload',
        text: `Choose an ${joinOr(S.inputFormats)} file up to ${durationLabel(S.maxDurationMinutes)} and ${S.maxUploadGb} GB. The page checks both limits before it sends anything, and large files upload in parts that retry and resume.`,
      },
      {
        title: 'Pick a language',
        text: `Choose ${joinOr(S.languages)}. If you know the language, choose it yourself instead of relying on auto-detect.`,
      },
      {
        title: 'Set effects, or skip',
        text: 'After the upload you can switch on animated captions, zoom or color grading. Leave everything off to use the defaults, then press Start processing.',
      },
      {
        title: 'Wait for the job',
        text: 'The progress page shows transcription, ranking and rendering. You can close the tab. If email notifications are on in Settings, you get a message when the job finishes or fails.',
      },
      {
        title: 'Review and download',
        text: 'Preview the ranked clips and download them one at a time or as a zip. A finished job keeps the look it was rendered with, so set captions and effects before you press Start processing.',
      },
    ],
    cta: 'Create your first shorts',
    ctaLink: '/shorts',
    specs: [`${S.outputWidth}×${S.outputHeight}`, 'H.264 + AAC', `${S.frameRate} fps`, `Up to ${S.maxClips} clips per upload`],
    limits: [
      { label: 'Length per upload', value: durationLabel(S.maxDurationMinutes) },
      { label: 'File size per upload', value: `${S.maxUploadGb} GB` },
      { label: 'File types', value: S.inputFormats.join(', ') },
      { label: 'Clips per upload', value: `Up to ${S.maxClips}, ${S.clipMinSeconds} to ${S.clipMaxSeconds} seconds each` },
      { label: 'Output', value: shortsOutputLabel() },
      { label: 'Transcript languages', value: S.languages.join(', ') },
      { label: 'Price', value: PRODUCT_FACTS.pricing },
    ],
    handling:
      'Your file is uploaded to private storage and processed on AWS: Amazon Transcribe for the transcript, Amazon Bedrock for ranking, Amazon Rekognition for face detection and FFmpeg for rendering. The source file, transcript and clips stay in your account until you delete the job. Deleting a Short removes its files and its record.',
    troubleshooting: [
      {
        q: 'The page says my file is too long or too large.',
        a: `The limits are ${durationLabel(S.maxDurationMinutes)} and ${S.maxUploadGb} GB per upload. Split a long file at a natural break, or re-encode a large video at a lower resolution. The podcast guide has the ffmpeg commands.`,
      },
      {
        q: 'The job failed with "no speech detected".',
        a: 'The transcript had no words. Check that the file has audible speech and that the language setting matches it. Retrying the same file and settings will fail the same way.',
      },
      {
        q: 'Hindi speech came out as English words.',
        a: 'Upload the file again with Hindi selected as the language instead of auto-detect.',
      },
      {
        q: 'The speaker is cut off or off-center in a clip.',
        a: 'The 9:16 window is placed once per clip, on the largest face found. It does not follow movement, and with two people in frame the larger face wins. Crop the video yourself and upload the vertical version, or use a recording where the speaker stays near the middle.',
      },
      {
        q: 'Captions run past the edge of the frame.',
        a: 'The caption size is too large for the words in that cue. Lower the size in the animated captions settings, which you can change before processing starts or on a failed job before you retry it. A finished job cannot be restyled, so upload the file again if you need a smaller size.',
      },
      {
        q: 'I pressed Retry and the job failed again.',
        a: 'Retry helps when a server step failed. A problem with the file, such as no speech or a file that is too long, fails again until the file is fixed.',
      },
    ],
    guides: ['podcast-to-youtube-shorts', 'how-ai-video-clipping-works', 'animated-captions-for-shorts', 'landscape-to-vertical-video'],
    reviewed: '2026-10-25',
  },
  watermark: {
    slug: 'watermark',
    title: 'Free Watermark Tool',
    hook: 'Add draggable text and logo watermarks to images and video: free, private, in-browser.',
    description:
      `Open a ${joinOr(W.imageFormats)} image, or an ${joinOr(W.videoFormats)} video, and stamp it with a text watermark or your own logo that you can drag, resize, recolor and rotate. Everything runs in your browser: your files are never uploaded anywhere.`,
    bullets: [
      'Draggable positioning with size, opacity, color and rotation controls',
      `Text in ${W.fontChoices} fonts, or your own logo image`,
      `Images in as ${joinOr(W.imageFormats)}, video in as ${joinOr(W.videoFormats)}`,
      `Images out as ${joinOr(W.imageExports)} at the original size, video out as an MP4 rendered on your own machine`,
      'A Before / After toggle, and crop guides for 16:9, 9:16 and 1:1 on the preview',
    ],
    steps: [
      { title: 'Open a file', text: 'Drop in an image or a video. It is read from your disk, never uploaded.' },
      { title: 'Choose text or logo', text: 'Type your words and pick a font, or choose a logo image. A PNG with a transparent background gives the cleanest result.' },
      { title: 'Place and tune', text: 'Drag the mark on the preview or snap it to one of nine positions, then adjust opacity, size, color and rotation.' },
      { title: 'Export', text: 'Download the image as PNG or JPEG, or start the video export and wait for it to finish. Video always comes out as an MP4.' },
    ],
    cta: 'Watermark an image or video now',
    ctaLink: '/tools/add-watermark',
    specs: [`${W.imageFormats.join(' · ')}`, `Opens ${W.videoFormats.join(' · ')}`, 'Original-size export', 'In-browser only'],
    limits: [
      { label: 'Images in', value: W.imageFormats.join(', ') },
      { label: 'Videos in', value: W.videoFormats.join(', ') },
      { label: 'Images out', value: `${W.imageExports.join(' or ')}, at the original pixel size` },
      { label: 'Videos out', value: W.videoExport },
      { label: 'Large videos', value: `A warning appears above ${W.videoWarnMb} MB; rendering can need about 2 GB of free memory` },
      { label: 'First video export', value: 'Downloads a video engine of about 30 MB once, then reuses the cached copy' },
      { label: 'Account', value: 'None needed' },
    ],
    handling:
      'Nothing is uploaded. Images are drawn on a canvas in your browser tab, and videos are processed by FFmpeg compiled to WebAssembly in the same tab. The one network request involved is the first video export, which downloads the video engine from the jsDelivr CDN. That request carries no part of your file.',
    troubleshooting: [
      {
        q: 'The first video export is slow to start.',
        a: 'The page is downloading the video engine, about 30 MB. It is cached afterwards, so later exports start straight away.',
      },
      {
        q: 'The video cannot be previewed or played.',
        a: 'Your browser cannot decode its codec. Convert it to MP4 with H.264 video and AAC audio and open it again.',
      },
      {
        q: 'The tab slows down or crashes on a large video.',
        a: 'The file needs more memory than the browser can spare. Use a shorter or lower-resolution copy and close other tabs.',
      },
      {
        q: 'My logo has a white rectangle around it.',
        a: 'The logo file has no transparency. Use a PNG with a transparent background.',
      },
      {
        q: 'The 16:9, 9:16 and 1:1 buttons do not crop my export.',
        a: 'They draw a dashed guide on the preview so you can see where a platform will crop. The export keeps the full frame.',
      },
    ],
    guides: ['add-watermark-to-image-in-browser', 'mp4-vs-webm-formats'],
    reviewed: '2026-10-25',
  },
  'txt-to-video-quiz': {
    slug: 'txt-to-video-quiz',
    title: 'TXT to Quiz Video Generator',
    hook: 'Turn a text file of questions into a narrated quiz video.',
    description:
      `Upload a TXT file with multiple-choice questions, pick an AI voice, and get a narrated ${Q.outputLabel}. Each question is read aloud on its slide, then its answer is revealed and read aloud, ready for YouTube or the classroom.`,
    bullets: [
      'Plain-text question format: numbered, bulleted or tab-indented, no special authoring tools',
      `${Q.voices.length} narration voices: ${joinAnd(Q.voices)}`,
      `Six designs (${joinAnd(Q.designs)}) with their own colours, type scale and layout, previewed in the project form`,
      'A question slide followed by an answer slide, both narrated',
      'Answers marked with an Answer: line, an asterisk or (correct)',
    ],
    steps: [
      { title: 'Write the questions', text: 'Put each question and its options in a plain text file and mark the correct answer. The format guide shows the three accepted layouts.' },
      { title: 'Create a project', text: 'Name it, then choose a slide design and one of the narration voices.' },
      { title: 'Upload the TXT file', text: `The file can be up to ${Q.maxFileMb} MB. Questions that cannot be parsed are listed as warnings on the project page.` },
      { title: 'Wait for the render', text: 'The pipeline narrates each slide, times the video and renders the MP4. The project page shows the live stage.' },
      { title: 'Preview and download', text: 'Watch the result in the preview, download the MP4, and re-render with another voice or design if you want.' },
    ],
    cta: 'Create a quiz video',
    ctaLink: '/projects/new',
    specs: [Q.outputLabel, `${Q.voices.length} AI voices`, 'Narrated answers'],
    limits: [
      { label: 'Input', value: `${Q.inputFormat} file, up to ${Q.maxFileMb} MB` },
      { label: 'Question layouts', value: 'Numbered, bulleted or tab-indented' },
      { label: 'Number of questions', value: 'No fixed cap. We suggest splitting a quiz of more than about 100 questions into separate videos.' },
      { label: 'Voices', value: Q.voices.join(', ') },
      { label: 'Designs', value: Q.designs.join(', ') },
      { label: 'Output', value: Q.outputLabel },
      { label: 'Price', value: PRODUCT_FACTS.pricing },
    ],
    handling:
      'The text file is uploaded and stored with your project so you can re-render it. Narration is generated by Amazon Polly, and the slides and video are rendered on AWS. Deleting a project removes its files and its record.',
    troubleshooting: [
      {
        q: 'The upload says no valid questions were found.',
        a: 'The parser could not find a question with options in the file. Check that each question is followed by its options and that the layout matches one of the three formats in the format guide.',
      },
      {
        q: 'A question has no answer reveal.',
        a: 'That question has no marked correct answer. The video still renders, and the project page lists the question as a warning so you can fix the file.',
      },
      {
        q: 'The file is rejected as too large.',
        a: `The limit is ${Q.maxFileMb} MB of plain text. Split a very large quiz into several files and make one video from each.`,
      },
      {
        q: 'The narrator mispronounces a word.',
        a: 'Respell the word in the text file so it sounds right when read aloud, and re-render. The narration guide has examples.',
      },
      {
        q: 'How long does rendering take?',
        a: 'A short quiz usually takes a few minutes, and a longer one takes longer. The project page shows the stage it is in.',
      },
    ],
    guides: ['quiz-file-format-guide', 'quiz-videos-youtube', 'ai-narration-guide', 'educational-video-best-practices'],
    reviewed: '2026-10-25',
  },
};

@Component({
  selector: 'app-feature-page',
  standalone: true,
  imports: [RouterLink, SiteFooterComponent],
  template: `
    <div class="sans bg-[#FAF7F2] text-[#1A1714] antialiased min-h-[100dvh]">
      <header class="max-w-3xl mx-auto px-4 pt-24 pb-6 text-center">
        <a routerLink="/" class="inline-flex items-center min-h-[44px] text-[13px] text-[#6B6560] hover:text-[#1A1714]">← Indifferent</a>
        <h1 class="mt-4 font-medium tracking-[-0.02em] leading-[1.02] text-[clamp(2rem,5vw,3.4rem)]">{{ data.title }}</h1>
        <p class="mt-4 text-[16.5px] text-[#6B6560] leading-relaxed">{{ data.hook }}</p>
      </header>
      <main class="max-w-3xl mx-auto px-4 pb-10">
        <p class="text-[15.5px] leading-relaxed">{{ data.description }}</p>
        <ul class="mt-6 space-y-3">
          @for (b of data.bullets; track $index) {
            <li class="flex items-start gap-3 rounded-[14px] border border-black/10 bg-white px-4 py-3 text-[14.5px]">
              <span class="mt-0.5 inline-flex w-5 h-5 shrink-0 items-center justify-center rounded-full bg-[#D96C3D]/15 text-[#A8481F] text-[12px] font-bold" aria-hidden="true">✓</span>
              {{ b }}
            </li>
          }
        </ul>
        <div class="mt-6 flex flex-wrap gap-2">
          @for (spec of data.specs; track $index) {
            <span class="rounded-full border border-black/15 px-3 py-1 text-[12px] text-[#6B6560]">{{ spec }}</span>
          }
        </div>

        <section class="mt-10" aria-labelledby="steps-heading">
          <h2 id="steps-heading" class="text-[20px] font-semibold">Step by step</h2>
          <ol class="mt-4 space-y-3">
            @for (s of data.steps; track s.title; let i = $index) {
              <li class="flex gap-4 rounded-[16px] border border-black/10 bg-white p-5">
                <span class="mt-0.5 inline-flex w-7 h-7 shrink-0 items-center justify-center rounded-full bg-[#1A1714] text-white text-[13px] font-semibold" aria-hidden="true">{{ i + 1 }}</span>
                <div>
                  <h3 class="font-semibold text-[15px]">{{ s.title }}</h3>
                  <p class="mt-1 text-[14px] leading-relaxed text-[#3E3A33]">{{ s.text }}</p>
                </div>
              </li>
            }
          </ol>
        </section>

        <section class="mt-10" aria-labelledby="limits-heading">
          <h2 id="limits-heading" class="text-[20px] font-semibold">Limits and formats</h2>
          <dl class="mt-4 rounded-[16px] border border-black/10 bg-white divide-y divide-black/10">
            @for (l of data.limits; track l.label) {
              <div class="grid grid-cols-1 sm:grid-cols-[200px_1fr] gap-1 sm:gap-4 px-5 py-3 text-[14px]">
                <dt class="font-semibold">{{ l.label }}</dt>
                <dd class="text-[#3E3A33]">{{ l.value }}</dd>
              </div>
            }
          </dl>
        </section>

        <section class="mt-10" aria-labelledby="handling-heading">
          <h2 id="handling-heading" class="text-[20px] font-semibold">How your files are handled</h2>
          <p class="mt-3 text-[14.5px] leading-relaxed text-[#3E3A33]">{{ data.handling }} The <a routerLink="/privacy" class="underline underline-offset-4">privacy policy</a> has the full list of services and the deletion details.</p>
        </section>

        <section class="mt-10" aria-labelledby="trouble-heading">
          <h2 id="trouble-heading" class="text-[20px] font-semibold">Troubleshooting</h2>
          <div class="mt-4 space-y-3">
            @for (t of data.troubleshooting; track t.q) {
              <details class="rounded-[14px] border border-black/10 bg-white px-5 py-4">
                <summary class="cursor-pointer font-semibold text-[14.5px]">{{ t.q }}</summary>
                <p class="mt-3 text-[14px] leading-relaxed text-[#3E3A33]">{{ t.a }}</p>
              </details>
            }
          </div>
        </section>

        <div class="mt-10 rounded-[20px] bg-[#1A1714] text-white px-6 py-8 text-center">
          <p class="font-medium text-[19px]">{{ data.cta }}</p>
          <a [routerLink]="data.ctaLink" class="mt-4 inline-flex items-center min-h-[44px] px-7 rounded-full bg-[#BC5227] hover:bg-[#A8481F] transition-colors text-white font-semibold text-[15px]">Try for free</a>
          <p class="mt-3 text-[12.5px] text-white/70">Free during beta · No credit card</p>
        </div>

        <section class="mt-10" aria-labelledby="guides-heading">
          <h2 id="guides-heading" class="text-[20px] font-semibold">Guides</h2>
          <ul class="mt-4 grid grid-cols-1 sm:grid-cols-2 gap-3">
            @for (g of guides; track g.slug) {
              <li>
                <a [routerLink]="path(g.slug)" class="block h-full rounded-[14px] border border-black/10 bg-white p-4 hover:border-black/30 transition-colors">
                  <span class="block text-[11.5px] uppercase tracking-[0.08em] text-[#6B6560]">{{ g.category }}</span>
                  <span class="mt-1 block text-[14.5px] font-semibold leading-snug">{{ g.title }}</span>
                </a>
              </li>
            }
          </ul>
        </section>

        <div class="mt-10">
          <p class="text-[13.5px] font-semibold mb-3">Other tools</p>
          <div class="flex flex-wrap gap-2">
            @for (f of others; track f.slug) {
              <a [routerLink]="['/features', f.slug]" class="inline-flex items-center min-h-[44px] rounded-full border border-black/15 px-4 text-[13px] hover:bg-black/5 transition-colors">{{ f.title }}</a>
            }
          </div>
        </div>
        <p class="mt-8 text-[12.5px] text-[#6B6560]">Last checked against the product on {{ reviewed }}.</p>
      </main>
      <app-site-footer />
    </div>
  `,
  styles: [`
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
  `],
})
export class FeaturePageComponent implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly document = inject(DOCUMENT);
  private readonly destroyRef = inject(DestroyRef);

  data: FeatureData = FEATURE_DATA['ai-shorts'];
  others: FeatureData[] = [];
  guides: ArticleMeta[] = [];
  reviewed = '';

  ngOnInit(): void {
    // The router reuses this component when only :slug changes, as it does for the
    // "Other tools" links, so follow the param map instead of reading it once.
    this.route.paramMap
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((params) => this.show(params.get('slug') ?? 'ai-shorts'));
  }

  private show(slug: string): void {
    const known = slug in FEATURE_DATA;
    this.data = FEATURE_DATA[slug] ?? FEATURE_DATA['ai-shorts'];
    this.others = Object.values(FEATURE_DATA).filter((f) => f.slug !== this.data.slug);
    this.guides = this.data.guides.map((g) => findArticle(g)).filter((a): a is ArticleMeta => !!a);
    this.reviewed = formatLongDate(this.data.reviewed);
    const url = `https://indifferent.fun/features/${this.data.slug}`;
    setPageSeo(this.document, { title: `${this.data.title} | Indifferent`, description: this.data.description, canonical: url });
    // An unknown slug shows the Shorts page at a made-up address: keep it out of the index.
    if (!known) setRobots(this.document, ROBOTS_NOINDEX);
    setJsonLd(this.document, 'webpage', {
      '@context': 'https://schema.org',
      '@type': 'WebPage',
      name: `${this.data.title} | Indifferent`,
      url,
      description: this.data.description,
    });
    setJsonLd(this.document, 'breadcrumbs', {
      '@context': 'https://schema.org',
      '@type': 'BreadcrumbList',
      itemListElement: [
        { '@type': 'ListItem', position: 1, name: 'Home', item: 'https://indifferent.fun/' },
        { '@type': 'ListItem', position: 2, name: this.data.title, item: url },
      ],
    });
  }

  path(slug: string): string {
    return articlePath(slug);
  }
}
