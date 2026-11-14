import { contrastRatio, detectDirection, detectLanguage, normalizeText, scriptsIn, slugify } from './text-utils';

describe('text-utils', () => {
  describe('normalizeText', () => {
    it('turns ligature characters into letters', () => {
      expect(normalizeText('\ufb01nd o\ufb03ce \ufb02ow')).toBe('find office flow');
    });

    it('composes Hindi so a letter is stored one way', () => {
      const decomposed = '\u0928\u093c'; // na + nukta
      expect(normalizeText(decomposed)).toBe('\u0929'); // NNA
    });

    it('keeps zero-width joiners that Indic shaping needs', () => {
      expect(normalizeText('क\u200dष')).toContain('\u200d');
    });

    it('removes control characters but keeps new lines when asked', () => {
      expect(normalizeText('a\u0000b\u0007c\r\nd', true)).toBe('abc\nd');
    });

    it('turns line breaks into spaces for single-line fields', () => {
      expect(normalizeText('a\n\nb\r\nc')).toBe('a b c');
    });

    it('turns tabs into spaces', () => {
      expect(normalizeText('a\tb')).toBe('a b');
    });
  });

  describe('scripts and language', () => {
    it('finds the scripts in mixed text and ignores digits and punctuation', () => {
      expect([...scriptsIn('Hello 123, नमस्ते! ક')].sort()).toEqual(['devanagari', 'gujarati', 'latin']);
    });

    it('detects Hindi, Gujarati, Russian and English', () => {
      expect(detectLanguage('कहानी से वीडियो')).toBe('hi');
      expect(detectLanguage('વાર્તા')).toBe('gu');
      expect(detectLanguage('Привет')).toBe('ru');
      expect(detectLanguage('Hello 42')).toBe('en');
      expect(detectLanguage('')).toBe('en');
    });

    it('calls mixed Hindi and English Hindi', () => {
      expect(detectLanguage('Story से video')).toBe('hi');
    });

    it('treats Japanese kana text with Han characters as Japanese', () => {
      expect(detectLanguage('日本語のテキスト')).toBe('ja');
      expect(detectLanguage('中文文本')).toBe('zh-Hans');
    });

    it('reads direction from the first letter that has one', () => {
      expect(detectDirection('مرحبا')).toBe('rtl');
      expect(detectDirection('123 مرحبا')).toBe('rtl');
      expect(detectDirection('Hello مرحبا')).toBe('ltr');
      expect(detectDirection('12345')).toBe('ltr');
      expect(detectDirection('שלום')).toBe('rtl');
    });
  });

  describe('slugify', () => {
    it('makes a plain file name', () => {
      expect(slugify('What is photosynthesis?')).toBe('what-is-photosynthesis');
    });

    it('keeps letters and marks of other scripts', () => {
      expect(slugify('कहानी से वीडियो')).toBe('कहानी-से-वीडियो');
    });

    it('falls back when nothing is left', () => {
      expect(slugify('???')).toBe('infographic');
      expect(slugify('')).toBe('infographic');
    });

    it('cuts long titles and leaves no trailing dash', () => {
      const out = slugify('word '.repeat(40));
      expect(out.length).toBeLessThanOrEqual(60);
      expect(out.endsWith('-')).toBeFalse();
    });
  });

  describe('contrastRatio', () => {
    it('is 21 for black on white and 1 for the same colour', () => {
      expect(contrastRatio('#000000', '#ffffff')).toBeCloseTo(21, 1);
      expect(contrastRatio('#336699', '#336699')).toBeCloseTo(1, 5);
    });

    it('does not depend on the order', () => {
      expect(contrastRatio('#123456', '#fedcba')).toBeCloseTo(contrastRatio('#fedcba', '#123456'), 8);
    });
  });
});
