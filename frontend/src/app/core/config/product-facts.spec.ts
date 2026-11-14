import { PRODUCT_FACTS, durationLabel, joinAnd, joinOr, shortsLengthLabel, shortsOutputLabel, storyOutputLabel } from './product-facts';
import { LEGAL_LAST_UPDATED, legalUpdatedLabel } from './legal';
import { GLYPHS } from '../../pages/tools/infographic/decor-library';
import { FORMATS, FORMAT_IDS, LIMITS } from '../../pages/tools/infographic/infographic.model';
import { LANGUAGES } from '../../pages/tools/infographic/languages';
import { TEMPLATES } from '../../pages/tools/infographic/templates';
import { THEMES } from '../../pages/tools/infographic/themes';

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
    expect(shortsLengthLabel()).toBe('30 seconds to 1 hour');
    expect(shortsOutputLabel()).toBe('1080×1920 MP4 (H.264 video, AAC audio, 30 fps)');
  });

  it('matches the limits the backend enforces', () => {
    // backend/internal/shorts/constants.go
    expect(PRODUCT_FACTS.shorts.maxDurationMinutes * 60).toBe(3600);
    expect(PRODUCT_FACTS.shorts.maxUploadGb).toBe(2);
    expect(PRODUCT_FACTS.shorts.maxClips).toBe(3);
    expect(PRODUCT_FACTS.shorts.clipMinSeconds).toBe(30);
    expect(PRODUCT_FACTS.shorts.clipMaxSeconds).toBe(90);
    // MinSourceDuration = MinClipDuration (#286)
    expect(PRODUCT_FACTS.shorts.minDurationSeconds).toBe(PRODUCT_FACTS.shorts.clipMinSeconds);
    // backend/internal/parser/validator.go
    expect(PRODUCT_FACTS.quiz.maxFileMb).toBe(5);
    // backend/internal/narrator/polly.go
    expect([...PRODUCT_FACTS.quiz.voices]).toEqual(['Joanna', 'Matthew', 'Amy', 'Brian', 'Aditi']);
    // backend/internal/shorts/language.go
    expect([...PRODUCT_FACTS.shorts.languages]).toEqual(['Auto-detect', 'English (US)', 'English (India)', 'Hindi']);
    // backend/internal/storyplan/limits.go
    expect(PRODUCT_FACTS.story.minStoryChars).toBe(30);
    expect(PRODUCT_FACTS.story.maxStoryChars).toBe(5000);
    expect(PRODUCT_FACTS.story.minScenes).toBe(2);
    expect(PRODUCT_FACTS.story.maxScenes).toBe(10);
    expect(PRODUCT_FACTS.story.maxTotalSeconds).toBe(90);
    // backend/internal/storyjob/job.go (voicesByLanguage)
    expect([...PRODUCT_FACTS.story.voicesEn]).toEqual(['Joanna', 'Matthew', 'Amy', 'Brian', 'Aditi']);
    expect([...PRODUCT_FACTS.story.voicesHi]).toEqual(['Aditi']);
    // backend/internal/api/storyvideo.go (defaults)
    expect(PRODUCT_FACTS.story.maxActive).toBe(2);
    expect(PRODUCT_FACTS.story.perDay).toBe(10);
  });

  it('states the usage limits the API enforces (#255)', () => {
    // Also checked against the Go source by scripts/check-facts.mjs.
    // backend/internal/api/quota.go
    expect(PRODUCT_FACTS.limits.quizProjectsPerDay).toBe(40);
    expect(PRODUCT_FACTS.limits.fileUploadsPerDay).toBe(120);
    expect(PRODUCT_FACTS.limits.shortsUploadsPerDay).toBe(30);
    expect(PRODUCT_FACTS.limits.shortsRetriesPerDay).toBe(15);
    expect(PRODUCT_FACTS.limits.thumbnailSetsPerDay).toBe(30);
    // backend/internal/api/storyplan.go, stockmedia.go
    expect(PRODUCT_FACTS.limits.storyPlansPerHour).toBe(20);
    expect(PRODUCT_FACTS.limits.stockSearchesPerHour).toBe(120);
    // The notice promised on /pricing before paid plans or lower free limits.
    expect(PRODUCT_FACTS.changeNoticeDays).toBe(30);
  });

  it('labels the story output the same way as Shorts', () => {
    // Both are 1080x1920 vertical MP4s at 30 fps.
    expect(storyOutputLabel()).toBe('1080×1920 MP4 (H.264 video, AAC audio, 30 fps)');
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

  it('is not older than the Story videos text it was last changed for (2026-11-04)', () => {
    // Change the date together with the text. This stops the text changing and the date staying behind.
    expect(LEGAL_LAST_UPDATED >= '2026-11-04').toBeTrue();
  });

  it('is not older than the infographic maker text added to the privacy policy and the terms (2026-11-14)', () => {
    expect(LEGAL_LAST_UPDATED >= '2026-11-14').toBeTrue();
  });

  it('reads as a long date in English', () => {
    expect(legalUpdatedLabel()).toMatch(/^[A-Z][a-z]+ \d{1,2}, \d{4}$/);
  });
});

describe('infographic facts', () => {
  // The facts are `as const`; compare them as plain numbers and strings.
  const I = PRODUCT_FACTS.infographic as unknown as {
    templates: number; categories: string[]; sizes: string[]; exports: string[]; themes: number;
    maxSections: number; iconsAndShapes: number; maxIcons: number; languages: string[];
  };

  it('match what the tool really offers, so the pages cannot promise more', () => {
    expect(I.templates).toBe(TEMPLATES.length);
    expect(I.categories).toEqual(Array.from(new Set(TEMPLATES.map((t) => t.category))));
    expect(I.sizes).toEqual(FORMAT_IDS.map((id) => `${FORMATS[id].width}×${FORMATS[id].height} ${id}`));
    expect(I.themes).toBe(THEMES.length);
    expect(I.maxSections).toBe(LIMITS.sections);
    expect(I.maxSections).toBe(Math.max(...TEMPLATES.map((t) => t.maxSections)));
    expect(I.iconsAndShapes).toBe(GLYPHS.length);
    expect(I.maxIcons).toBe(LIMITS.items);
    expect(I.languages).toEqual(LANGUAGES.map((l) => l.name));
  });

  it('lists the downloads the page offers', () => {
    expect(I.exports).toEqual(['PNG', 'JPG', 'PDF']);
  });
});
