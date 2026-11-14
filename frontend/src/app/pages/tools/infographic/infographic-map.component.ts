import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { InfographicDoc } from './infographic.model';
import { MapLayout } from './infographic-diagram.component';

const CYCLE_RX = 35;
const CYCLE_RY = 36;

/** Where the nodes of a cycle sit, as percent of its box: the first at the top, then round the clock (or the other way when mirrored). */
export function cyclePositions(n: number, rtl: boolean): { x: number; y: number }[] {
  return Array.from({ length: n }, (_, i) => {
    const angle = (i / n) * 2 * Math.PI;
    const dx = Math.sin(angle) * CYCLE_RX;
    return { x: 50 + (rtl ? -dx : dx), y: 50 - Math.cos(angle) * CYCLE_RY };
  });
}

/**
 * Where the arrowheads of a cycle sit (the middle of each stretch between two
 * nodes) and which way they point, in degrees with 0 as "right" and 90 as
 * "down". `aspect` is the box's width over its height, so the direction is
 * right for a wide box too. Positions are in percent of the box.
 */
export function cycleArrows(n: number, rtl: boolean, aspect = 1): { x: number; y: number; rot: number }[] {
  const m = rtl ? -1 : 1;
  return Array.from({ length: n }, (_, i) => {
    const mid = ((i + 0.5) / n) * 2 * Math.PI;
    const rot = (Math.atan2(CYCLE_RY * Math.sin(mid), m * CYCLE_RX * aspect * Math.cos(mid)) * 180) / Math.PI;
    return { x: 50 + m * Math.sin(mid) * CYCLE_RX, y: 50 - Math.cos(mid) * CYCLE_RY, rot };
  });
}

/**
 * The flow chart, the cycle, the mind map and the tree (#217). The nodes of a
 * flow chart and a cycle are marked `data-node`, and the artboard checks that
 * two of them never overlap. The mind map and the tree draw the title as their
 * centre or top.
 */
