import { isPlatformBrowser } from '@angular/common';
import { ChangeDetectionStrategy, Component, NgZone, PLATFORM_ID, computed, effect, inject, input, output, signal } from '@angular/core';
import { CookieConsentService } from '../../../core/consent/consent.service';
import { CaptionPreference } from '../../caption-preference.service';
import {
  CAPTION_FONT_LIST,
  CAPTION_POSITIONS,
  CAPTION_PRESET_LIST,
  CAPTION_SIZE_MAX,
  CAPTION_SIZE_MIN,
  CaptionFont,
  CaptionStyleValue,
  bottomInsetPercent,
  previewFontFamily,
  sampleCaption,
  scriptFor,
} from '../../caption-style';

/** The caption's own height unit: the renderer sizes text as size / 288 of the frame height. */
const FRAME_UNITS = 288;
const TICK_MS = 450;

/**
 * The caption controls and a live 9:16 preview, used by AI Shorts and by story
 * videos (#327). The parent owns the value and the Save or Make button; this
 * component only edits a style and shows what it looks like.
 *
 * The preview is an approximation drawn in the browser. The server renders with
 * its own fonts, so line breaks and widths can differ a little, and the
 * component says so. The karaoke highlight moves on a timer; it stops for
 * people who ask for reduced motion (a fixed half-lit caption is shown) and it
 * has a Pause button for everyone else.
 */
