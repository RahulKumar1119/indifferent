import { Script } from './text-utils';

/**
 * The languages the infographic maker claims (#217). A language is listed only
 * after its batch passed the checks in the plan: font coverage, no clipping in
 * every template at every size, preview and export matching, and a look by
 * someone who reads it. Batches 2 to 4 (more Indian scripts and Thai, Arabic,
 * Chinese, Japanese and Korean) are added here when they are verified, and
 * nowhere else, so the page and the docs cannot claim more than was released.
 */
export interface LanguageOption {
  code: string;
  name: string;
  batch: number;
  /** True once someone who reads the language has looked at the real output. Until then the page says it is checked by script only. */
  reviewed?: boolean;
}

export const RELEASED_BATCHES = [1];

const ALL: LanguageOption[] = [
  { code: 'en', name: 'English', batch: 1, reviewed: true },
  { code: 'es', name: 'Spanish', batch: 1 },
  { code: 'fr', name: 'French', batch: 1 },
  { code: 'de', name: 'German', batch: 1 },
  { code: 'pt', name: 'Portuguese', batch: 1 },
  { code: 'it', name: 'Italian', batch: 1 },
  { code: 'ru', name: 'Russian', batch: 1 },
  { code: 'tr', name: 'Turkish', batch: 1 },
  { code: 'id', name: 'Indonesian', batch: 1 },
  { code: 'vi', name: 'Vietnamese', batch: 1 },
  { code: 'hi', name: 'Hindi', batch: 1 },
  { code: 'mr', name: 'Marathi', batch: 1 },
  { code: 'gu', name: 'Gujarati', batch: 1 },
];

export const LANGUAGES: LanguageOption[] = ALL.filter((l) => RELEASED_BATCHES.includes(l.batch));

/** Writing systems whose fonts ship with the released batches. Greek comes with Noto Sans. */
export const RELEASED_SCRIPTS: ReadonlySet<Script> = new Set<Script>(['latin', 'cyrillic', 'greek', 'devanagari', 'gujarati']);

/** The language that is not yet reviewed by a reader, or null when it is reviewed or unknown. */
export function unreviewedLanguage(code: string): LanguageOption | null {
  const l = LANGUAGES.find((x) => x.code === code);
  return l && !l.reviewed ? l : null;
}

export function isReleasedLanguage(code: string): boolean {
  return LANGUAGES.some((l) => l.code === code);
}

/** Writing systems in `scripts` that have no released font yet. */
export function unreleasedScripts(scripts: Iterable<Script>): Script[] {
  return [...scripts].filter((s) => !RELEASED_SCRIPTS.has(s));
}
