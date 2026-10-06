import { Component, input } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { ActivityItem } from '../models/creator-stats.model';

/** Recent creator activity grouped as Today / Yesterday / Earlier. */
@Component({
  selector: 'app-recent-activity',
  standalone: true,
  imports: [LucideDynamicIcon],
  template: `
    <section class="glass-card p-5 md:p-7" aria-label="Recent activity">
      <h2 class="text-lg font-semibold flex items-center gap-2">
        <svg lucideIcon="clock" [size]="19" class="text-[hsl(var(--primary))]" aria-hidden="true"></svg>
        Recent Activity
      </h2>

      @if (loading()) {
        <div class="mt-4 space-y-3">
          @for (i of [1, 2, 3, 4]; track i) {
            <div class="flex items-center gap-3">
              <div class="skeleton w-9 h-9 rounded-full shrink-0"></div>
              <div class="flex-1 space-y-2">
                <div class="skeleton h-3.5 w-3/4"></div>
                <div class="skeleton h-3 w-1/3"></div>
              </div>
            </div>
          }
        </div>
      } @else if (groups().length === 0) {
        <div class="mt-4 rounded-2xl border border-dashed border-[hsl(var(--border))] px-6 py-8 text-center">
          <p class="font-medium text-[14.5px]">No activity yet</p>
          <p class="mt-1 text-[13px] text-[hsl(var(--muted-foreground))]">Your creations and publishes will show up here.</p>
        </div>
      } @else {
        @for (group of groups(); track group.label) {
          <p class="mt-5 mb-2 first:mt-4 text-[11.5px] uppercase tracking-[0.14em] text-[hsl(var(--muted-foreground))]">
            {{ group.label }}
          </p>
          <ul class="space-y-1">
            @for (item of group.items; track item.id) {
              <li class="flex items-center gap-3 rounded-xl px-2 py-2 hover:bg-white/[0.03] transition-colors">
                <span
                  class="w-9 h-9 rounded-full bg-[hsl(var(--primary))]/10 text-[hsl(var(--primary))] flex items-center justify-center shrink-0"
                  aria-hidden="true"
                >
                  <svg lucideIcon="{{ item.icon }}" [size]="16"></svg>
                </span>
                <span class="flex-1 min-w-0">
                  <span class="block text-[13.5px] truncate">
                    {{ item.text }}
                    @if (item.detail) {
                      <span class="text-[hsl(var(--muted-foreground))]"> · {{ item.detail }}</span>
                    }
                  </span>
                  <span class="block text-[12px] text-[hsl(var(--muted-foreground))]">{{ relativeTime(item.timestamp) }}</span>
                </span>
              </li>
            }
          </ul>
        }
      }
    </section>
  `,
})
export class RecentActivityComponent {
  readonly items = input.required<ActivityItem[]>();
  readonly loading = input(false);

  groups(): { label: string; items: ActivityItem[] }[] {
    const order = ['Today', 'Yesterday', 'Earlier'];
    return order
      .map((label) => ({ label, items: this.items().filter((i) => i.group === label) }))
      .filter((g) => g.items.length > 0);
  }

  relativeTime(iso: string): string {
    const t = new Date(iso).getTime();
    if (isNaN(t)) return '';
    const mins = Math.max(0, Math.floor((Date.now() - t) / 60000));
    if (mins < 1) return 'just now';
    if (mins < 60) return `${mins}m ago`;
    const hours = Math.floor(mins / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    return days === 1 ? 'yesterday' : `${days}d ago`;
  }
}
