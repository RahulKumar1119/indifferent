import {
  PendingMedia,
  buildEdits,
  changedSceneCount,
  currentRef,
  sceneMediaView,
  withPick,
  withRemoval,
} from './story-plan-edits';
import { MAX_QUERY_CHARS, charCount, mediaKey, sceneSearchQuery, toMediaRef } from './story-plan.models';
import { asset, planRecord } from './story-plan-fixtures.spec';

describe('story plan edits', () => {
  const none: (PendingMedia | undefined)[] = [];

  describe('sceneMediaView', () => {
    it('shows the saved picture as it is now', () => {
      const v = sceneMediaView(planRecord(), 0, undefined);
      expect(v.state === 'asset' && v.asset.id).toBe('101');
      expect(v.state === 'asset' && v.changed).toBeFalse();
    });

    it('shows nothing for a scene with no saved picture', () => {
      expect(sceneMediaView(planRecord(), 2, undefined)).toEqual({ state: 'none' });
    });

    it('shows a pending pick over the saved one', () => {
      const v = sceneMediaView(planRecord(), 0, { kind: 'set', asset: asset({ id: '555' }) });
      expect(v.state === 'asset' && v.asset.id).toBe('555');
      expect(v.state === 'asset' && v.changed).toBeTrue();
    });

    it('shows nothing for a pending removal', () => {
      expect(sceneMediaView(planRecord(), 0, { kind: 'clear' })).toEqual({ state: 'none' });
    });

    it('tells a picture the provider removed from one that failed to load', () => {
      const rec = planRecord();
      rec.media![0] = { provider: 'pixabay', type: 'photo', id: '101', status: 'unavailable' };
      rec.media![1] = { provider: 'pexels', type: 'video', id: '202', status: 'error' };
      expect(sceneMediaView(rec, 0, undefined).state).toBe('unavailable');
      expect(sceneMediaView(rec, 1, undefined).state).toBe('error');
    });

    it('treats a saved pick the API did not describe as a failure to load, not as no picture', () => {
      const rec = planRecord();
      delete rec.media;
      const v = sceneMediaView(rec, 0, undefined);
      expect(v.state).toBe('error');
    });

    it('survives a plan with no scenes', () => {
      expect(sceneMediaView(planRecord({ plan: undefined }), 0, undefined)).toEqual({ state: 'none' });
    });
  });

  describe('currentRef', () => {
    it('follows a pending change, else the saved pick', () => {
      const rec = planRecord();
      expect(currentRef(rec, 0, undefined)).toEqual({ provider: 'pixabay', type: 'photo', id: '101' });
      expect(currentRef(rec, 0, { kind: 'set', asset: asset({ id: '9' }) })).toEqual({ provider: 'pixabay', type: 'photo', id: '9' });
      expect(currentRef(rec, 0, { kind: 'clear' })).toBeNull();
      expect(currentRef(rec, 2, undefined)).toBeNull();
    });
  });

  describe('withPick', () => {
    it('records a new pick for that scene only', () => {
      const next = withPick(planRecord(), none, 2, asset({ id: '7' }));
      expect(next[2]).toEqual({ kind: 'set', asset: asset({ id: '7' }) });
      expect(next[0]).toBeUndefined();
    });

    it('does not change the list it was given', () => {
      const before: (PendingMedia | undefined)[] = [];
      withPick(planRecord(), before, 0, asset({ id: '7' }));
      expect(before).toEqual([]);
    });

    it('cancels a pending change when the saved picture is chosen again', () => {
      const rec = planRecord();
      const changed = withPick(rec, none, 0, asset({ id: '7' }));
      expect(withPick(rec, changed, 0, asset({ id: '101' }))[0]).toBeUndefined();
    });

    it('does not take the same id from another library or kind for the saved one', () => {
      const rec = planRecord();
      expect(withPick(rec, none, 0, asset({ id: '101', provider: 'pexels' }))[0]).toBeDefined();
      expect(withPick(rec, none, 0, asset({ id: '101', type: 'video' }))[0]).toBeDefined();
    });
  });

  describe('withRemoval', () => {
    it('clears a saved picture', () => {
      expect(withRemoval(planRecord(), none, 0)[0]).toEqual({ kind: 'clear' });
    });

    it('only drops the pending pick of a scene that had no saved picture', () => {
      const rec = planRecord();
      const picked = withPick(rec, none, 2, asset({ id: '7' }));
      expect(withRemoval(rec, picked, 2)[2]).toBeUndefined();
    });
  });

  describe('buildEdits', () => {
    const rec = planRecord();
    const same = rec.plan!.scenes.map((s) => s.narration);

    it('is one empty entry per scene when nothing changed', () => {
      expect(buildEdits(rec, same, none)).toEqual([{}, {}, {}]);
      expect(changedSceneCount(buildEdits(rec, same, none))).toBe(0);
    });

    it('sends narration only for scenes whose text changed', () => {
      const edits = buildEdits(rec, [same[0], 'New words for scene two.', same[2]], none);
      expect(edits).toEqual([{}, { narration: 'New words for scene two.' }, {}]);
    });

    it('ignores blanks around otherwise unchanged text', () => {
      expect(buildEdits(rec, [`  ${same[0]}\n`, same[1], same[2]], none)).toEqual([{}, {}, {}]);
    });

    it('sends an empty narration, so the API can say what is wrong with it', () => {
      expect(buildEdits(rec, ['', same[1], same[2]], none)[0]).toEqual({ narration: '' });
    });

    it('sends a pick as a reference with no URL in it', () => {
      const edits = buildEdits(rec, same, [undefined, { kind: 'set', asset: asset({ id: '7', provider: 'pexels', type: 'video' }) }]);
      expect(edits[1]).toEqual({ media: { provider: 'pexels', type: 'video', id: '7' } });
      expect(JSON.stringify(edits)).not.toMatch(/http|preview|credit/);
    });

    it('sends a removal as clearMedia', () => {
      expect(buildEdits(rec, same, [{ kind: 'clear' }])[0]).toEqual({ clearMedia: true });
    });

    it('can change narration and picture of one scene together', () => {
      const edits = buildEdits(rec, ['Changed.', same[1], same[2]], [{ kind: 'set', asset: asset({ id: '8' }) }]);
      expect(edits[0]).toEqual({ narration: 'Changed.', media: { provider: 'pixabay', type: 'photo', id: '8' } });
      expect(changedSceneCount(edits)).toBe(1);
    });

    it('has no entries for a record with no plan', () => {
      expect(buildEdits(planRecord({ plan: undefined }), [], none)).toEqual([]);
    });
  });
});

