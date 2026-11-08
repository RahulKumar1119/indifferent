import { Component, computed, input, output } from '@angular/core';

export type StatePanelKind = 'loading' | 'empty' | 'error';

/**
 * The three non-data states of any region that loads data: loading (a
 * skeleton in the shape of what is coming), empty (what belongs here and the
 * one action that fills it) and error (what broke and what to do). The full
 * state is the page's own.
 */
@Component({
  selector: 'app-state-panel',
  standalone: true,
  styles: [`
    :host { display: block; }
    .panel { font-family: var(--font-body); color: var(--color-ink); padding: var(--space-xl) var(--space-lg); text-align: center; border-radius: var(--radius-surface); }
    .panel.error { background: color-mix(in oklab, var(--color-negative) 8%, var(--color-paper)); border: 1px solid color-mix(in oklab, var(--color-negative) 40%, transparent); text-align: left; padding: var(--space-md) var(--space-lg); display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-md); }
    .title { margin: 0; font-size: var(--type-lg); font-weight: 500; }
    .error .title { font-size: var(--type-md); }
    .message { margin: var(--space-xs) 0 0; font-size: var(--type-sm); color: var(--color-muted); max-width: 52ch; margin-inline: auto; }
    .error .message { margin: var(--space-2xs) 0 0; color: var(--color-ink-2); margin-inline: 0; }
    .action { margin-top: var(--space-lg); }
    .error .action { margin-top: 0; }
    .bars { display: grid; gap: var(--space-sm); }
  `],
  template: `
    @switch (kind()) {
      @case ('loading') {
        <div class="bars" role="status" aria-busy="true" [attr.aria-label]="title() || 'Loading'">
          @for (i of rows(); track i) {
            <div class="ui-skeleton" [style.height.px]="i === 0 ? 28 : 16" [style.width.%]="i === 0 ? 40 : 100"></div>
          }
        </div>
      }
      @case ('error') {
        <div class="panel error" role="alert">
          <div>
            <p class="title">{{ title() }}</p>
            @if (message()) { <p class="message">{{ message() }}</p> }
          </div>
          @if (actionLabel()) {
            <button type="button" class="ui-btn ui-btn--primary action" (click)="action.emit()">{{ actionLabel() }}</button>
          }
        </div>
      }
      @default {
        <div class="panel">
          <p class="title">{{ title() }}</p>
          @if (message()) { <p class="message">{{ message() }}</p> }
          @if (actionLabel()) {
            <button type="button" class="ui-btn ui-btn--primary action" (click)="action.emit()">{{ actionLabel() }}</button>
          }
        </div>
      }
    }
  `,
})
export class StatePanelComponent {
  readonly kind = input<StatePanelKind>('empty');
  readonly title = input('');
  readonly message = input('');
  /** Label of the one action (Retry, Create your first video). Omit for none. */
  readonly actionLabel = input('');
  /** Skeleton rows for the loading state. */
  readonly skeletonRows = input(3);
  readonly action = output<void>();
  protected readonly rows = computed(() => Array.from({ length: this.skeletonRows() }, (_, i) => i));
}
