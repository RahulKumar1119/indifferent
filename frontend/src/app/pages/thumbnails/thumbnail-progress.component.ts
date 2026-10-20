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
    <ol class="m-0 list-none space-y-3 p-0" role="status" aria-live="polite" aria-label="Thumbnail progress">
      <li class="flex items-center gap-3">
        <span class="dot dot-done"><svg lucideIcon="check" [size]="14" aria-hidden="true"></svg></span>
        <span class="text-sm">Video uploaded<span class="sr-only"> (done)</span></span>
      </li>
      @for (s of steps(); track s.key) {
        <li class="flex items-center gap-3" [attr.aria-current]="s.state === 'active' ? 'step' : null">
          <span class="dot" [class.dot-done]="s.state === 'done'" [class.dot-active]="s.state === 'active'">
            @if (s.state === 'done') {
              <svg lucideIcon="check" [size]="14" aria-hidden="true"></svg>
            } @else if (s.state === 'active') {
              <svg lucideIcon="loader-2" [size]="14" class="spin" aria-hidden="true"></svg>
            }
          </span>
          <span class="text-sm" [class.text-muted]="s.state === 'pending'" [class.font-medium]="s.state === 'active'">
            {{ s.label }}
            <span class="sr-only">{{ s.state === 'done' ? ' (done)' : s.state === 'active' ? ' (in progress)' : ' (waiting)' }}</span>
          </span>
        </li>
      }
    </ol>
  `,
  styles: [`
    .dot {
      display: inline-flex; align-items: center; justify-content: center;
      width: 22px; height: 22px; border-radius: 9999px; flex-shrink: 0;
      border: 2px solid hsl(var(--border));
    }
    .dot-done { background: #16a34a; border-color: #16a34a; color: #fff; }
    .dot-active { border-color: hsl(var(--primary)); color: hsl(var(--primary)); }
    .text-muted { color: hsl(var(--muted-foreground)); }
    .spin { animation: spin 1s linear infinite; }
    @keyframes spin { to { transform: rotate(360deg); } }
    @media (prefers-reduced-motion: reduce) { .spin { animation: none; } }
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
