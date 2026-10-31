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
  styles: [
    `
      .serif {
        font-family: 'Cormorant Garamond', Georgia, serif;
      }
    `,
  ],
  template: `
    <article
      class="rounded-2xl border border-[hsl(var(--border))] bg-[hsl(var(--card))] p-4 sm:p-5"
      [attr.aria-labelledby]="titleId()"
    >
      <header class="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <h3 [id]="titleId()" class="serif text-2xl font-medium leading-tight">Scene {{ index() + 1 }}</h3>
        <p class="text-sm text-[hsl(var(--muted-foreground))]">
          About {{ scene().duration | number: '1.0-1' }} s
          @if (edited()) {
            <span class="font-semibold text-[hsl(var(--foreground))]" data-testid="edited">· Edited, not saved</span>
          }
        </p>
      </header>

      <div class="mt-4 grid gap-5 md:grid-cols-[13rem_minmax(0,1fr)]">
        <figure class="m-0 min-w-0">
          <div
            class="flex aspect-[3/4] w-44 items-center justify-center overflow-hidden rounded-xl bg-[hsl(var(--muted))] md:w-full"
          >
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
                      class="size-full object-contain"
                    ></video>
                  } @else if (a.previewUrl) {
                    <img [src]="a.previewUrl" [alt]="a.alt || scene().description" class="size-full object-contain" />
                  } @else {
                    <p class="p-3 text-center text-sm">No preview available</p>
                  }
                }
              }
              @case ('unavailable') {
                <p class="p-3 text-center text-sm text-[hsl(var(--foreground))]" data-testid="media-state">
                  <svg lucideIcon="circle-x" [size]="20" aria-hidden="true" class="mx-auto mb-2"></svg>
                  This picture is no longer available
                </p>
              }
              @case ('error') {
                <p class="p-3 text-center text-sm text-[hsl(var(--foreground))]" data-testid="media-state">
                  <svg lucideIcon="triangle-alert" [size]="20" aria-hidden="true" class="mx-auto mb-2"></svg>
                  This picture could not be loaded
                </p>
              }
              @default {
                <p class="p-3 text-center text-sm text-[hsl(var(--foreground))]" data-testid="media-state">
                  <svg lucideIcon="image" [size]="20" aria-hidden="true" class="mx-auto mb-2"></svg>
                  No picture chosen
                </p>
              }
            }
          </div>

          <figcaption class="mt-2 text-sm text-[hsl(var(--muted-foreground))]">
            @switch (media().state) {
              @case ('asset') {
                @if (asset(); as a) {
                  <a
                    [href]="a.pageUrl"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="inline-flex min-h-11 items-center underline underline-offset-2 [overflow-wrap:anywhere]"
                    data-testid="credit"
                    >{{ a.credit }}<span class="sr-only"> (opens in a new tab)</span></a
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

          <div class="mt-2 flex flex-wrap gap-2">
            <button
              type="button"
              [id]="chooseId()"
              (click)="choose.emit()"
              [disabled]="disabled()"
              class="inline-flex min-h-11 items-center gap-2 rounded-full border border-[hsl(var(--foreground))] px-4 text-sm font-semibold hover:bg-[hsl(var(--foreground))] hover:text-[hsl(var(--background))] disabled:opacity-60"
            >
              <svg lucideIcon="image" [size]="16" aria-hidden="true"></svg>
              {{ media().state === 'none' ? 'Choose picture' : 'Replace picture' }}
              <span class="sr-only"> for scene {{ index() + 1 }}</span>
            </button>
            @if (media().state !== 'none') {
              <button
                type="button"
                (click)="remove.emit()"
                [disabled]="disabled()"
                class="inline-flex min-h-11 items-center gap-2 rounded-full border border-[hsl(var(--border))] px-4 text-sm font-medium hover:bg-[hsl(var(--muted))] disabled:opacity-60"
              >
                <svg lucideIcon="trash-2" [size]="16" aria-hidden="true"></svg>
                Remove picture
                <span class="sr-only"> from scene {{ index() + 1 }}</span>
              </button>
            }
          </div>
        </figure>

        <div class="min-w-0">
          <label [for]="textareaId()" class="block text-sm font-medium">Narration</label>
          <textarea
            [id]="textareaId()"
            rows="5"
            [value]="narration()"
            (input)="onInput($event)"
            [readOnly]="disabled()"
            [attr.lang]="language()"
            [attr.maxlength]="maxChars"
            [attr.aria-describedby]="countId()"
            class="mt-1 block min-h-28 w-full resize-y rounded-lg border border-[hsl(var(--border))] bg-[hsl(var(--background))] px-3 py-2 text-base leading-relaxed focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-primary)]"
          ></textarea>
          <p [id]="countId()" class="mt-1 text-sm text-[hsl(var(--muted-foreground))]">
            {{ chars() }} of {{ maxChars }} characters
          </p>
          <p class="mt-3 text-sm text-[hsl(var(--muted-foreground))]">
            Search words: {{ scene().keywords.join(', ') }}
          </p>
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
