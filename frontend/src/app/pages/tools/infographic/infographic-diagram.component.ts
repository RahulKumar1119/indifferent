import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { InfographicDoc } from './infographic.model';
import { Layout, TEMPLATES, templateById } from './templates';

/** The layouts of {@link InfographicDiagramComponent} and of {@link InfographicMapComponent}. */
export type ListLayout = Extract<Layout, 'qa' | 'stairs' | 'checklist' | 'proscons' | 'versus' | 'zigzag' | 'milestones'>;
export type MapLayout = Extract<Layout, 'flow' | 'cycle' | 'mindmap' | 'tree'>;
export type DiagramLayout = ListLayout | MapLayout;

export const LIST_LAYOUTS: readonly Layout[] = ['qa', 'stairs', 'checklist', 'proscons', 'versus', 'zigzag', 'milestones'];
export const MAP_LAYOUTS: readonly Layout[] = ['flow', 'cycle', 'mindmap', 'tree'];
export const DIAGRAM_LAYOUTS: readonly Layout[] = [...LIST_LAYOUTS, ...MAP_LAYOUTS];

/**
 * Question cards, a staircase, a checklist, pros and cons, side by side and the
 * two timelines (#217). Boxes and words are HTML, so the fonts, the export and
 * the right-to-left mirror work as for the other templates. Symbols are CSS
 * shapes, not text characters, so no glyph can be missing from the font files
 * the export embeds. The flow chart, cycle, mind map and tree are in
 * infographic-map.component.ts (each component's CSS has a size limit).
 */
