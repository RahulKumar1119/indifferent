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
    <fieldset class="m-0 min-w-0 border-0 p-0">
      <legend class="mb-2 text-sm font-semibold">Thumbnail style</legend>
      <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <label class="relative block cursor-pointer">
          <input
            type="radio"
            name="thumb-style"
            class="peer sr-only"
            value=""
            [checked]="style() === ''"
            (change)="style.set('')"
          />
          <span class="opt block h-full rounded-lg border border-[hsl(var(--border))] p-3 text-left">
            <span class="block text-sm font-medium">Auto</span>
            <span class="block text-xs text-[hsl(var(--muted-foreground))]">AI picks to suit the video</span>
          </span>
        </label>
        @for (s of styles; track s.value) {
          <label class="relative block cursor-pointer">
            <input
              type="radio"
              name="thumb-style"
              class="peer sr-only"
              [value]="s.value"
              [checked]="style() === s.value"
              (change)="style.set(s.value)"
            />
            <span class="opt block h-full rounded-lg border border-[hsl(var(--border))] p-3 text-left">
              <span class="block text-sm font-medium">{{ s.label }}</span>
              <span class="block text-xs text-[hsl(var(--muted-foreground))]">{{ s.blurb }}</span>
            </span>
          </label>
        }
      </div>
    </fieldset>

    <fieldset class="m-0 mt-5 min-w-0 border-0 p-0">
      <legend class="mb-2 text-sm font-semibold">Shape</legend>
      <div class="flex flex-wrap gap-2">
        @for (f of formats; track f.value) {
          <label class="relative block cursor-pointer">
            <input
              type="radio"
              name="thumb-format"
              class="peer sr-only"
              [value]="f.value"
              [checked]="format() === f.value"
              (change)="format.set(f.value)"
            />
            <span class="opt block rounded-lg border border-[hsl(var(--border))] px-4 py-2 text-left">
              <span class="block text-sm font-medium">{{ f.label }}</span>
              <span class="block text-xs text-[hsl(var(--muted-foreground))]">{{ f.blurb }}</span>
            </span>
          </label>
        }
      </div>
    </fieldset>
  `,
  styles: [`
    /* Selected and focus states hang off the hidden radio via the sibling selector. */
    input:checked + .opt {
      border-color: hsl(var(--primary));
      background: hsl(var(--primary) / 0.12);
    }
    input:focus-visible + .opt {
      outline: 2px solid hsl(var(--primary));
      outline-offset: 2px;
    }
    .opt { transition: border-color .15s, background-color .15s; }
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
