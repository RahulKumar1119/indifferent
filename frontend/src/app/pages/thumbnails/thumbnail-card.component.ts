import { Component, computed, input, output } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { CONCEPT_LABELS, Thumbnail } from './thumbnail.service';

/** One thumbnail suggestion: a preview image with Select and Download. */
@Component({
  selector: 'app-thumbnail-card',
  standalone: true,
  imports: [LucideDynamicIcon],
  template: `
    <article class="glass-card card flex h-full flex-col overflow-hidden" [class.chosen]="chosen()">
      <button
        type="button"
        class="relative block w-full cursor-zoom-in p-0 text-left"
        (click)="preview.emit()"
        [attr.aria-label]="'Preview thumbnail ' + number() + ', ' + conceptLabel()"
      >
        @if (thumb().url) {
          <img
            [src]="thumb().url"
            [alt]="alt()"
            [attr.width]="thumb().width"
            [attr.height]="thumb().height"
            loading="lazy"
            class="block h-auto w-full bg-black/20"
          />
        } @else {
          <span class="flex aspect-video w-full items-center justify-center bg-black/20 text-xs text-[hsl(var(--muted-foreground))]">
            Image unavailable
          </span>
        }
        <span class="tag">{{ conceptLabel() }}</span>
        @if (saved()) {
          <span class="saved">
            <svg lucideIcon="check" [size]="12" aria-hidden="true"></svg>
            Your thumbnail
          </span>
        }
      </button>
      <div class="flex items-center gap-2 p-3">
        <button
          type="button"
          class="btn-interactive flex-1 rounded-lg border px-3 py-2 text-sm font-medium"
          [class.pick]="chosen()"
          [attr.aria-pressed]="chosen()"
          (click)="choose.emit()"
        >
          {{ chosen() ? 'Selected' : 'Select' }}
          <span class="sr-only"> thumbnail {{ number() }}</span>
        </button>
        <button
          type="button"
          class="btn-interactive inline-flex items-center gap-1.5 rounded-lg border px-3 py-2 text-sm font-medium disabled:opacity-60"
          [disabled]="downloading()"
          (click)="download.emit()"
          [attr.aria-label]="'Download thumbnail ' + number()"
        >
          @if (downloading()) {
            <svg lucideIcon="loader-2" [size]="15" class="animate-spin" aria-hidden="true"></svg>
          } @else {
            <svg lucideIcon="download" [size]="15" aria-hidden="true"></svg>
          }
          <span class="hidden sm:inline">Download</span>
        </button>
      </div>
    </article>
  `,
  styles: [`
    .card { border: 2px solid transparent; transition: border-color .15s; }
    .card.chosen { border-color: hsl(var(--primary)); }
    button { border-color: hsl(var(--border)); }
    button.pick { border-color: hsl(var(--primary)); background: hsl(var(--primary) / 0.14); }
    .tag, .saved {
      position: absolute; left: 8px; padding: 2px 8px; border-radius: 9999px;
      font-size: 11px; font-weight: 600; background: rgba(0, 0, 0, 0.72); color: #fff;
    }
    .tag { top: 8px; }
    /* Top right, so it never covers the text drawn on the thumbnail. */
    .saved { top: 8px; left: auto; right: 8px; display: inline-flex; align-items: center; gap: 4px; background: #15803d; }
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
