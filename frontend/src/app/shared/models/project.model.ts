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
}
