import { defaultHook, hookOpacity, hookProblem, hookTextLength, sameHook } from './clip-hook.models';

describe('clip hook models', () => {
  it('counts characters the way a reader sees them, matching the server', () => {
    const cases: [string, number][] = [
      ['', 0],
      ['Stop scrolling', 14],
      ['cafe\u0301', 4],
      ['क्षमा', 2],
      ['नमस्ते', 3],
      ['हिंदी', 2],
      ['प्रश्न', 2],
      ['ज़िंदगी', 3],
      ['👍🏽', 1],
      ['👨‍👩‍👧', 1],
      ['🇮🇳🇺🇸', 2],
      ['a\r\nb', 3],
    ];
    for (const [text, want] of cases) expect(hookTextLength(text)).withContext(text).toBe(want);
  });

  it('allows 80 Hindi characters and refuses 81', () => {
    const h = { ...defaultHook(), enabled: true };
    expect(hookProblem({ ...h, text: 'क्ष'.repeat(80) })).toBe('');
    expect(hookProblem({ ...h, text: 'क्ष'.repeat(81) })).toContain('81 characters');
    expect(hookProblem({ ...h, text: 'a'.repeat(81) })).toContain('limit is 80');
  });

  it('names each problem', () => {
    const h = { ...defaultHook(), enabled: true, text: 'Watch this' };
    expect(hookProblem(h)).toBe('');
    expect(hookProblem({ ...h, text: '  ' })).toContain('Write the hook');
    expect(hookProblem({ ...h, enabled: false, text: '' })).toBe('');
    expect(hookProblem({ ...h, text: 'one\ntwo' })).toContain('one line');
    expect(hookProblem({ ...h, seconds: 0.5 })).toContain('1 to 5 seconds');
    expect(hookProblem({ ...h, seconds: 6 })).toContain('1 to 5 seconds');
  });

  it('compares hooks ignoring surrounding spaces and the save time', () => {
    const a = { ...defaultHook(), text: 'x' };
    expect(sameHook(a, { ...a, text: ' x ', updatedAt: 'later' })).toBeTrue();
    expect(sameHook(a, { ...a, style: 'band' })).toBeFalse();
  });

  it('fades 0.2 s in and out, never overlapping, and not at all with reduced motion', () => {
    expect(hookOpacity(0, 2.5, 30, false)).toBe(0);
    expect(hookOpacity(0.1, 2.5, 30, false)).toBeCloseTo(0.5, 5);
    expect(hookOpacity(1, 2.5, 30, false)).toBe(1);
    expect(hookOpacity(2.4, 2.5, 30, false)).toBeCloseTo(0.5, 5);
    expect(hookOpacity(2.5, 2.5, 30, false)).toBe(0);
    expect(hookOpacity(3, 2.5, 30, false)).toBe(0);
    // A clip shorter than the hook: the showing ends with the clip, each fade at most half.
    expect(hookOpacity(0.15, 2.5, 0.3, false)).toBeCloseTo(1, 5);
    for (let t = 0; t < 0.3; t += 0.01) expect(hookOpacity(t, 2.5, 0.3, false)).toBeLessThanOrEqual(1);
    expect(hookOpacity(0.01, 2.5, 30, true)).toBe(1);
    expect(hookOpacity(2.6, 2.5, 30, true)).toBe(0);
  });
});
