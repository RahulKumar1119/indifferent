import { FORMATS } from './infographic.model';
import { THEMES, themeById } from './themes';
import { contrastRatio } from './text-utils';

describe('themes', () => {
  it('has unique ids', () => {
    expect(new Set(THEMES.map((t) => t.id)).size).toBe(THEMES.length);
  });

  for (const t of THEMES) {
    describe(t.name, () => {
      it('has readable text on the page and on cards', () => {
        expect(contrastRatio(t.ink, t.bg)).toBeGreaterThanOrEqual(4.5);
        expect(contrastRatio(t.ink, t.surface)).toBeGreaterThanOrEqual(4.5);
        expect(contrastRatio(t.ink2, t.bg)).toBeGreaterThanOrEqual(4.5);
        expect(contrastRatio(t.ink2, t.surface)).toBeGreaterThanOrEqual(4.5);
      });

      it('has readable text on the accent colour', () => {
        expect(contrastRatio(t.accentInk, t.accent)).toBeGreaterThanOrEqual(4.5);
      });

      it('has an accent that stands out from the page', () => {
        expect(contrastRatio(t.accent, t.bg)).toBeGreaterThanOrEqual(3);
      });
    });
  }

  it('falls back to the first theme for an unknown id', () => {
    expect(themeById('nope')).toBe(THEMES[0]);
    expect(themeById('ocean').id).toBe('ocean');
  });

  it('defines the three export sizes', () => {
    const size = (id: 'portrait' | 'square' | 'landscape') => [FORMATS[id].width, FORMATS[id].height];
    expect(size('portrait')).toEqual([1080, 1920]);
    expect(size('square')).toEqual([1080, 1080]);
    expect(size('landscape')).toEqual([1920, 1080]);
  });
});
