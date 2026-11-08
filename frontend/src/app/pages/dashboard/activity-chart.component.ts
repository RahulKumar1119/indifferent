import { Component, computed, input } from '@angular/core';
import { DayBucket } from './activity';

/** Bar chart of work per day. Bars are decoration; the table under them carries the numbers for screen readers. */
@Component({
  selector: 'app-activity-chart',
  standalone: true,
  styles: [`
    :host { display: block; }
    .total { font-size: var(--type-sm); color: var(--color-muted); }
    .bars { margin-top: var(--space-md); height: 9rem; display: grid; grid-template-columns: repeat(14, 1fr); gap: var(--space-2xs); align-items: end; }
    .col { height: 100%; display: flex; flex-direction: column; justify-content: flex-end; align-items: center; gap: var(--space-3xs); }
    .n { font-size: 0.6875rem; color: var(--color-ink-2); font-variant-numeric: lining-nums tabular-nums; }
    .bar { width: 100%; max-width: 1.75rem; border-radius: 4px 4px 2px 2px; background: color-mix(in oklab, var(--color-accent) 22%, var(--card)); min-height: 3px; }
    .bar.on { background: var(--color-accent); }
    .bar.today { outline: 2px solid var(--color-accent-text); outline-offset: 1px; }
    .axis { margin-top: var(--space-xs); display: flex; justify-content: space-between; font-size: var(--type-xs); color: var(--color-muted); }
    .empty { margin-top: var(--space-md); padding: var(--space-lg); border-radius: var(--radius-control); background: var(--color-paper-3); color: var(--color-ink-2); font-size: var(--type-sm); }
  `],
  template: `
    @if (sum() === 0) {
      <p class="empty" data-testid="activity-empty">No new work in the last 14 days. Make something and it will show here.</p>
    } @else {
      <p class="total">{{ sum() }} {{ sum() === 1 ? 'piece' : 'pieces' }} of work{{ partial() ? ', from the work that loaded' : '' }}</p>
      <div class="bars" aria-hidden="true">
        @for (d of days(); track d.key; let last = $last) {
          <div class="col" [title]="label(d) + ': ' + d.count">
            @if (d.count > 0) {
              <span class="n">{{ d.count }}</span>
            }
            <div class="bar" [class.on]="d.count > 0" [class.today]="last" [style.height.%]="d.count ? (d.count / max()) * 82 : 0"></div>
          </div>
        }
      </div>
      <div class="axis" aria-hidden="true">
        <span>{{ label(days()[0]) }}</span>
        <span>Today</span>
      </div>
      <table class="sr-only">
        <caption>Work per day, last 14 days</caption>
        <tr><th scope="col">Day</th><th scope="col">Pieces of work</th></tr>
        @for (d of days(); track d.key) {
          <tr><td>{{ label(d) }}</td><td>{{ d.count }}</td></tr>
        }
      </table>
    }
  `,
})
export class ActivityChartComponent {
  readonly days = input.required<DayBucket[]>();
  /** Some sources were capped or failed, so the chart may be short. */
  readonly partial = input(false);
  readonly sum = computed(() => this.days().reduce((s, d) => s + d.count, 0));
  readonly max = computed(() => Math.max(1, ...this.days().map((d) => d.count)));

  label(d: DayBucket): string {
    return d.date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
  }
}
