import { Component, computed, input } from '@angular/core';

interface Slice {
  label: string;
  n: number;
  cls: string;
  /** Percent of the total, rounded. */
  pct: number;
  /** Where the slice starts on the ring, in percent. */
  start: number;
}

/** Donut of Ready / Cutting / Retakes. The legend states every number in text. */
@Component({
  selector: 'app-status-ring',
  standalone: true,
  styles: [`
    :host { display: block; }
    .wrap { margin-top: var(--space-md); display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-lg); }
    .donut { position: relative; width: 8.5rem; height: 8.5rem; flex: none; }
    svg { width: 100%; height: 100%; transform: rotate(-90deg); }
    circle { fill: none; stroke-width: 3.6; }
    .track { stroke: var(--color-paper-3); }
    .s-done { stroke: var(--color-positive); color: var(--color-positive); }
    .s-cut { stroke: var(--color-caution); color: var(--color-caution); }
    .s-ret { stroke: var(--color-negative); color: var(--color-negative); }
    .mid { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; }
    .mid b { font-size: var(--type-xl); font-weight: 600; line-height: 1; font-variant-numeric: lining-nums tabular-nums; }
    .mid span { font-size: var(--type-xs); color: var(--color-muted); }
    ul { flex: 1; min-width: 9rem; display: flex; flex-direction: column; gap: var(--space-xs); }
    li { display: flex; align-items: center; gap: var(--space-xs); font-size: var(--type-sm); }
    .dot { width: 0.625rem; height: 0.625rem; border-radius: 50%; background: currentColor; flex: none; }
    .name { flex: 1; color: var(--color-ink-2); }
    .v { font-weight: 600; color: var(--color-ink); font-variant-numeric: lining-nums tabular-nums; }
    .p { width: 3rem; text-align: right; color: var(--color-muted); font-variant-numeric: lining-nums tabular-nums; }
  `],
  template: `
    <div class="wrap">
      <div class="donut" aria-hidden="true">
        <svg viewBox="0 0 42 42">
          <circle class="track" cx="21" cy="21" r="15.9155" />
          @for (s of slices(); track s.label) {
            @if (s.n > 0) {
              <circle [class]="s.cls" cx="21" cy="21" r="15.9155"
                [attr.stroke-dasharray]="share(s) + ' ' + (100 - share(s))"
                [attr.stroke-dashoffset]="-s.start" />
            }
          }
        </svg>
        <div class="mid"><b>{{ total() }}</b><span>total</span></div>
      </div>
      <ul aria-label="Work by status">
        @for (s of slices(); track s.label) {
          <li>
            <span class="dot" [class]="s.cls" aria-hidden="true"></span>
            <span class="name">{{ s.label }}</span>
            <span class="v" [attr.data-testid]="'ring-' + s.label">{{ s.n }}</span>
            <span class="p">{{ s.pct }}%</span>
          </li>
        }
      </ul>
    </div>
  `,
})
export class StatusRingComponent {
  readonly done = input(0);
  readonly cutting = input(0);
  readonly retakes = input(0);
  readonly total = computed(() => this.done() + this.cutting() + this.retakes());
  readonly slices = computed<Slice[]>(() => {
    const t = this.total() || 1;
    const raw = [
      { label: 'Ready', n: this.done(), cls: 's-done' },
      { label: 'Cutting', n: this.cutting(), cls: 's-cut' },
      { label: 'Retakes', n: this.retakes(), cls: 's-ret' },
    ];
    let start = 0;
    return raw.map((r) => {
      const exact = (r.n / t) * 100;
      const s = { ...r, pct: Math.round(exact), start };
      start += exact;
      return s;
    });
  });

  /** Exact share of the ring, so slices meet without gaps from rounding. */
  share(s: Slice): number {
    return (s.n / (this.total() || 1)) * 100;
  }
}
