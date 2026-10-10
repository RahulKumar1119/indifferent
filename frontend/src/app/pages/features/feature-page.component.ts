import { Component, OnInit, inject } from '@angular/core';
import { CommonModule, DOCUMENT } from '@angular/common';
import { Meta, Title } from '@angular/platform-browser';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { setCanonical } from '../../shared/seo';

export interface FeatureStep {
  title: string;
  text: string;
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
}

/** Shared template for product feature pages; content comes from FEATURE_DATA. */
export const FEATURE_DATA: Record<string, FeatureData> = {
  'ai-shorts': {
    slug: 'ai-shorts',
    title: 'AI Shorts Generator',
    hook: 'Turn long videos into ranked 9:16 vertical clips automatically.',
    description:
      'Upload a video or audio file up to 10 minutes long. AI transcribes it with word-level timestamps, scores every moment for engagement, and renders the top 3 segments as full-HD vertical clips with burned-in captions.',
    bullets: [
      'Word-timed captions in short lines readable without sound',
      'Engagement-ranked segments: the best moments, not random cuts',
      'Face tracking pans the 9:16 window onto the speaker',
      'Blur-fill fit keeps cartoons, text and diagrams fully visible',
    ],
    steps: [
      { title: 'Upload', text: 'MP4, MOV, MP3 or WAV: podcasts, interviews, lectures, phone footage.' },
      { title: 'AI edits', text: 'Transcription, ranking, reframing and caption burn-in run automatically.' },
      { title: 'Download', text: 'Preview ranked clips and download 1080×1920 MP4s ready to post.' },
    ],
    cta: 'Create your first shorts',
    ctaLink: '/shorts',
    specs: ['1080×1920 full-HD', 'H.264 + AAC', '30 fps', 'Up to 3 clips per upload'],
  },
  watermark: {
    slug: 'watermark',
    title: 'Free Watermark Tool',
    hook: 'Add draggable text watermarks to images: free, private, in-browser.',
    description:
      'Open any JPG, PNG or WebP image and stamp it with a text watermark you can drag, resize, recolor and rotate. Everything runs locally in your browser: your images are never uploaded anywhere.',
    bullets: [
      'Draggable positioning with size, opacity, color and rotation controls',
      'JPG, PNG and WebP input with full-resolution PNG export',
      '100% private: files never leave your device',
      'No account needed to use the tool',
    ],
    steps: [
      { title: 'Open an image', text: 'Drop a JPG, PNG or WebP file into the tool.' },
      { title: 'Style the mark', text: 'Type your text, then drag, resize and tune opacity, color and rotation.' },
      { title: 'Export', text: 'Download the watermarked image at full resolution as PNG.' },
    ],
    cta: 'Watermark an image now',
    ctaLink: '/tools/add-watermark',
    specs: ['JPG · PNG · WebP', 'Full-resolution export', 'In-browser only', 'Free forever'],
  },
  'txt-to-video-quiz': {
    slug: 'txt-to-video-quiz',
    title: 'TXT to Quiz Video Generator',
    hook: 'Turn a text file of questions into a narrated quiz video.',
    description:
      'Upload a TXT file with multiple-choice questions, pick a template and an AI voice, and get a narrated MP4 video with timed question cards and answer reveals, ready for YouTube or the classroom.',
    bullets: [
      'Plain-text question format: no special authoring tools',
      '5 AI narration voices with timed pacing',
      'Multiple visual templates (classic, modern, education, dark, minimal, neon)',
      'Answer reveals and smooth transitions built in',
    ],
    steps: [
      { title: 'Upload TXT', text: 'Your questions in a simple text format.' },
      { title: 'Pick template + voice', text: 'Choose a visual style and one of 5 AI voices.' },
      { title: 'Render + download', text: 'The pipeline narrates, times and renders your MP4.' },
    ],
    cta: 'Create a quiz video',
    ctaLink: '/projects/new',
    specs: ['1080p MP4', '5 AI voices', '6 templates', 'Timed reveals'],
  },
};

