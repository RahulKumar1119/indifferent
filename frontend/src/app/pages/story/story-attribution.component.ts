import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { StockAttribution } from './story-plan.models';

/**
 * The credit Pexels and Pixabay ask for wherever their results are shown
 * (see backend/internal/stockmedia/TERMS.md). The text and link come from the
 * API, one entry per provider whose pictures are on screen.
 */
@Component({
  selector: 'app-story-attribution',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    @if (attributions().length) {
      <ul class="flex flex-wrap gap-x-6 text-sm text-[hsl(var(--muted-foreground))]" data-testid="attribution">
        @for (a of attributions(); track a.provider) {
          <li>
            <a
              [href]="a.url"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex min-h-11 items-center underline underline-offset-2 hover:text-[hsl(var(--foreground))]"
              >{{ a.text }}<span class="sr-only"> (opens in a new tab)</span></a
            >
          </li>
        }
      </ul>
    }
  `,
})
export class StoryAttributionComponent {
  readonly attributions = input<StockAttribution[]>([]);
}
