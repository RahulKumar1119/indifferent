import { Component, input } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { ActivityItem } from '../models/creator-stats.model';

/** Recent creator activity grouped as Today / Yesterday / Earlier. */
@Component({
  selector: 'app-recent-activity',
  standalone: true,
  imports: [LucideDynamicIcon],
  styles: [`
    :host { display: block; }
    .group { margin: var(--space-md) 0 var(--space-2xs); font-size: var(--type-xs); font-weight: 600; color: var(--color-muted); }
    ul { margin: 0; padding: 0; list-style: none; display: flex; flex-direction: column; gap: var(--space-2xs); }
    li { display: flex; align-items: center; gap: var(--space-sm); padding: var(--space-2xs) 0; }
    .main { flex: 1; min-width: 0; }
    .text { display: block; font-size: var(--type-sm); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .meta { display: flex; flex-wrap: wrap; gap: 0 var(--space-sm); font-size: var(--type-xs); color: var(--color-muted); }
    .empty { margin-top: var(--space-md); padding: var(--space-lg); text-align: center; border-radius: var(--radius-control); background: var(--color-paper-3); }
    .empty p { margin: 0; } .empty p + p { margin-top: var(--space-2xs); font-size: var(--type-sm); color: var(--color-ink-2); }
    .sk { height: 2.25rem; border-radius: var(--radius-control); background: var(--color-paper-3); margin-top: var(--space-sm); }
  `],
  template: `
    <section class="ui-card" aria-labelledby="activity-heading">
      <h2 id="activity-heading" class="ui-card-title">Recent activity</h2>
      @if (loading()) {
        @for (i of [1, 2, 3, 4]; track i) {
          <div class="sk" aria-hidden="true"></div>
        }
      } @else if (groups().length === 0) {
        <div class="empty">
          <p><strong>No activity yet</strong></p>
          <p>Your creations and publishes will show up here.</p>
        </div>
      } @else {
        @for (group of groups(); track group.label) {
          <p class="group">{{ group.label }}</p>
          <ul>
            @for (item of group.items; track item.id) {
              <li>
                <span class="ui-chip" aria-hidden="true"><svg [lucideIcon]="item.icon" [size]="16"></svg></span>
                <span class="main">
                  <span class="text">{{ item.text }}</span>
                  <span class="meta">
                    @if (item.detail) {
                      <span>{{ item.detail }}</span>
                    }
                    <span>{{ relativeTime(item.timestamp) }}</span>
                  </span>
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
