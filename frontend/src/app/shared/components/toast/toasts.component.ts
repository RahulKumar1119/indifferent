import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { LucideDynamicIcon } from '@lucide/angular';
import { ToastService } from './toast.service';

/** Fixed top-right toast stack. Mount once per page. */
@Component({
  selector: 'app-toasts',
  standalone: true,
  imports: [CommonModule, LucideDynamicIcon],
  template: `
    <div class="fixed top-4 right-4 z-[100] flex flex-col gap-2 w-[min(320px,calc(100vw-2rem))]" aria-live="polite">
      @for (toast of toasts$ | async; track toast.id) {
        <div
          class="toast-enter glass-card px-4 py-3 flex items-start gap-2.5 text-sm cursor-pointer"
          (click)="toasts.dismiss(toast.id)"
          role="status"
        >
          @if (toast.kind === 'success') {
            <svg lucideIcon="check" [size]="16" class="text-green-400 shrink-0 mt-0.5"></svg>
          } @else if (toast.kind === 'error') {
            <svg lucideIcon="circle-x" [size]="16" class="text-red-400 shrink-0 mt-0.5"></svg>
          } @else {
            <svg lucideIcon="circle-check" [size]="16" class="text-[hsl(var(--primary))] shrink-0 mt-0.5"></svg>
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
}
