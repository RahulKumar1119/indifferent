export type Template = 'classic' | 'modern' | 'education' | 'dark' | 'minimal' | 'neon';

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