@Component({
  selector: 'app-infographic-diagram',
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

    /* question cards */
    .qa-grid { display: grid; gap: calc(var(--gap) * 0.7 * var(--k)); grid-template-columns: repeat(var(--cols), minmax(0, 1fr)); align-content: start; }
    .qa { border-inline-start: calc(12px * var(--k)) solid var(--accent); }

    /* staircase */
    .stair { display: flex; align-items: flex-start; gap: calc(var(--gap) * 0.7 * var(--k)); margin-inline-start: calc(var(--i) * 56px * var(--k)); background: var(--surface); border: 2px solid var(--rule); border-block-end: calc(10px * var(--k)) solid var(--accent); border-radius: calc(20px * var(--k)); padding: calc(var(--gap) * 0.7 * var(--k)); }

    /* checklist */
    .check { flex: none; position: relative; width: calc(var(--num) * 0.8 * var(--k)); height: calc(var(--num) * 0.8 * var(--k)); border: calc(5px * var(--k)) solid var(--accent); border-radius: calc(14px * var(--k)); }
    .check::after { content: ''; position: absolute; inset-inline-start: 30%; inset-block-start: 8%; width: 28%; height: 56%; border: solid var(--accent); border-width: 0 calc(7px * var(--k)) calc(7px * var(--k)) 0; transform: rotate(45deg); }
    .tick { padding-block: calc(var(--gap) * 0.45 * var(--k)); border-block-end: 2px solid var(--rule); }
    .tick:last-child { border-block-end: 0; }

    /* pros and cons, side by side */
    .two { display: grid; gap: calc(var(--gap) * 0.7 * var(--k)); grid-template-columns: repeat(var(--cols2), minmax(0, 1fr)); align-content: start; }
    .col { display: flex; flex-direction: column; padding: 0; overflow: hidden; }
    .band { display: flex; align-items: center; gap: calc(16px * var(--k)); padding: calc(var(--gap) * 0.6 * var(--k)) calc(var(--gap) * 0.8 * var(--k)); background: var(--accent); color: var(--accent-ink); }
    .col:nth-child(2) .band { background: var(--ink); color: var(--bg); }
    .sym { flex: none; width: calc(var(--small) * 1.4 * var(--k)); height: calc(var(--small) * 1.4 * var(--k)); border-radius: 50%; border: calc(4px * var(--k)) solid currentColor; color: inherit;
      background: linear-gradient(currentColor, currentColor) center / 56% calc(4px * var(--k)) no-repeat, linear-gradient(currentColor, currentColor) center / calc(4px * var(--k)) 56% no-repeat; }
    .sym.minus { background: linear-gradient(currentColor, currentColor) center / 56% calc(4px * var(--k)) no-repeat; }
    .pts { padding: calc(var(--gap) * 0.8 * var(--k)); display: grid; gap: calc(14px * var(--k)); font-size: calc(var(--body) * var(--k)); color: var(--ink2); }
    .pts li { display: flex; align-items: flex-start; gap: calc(16px * var(--k)); }
    .pts .sym { margin-block-start: calc(var(--body) * 0.28 * var(--k)); width: calc(var(--body) * 0.8 * var(--k)); height: calc(var(--body) * 0.8 * var(--k)); border-width: calc(3px * var(--k)); color: var(--accent); }
    .col:nth-child(2) .pts .sym { color: var(--ink); }
    .vs { position: relative; }
    .vs-mark { position: absolute; inset-block-start: 50%; inset-inline-start: 50%; width: calc(var(--num) * 0.9 * var(--k)); height: calc(var(--num) * 0.9 * var(--k)); transform: translate(-50%, -50%); border-radius: 50%; background: var(--ink); border: calc(6px * var(--k)) solid var(--bg); }
    :host-context([dir=rtl]) .vs-mark { transform: translate(50%, -50%); }
    .vs-mark::after { content: ''; position: absolute; inset: 22% 46%; background: var(--bg); transform: rotate(35deg) scaleY(1.9); border-radius: 2px; }
    .vs .col .card-body { padding: calc(var(--gap) * 0.9 * var(--k)); }

    /* timelines */
    .zz { position: relative; display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); column-gap: calc(var(--gap) * 1.4 * var(--k)); row-gap: calc(var(--gap) * 0.5 * var(--k)); align-content: start; }
    .zz::before { content: ''; position: absolute; inset-block: 0; inset-inline-start: 50%; width: calc(6px * var(--k)); margin-inline-start: calc(-3px * var(--k)); background: var(--rule); border-radius: 3px; }
    .zz > li { position: relative; }
    .zz > li:nth-child(odd) { grid-column: 1; text-align: end; }
    .zz > li:nth-child(even) { grid-column: 2; }
    .zz > li::after { content: ''; position: absolute; inset-block-start: calc(14px * var(--k)); width: calc(30px * var(--k)); height: calc(30px * var(--k)); border-radius: 50%; background: var(--accent); border: calc(6px * var(--k)) solid var(--bg); }
    .zz > li:nth-child(odd)::after { inset-inline-end: calc(var(--gap) * -0.7 * var(--k) - 15px * var(--k)); }
    .zz > li:nth-child(even)::after { inset-inline-start: calc(var(--gap) * -0.7 * var(--k) - 15px * var(--k)); }
    .ms { position: relative; display: grid; gap: calc(var(--gap) * 0.8 * var(--k)); grid-template-columns: repeat(var(--cols), minmax(0, 1fr)); align-content: start; padding-block-start: calc(26px * var(--k)); row-gap: calc(var(--gap) * 0.8 * var(--k) + 32px * var(--k)); }
    .ms > li { position: relative; }
    .ms > li::before { content: ''; position: absolute; inset-block-start: calc(-26px * var(--k)); inset-inline: calc(var(--gap) * -0.4 * var(--k)); height: calc(8px * var(--k)); background: var(--rule); }
    .ms > li::after { content: ''; position: absolute; inset-block-start: calc(-38px * var(--k)); inset-inline-start: calc(10px * var(--k)); width: calc(32px * var(--k)); height: calc(32px * var(--k)); border-radius: 50%; background: var(--accent); border: calc(6px * var(--k)) solid var(--bg); }
    .ms .when { font-size: calc(var(--num) * 0.8 * var(--k)); font-weight: 700; color: var(--accent); line-height: 1.2; }

  `],
  template: `
    @switch (layout()) {
      @case ('qa') {
        <ol class="qa-grid" [style.--cols]="wide() ? 2 : 1">
          @for (s of doc().sections; track s.id; let i = $index) {
            <li class="card qa">
              <div class="row">
                <span class="badge" aria-hidden="true">{{ i + 1 }}</span>
                <div class="txt">
                  <h2 dir="auto">{{ s.heading }}</h2>
                  <p class="body" dir="auto">{{ s.body }}</p>
                </div>
              </div>
            </li>
          }
        </ol>
      }
      @case ('stairs') {
        <ol class="stack">
          @for (s of doc().sections; track s.id; let i = $index) {
            <li class="stair" [style.--i]="i">
              <span class="badge" aria-hidden="true">{{ i + 1 }}</span>
              <div class="txt">
                <h2 dir="auto">{{ s.heading }}</h2>
                @if (s.body) {
                  <p class="body" dir="auto">{{ s.body }}</p>
                }
              </div>
            </li>
          }
        </ol>
      }
      @case ('checklist') {
        <ul class="stack" style="gap: 0">
          @for (s of doc().sections; track s.id) {
            <li class="tick row">
              <span class="check" aria-hidden="true"></span>
              <div class="txt">
                <h2 dir="auto">{{ s.heading }}</h2>
                @if (s.body) {
                  <p class="body" dir="auto">{{ s.body }}</p>
                }
              </div>
            </li>
          }
        </ul>
      }
      @case ('proscons') {
        <div class="two" [style.--cols2]="wide() ? 2 : 1">
          @for (s of doc().sections; track s.id; let i = $index) {
            <section class="card col">
              <div class="band">
                <span class="sym" [class.minus]="i === 1" aria-hidden="true"></span>
                <h2 dir="auto">{{ s.heading }}</h2>
              </div>
              <ul class="pts">
                @for (line of lines(s.body); track $index) {
                  <li><span class="sym" [class.minus]="i === 1" aria-hidden="true"></span><span dir="auto">{{ line }}</span></li>
                }
              </ul>
            </section>
          }
        </div>
      }
      @case ('versus') {
        <div class="two vs" [style.--cols2]="wide() ? 2 : 1">
          @for (s of doc().sections; track s.id) {
            <section class="card col">
              <div class="band"><h2 dir="auto">{{ s.heading }}</h2></div>
              <div class="card-body"><p class="body" style="margin-block-start: 0" dir="auto">{{ s.body }}</p></div>
            </section>
          }
          @if (wide()) {
            <span class="vs-mark" aria-hidden="true"></span>
          }
        </div>
      }
      @case ('zigzag') {
        <ol class="zz">
          @for (s of doc().sections; track s.id; let i = $index) {
            <li [style.grid-row]="(i + 1) + ' / span 2'">
              @if (s.label) {
                <span class="pill" dir="auto">{{ s.label }}</span>
              }
              <h2 dir="auto">{{ s.heading }}</h2>
              <p class="body" dir="auto">{{ s.body }}</p>
            </li>
          }
        </ol>
      }
      @default {
        <ol class="ms" [style.--cols]="msCols()">
          @for (s of doc().sections; track s.id) {
            <li class="card">
              @if (s.label) {
                <p class="when" dir="auto">{{ s.label }}</p>
              }
              <h2 dir="auto">{{ s.heading }}</h2>
              <p class="body" dir="auto">{{ s.body }}</p>
            </li>
          }
        </ol>
      }
    }
  `,
})
export class InfographicDiagramComponent {
  readonly doc = input.required<InfographicDoc>();
  readonly layout = input.required<ListLayout>();
  readonly rtl = input(false);

  protected readonly wide = computed(() => this.doc().format === 'landscape');
  protected readonly msCols = computed(() => {
    const n = this.doc().sections.length;
    return this.wide() ? Math.min(n, 3) : Math.min(n, 2);
  });

  protected lines(body: string): string[] {
    return body.split('\n').map((l) => l.trim()).filter(Boolean);
  }
}

/** Which layouts the diagram components draw, for the artboard. */
export function isDiagram(templateId: string): boolean {
  const t = templateById(templateId) ?? TEMPLATES[0];
  return DIAGRAM_LAYOUTS.includes(t.layout);
}
