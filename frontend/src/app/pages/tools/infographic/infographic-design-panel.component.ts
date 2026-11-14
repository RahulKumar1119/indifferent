import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { withTemplate } from './doc-ops';
import { DIRECTIONS, Direction, FORMATS, FORMAT_IDS, FormatId, InfographicDoc, MARK_TEXT } from './infographic.model';
import { LANGUAGES, unreviewedLanguage } from './languages';
import { TEMPLATES } from './templates';
import { THEMES } from './themes';

/** A change to the document, and, for typing, the key that lets undo treat it as one step. */
export interface DocChange {
  doc: InfographicDoc;
  key?: string;
}

const DIRECTION_LABELS: Record<Direction, string> = { auto: 'Automatic', ltr: 'Left to right', rtl: 'Right to left' };

/** Template, size, colours, language, direction and the optional mark. */
@Component({
  selector: 'app-infographic-design-panel',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: [`
    :host { display: block; }
    fieldset { margin: 0; padding: 0; border: 0; min-width: 0; }
    legend { padding: 0; margin-block-end: var(--space-xs); font-size: var(--type-sm); font-weight: 600; }
    .group + .group { margin-block-start: var(--space-lg); }
    .tiles { display: grid; gap: var(--space-xs); grid-template-columns: repeat(auto-fill, minmax(9.5rem, 1fr)); }
    .cat { margin: var(--space-sm) 0 var(--space-xs); font-size: var(--type-xs); font-weight: 600; color: var(--color-ink-2); }
    .cat:first-child { margin-block-start: 0; }
    .tile { position: relative; display: block; cursor: pointer; }
    .tile > span { display: flex; flex-direction: column; gap: 2px; min-height: 44px; height: 100%; padding: var(--space-xs) var(--space-sm); border: 1px solid var(--color-rule); border-radius: var(--radius-control); background: var(--color-paper); color: var(--color-ink); }
    .tile:hover > span { border-color: var(--color-ink-2); }
    .tile:has(input:checked) > span { border-color: var(--color-ink); box-shadow: inset 0 0 0 1px var(--color-ink); }
    .tile:has(input:focus-visible) { outline: 2px solid var(--color-focus); outline-offset: 2px; border-radius: var(--radius-control); }
    .tile strong { font-size: var(--type-sm); }
    .tile small { font-size: var(--type-xs); color: var(--color-ink-2); line-height: 1.35; }
    .dots { display: flex; gap: 4px; margin-block-start: 2px; }
    .dots i { width: 18px; height: 18px; border-radius: 50%; border: 1px solid var(--color-rule); }
    .two { display: grid; gap: var(--space-sm); grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr)); }
    .mark { display: flex; align-items: center; justify-content: space-between; gap: var(--space-sm); }
    .note { margin: var(--space-xs) 0 0; font-size: var(--type-xs); color: var(--color-ink-2); }
    .mark p { margin: 0; font-size: var(--type-xs); color: var(--color-ink-2); }
  `],
  template: `
    <fieldset class="group">
      <legend>Template</legend>
      <div role="radiogroup" aria-label="Template">
        @for (g of groups; track g.name) {
          <p class="cat">{{ g.name }}</p>
          <div class="tiles">
            @for (t of g.items; track t.id) {
              <label class="tile">
                <input type="radio" class="sr-only" name="template" [value]="t.id" [checked]="doc().templateId === t.id" (change)="template(t.id)" />
                <span>
                  <strong>{{ t.name }}</strong>
                  <small>{{ t.description }}</small>
                </span>
              </label>
            }
          </div>
        }
      </div>
    </fieldset>

    <fieldset class="group">
      <legend>Size</legend>
      <div class="tiles" role="radiogroup" aria-label="Size">
        @for (f of formats(); track f.id) {
          <label class="tile">
            <input type="radio" class="sr-only" name="format" [value]="f.id" [checked]="doc().format === f.id" (change)="format(f.id)" />
            <span>
              <strong>{{ f.label }}</strong>
              <small>{{ f.width }} &times; {{ f.height }}. {{ f.hint }}</small>
            </span>
          </label>
        }
      </div>
    </fieldset>

    <fieldset class="group">
      <legend>Colours</legend>
      <div class="tiles" role="radiogroup" aria-label="Colours">
        @for (t of themes; track t.id) {
          <label class="tile">
            <input type="radio" class="sr-only" name="theme" [value]="t.id" [checked]="doc().themeId === t.id" (change)="theme(t.id)" />
            <span>
              <strong>{{ t.name }}</strong>
              <span class="dots" aria-hidden="true">
                <i [style.background]="t.bg"></i><i [style.background]="t.surface"></i><i [style.background]="t.ink"></i><i [style.background]="t.accent"></i>
              </span>
            </span>
          </label>
        }
      </div>
    </fieldset>

    <div class="group two">
      <div class="ui-f">
        <label class="ui-flabel" for="ig-lang">Language of the text</label>
        <select id="ig-lang" class="ui-field" (change)="language($any($event.target).value)">
          <option value="auto" [selected]="doc().language === 'auto'">Detect from the text ({{ detectedName() }})</option>
          @for (l of languages; track l.code) {
            <option [value]="l.code" [selected]="doc().language === l.code">{{ l.name }}</option>
          }
        </select>
      </div>
      <div class="ui-f">
        <label class="ui-flabel" for="ig-dir">Text direction</label>
        <select id="ig-dir" class="ui-field" (change)="direction($any($event.target).value)">
          @for (d of directions; track d) {
            <option [value]="d" [selected]="doc().direction === d">{{ directionLabel(d) }}</option>
          }
        </select>
      </div>
    </div>
    @if (unreviewed(); as l) {
      <p class="note" data-testid="lang-note">
        {{ l.name }}: the fonts and the layout are checked by our own tests. A reader of {{ l.name }} has not reviewed the output yet, so please look at your image before you share it. Your words are never translated or changed.
      </p>
    }

    <div class="group mark">
      <div>
        <span id="ig-mark-label" class="ui-flabel">Add "{{ markText }}"</span>
        <p>A small line at the bottom of the image. Off unless you turn it on.</p>
      </div>
      <button type="button" role="switch" class="ui-switch" aria-labelledby="ig-mark-label" [attr.aria-checked]="doc().mark" (click)="mark()">
        <span></span>
      </button>
    </div>
  `,
})
export class InfographicDesignPanelComponent {
  readonly doc = input.required<InfographicDoc>();
  /** The language the text looks like, for the "detect" choice. */
  readonly detected = input('en');
  readonly docChange = output<DocChange>();

