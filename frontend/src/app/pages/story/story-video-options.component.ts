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
  template: `
    <div class="grid gap-8 md:grid-cols-2">
      <div>
        @if (voices().length > 1) {
          <label for="video-voice" class="block font-medium">Voice</label>
          <p id="video-voice-hint" class="mt-1 text-sm text-[hsl(var(--muted-foreground))]">
            Who reads the {{ language() === 'en' ? '' : languageName(language()) + ' ' }}narration.
          </p>
          <select
            id="video-voice"
            [value]="value().voice"
            [disabled]="disabled()"
            (change)="setVoice($event)"
            aria-describedby="video-voice-hint video-voice-note"
            class="mt-2 block min-h-11 w-full rounded-xl border border-[hsl(var(--border))] bg-[hsl(var(--background))] px-3 text-base focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-primary)] disabled:opacity-60"
          >
            @for (v of voices(); track v) {
              <option [value]="v" [selected]="v === value().voice">{{ v }}</option>
            }
          </select>
          <p id="video-voice-note" class="mt-2 text-sm text-[hsl(var(--muted-foreground))]" data-testid="voice-note">
            {{ note() }}
          </p>
        } @else {
          <p class="font-medium">Voice</p>
          <p class="mt-1 text-base" data-testid="voice-fixed">{{ value().voice }}</p>
          <p class="mt-1 text-sm text-[hsl(var(--muted-foreground))]" data-testid="voice-note">
            {{ languageName(language()) }} narration has one voice, so there is nothing to choose.
          </p>
        }
      </div>

      <fieldset [disabled]="disabled()" class="min-w-0">
        <legend class="font-medium">Captions</legend>
        <p id="video-caption-hint" class="mt-1 text-sm text-[hsl(var(--muted-foreground))]">
          Burned into the video, in time with the narration.
        </p>
        <div class="mt-2 space-y-1" role="radiogroup" aria-describedby="video-caption-hint">
          @for (p of presets; track p.id) {
            <label
              class="flex min-h-11 cursor-pointer items-start gap-3 rounded-xl border border-[hsl(var(--border))] px-3 py-2 has-[:checked]:border-[hsl(var(--foreground))] has-[:disabled]:cursor-not-allowed has-[:disabled]:opacity-60"
            >
              <input
                type="radio"
                name="video-caption"
                [value]="p.id"
                [checked]="p.id === value().caption"
                (change)="setCaption(p.id)"
                class="mt-1 size-4 shrink-0 accent-[hsl(var(--foreground))]"
              />
              <span class="min-w-0">
                <span class="block font-medium">{{ p.label }}</span>
                <span class="block text-sm text-[hsl(var(--muted-foreground))]">{{ p.blurb }}</span>
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
