import { Component, input, output } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';

/**
 * Destructive-action confirmation dialog. Parent owns visibility (`open`)
 * and the in-flight flag (`busy`); this component only renders and emits.
 * Full-width buttons on mobile, works with keyboard (Esc cancels).
 */
@Component({
  selector: 'app-confirm-dialog',
  standalone: true,
  imports: [LucideDynamicIcon],
  styles: [`
    .scrim { background: color-mix(in oklab, var(--color-paper) 55%, black); opacity: 0.8; }
    .sheet { background: var(--color-paper-2); color: var(--color-ink); border: 1px solid var(--color-rule); border-radius: var(--radius-surface); font-family: var(--font-body); box-shadow: 0 24px 60px rgb(0 0 0 / 0.35); }
    .icon { background: color-mix(in oklab, var(--color-negative) 14%, transparent); color: var(--color-negative); }
    .muted { color: var(--color-muted); }
    .danger { background: var(--color-negative); color: var(--color-paper); }
    .danger:hover:not(:disabled) { background: color-mix(in oklab, var(--color-negative) 85%, var(--color-ink)); }
    .warn { color: var(--color-negative); }
    .list { background: var(--color-paper); border: 1px solid var(--color-rule); color: var(--color-muted); }
  `],
  template: `
    @if (open()) {
      <div
        class="fixed inset-0 z-[90] flex items-end sm:items-center justify-center p-4"
        role="alertdialog"
        aria-modal="true"
        [attr.aria-label]="title()"
        (keydown.escape)="onCancel()"
      >
        <div class="scrim absolute inset-0" (click)="onCancel()" aria-hidden="true"></div>
        <div
          class="sheet hm relative w-full max-w-md p-6"
        >
          <div class="flex items-start gap-3.5">
            <span
              class="icon w-10 h-10 rounded-full flex items-center justify-center shrink-0"
              aria-hidden="true"
            >
              <svg lucideIcon="trash-2" [size]="19"></svg>
            </span>
            <div class="min-w-0">
              <h2 class="text-[16px] font-semibold">{{ title() }}</h2>
              <p class="muted mt-1 text-[13.5px] leading-relaxed">
                {{ message() }}
              </p>
            </div>
          </div>

          @if (details().length > 0) {
            <ul class="list mt-4 rounded-xl px-4 py-3 space-y-1.5 text-[13px]">
              @for (d of details(); track d) {
                <li class="flex items-center gap-2">
                  <span class="w-1 h-1 rounded-full bg-current shrink-0" aria-hidden="true"></span>
                  {{ d }}
                </li>
              }
            </ul>
          }

          <p class="warn mt-3 text-[12.5px] font-medium">This action cannot be undone.</p>

          <div class="mt-5 flex flex-col-reverse sm:flex-row sm:justify-end gap-2">
            <button
              type="button"
              (click)="onCancel()"
              [disabled]="busy()"
              class="ui-btn ui-btn--secondary"
            >
              Cancel
            </button>
            <button
              type="button"
              (click)="confirmed.emit()"
              [disabled]="busy()"
              class="ui-btn danger"
            >
              @if (busy()) {
                <svg lucideIcon="loader-2" [size]="15" class="animate-spin" aria-hidden="true"></svg>
                Deleting…
              } @else {
                {{ confirmLabel() }}
              }
            </button>
          </div>
        </div>
      </div>
    }
  `,
})
export class ConfirmDialogComponent {
  readonly open = input(false);
  readonly title = input('Delete?');
  readonly message = input('Are you sure?');
  readonly details = input<string[]>([]);
  readonly confirmLabel = input('Delete Permanently');
  readonly busy = input(false);
  readonly confirmed = output<void>();
  readonly cancelled = output<void>();

  onCancel(): void {
    if (!this.busy()) this.cancelled.emit();
  }
}
