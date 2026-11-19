import {
  NEW_JOB_CAPTION_STYLE,
  bottomInsetPercent,
  cleanCaptionStyle,
  sampleCaption,
  scriptFor,
  withSavedStyle,
} from './caption-style';

describe('caption style', () => {
  it('starts new jobs above the platform controls', () => {
    expect(NEW_JOB_CAPTION_STYLE.aboveControls).toBeTrue();
    expect(NEW_JOB_CAPTION_STYLE.font).toBe('');
  });

  describe('withSavedStyle', () => {
    it('reads a job saved before the option existed as off, so the preview matches its render', () => {
      const v = withSavedStyle({ preset: 'karaoke', fontSize: 30, color: '#112233', highlight: '#445566', position: 'top' });
      expect(v.aboveControls).toBeFalse();
      expect(v.font).toBe('');
      expect(v.preset).toBe('karaoke');
      expect(v.fontSize).toBe(30);
    });
    it('keeps what the job saved', () => {
      const v = withSavedStyle({ font: 'serif', aboveControls: true });
      expect(v.font).toBe('serif');
      expect(v.aboveControls).toBeTrue();
    });
  });

  describe('cleanCaptionStyle', () => {
    it('drops anything that is not a valid value', () => {
      const v = cleanCaptionStyle(
        { preset: 'neon', fontSize: 500, color: 'red', highlight: 7, position: 'left', font: 'comic', aboveControls: 'yes' },
        NEW_JOB_CAPTION_STYLE,
      );
      expect(v).toEqual({ ...NEW_JOB_CAPTION_STYLE, fontSize: 72 });
    });
    it('clamps the size to the range the renderer takes', () => {
      expect(cleanCaptionStyle({ fontSize: 1 }, NEW_JOB_CAPTION_STYLE).fontSize).toBe(8);
      expect(cleanCaptionStyle({ fontSize: 30.4 }, NEW_JOB_CAPTION_STYLE).fontSize).toBe(30);
    });
    it('survives null and non-objects', () => {
      expect(cleanCaptionStyle(null, NEW_JOB_CAPTION_STYLE)).toEqual(NEW_JOB_CAPTION_STYLE);
      expect(cleanCaptionStyle('x', NEW_JOB_CAPTION_STYLE)).toEqual(NEW_JOB_CAPTION_STYLE);
    });
  });

  describe('bottomInsetPercent', () => {
    it('matches the renderer margins when off', () => {
      expect(bottomInsetPercent('karaoke', false)).toBeCloseTo(90 / 19.2, 5);
      expect(bottomInsetPercent('bold', false)).toBeCloseTo(67 / 19.2, 5);
    });
    it('lifts karaoke and bold to about 15%, and never lowers minimal or clean', () => {
      expect(bottomInsetPercent('karaoke', true)).toBeCloseTo(15, 5);
      expect(bottomInsetPercent('bold', true)).toBeCloseTo(15, 5);
      expect(bottomInsetPercent('minimal', true)).toBe(bottomInsetPercent('minimal', false));
      expect(bottomInsetPercent('clean', true)).toBe(bottomInsetPercent('clean', false));
    });
  });

  describe('sample text', () => {
    it('follows the script of the language or the words', () => {
      expect(scriptFor('hi')).toBe('devanagari');
      expect(scriptFor('hi-IN')).toBe('devanagari');
      expect(scriptFor('en')).toBe('latin');
      expect(scriptFor(undefined)).toBe('latin');
      expect(scriptFor('auto', 'यह')).toBe('devanagari');
      expect(/[\u0900-\u097F]/.test(sampleCaption('devanagari'))).toBeTrue();
      expect(/[\u0900-\u097F]/.test(sampleCaption('latin'))).toBeFalse();
    });
    it('uses no em dash', () => {
      expect(sampleCaption('latin') + sampleCaption('devanagari')).not.toContain('\u2014');
    });
  });
});