@Component({
  selector: 'app-feature-page',
  standalone: true,
  imports: [CommonModule, RouterLink],
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
          @for (b of data.bullets; track b) {
            <li class="flex items-start gap-3 rounded-[14px] border border-black/10 bg-white px-4 py-3 text-[14.5px]">
              <span class="mt-0.5 inline-flex w-5 h-5 shrink-0 items-center justify-center rounded-full bg-[#D96C3D]/15 text-[#A8481F] text-[12px] font-bold">✓</span>
              {{ b }}
            </li>
          }
        </ul>
        <div class="mt-8 grid grid-cols-1 sm:grid-cols-3 gap-4">
          @for (s of data.steps; track s.title) {
            <div class="rounded-[16px] border border-black/10 bg-white p-5">
              <h2 class="font-semibold text-[15px]">{{ s.title }}</h2>
              <p class="mt-1.5 text-[13.5px] leading-relaxed text-[#6B6560]">{{ s.text }}</p>
            </div>
          }
        </div>
        <div class="mt-6 flex flex-wrap gap-2">
          @for (spec of data.specs; track spec) {
            <span class="rounded-full border border-black/15 px-3 py-1 text-[12px] text-[#6B6560]">{{ spec }}</span>
          }
        </div>
        <div class="mt-8 rounded-[20px] bg-[#1A1714] text-white px-6 py-8 text-center">
          <p class="font-medium text-[19px]">{{ data.cta }}</p>
          <a [routerLink]="data.ctaLink" class="mt-4 inline-block px-7 py-3.5 rounded-full bg-[#BC5227] hover:bg-[#A8481F] transition-colors text-white font-semibold text-[15px]">Try for free</a>
          <p class="mt-3 text-[12.5px] text-white/60">Free during beta · No credit card</p>
        </div>
        <div class="mt-8">
          <p class="text-[13.5px] font-semibold mb-3">Related guides</p>
          <div class="flex flex-wrap gap-2">
            <a routerLink="/blog/quiz-videos-youtube" class="inline-flex items-center min-h-[44px] rounded-full border border-black/15 px-4 text-[13px] hover:bg-black/5 transition-colors">Quiz videos for YouTube</a>
            <a routerLink="/blog/ai-narration-guide" class="inline-flex items-center min-h-[44px] rounded-full border border-black/15 px-4 text-[13px] hover:bg-black/5 transition-colors">AI narration guide</a>
            <a routerLink="/blog/educational-video-best-practices" class="inline-flex items-center min-h-[44px] rounded-full border border-black/15 px-4 text-[13px] hover:bg-black/5 transition-colors">Educational video tips</a>
          </div>
        </div>
        <div class="mt-8">
          <p class="text-[13.5px] font-semibold mb-3">Other features</p>
          <div class="flex flex-wrap gap-2">
            @for (f of others; track f.slug) {
              <a [routerLink]="['/features', f.slug]" class="inline-flex items-center min-h-[44px] rounded-full border border-black/15 px-4 text-[13px] hover:bg-black/5 transition-colors">{{ f.title }}</a>
            }
          </div>
        </div>
      </main>
      <footer class="border-t border-black/10 px-4 py-8">
        <p class="text-center text-[12.5px] text-[#6B6560]">© {{ currentYear }} Indifferent · <a routerLink="/" class="inline-flex items-center min-h-[44px] px-1 underline underline-offset-4">Home</a></p>
      </footer>
    </div>
  `,
  styles: [`
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
  `],
})
export class FeaturePageComponent implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly title = inject(Title);
  private readonly meta = inject(Meta);
  private readonly document = inject(DOCUMENT);

  data: FeatureData = FEATURE_DATA['ai-shorts'];
  others: FeatureData[] = [];
  currentYear = new Date().getFullYear();

  ngOnInit(): void {
    const slug = this.route.snapshot.paramMap.get('slug') ?? 'ai-shorts';
    this.data = FEATURE_DATA[slug] ?? FEATURE_DATA['ai-shorts'];
    this.others = Object.values(FEATURE_DATA).filter((f) => f.slug !== this.data.slug);
    this.title.setTitle(`${this.data.title} | Indifferent`);
    this.meta.updateTag({ name: 'description', content: this.data.description });
    this.meta.updateTag({ property: 'og:title', content: `${this.data.title} | Indifferent` });
    this.meta.updateTag({ property: 'og:description', content: this.data.description });
    this.meta.updateTag({ property: 'og:url', content: `https://indifferent.fun/features/${this.data.slug}` });
    this.meta.updateTag({ property: 'og:image', content: 'https://indifferent.fun/og-cover.jpg' });
    setCanonical(this.document, `https://indifferent.fun/features/${this.data.slug}`);
    const script = this.document.createElement('script');
    script.type = 'application/ld+json';
    script.text = JSON.stringify({
      '@context': 'https://schema.org',
      '@type': 'WebPage',
      name: `${this.data.title} | Indifferent`,
      url: `https://indifferent.fun/features/${this.data.slug}`,
      description: this.data.description,
    });
    this.document.head.appendChild(script);
  }
}
