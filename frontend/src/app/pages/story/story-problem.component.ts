import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { StoryPlanProblem } from './story-plan-problem';

/**
 * A failure the user can act on: what happened, what to do next, and (when
 * trying again can help) a button to do it. The icon and the words carry the
 * meaning; colour never does.
 *
 * Announced at once (role="alert") for failures, politely (role="status") for
 * a limit the user has to wait out.
 */
@Component({
  selector: 'app-story-problem',
  standalone: true,
  imports: [LucideDynamicIcon],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      class="flex items-start gap-3 rounded-xl border border-[hsl(var(--border))] bg-[hsl(var(--card))] p-4"
      [attr.role]="problem().kind === 'rate_limited' || problem().kind === 'disabled' ? 'status' : 'alert'"
      data-testid="problem"
      [attr.data-kind]="problem().kind"
    >
      <svg
        [lucideIcon]="problem().kind === 'rate_limited' ? 'clock' : 'triangle-alert'"
        [size]="20"
        aria-hidden="true"
        class="mt-0.5 shrink-0 text-[var(--color-error)]"
      ></svg>
      <div class="min-w-0 flex-1">
        <p class="font-semibold text-[hsl(var(--foreground))]">{{ problem().title }}</p>
        <p class="mt-1 text-sm text-[hsl(var(--muted-foreground))] [overflow-wrap:anywhere]">{{ problem().message }}</p>
        @if (actionLabel()) {
          <button
            type="button"
            (click)="action.emit()"
            class="mt-3 inline-flex min-h-11 items-center gap-2 rounded-full border border-[hsl(var(--border))] px-4 text-sm font-medium text-[hsl(var(--foreground))] hover:bg-[hsl(var(--muted))]"
          >
            {{ actionLabel() }}
          </button>
        }
      </div>
    </div>
  `,
})
export class StoryProblemComponent {
  readonly problem = input.required<StoryPlanProblem>();
  /** Text for a button under the message; none when empty. */
  readonly actionLabel = input<string>('');
  readonly action = output<void>();
}
