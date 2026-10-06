import { Component, input } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { CreatorStats } from '../models/creator-stats.model';

/** Six creator metrics rendered as premium stat cards with skeleton loading. */
@Component({
  selector: 'app-creator-stats',
  standalone: true,
  imports: [LucideDynamicIcon],
  template: `
    <section aria-label="Creator statistics">
      @if (loading()) {
        <div class="grid grid-cols-2 lg:grid-cols-3 xl:grid-cols-6 gap-3 md:gap-4">
          @for (i of [1, 2, 3, 4, 5, 6]; track i) {
            <div class="skeleton h-[118px] rounded-2xl"></div>
          }
        </div>
      } @else {
        <div class="grid grid-cols-2 lg:grid-cols-3 xl:grid-cols-6 gap-3 md:gap-4">
          @for (stat of cards(); track stat.label) {
            <div
              class="glass-card spotlight-card group rounded-2xl p-4 md:p-5 transition-transform duration-200 hover:-translate-y-1"
            >
              <div class="flex items-start justify-between gap-2">
                <p
                  class="text-[11px] uppercase tracking-[0.12em] text-[hsl(var(--muted-foreground))]"
                >
                  {{ stat.label }}
                </p>
                <span
                  class="w-8 h-8 rounded-full flex items-center justify-center shrink-0"
                  [style.background]="stat.tint"
                  aria-hidden="true"
                >
                  <svg lucideIcon="{{ stat.icon }}" [size]="15" [style.color]="stat.color"></svg>
                </span>
              </div>
              <p class="mt-1 text-[30px] md:text-[34px] leading-none font-semibold tracking-tight" [style.color]="stat.color || 'inherit'">
                {{ stat.value }}
              </p>
              <p class="mt-1 text-[12px] text-[hsl(var(--muted-foreground))]">{{ stat.hint }}</p>
            </div>
          }
        </div>
      }
    </section>
  `,
})
export class CreatorStatsComponent {
  readonly stats = input.required<CreatorStats>();
  readonly loading = input(false);

  cards(): { label: string; value: string; hint: string; icon: string; color: string; tint: string }[] {
    const s = this.stats();
    return [
      { label: 'Videos', value: String(s.videosCreated), hint: 'Quiz films created', icon: 'clapperboard', color: '', tint: 'hsl(var(--primary) / 0.1)' },
      { label: 'Shorts', value: String(s.shortsGenerated), hint: 'AI clips generated', icon: 'scissors', color: '', tint: 'hsl(var(--primary) / 0.1)' },
      { label: 'Published', value: String(s.published), hint: 'Marked published', icon: 'circle-check', color: '#1E7A4C', tint: 'rgba(30,122,76,0.12)' },
      { label: 'Favorites', value: String(s.favorites), hint: 'Saved to favorites', icon: 'star', color: '#B8860B', tint: 'rgba(184,134,11,0.12)' },
      { label: 'Credits used', value: String(s.creditsUsed), hint: `of ${s.creditsQuota} this cycle`, icon: 'zap', color: '#BC5227', tint: 'rgba(188,82,39,0.12)' },
      { label: 'Time saved', value: `${s.minutesSaved}m`, hint: 'Vs. manual editing', icon: 'clock', color: '#6D28D9', tint: 'rgba(109,40,217,0.12)' },
    ];
  }
}
