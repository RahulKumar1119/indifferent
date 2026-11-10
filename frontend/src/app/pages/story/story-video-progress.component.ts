import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { STAGES, VideoStatus, stageIndex } from './story-video.models';

/**
 * How far a video has got: the three things that happen to it, in order, and a
 * bar for the percent the worker reports. Both are real: the stage is the job's
 * status and the percent is the number on the job record.
 *
 * The bar is a native <progress>, which a screen reader reads as a progress bar
 * without any ARIA. It is not announced on every change; the sentence under it
 * names the stage and is announced politely when the stage changes. Nothing
 * pulses or loops: the only movement is the bar filling as the number grows.
 */
@Component({
  selector: 'app-story-video-progress',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [LucideDynamicIcon],
  styles: [`
    :host { display: block; font-family: var(--font-body); }
    progress { display: block; width: 100%; appearance: none; height: 0.5rem; border: 0; border-radius: var(--radius-pill); overflow: hidden; background: var(--color-paper-3); }
    progress::-webkit-progress-bar { background: var(--color-paper-3); }
    progress::-webkit-progress-value { background: var(--color-accent); transition: width 0.6s var(--ease-out); }
    progress::-moz-progress-bar { background: var(--color-accent); }
    @media (prefers-reduced-motion: reduce) { progress::-webkit-progress-value { transition: none; } }
    .under { margin: var(--space-xs) 0 0; display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 0 var(--space-md); font-size: var(--type-sm); }
    .pct { font-variant-numeric: tabular-nums; color: var(--color-ink-2); }
    ol { margin: var(--space-md) 0 0; padding: 0; list-style: none; display: grid; gap: var(--space-xs); }
    @media (min-width: 40rem) { ol { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
    li { display: flex; align-items: center; gap: var(--space-sm); min-height: 44px; padding: var(--space-xs) var(--space-sm); border: 1px solid var(--color-rule); border-radius: var(--radius-control); }
    li.accent { border-color: var(--color-accent); box-shadow: inset 0 0 0 1px var(--color-accent); }
    .mk { width: 1.5rem; height: 1.5rem; flex: none; display: flex; align-items: center; justify-content: center; }
    .done .mk { color: var(--color-positive); }
    .dot { width: 0.625rem; height: 0.625rem; border-radius: 50%; border: 2px solid var(--color-muted); }
    .accent .dot { border-color: var(--color-accent); background: var(--color-accent); }
    .lbl { display: block; font-weight: 600; }
    .waiting .lbl { font-weight: 500; color: var(--color-ink-2); }
    .word { display: block; font-size: var(--type-sm); color: var(--color-ink-2); }
  `],
  template: `
    <progress max="100" [value]="percent()" aria-label="Video progress" [attr.aria-valuetext]="percent() + ' percent'"></progress>
    <p class="under">
      <span role="status" data-testid="doing">{{ doing() }}</span>
      <span class="pct" data-testid="percent">{{ percent() }}%</span>
    </p>

    <ol aria-label="Steps">
      @for (s of stages; track s.status; let i = $index) {
        <li
          [class]="state(i) + (state(i) === 'current' ? ' accent' : '')"
          [attr.aria-current]="state(i) === 'current' ? 'step' : null"
          [attr.data-state]="state(i)"
        >
          <span class="mk" aria-hidden="true">
            @if (state(i) === 'done') {
              <svg lucideIcon="check" [size]="18"></svg>
            } @else {
              <span class="dot"></span>
            }
          </span>
          <span style="min-width: 0">
            <span class="lbl">{{ s.label }}</span>
            <span class="word">{{ stateWord(i) }}</span>
          </span>
        </li>
      }
    </ol>
  `,
})
export class StoryVideoProgressComponent {
  readonly status = input.required<VideoStatus>();
  /** 0 to 100, from the job record. */
  readonly progress = input.required<number>();

  protected readonly stages = STAGES;
  protected readonly percent = computed(() => Math.min(100, Math.max(0, Math.round(this.progress()))));
  private readonly at = computed(() => stageIndex(this.status()));
  protected readonly doing = computed(() => {
    const i = this.at();
    return i < 0 ? 'Starting' : (STAGES[i]?.doing ?? 'Finishing');
  });

  protected state(i: number): 'done' | 'current' | 'waiting' {
    const at = this.at();
    if (i < at) return 'done';
    return i === at ? 'current' : 'waiting';
  }

  /** The state in words, because the marker beside it is a picture. */
  protected stateWord(i: number): string {
    return { done: 'Done', current: 'In progress', waiting: 'Waiting' }[this.state(i)];
  }
}
