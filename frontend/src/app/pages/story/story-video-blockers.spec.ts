import { videoBlocker } from './story-video-blockers';
import { StoryPlanRecord } from './story-plan.models';
import { planRecord } from './story-plan-fixtures.spec';

/** Every scene of the fixture plan gets a picture. */
function withAllPictures(): StoryPlanRecord {
  const rec = planRecord();
  rec.plan!.scenes[2].media = { provider: 'pixabay', type: 'photo', id: '303' };
  rec.media = [rec.media![0], rec.media![1], { provider: 'pixabay', type: 'photo', id: '303' } as never];
  return rec;
}

describe('videoBlocker', () => {
  it('is empty when the saved plan has a picture for every scene', () => {
    expect(videoBlocker(withAllPictures(), false)).toBe('');
  });

  it('asks to save first, before anything else, because the video uses the saved plan', () => {
    const msg = videoBlocker(planRecord(), true);
    expect(msg).toContain('Save your changes first');
    expect(msg).toContain('not from what is on screen');
  });

  it('names the one scene with no picture', () => {
    expect(videoBlocker(planRecord(), false)).toBe('Scene 3 has no picture. Choose one, then save.');
  });

  it('names several scenes with no picture', () => {
    const rec = withAllPictures();
    delete rec.plan!.scenes[0].media;
    delete rec.plan!.scenes[2].media;
    expect(videoBlocker(rec, false)).toBe('Scenes 1 and 3 have no picture. Choose one for each, then save.');
    delete rec.plan!.scenes[1].media;
    expect(videoBlocker(rec, false)).toBe('Scenes 1, 2 and 3 have no picture. Choose one for each, then save.');
  });

  it('says when a chosen picture is gone from the photo library', () => {
    const rec = withAllPictures();
    rec.media![1] = { provider: 'pexels', type: 'video', id: '202', status: 'unavailable' };
    expect(videoBlocker(rec, false)).toBe("The picture for scene 2 can't be used any more: the photo library no longer has it. Choose another, then save.");
  });

  it('says it for several scenes in the plural', () => {
    const rec = withAllPictures();
    rec.media![0] = { provider: 'pixabay', type: 'photo', id: '101', status: 'unavailable' };
    rec.media![1] = { provider: 'pexels', type: 'video', id: '202', status: 'unavailable' };
    expect(videoBlocker(rec, false)).toBe("The pictures for scenes 1 and 2 can't be used any more: the photo library no longer has them. Choose another, then save.");
  });

  it('does not block on a picture that could not be looked up for now', () => {
    const rec = withAllPictures();
    rec.media![0] = { provider: 'pixabay', type: 'photo', id: '101', status: 'error' };
    expect(videoBlocker(rec, false)).toBe('');
  });

  it('puts a missing picture before a gone one', () => {
    const rec = planRecord();
    rec.media![0] = { provider: 'pixabay', type: 'photo', id: '101', status: 'unavailable' };
    expect(videoBlocker(rec, false)).toContain('Scene 3 has no picture');
  });

  it('uses no em dash', () => {
    expect(videoBlocker(planRecord(), true)).not.toContain('\u2014');
    expect(videoBlocker(planRecord(), false)).not.toContain('\u2014');
  });
});
