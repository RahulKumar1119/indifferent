import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';

interface FooterLink {
  label: string;
  path: string;
}

/** Footer with the same link set on every content page, so the products, the
 * journal and the legal pages are always one click away. */
@Component({
  selector: 'app-site-footer',
  standalone: true,
  imports: [RouterLink],
  styles: [`
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
  `],
  template: `
    <footer class="sans border-t border-black/10 bg-[#FAF7F2] text-[#1A1714] px-4 py-10" aria-label="Site footer">
      <div class="max-w-3xl mx-auto grid grid-cols-1 sm:grid-cols-3 gap-8">
        <nav aria-label="Tools">
          <p class="text-[11.5px] uppercase tracking-[0.1em] text-[#6B6560]">Tools</p>
          <ul class="mt-2">
            @for (l of tools; track l.path) {
              <li><a [routerLink]="l.path" class="inline-flex items-center min-h-[44px] text-[14px] underline-offset-4 hover:underline">{{ l.label }}</a></li>
            }
          </ul>
        </nav>
        <nav aria-label="Learn">
          <p class="text-[11.5px] uppercase tracking-[0.1em] text-[#6B6560]">Learn</p>
          <ul class="mt-2">
            @for (l of learn; track l.path) {
              <li><a [routerLink]="l.path" class="inline-flex items-center min-h-[44px] text-[14px] underline-offset-4 hover:underline">{{ l.label }}</a></li>
            }
          </ul>
        </nav>
        <nav aria-label="Company">
          <p class="text-[11.5px] uppercase tracking-[0.1em] text-[#6B6560]">Company</p>
          <ul class="mt-2">
            @for (l of company; track l.path) {
              <li><a [routerLink]="l.path" class="inline-flex items-center min-h-[44px] text-[14px] underline-offset-4 hover:underline">{{ l.label }}</a></li>
            }
            <li><a href="mailto:support@indifferent.fun" class="inline-flex items-center min-h-[44px] text-[14px] underline-offset-4 hover:underline">support&#64;indifferent.fun</a></li>
          </ul>
        </nav>
      </div>
      <p class="max-w-3xl mx-auto mt-8 text-[12.5px] text-[#6B6560]">© {{ year }} Indifferent. Free during beta.</p>
    </footer>
  `,
})
export class SiteFooterComponent {
  readonly year = new Date().getFullYear();

  readonly tools: FooterLink[] = [
    { label: 'AI Shorts generator', path: '/features/ai-shorts' },
    { label: 'TXT to quiz video', path: '/features/txt-to-video-quiz' },
    { label: 'Watermark tool', path: '/features/watermark' },
  ];

  readonly learn: FooterLink[] = [
    { label: 'Journal', path: '/blog' },
    { label: 'Help and FAQ', path: '/help' },
    { label: 'Quiz file format guide', path: '/blog/quiz-file-format-guide' },
  ];

  readonly company: FooterLink[] = [
    { label: 'Home', path: '/' },
    { label: 'About', path: '/about' },
    { label: 'Contact', path: '/contact' },
    { label: 'Privacy', path: '/privacy' },
    { label: 'Terms', path: '/terms' },
  ];
}
