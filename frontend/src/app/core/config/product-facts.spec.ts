import { PRODUCT_FACTS, durationLabel, joinAnd, joinOr, shortsOutputLabel } from './product-facts';
import { LEGAL_LAST_UPDATED, legalUpdatedLabel } from './legal';

describe('product facts', () => {
  it('formats lists the way the pages read them', () => {
    expect(joinOr(['MP4', 'MOV', 'MP3', 'WAV'])).toBe('MP4, MOV, MP3 or WAV');
    expect(joinAnd(['MP4', 'MOV', 'MP3', 'WAV'])).toBe('MP4, MOV, MP3 and WAV');
    expect(joinOr(['PNG'])).toBe('PNG');
    expect(joinOr([])).toBe('');
    expect(joinAnd(['A', 'B'])).toBe('A and B');
  });

  it('labels durations and the Shorts output', () => {
    expect(durationLabel(60)).toBe('1 hour');
    expect(durationLabel(45)).toBe('45 minutes');
    expect(shortsOutputLabel()).toBe('1080×1920 MP4 (H.264 video, AAC audio, 30 fps)');
  });

  it('matches the limits the backend enforces', () => {
    // backend/internal/shorts/constants.go
    expect(PRODUCT_FACTS.shorts.maxDurationMinutes * 60).toBe(3600);
    expect(PRODUCT_FACTS.shorts.maxUploadGb).toBe(2);
    expect(PRODUCT_FACTS.shorts.maxClips).toBe(3);
    expect(PRODUCT_FACTS.shorts.clipMinSeconds).toBe(30);
    expect(PRODUCT_FACTS.shorts.clipMaxSeconds).toBe(90);
    // backend/internal/parser/validator.go
    expect(PRODUCT_FACTS.quiz.maxFileMb).toBe(5);
    // backend/internal/narrator/polly.go
    expect([...PRODUCT_FACTS.quiz.voices]).toEqual(['Joanna', 'Matthew', 'Amy', 'Brian', 'Aditi']);
    // backend/internal/shorts/language.go
    expect([...PRODUCT_FACTS.shorts.languages]).toEqual(['Auto-detect', 'English (US)', 'English (India)', 'Hindi']);
  });

  it('keeps the clip window inside a Shorts-sized clip', () => {
    const s = PRODUCT_FACTS.shorts;
    expect(s.clipMinSeconds).toBeLessThan(s.clipMaxSeconds);
    expect(s.outputHeight / s.outputWidth).toBeCloseTo(16 / 9, 3);
  });
});

describe('legal date', () => {
  it('is a valid date that is not in the future', () => {
    expect(LEGAL_LAST_UPDATED).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    const date = new Date(`${LEGAL_LAST_UPDATED}T00:00:00Z`);
    expect(Number.isNaN(date.getTime())).toBeFalse();
    expect(date.getTime()).toBeLessThanOrEqual(Date.now());
  });

  it('reads as a long date in English', () => {
    expect(legalUpdatedLabel()).toMatch(/^[A-Z][a-z]+ \d{1,2}, \d{4}$/);
  });
});
