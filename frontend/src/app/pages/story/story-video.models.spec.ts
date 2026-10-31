import {
  CAPTION_PRESETS,
  STAGES,
  captionLabel,
  formatLength,
  isActive,
  languageName,
  sceneInMessage,
  stageIndex,
  voiceNote,
  wasCancelled,
} from './story-video.models';

describe('story video models', () => {
  describe('stages', () => {
    it('are the three things the worker does, in its order', () => {
      expect(STAGES.map((s) => s.status)).toEqual(['preparing', 'narrating', 'rendering']);
    });

    it('place each status: before the first, at one, or past the last', () => {
      expect(stageIndex('queued')).toBe(-1);
      expect(stageIndex('preparing')).toBe(0);
      expect(stageIndex('narrating')).toBe(1);
      expect(stageIndex('rendering')).toBe(2);
      expect(stageIndex('completed')).toBe(3);
    });

    it('tell what is still going on from what has ended', () => {
      for (const s of ['queued', 'preparing', 'narrating', 'rendering'] as const) expect(isActive(s)).toBeTrue();
      for (const s of ['completed', 'failed'] as const) expect(isActive(s)).toBeFalse();
    });
  });

  describe('wasCancelled', () => {
    it('is true only for a failed video with the cancel code', () => {
      expect(wasCancelled({ status: 'failed', errorCode: 'CANCELLED' })).toBeTrue();
      expect(wasCancelled({ status: 'failed', errorCode: 'INTERNAL' })).toBeFalse();
      expect(wasCancelled({ status: 'rendering', errorCode: 'CANCELLED' })).toBeFalse();
      expect(wasCancelled({ status: 'completed' })).toBeFalse();
    });
  });

  describe('formatLength', () => {
    it('reads as seconds, then minutes and seconds', () => {
      expect(formatLength(0)).toBe('0 seconds');
      expect(formatLength(1)).toBe('1 second');
      expect(formatLength(41.2)).toBe('41 seconds');
      expect(formatLength(59.6)).toBe('1 minute');
      expect(formatLength(60)).toBe('1 minute');
      expect(formatLength(78)).toBe('1 minute 18 seconds');
      expect(formatLength(121)).toBe('2 minutes 1 second');
    });

    it('does not go below zero', () => {
      expect(formatLength(-5)).toBe('0 seconds');
    });
  });

  describe('sceneInMessage', () => {
    it('finds the scene the API names at the start of its sentence', () => {
      expect(sceneInMessage('Scene 2: the picture is no longer available from Pixabay. Choose another and make the video again.')).toBe(2);
      expect(sceneInMessage('Scene 10: the chosen picture can\'t be used.')).toBe(10);
    });

    it('finds nothing in a message about the whole video, or in none', () => {
      expect(sceneInMessage('The narration runs 94 seconds, over the 90 second limit.')).toBeNull();
      expect(sceneInMessage('Making the video took too long. Scene 2 was slow.')).toBeNull();
      expect(sceneInMessage('The video was not finished. Scene 2: the picture is gone.')).toBeNull();
      expect(sceneInMessage(' Scene 2: starts with a space.')).toBeNull();
      expect(sceneInMessage('Scene 2 has no picture')).toBeNull();
      expect(sceneInMessage('')).toBeNull();
      expect(sceneInMessage(undefined)).toBeNull();
    });
  });

  describe('captions', () => {
    it('offer the four looks the renderer has, with the names the Shorts page uses', () => {
      expect(CAPTION_PRESETS.map((p) => p.id).sort()).toEqual(['bold', 'clean', 'karaoke', 'minimal']);
      expect(CAPTION_PRESETS.find((p) => p.id === 'karaoke')!.label).toBe('Karaoke highlight');
    });

    it('name a video\'s look, and the default look when none is stored', () => {
      expect(captionLabel({ preset: 'clean' })).toBe('Clean subtitles');
      expect(captionLabel({})).toBe('Bold');
      expect(captionLabel(undefined)).toBe('Bold');
    });

    it('give every look a sentence that says what it is', () => {
      for (const p of CAPTION_PRESETS) expect(p.blurb.length).toBeGreaterThan(10);
    });
  });

  describe('voices', () => {
    it('describe each English voice, and say what Hindi narration is', () => {
      expect(voiceNote('Matthew', 'en')).toBe('US English, male, warm and authoritative');
      expect(voiceNote('Aditi', 'hi')).toBe('The voice for Hindi narration');
      expect(voiceNote('Zoltan', 'en')).toBe('');
    });

    it('name the languages, and fall back to the code', () => {
      expect(languageName('en')).toBe('English');
      expect(languageName('hi')).toBe('Hindi');
      expect(languageName('fr')).toBe('fr');
    });
  });

  it('uses no em dash in any text it shows', () => {
    const text = JSON.stringify([STAGES, CAPTION_PRESETS]);
    expect(text).not.toContain('\u2014');
  });
});
