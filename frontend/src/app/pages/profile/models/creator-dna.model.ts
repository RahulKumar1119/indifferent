/**
 * Creator DNA — reusable AI preferences that will eventually feed video
 * generation ("Generate using my Creator DNA").
 */
export interface CreatorDNA {
  niche: string;
  audience: string;
  language: string;
  tone: string;
  visualStyle: string;
  voice: string;
  captionStyle: string;
  format: string;
  music: string;
}

export const DEFAULT_CREATOR_DNA: CreatorDNA = {
  niche: 'Technology',
  audience: '18–34',
  language: 'English',
  tone: 'Educational',
  visualStyle: 'Cinematic',
  voice: 'Confident',
  captionStyle: 'Bold',
  format: '9:16',
  music: 'Upbeat',
};

/** Full option lists — intentionally broad so the UI never needs curation. */
export const CREATOR_DNA_OPTIONS: Record<keyof CreatorDNA, string[]> = {
  niche: [
    'Technology',
    'Education',
    'Finance',
    'Fitness',
    'Food',
    'Travel',
    'Gaming',
    'Fashion',
    'Comedy',
    'Motivation',
    'Science',
    'History',
    'Music',
    'Sports',
  ],
  audience: ['13–17', '18–24', '18–34', '25–40', '35–54', '55+', 'Everyone'],
  language: ['English', 'Hindi', 'Hinglish', 'Spanish', 'French', 'German', 'Portuguese'],
  tone: ['Educational', 'Funny', 'Inspirational', 'Serious', 'Casual', 'Dramatic', 'Calm'],
  visualStyle: ['Cinematic', 'Minimal', 'Neon', 'Documentary', 'Anime', 'Retro', 'Bold'],
  voice: ['Confident', 'Warm', 'Energetic', 'Calm', 'Deep', 'Friendly'],
  captionStyle: ['Bold', 'Subtle', 'Neon', 'Minimal', 'Karaoke', 'None'],
  format: ['9:16', '16:9', '1:1'],
  music: ['Upbeat', 'Lo-fi', 'Epic', 'Chill', 'Electronic', 'None'],
};

/** Human labels for the DNA fields. */
export const CREATOR_DNA_LABELS: Record<keyof CreatorDNA, string> = {
  niche: 'Niche',
  audience: 'Audience',
  language: 'Language',
  tone: 'Tone',
  visualStyle: 'Visual Style',
  voice: 'Voice',
  captionStyle: 'Captions',
  format: 'Format',
  music: 'Music',
};