@Component({
  selector: 'app-caption-studio',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: [`
    :host { display: block; }
    .studio { display: grid; gap: var(--space-lg); }
    @media (min-width: 48rem) { .studio { grid-template-columns: minmax(0, 1fr) 12rem; align-items: start; } }
    fieldset { margin: 0; padding: 0; border: 0; min-width: 0; }
    legend, .lbl { padding: 0; font-weight: 600; }
    .hint { margin: var(--space-3xs) 0 0; font-size: var(--type-sm); color: var(--color-ink-2); }
    .controls { display: grid; gap: var(--space-md); min-width: 0; }
    .opts { margin-top: var(--space-xs); display: grid; gap: var(--space-xs); grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr)); }
    .opt { display: flex; align-items: flex-start; gap: var(--space-sm); min-height: 44px; padding: var(--space-xs) var(--space-sm); border: 1px solid var(--color-muted); border-radius: var(--radius-control); cursor: pointer; transition: border-color var(--dur-short) var(--ease-out); }
    .opt:hover { border-color: var(--color-ink-2); }
    .opt:has(input:checked) { border-color: var(--color-accent); box-shadow: inset 0 0 0 1px var(--color-accent); }
    .opt:has(input:focus-visible) { outline: 2px solid var(--color-focus); outline-offset: 2px; }
    .opt:has(input:disabled) { cursor: not-allowed; opacity: 0.6; }
    .opt input { margin: 3px 0 0; width: 1.125rem; height: 1.125rem; flex: none; accent-color: var(--color-accent); }
    .name { display: block; font-weight: 600; }
    .blurb { display: block; font-size: var(--type-sm); color: var(--color-ink-2); }
    .row { display: flex; flex-wrap: wrap; gap: var(--space-md); align-items: flex-end; }
    .row label { display: grid; gap: var(--space-3xs); font-weight: 600; }
    .size { display: grid; gap: var(--space-3xs); flex: 1 1 12rem; }
    .size input[type=range] { width: 100%; min-height: 44px; accent-color: var(--color-accent); }
    input[type=color] { width: 3rem; height: 2.75rem; padding: 2px; }
    .check { display: flex; align-items: flex-start; gap: var(--space-sm); min-height: 44px; }
    .check input { margin: 4px 0 0; width: 1.125rem; height: 1.125rem; flex: none; accent-color: var(--color-accent); }
    .preview { display: grid; gap: var(--space-xs); justify-items: center; }
    .frame { container-type: size; position: relative; overflow: hidden; width: 10rem; aspect-ratio: 9 / 16; border-radius: var(--radius-control); background: linear-gradient(160deg, #2b3a55, #121826 60%, #3a2b2b); border: 1px solid var(--color-muted); }
    .btns { position: absolute; right: 4%; bottom: 14%; display: grid; gap: 6%; opacity: .5; }
    .btns i { display: block; width: 7cqw; height: 7cqw; border-radius: 50%; background: #fff; }
    .cap { position: absolute; left: 5%; right: 5%; text-align: center; line-height: 1.2; font-size: calc(var(--cs) / 288 * 100cqh); overflow-wrap: anywhere; }
    .cap span.t { padding: 0 .25em; }
    .cap.bold span.t { background: rgba(0, 0, 0, .62); font-weight: 800; border-radius: .2em; box-decoration-break: clone; -webkit-box-decoration-break: clone; }
    .cap.karaoke { font-weight: 800; text-shadow: 0 .06em .12em rgba(0, 0, 0, .9); }
    .cap.clean { text-shadow: -.05em 0 #000, .05em 0 #000, 0 -.05em #000, 0 .05em #000; }
    .cap.minimal { text-shadow: 0 .04em .1em rgba(0, 0, 0, .8); }
    .note { margin: 0; font-size: var(--type-sm); color: var(--color-ink-2); text-align: center; }
    .pause { min-height: 44px; }
  `],
  template: `
    <div class="studio">
      <div class="controls">
        <fieldset [disabled]="disabled()">
          <legend>Caption look</legend>
          <div class="opts" role="radiogroup" aria-label="Caption look">
            @for (p of presets; track p.id) {
              <label class="opt">
                <input type="radio" [name]="id('preset')" [value]="p.id" [checked]="p.id === value().preset" (change)="patch({ preset: p.id })" />
                <span style="min-width: 0">
                  <span class="name">{{ p.label }}</span>
                  <span class="blurb">{{ p.blurb }}</span>
                </span>
              </label>
            }
          </div>
        </fieldset>

        <div class="row">
          <label>
            <span>Font</span>
            <select class="ui-field" [id]="id('font')" [disabled]="disabled()" (change)="setFont($event)">
              @for (f of fonts; track f.id) {
                <option [value]="f.id" [selected]="f.id === value().font">{{ f.label }}</option>
              }
            </select>
          </label>
          <label class="size">
            <span>Size <output [for]="id('size')">{{ value().fontSize }}</output></span>
            <input type="range" [id]="id('size')" [min]="sizeMin" [max]="sizeMax" step="1" [value]="value().fontSize" [disabled]="disabled()" (input)="setSize($event)" aria-label="Caption size" />
          </label>
        </div>

        <div class="row">
          <label>
            <span>Text</span>
            <input type="color" [value]="value().color" [disabled]="disabled()" (input)="setColor('color', $event)" aria-label="Caption text colour" />
          </label>
          @if (value().preset === 'karaoke') {
            <label>
              <span>Highlight</span>
              <input type="color" [value]="value().highlight" [disabled]="disabled()" (input)="setColor('highlight', $event)" aria-label="Karaoke highlight colour" />
            </label>
          }
        </div>

        <fieldset [disabled]="disabled()">
          <legend>Position</legend>
          <div class="opts" role="radiogroup" aria-label="Caption position">
            @for (pos of positions; track pos) {
              <label class="opt">
                <input type="radio" [name]="id('pos')" [value]="pos" [checked]="pos === value().position" (change)="patch({ position: pos })" />
                <span class="name" style="text-transform: capitalize">{{ pos }}</span>
              </label>
            }
          </div>
        </fieldset>

        <div>
          <label class="check">
            <input type="checkbox" [id]="id('above')" [checked]="value().aboveControls" [disabled]="disabled()" (change)="setAbove($event)" [attr.aria-describedby]="id('above-hint')" />
            <span>
              <span class="name">Above platform controls</span>
              <span class="blurb" [id]="id('above-hint')">
                Lifts bottom captions so TikTok, Reels and Shorts buttons do not cover them.
                @if (value().position !== 'bottom') { It only moves captions placed at the bottom. }
              </span>
            </span>
          </label>
        </div>
        <ng-content />
        <p class="hint" data-testid="remember-note">
          @if (pref.remembering) {
            Saving or making a video also keeps this style as the start for your next new one. Anything you already saved keeps its own style.
          } @else {
            This style will not be remembered for your next one. To keep it, allow Functional storage in
            <a href="#" class="ui-link" (click)="openCookieSettings($event)">cookie settings</a>.
          }
        </p>
      </div>

      <div class="preview">
        <div class="frame" role="img" [attr.aria-label]="label()" data-testid="caption-preview" [style.--cs]="value().fontSize">
          <div class="btns" aria-hidden="true"><i></i><i></i><i></i></div>
          <div class="cap" [class]="'cap ' + value().preset" [style.font-family]="family()" [style.color]="value().color" [style]="placement()" data-testid="caption-preview-text">
            @if (value().preset === 'karaoke') {
              @for (w of words(); track $index) {
                <span class="t" [style.color]="$index < lit() ? value().highlight : value().color">{{ w }}{{ $last ? '' : ' ' }}</span>
              }
            } @else {
              <span class="t">{{ text() }}</span>
            }
          </div>
        </div>
        @if (value().preset === 'karaoke' && !reduced) {
          <button type="button" class="ui-btn ui-btn--secondary pause" (click)="paused.set(!paused())" [attr.aria-pressed]="paused()" data-testid="preview-pause">
            {{ paused() ? 'Play preview' : 'Pause preview' }}
          </button>
        }
        <p class="note">Approximate. The finished video can break lines a little differently.</p>
      </div>
    </div>
  `,
})
export class CaptionStudioComponent {
  readonly value = input.required<CaptionStyleValue>();
  readonly disabled = input(false);
  /** Language code of what is being captioned. It picks a script for the sample text. */
  readonly language = input<string | undefined>(undefined);
  /** Real caption words, when there are some. Otherwise a sample sentence is shown. */
  readonly sampleText = input<string | undefined>(undefined);
  /** Keeps ids unique when two studios share a page. */
  readonly idPrefix = input('cs');
  readonly valueChange = output<CaptionStyleValue>();

