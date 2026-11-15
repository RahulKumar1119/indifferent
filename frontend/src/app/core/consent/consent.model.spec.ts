import {
  CONSENT_KEY, CONSENT_VERSION, ConsentChoices, FUNCTIONAL_STORAGE_KEYS, addMonths, googleConsentSignals, newRecord, parseRecord,
} from './consent.model';

const NOW = Date.UTC(2026, 9, 29, 12);
const none: ConsentChoices = { analytics: false, advertising: false, functional: false };
const all: ConsentChoices = { analytics: true, advertising: true, functional: true };

describe('consent model', () => {
  it('keeps a record for six months', () => {
    const r = newRecord(all, NOW);
    expect(r.expiresAt).toBe('2027-04-29T12:00:00.000Z');
    expect(r.version).toBe(CONSENT_VERSION);
  });

  it('addMonths clamps to the last day of a shorter month', () => {
    expect(addMonths(new Date(Date.UTC(2026, 7, 31)), 6).toISOString().slice(0, 10)).toBe('2027-02-28');
    expect(addMonths(new Date(Date.UTC(2026, 10, 30)), 3).toISOString().slice(0, 10)).toBe('2027-02-28');
  });

  it('round-trips a valid record', () => {
    const r = newRecord({ ...none, analytics: true }, NOW);
    expect(parseRecord(JSON.stringify(r), NOW + 1000)).toEqual(r);
  });

  it('treats anything that is not a valid current record as no choice', () => {
    const good = newRecord(all, NOW);
    const bad: (string | null)[] = [
      null, '', '{not json', '[]', '"x"', 'null',
      JSON.stringify({ ...good, version: CONSENT_VERSION + 1 }),
      JSON.stringify({ ...good, expiresAt: 'soon' }),
      JSON.stringify({ ...good, savedAt: 5 }),
      JSON.stringify({ ...good, choices: { analytics: true } }),
      JSON.stringify({ ...good, choices: { analytics: 'yes', advertising: true, functional: true } }),
      JSON.stringify({ ...good, choices: null }),
      JSON.stringify({ ...good, savedAt: new Date(NOW + 10 * 86400000).toISOString() }),
    ];
    for (const raw of bad) expect(parseRecord(raw, NOW)).withContext(String(raw)).toBeNull();
  });

  it('expires exactly at the expiry time', () => {
    const r = newRecord(all, NOW);
    const at = Date.parse(r.expiresAt);
    expect(parseRecord(JSON.stringify(r), at - 1)).not.toBeNull();
    expect(parseRecord(JSON.stringify(r), at)).toBeNull();
  });

  it('maps every choice to the Consent Mode v2 signals', () => {
    expect(googleConsentSignals(none)).toEqual({
      analytics_storage: 'denied', ad_storage: 'denied', ad_user_data: 'denied', ad_personalization: 'denied',
      functionality_storage: 'denied', security_storage: 'granted',
    });
    const a = googleConsentSignals(all);
    expect(a['analytics_storage']).toBe('granted');
    expect(a['ad_storage']).toBe('granted');
    expect(a['ad_user_data']).toBe('granted');
    expect(a['functionality_storage']).toBe('granted');
    // Personalised ads stay off until a certified TCF consent platform is integrated.
    expect(a['ad_personalization']).toBe('denied');
  });

  it('lists the browser keys the Functional choice covers', () => {
    for (const k of ['app-theme', 'shorts-upload-speed', 'indifferent.creatorProfile.v1', 'indifferent.infographic.draft.v1']) {
      expect(FUNCTIONAL_STORAGE_KEYS).toContain(k);
    }
    expect(FUNCTIONAL_STORAGE_KEYS).not.toContain(CONSENT_KEY);
  });
});
