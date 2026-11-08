import { Component, HostListener, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { LucideDynamicIcon } from '@lucide/angular';
import { ToastService, Toast } from './toast.service';

/** Fixed top-right toast stack. Mount once per page. */
@Component({
  selector: 'app-toasts',
  standalone: true,
  imports: [CommonModule, LucideDynamicIcon],
  styles: [`
    .toast { background: var(--color-paper-2); color: var(--color-ink); border: 1px solid var(--color-rule); border-radius: var(--radius-surface); font-family: var(--font-body); box-shadow: 0 8px 24px rgb(0 0 0 / 0.25); }
    .dismiss { color: var(--color-muted); }
    .dismiss:hover { background: var(--color-paper-3); color: var(--color-ink); }
    .dismiss:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; }
    .ok { color: var(--color-positive); }
    .bad { color: var(--color-negative); }
    .info { color: var(--color-accent-text); }
  `],
  template: `
    <div class="fixed top-4 right-4 z-[100] flex flex-col gap-2 w-[min(320px,calc(100vw-2rem))]" aria-live="polite">
      @for (toast of toasts$ | async; track toast.id) {
        <div
          class="toast toast-enter px-4 py-3 flex items-start gap-2.5 text-sm cursor-pointer"
          (click)="toasts.dismiss(toast.id)"
          role="status"
        >
          <button
            type="button"
            (click)="toasts.dismiss(toast.id)"
            class="dismiss shrink-0 rounded-[6px] p-1 inline-flex items-center justify-center min-w-8 min-h-8"
            aria-label="Dismiss notification"
          >
            <svg lucideIcon="x" [size]="14"></svg>
          </button>
          @if (toast.kind === 'success') {
            <svg lucideIcon="check" [size]="16" class="ok shrink-0 mt-0.5"></svg>
          } @else if (toast.kind === 'error') {
            <svg lucideIcon="circle-x" [size]="16" class="bad shrink-0 mt-0.5"></svg>
          } @else {
            <svg lucideIcon="circle-check" [size]="16" class="info shrink-0 mt-0.5"></svg>
          }
          <span class="flex-1">{{ toast.message }}</span>
        </div>
      }
    </div>
  `,
})
export class ToastsComponent {
  protected readonly toasts = inject(ToastService);
  protected readonly toasts$ = this.toasts.toasts;
  protected current: Toast[] = [];

  constructor() {
    this.toasts.toasts.subscribe((t) => {
      this.current = t;
    });
  }

  @HostListener('document:keydown.escape')
  onEscape(): void {
    if (this.current.length) {
      this.toasts.dismiss(this.current[this.current.length - 1].id);
    }
  }
}
