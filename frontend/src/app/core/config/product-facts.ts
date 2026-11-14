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
    /** Shortest source: one clip's minimum, since a clip must fit inside the source (#286). */
    minDurationSeconds: 30,
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
  story: {
    minStoryChars: 30,
    maxStoryChars: 5000,
    minScenes: 2,
    maxScenes: 10,
    maxTotalSeconds: 90,
    outputWidth: 1080,
    outputHeight: 1920,
    frameRate: 30,
    /** Narration voices by language, from backend/internal/storyjob/job.go. */
    voicesEn: ['Joanna', 'Matthew', 'Amy', 'Brian', 'Aditi'],
    voicesHi: ['Aditi'],
    /** Stock-media libraries the scenes draw from. */
    stockLibraries: ['Pexels', 'Pixabay'],
    /** How many videos one account may have rendering at once, and start per day. */
    maxActive: 2,
    perDay: 10,
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
  /** The browser-only infographic maker. product-facts.spec.ts compares each value with the tool's own code. */
  infographic: {
    templates: 20,
    categories: ['Educational', 'Step by step', 'Comparison', 'Timeline', 'Statistics', 'Process', 'Mind map'],
    sizes: ['1080×1920 portrait', '1080×1080 square', '1920×1080 landscape'],
    exports: ['PNG', 'JPG', 'PDF'],
    themes: 8,
    maxSections: 10,
    /** Shapes and icons to pick from, and how many fit on one image. */
    iconsAndShapes: 36,
    maxIcons: 20,
    languages: ['English', 'Spanish', 'French', 'German', 'Portuguese', 'Italian', 'Russian', 'Turkish', 'Indonesian', 'Vietnamese', 'Hindi', 'Marathi', 'Gujarati'],
  },
  pricing: 'Free during beta',
  /**
   * Per-account usage limits during the beta, as the API enforces them.
   * scripts/check-facts.mjs compares every number here with the Go source.
   */
  limits: {
    /** backend/internal/api/quota.go: daily counters, reset at midnight UTC. */
    quizProjectsPerDay: 40,
    /** Question files and logos share one counter (handleUpload, handleLogoUpload). */
    fileUploadsPerDay: 120,
    shortsUploadsPerDay: 30,
    shortsRetriesPerDay: 15,
    thumbnailSetsPerDay: 30,
    /** backend/internal/api/storyplan.go: any rolling hour. */
    storyPlansPerHour: 20,
    /** backend/internal/api/stockmedia.go. */
    stockSearchesPerHour: 120,
    /** Story videos use story.perDay (any rolling 24 hours) and story.maxActive. */
  },
  /** Promise on /pricing: how far ahead registered users hear about paid plans or lower free limits. */
  changeNoticeDays: 30,
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

/** "1080×1920 MP4 (H.264 video, AAC audio, 30 fps)" for the story video. */
export function storyOutputLabel(): string {
  const s = PRODUCT_FACTS.story;
  return `${s.outputWidth}×${s.outputHeight} MP4 (H.264 video, AAC audio, ${s.frameRate} fps)`;
}

/** "1 hour" for 60, "45 minutes" otherwise. */
export function durationLabel(minutes: number): string {
  if (minutes === 60) return '1 hour';
  return `${minutes} minutes`;
}

/** "30 seconds to 1 hour": the length a Shorts source may have. */
export function shortsLengthLabel(): string {
  const s = PRODUCT_FACTS.shorts;
  return `${s.minDurationSeconds} seconds to ${durationLabel(s.maxDurationMinutes)}`;
}
