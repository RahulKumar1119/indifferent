import { Component, ElementRef, computed, effect, input, output, viewChild } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { CONCEPT_LABELS, Thumbnail } from './thumbnail.service';

/**
 * A large preview in a native modal dialog. The browser provides the focus
 * trap, the Escape key and the backdrop; this only opens and closes it in step
 * with the thumbnail it is given.
 */
@Component({
  selector: 'app-thumbnail-preview',
  standalone: true,
  imports: [LucideDynamicIcon],
  template: `
    <dialog #dialog class="preview hm" aria-labelledby="thumb-preview-title" (close)="closed.emit()" (click)="onBackdropClick($event)">
      @if (thumb(); as t) {
        <div class="body">
          <div class="head">
            <h2 id="thumb-preview-title">Thumbnail {{ number() }} <span class="kind">{{ label() }}</span></h2>
            <button type="button" class="ui-btn ui-btn--quiet close" (click)="close()" aria-label="Close preview">
              <svg lucideIcon="x" [size]="18" aria-hidden="true"></svg>
            </button>
          </div>
          @if (t.url) {
            <img [src]="t.url" [alt]="'Thumbnail ' + number() + '. Text on the image: ' + t.headline" [attr.width]="t.width" [attr.height]="t.height" />
          }
          <div class="acts">
            <button type="button" class="ui-btn ui-btn--secondary" (click)="download.emit()">
              <svg lucideIcon="download" [size]="16" aria-hidden="true"></svg>Download
            </button>
            <button type="button" class="ui-btn ui-btn--primary" (click)="choose.emit(); close()">
              <svg lucideIcon="check" [size]="16" aria-hidden="true"></svg>Select this one
            </button>
          </div>
        </div>
      }
    </dialog>
  `,
  styles: [`
    /* margin: auto puts the modal back in the centre; the global reset zeroes it. */
    dialog.preview { margin: auto; width: min(960px, calc(100vw - 24px)); max-height: calc(100dvh - 24px); padding: 0; border: 1px solid var(--color-rule); border-radius: var(--radius-surface); background: var(--color-paper); color: var(--color-ink); font-family: var(--font-body); overflow: auto; }
    /* Same scrim as the shared confirm dialog: dark in both themes. */
    dialog.preview::backdrop { background: color-mix(in oklab, var(--color-paper) 55%, black); opacity: 0.8; }
    .body { padding: var(--space-md); }
    @media (min-width: 40rem) { .body { padding: var(--space-lg); } }
    .head { display: flex; align-items: center; justify-content: space-between; gap: var(--space-sm); margin-bottom: var(--space-sm); }
    h2 { margin: 0; font-size: var(--type-md); font-weight: 600; display: flex; flex-wrap: wrap; align-items: baseline; gap: var(--space-xs); }
    .kind { font-weight: 400; font-size: var(--type-sm); color: var(--color-muted); }
    .close { width: 44px; padding: 0; }
    img { display: block; margin-inline: auto; height: auto; width: auto; max-width: 100%; max-height: 68vh; border-radius: var(--radius-control); }
    .acts { margin-top: var(--space-md); display: flex; flex-wrap: wrap; justify-content: flex-end; gap: var(--space-xs); }
  `],
})
export class ThumbnailPreviewComponent {
  /** The thumbnail to show; null keeps the dialog closed. */
  readonly thumb = input<Thumbnail | null>(null);
  readonly number = input(1);

  readonly closed = output<void>();
  readonly choose = output<void>();
  readonly download = output<void>();

  private readonly dialog = viewChild<ElementRef<HTMLDialogElement>>('dialog');

  readonly label = computed(() => {
    const t = this.thumb();
    return t ? (CONCEPT_LABELS[t.concept] ?? 'Thumbnail') : '';
  });

  constructor() {
    effect(() => {
      const el = this.dialog()?.nativeElement;
      const open = this.thumb() !== null;
      // Server rendering has no dialog methods.
      if (!el || typeof el.showModal !== 'function') return;
      if (open && !el.open) el.showModal();
      if (!open && el.open) el.close();
    });
  }

  close(): void {
    const el = this.dialog()?.nativeElement;
    if (el?.open) el.close();
  }

  /** A click on the backdrop lands on the dialog element itself. */
  onBackdropClick(event: MouseEvent): void {
    if (event.target === this.dialog()?.nativeElement) this.close();
  }
}
