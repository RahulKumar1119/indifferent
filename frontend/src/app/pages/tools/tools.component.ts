import { Component, OnInit, inject } from '@angular/core';
import { CommonModule, DOCUMENT } from '@angular/common';
import { Meta, Title } from '@angular/platform-browser';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { setCanonical } from '../../shared/seo';

interface Tool {
  icon: string;
  badge: string;
  title: string;
  text: string;
  points: string[];
  infoLink: string;
  ctaLink: string;
  ctaLabel: string;
}

@Component({
  selector: 'app-tools',
  standalone: true,
  imports: [CommonModule, RouterLink, LucideDynamicIcon],
  styles: [`
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
    .card-lift { transition: transform .35s cubic-bezier(0.23,1,0.32,1), box-shadow .35s; }
    .card-lift:hover { transform: translateY(-6px); }
    @media (prefers-reduced-motion: reduce) {
      .card-lift { transition: none; }
    }
  `],
  template: `
    <div class="sans bg-[#FAF7F2] text-[#1A1714] antialiased min-h-[100dvh]">
      <main class="max-w-5xl mx-auto px-4 pt-24 pb-16">
        <p class="text-center text-[11.5px] uppercase tracking-[0.22em] text-[#6B6560]">Free tools</p>
        <h1 class="mt-3 text-center font-medium tracking-[-0.02em] leading-[1.02] text-[clamp(2rem,5vw,3.2rem)]">
          Tools that do <span class="font-light italic">the boring work.</span>
        </h1>
        <p class="mt-4 text-center text-[15.5px] text-[#6B6560] max-w-[56ch] mx-auto leading-relaxed">
          Free utilities for video creators. The watermark tool runs entirely in your browser. Nothing uploads anywhere.
        </p>

        <div class="mt-10 grid grid-cols-1 md:grid-cols-3 gap-5">
          @for (tool of tools; track tool.title) {
            <div class="card-lift rounded-[20px] border border-black/10 bg-white p-6 flex flex-col">
              <span class="inline-flex w-11 h-11 items-center justify-center rounded-xl bg-[#D96C3D]/12 text-[#BC5227]">
                <svg lucideIcon="{{ tool.icon }}" [size]="22"></svg>
              </span>
              <span class="mt-4 text-[11px] uppercase tracking-[0.18em] text-[#6B6560]">{{ tool.badge }}</span>
              <h2 class="mt-1 font-semibold text-[18px]">{{ tool.title }}</h2>
              <p class="mt-2 text-[14px] leading-relaxed text-[#6B6560]">{{ tool.text }}</p>
              <ul class="mt-3 space-y-1.5 text-[13px] text-[#6B6560]">
                @for (point of tool.points; track point) {
                  <li class="flex items-start gap-2">
                    <span class="mt-0.5 inline-flex w-4 h-4 shrink-0 items-center justify-center rounded-full bg-[#D96C3D]/15 text-[#BC5227] text-[10px] font-bold">✓</span>
                    {{ point }}
                  </li>
                }
              </ul>
              <div class="mt-5 pt-1 flex items-center gap-3 mt-auto">
                <a [routerLink]="tool.ctaLink" class="px-5 py-2.5 rounded-full bg-[#1A1714] text-white hover:bg-[#2A2620] transition-colors text-[13.5px] font-medium">{{ tool.ctaLabel }}</a>
                <a [routerLink]="tool.infoLink" class="text-[13.5px] font-semibold text-[#BC5227] underline underline-offset-4">Learn more</a>
              </div>
            </div>
          }
        </div>

        <div class="mt-10 rounded-[20px] bg-[#1A1714] text-white px-6 py-8 text-center">
          <p class="font-medium text-[19px]">Need full videos, not just tools?</p>
          <p class="mt-2 text-[14px] text-white/60">Turn text or long footage into finished MP4s with AI narration, ranking and captions.</p>
          <a routerLink="/login" class="mt-4 inline-block px-7 py-3 rounded-full bg-[#D96C3D] hover:bg-[#BC5227] transition-colors text-white font-semibold text-[14.5px]">Try for free</a>
        </div>
      </main>
      <footer class="border-t border-black/10 px-4 py-8">
        <p class="text-center text-[12.5px] text-[#6B6560]">© {{ currentYear }} Indifferent · <a routerLink="/" class="underline underline-offset-4">Home</a></p>
      </footer>
    </div>
  `,
})
export class ToolsComponent implements OnInit {
  private readonly title = inject(Title);
  private readonly meta = inject(Meta);
  private readonly document = inject(DOCUMENT);

  currentYear = new Date().getFullYear();

  tools: Tool[] = [
    {
      icon: 'stamp',
      badge: 'In-browser · Free forever',
      title: 'Watermark Tool',
      text: 'Stamp images with draggable text watermarks. Files never leave your device.',
      points: ['JPG, PNG, WebP up to 20MB', 'Size, opacity, color, rotation', 'Full-resolution PNG export'],
      infoLink: '/features/watermark',
      ctaLink: '/tools/add-watermark',
      ctaLabel: 'Open tool',
    },
    {
      icon: 'file-text',
      badge: 'AI pipeline',
      title: 'TXT to Quiz Video',
      text: 'Plain-text questions in, narrated quiz MP4 out. Six templates, five voices.',
      points: ['Answer reveals + transitions', 'Timed AI narration', '1080p MP4 download'],
      infoLink: '/features/txt-to-video-quiz',
      ctaLink: '/projects/new',
      ctaLabel: 'Create video',
    },
    {
      icon: 'scissors',
      badge: 'AI pipeline',
      title: 'AI Shorts Generator',
      text: 'Long video or audio into ranked 9:16 clips with burned-in captions.',
      points: ['Engagement-ranked moments', 'Speaker tracking + blur-fill', '1080×1920 exports'],
      infoLink: '/features/ai-shorts',
      ctaLink: '/shorts',
      ctaLabel: 'Make shorts',
    },
  ];

  ngOnInit(): void {
    const url = 'https://indifferent.fun/tools';
    const description = 'Free creator tools: in-browser image watermarking, TXT-to-quiz-video AI pipeline, and AI shorts generation.';
    this.title.setTitle('Free Creator Tools | Indifferent');
    this.meta.updateTag({ name: 'description', content: description });
    this.meta.updateTag({ property: 'og:title', content: 'Free Creator Tools | Indifferent' });
    this.meta.updateTag({ property: 'og:description', content: description });
    this.meta.updateTag({ property: 'og:url', content: url });
    this.meta.updateTag({ property: 'og:image', content: 'https://indifferent.fun/og-cover.jpg' });
    setCanonical(this.document, url);
    const script = this.document.createElement('script');
    script.type = 'application/ld+json';
    script.text = JSON.stringify({
      '@context': 'https://schema.org',
      '@type': 'WebPage',
      name: 'Free Creator Tools | Indifferent',
      url,
      description,
    });
    this.document.head.appendChild(script);
  }
}
