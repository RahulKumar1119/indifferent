import { Component, OnInit, inject } from '@angular/core';
import { CommonModule, DOCUMENT } from '@angular/common';
import { Meta, Title } from '@angular/platform-browser';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { setCanonical } from '../../shared/seo';
export interface UseCaseData {
  slug: string;
  title: string;
  hook: string;
  description: string;
  benefits: string[];
  cta: string;
}

/** Shared template for all persona SEO pages; content comes from route data. */
export const USE_CASE_DATA: Record<string, UseCaseData> = {
  'content-creators': {
    slug: 'content-creators',
    title: 'AI Shorts Maker for Content Creators',
    hook: 'Repurpose podcasts, interviews and videos into ranked vertical shorts.',
    description: 'Upload a long video or audio file and get up to 3 ranked 9:16 clips with burned-in captions, ready for Shorts, Reels and TikTok.',
    benefits: [
      'AI finds your most engaging moments: no timeline scrubbing',
      'Speaker tracking keeps you framed, blur-fill protects on-screen text',
      'Short caption lines styled for silent viewing',
      'Full-HD 1080×1920 MP4 exports, free during beta',
    ],
    cta: 'Turn your next video into shorts',
  },
  'marketing-teams': {
    slug: 'marketing-teams',
    title: 'AI Shorts Maker for Marketing Teams',
    hook: 'Create high-performing shorts that drive business growth.',
    description: 'Turn webinars, launches and interviews into a pipeline of short-form content without adding editing headcount.',
    benefits: [
      'One upload becomes three ranked, ready-to-post clips',
      'Consistent captioned style across every video',
      'YouTube Shorts-spec output: 1080×1920, H.264 + AAC, 30fps',
      'Minutes per video instead of hours in an editor',
    ],
    cta: 'Scale your short-form output',
  },
  agencies: {
    slug: 'agencies',
    title: 'AI Shorts Maker for Agencies',
    hook: 'Produce on-brand client content with consistent quality, fast.',
    description: 'Repurpose client long-form into platform-ready shorts with predictable turnaround and consistent caption styling.',
    benefits: [
      'Ranked clips remove subjective “which moment?” debates',
      'Same caption and framing treatment on every deliverable',
      'Blur-fill fit keeps client graphics and text intact',
      'Downloadable MP4s drop straight into scheduling tools',
    ],
    cta: 'Speed up client delivery',
  },
  coaches: {
    slug: 'coaches',
    title: 'AI Shorts Maker for Coaches',
    hook: 'Turn long lessons and calls into authority-building clips.',
    description: 'Extract the key insight from every coaching call, lesson or live session and publish it as a captioned short.',
    benefits: [
      'Key moments ranked by predicted engagement',
      'Speaker-tracked framing keeps you centered on camera',
      'Captions readable with sound off, where feeds are watched',
      'No editing skills or software needed',
    ],
    cta: 'Clip your next lesson',
  },
  'media-companies': {
    slug: 'media-companies',
    title: 'AI Shorts Maker for Media Companies',
    hook: 'Turn long-form content into high-performing shorts at scale.',
    description: 'Convert shows, interviews and coverage into a steady stream of vertical clips with broadcast-safe specs.',
    benefits: [
      'Batch-friendly: every upload yields ranked clips automatically',
      'Face tracking handles multi-speaker footage gracefully',
      'Blur-fill preserves graphics, lower-thirds and diagrams',
      'H.264 + AAC exports accepted everywhere',
    ],
    cta: 'Repurpose your archive',
  },
  educators: {
    slug: 'educators',
    title: 'AI Shorts Maker for Educators',
    hook: 'Clip the key explanation from every lecture, automatically.',
    description: 'Upload a lecture recording and get the most engaging explanations back as captioned vertical clips for revision and reach.',
    benefits: [
      'Engagement ranking surfaces the moments students rewatch',
      'Short caption lines aid comprehension and retention',
      'Diagrams and slides stay fully visible with blur-fill fit',
      'MP3 and WAV lecture audio supported too',
    ],
    cta: 'Clip your next lecture',
  },
};

