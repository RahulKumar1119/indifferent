import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { ThemeService } from '../../../core/services/theme.service';

/** Slim app navbar for the Shorts pages: brand, section links, theme toggle. */
@Component({
  selector: 'app-navbar',
  standalone: true,
  imports: [CommonModule, RouterLink, LucideDynamicIcon],
  template: `
    <header class="border-b border-[hsl(var(--border))] bg-[hsl(var(--background))]/90 backdrop-blur sticky top-0 z-40">
      <div class="max-w-7xl mx-auto px-4 h-14 flex items-center justify-between gap-2">
        <a routerLink="/" class="font-semibold tracking-tight text-[16px]">Indifferent<span class="text-[#D96C3D]">.</span></a>
        <nav class="flex items-center gap-1 text-[13px]" aria-label="Shorts sections">
          <a routerLink="/shorts" class="px-3 py-1.5 rounded-lg hover:bg-white/5 transition-colors">New</a>
          <a routerLink="/shorts/history" class="px-3 py-1.5 rounded-lg hover:bg-white/5 transition-colors">History</a>
          <a routerLink="/dashboard" class="hidden sm:inline px-3 py-1.5 rounded-lg hover:bg-white/5 transition-colors">Dashboard</a>
          <button
            class="btn-interactive ml-1 w-9 h-9 rounded-lg border border-[hsl(var(--border))] inline-flex items-center justify-center"
            (click)="theme.toggleTheme()"
            [attr.aria-label]="theme.currentTheme() === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'"
            [title]="theme.currentTheme() === 'dark' ? 'Light mode' : 'Dark mode'"
          >
            @if (theme.currentTheme() === 'dark') {
              <svg lucideIcon="sun" [size]="17"></svg>
            } @else {
              <svg lucideIcon="moon" [size]="17"></svg>
            }
          </button>
        </nav>
      </div>
    </header>
  `,
})
export class NavbarComponent {
  readonly theme = inject(ThemeService);
}
