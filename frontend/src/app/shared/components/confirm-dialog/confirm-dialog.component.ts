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
  template: `
    @if (open()) {
      <div
        class="fixed inset-0 z-[90] flex items-end sm:items-center justify-center p-4"
        role="alertdialog"
        aria-modal="true"
        [attr.aria-label]="title()"
        (keydown.escape)="onCancel()"
      >
        <div class="absolute inset-0 bg-black/60" (click)="onCancel()" aria-hidden="true"></div>
        <div
          class="relative w-full max-w-md rounded-2xl border border-[hsl(var(--border))] bg-[hsl(var(--card))] p-6 shadow-2xl"
        >
          <div class="flex items-start gap-3.5">
            <span
              class="w-10 h-10 rounded-full bg-red-500/10 text-red-400 flex items-center justify-center shrink-0"
              aria-hidden="true"
            >
              <svg lucideIcon="trash-2" [size]="19"></svg>
            </span>
            <div class="min-w-0">
              <h2 class="text-[16px] font-semibold">{{ title() }}</h2>
              <p class="mt-1 text-[13.5px] text-[hsl(var(--muted-foreground))] leading-relaxed">
                {{ message() }}
              </p>
            </div>
          </div>

          @if (details().length > 0) {
            <ul class="mt-4 rounded-xl border border-[hsl(var(--border))]/70 bg-white/[0.02] px-4 py-3 space-y-1.5 text-[13px] text-[hsl(var(--muted-foreground))]">
              @for (d of details(); track d) {
                <li class="flex items-center gap-2">
                  <span class="w-1 h-1 rounded-full bg-current shrink-0" aria-hidden="true"></span>
                  {{ d }}
                </li>
              }
            </ul>
          }

          <p class="mt-3 text-[12.5px] font-medium text-red-400">This action cannot be undone.</p>

          <div class="mt-5 flex flex-col-reverse sm:flex-row sm:justify-end gap-2">
            <button
              type="button"
              (click)="onCancel()"
              [disabled]="busy()"
              class="rounded-full px-5 py-2.5 text-[13.5px] font-medium border border-[hsl(var(--border))] hover:bg-white/5 disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              type="button"
              (click)="confirmed.emit()"
              [disabled]="busy()"
              class="btn-interactive rounded-full px-5 py-2.5 text-[13.5px] font-semibold bg-red-500 text-white hover:bg-red-600 disabled:opacity-60 inline-flex items-center justify-center gap-2"
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
