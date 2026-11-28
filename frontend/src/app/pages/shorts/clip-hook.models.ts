/** The hook line for one clip, and rendering one finished clip again (#329). */

export type HookStyle = 'box' | 'outline' | 'band';
export type HookPosition = 'top' | 'middle';

/** Matches backend models.ClipHook. */
export interface ClipHook {
  enabled: boolean;
  text: string;
  style: HookStyle;
  position: HookPosition;
  seconds: number;
  updatedAt?: string;
}

/** Matches backend models.ClipRenderView: the latest single-clip render. */
export interface ClipRenderView {
  state: 'rendering' | 'done' | 'failed';
  requestedAt?: string;
  renderedAt?: string;
  error?: string;
  /** Finished single-clip renders, so a new file can be told from the old one. */
  renders?: number;
}

/** Body of GET /shorts/{id}/clips/{clipId}/hook. */
export interface HookView {
  hook: ClipHook;
  saved: boolean;
  ranker?: string;
  suggestionsEnabled: boolean;
  renderEnabled: boolean;
  render?: ClipRenderView;
  suggestRemainingToday?: number;
  suggestDailyLimit: number;
  renderRemainingToday?: number;
  renderDailyLimit: number;
  maxChars: number;
}

export interface HookSuggestResponse {
  suggestions: string[];
  ranker?: string;
  dropped: number;
  remainingToday?: number;
  dailyLimit: number;
  replayed: boolean;
}

export interface ClipRenderResponse {
  render: ClipRenderView;
  replayed: boolean;
  remainingToday?: number;
  dailyLimit: number;
}

export const MAX_HOOK_CHARS = 80;
export const MIN_HOOK_SECONDS = 1;
export const MAX_HOOK_SECONDS = 5;
export const DEFAULT_HOOK_SECONDS = 2.5;
/** The fade in and the fade out, in seconds. */
export const HOOK_FADE_SECONDS = 0.2;

export const HOOK_STYLES: { id: HookStyle; label: string; blurb: string }[] = [
  { id: 'box', label: 'Dark box', blurb: 'Bold white on a dark box' },
  { id: 'outline', label: 'Outlined', blurb: 'White with a dark outline' },
  { id: 'band', label: 'Accent band', blurb: 'Dark on a yellow band' },
];

export const HOOK_POSITIONS: { id: HookPosition; label: string }[] = [
  { id: 'top', label: 'Top' },
  { id: 'middle', label: 'Middle' },
];

export function defaultHook(): ClipHook {
  return { enabled: false, text: '', style: 'box', position: 'top', seconds: DEFAULT_HOOK_SECONDS };
}

function isConsonant(cp: number): boolean {
  return (cp >= 0x0915 && cp <= 0x0939) || (cp >= 0x0958 && cp <= 0x095f) || (cp >= 0x0978 && cp <= 0x097f);
}

function isRegional(cp: number): boolean {
  return cp >= 0x1f1e6 && cp <= 0x1f1ff;
}

const MARK = /\p{M}/u;

/**
 * Counts the characters a reader sees, the same way the server does
 * (models.HookTextLength): a Hindi vowel sign, nasal mark or virama belongs to
 * its letter, and a conjunct is one character, so "क्षमा" is two.
 */
export function hookTextLength(s: string): number {
  let n = 0;
  let prev = -1;
  let afterZwj = false;
  let regionalOpen = false;
  let first = true;
  for (const ch of s) {
    const cp = ch.codePointAt(0) ?? 0;
    let joins = false;
    if (first) {
      first = false;
    } else if (MARK.test(ch)) {
      joins = true;
    } else if (cp === 0x200c || cp === 0x200d) {
      joins = true;
    } else if ((cp >= 0xfe00 && cp <= 0xfe0f) || (cp >= 0xe0100 && cp <= 0xe01ef) || (cp >= 0x1f3fb && cp <= 0x1f3ff)) {
      joins = true;
    } else if (afterZwj) {
      joins = true;
    } else if (prev === 0x094d && isConsonant(cp)) {
      joins = true;
    } else if (cp === 0x0a && prev === 0x0d) {
      joins = true;
    } else if (isRegional(cp) && regionalOpen) {
      joins = true;
    }
    regionalOpen = isRegional(cp) ? !joins : false;
    if (!joins) n++;
    afterZwj = cp === 0x200d;
    prev = cp;
  }
  return n;
}

/** What is wrong with the hook as typed, or '' when it can be saved. The server checks again. */
export function hookProblem(h: ClipHook): string {
  const text = h.text.trim();
  // eslint-disable-next-line no-control-regex
  if (/[\u0000-\u001f\u007f\u2028\u2029]/.test(h.text)) return 'The hook is one line. It is split into lines for you.';
  const n = hookTextLength(text);
  if (n > MAX_HOOK_CHARS) return `The hook is ${n} characters. The limit is ${MAX_HOOK_CHARS}.`;
  if (h.enabled && !text) return 'Write the hook, or switch it off.';
  if (!(h.seconds >= MIN_HOOK_SECONDS && h.seconds <= MAX_HOOK_SECONDS)) return `The hook shows for ${MIN_HOOK_SECONDS} to ${MAX_HOOK_SECONDS} seconds.`;
  return '';
}

export function sameHook(a: ClipHook, b: ClipHook): boolean {
  return a.enabled === b.enabled && a.text.trim() === b.text.trim() && a.style === b.style && a.position === b.position && a.seconds === b.seconds;
}

/**
 * How visible the hook is at time t (seconds into the clip), as the render draws
 * it: a 0.2 s fade in and out, each at most half of a very short showing. With
 * reduced motion there is no fade.
 */
export function hookOpacity(t: number, seconds: number, clipSeconds: number, reducedMotion: boolean): number {
  const end = clipSeconds > 0 ? Math.min(seconds, clipSeconds) : seconds;
  if (!(t >= 0) || t >= end) return 0;
  if (reducedMotion) return 1;
  const fade = Math.min(HOOK_FADE_SECONDS, end / 2);
  if (fade <= 0) return 1;
  return Math.max(0, Math.min(1, t / fade, (end - t) / fade));
}
