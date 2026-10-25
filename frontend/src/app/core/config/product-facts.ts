/**
 * Single source of truth for the product specifications the public pages quote.
 *
 * Each value mirrors a limit that the backend actually enforces
 * (backend/internal/shorts/constants.go, backend/internal/parser/validator.go,
 * backend/internal/narrator/polly.go) or a capability of the browser-only
 * watermark tool. Pages that state a spec should read it from here, and
 * product-facts.spec.ts fails when a page quotes something different.
 *
 * No imports on purpose: scripts may read this file directly.
 */
export const PRODUCT_FACTS = {
  shorts: {
    maxDurationMinutes: 60,
    maxUploadGb: 2,
    inputFormats: ['MP4', 'MOV', 'MP3', 'WAV'],
    maxClips: 3,
    clipMinSeconds: 30,
    clipMaxSeconds: 90,
    outputWidth: 1080,
    outputHeight: 1920,
    frameRate: 30,
    languages: ['Auto-detect', 'English (US)', 'English (India)', 'Hindi'],
    captionPresets: ['Minimal', 'Bold', 'Karaoke', 'Clean'],
    /** Words per caption cue; keeps lines short enough to read at a glance. */
    captionWordsPerCue: 4,
    /** The ranking model sees the transcript in blocks of this many seconds. */
    rankingBlockSeconds: 10,
    /** Frames sampled per clip for face detection. */
    faceSampleFrames: 8,
  },
  quiz: {
    inputFormat: 'TXT',
    maxFileMb: 5,
    voices: ['Joanna', 'Matthew', 'Amy', 'Brian', 'Aditi'],
    designs: ['Classic', 'Modern', 'Education', 'Dark', 'Minimal', 'Neon'],
    outputLabel: '1080p MP4',
  },
  watermark: {
    imageFormats: ['JPG', 'PNG', 'WebP'],
    videoFormats: ['MP4', 'WebM', 'MOV'],
    imageExports: ['PNG', 'JPG'],
    videoExport: 'MP4 (H.264 video, AAC audio)',
    fontChoices: 10,
    /** The tool warns about video files larger than this. */
    videoWarnMb: 500,
  },
  pricing: 'Free during beta',
} as const;

/** "MP4, MOV, MP3 or WAV" */
export function joinOr(items: readonly string[]): string {
  if (items.length <= 1) return items.join('');
  return `${items.slice(0, -1).join(', ')} or ${items[items.length - 1]}`;
}

/** "MP4, MOV, MP3 and WAV" */
export function joinAnd(items: readonly string[]): string {
  if (items.length <= 1) return items.join('');
  return `${items.slice(0, -1).join(', ')} and ${items[items.length - 1]}`;
}

/** "1080×1920 MP4 (H.264 video, AAC audio, 30 fps)" */
export function shortsOutputLabel(): string {
  const s = PRODUCT_FACTS.shorts;
  return `${s.outputWidth}×${s.outputHeight} MP4 (H.264 video, AAC audio, ${s.frameRate} fps)`;
}

/** "1 hour" for 60, "45 minutes" otherwise. */
export function durationLabel(minutes: number): string {
  if (minutes === 60) return '1 hour';
  return `${minutes} minutes`;
}
