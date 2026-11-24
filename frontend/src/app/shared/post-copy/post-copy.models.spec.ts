import {
  PlatformSpec,
  PostCopySaved,
  countUnits,
  fullText,
  hasProblems,
  langOf,
  newRequestId,
  normalizeHashtag,
  parseHashtags,
  platformProblems,
  quickCopyText,
  unitWord,
  withCommas,
} from './post-copy.models';

const YT: PlatformSpec = { id: 'youtube', label: 'YouTube', hasTitle: true, titleMax: 100, titleUnit: 'characters', textLabel: 'Description', textMax: 5000, textUnit: 'bytes', aim: 700, hashtagsMax: 5, hashtagsMin: 3 };
const TT: PlatformSpec = { id: 'tiktok', label: 'TikTok', hasTitle: false, textLabel: 'Caption', textMax: 2200, textUnit: 'utf16', aim: 220, hashtagsMax: 5, hashtagsMin: 3 };

describe('post copy models', () => {
  describe('counting', () => {
    it('counts Hindi the way each platform does', () => {
      const hi = 'दीपक';
      expect(countUnits(hi, 'characters')).toBe(4);
      expect(countUnits(hi, 'utf16')).toBe(4);
      expect(countUnits(hi, 'bytes')).toBe(12);
    });
    it('counts an emoji as one character, two UTF-16 units and four bytes', () => {
      expect(countUnits('😀', 'characters')).toBe(1);
      expect(countUnits('😀', 'utf16')).toBe(2);
      expect(countUnits('😀', 'bytes')).toBe(4);
    });
    it('writes numbers and units for people', () => {
      expect(withCommas(5000)).toBe('5,000');
      expect(unitWord('bytes')).toBe('bytes');
      expect(unitWord('utf16')).toBe('characters');
      expect(unitWord('characters')).toBe('characters');
    });
  });

  describe('fullText', () => {
    it('is the text, a blank line, then the hashtags on one line', () => {
      expect(fullText({ text: 'Hello', hashtags: ['#a', '#b'] })).toBe('Hello\n\n#a #b');
      expect(fullText({ text: 'Hello', hashtags: [] })).toBe('Hello');
      expect(fullText({ text: '', hashtags: ['#a'] })).toBe('#a');
    });
  });

  describe('hashtags', () => {
    const cases: Record<string, string> = {
      '#BrassLamp': '#BrassLamp',
      'brass lamp': '#brasslamp',
      '#Brass Lamp!': '#BrassLamp',
      '＃Lamp': '#Lamp',
      '#दीपक': '#दीपक',
      'पीतल का दीपक': '#पीतलकादीपक',
      '#2024': '',
      '#': '',
      '#!!!': '',
      '#a_b': '#a_b',
    };
    for (const [input, want] of Object.entries(cases)) {
      it(`${JSON.stringify(input)} becomes ${JSON.stringify(want)}`, () => expect(normalizeHashtag(input)).toBe(want));
    }
    it('refuses one longer than 31 characters with the #', () => {
      expect(normalizeHashtag('#' + 'a'.repeat(30))).toBe('#' + 'a'.repeat(30));
      expect(normalizeHashtag('#' + 'a'.repeat(31))).toBe('');
    });
    it('splits a box on spaces and commas and reports what is not a hashtag', () => {
      expect(parseHashtags('#a1 #b2,#c3  lamp')).toEqual({ tags: ['#a1', '#b2', '#c3', '#lamp'], bad: [] });
      expect(parseHashtags('#a1 !!! #b2')).toEqual({ tags: ['#a1', '#b2'], bad: ['!!!'] });
      expect(parseHashtags('')).toEqual({ tags: [], bad: [] });
    });
  });

  describe('platformProblems, the same rules as the server', () => {
    const fine = { title: 'A title', text: 'A description.', hashtags: ['#a1'] };
    it('has nothing to say about a copy that is within its limits', () => {
      expect(hasProblems(platformProblems(YT, fine))).toBeFalse();
      expect(hasProblems(platformProblems(TT, { text: 'A caption', hashtags: [] }))).toBeFalse();
    });
    it('stops a YouTube title over 100 characters', () => {
      const p = platformProblems(YT, { ...fine, title: 'a'.repeat(101) });
      expect(p.title).toContain('101 characters');
      expect(p.title).toContain('limit is 100');
      expect(platformProblems(YT, { ...fine, title: 'a'.repeat(100) }).title).toBeUndefined();
    });
    it('counts a YouTube description in bytes, hashtags included', () => {
      // 1,700 Devanagari letters are 1,700 characters but 5,100 bytes.
      const p = platformProblems(YT, { ...fine, text: 'द'.repeat(1700) });
      expect(p.text).toContain('bytes');
      expect(p.text).toContain('limit is 5,000');
      expect(platformProblems(YT, { ...fine, text: 'द'.repeat(1600) }).text).toBeUndefined();
      // Hashtags are part of what is pasted.
      expect(platformProblems(YT, { ...fine, text: 'a'.repeat(4995), hashtags: ['#a1', '#b2'] }).text).toBeDefined();
    });
    it('counts a TikTok caption in UTF-16 units', () => {
      expect(platformProblems(TT, { text: 'a'.repeat(2200), hashtags: [] }).text).toBeUndefined();
      expect(platformProblems(TT, { text: 'a'.repeat(2199) + '😀', hashtags: [] }).text).toBeDefined();
    });
    it('needs a title where there is one, and a text everywhere', () => {
      expect(platformProblems(YT, { ...fine, title: '  ' }).title).toContain('needs a title');
      expect(platformProblems(TT, { text: '   ', hashtags: [] }).text).toContain('needs its caption');
    });
    it('refuses too many, repeated and unusable hashtags', () => {
      expect(platformProblems(TT, { text: 'x', hashtags: ['#a1', '#b2', '#c3', '#d4', '#e5', '#f6'] }).hashtags).toContain('at most 5');
      expect(platformProblems(TT, { text: 'x', hashtags: ['#Lamp', '#lamp'] }).hashtags).toContain('twice');
      expect(platformProblems(TT, { text: 'x', hashtags: ['#a1'] }, ['!!!']).hashtags).toContain('!!! is not a hashtag');
    });
  });

  it('makes request ids the server accepts', () => {
    const ids = new Set(Array.from({ length: 50 }, () => newRequestId()));
    expect(ids.size).toBe(50);
    for (const id of ids) expect(id).toMatch(/^[A-Za-z0-9_-]{16,64}$/);
  });

  it('tells Hindi text from the rest', () => {
    expect(langOf('दीपक')).toBe('hi');
    expect(langOf('A lamp')).toBeNull();
  });

  describe('quickCopyText', () => {
    const saved = (text: string, tags: string[] = []): PostCopySaved => ({
      copy: { tiktok: { text, hashtags: tags } },
      notes: [],
      source: 'generated',
      updatedAt: '2026-11-24T10:00:00Z',
    });
    it('uses the saved TikTok caption with its hashtags', () => {
      expect(quickCopyText(saved('A caption.', ['#a1', '#b2']), 'the hook', 1)).toBe('A caption.\n\n#a1 #b2');
    });
    it('uses the plain hook line otherwise, with no generic tags', () => {
      for (const out of [quickCopyText(undefined, 'the hook', 1), quickCopyText(saved('   '), 'the hook', 1)]) {
        expect(out).toBe('the hook');
        expect(out).not.toMatch(/#/);
      }
    });
    it('falls back to the clip number when there is neither', () => {
      expect(quickCopyText(undefined, '  ', 3)).toBe('Short #3');
      expect(quickCopyText(undefined, undefined, 3)).toBe('Short #3');
    });
  });
});
