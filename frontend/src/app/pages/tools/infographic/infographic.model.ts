/**
 * The infographic document (#217): everything the maker saves, undoes and
 * exports. It is plain data, so it can be copied, compared, stored in the
 * browser and rebuilt from storage after checking it.
 */

export type FormatId = 'portrait' | 'square' | 'landscape';

export interface Format {
  id: FormatId;
  label: string;
  /** Where people use it, to help them choose. */
  hint: string;
  width: number;
  height: number;
}

export const FORMATS: Record<FormatId, Format> = {
  portrait: { id: 'portrait', label: 'Portrait', hint: 'Stories, Reels and Shorts', width: 1080, height: 1920 },
  square: { id: 'square', label: 'Square', hint: 'Feed posts', width: 1080, height: 1080 },
  landscape: { id: 'landscape', label: 'Landscape', hint: 'YouTube and slides', width: 1920, height: 1080 },
};
export const FORMAT_IDS = Object.keys(FORMATS) as FormatId[];

export type Direction = 'auto' | 'ltr' | 'rtl';
export const DIRECTIONS: Direction[] = ['auto', 'ltr', 'rtl'];

/** One block of content: a step, a point, a timeline event or one side of a comparison. */
export interface Section {
  id: string;
  /** A short caption above the heading, such as a year. Shown only by templates that use it. */
  label: string;
  heading: string;
  body: string;
}

/** The colours an icon or shape can take. Each is a colour of the chosen theme, so it always suits the page. */
export type DecorColor = 'accent' | 'ink' | 'ink2';
export const DECOR_COLORS: DecorColor[] = ['accent', 'ink', 'ink2'];

/** An icon or a shape placed on the artboard. Its place and size are percentages of the artboard's width and height, so they hold at every size. */
export interface Decoration {
  id: string;
  /** Which icon or shape, by id (decor-library.ts). */
  glyph: string;
  /** The centre, in percent of the artboard's width (0 is the left edge). */
  x: number;
  /** The centre, in percent of the artboard's height (0 is the top edge). */
  y: number;
  /** The width, in percent of the artboard's width. */
  size: number;
  color: DecorColor;
}

export interface InfographicDoc {
  version: 1;
  templateId: string;
  format: FormatId;
  themeId: string;
  /** "auto" picks the language from the text; otherwise an ISO 639-1 code. */
  language: string;
  direction: Direction;
  title: string;
  subtitle: string;
  sections: Section[];
  footer: string;
  /** The optional "Made with indifferent.fun" line on the image. Off unless asked for. */
  mark: boolean;
  /** Icons and shapes, the first at the back. */
  items: Decoration[];
}

/** The longest each text may be. The page and the stored draft both enforce them. */
export const LIMITS = {
  title: 120,
  subtitle: 240,
  label: 24,
  heading: 80,
  body: 400,
  footer: 100,
  /** The most sections any template allows. */
  sections: 10,
  /** The most icons and shapes on one artboard. */
  items: 20,
  /** The smallest and the largest width of an icon or shape, in percent of the artboard's width. */
  itemMin: 4,
  itemMax: 60,
} as const;

export const MARK_TEXT = 'Made with indifferent.fun';
export const DEFAULT_THEME_ID = 'paper';
export const DEFAULT_TEMPLATE_ID = 'key-points';
export const DEFAULT_FORMAT: FormatId = 'portrait';
