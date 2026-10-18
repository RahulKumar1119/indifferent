export type Template = 'classic' | 'modern' | 'education' | 'dark' | 'minimal' | 'neon';

export interface TemplateOption {
  value: Template;
  label: string;
  /** Plain-words description of the palette; also the preview image's alt text. */
  look: string;
}

/**
 * Every template the renderer draws differently (backend TestNoTwoThemesRenderTheSame).
 * Preview images live in public/templates/<value>.webp and come from
 * scripts/generate-template-previews.sh. If a palette changes, change `look` too.
 */
export const TEMPLATE_OPTIONS: readonly TemplateOption[] = [
  { value: 'classic', label: 'Classic', look: 'white slides with a purple accent' },
  { value: 'modern', label: 'Modern', look: 'light slate slides with a blue accent' },
  { value: 'education', label: 'Education', look: 'cream paper with navy text and a red accent' },
  { value: 'dark', label: 'Dark', look: 'near-black slides with a violet accent' },
  { value: 'minimal', label: 'Minimal', look: 'off-white slides with black text and no accent color' },
  { value: 'neon', label: 'Neon', look: 'deep indigo-black slides with magenta and green highlights' },
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
  /** Linked Shorts job IDs (unified project). */
  shortsJobIds?: string[];
  /** Saved watermark preferences (unified project). */
  watermark?: WatermarkSettings;
  /** Custom branding burned into renders (unified project). */
  branding?: Branding;
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
