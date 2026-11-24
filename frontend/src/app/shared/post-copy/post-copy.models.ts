/**
 * Wire types and checks for the copy written for each platform (#330). They
 * mirror backend/internal/videocopy (platform.go, save.go) and
 * backend/internal/api/postcopy.go. The server is the authority: it counts and
 * refuses again on save. These checks exist so the page can show a live count
 * and keep Copy and Save off until a field is valid, in the same units.
 */

/** What a platform counts its limit in. "utf16" is shown to people as characters. */
export type CopyUnit = 'characters' | 'bytes' | 'utf16';

export interface PlatformSpec {
  id: string;
  label: string;
  hasTitle: boolean;
  titleMax?: number;
  titleUnit?: CopyUnit;
  /** "Description" or "Caption". */
  textLabel: string;
  /** The limit on the text with its hashtags, which is what is pasted. */
  textMax: number;
  textUnit: CopyUnit;
  aim: number;
  hashtagsMax: number;
  hashtagsMin: number;
}

export interface PlatformCopy {
  title?: string;
  text: string;
  hashtags: string[];
}

export type CopyMap = Record<string, PlatformCopy>;

/** A copy as stored (backend PostCopySaved). */
export interface PostCopySaved {
  copy: CopyMap;
  notes: string[];
  /** "generated" until a person saves an edit. */
  source: 'generated' | 'edited';
  generatedAt?: string;
  updatedAt: string;
}

/** GET of one clip's or one video's copy. */
export interface PostCopyView {
  enabled: boolean;
  platforms: PlatformSpec[];
  saved: PostCopySaved | null;
  remainingToday?: number;
  dailyLimit: number;
}

/** GET /shorts/{id}/post-copy: which clips of a job have copy. */
export interface PostCopyListView {
  enabled: boolean;
  platforms: PlatformSpec[];
  saved: Record<string, PostCopySaved>;
  remainingToday?: number;
  dailyLimit: number;
}

/** POST: the written copy. */
export interface PostCopyResult extends PostCopySaved {
  remainingToday?: number;
  dailyLimit: number;
  /** An answer to an earlier try with the same id; it used nothing. */
  replayed: boolean;
}

/** What a copy belongs to. */
export type PostCopyTarget = { kind: 'clip'; jobId: string; clipId: string } | { kind: 'story'; videoId: string };

/** The platform whose caption the quick-copy button uses. */
export const QUICK_COPY_PLATFORM = 'tiktok';

// --- counting ---------------------------------------------------------------

const utf8 = new TextEncoder();

/** The length of s in the unit a platform counts in. */
export function countUnits(s: string, unit: CopyUnit): number {
  switch (unit) {
    case 'bytes':
      return utf8.encode(s).length;
    case 'utf16':
      return s.length;
    default:
      return Array.from(s).length;
  }
}

/** The unit as a person reads it. UTF-16 units are called characters. */
export function unitWord(unit: CopyUnit): string {
  return unit === 'bytes' ? 'bytes' : 'characters';
}

/** 5000 as "5,000". */
export function withCommas(n: number): string {
  return n.toLocaleString('en-US');
}

/** The text a person pastes: the text, a blank line, then the hashtags on one line. */
export function fullText(c: PlatformCopy): string {
  const tags = c.hashtags.join(' ');
  if (!tags) return c.text;
  return c.text ? `${c.text}\n\n${tags}` : tags;
}

// --- hashtags ---------------------------------------------------------------

/** The most characters in one hashtag, the # included (backend MaxHashtagRunes). */
export const MAX_HASHTAG_CHARS = 31;

/**
 * One hashtag from whatever was typed, the way the server forms it: a #, then
 * letters, digits, underscores and the marks inside Indic words. Spaces and
 * punctuation inside the word are dropped. "" when nothing usable is left.
 */
