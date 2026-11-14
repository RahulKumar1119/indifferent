import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { DecorGlyphComponent } from './decor-glyph.component';
import { Glyph, GLYPHS, ICONS, SHAPES, glyphById } from './decor-library';
import { DecorationPatch, addDecoration, removeDecoration, updateDecoration } from './doc-ops';
import { DocChange } from './infographic-design-panel.component';
import { DECOR_COLORS, DecorColor, Decoration, InfographicDoc, LIMITS } from './infographic.model';

const COLOR_LABELS: Record<DecorColor, string> = { accent: 'Accent', ink: 'Text', ink2: 'Soft text' };

/**
 * Icons and shapes: pick one to add it, then drag it on the preview. For people who do not drag,
 * every item also has number boxes for its place and a size slider, and the keys work on the preview.
 */
@Component({
  selector: 'app-infographic-decor-panel',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [DecorGlyphComponent, LucideDynamicIcon],
  styles: [`
    :host { display: block; }
    .hint { margin: 0 0 var(--space-sm); font-size: var(--type-xs); color: var(--color-ink-2); }
    h3 { margin: var(--space-sm) 0 var(--space-xs); font-size: var(--type-sm); font-weight: 600; }
    .pick { display: grid; gap: 6px; grid-template-columns: repeat(auto-fill, minmax(2.75rem, 1fr)); }
    .pick button { display: grid; place-items: center; min-height: 44px; padding: 8px; border: 1px solid var(--color-rule); border-radius: var(--radius-control); background: var(--color-paper); color: var(--color-ink); cursor: pointer; }
    .pick button:hover:not(:disabled) { border-color: var(--color-ink-2); }
    .pick button:disabled { opacity: 0.45; cursor: not-allowed; }
    .pick app-decor-glyph { width: 22px; height: 22px; }
    ul { list-style: none; margin: var(--space-md) 0 0; padding: 0; display: flex; flex-direction: column; gap: var(--space-sm); }
    li { padding: var(--space-sm); border: 1px solid var(--color-rule); border-radius: var(--radius-control); background: var(--color-paper); display: grid; gap: var(--space-xs); }
    li.on { border-color: var(--color-ink); box-shadow: inset 0 0 0 1px var(--color-ink); }
    .head { display: flex; align-items: center; gap: var(--space-xs); }
    .head app-decor-glyph { width: 22px; height: 22px; flex: none; }
    .head strong { flex: 1; font-size: var(--type-sm); }
    .fields { display: grid; gap: var(--space-xs); grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .fields .wide { grid-column: 1 / -1; }
    .ui-flabel { font-size: var(--type-xs); }
    input[type='range'] { width: 100%; min-height: 44px; }
    .count { margin: var(--space-xs) 0 0; }
  `],
  template: `
    <p class="hint">Pick one to add it. Then drag it on the preview, or focus it and use the arrow keys (hold Shift for bigger steps). Delete removes it. Up to {{ limit }}.</p>
    <h3 id="ig-shapes">Shapes</h3>
    <div class="pick" role="group" aria-labelledby="ig-shapes">
      @for (g of shapes; track g.id) {
        <button type="button" [disabled]="full()" [attr.aria-label]="'Add ' + g.label.toLowerCase()" [title]="g.label" (click)="add(g)">
          <app-decor-glyph [glyph]="g" />
        </button>
      }
    </div>
    <h3 id="ig-icons">Icons</h3>
    <div class="pick" role="group" aria-labelledby="ig-icons">
      @for (g of icons; track g.id) {
        <button type="button" [disabled]="full()" [attr.aria-label]="'Add ' + g.label.toLowerCase() + ' icon'" [title]="g.label" (click)="add(g)">
          <app-decor-glyph [glyph]="g" />
        </button>
      }
    </div>
    @if (full()) {
      <p class="hint count" data-testid="decor-full">That is the most the preview can hold ({{ limit }}). Remove one to add another.</p>
    }

    @if (doc().items.length > 0) {
      <ul aria-label="Icons and shapes on the preview">
        @for (it of doc().items; track it.id; let i = $index) {
          <li [class.on]="selected() === it.id" [attr.data-item]="it.id">
            <div class="head">
              <app-decor-glyph [glyph]="glyphOf(it)" />
              <strong>{{ nameOf(it) }} {{ i + 1 }}</strong>
              <button type="button" class="ui-btn ui-btn--quiet" (click)="remove(it)" [attr.aria-label]="'Remove ' + nameOf(it).toLowerCase() + ' ' + (i + 1)">
                <svg lucideIcon="trash-2" [size]="18" aria-hidden="true"></svg>
              </button>
            </div>
            <div class="fields">
              <div class="ui-f wide">
                <label class="ui-flabel" [for]="'ig-size-' + it.id">Size</label>
                <input [id]="'ig-size-' + it.id" type="range" [min]="min" [max]="max" step="1" [value]="it.size" (input)="num(it, 'size', $event)" (focus)="pick.emit(it.id)" />
              </div>
              <div class="ui-f">
                <label class="ui-flabel" [for]="'ig-x-' + it.id">Across (%)</label>
                <input [id]="'ig-x-' + it.id" class="ui-field" type="number" min="0" max="100" step="1" [value]="it.x" (input)="num(it, 'x', $event)" (focus)="pick.emit(it.id)" />
              </div>
              <div class="ui-f">
                <label class="ui-flabel" [for]="'ig-y-' + it.id">Down (%)</label>
                <input [id]="'ig-y-' + it.id" class="ui-field" type="number" min="0" max="100" step="1" [value]="it.y" (input)="num(it, 'y', $event)" (focus)="pick.emit(it.id)" />
              </div>
              <div class="ui-f wide">
                <label class="ui-flabel" [for]="'ig-color-' + it.id">Colour</label>
                <select [id]="'ig-color-' + it.id" class="ui-field" (change)="color(it, $any($event.target).value)" (focus)="pick.emit(it.id)">
                  @for (c of colors; track c) {
                    <option [value]="c" [selected]="it.color === c">{{ colorLabel(c) }}</option>
                  }
                </select>
              </div>
            </div>
          </li>
        }
      </ul>
    }
  `,
})
export class InfographicDecorPanelComponent {
  readonly doc = input.required<InfographicDoc>();
  readonly selected = input<string | null>(null);
  readonly docChange = output<DocChange>();
  readonly announce = output<string>();
  readonly pick = output<string>();

