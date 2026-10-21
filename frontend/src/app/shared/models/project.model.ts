export type Template = 'classic' | 'modern' | 'education' | 'dark' | 'minimal' | 'neon';

export interface TemplateOption {
  value: Template;
  label: string;
  /** Plain-words description of the palette and layout; also the preview alt text. */
  look: string;
}

/**
 * Every design the renderer draws differently (backend TestNoTwoThemesRenderTheSame).
 * A design is a palette, type scale and layout together (issue #104). Preview
 * images live in public/templates/<value>.webp and come from
 * scripts/generate-template-previews.sh. If a design changes, change `look` too.
 */
export const TEMPLATE_OPTIONS: readonly TemplateOption[] = [
  { value: 'classic', label: 'Classic', look: 'White slides, purple accent and rounded 2×2 answer cards' },
  { value: 'modern', label: 'Modern', look: 'Cool slate slides, bold type and blue-tinted cards' },
  { value: 'education', label: 'Education', look: 'Cream paper, navy ink and a terracotta accent' },
  { value: 'dark', label: 'Dark', look: 'Near-black slides, high-contrast text and green answers' },
  { value: 'minimal', label: 'Minimal', look: 'Extra-large black type, no cards and a thin rule' },
  { value: 'neon', label: 'Neon', look: 'Deep navy slides with vivid magenta and green highlights' },
];

export type Voice = 'Joanna' | 'Matthew' | 'Amy' | 'Brian' | 'Aditi';

export type ProjectStatus =
  | 'created'
  | 'parsing'
  | 'generating_slides'
  | 'narrating'
  | 'rendering'
  | 'completed'
  | 'failed';

export interface Project {
  id: string;
  name: string;
  template: Template;
  voice: Voice;
  status: ProjectStatus;
  createdAt: string;
  completedAt?: string;
  videoUrl?: string;
  thumbnailUrl?: string;
  error?: string;
  /** Parse warnings, e.g. a question with no marked answer. */
  warnings?: ProjectWarning[];
  /** Linked Shorts job IDs (unified project). */
  shortsJobIds?: string[];
  /** Saved watermark preferences (unified project). */
  watermark?: WatermarkSettings;
  /** Custom branding burned into renders. */
  branding?: Branding;
}

/** A non-fatal problem found while parsing the TXT file. */
export interface ProjectWarning {
  /** 1-based question number. */
  line: number;
  reason: string;
}

/** Project branding: uploaded logo overlay + channel handle. */
export interface Branding {
  logoKey?: string;
  channelName?: string;
}

/** Watermark preferences saved on a project (mirrors the watermark tool). */
export interface WatermarkSettings {
  text: string;
  fontSize?: number;
  opacity?: number;
  color?: string;
  rotation?: number;
  x?: number;
  y?: number;
}

export interface PipelineProgress {
  stage: ProjectStatus;
  percentage: number; // 0-100 during rendering
  slidesProcessed?: number;
  slidesTotal?: number;
}

export interface CreateProjectRequest {
  name: string;
  template: Template;
  voice: Voice;
  watermark?: WatermarkSettings;
  branding?: Branding;
  shorts?: {
    fileType: string;
    duration: number;
  };
}

/** POST /projects response: the project plus an optional linked upload. */
export interface CreateProjectResponse extends Project {
  shorts?: {
    jobId: string;
    uploadUrl: string;
    sourceKey: string;
  };
}
