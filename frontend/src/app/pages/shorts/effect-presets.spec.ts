import { EFFECT_PRESETS } from './effect-presets';
import { ResolvedVideoEffects } from './shorts.service';

/**
 * These checks mirror the backend validation rules (models/effects.go) so a
 * preset can never produce an effect set the server would reject. If a rule
 * changes there, update it here too.
 */
function expectValid(e: ResolvedVideoEffects): void {
  // Caption intensity/size ranges.
  expect(e.captions.intensity).toBeGreaterThanOrEqual(0);
  expect(e.captions.intensity).toBeLessThanOrEqual(100);
  expect(e.captions.fontSize).toBeGreaterThanOrEqual(8);
  expect(e.captions.fontSize).toBeLessThanOrEqual(72);

  // Zoom: enabled requires at least one punch-in, each 0.5-6s, in 0-90s.
  expect(e.zoom.intensity).toBeGreaterThanOrEqual(0);
  expect(e.zoom.intensity).toBeLessThanOrEqual(100);
  if (e.zoom.enabled) {
    expect(e.zoom.punchIns.length).toBeGreaterThan(0);
  }
  for (const p of e.zoom.punchIns) {
    expect(p.at).toBeGreaterThanOrEqual(0);
    expect(p.at).toBeLessThanOrEqual(90);
    expect(p.duration).toBeGreaterThanOrEqual(0.5);
    expect(p.duration).toBeLessThanOrEqual(6);
  }

  // Grade look enum.
  expect(['cinematic', 'warm', 'cool', 'high-contrast']).toContain(e.grade.look);

  // Freeze holds 0.2-3s, impact text 0.3-4s and <= 16 chars, non-empty.
  for (const h of e.freeze.freezes) {
    expect(h.duration).toBeGreaterThanOrEqual(0.2);
    expect(h.duration).toBeLessThanOrEqual(3);
  }
  for (const t of e.freeze.impactTexts) {
    expect(t.duration).toBeGreaterThanOrEqual(0.3);
    expect(t.duration).toBeLessThanOrEqual(4);
    expect(t.text.trim().length).toBeGreaterThan(0);
    expect(t.text.length).toBeLessThanOrEqual(16);
  }

  // Fade sides 0-2s.
  expect(e.fade.inDuration).toBeGreaterThanOrEqual(0);
  expect(e.fade.inDuration).toBeLessThanOrEqual(2);
  expect(e.fade.outDuration).toBeGreaterThanOrEqual(0);
  expect(e.fade.outDuration).toBeLessThanOrEqual(2);

  // Finish strengths 0-100; if enabled, at least one above 0.
  for (const v of [e.finish.grain, e.finish.glow, e.finish.lightLeak]) {
    expect(v).toBeGreaterThanOrEqual(0);
    expect(v).toBeLessThanOrEqual(100);
  }
  if (e.finish.enabled) {
    expect(e.finish.grain + e.finish.glow + e.finish.lightLeak).toBeGreaterThan(0);
  }

  // The one hard cross-effect rule: freeze and speed cannot both be enabled.
  expect(e.freeze.enabled && e.speed.enabled).toBeFalse();
}

describe('effect presets', () => {
  it('ships the five presets from the issue', () => {
    expect(EFFECT_PRESETS.map((p) => p.id)).toEqual([
      'viral-captions',
      'podcast-pro',
      'cinematic-story',
      'motivation',
      'meme-mode',
    ]);
  });

  for (const preset of EFFECT_PRESETS) {
    describe(preset.id, () => {
      it('builds a valid, server-acceptable effect set', () => {
        expectValid(preset.build());
      });

      it('enables at least one effect', () => {
        const e = preset.build();
        const anyOn =
          e.captions.enabled || e.zoom.enabled || e.grade.enabled ||
          e.freeze.enabled || e.fade.enabled || e.speed.enabled ||
          e.finish.enabled || e.waveform.enabled || e.shake.enabled;
        expect(anyOn).withContext(preset.id).toBeTrue();
      });

      it('returns an independent object each time (no shared mutable state)', () => {
        const a = preset.build();
        const b = preset.build();
        expect(a).not.toBe(b);
        a.zoom.punchIns.push({ at: 1, duration: 1 });
        expect(b.zoom.punchIns.length).not.toBe(a.zoom.punchIns.length);
      });

      it('has a label and a blurb', () => {
        expect(preset.label.length).toBeGreaterThan(0);
        expect(preset.blurb.length).toBeGreaterThan(10);
      });
    });
  }

  it('Cinematic Story uses a fade and does not claim a transition it lacks', () => {
    const e = EFFECT_PRESETS.find((p) => p.id === 'cinematic-story')!;
    expect(e.build().fade.enabled).toBeTrue();
    expect(e.blurb.toLowerCase()).toContain('fade');
  });

  it('Meme Mode is the reduced form: freeze and text, no promise of stickers or sound', () => {
    const p = EFFECT_PRESETS.find((p) => p.id === 'meme-mode')!;
    const e = p.build();
    expect(e.freeze.enabled).toBeTrue();
    expect(e.freeze.impactTexts.length).toBeGreaterThan(0);
    expect(p.blurb.toLowerCase()).toContain('not available');
  });
});
