import { ResolvedVideoEffects, defaultVideoEffects } from './shorts.service';

/**
 * One-click editing preset (#164 Phase 4). Each preset is a named combination
 * of the effects built in phases 1-3. Applying one fills the effects panel with
 * that combination; the user can then edit any control before saving. Presets
 * are a client-side convenience only: the server validates and renders whatever
 * effect set it receives, so no preset can produce anything the panel could not.
 */
export interface EffectPreset {
  id: string;
  label: string;
  /** One line shown under the button. States honestly what the preset does. */
  blurb: string;
  /** Builds a fresh effect set. Always starts from the defaults, so unused
   * effects are off with sensible values the user can turn on later. */
  build: () => ResolvedVideoEffects;
}

/**
 * The five presets from #164. Times for zoom punch-ins and impact text are
 * clip-relative seconds near the clip start, chosen to be valid on a typical
 * 30-90s clip; the user adjusts them after applying.
 *
 * Honesty notes carried in the blurbs:
 * - Cinematic Story uses the per-clip fade, not a cross-clip transition, which
 *   the product does not have.
 * - Meme Mode is the reduced form: freeze frames and impact text only. The
 *   issue's stickers and sound effects are not built, so the preset does not
 *   promise them.
 */
export const EFFECT_PRESETS: EffectPreset[] = [
  {
    id: 'viral-captions',
    label: 'Viral Captions',
    blurb: 'Animated captions with subtle zoom punch-ins.',
    build: () => {
      const e = defaultVideoEffects();
      e.captions.enabled = true;
      e.captions.intensity = 60;
      e.zoom.enabled = true;
      e.zoom.intensity = 30;
      e.zoom.punchIns = [
        { at: 3, duration: 1 },
        { at: 12, duration: 1 },
      ];
      return e;
    },
  },
  {
    id: 'podcast-pro',
    label: 'Podcast Pro',
    blurb: 'Word-by-word captions, dynamic zoom and a line of impact text.',
    build: () => {
      const e = defaultVideoEffects();
      e.captions.enabled = true;
      e.captions.intensity = 55;
      e.zoom.enabled = true;
      e.zoom.intensity = 45;
      e.zoom.punchIns = [
        { at: 2, duration: 1.5 },
        { at: 10, duration: 1.5 },
      ];
      e.freeze.enabled = true;
      e.freeze.intensity = 50;
      e.freeze.impactTexts = [{ at: 1, duration: 2, text: 'LISTEN' }];
      return e;
    },
  },
  {
    id: 'cinematic-story',
    label: 'Cinematic Story',
    blurb: 'Cinematic color, a slow zoom, film grain and a gentle fade. (Fade, not a cross-clip transition.)',
    build: () => {
      const e = defaultVideoEffects();
      e.grade.enabled = true;
      e.grade.look = 'cinematic';
      e.grade.intensity = 60;
      e.zoom.enabled = true;
      e.zoom.intensity = 20;
      e.zoom.punchIns = [{ at: 1, duration: 5 }];
      e.finish.enabled = true;
      e.finish.grain = 30;
      e.finish.glow = 20;
      e.finish.lightLeak = 0;
      e.fade.enabled = true;
      e.fade.inDuration = 0.6;
      e.fade.outDuration = 0.8;
      return e;
    },
  },
  {
    id: 'motivation',
    label: 'Motivation',
    blurb: 'Bold captions, a strong zoom and a cinematic grade.',
    build: () => {
      const e = defaultVideoEffects();
      e.captions.enabled = true;
      e.captions.intensity = 70;
      e.captions.fontSize = 24;
      e.zoom.enabled = true;
      e.zoom.intensity = 70;
      e.zoom.punchIns = [
        { at: 2, duration: 1 },
        { at: 8, duration: 1 },
      ];
      e.grade.enabled = true;
      e.grade.look = 'cinematic';
      e.grade.intensity = 70;
      return e;
    },
  },
  {
    id: 'meme-mode',
    label: 'Meme Mode',
    blurb: 'Freeze frames and impact text. (Stickers and sound effects are not available yet.)',
    build: () => {
      const e = defaultVideoEffects();
      e.freeze.enabled = true;
      e.freeze.intensity = 70;
      e.freeze.freezes = [{ at: 2, duration: 0.5 }];
      e.freeze.impactTexts = [
        { at: 2, duration: 1.5, text: 'WAIT' },
        { at: 5, duration: 1.5, text: 'LOL' },
      ];
      return e;
    },
  },
];