export function normalizeHashtag(input: string): string {
  const s = input.trim().replace(/^[#\uFF03\s]+/, '');
  let w = '';
  for (const ch of s) {
    if (/[\p{L}\p{N}_]/u.test(ch)) w += ch;
    else if (/\p{M}/u.test(ch) && w) w += ch;
    else if ((ch === '\u200c' || ch === '\u200d') && w) w += ch;
  }
  w = w.replace(/[\u200c\u200d]+$/, '');
  if (!w || !/\p{L}/u.test(w) || Array.from(w).length + 1 > MAX_HASHTAG_CHARS) return '';
  return `#${w}`;
}

/** Splits a hashtag box into tags. `bad` are the pieces that are not usable. */
export function parseHashtags(text: string): { tags: string[]; bad: string[] } {
  const tags: string[] = [];
  const bad: string[] = [];
  for (const piece of text.split(/[\s,]+/).filter(Boolean)) {
    const tag = normalizeHashtag(piece);
    if (tag) tags.push(tag);
    else bad.push(piece);
  }
  return { tags, bad };
}

// --- checking ---------------------------------------------------------------

/** What is wrong with one platform's copy, in words; a missing key means that part is fine. */
export interface PlatformProblems {
  title?: string;
  text?: string;
  hashtags?: string;
}

export function hasProblems(p: PlatformProblems): boolean {
  return !!(p.title || p.text || p.hashtags);
}

/** The same rules as backend videocopy.ValidateSaved, for one platform. */
export function platformProblems(spec: PlatformSpec, c: PlatformCopy, bad: string[] = []): PlatformProblems {
  const out: PlatformProblems = {};
  if (spec.hasTitle) {
    const title = c.title ?? '';
    if (!title.trim()) out.title = `${spec.label} needs a title.`;
    else if (spec.titleMax && spec.titleUnit) {
      const n = countUnits(title, spec.titleUnit);
      if (n > spec.titleMax) out.title = `The title is ${withCommas(n)} ${unitWord(spec.titleUnit)}; the limit is ${withCommas(spec.titleMax)}.`;
    }
  }
  if (!c.text.trim()) {
    out.text = `${spec.label} needs its ${spec.textLabel.toLowerCase()}.`;
  } else {
    const n = countUnits(fullText(c), spec.textUnit);
    if (n > spec.textMax) {
      out.text = `The ${spec.textLabel.toLowerCase()} with its hashtags is ${withCommas(n)} ${unitWord(spec.textUnit)}; the limit is ${withCommas(spec.textMax)}.`;
    }
  }
  const seen = new Set<string>();
  for (const t of c.hashtags) {
    const k = t.toLowerCase();
    if (seen.has(k)) {
      out.hashtags = `${t} is there twice.`;
      break;
    }
    seen.add(k);
  }
  if (!out.hashtags && bad.length > 0) out.hashtags = `${bad[0]} is not a hashtag. Use letters, digits or underscores.`;
  if (!out.hashtags && c.hashtags.length > spec.hashtagsMax) out.hashtags = `${spec.label} allows at most ${spec.hashtagsMax} hashtags.`;
  return out;
}

/** A request id for one try: the server uses it to make a retry safe. */
export function newRequestId(): string {
  const c = globalThis.crypto;
  if (c?.randomUUID) return c.randomUUID();
  const bytes = new Uint8Array(16);
  if (c?.getRandomValues) c.getRandomValues(bytes);
  else for (let i = 0; i < bytes.length; i++) bytes[i] = Math.floor(Math.random() * 256);
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}

/** "hi" for text with Devanagari in it, so a screen reader and the font choose well. */
export function langOf(text: string): string | null {
  return /[\u0900-\u097F]/.test(text) ? 'hi' : null;
}

/** The text a clip's quick-copy button copies: the saved TikTok caption, else the plain hook. */
export function quickCopyText(saved: PostCopySaved | undefined, hookText: string | undefined, rank: number): string {
  const tiktok = saved?.copy?.[QUICK_COPY_PLATFORM];
  if (tiktok && tiktok.text.trim()) return fullText(tiktok);
  const hook = (hookText ?? '').trim();
  return hook || `Short #${rank}`;
}
