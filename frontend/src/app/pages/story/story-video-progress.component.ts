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
  styles: [
    `
      progress {
        appearance: none;
        height: 0.5rem;
        border-radius: 9999px;
        overflow: hidden;
        background: hsl(var(--muted));
      }
      progress::-webkit-progress-bar {
        background: hsl(var(--muted));
      }
      progress::-webkit-progress-value {
        background: hsl(var(--foreground));
        transition: width 0.6s ease;
      }
      progress::-moz-progress-bar {
        background: hsl(var(--foreground));
      }
      @media (prefers-reduced-motion: reduce) {
        progress::-webkit-progress-value {
          transition: none;
        }
      }
    `,
  ],
  template: `
    <progress
      class="block w-full"
      max="100"
      [value]="percent()"
      aria-label="Video progress"
      [attr.aria-valuetext]="percent() + ' percent'"
    ></progress>
    <p class="mt-2 flex flex-wrap items-baseline justify-between gap-x-4 text-sm">
      <span role="status" data-testid="doing">{{ doing() }}</span>
      <span class="tabular-nums text-[hsl(var(--muted-foreground))]" data-testid="percent">{{ percent() }}%</span>
    </p>

    <ol class="mt-5 grid gap-2 sm:grid-cols-3" aria-label="Steps">
      @for (s of stages; track s.status; let i = $index) {
        <li
          class="flex min-h-11 items-center gap-3 rounded-xl border px-3 py-2"
          [class]="stepClass(i)"
          [attr.aria-current]="state(i) === 'current' ? 'step' : null"
          [attr.data-state]="state(i)"
        >
          <span class="flex size-6 shrink-0 items-center justify-center" aria-hidden="true">
            @if (state(i) === 'done') {
              <svg lucideIcon="check" [size]="18"></svg>
            } @else {
              <span class="block size-2.5 rounded-full border-2" [class]="markerClass(i)"></span>
            }
          </span>
          <span class="min-w-0">
            <span class="block font-medium" [class]="labelClass(i)">{{ s.label }}</span>
            <span class="block text-sm text-[hsl(var(--muted-foreground))]">{{ stateWord(i) }}</span>
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

  // The accent is for the step in progress, and only there (DESIGN.md).
  protected stepClass(i: number): string {
    return this.state(i) === 'current' ? 'border-[#BC5227]' : 'border-[hsl(var(--border))]';
  }

  protected markerClass(i: number): string {
    return this.state(i) === 'current' ? 'border-[#BC5227] bg-[#BC5227]' : 'border-[hsl(var(--muted-foreground))]';
  }

  protected labelClass(i: number): string {
    return this.state(i) === 'waiting' ? 'text-[hsl(var(--muted-foreground))]' : '';
  }

  /** The state in words, because the marker beside it is a picture. */
  protected stateWord(i: number): string {
    return { done: 'Done', current: 'In progress', waiting: 'Waiting' }[this.state(i)];
  }
}