describe('story plan models', () => {
  describe('charCount', () => {
    it('counts what a person sees as one character as one', () => {
      expect(charCount('abc')).toBe(3);
      expect(charCount('🌊🌊')).toBe(2);
      expect(charCount('नमस्ते')).toBe(6);
      expect(charCount('')).toBe(0);
    });
  });

  describe('sceneSearchQuery', () => {
    it('joins the three strongest keywords', () => {
      expect(sceneSearchQuery({ keywords: ['a', 'b', 'c', 'd'] })).toBe('a b c');
      expect(sceneSearchQuery({ keywords: ['only one'] })).toBe('only one');
      expect(sceneSearchQuery({ keywords: [] })).toBe('');
    });

    it('stops before it would go over the API\'s limit, and never cuts a keyword in half', () => {
      const long = 'x'.repeat(60);
      const q = sceneSearchQuery({ keywords: [long, long] });
      expect(q).toBe(long);
      expect(charCount(q)).toBeLessThanOrEqual(MAX_QUERY_CHARS);
    });
  });

  it('makes the same key for the same asset and tells libraries and kinds apart', () => {
    expect(mediaKey(asset({ id: '1' }))).toBe(mediaKey(toMediaRef(asset({ id: '1' }))));
    expect(mediaKey(asset({ id: '1' }))).not.toBe(mediaKey(asset({ id: '1', provider: 'pexels' })));
    expect(mediaKey(asset({ id: '1' }))).not.toBe(mediaKey(asset({ id: '1', type: 'video' })));
  });
});
