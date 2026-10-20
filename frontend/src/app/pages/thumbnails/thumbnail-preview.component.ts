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
    <dialog
      #dialog
      class="preview"
      aria-labelledby="thumb-preview-title"
      (close)="closed.emit()"
      (click)="onBackdropClick($event)"
    >
      @if (thumb(); as t) {
        <div class="p-4 sm:p-5">
          <div class="mb-3 flex items-center justify-between gap-3">
            <h2 id="thumb-preview-title" class="text-base font-semibold">
              Thumbnail {{ number() }} <span class="font-normal text-[hsl(var(--muted-foreground))]">· {{ label() }}</span>
            </h2>
            <button type="button" class="icon-btn" (click)="close()" aria-label="Close preview">
              <svg lucideIcon="x" [size]="18" aria-hidden="true"></svg>
            </button>
          </div>
          @if (t.url) {
            <img
              [src]="t.url"
              [alt]="'Thumbnail ' + number() + '. Text on the image: ' + t.headline"
              [attr.width]="t.width"
              [attr.height]="t.height"
              class="mx-auto block h-auto max-h-[68vh] w-auto max-w-full rounded-lg"
            />
          }
          <div class="mt-4 flex flex-wrap items-center justify-end gap-2">
            <button type="button" class="act" (click)="download.emit()">
              <svg lucideIcon="download" [size]="15" aria-hidden="true"></svg> Download
            </button>
            <button type="button" class="act primary" (click)="choose.emit(); close()">
              <svg lucideIcon="check" [size]="15" aria-hidden="true"></svg> Select this one
            </button>
          </div>
        </div>
      }
    </dialog>
  `,
  styles: [`
    dialog.preview {
      width: min(960px, calc(100vw - 24px));
      max-height: calc(100dvh - 24px);
      padding: 0; border: 1px solid hsl(var(--border)); border-radius: 12px;
      background: hsl(var(--background)); color: hsl(var(--foreground));
      overflow: auto;
    }
    dialog.preview::backdrop { background: rgba(0, 0, 0, 0.7); }
    .icon-btn { display: inline-flex; padding: 6px; border-radius: 8px; }
    .icon-btn:hover { background: hsl(var(--muted)); }
    .act {
      display: inline-flex; align-items: center; gap: 6px; padding: 8px 14px;
      border: 1px solid hsl(var(--border)); border-radius: 8px; font-size: 14px; font-weight: 500;
    }
    .act:hover { background: hsl(var(--muted)); }
    .act.primary { background: hsl(var(--primary)); border-color: hsl(var(--primary)); color: #fff; }
    .act.primary:hover { filter: brightness(1.08); }
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
