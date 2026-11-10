import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { LucideDynamicIcon } from '@lucide/angular';
import { SceneMediaView } from './story-plan-edits';
import { MAX_NARRATION_CHARS, PROVIDER_NAMES, PlanScene, charCount } from './story-plan.models';

/**
 * One scene of the plan: its picture (with credit), and its narration to edit.
 * It holds no state of its own; the page owns the drafts and the pending
 * picture and passes them in.
 */
@Component({
  selector: 'app-story-scene-card',
  standalone: true,
  imports: [DecimalPipe, LucideDynamicIcon],
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: [`
    :host { display: block; }
    article { scroll-margin-top: calc(56px + var(--space-md)); }
    header { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: var(--space-2xs) var(--space-md); }
    h3 { margin: 0; font-size: var(--type-lg); font-weight: 600; line-height: 1.25; }
    h3:focus { outline: none; }
    h3:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 4px; border-radius: 2px; }
    .meta { margin: 0; display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-xs); font-size: var(--type-sm); color: var(--color-muted); }
    .edited { color: var(--color-caution); }
    .grid { margin-top: var(--space-md); display: grid; gap: var(--space-lg); }
    @media (min-width: 48rem) { .grid { grid-template-columns: 13rem minmax(0, 1fr); } }
    figure { margin: 0; min-width: 0; }
    .frame { display: flex; align-items: center; justify-content: center; width: 11rem; aspect-ratio: 3 / 4; overflow: hidden; border-radius: var(--radius-control); background: var(--color-paper-3); border: 1px solid var(--color-rule); }
    @media (min-width: 48rem) { .frame { width: 100%; } }
    .frame img, .frame video { width: 100%; height: 100%; object-fit: contain; display: block; }
    .state { margin: 0; padding: var(--space-sm); text-align: center; font-size: var(--type-sm); color: var(--color-ink); }
    .state svg { display: block; margin: 0 auto var(--space-2xs); color: var(--color-ink-2); }
    figcaption { margin-top: var(--space-2xs); font-size: var(--type-sm); color: var(--color-ink-2); overflow-wrap: anywhere; }
    figcaption a { display: inline-flex; align-items: center; min-height: 44px; }
    .acts { margin-top: var(--space-xs); display: flex; flex-wrap: wrap; gap: var(--space-xs); }
    label { display: block; font-size: var(--type-sm); font-weight: 600; }
    textarea { margin-top: var(--space-2xs); min-height: 7rem; resize: vertical; line-height: 1.55; }
    .count { margin: var(--space-2xs) 0 0; font-size: var(--type-xs); color: var(--color-muted); }
    .kw { margin: var(--space-sm) 0 0; font-size: var(--type-sm); color: var(--color-ink-2); }
    .sr { position: absolute; width: 1px; height: 1px; margin: -1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
  `],
  template: `
    <article class="ui-card" [id]="'scene-' + (index() + 1)" [attr.aria-labelledby]="titleId()">
      <header>
        <h3 [id]="titleId()" tabindex="-1">Scene {{ index() + 1 }}</h3>
        <p class="meta">
          <span>About {{ scene().duration | number: '1.0-1' }} s</span>
          @if (edited()) {
            <span class="ui-pill edited" data-testid="edited">Edited, not saved</span>
          }
        </p>
      </header>

      <div class="grid">
        <figure>
          <div class="frame">
            @switch (media().state) {
              @case ('asset') {
                @if (asset(); as a) {
                  @if (a.type === 'video' && a.playUrl) {
                    <video
                      [src]="a.playUrl"
                      [poster]="a.previewUrl ?? ''"
                      controls
                      muted
                      playsinline
                      preload="none"
                      [attr.aria-label]="'Preview clip for scene ' + (index() + 1)"
                    ></video>
                  } @else if (a.previewUrl) {
                    <img [src]="a.previewUrl" [alt]="a.alt || scene().description" />
                  } @else {
                    <p class="state">No preview available</p>
                  }
                }
              }
              @case ('unavailable') {
                <p class="state" data-testid="media-state">
                  <svg lucideIcon="circle-x" [size]="20" aria-hidden="true"></svg>
                  This picture is no longer available
                </p>
              }
              @case ('error') {
                <p class="state" data-testid="media-state">
                  <svg lucideIcon="triangle-alert" [size]="20" aria-hidden="true"></svg>
                  This picture could not be loaded
                </p>
              }
              @default {
                <p class="state" data-testid="media-state">
                  <svg lucideIcon="image" [size]="20" aria-hidden="true"></svg>
                  No picture chosen
                </p>
              }
            }
          </div>

          <figcaption>
            @switch (media().state) {
              @case ('asset') {
                @if (asset(); as a) {
                  <a [href]="a.pageUrl" target="_blank" rel="noopener noreferrer" class="ui-link" data-testid="credit"
                    >{{ a.credit }}<span class="sr"> (opens in a new tab)</span></a
                  >
                }
              }
              @case ('unavailable') {
                {{ providerName() }} no longer has it. Choose another.
              }
              @case ('error') {
                Try again later, or choose another.
              }
              @default {
                Suggested: {{ scene().description }}
              }
            }
          </figcaption>

          <div class="acts">
            <button type="button" [id]="chooseId()" (click)="choose.emit()" [disabled]="disabled()" class="ui-btn ui-btn--secondary">
              <svg lucideIcon="image" [size]="16" aria-hidden="true"></svg>
              {{ media().state === 'none' ? 'Choose picture' : 'Replace picture' }}
              <span class="sr"> for scene {{ index() + 1 }}</span>
            </button>
            @if (media().state !== 'none') {
              <button type="button" (click)="remove.emit()" [disabled]="disabled()" class="ui-btn ui-btn--quiet">
                <svg lucideIcon="trash-2" [size]="16" aria-hidden="true"></svg>
                Remove picture
                <span class="sr"> from scene {{ index() + 1 }}</span>
              </button>
            }
          </div>
        </figure>

        <div style="min-width: 0">
          <label [for]="textareaId()">Narration</label>
          <textarea
            [id]="textareaId()"
            rows="5"
            [value]="narration()"
            (input)="onInput($event)"
            [readOnly]="disabled()"
            [attr.lang]="language()"
            [attr.maxlength]="maxChars"
            [attr.aria-describedby]="countId()"
            class="ui-field"
          ></textarea>
          <p [id]="countId()" class="count">{{ chars() }} of {{ maxChars }} characters</p>
          <p class="kw">Search words: {{ scene().keywords.join(', ') }}</p>
        </div>
      </div>
    </article>
  `,
})
export class StorySceneCardComponent {
  readonly index = input.required<number>();
  readonly scene = input.required<PlanScene>();
  /** The narration as it is now, with any unsaved typing. */
  readonly narration = input.required<string>();
  readonly media = input.required<SceneMediaView>();
  /** ISO 639-1 code, so screen readers use the right voice. */
  readonly language = input<string>('');
  /** True while a save is in flight. */
  readonly disabled = input<boolean>(false);
  /** True when this scene has a change that is not saved. */
  readonly edited = input<boolean>(false);

  readonly narrationChange = output<string>();
  readonly choose = output<void>();
  readonly remove = output<void>();

  protected readonly maxChars = MAX_NARRATION_CHARS;
  protected readonly titleId = computed(() => `scene-${this.index() + 1}-title`);
  protected readonly textareaId = computed(() => `scene-${this.index() + 1}-narration`);
  protected readonly countId = computed(() => `scene-${this.index() + 1}-count`);
  protected readonly chooseId = computed(() => `scene-${this.index() + 1}-choose`);
  protected readonly chars = computed(() => charCount(this.narration()));
  protected readonly asset = computed(() => {
    const m = this.media();
    return m.state === 'asset' ? m.asset : null;
  });
  protected readonly providerName = computed(() => {
    const m = this.media();
    return m.state === 'unavailable' ? PROVIDER_NAMES[m.ref.provider] : '';
  });

  protected onInput(event: Event): void {
    this.narrationChange.emit((event.target as HTMLTextAreaElement).value);
  }
}