  protected readonly presets = CAPTION_PRESET_LIST;
  protected readonly fonts = CAPTION_FONT_LIST;
  protected readonly positions = CAPTION_POSITIONS;
  protected readonly sizeMin = CAPTION_SIZE_MIN;
  protected readonly sizeMax = CAPTION_SIZE_MAX;
  protected readonly pref = inject(CaptionPreference);
  private readonly consent = inject(CookieConsentService);
  protected readonly paused = signal(false);
  protected readonly reduced: boolean;
  private readonly tick = signal(0);

  protected readonly text = computed(() => {
    const own = (this.sampleText() ?? '').trim().replace(/\s+/g, ' ');
    return own || sampleCaption(scriptFor(this.language()));
  });
  protected readonly words = computed(() => this.text().split(' '));
  /** How many words are lit. Reduced motion shows a fixed half-lit caption. */
  protected readonly lit = computed(() => {
    const n = this.words().length;
    if (this.reduced) return Math.ceil(n / 2);
    return this.tick() % (n + 2);
  });
  protected readonly family = computed(() => previewFontFamily(this.value().font));
  protected readonly label = computed(() => {
    const v = this.value();
    const p = this.presets.find((x) => x.id === v.preset)?.label ?? v.preset;
    return `Approximate preview: ${p} captions, size ${v.fontSize}, ${v.position}`;
  });
  protected readonly placement = computed(() => {
    const v = this.value();
    if (v.position === 'top') return { top: '6%' };
    if (v.position === 'middle') return { top: '50%', transform: 'translateY(-50%)' };
    return { bottom: `${bottomInsetPercent(v.preset, v.aboveControls)}%` };
  });

  constructor() {
    const browser = isPlatformBrowser(inject(PLATFORM_ID));
    const zone = inject(NgZone);
    this.reduced = browser && typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;
    effect((onCleanup) => {
      if (!browser || this.reduced || this.paused() || this.value().preset !== 'karaoke') return;
      // Outside the zone, so a page waiting for the app to be stable is not held up by a loop.
      const timer = zone.runOutsideAngular(() => setInterval(() => this.tick.update((n) => n + 1), TICK_MS));
      onCleanup(() => clearInterval(timer));
    });
  }

  protected openCookieSettings(event: Event): void {
    event.preventDefault();
    this.consent.openPreferences(event.currentTarget as HTMLElement);
  }

  protected id(part: string): string {
    return `${this.idPrefix()}-${part}`;
  }

  protected patch(change: Partial<CaptionStyleValue>): void {
    this.valueChange.emit({ ...this.value(), ...change });
  }

  protected setFont(event: Event): void {
    this.patch({ font: (event.target as HTMLSelectElement).value as CaptionFont });
  }

  protected setSize(event: Event): void {
    const n = Number((event.target as HTMLInputElement).value);
    if (Number.isFinite(n)) this.patch({ fontSize: Math.min(CAPTION_SIZE_MAX, Math.max(CAPTION_SIZE_MIN, Math.round(n))) });
  }

  protected setColor(field: 'color' | 'highlight', event: Event): void {
    this.patch({ [field]: (event.target as HTMLInputElement).value } as Partial<CaptionStyleValue>);
  }

  protected setAbove(event: Event): void {
    this.patch({ aboveControls: (event.target as HTMLInputElement).checked });
  }
}
