import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { StoryPlanProblem } from './story-plan-problem';

/**
 * A failure the user can act on: what happened, what to do next, and (when
 * trying again can help) a button to do it. The icon and the words carry the
 * meaning; colour never does.
 *
 * Announced at once (role="alert") for failures, politely (role="status") for
 * a limit the user has to wait out, or when `polite` says the page is only
 * reporting something that already happened (a failure from an earlier visit).
 */
@Component({
  selector: 'app-story-problem',
  standalone: true,
  imports: [LucideDynamicIcon],
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: [`
    :host { display: block; }
    .box { display: flex; align-items: flex-start; gap: var(--space-sm); padding: var(--space-md); border-radius: var(--radius-control); border: 1px solid color-mix(in oklab, var(--color-negative) 45%, transparent); background: color-mix(in oklab, var(--color-negative) 6%, var(--app-card, var(--color-paper))); color: var(--color-ink); font-family: var(--font-body); }
    .box.wait { border-color: color-mix(in oklab, var(--color-caution) 50%, transparent); background: color-mix(in oklab, var(--color-caution) 8%, var(--app-card, var(--color-paper))); }
    .ic { flex: none; margin-top: 2px; color: var(--color-negative); }
    .wait .ic { color: var(--color-ink-2); }
    .body { flex: 1; min-width: 0; }
    .title { margin: 0; font-weight: 600; }
    .msg { margin: var(--space-3xs) 0 0; font-size: var(--type-sm); color: var(--color-ink-2); overflow-wrap: anywhere; }
    .ui-btn { margin-top: var(--space-sm); }
  `],
  template: `
    <div
      class="box"
      [class.wait]="problem().kind === 'rate_limited' || problem().kind === 'disabled'"
      [attr.role]="polite() || problem().kind === 'rate_limited' || problem().kind === 'disabled' ? 'status' : 'alert'"
      data-testid="problem"
      [attr.data-kind]="problem().kind"
    >
      <svg [lucideIcon]="problem().kind === 'rate_limited' ? 'clock' : 'triangle-alert'" [size]="20" aria-hidden="true" class="ic"></svg>
      <div class="body">
        <p class="title">{{ problem().title }}</p>
        <p class="msg">{{ problem().message }}</p>
        @if (actionLabel()) {
          <button type="button" (click)="action.emit()" class="ui-btn ui-btn--secondary">{{ actionLabel() }}</button>
        }
      </div>
    </div>
  `,
})
export class StoryProblemComponent {
  readonly problem = input.required<StoryPlanProblem>();
  readonly polite = input(false);
  /** Text for a button under the message; none when empty. */
  readonly actionLabel = input<string>('');
  readonly action = output<void>();
}
