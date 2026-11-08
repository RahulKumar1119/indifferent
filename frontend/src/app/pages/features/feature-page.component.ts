import { Component, DestroyRef, OnInit, inject } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { DOCUMENT } from '@angular/common';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { PRODUCT_FACTS, durationLabel, joinAnd, joinOr, shortsOutputLabel, storyOutputLabel } from '../../core/config/product-facts';
import { PublicHeaderComponent } from '../../shared/components/public-header/public-header.component';
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
const T = PRODUCT_FACTS.story;

/** Shared template for product feature pages; content comes from FEATURE_DATA. */
export const FEATURE_DATA: Record<string, FeatureData> = {
  'story-video': {
    slug: 'story-video',
    title: 'Story to Video Generator',
    hook: `Paste a short story and get a narrated ${T.outputWidth}×${T.outputHeight} video with matching stock footage.`,
    description:
      `Write or paste a story of ${T.minStoryChars} to ${T.maxStoryChars} characters. A language model breaks it into ${T.minScenes} to ${T.maxScenes} scenes, each with its own narration and a search for a matching photo or clip from ${joinAnd(T.stockLibraries)}. You review the scenes, swap any picture you want, then render a ${storyOutputLabel()} up to ${T.maxTotalSeconds} seconds long. Narration stays in the story's own language.`,
    bullets: [
      `A model turns your story into ${T.minScenes} to ${T.maxScenes} scenes, each with narration and a suggested visual`,
      `Stock photos and clips from ${joinAnd(T.stockLibraries)}, searched per scene and yours to swap or remove`,
      `Narration by Amazon Polly in the story's own language: English (${joinAnd(T.voicesEn)}) or Hindi (${joinAnd(T.voicesHi)})`,
      `A ${T.outputWidth}×${T.outputHeight} vertical video of up to ${T.maxTotalSeconds} seconds, with captions`,
      'You confirm every scene and picture before anything renders',
    ],
    steps: [
      { title: 'Paste your story', text: `Put in a story of ${T.minStoryChars} to ${T.maxStoryChars} characters. The story text is sent to the model to plan the video and is not stored.` },
      { title: 'Review the plan', text: `The model returns ${T.minScenes} to ${T.maxScenes} scenes, each with narration, a length and a suggested photo or clip. Read it through before you change anything.` },
      { title: 'Choose the visuals', text: `Keep the suggested picture for a scene, or search ${joinAnd(T.stockLibraries)} and pick another. You can edit the narration too, as long as the whole video stays within ${T.maxTotalSeconds} seconds.` },
      { title: 'Make the video', text: 'Press Make the video. The chosen pictures are copied, each scene is narrated, and the video is rendered. You can close the tab and come back to it.' },
      { title: 'Download', text: 'When it is ready, preview the video and download the MP4. The download is the finished video on its own.' },
    ],
    cta: 'Turn a story into a video',
    ctaLink: '/story',
    specs: [`${T.outputWidth}×${T.outputHeight}`, 'H.264 + AAC', `${T.frameRate} fps`, `Up to ${T.maxTotalSeconds} seconds`],
    limits: [
      { label: 'Story length', value: `${T.minStoryChars} to ${T.maxStoryChars} characters` },
      { label: 'Scenes', value: `${T.minScenes} to ${T.maxScenes}, planned from the story` },
      { label: 'Video length', value: `Up to ${T.maxTotalSeconds} seconds` },
      { label: 'Stock libraries', value: joinAnd(T.stockLibraries) },
      { label: 'Narration', value: `English (${T.voicesEn.join(', ')}) or Hindi (${T.voicesHi.join(', ')})` },
      { label: 'Output', value: storyOutputLabel() },
      { label: 'Per account', value: `${T.maxActive} videos rendering at once, ${T.perDay} started per day` },
      { label: 'Price', value: PRODUCT_FACTS.pricing },
    ],
    handling:
      `Your story is sent to Amazon Bedrock to plan the video and is not stored; only its length is kept. The stock pictures you choose are copied into your account's private storage while the video is made, narration is generated by Amazon Polly, and the video is rendered on AWS. The working files are removed after 7 days, and the finished video stays until you delete it. Stock photos and clips come from ${joinAnd(T.stockLibraries)} under their licenses; the download is the finished video, not the separate stock files.`,
    troubleshooting: [
      {
        q: 'A scene has no suitable picture.',
        a: `The search found nothing for that scene's keywords. Open the scene and search ${joinAnd(T.stockLibraries)} yourself with simpler words, or choose a photo instead of a clip, then make the video again.`,
      },
      {
        q: 'The video would run over the limit.',
        a: `The whole video must stay within ${T.maxTotalSeconds} seconds. If your edits push it over, the save is refused rather than trimmed. Shorten the narration on one or more scenes and try again.`,
      },
      {
        q: 'A scene failed with "Pixabay is limiting downloads".',
        a: 'The picture library briefly limited downloads. This clears on its own, so make the video again in a few minutes, or choose a different picture for that scene.',
      },
      {
        q: 'My Hindi story was narrated in English.',
        a: `Narration follows the story's own script. Write the story in Devanagari so it is read as Hindi; a story typed in English letters is read as English.`,
      },
      {
        q: 'Can I keep the stock photos on their own?',
        a: `No. The download is the finished video. ${joinAnd(T.stockLibraries)} licenses cover using the pictures inside a video you make, not handing them out as separate files.`,
      },
    ],
    guides: ['ai-narration-guide'],
    reviewed: '2026-11-04',
  },
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
  imports: [RouterLink, PublicHeaderComponent, SiteFooterComponent],
  styles: [`
    :host { display: block; }
    .page { background: var(--color-paper); color: var(--color-ink); font-family: var(--font-body); font-size: var(--type-md); line-height: 1.55; -webkit-font-smoothing: antialiased; min-height: 100dvh; }
    .wrap { width: min(var(--container), 100%); margin-inline: auto; padding-inline: var(--gutter); }
    .col { width: min(52rem, 100%); display: grid; gap: var(--space-lg); }
    h1, h2, h3, p, ul, ol, dl, dd { margin: 0; }
    h1.title { font-family: var(--font-display); font-weight: 500; font-size: var(--type-display-s); line-height: 1.04; letter-spacing: var(--tracking-display); overflow-wrap: break-word; min-width: 0; text-wrap: balance; max-width: 16ch; }
    h2 { font-family: var(--font-display); font-weight: 500; font-size: var(--type-2xl); line-height: 1.1; letter-spacing: var(--tracking-display); }
    h3 { font-size: var(--type-md); font-weight: 600; }
    .lead { margin-top: var(--space-lg); font-size: var(--type-lead); line-height: 1.45; color: var(--color-ink-2); max-width: 46ch; }
    .muted { color: var(--color-muted); }

    .hero { background: var(--color-paper); color: var(--color-ink); padding: calc(56px + var(--space-3xl)) 0 var(--space-2xl); }
    .hero-grid { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-2xl); align-items: end; }
    @media (min-width: 64rem) { .hero-grid { grid-template-columns: minmax(0, 1.1fr) minmax(0, 0.9fr); } }
    .actions { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-md); margin-top: var(--space-xl); }
    .fine { margin-top: var(--space-sm); font-size: var(--type-sm); color: var(--color-muted); }
    .glance { list-style: none; padding: 0; display: grid; gap: var(--space-xs); border-top: 1px solid var(--color-rule); }
    .glance li { padding: var(--space-sm) 0; border-bottom: 1px solid var(--color-rule); font-size: var(--type-sm); color: var(--color-ink-2); }

    .block { padding: var(--space-2xl) 0; border-top: 1px solid var(--color-rule); }
    .block.first { border-top: 0; }
    .block > .wrap > * + * { margin-top: var(--space-lg); }
    .intro { font-size: var(--type-lead); line-height: 1.5; color: var(--color-ink-2); max-width: 60ch; }
    ul.ticks { list-style: none; padding: 0; display: grid; gap: var(--space-sm); color: var(--color-ink-2); }
    ul.ticks li { padding-left: var(--space-lg); position: relative; }
    ul.ticks li::before { content: ''; position: absolute; left: 0; top: 0.7em; width: 10px; height: 2px; background: var(--color-accent); }

    ol.steps { list-style: none; padding: 0; counter-reset: step; border-top: 1px solid var(--color-rule); }
    ol.steps > li { display: grid; grid-template-columns: 56px minmax(0, 1fr); gap: var(--space-lg); padding: var(--space-lg) 0; border-bottom: 1px solid var(--color-rule); }
    ol.steps > li::before { counter-increment: step; content: counter(step); font-family: var(--font-body); font-size: var(--type-xl); font-weight: 500; line-height: 1.1; color: var(--color-accent-text); font-variant-numeric: tabular-nums; }
    ol.steps p { margin-top: var(--space-2xs); color: var(--color-ink-2); }

    dl.limits { border-top: 1px solid var(--color-rule); }
    dl.limits > div { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-3xs); padding: var(--space-sm) 0; border-bottom: 1px solid var(--color-rule); }
    @media (min-width: 40rem) { dl.limits > div { grid-template-columns: 14rem minmax(0, 1fr); gap: var(--space-lg); } }
    dl.limits dt { font-weight: 600; }
    dl.limits dd { color: var(--color-ink-2); }

    .text { color: var(--color-ink-2); max-width: 65ch; }
    .link { color: var(--color-accent-text); text-decoration: underline; text-underline-offset: 0.2em; }
    .link:hover { color: var(--color-ink); }
    .link:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: 2px; }

    .faq { display: grid; gap: var(--space-sm); }
    .faq p { color: var(--color-ink-2); }

    .cta { background: var(--color-paper-2); }
    .cta .wrap { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-lg); }
    .cta p { font-family: var(--font-display); font-size: var(--type-xl); font-weight: 500; line-height: 1.2; max-width: 30ch; }

    ul.cards { list-style: none; padding: 0; display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-md); }
    @media (min-width: 40rem) { ul.cards { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
    .card { display: block; height: 100%; padding: var(--space-md) var(--space-lg); background: var(--color-paper-2); border: 1px solid var(--color-rule); border-radius: var(--radius-surface); color: inherit; text-decoration: none; transition: border-color var(--dur-short) var(--ease-out); }
    .card:hover { border-color: var(--color-muted); }
    .card:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; }
    .card small { display: block; font-size: var(--type-sm); color: var(--color-muted); }
    .card strong { display: block; margin-top: var(--space-3xs); font-weight: 600; line-height: 1.35; }

    .others { display: flex; flex-wrap: wrap; gap: var(--space-xs); }
    .reviewed { font-size: var(--type-sm); color: var(--color-muted); }
    @media (prefers-reduced-motion: reduce) { .card { transition: none; } }
  `],
  template: `
    <div class="page hm hm-on-light">
      <app-public-header />
      <main>
        <section class="hero hm-on-dark" aria-labelledby="feature-title">
          <div class="wrap hero-grid">
            <div>
              <h1 id="feature-title" class="title">{{ data.title }}</h1>
              <p class="lead">{{ data.hook }}</p>
              <div class="actions">
                <a [routerLink]="data.ctaLink" class="ui-btn ui-btn--primary">Try for free</a>
              </div>
              <p class="fine">Free during beta. No credit card.</p>
            </div>
            <ul class="glance" aria-label="At a glance">
              @for (spec of data.specs; track $index) {
                <li>{{ spec }}</li>
              }
            </ul>
          </div>
        </section>

        <section class="block first">
          <div class="wrap"><div class="col">
            <p class="intro">{{ data.description }}</p>
            <ul class="ticks">
              @for (b of data.bullets; track $index) {
                <li>{{ b }}</li>
              }
            </ul>
          </div></div>
        </section>

        <section class="block" aria-labelledby="steps-heading">
          <div class="wrap"><div class="col">
            <h2 id="steps-heading">Step by step</h2>
            <ol class="steps">
              @for (s of data.steps; track s.title) {
                <li>
                  <div>
                    <h3>{{ s.title }}</h3>
                    <p>{{ s.text }}</p>
                  </div>
                </li>
              }
            </ol>
          </div></div>
        </section>

        <section class="block" aria-labelledby="limits-heading">
          <div class="wrap"><div class="col">
            <h2 id="limits-heading">Limits and formats</h2>
            <dl class="limits">
              @for (l of data.limits; track l.label) {
                <div>
                  <dt>{{ l.label }}</dt>
                  <dd>{{ l.value }}</dd>
                </div>
              }
            </dl>
          </div></div>
        </section>

        <section class="block" aria-labelledby="handling-heading">
          <div class="wrap"><div class="col">
            <h2 id="handling-heading">How your files are handled</h2>
            <p class="text">{{ data.handling }} The <a routerLink="/privacy" class="link">privacy policy</a> has the full list of services and the deletion details.</p>
          </div></div>
        </section>

        <section class="block" aria-labelledby="trouble-heading">
          <div class="wrap"><div class="col">
            <h2 id="trouble-heading">Troubleshooting</h2>
            <div class="faq">
              @for (t of data.troubleshooting; track t.q) {
                <details class="ui-disclosure">
                  <summary>{{ t.q }}</summary>
                  <p>{{ t.a }}</p>
                </details>
              }
            </div>
          </div></div>
        </section>

        <section class="block cta" aria-label="Get started">
          <div class="wrap">
            <p>{{ data.cta }}</p>
            <a [routerLink]="data.ctaLink" class="ui-btn ui-btn--secondary">Try for free</a>
          </div>
        </section>

        <section class="block" aria-labelledby="guides-heading">
          <div class="wrap">
            <h2 id="guides-heading">Guides</h2>
            <ul class="cards">
              @for (g of guides; track g.slug) {
                <li>
                  <a [routerLink]="path(g.slug)" class="card">
                    <small>{{ g.category }}</small>
                    <strong>{{ g.title }}</strong>
                  </a>
                </li>
              }
            </ul>
            <p class="muted" style="font-size:var(--type-sm);font-weight:600;margin-top:var(--space-xl)">Other tools</p>
            <div class="others">
              @for (f of others; track f.slug) {
                <a [routerLink]="['/features', f.slug]" class="ui-btn ui-btn--secondary">{{ f.title }}</a>
              }
            </div>
            <p class="reviewed">Last checked against the product on {{ reviewed }}.</p>
          </div>
        </section>
      </main>
      <app-site-footer />
    </div>
  `,
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
