import type { ArticleContentModule } from './article-types';

/** Lazy loaders for the data-driven articles. Each article is its own chunk,
 * so reading one never downloads the others. Keep this in step with
 * articles.json: the registry spec fails when a slug has no loader. */
export const ARTICLE_LOADERS: Record<string, () => Promise<ArticleContentModule>> = {
  'podcast-to-youtube-shorts': () => import('./content/podcast-to-youtube-shorts'),
  'video-effects-for-shorts': () => import('./content/video-effects-for-shorts'),
  'animated-captions-for-shorts': () => import('./content/animated-captions-for-shorts'),
  'how-ai-video-clipping-works': () => import('./content/how-ai-video-clipping-works'),
  'landscape-to-vertical-video': () => import('./content/landscape-to-vertical-video'),
  'add-watermark-to-image-in-browser': () => import('./content/add-watermark-to-image-in-browser'),
  'mp4-vs-webm-formats': () => import('./content/mp4-vs-webm-formats'),
};