  protected readonly shapes = SHAPES;
  protected readonly icons = ICONS;
  protected readonly colors = DECOR_COLORS;
  protected readonly limit = LIMITS.items;
  protected readonly min = LIMITS.itemMin;
  protected readonly max = LIMITS.itemMax;
  protected readonly full = computed(() => this.doc().items.length >= LIMITS.items);

  protected glyphOf(it: Decoration): Glyph | undefined {
    return glyphById(it.glyph);
  }

  protected nameOf(it: Decoration): string {
    return glyphById(it.glyph)?.label ?? 'Icon';
  }

  protected colorLabel(c: DecorColor): string {
    return COLOR_LABELS[c];
  }

  protected add(g: Glyph): void {
    const next = addDecoration(this.doc(), g.id);
    if (next === this.doc()) return;
    this.docChange.emit({ doc: next });
    const added = next.items[next.items.length - 1];
    this.pick.emit(added.id);
    this.announce.emit(`${g.label} added as item ${next.items.length}. Drag it on the preview, or use its boxes below.`);
  }

  protected remove(it: Decoration): void {
    const next = removeDecoration(this.doc(), it.id);
    if (next === this.doc()) return;
    this.docChange.emit({ doc: next });
    this.announce.emit(`${this.nameOf(it)} removed.`);
  }

  protected num(it: Decoration, field: 'x' | 'y' | 'size', event: Event): void {
    const raw = (event.target as HTMLInputElement).value;
    // An empty box while typing is left alone; it is not a move to 0.
    if (raw.trim() === '') return;
    const patch: DecorationPatch = { [field]: Number(raw) };
    const next = updateDecoration(this.doc(), it.id, patch);
    if (next !== this.doc()) this.docChange.emit({ doc: next, key: `deco:${it.id}:${field}` });
  }

  protected color(it: Decoration, value: string): void {
    const next = updateDecoration(this.doc(), it.id, { color: value as DecorColor });
    if (next !== this.doc()) this.docChange.emit({ doc: next });
  }
}

/** The glyph ids, for tests that check every one can be added. */
export const ALL_GLYPH_IDS = GLYPHS.map((g) => g.id);
