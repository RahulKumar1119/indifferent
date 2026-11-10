import { Component, input } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { CreatorStats } from '../models/creator-stats.model';

/** Six creator metrics rendered as premium stat cards with skeleton loading. */
@Component({
  selector: 'app-creator-stats',
  standalone: true,
  imports: [LucideDynamicIcon],
  styles: [`
    :host { display: block; }
    .grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-sm); }
    @media (min-width: 60rem) { .grid { grid-template-columns: repeat(4, minmax(0, 1fr)); gap: var(--space-md); } }
    .stat { display: flex; flex-direction: column; gap: var(--space-2xs); }
    .top { display: flex; align-items: center; justify-content: space-between; gap: var(--space-xs); font-size: var(--type-sm); font-weight: 600; }
    .n { margin: var(--space-xs) 0 0; font-size: var(--type-2xl); font-weight: 600; line-height: 1; font-variant-numeric: lining-nums tabular-nums; }
    .hint { margin: 0; font-size: var(--type-xs); color: var(--color-muted); }
    .sk { height: 7.5rem; border-radius: var(--radius-surface); background: var(--color-paper-3); }
  `],
  template: `
    <section aria-label="Creator statistics">
      <div class="grid">
        @if (loading()) {
          @for (i of [1, 2, 3, 4]; track i) {
            <div class="sk" aria-hidden="true"></div>
          }
        } @else {
          @for (stat of cards(); track stat.label) {
            <div class="ui-card stat">
              <div class="top">
                <span>{{ stat.label }}</span>
                <span class="ui-chip" aria-hidden="true"><svg [lucideIcon]="stat.icon" [size]="18"></svg></span>
              </div>
              <p class="n">{{ stat.value }}</p>
              <p class="hint">{{ stat.hint }}</p>
            </div>
          }
        }
      </div>
    </section>
  `,
})
export class CreatorStatsComponent {
  readonly stats = input.required<CreatorStats>();
  readonly loading = input(false);

  cards(): { label: string; value: string; hint: string; icon: string }[] {
    const s = this.stats();
    return [
      { label: 'Videos', value: String(s.videosCreated), hint: 'Quiz films created', icon: 'clapperboard' },
      { label: 'Shorts', value: String(s.shortsGenerated), hint: 'AI clips generated', icon: 'scissors' },
      { label: 'Published', value: String(s.published), hint: 'Marked published', icon: 'circle-check' },
      { label: 'Favorites', value: String(s.favorites), hint: 'Saved to favorites', icon: 'star' },
    ];
  }
}
