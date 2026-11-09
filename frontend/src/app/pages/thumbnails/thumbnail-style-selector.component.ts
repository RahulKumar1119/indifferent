import { Component, model } from '@angular/core';
import { THUMBNAIL_STYLES, ThumbnailFormat, ThumbnailStyle } from './thumbnail.service';

/**
 * Picks the look and the shape of the thumbnails. Both groups are native
 * radio buttons, so arrow keys, focus and screen readers work without extra
 * code. An empty value means "automatic".
 */
@Component({
  selector: 'app-thumbnail-style-selector',
  standalone: true,
  template: `
    <fieldset>
      <legend>Thumbnail style</legend>
      <div class="styles">
        <label class="choice">
          <input type="radio" name="thumb-style" class="sr-only" value="" [checked]="style() === ''" (change)="style.set('')" />
          <span class="opt"><b>Auto</b><small>AI picks to suit the video</small></span>
        </label>
        @for (s of styles; track s.value) {
          <label class="choice">
            <input type="radio" name="thumb-style" class="sr-only" [value]="s.value" [checked]="style() === s.value" (change)="style.set(s.value)" />
            <span class="opt"><b>{{ s.label }}</b><small>{{ s.blurb }}</small></span>
          </label>
        }
      </div>
    </fieldset>
    <fieldset class="shape">
      <legend>Shape</legend>
      <div class="formats">
        @for (f of formats; track f.value) {
          <label class="choice">
            <input type="radio" name="thumb-format" class="sr-only" [value]="f.value" [checked]="format() === f.value" (change)="format.set(f.value)" />
            <span class="opt"><b>{{ f.label }}</b><small>{{ f.blurb }}</small></span>
          </label>
        }
      </div>
    </fieldset>
  `,
  styles: [`
    fieldset { margin: 0; padding: 0; border: 0; min-width: 0; }
    fieldset.shape { margin-top: var(--space-lg); }
    legend { margin-bottom: var(--space-xs); font-size: var(--type-sm); font-weight: 600; color: var(--color-ink); }
    .styles { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-xs); }
    @media (min-width: 40rem) { .styles { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
    .formats { display: flex; flex-wrap: wrap; gap: var(--space-xs); }
    .choice { position: relative; display: block; cursor: pointer; }
    .opt { display: flex; flex-direction: column; gap: var(--space-3xs); height: 100%; min-height: 44px; padding: var(--space-sm); border-radius: var(--radius-control); border: 1px solid var(--color-rule); text-align: left; transition: border-color var(--dur-short) var(--ease-out), background-color var(--dur-short) var(--ease-out); }
    .opt:hover { background: var(--color-paper-3); }
    .opt b { font-size: var(--type-sm); font-weight: 600; color: var(--color-ink); }
    .opt small { font-size: var(--type-xs); color: var(--color-muted); }
    /* Selected and focus states hang off the hidden radio via the sibling selector. */
    input:checked + .opt { border: 2px solid var(--color-accent); background: color-mix(in oklab, var(--color-accent) 8%, var(--app-card, var(--color-paper))); }
    input:focus-visible + .opt { outline: 2px solid var(--color-focus); outline-offset: 2px; }
    @media (prefers-reduced-motion: reduce) { .opt { transition: none; } }
  `],
})
export class ThumbnailStyleSelectorComponent {
  /** The chosen style; '' means automatic. */
  readonly style = model<ThumbnailStyle | ''>('');
  /** The chosen shape; '' follows the video. */
  readonly format = model<ThumbnailFormat | ''>('');

  readonly styles = THUMBNAIL_STYLES;
  readonly formats: readonly { value: ThumbnailFormat | ''; label: string; blurb: string }[] = [
    { value: '', label: 'Match my video', blurb: 'Wide for landscape, tall for vertical' },
    { value: '16:9', label: '16:9', blurb: 'YouTube video' },
    { value: '9:16', label: '9:16', blurb: 'YouTube Shorts' },
  ];
}
