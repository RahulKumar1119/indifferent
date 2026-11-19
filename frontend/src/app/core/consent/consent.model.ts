/**
 * Cookie and storage consent (#321): the categories, the stored record, and
 * the rules that decide whether a record still counts. Plain functions and
 * constants, no Angular, so they are easy to test and the inline theme script
 * in index.html can mirror them (a spec keeps the two in step).
 */

/** Where the choice is kept. localStorage, never a cookie, and nothing in it identifies anyone. */
export const CONSENT_KEY = 'indifferent.consent.v1';

/**
 * Raise this when the privacy policy or the list of optional services changes
 * in a way that needs a fresh choice. A record with another version is ignored,
 * so every visitor is asked again.
 */
export const CONSENT_VERSION = 1;

/** A saved choice is asked for again after this many months. */
export const CONSENT_MONTHS = 6;

export type OptionalCategory = 'analytics' | 'advertising' | 'functional';
export type ConsentCategory = 'necessary' | OptionalCategory;

export const OPTIONAL_CATEGORIES: readonly OptionalCategory[] = ['analytics', 'advertising', 'functional'];

export interface ConsentChoices {
  analytics: boolean;
  advertising: boolean;
  functional: boolean;
}

export interface ConsentRecord {
  version: number;
  /** ISO time the choice was saved. */
  savedAt: string;
  /** ISO time after which the choice is asked for again. */
  expiresAt: string;
  choices: ConsentChoices;
}

/** Nothing optional. Also what a visitor who has not chosen yet is treated as. */
export const NO_OPTIONAL: Readonly<ConsentChoices> = Object.freeze({ analytics: false, advertising: false, functional: false });
export const ALL_OPTIONAL: Readonly<ConsentChoices> = Object.freeze({ analytics: true, advertising: true, functional: true });

export interface CategoryInfo {
  id: ConsentCategory;
  title: string;
  /** What it is used for, in plain words, and only what the code really does. */
  description: string;
}

export const CATEGORIES: readonly CategoryInfo[] = [
  {
    id: 'necessary',
    title: 'Strictly necessary',
    description: 'Keeps you signed in, protects the sign-in flow, finishes an upload you started and remembers this choice. Always on.',
  },
  {
    id: 'analytics',
    title: 'Analytics',
    description: 'Google Analytics and Amazon CloudWatch RUM count visits and measure speed and errors. Nothing is loaded until you allow it.',
  },
  {
    id: 'advertising',
    title: 'Advertising',
    description:
      'Google Ads measures whether an ad led to a sign-in, and the Google AdSense script is loaded. Nothing is loaded until you allow it. We do not use personalised ads yet.',
  },
  {
    id: 'functional',
    title: 'Functional',
    description:
      'Remembers your theme, your profile details, your upload speed estimate and your infographic draft in this browser. Turning it off deletes them.',
  },
];

/**
 * Whether Google may personalise ads for people who allow Advertising. Off on
 * purpose: for visitors in the EEA, the UK and Switzerland Google requires a
 * certified consent platform that supports IAB TCF before personalised ads are
 * served, and this banner is not one. Turn it on only after one is integrated.
 */
export const PERSONALIZED_ADS_ENABLED = false;

/**
 * The browser storage the Functional category covers. Turning Functional off
 * removes every one of these, and nothing writes to them without it. Sign-in
 * keys, an upload in progress and the consent record itself are not here: they
 * are strictly necessary.
 */
export const FUNCTIONAL_STORAGE_KEYS: readonly string[] = [
  'app-theme',
  'shorts-upload-speed',
  'indifferent.creatorProfile.v1',
  'indifferent.creatorDna.v1',
  'indifferent.creatorSocial.v1',
  'indifferent.creatorFavorites.v1',
  'indifferent.creatorPublished.v1',
  'indifferent.creatorActivity.v1',
  'indifferent.creatorAvatarSynced.v1',
  'indifferent.infographic.draft.v1',
  'indifferent.captionStyle.v1',
];

/**
 * Cookies that Google's tags and CloudWatch RUM write on this site's own
 * domain, by name pattern. They are the only cookies this site can see and
 * remove; cookies on Google's or Amazon's own domains are out of reach.
 * The Google names are the ones Google documents for Analytics and Ads.
 */
export const ANALYTICS_COOKIE_PATTERNS: readonly RegExp[] = [/^_ga$/, /^_ga_.+$/, /^_gid$/, /^_gat(_.+)?$/, /^cwr_.+$/];
export const ADVERTISING_COOKIE_PATTERNS: readonly RegExp[] = [/^_gcl_.+$/, /^_gac_.+$/];

export function addMonths(from: Date, months: number): Date {
  const d = new Date(from.getTime());
  const day = d.getUTCDate();
  d.setUTCDate(1);
  d.setUTCMonth(d.getUTCMonth() + months);
  // Keep the day of the month, or the last day of a shorter month.
  const last = new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + 1, 0)).getUTCDate();
  d.setUTCDate(Math.min(day, last));
  return d;
}

export function newRecord(choices: ConsentChoices, now: number): ConsentRecord {
  const saved = new Date(now);
  return {
    version: CONSENT_VERSION,
    savedAt: saved.toISOString(),
    expiresAt: addMonths(saved, CONSENT_MONTHS).toISOString(),
    choices: { analytics: choices.analytics === true, advertising: choices.advertising === true, functional: choices.functional === true },
  };
}

const DAY_MS = 24 * 60 * 60 * 1000;

/**
 * Reads a stored record. Anything that is not exactly a valid, current record
 * (damaged JSON, another version, a past expiry, a save date in the future,
 * a choice that is not true or false) gives null, which asks the visitor again.
 */
export function parseRecord(raw: string | null, now: number): ConsentRecord | null {
  if (!raw) return null;
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    return null;
  }
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return null;
  const r = value as Record<string, unknown>;
  if (r['version'] !== CONSENT_VERSION) return null;
  const saved = typeof r['savedAt'] === 'string' ? Date.parse(r['savedAt']) : NaN;
  const expires = typeof r['expiresAt'] === 'string' ? Date.parse(r['expiresAt']) : NaN;
  if (!Number.isFinite(saved) || !Number.isFinite(expires)) return null;
  if (expires <= now) return null;
  // A save date well in the future means the clock or the record is wrong.
  if (saved > now + DAY_MS) return null;
  const c = r['choices'];
  if (typeof c !== 'object' || c === null || Array.isArray(c)) return null;
  const choices = c as Record<string, unknown>;
  for (const key of OPTIONAL_CATEGORIES) if (typeof choices[key] !== 'boolean') return null;
  return {
    version: CONSENT_VERSION,
    savedAt: r['savedAt'] as string,
    expiresAt: r['expiresAt'] as string,
    choices: { analytics: choices['analytics'] as boolean, advertising: choices['advertising'] as boolean, functional: choices['functional'] as boolean },
  };
}

export type GoogleConsentValue = 'granted' | 'denied';

/** The Google Consent Mode v2 signals for a set of choices. */
export function googleConsentSignals(choices: ConsentChoices): Record<string, GoogleConsentValue> {
  const v = (on: boolean): GoogleConsentValue => (on ? 'granted' : 'denied');
  return {
    analytics_storage: v(choices.analytics),
    ad_storage: v(choices.advertising),
    ad_user_data: v(choices.advertising),
    ad_personalization: v(choices.advertising && PERSONALIZED_ADS_ENABLED),
    functionality_storage: v(choices.functional),
    // The sign-in and security cookies are strictly necessary, so this is always on.
    security_storage: 'granted',
  };
}
