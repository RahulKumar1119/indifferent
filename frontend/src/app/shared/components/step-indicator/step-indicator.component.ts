import { Component, input } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';

/**
 * A small "step 1 of 2" indicator for multi-page flows (quiz: Details, then
 * Questions). It only shows where you are; moving between steps stays with
 * the page, so validation and navigation keep their own rules.
 */
@Component({
  selector: 'app-step-indicator',
  standalone: true,
  imports: [LucideDynamicIcon],
  styles: [`
    :host { display: block; }
    ol { margin: 0; padding: 0; list-style: none; display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-xs) var(--space-sm); font-family: var(--font-body); font-size: var(--type-sm); }
    li { display: inline-flex; align-items: center; gap: var(--space-xs); color: var(--color-muted); }
    li + li::before { content: ''; width: var(--space-lg); height: 1px; background: var(--color-muted); }
    .n { width: 1.75rem; height: 1.75rem; flex: none; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; border: 1px solid var(--color-muted); font-weight: 600; font-variant-numeric: tabular-nums; }
    .now { color: var(--color-ink); font-weight: 600; }
    .now .n { background: var(--color-accent); border-color: var(--color-accent); color: var(--color-accent-ink); }
    .done { color: var(--color-ink-2); }
    .done .n { background: var(--color-ink); border-color: var(--color-ink); color: var(--color-paper); }
    .sr { position: absolute; width: 1px; height: 1px; margin: -1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
  `],
  template: `
    <ol [attr.aria-label]="label()">
      @for (step of steps(); track step; let i = $index) {
        <li
          [class.now]="i === current()"
          [class.done]="i < current()"
          [attr.aria-current]="i === current() ? 'step' : null"
          [attr.data-testid]="'step-' + (i + 1)"
        >
          <span class="n" aria-hidden="true">
            @if (i < current()) {
              <svg lucideIcon="check" [size]="14"></svg>
            } @else {
              {{ i + 1 }}
            }
          </span>
          <span>
            <span class="sr">Step {{ i + 1 }} of {{ steps().length }}: </span>{{ step }}@if (i < current()) {<span class="sr"> (done)</span>}
          </span>
        </li>
      }
    </ol>
  `,
})
export class StepIndicatorComponent {
  readonly steps = input.required<readonly string[]>();
  /** Zero-based index of the step the page is on. */
  readonly current = input(0);
  readonly label = input('Progress');
}