@Component({
  selector: 'app-infographic-map',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: [`
    :host { display: block; flex: 1 0 auto; }
    * { box-sizing: border-box; margin: 0; padding: 0; letter-spacing: 0; text-transform: none; overflow-wrap: break-word; }
    ol, ul { list-style: none; }
    h2 { font-size: calc(var(--h2) * var(--k)); line-height: 1.25; font-weight: 700; overflow-wrap: normal; text-wrap: balance; }
    :host-context([data-break]) h2 { overflow-wrap: anywhere; }
    p.body { margin-block-start: calc(8px * var(--k)); font-size: calc(var(--body) * var(--k)); color: var(--ink2); white-space: pre-line; }
    .card { background: var(--surface); border: 2px solid var(--rule); border-radius: calc(24px * var(--k)); padding: calc(var(--gap) * 0.8 * var(--k)); }
    .stack { display: grid; gap: calc(var(--gap) * 0.7 * var(--k)); align-content: start; }
    .badge { flex: none; display: grid; place-items: center; width: calc(var(--num) * var(--k)); height: calc(var(--num) * var(--k)); border-radius: 50%; background: var(--accent); color: var(--accent-ink); font-size: calc(var(--num) * 0.5 * var(--k)); font-weight: 700; line-height: 1; }
    .row { display: flex; align-items: flex-start; gap: calc(var(--gap) * 0.7 * var(--k)); }
    .txt { flex: 1; min-width: 0; }
    .cap { margin-block-end: calc(4px * var(--k)); font-size: calc(var(--small) * var(--k)); font-weight: 700; color: var(--ink2); }
    .pill { display: inline-block; margin-block-end: calc(6px * var(--k)); padding: calc(2px * var(--k)) calc(14px * var(--k)); border-radius: 999px; background: var(--accent); color: var(--accent-ink); font-size: calc(var(--small) * var(--k)); font-weight: 700; }

    /* flow chart */
    .fl { display: grid; gap: calc(var(--gap) * 0.9 * var(--k)); grid-template-columns: repeat(var(--cols), minmax(0, 1fr)); align-content: start; }
    .fl > li { position: relative; }
    .fl > li:not(.end)::after { content: ''; position: absolute; inset-inline: 0; margin-inline: auto; width: calc(34px * var(--k)); height: calc(26px * var(--k)); background: var(--accent); inset-block-start: 100%; margin-block-start: calc(var(--gap) * 0.45 * var(--k) - 13px * var(--k)); clip-path: polygon(0 0, 100% 0, 50% 100%); }
    .fl.wide > li:not(.end)::after { inset-inline: auto; inset-inline-start: 100%; margin-inline: calc(var(--gap) * 0.45 * var(--k) - 13px * var(--k)) 0; inset-block-start: 50%; margin-block-start: calc(-17px * var(--k)); width: calc(26px * var(--k)); height: calc(34px * var(--k)); clip-path: polygon(0 0, 100% 50%, 0 100%); }
    :host-context([dir=rtl]) .fl.wide > li:not(.end)::after { transform: scaleX(-1); }
    .fl .card { height: 100%; }

    /* cycle */
    .cyc-wrap { display: grid; justify-items: center; }
    .cyc { position: relative; width: min(100%, var(--cyc)); }
    .cyc svg { position: absolute; inset: 0; width: 100%; height: 100%; }
    .node { position: absolute; width: var(--nw); transform: translate(-50%, -50%); text-align: center; padding: calc(var(--gap) * 0.45 * var(--k)); background: var(--surface); border: 2px solid var(--rule); border-block-end: calc(8px * var(--k)) solid var(--accent); border-radius: calc(20px * var(--k)); }
    .node h2 { font-size: calc(var(--h2) * 0.85 * var(--k)); }

    /* mind map and tree: the title is the centre or the top */
    .core { justify-self: center; text-align: center; max-width: 86%; padding: calc(var(--gap) * 0.7 * var(--k)) calc(var(--gap) * 1.3 * var(--k)); border-radius: 999px; background: var(--accent); color: var(--accent-ink); }
    .core .title { font-size: calc(var(--h1) * 0.75 * var(--k)); line-height: 1.2; font-weight: 700; overflow-wrap: normal; text-wrap: balance; }
    :host-context([data-break]) .core .title { overflow-wrap: anywhere; }
    .mm, .tr { display: grid; gap: calc(var(--gap) * 0.9 * var(--k)); align-content: start; }
    .mm-grid { position: relative; display: grid; gap: calc(var(--gap) * 0.7 * var(--k)) calc(var(--gap) * 1.2 * var(--k)); grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .mm-grid::before { content: ''; position: absolute; inset-block: calc(var(--gap) * -0.9 * var(--k)) calc(var(--gap) * 0.35 * var(--k)); inset-inline-start: 50%; width: calc(6px * var(--k)); margin-inline-start: calc(-3px * var(--k)); background: var(--accent); border-radius: 3px; opacity: 0.55; }
    .mm-grid > li { position: relative; border-inline-start: calc(10px * var(--k)) solid var(--accent); }
    .mm-grid > li::after { content: ''; position: absolute; inset-block-start: calc(var(--h2) * 0.7 * var(--k)); width: calc(var(--gap) * 0.6 * var(--k)); height: calc(6px * var(--k)); background: var(--accent); opacity: 0.55; }
    .mm-grid > li:nth-child(odd)::after { inset-inline-end: calc(var(--gap) * -0.6 * var(--k)); }
    .mm-grid > li:nth-child(even)::after { inset-inline-start: calc(var(--gap) * -0.6 * var(--k)); }
    .mm-grid > li:last-child:nth-child(odd), .tr-cols > li:last-child:nth-child(odd) { grid-column: 1 / -1; }
    .mm-grid > li:last-child:nth-child(odd)::after { display: none; }
    .core.top { border-radius: calc(24px * var(--k)); position: relative; }
    .core.top::after { content: ''; position: absolute; inset-inline: 0; margin-inline: auto; inset-block-start: 100%; width: calc(6px * var(--k)); height: calc(var(--gap) * 0.9 * var(--k)); background: var(--accent); opacity: 0.55; }
    .tr-cols { display: grid; gap: calc(var(--gap) * 0.7 * var(--k)); grid-template-columns: repeat(var(--cols2), minmax(0, 1fr)); padding-block-start: calc(var(--gap) * 0.2 * var(--k)); }
    .tr-cols > li { border-block-start: calc(10px * var(--k)) solid var(--accent); }
    .pts.dots { padding: calc(var(--gap) * 0.35 * var(--k)) 0 0; gap: calc(10px * var(--k)); }
    .pts.dots li::before { content: ''; flex: none; width: calc(var(--body) * 0.3 * var(--k)); height: calc(var(--body) * 0.3 * var(--k)); margin-block-start: calc(var(--body) * 0.5 * var(--k)); border-radius: 50%; background: var(--accent); }
  `],
  template: `
    @switch (layout()) {
      @case ('flow') {
        <ol class="fl" [class.wide]="flowCols() > 1" [style.--cols]="flowCols()">
          @for (s of doc().sections; track s.id; let i = $index) {
            <li [class.end]="i === last() || (flowCols() > 1 && (i + 1) % flowCols() === 0)">
              <div class="card row" data-node>
                <span class="badge" aria-hidden="true">{{ i + 1 }}</span>
                <div class="txt">
                  <h2 dir="auto">{{ s.heading }}</h2>
                  @if (s.body) {
                    <p class="body" dir="auto">{{ s.body }}</p>
                  }
                </div>
              </div>
            </li>
          }
        </ol>
      }
      @case ('cycle') {
        <div class="cyc-wrap">
          <div class="cyc" [style.--cyc]="cycleSize()" [style.aspect-ratio]="aspect()" [style.--nw]="wide() ? '23%' : '31%'">
            <svg [attr.viewBox]="'0 0 ' + aspect() * 100 + ' 100'" aria-hidden="true">
              <ellipse [attr.cx]="aspect() * 50" cy="50" [attr.rx]="aspect() * 35" ry="36" fill="none" stroke="var(--rule)" stroke-width="1.6" />
              @for (a of arrows(); track $index) {
                <polygon points="-2.6,-2.2 2.6,0 -2.6,2.2" fill="var(--accent)" [attr.transform]="'translate(' + a.x * aspect() + ' ' + a.y + ') rotate(' + a.rot + ')'" />
              }
            </svg>
            @for (s of doc().sections; track s.id; let i = $index) {
              <div class="node" data-node [style.left.%]="nodes()[i].x" [style.top.%]="nodes()[i].y">
                <h2 dir="auto">{{ s.heading }}</h2>
              </div>
            }
          </div>
        </div>
      }
      @case ('mindmap') {
        <div class="mm">
          <div class="core"><p class="title" dir="auto">{{ doc().title }}</p></div>
          <ul class="mm-grid">
            @for (s of doc().sections; track s.id) {
              <li class="card">
                <h2 dir="auto">{{ s.heading }}</h2>
                @if (s.body) {
                  <p class="body" dir="auto">{{ s.body }}</p>
                }
              </li>
            }
          </ul>
        </div>
      }
      @default {
        <div class="tr">
          <div class="core top"><p class="title" dir="auto">{{ doc().title }}</p></div>
          <ul class="tr-cols" [style.--cols2]="wide() ? doc().sections.length : 2">
            @for (s of doc().sections; track s.id) {
              <li class="card">
                <h2 dir="auto">{{ s.heading }}</h2>
                <ul class="pts dots">
                  @for (line of lines(s.body); track $index) {
                    <li><span dir="auto">{{ line }}</span></li>
                  }
                </ul>
              </li>
            }
          </ul>
        </div>
      }
    }
  `,
})
export class InfographicMapComponent {
  readonly doc = input.required<InfographicDoc>();
  readonly layout = input.required<MapLayout>();
  readonly rtl = input(false);

  protected readonly wide = computed(() => this.doc().format === 'landscape');
  protected readonly last = computed(() => this.doc().sections.length - 1);
  protected readonly nodes = computed(() => cyclePositions(this.doc().sections.length, this.rtl()));
  protected readonly aspect = computed(() => (this.wide() ? 1.5 : 1));
  protected readonly arrows = computed(() => cycleArrows(this.doc().sections.length, this.rtl(), this.aspect()));
  /** Columns of the flow chart: one tall column, or rows on a landscape artboard. */
  protected readonly flowCols = computed(() => {
    const n = this.doc().sections.length;
    return this.wide() ? (n <= 3 ? n : n === 4 ? 2 : 3) : 1;
  });
  /** The side of the cycle, fixed by the size of the artboard and not by the text scale, so that shrinking the text can make room. */
  protected readonly cycleSize = computed(() => ({ portrait: '880px', square: '640px', landscape: '1020px' })[this.doc().format]);

  protected lines(body: string): string[] {
    return body.split('\n').map((l) => l.trim()).filter(Boolean);
  }
}
