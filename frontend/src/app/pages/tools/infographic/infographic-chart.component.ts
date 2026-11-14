import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { DataItem, formatPercent, niceMax, readItems, shares, sliceColors } from './chart-data';
import { InfographicDoc } from './infographic.model';
import { themeById } from './themes';

/** The drawing area of the line chart is 1000 units wide and this tall, by size, so circles stay round and lines even. */
const LINE_HEIGHT = { portrait: 760, square: 480, landscape: 330 } as const;
const PLOT_TOP = 18; // percent left above the highest value, for its label
const PLOT_BOTTOM = 4; // percent below zero

/**
 * The statistics layouts of the artboard (#217): bars, a line, a donut and big
 * numbers. Shapes are SVG or CSS and every word is ordinary HTML text, so the
 * fonts, the right-to-left mirror and the export (which reads the text to pick
 * font files) work the same as for the other templates. It sits inside the
 * artboard and uses its colour and size variables (--k and the rest).
 */
@Component({
  selector: 'app-infographic-chart',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: [`
    :host { display: block; flex: 1 0 auto; }
    * { box-sizing: border-box; margin: 0; padding: 0; letter-spacing: 0; text-transform: none; overflow-wrap: break-word; }
    h2 { overflow-wrap: normal; font-size: calc(var(--h2) * 0.8 * var(--k)); line-height: 1.25; font-weight: 700; text-wrap: balance; }
    :host-context([data-break]) h2 { overflow-wrap: anywhere; }
    ol, ul { list-style: none; }
    .val { font-size: calc(var(--h2) * 0.8 * var(--k)); font-weight: 700; color: var(--ink); white-space: nowrap; }
    /* bars */
    .bars { display: grid; gap: calc(var(--gap) * 0.7 * var(--k)); align-content: start; }
    .rh { display: flex; justify-content: space-between; align-items: baseline; gap: calc(24px * var(--k)); margin-block-end: calc(8px * var(--k)); }
    .track { height: calc(34px * var(--k)); border-radius: 999px; background: var(--surface); border: 2px solid var(--rule); overflow: hidden; }
    .fill { height: 100%; border-radius: 999px; background: var(--accent); }
    /* line */
    .chart { width: calc(50% + 50% * (var(--k) - 0.58) / 0.82); min-width: 60%; max-width: 100%; margin-inline: auto; }
    .plot { position: relative; }
    .plot svg { display: block; width: 100%; height: auto; }
    .pv { position: absolute; transform: translate(-50%, -135%); font-size: calc(var(--small) * var(--k)); font-weight: 700; color: var(--ink); white-space: nowrap; }
    .xs { display: grid; gap: calc(6px * var(--k)); margin-block-start: calc(10px * var(--k)); }
    .xs span { text-align: center; font-size: calc(var(--small) * var(--k)); color: var(--ink2); }
    /* donut */
    .dn { display: flex; flex-direction: column; align-items: center; gap: calc(var(--gap) * var(--k)); }
    :host-context(.f-landscape) .dn { flex-direction: row; justify-content: center; }
    .donut { flex: none; width: calc(500px * var(--k)); max-width: 100%; }
    .donut svg { display: block; width: 100%; height: auto; }
    .legend { display: grid; gap: calc(14px * var(--k)); width: 100%; max-width: calc(760px * var(--k)); }
    .lg { display: flex; align-items: center; gap: calc(16px * var(--k)); font-size: calc(var(--body) * var(--k)); padding: calc(10px * var(--k)) calc(18px * var(--k)); border-radius: calc(16px * var(--k)); background: var(--surface); border: 2px solid var(--rule); }
    .sw { flex: none; width: calc(30px * var(--k)); height: calc(30px * var(--k)); border-radius: 8px; }
    .lg .nm { flex: 1; min-width: 0; font-weight: 700; }
    .lg .pc { color: var(--ink2); white-space: nowrap; }
    /* big numbers */
    .nums { display: grid; gap: calc(var(--gap) * 0.7 * var(--k)); grid-template-columns: repeat(var(--cols), minmax(0, 1fr)); align-content: start; }
    .nums.odd > li:last-child { grid-column: 1 / -1; }
    .nc { background: var(--surface); border: 2px solid var(--rule); border-block-start: calc(10px * var(--k)) solid var(--accent); border-radius: calc(24px * var(--k)); padding: calc(var(--gap) * 0.8 * var(--k)); }
    .big { font-size: calc(var(--num) * 1.1 * var(--k)); line-height: 1.15; font-weight: 700; color: var(--accent); overflow-wrap: normal; }
    :host-context([data-break]) .big { overflow-wrap: anywhere; }
    .nc h2 { margin-block-start: calc(6px * var(--k)); }
    .nc .note { margin-block-start: calc(6px * var(--k)); font-size: calc(var(--body) * var(--k)); color: var(--ink2); }
  `],
  template: `
    @switch (layout()) {
      @case ('bars') {
        <ol class="bars">
          @for (it of items(); track it.id) {
            <li>
              <div class="rh">
                <h2 dir="auto">{{ it.label }}</h2>
                <span class="val" dir="auto">{{ it.text }}</span>
              </div>
              <div class="track" aria-hidden="true"><div class="fill" [style.width.%]="barPercent(it)"></div></div>
            </li>
          }
        </ol>
      }
      @case ('line') {
        <div class="chart">
          <div class="plot">
            <svg [attr.viewBox]="'0 0 1000 ' + lineHeight()" aria-hidden="true">
              @for (y of gridYs(); track $index) {
                <line x1="0" x2="1000" [attr.y1]="y" [attr.y2]="y" stroke="var(--rule)" stroke-width="2" />
              }
              <polyline [attr.points]="polyline()" fill="none" stroke="var(--accent)" stroke-width="6" stroke-linejoin="round" stroke-linecap="round" />
              @for (p of points(); track p.id) {
                <circle [attr.cx]="p.x * 10" [attr.cy]="p.y * lineHeight() / 100" r="10" fill="var(--accent)" stroke="var(--bg)" stroke-width="4" />
              }
            </svg>
            @for (p of points(); track p.id) {
              <span class="pv" dir="auto" [style.left.%]="p.x" [style.top.%]="p.y">{{ p.text }}</span>
            }
          </div>
          <div class="xs" [style.grid-template-columns]="'repeat(' + items().length + ', minmax(0, 1fr))'">
            @for (it of items(); track it.id) {
              <span dir="auto">{{ it.label }}</span>
            }
          </div>
        </div>
      }
      @case ('donut') {
        <div class="dn">
          <div class="donut">
            <svg viewBox="0 0 100 100" aria-hidden="true" [style.transform]="rtl() ? 'scaleX(-1)' : null">
              <circle cx="50" cy="50" r="38" fill="none" stroke="var(--rule)" stroke-width="22" />
              @for (s of slices(); track s.id) {
                <circle cx="50" cy="50" r="38" fill="none" [attr.stroke]="s.color" stroke-width="22" pathLength="100"
                  [attr.stroke-dasharray]="s.dash + ' ' + (100 - s.dash)" [attr.stroke-dashoffset]="-s.start" transform="rotate(-90 50 50)" />
              }
            </svg>
          </div>
          <ul class="legend">
            @for (s of slices(); track s.id) {
              <li class="lg">
                <span class="sw" [style.background]="s.color" aria-hidden="true"></span>
                <span class="nm" dir="auto">{{ s.label }}</span>
                <span class="val" dir="auto">{{ s.text }}</span>
                <span class="pc">{{ s.percent }}</span>
              </li>
            }
          </ul>
        </div>
      }
      @default {
        <ul class="nums" [class.odd]="items().length % 2 === 1 && cols() === 2" [style.--cols]="cols()">
          @for (it of items(); track it.id) {
            <li class="nc">
              <p class="big" dir="auto">{{ it.text }}</p>
              <h2 dir="auto">{{ it.label }}</h2>
              @if (it.note) {
                <p class="note" dir="auto">{{ it.note }}</p>
              }
            </li>
          }
        </ul>
      }
    }
  `,
})
export class InfographicChartComponent {
  readonly doc = input.required<InfographicDoc>();
  readonly layout = input.required<'bars' | 'line' | 'donut' | 'numbers'>();
  /** True when the artboard reads right to left, so the line and the donut run from the right. */
  readonly rtl = input(false);

