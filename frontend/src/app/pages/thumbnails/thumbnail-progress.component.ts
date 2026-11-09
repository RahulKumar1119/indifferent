import { Component, computed, input } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { THUMBNAIL_STAGES, ThumbnailStage, stageIndex } from './thumbnail.service';

type StepState = 'done' | 'active' | 'pending';

/**
 * The step list shown while thumbnails are made. The first step is always
 * done: a generation only exists once the video is uploaded.
 */
@Component({
  selector: 'app-thumbnail-progress',
  standalone: true,
  imports: [LucideDynamicIcon],
  template: `
    <ol class="steps" role="status" aria-live="polite" aria-label="Thumbnail progress">
      <li>
        <span class="dot done"><svg lucideIcon="check" [size]="14" aria-hidden="true"></svg></span>
        <span class="lbl">Video uploaded<span class="sr-only"> (done)</span></span>
      </li>
      @for (s of steps(); track s.key) {
        <li [attr.aria-current]="s.state === 'active' ? 'step' : null" [attr.data-state]="s.state">
          <span class="dot" [class.done]="s.state === 'done'" [class.active]="s.state === 'active'">
            @if (s.state === 'done') {
              <svg lucideIcon="check" [size]="14" aria-hidden="true"></svg>
            } @else if (s.state === 'active') {
              <svg lucideIcon="loader-2" [size]="14" class="ui-spin" aria-hidden="true"></svg>
            }
          </span>
          <span class="lbl" [class.wait]="s.state === 'pending'" [class.now]="s.state === 'active'">
            {{ s.label }}
            <span class="sr-only">{{ s.state === 'done' ? ' (done)' : s.state === 'active' ? ' (in progress)' : ' (waiting)' }}</span>
          </span>
        </li>
      }
    </ol>
  `,
  styles: [`
    .steps { margin: 0; padding: 0; list-style: none; display: flex; flex-direction: column; gap: var(--space-sm); }
    li { display: flex; align-items: center; gap: var(--space-sm); }
    .dot { width: 1.75rem; height: 1.75rem; flex: none; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center; border: 2px solid var(--color-rule); }
    .dot.done { background: var(--color-positive); border-color: var(--color-positive); color: var(--color-paper); }
    .dot.active { border-color: var(--color-accent); color: var(--color-accent-text); }
    .lbl { font-size: var(--type-sm); color: var(--color-ink); }
    .lbl.wait { color: var(--color-muted); }
    .lbl.now { font-weight: 600; }
  `],
})
export class ThumbnailProgressComponent {
  /** The stage that is running now; absent while the run is still queued. */
  readonly stage = input<ThumbnailStage | undefined>(undefined);

  readonly steps = computed(() => {
    const current = stageIndex(this.stage());
    return THUMBNAIL_STAGES.map((s, i) => {
      // Queued (no stage yet): the first step is the one about to run.
      const active = current === -1 ? 0 : current;
      const state: StepState = i < active ? 'done' : i === active ? 'active' : 'pending';
      return { key: s.key, label: s.label, state };
    });
  });
}
