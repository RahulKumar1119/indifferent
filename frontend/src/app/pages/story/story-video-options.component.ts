import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import {
  CAPTION_PRESETS,
  CaptionPreset,
  languageName,
  voiceNote,
} from './story-video.models';

/** What the person chose: the voice, and which of the caption looks. */
export interface VideoOptions {
  voice: string;
  caption: CaptionPreset;
}

/**
 * The two choices that shape a video: who reads it, and how the captions look.
 *
 * A story in Hindi has one voice, so that is stated instead of offered: a menu
 * with a single entry is a choice that is not one. Both groups are native form
 * controls (a select, and radio buttons in a fieldset), so the keyboard and
 * screen readers work as they do everywhere else. Nothing here pretends to
 * preview a voice or a caption look: there is no sample to play or show.
 */
@Component({
  selector: 'app-story-video-options',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: [`
    :host { display: block; }
    .grid { display: grid; gap: var(--space-lg); }
    @media (min-width: 48rem) { .grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
    .lbl, legend { padding: 0; font-weight: 600; }
    .hint { margin: var(--space-3xs) 0 0; font-size: var(--type-sm); color: var(--color-ink-2); }
    select { margin-top: var(--space-xs); }
    .fixed { margin: var(--space-3xs) 0 0; }
    fieldset { margin: 0; padding: 0; border: 0; min-width: 0; }
    .opts { margin-top: var(--space-xs); display: flex; flex-direction: column; gap: var(--space-xs); }
    .opt { display: flex; align-items: flex-start; gap: var(--space-sm); min-height: 44px; padding: var(--space-xs) var(--space-sm); border: 1px solid var(--color-muted); border-radius: var(--radius-control); cursor: pointer; transition: border-color var(--dur-short) var(--ease-out); }
    .opt:hover { border-color: var(--color-ink-2); }
    .opt:has(input:checked) { border-color: var(--color-accent); box-shadow: inset 0 0 0 1px var(--color-accent); }
    .opt:has(input:focus-visible) { outline: 2px solid var(--color-focus); outline-offset: 2px; }
    .opt:has(input:disabled) { cursor: not-allowed; opacity: 0.6; }
    .opt input { margin: 3px 0 0; width: 1.125rem; height: 1.125rem; flex: none; accent-color: var(--color-accent); }
    .opt input:focus-visible { outline: none; }
    .name { display: block; font-weight: 600; }
    .blurb { display: block; font-size: var(--type-sm); color: var(--color-ink-2); }
    @media (prefers-reduced-motion: reduce) { .opt { transition: none; } }
  `],
  template: `
    <div class="grid">
      <div>
        @if (voices().length > 1) {
          <label for="video-voice" class="lbl">Voice</label>
          <p id="video-voice-hint" class="hint">
            Who reads the {{ language() === 'en' ? '' : languageName(language()) + ' ' }}narration.
          </p>
          <select
            id="video-voice"
            [value]="value().voice"
            [disabled]="disabled()"
            (change)="setVoice($event)"
            aria-describedby="video-voice-hint video-voice-note"
            class="ui-field"
          >
            @for (v of voices(); track v) {
              <option [value]="v" [selected]="v === value().voice">{{ v }}</option>
            }
          </select>
          <p id="video-voice-note" class="hint" data-testid="voice-note">{{ note() }}</p>
        } @else {
          <p class="lbl" style="margin: 0">Voice</p>
          <p class="fixed" data-testid="voice-fixed">{{ value().voice }}</p>
          <p class="hint" data-testid="voice-note">{{ languageName(language()) }} narration has one voice, so there is nothing to choose.</p>
        }
      </div>

      <fieldset [disabled]="disabled()">
        <legend>Captions</legend>
        <p id="video-caption-hint" class="hint">Burned into the video, in time with the narration.</p>
        <div class="opts" role="radiogroup" aria-describedby="video-caption-hint">
          @for (p of presets; track p.id) {
            <label class="opt">
              <input type="radio" name="video-caption" [value]="p.id" [checked]="p.id === value().caption" (change)="setCaption(p.id)" />
              <span style="min-width: 0">
                <span class="name">{{ p.label }}</span>
                <span class="blurb">{{ p.blurb }}</span>
              </span>
            </label>
          }
        </div>
      </fieldset>
    </div>
  `,
})
export class StoryVideoOptionsComponent {
  /** The plan's language code. It decides which voices can read the story. */
  readonly language = input.required<string>();
  /** The voices for that language, default first. */
  readonly voices = input.required<string[]>();
  readonly value = input.required<VideoOptions>();
  readonly disabled = input(false);
  readonly valueChange = output<VideoOptions>();

  protected readonly presets = CAPTION_PRESETS;
  protected readonly languageName = languageName;
  protected readonly note = computed(() => voiceNote(this.value().voice, this.language()));

  protected setVoice(event: Event): void {
    this.valueChange.emit({ ...this.value(), voice: (event.target as HTMLSelectElement).value });
  }

  protected setCaption(caption: CaptionPreset): void {
    this.valueChange.emit({ ...this.value(), caption });
  }
}
