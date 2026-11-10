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
  styles: [`
    :host { display: block; }
    ul { margin: 0; padding: 0; list-style: none; display: flex; flex-wrap: wrap; gap: 0 var(--space-lg); font-size: var(--type-sm); }
    a { display: inline-flex; align-items: center; min-height: 44px; }
    .sr { position: absolute; width: 1px; height: 1px; margin: -1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
  `],
  template: `
    @if (attributions().length) {
      <ul data-testid="attribution">
        @for (a of attributions(); track a.provider) {
          <li>
            <a [href]="a.url" target="_blank" rel="noopener noreferrer" class="ui-link">{{ a.text }}<span class="sr"> (opens in a new tab)</span></a>
          </li>
        }
      </ul>
    }
  `,
})
export class StoryAttributionComponent {
  readonly attributions = input<StockAttribution[]>([]);
}
