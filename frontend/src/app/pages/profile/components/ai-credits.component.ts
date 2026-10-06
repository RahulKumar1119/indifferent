import { Component, input, output } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { CreatorStats, CreditUsage } from '../models/creator-stats.model';

/** AI credit balance with progress bar, reset countdown and usage ledger. */
@Component({
  selector: 'app-ai-credits',
  standalone: true,
  imports: [LucideDynamicIcon],
  template: `
    <section class="glass-card p-5 md:p-7" aria-label="AI credits">
      <h2 class="text-lg font-semibold flex items-center gap-2">
        <svg lucideIcon="zap" [size]="19" class="text-[hsl(var(--primary))]" aria-hidden="true"></svg>
        AI Credits
      </h2>

      <div class="mt-3 flex items-baseline gap-2">
        <p class="text-[34px] leading-none font-semibold tracking-tight">{{ stats().creditsUsed }}</p>
        <p class="text-[14px] text-[hsl(var(--muted-foreground))]">/ {{ stats().creditsQuota }} used</p>
      </div>

      <div
        class="mt-3 h-2.5 overflow-hidden rounded-full bg-[hsl(var(--secondary))]"
        role="progressbar"
        [attr.aria-valuenow]="percent()"
        aria-valuemin="0"
        aria-valuemax="100"
        [attr.aria-label]="'Credits used: ' + percent() + ' percent'"
      >
        <div
          class="h-full rounded-full transition-all duration-500"
          [style.width.%]="percent()"
          style="background: linear-gradient(90deg, hsl(var(--primary)), #d96c3d)"
        ></div>
      </div>
      <p class="mt-2 text-[12.5px] text-[hsl(var(--muted-foreground))]">
        {{ remaining() }} credits left · Resets in {{ resetsIn() }} days
      </p>

      <div class="mt-4 flex gap-2">
        <button
          type="button"
          (click)="buy.emit()"
          class="btn-interactive flex-1 inline-flex items-center justify-center gap-2 rounded-full bg-[hsl(var(--foreground))] text-[hsl(var(--background))] py-2.5 text-[13.5px] font-semibold"
        >
          <svg lucideIcon="credit-card" [size]="15" aria-hidden="true"></svg>
          Buy Credits
        </button>
        <button
          type="button"
          (click)="upgrade.emit()"
          class="btn-interactive flex-1 inline-flex items-center justify-center gap-2 rounded-full border border-[hsl(var(--border))] py-2.5 text-[13.5px] font-medium hover:bg-white/5"
        >
          Upgrade
          <svg lucideIcon="arrow-up-right" [size]="15" aria-hidden="true"></svg>
        </button>
      </div>

      <div class="mt-5 border-t border-[hsl(var(--border))] pt-4">
        <p class="text-[11.5px] uppercase tracking-[0.14em] text-[hsl(var(--muted-foreground))]">Recent usage</p>
        <ul class="mt-2 space-y-2">
          @for (u of usage(); track u.label + u.when) {
            <li class="flex items-center justify-between text-[13.5px]">
              <span>
                {{ u.label }}
                <span class="ml-1.5 text-[12px] text-[hsl(var(--muted-foreground))]">{{ u.when }}</span>
              </span>
              <span class="font-semibold tabular-nums">−{{ u.cost }}</span>
            </li>
          } @empty {
            <li class="text-[13.5px] text-[hsl(var(--muted-foreground))]">No usage yet this cycle.</li>
          }
        </ul>
      </div>
    </section>
  `,
})
export class AiCreditsComponent {
  readonly stats = input.required<CreatorStats>();
  readonly usage = input<CreditUsage[]>([]);
  readonly resetsIn = input(12);
  readonly buy = output<void>();
  readonly upgrade = output<void>();

  percent(): number {
    const s = this.stats();
    if (s.creditsQuota <= 0) return 0;
    return Math.min(100, Math.round((s.creditsUsed / s.creditsQuota) * 100));
  }

  remaining(): number {
    return Math.max(0, this.stats().creditsQuota - this.stats().creditsUsed);
  }
}
