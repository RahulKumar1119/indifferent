/**
 * One caption style, shared by AI Shorts and story videos (#327). It mirrors
 * the backend's models.CaptionStyle. The two pages used to keep their own copy
 * of the type and of the four presets.
 */
export type CaptionPresetId = 'minimal' | 'bold' | 'karaoke' | 'clean';
export type CaptionPosition = 'top' | 'middle' | 'bottom';
/** '' keeps the renderer's default font. */
export type CaptionFont = '' | 'sans' | 'serif' | 'condensed';

export interface CaptionStyleValue {
  preset: CaptionPresetId;
  fontSize: number;
  color: string;
  highlight: string;
  position: CaptionPosition;
  font: CaptionFont;
  /** Lifts bottom captions clear of the platform's own buttons. */
  aboveControls: boolean;
}

export const CAPTION_PRESET_LIST: readonly { id: CaptionPresetId; label: string; blurb: string }[] = [
  { id: 'bold', label: 'Bold', blurb: 'Large white text on a dark box' },
  { id: 'minimal', label: 'Minimal', blurb: 'Small and subtle, no box' },
  { id: 'karaoke', label: 'Karaoke highlight', blurb: 'The spoken word lights up as it is said' },
  { id: 'clean', label: 'Clean subtitles', blurb: 'Classic outlined text' },
];

export const CAPTION_FONT_LIST: readonly { id: CaptionFont; label: string }[] = [
  { id: '', label: 'Default' },
  { id: 'sans', label: 'Sans' },
  { id: 'serif', label: 'Serif' },
  { id: 'condensed', label: 'Condensed' },
];

export const CAPTION_POSITIONS: readonly CaptionPosition[] = ['top', 'middle', 'bottom'];
export const CAPTION_SIZE_MIN = 8;
export const CAPTION_SIZE_MAX = 72;

/**
 * The style a new job starts from. `aboveControls` is on here: a job that was
 * saved before the option existed has none, and reads as off (see
 * {@link withSavedStyle}), because the server keeps those jobs where they were.
 */
export const NEW_JOB_CAPTION_STYLE: CaptionStyleValue = {
  preset: 'bold',
  fontSize: 18,
  color: '#FFFFFF',
  highlight: '#FFD60A',
  position: 'bottom',
  font: '',
  aboveControls: true,
};

const HEX = /^#[0-9a-fA-F]{6}$/;

/** Anything that is not a valid value falls back to `base`, so stored junk cannot reach a form. */
export function cleanCaptionStyle(raw: unknown, base: CaptionStyleValue): CaptionStyleValue {
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>;
  const preset = CAPTION_PRESET_LIST.find((p) => p.id === r['preset'])?.id ?? base.preset;
  const size = typeof r['fontSize'] === 'number' && Number.isFinite(r['fontSize']) ? Math.round(r['fontSize']) : base.fontSize;
  const font = CAPTION_FONT_LIST.find((f) => f.id === r['font'])?.id ?? base.font;
  const position = CAPTION_POSITIONS.find((p) => p === r['position']) ?? base.position;
  return {
    preset,
    fontSize: Math.min(CAPTION_SIZE_MAX, Math.max(CAPTION_SIZE_MIN, size)),
    color: typeof r['color'] === 'string' && HEX.test(r['color']) ? r['color'] : base.color,
    highlight: typeof r['highlight'] === 'string' && HEX.test(r['highlight']) ? r['highlight'] : base.highlight,
    position,
    font,
    aboveControls: typeof r['aboveControls'] === 'boolean' ? r['aboveControls'] : base.aboveControls,
  };
}

/**
 * A job's saved style always wins. A field the saved style does not have takes
 * the legacy value (above-controls off, default font), never the new-job value,
 * so opening an old job shows what it will really render with.
 */
export function withSavedStyle(saved: unknown): CaptionStyleValue {
  const legacy: CaptionStyleValue = { ...NEW_JOB_CAPTION_STYLE, aboveControls: false };
  return cleanCaptionStyle(saved, legacy);
}

/** What goes on the wire: every field, so the server stores exactly what was shown. */
export function captionPayload(style: CaptionStyleValue): CaptionStyleValue {
  return { ...style };
}

export type SampleScript = 'latin' | 'devanagari';

/** The caption text the preview shows when it has no real words yet. */
export function sampleCaption(script: SampleScript): string {
  return script === 'devanagari' ? 'यह आपके कैप्शन का नमूना है' : 'This is how your captions will look';
}

/** Hindi stories and Hindi transcripts draw in Devanagari. */
export function scriptFor(language: string | undefined, text?: string): SampleScript {
  if (text && /[\u0900-\u097F]/.test(text)) return 'devanagari';
  return language && language.toLowerCase().startsWith('hi') ? 'devanagari' : 'latin';
}

/**
 * The caption's distance from the bottom edge as a share of the frame height,
 * mirroring the renderer. Karaoke sits at 90 px of 1920, bold at 67 px, and
 * minimal and clean already sit high. Above-controls raises only the bottom
 * position, and never lowers a caption.
 */
export function bottomInsetPercent(preset: CaptionPresetId, aboveControls: boolean): number {
  const legacy = { karaoke: 90, bold: 67, minimal: 300, clean: 333 }[preset];
  const px = aboveControls ? Math.max(legacy, 288) : legacy;
  return (px / 1920) * 100;
}

/** The preview's font stack for a font choice. The real font is chosen on the server; this is close. */
export function previewFontFamily(font: CaptionFont): string {
  switch (font) {
    case 'serif':
      return '"DejaVu Serif", Georgia, "Noto Serif", serif';
    case 'condensed':
      return '"DejaVu Sans Condensed", "Arial Narrow", "Noto Sans", sans-serif';
    case 'sans':
      return '"DejaVu Sans", Verdana, "Noto Sans", sans-serif';
    default:
      return 'Arial, "Noto Sans Devanagari", "Noto Sans", sans-serif';
  }
}
