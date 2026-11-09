import { Component, computed, input, output } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { CONCEPT_LABELS, Thumbnail } from './thumbnail.service';

/** One thumbnail suggestion: a preview image with Select and Download. */
@Component({
  selector: 'app-thumbnail-card',
  standalone: true,
  imports: [LucideDynamicIcon],
  template: `
    <article class="card" [class.chosen]="chosen()">
      <button type="button" class="pic" (click)="preview.emit()" [attr.aria-label]="'Preview thumbnail ' + number() + ', ' + conceptLabel()">
        @if (thumb().url) {
          <img [src]="thumb().url" [alt]="alt()" [attr.width]="thumb().width" [attr.height]="thumb().height" loading="lazy" />
        } @else {
          <span class="none">Image unavailable</span>
        }
        <span class="tag">{{ conceptLabel() }}</span>
        @if (saved()) {
          <span class="saved"><svg lucideIcon="check" [size]="12" aria-hidden="true"></svg>Your thumbnail</span>
        }
      </button>
      <div class="acts">
        <button type="button" class="ui-btn ui-btn--secondary pick" [attr.aria-pressed]="chosen()" (click)="choose.emit()">
          @if (chosen()) {
            <svg lucideIcon="check" [size]="16" aria-hidden="true"></svg>
          }
          {{ chosen() ? 'Selected' : 'Select' }}
          <span class="sr-only"> thumbnail {{ number() }}</span>
        </button>
        <button type="button" class="ui-btn ui-btn--secondary" [disabled]="downloading()" (click)="download.emit()" [attr.aria-label]="'Download thumbnail ' + number()">
          @if (downloading()) {
            <svg lucideIcon="loader-2" [size]="16" class="ui-spin" aria-hidden="true"></svg>
          } @else {
            <svg lucideIcon="download" [size]="16" aria-hidden="true"></svg>
          }
          <span class="wide">Download</span>
        </button>
      </div>
    </article>
  `,
  styles: [`
    :host { display: block; height: 100%; }
    .card { height: 100%; display: flex; flex-direction: column; overflow: hidden; border-radius: var(--radius-surface); border: 2px solid var(--color-rule); background: var(--app-card, var(--color-paper)); box-shadow: 0 1px 2px color-mix(in oklab, var(--color-ink) 5%, transparent), 0 8px 24px -16px color-mix(in oklab, var(--color-ink) 18%, transparent); transition: border-color var(--dur-short) var(--ease-out); }
    .card.chosen { border-color: var(--color-accent); }
    .pic { position: relative; display: block; width: 100%; padding: 0; border: 0; background: var(--color-paper-3); cursor: zoom-in; }
    .pic:focus-visible { outline: 2px solid var(--color-focus); outline-offset: -3px; }
    .pic img { display: block; width: 100%; height: auto; }
    .none { display: flex; aspect-ratio: 16 / 9; align-items: center; justify-content: center; font-size: var(--type-xs); color: var(--color-muted); }
    .tag, .saved { position: absolute; top: var(--space-xs); padding: var(--space-3xs) var(--space-xs); border-radius: var(--radius-pill); font-size: var(--type-xs); font-weight: 600; }
    .tag { left: var(--space-xs); background: var(--color-paper); color: var(--color-ink); }
    /* Top right, so it never covers the text drawn on the thumbnail. */
    .saved { right: var(--space-xs); display: inline-flex; align-items: center; gap: var(--space-3xs); background: var(--color-positive); color: var(--color-paper); }
    .acts { display: flex; gap: var(--space-xs); padding: var(--space-sm); }
    .pick { flex: 1; }
    .pick[aria-pressed='true'] { background: var(--color-accent); border-color: var(--color-accent); color: var(--color-accent-ink); }
    /* Beats the shared secondary-button hover, so a selected card stays readable under the pointer. */
    .pick[aria-pressed='true']:hover:not(:disabled) { background: var(--color-accent-hover); border-color: var(--color-accent-hover); color: var(--color-accent-ink); }
    @media (max-width: 39.99rem) { .wide { display: none; } }
    @media (prefers-reduced-motion: reduce) { .card { transition: none; } }
  `],
})
export class ThumbnailCardComponent {
  readonly thumb = input.required<Thumbnail>();
  /** 1-based position, used in labels. */
  readonly number = input.required<number>();
  /** The user has picked this one (not yet saved). */
  readonly chosen = input(false);
  /** This is the thumbnail saved against the video. */
  readonly saved = input(false);
  readonly downloading = input(false);

  readonly choose = output<void>();
  readonly download = output<void>();
  readonly preview = output<void>();

  readonly conceptLabel = computed(() => CONCEPT_LABELS[this.thumb().concept] ?? 'Thumbnail');
  readonly alt = computed(() => {
    const t = this.thumb();
    const text = t.secondaryText ? `${t.headline}, ${t.secondaryText}` : t.headline;
    return `Thumbnail ${this.number()}, ${this.conceptLabel().toLowerCase()}. Text on the image: ${text}`;
  });
}
