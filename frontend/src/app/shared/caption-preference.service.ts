import { Injectable, inject } from '@angular/core';
import { FunctionalStorage } from '../core/consent/functional-storage.service';
import { CaptionStyleValue, NEW_JOB_CAPTION_STYLE, cleanCaptionStyle } from './caption-style';

export const CAPTION_PREFERENCE_KEY = 'indifferent.captionStyle.v1';

/**
 * Remembers the last caption style a person saved, as the starting point for
 * their next new job (#327). It writes only while Functional is allowed:
 * nothing is held in memory either, so turning the choice off leaves no trace.
 * A job that already has a saved style never reads this.
 */
@Injectable({ providedIn: 'root' })
export class CaptionPreference {
  private readonly storage = inject(FunctionalStorage);

  /** True while the browser may keep the choice. Reads a signal, so a template that uses it updates. */
  get remembering(): boolean {
    return this.storage.permitted;
  }

  /** The remembered style, or null when none is kept or Functional is off. */
  load(): CaptionStyleValue | null {
    if (!this.storage.permitted) return null;
    const raw = this.storage.getItem(CAPTION_PREFERENCE_KEY);
    if (!raw) return null;
    try {
      return cleanCaptionStyle(JSON.parse(raw), NEW_JOB_CAPTION_STYLE);
    } catch {
      return null;
    }
  }

  /** What a new job starts from: the remembered style, else the defaults. */
  startingStyle(): CaptionStyleValue {
    return this.load() ?? { ...NEW_JOB_CAPTION_STYLE };
  }

  /** Returns true when the style was kept. Without Functional nothing is written. */
  save(style: CaptionStyleValue): boolean {
    if (!this.storage.permitted) return false;
    return this.storage.setItem(CAPTION_PREFERENCE_KEY, JSON.stringify(style));
  }

  forget(): void {
    this.storage.removeItem(CAPTION_PREFERENCE_KEY);
  }
}