@Component({
  selector: 'app-use-case',
  standalone: true,
  imports: [CommonModule, RouterLink],
  template: `
    <div class="sans bg-[#FAF7F2] text-[#1A1714] antialiased min-h-[100dvh]">
      <header class="max-w-3xl mx-auto px-4 pt-24 pb-6 text-center">
        <a routerLink="/shorts-maker" class="inline-flex items-center min-h-[44px] text-[13px] text-[#6B6560] hover:text-[#1A1714]">← AI Shorts Maker</a>
        <h1 class="mt-4 font-medium tracking-[-0.02em] leading-[1.02] text-[clamp(2rem,5vw,3.4rem)]">{{ data.title }}</h1>
        <p class="mt-4 text-[16.5px] text-[#6B6560] leading-relaxed">{{ data.hook }}</p>
      </header>
      <main class="max-w-3xl mx-auto px-4 pb-10">
        <p class="text-[15.5px] leading-relaxed">{{ data.description }}</p>
        <ul class="mt-6 space-y-3">
          @for (b of data.benefits; track b) {
            <li class="flex items-start gap-3 rounded-[14px] border border-black/10 bg-white px-4 py-3 text-[14.5px]">
              <span class="mt-0.5 inline-flex w-5 h-5 shrink-0 items-center justify-center rounded-full bg-[#D96C3D]/15 text-[#A8481F] text-[12px] font-bold">✓</span>
              {{ b }}
            </li>
          }
        </ul>
        <div class="mt-8 rounded-[20px] bg-[#1A1714] text-white px-6 py-8 text-center">
          <p class="font-medium text-[19px]">{{ data.cta }}</p>
          <a routerLink="/shorts" class="mt-4 inline-block px-7 py-3.5 rounded-full bg-[#BC5227] hover:bg-[#A8481F] transition-colors text-white font-semibold text-[15px]">Try for free</a>
          <p class="mt-3 text-[12.5px] text-white/60">Free during beta · No credit card</p>
        </div>
        <div class="mt-8">
          <p class="text-[13.5px] font-semibold mb-3">Other use cases</p>
          <div class="flex flex-wrap gap-2">
            @for (u of others; track u.slug) {
              <a [routerLink]="['/use-cases', u.slug]" class="inline-flex items-center min-h-[44px] rounded-full border border-black/15 px-4 text-[13px] hover:bg-black/5 transition-colors">{{ u.title }}</a>
            }
          </div>
        </div>
      </main>
      <footer class="border-t border-black/10 px-4 py-8">
        <p class="text-center text-[12.5px] text-[#6B6560]">© {{ currentYear }} Indifferent · <a routerLink="/shorts-maker" class="inline-flex items-center min-h-[44px] px-1 underline underline-offset-4">AI Shorts Maker</a></p>
      </footer>
    </div>
  `,
  styles: [`
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
  `],
})
export class UseCaseComponent implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly title = inject(Title);
  private readonly meta = inject(Meta);
  private readonly document = inject(DOCUMENT);

  data: UseCaseData = USE_CASE_DATA['content-creators'];
  others: UseCaseData[] = [];
  currentYear = new Date().getFullYear();

  ngOnInit(): void {
    const slug = this.route.snapshot.paramMap.get('slug') ?? 'content-creators';
    this.data = USE_CASE_DATA[slug] ?? USE_CASE_DATA['content-creators'];
    this.others = Object.values(USE_CASE_DATA).filter((u) => u.slug !== this.data.slug);
    const pageUrl = `https://indifferent.fun/use-cases/${this.data.slug}`;
    this.title.setTitle(`${this.data.title} | Indifferent`);
    this.meta.updateTag({ name: 'description', content: this.data.description });
    this.meta.updateTag({ property: 'og:title', content: `${this.data.title} | Indifferent` });
    this.meta.updateTag({ property: 'og:description', content: this.data.description });
    this.meta.updateTag({ property: 'og:url', content: pageUrl });
    setCanonical(this.document, pageUrl);
    const script = this.document.createElement('script');
    script.type = 'application/ld+json';
    script.text = JSON.stringify({
      '@context': 'https://schema.org',
      '@type': 'WebPage',
      name: `${this.data.title} | Indifferent`,
      url: pageUrl,
      description: this.data.description,
    });
    this.document.head.appendChild(script);
  }
}