  protected readonly items = computed<DataItem[]>(() => readItems(this.doc().sections));
  private readonly max = computed(() => Math.max(0, ...this.items().map((i) => i.value)));

  protected barPercent(it: DataItem): number {
    return this.max() > 0 ? (it.value / this.max()) * 100 : 0;
  }

  // --- line

  protected readonly lineHeight = computed(() => LINE_HEIGHT[this.doc().format]);
  private readonly top = computed(() => niceMax(this.max()));
  /** x and y in percent of the drawing area. */
  protected readonly points = computed(() => {
    const items = this.items();
    const top = this.top();
    return items.map((it, i) => {
      const fx = ((i + 0.5) / items.length) * 100;
      return {
        id: it.id,
        text: it.text,
        x: this.rtl() ? 100 - fx : fx,
        y: 100 - (PLOT_BOTTOM + (it.value / top) * (100 - PLOT_TOP - PLOT_BOTTOM)),
      };
    });
  });
  protected readonly polyline = computed(() => this.points().map((p) => `${p.x * 10},${(p.y * this.lineHeight()) / 100}`).join(' '));
  protected readonly gridYs = computed(() =>
    [0, 0.25, 0.5, 0.75, 1].map((f) => ((100 - (PLOT_BOTTOM + f * (100 - PLOT_TOP - PLOT_BOTTOM))) * this.lineHeight()) / 100),
  );

  // --- donut

  protected readonly slices = computed(() => {
    const items = this.items();
    const colors = sliceColors(themeById(this.doc().themeId), items.length);
    const parts = shares(items.map((i) => i.value));
    let start = 0;
    return items.map((it, i) => {
      const part = parts[i];
      const slice = {
        id: it.id,
        label: it.label,
        text: it.text,
        color: colors[i % colors.length],
        percent: formatPercent(part),
        start,
        // A thin gap between slices, so neighbours with close colours stay apart.
        dash: part > 1 ? part - 0.8 : part,
      };
      start += part;
      return slice;
    });
  });

  // --- big numbers

  protected readonly cols = computed(() => {
    const n = this.items().length;
    if (this.doc().format === 'landscape') return n <= 2 ? n : n === 4 ? 2 : 3;
    return n === 1 ? 1 : 2;
  });
}