  protected readonly templates = TEMPLATES;
  /** The templates under their category, in the order the categories first appear. */
  protected readonly groups = Array.from(new Set(TEMPLATES.map((t) => t.category))).map((name) => ({ name, items: TEMPLATES.filter((t) => t.category === name) }));
  protected readonly themes = THEMES;
  protected readonly languages = LANGUAGES;
  protected readonly directions = DIRECTIONS;
  protected readonly markText = MARK_TEXT;
  protected readonly formats = computed(() => FORMAT_IDS.map((id) => FORMATS[id]));
  /** The language in use (chosen, or detected) when no reader of it has reviewed the output yet. */
  protected readonly unreviewed = computed(() => unreviewedLanguage(this.doc().language === 'auto' ? this.detected() : this.doc().language));
  protected readonly detectedName = computed(() => LANGUAGES.find((l) => l.code === this.detected())?.name ?? this.detected());

  protected directionLabel(d: Direction): string {
    return DIRECTION_LABELS[d];
  }

  protected template(id: string): void {
    const t = TEMPLATES.find((x) => x.id === id);
    if (t) this.docChange.emit({ doc: withTemplate(this.doc(), t) });
  }
  protected format(id: FormatId): void {
    this.docChange.emit({ doc: { ...this.doc(), format: id } });
  }
  protected theme(id: string): void {
    this.docChange.emit({ doc: { ...this.doc(), themeId: id } });
  }
  protected language(code: string): void {
    this.docChange.emit({ doc: { ...this.doc(), language: code } });
  }
  protected direction(d: Direction): void {
    this.docChange.emit({ doc: { ...this.doc(), direction: d } });
  }
  protected mark(): void {
    this.docChange.emit({ doc: { ...this.doc(), mark: !this.doc().mark } });
  }
}
