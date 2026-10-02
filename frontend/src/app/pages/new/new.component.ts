import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';

interface Product {
  title: string;
  text: string;
  icon: string;
  link: string;
  badge: string;
}

@Component({
  selector: 'app-new',
  standalone: true,
  imports: [CommonModule, RouterLink, LucideDynamicIcon],
  template: `
    <div class="sans bg-[#FAF7F2] text-[#1A1714] antialiased min-h-[100dvh]">
      <main class="max-w-5xl mx-auto px-4 pt-20 pb-16">
        <p class="text-center text-[11.5px] uppercase tracking-[0.22em] text-[#6B6560]">What do you want to create?</p>
        <h1 class="mt-3 text-center font-medium tracking-[-0.02em] leading-[1.02] text-[clamp(2rem,5vw,3.2rem)]">
          Pick your <span class="font-light italic">studio tool.</span>
        </h1>
        <div class="mt-10 grid grid-cols-1 md:grid-cols-3 gap-5">
          @for (p of products; track p.link) {
            <a [routerLink]="p.link" class="card-lift rounded-[20px] border border-black/10 bg-white p-6 flex flex-col hover:border-black/25 transition-colors">
              <span class="inline-flex w-11 h-11 items-center justify-center rounded-xl bg-[#D96C3D]/12 text-[#BC5227]">
                <svg lucideIcon="{{ p.icon }}" [size]="22"></svg>
              </span>
              <span class="mt-4 text-[11px] uppercase tracking-[0.18em] text-[#6B6560]">{{ p.badge }}</span>
              <h2 class="mt-1 font-semibold text-[18px]">{{ p.title }}</h2>
              <p class="mt-2 text-[14px] leading-relaxed text-[#6B6560] flex-1">{{ p.text }}</p>
              <span class="mt-4 inline-flex items-center gap-1.5 text-[14px] font-semibold text-[#BC5227]">Open <span aria-hidden="true">→</span></span>
            </a>
          }
        </div>
        <p class="mt-8 text-center text-[13.5px] text-[#6B6560]">
          Not sure? <a routerLink="/dashboard" class="font-semibold text-[#1A1714] underline underline-offset-4">Go to your dashboard</a>
        </p>
      </main>
    </div>
  `,
  styles: [`
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
    .card-lift { transition: transform .35s cubic-bezier(0.23,1,0.32,1), box-shadow .35s; }
    .card-lift:hover { transform: translateY(-6px); }
  `],
})
export class NewComponent {
  products: Product[] = [
    {
      title: 'TXT to Quiz Video',
      text: 'Upload a text file of questions, pick a template and voice — get a narrated quiz MP4.',
      icon: 'file-text',
      link: '/projects/new',
      badge: 'Quiz films',
    },
    {
      title: 'AI Shorts Generator',
      text: 'Turn a long video or audio into ranked 9:16 vertical clips with burned-in captions.',
      icon: 'scissors',
      link: '/shorts',
      badge: 'Vertical clips',
    },
    {
      title: 'Watermark Tool',
      text: 'Stamp images with draggable text watermarks. Free, private, 100% in-browser.',
      icon: 'stamp',
      link: '/tools/add-watermark',
      badge: 'Free tool',
    },
  ];
}
