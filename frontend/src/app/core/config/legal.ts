/** Date of the last review of the privacy policy and the terms against what the
 * product does. Change it together with the text, never on its own. */
export const LEGAL_LAST_UPDATED = '2026-10-31';

/** "October 31, 2026" */
export function legalUpdatedLabel(): string {
  return new Intl.DateTimeFormat('en-US', { year: 'numeric', month: 'long', day: 'numeric', timeZone: 'UTC' }).format(
    new Date(`${LEGAL_LAST_UPDATED}T00:00:00Z`),
  );
}
