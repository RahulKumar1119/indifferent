import { StockAsset } from './story-plan.models';
import { asset, planRecord } from './story-plan-fixtures.spec';
import {
  MAX_SCENE_OPS,
  WorkingPlan,
  buildWorkingEdits,
  mergeScenes,
  moveScene,
  opsToSend,
  pickFor,
  removalFor,
  splitScene,
  startWorking,
  structureChanged,
  withNarration,
} from './story-plan-structure';

const LIM = { minScenes: 2, maxScenes: 10 };

describe('story plan structure', () => {
  const rec = planRecord(); // three scenes; the first two have pictures
  const fresh = () => startWorking(rec);
  const order = (w: WorkingPlan) => w.scenes.map((s) => s.src).join('');
  const ok = (r: ReturnType<typeof moveScene>): WorkingPlan => {
    if (!r.ok) throw new Error(r.reason);
    return r.plan;
  };

  it('starts as the saved plan, with nothing to send', () => {
    const w = fresh();
    expect(order(w)).toBe('012');
    expect(structureChanged(w, 3)).toBeFalse();
    expect(opsToSend(rec, w)).toEqual([]);
    expect(buildWorkingEdits(rec, w)).toEqual([{}, {}, {}]);
  });

  describe('moving', () => {
    it('moves a scene and records the 1-based operation', () => {
      const w = ok(moveScene(fresh(), 0, 2));
      expect(order(w)).toBe('120');
      expect(w.ops).toEqual([{ op: 'move', from: 1, to: 3 }]);
      expect(opsToSend(rec, w)).toEqual(w.ops);
    });

    it('sends nothing when the order is put back', () => {
      const w = ok(moveScene(ok(moveScene(fresh(), 0, 1)), 1, 0));
      expect(order(w)).toBe('012');
      expect(opsToSend(rec, w)).toEqual([]);
    });

    it('sends no narration for scenes that only moved', () => {
      expect(buildWorkingEdits(rec, ok(moveScene(fresh(), 2, 0)))).toEqual([{}, {}, {}]);
    });

    it('refuses places outside the plan, and staying put', () => {
      for (const [from, to] of [[0, 3], [3, 0], [-1, 1], [1, 1]]) {
        expect(moveScene(fresh(), from, to).ok).withContext(`${from}->${to}`).toBeFalse();
      }
    });

    it('a picture travels with its scene', () => {
      const w = ok(moveScene(fresh(), 0, 2));
      expect(w.scenes[2].src).toBe(0);
    });
  });

  describe('splitting', () => {
    it('cuts the narration at the cursor and gives both halves the same picture source', () => {
      const text = rec.plan!.scenes[0].narration; // 'The lighthouse stood alone on the cliff.'
      const w = ok(splitScene(fresh(), 0, 'The lighthouse stood alone'.length, LIM));
      expect(w.scenes.length).toBe(4);
      expect(w.scenes[0].narration).toBe('The lighthouse stood alone');
      expect(w.scenes[1].narration).toBe('on the cliff.');
      expect(`${w.scenes[0].narration} ${w.scenes[1].narration}`).toBe(text);
      expect(w.scenes[0].src).toBe(0);
      expect(w.scenes[1].src).toBe(0);
      expect(w.ops).toEqual([{ op: 'split', scene: 1 }]);
    });

    it('sends the narration of both halves and nothing else', () => {
      const w = ok(splitScene(fresh(), 1, 14, LIM));
      const edits = buildWorkingEdits(rec, w);
      expect(edits.length).toBe(4);
      expect(edits[1].narration).toBeDefined();
      expect(edits[2].narration).toBeDefined();
      expect(edits[0]).toEqual({});
      expect(edits[3]).toEqual({});
    });

    it('copies an unsaved picture choice to the second half', () => {
      const a: StockAsset = asset({ id: '555' });
      const picked = pickFor(rec, fresh(), 0, a);
      const w = ok(splitScene(picked, 0, 10, LIM));
      expect(w.scenes[0].pending).toEqual({ kind: 'set', asset: a });
      expect(w.scenes[1].pending).toEqual({ kind: 'set', asset: a });
    });

    it('needs words on both sides of the cursor', () => {
      for (const at of [0, 1000]) expect(splitScene(fresh(), 0, at, LIM).ok).withContext(`${at}`).toBeFalse();
      const w = withNarration(fresh(), 0, 'one two   ');
      expect(splitScene(w, 0, 7, LIM).ok).toBeFalse();
    });

    it('stops at the most scenes a video may have', () => {
      let w = withNarration(fresh(), 2, 'w '.repeat(40));
      while (w.scenes.length < LIM.maxScenes) w = ok(splitScene(w, w.scenes.length - 1, 2, LIM));
      const r = splitScene(w, 0, 1, LIM);
      expect(r.ok).toBeFalse();
    });
  });

  describe('merging', () => {
    it('joins a scene with the next and keeps the first picture source', () => {
      const w = ok(mergeScenes(fresh(), 0, LIM));
      expect(w.scenes.length).toBe(2);
      expect(w.scenes[0].src).toBe(0);
      expect(w.scenes[0].narration).toBe(
        `${rec.plan!.scenes[0].narration} ${rec.plan!.scenes[1].narration}`,
      );
      expect(w.ops).toEqual([{ op: 'merge', scene: 1 }]);
      expect(buildWorkingEdits(rec, w)[0].narration).toBe(w.scenes[0].narration);
    });

    it('refuses the last scene, a plan at the minimum, and text too long for one scene', () => {
      expect(mergeScenes(fresh(), 2, LIM).ok).toBeFalse();
      const two = ok(mergeScenes(fresh(), 0, LIM));
      expect(mergeScenes(two, 0, LIM).ok).toBeFalse();
      const long = withNarration(withNarration(fresh(), 0, 'a'.repeat(400)), 1, 'b'.repeat(400));
      expect(mergeScenes(long, 0, LIM).ok).toBeFalse();
    });
  });

  it('composes operations in order, as the server applies them', () => {
    let w = ok(splitScene(fresh(), 0, 10, LIM)); // A A' B C
    w = ok(moveScene(w, 1, 3)); // A B C A'
    w = ok(mergeScenes(w, 2, LIM)); // A B (C+A')
    expect(order(w)).toBe('012');
    expect(w.ops.map((o) => o.op)).toEqual(['split', 'move', 'merge']);
    const edits = buildWorkingEdits(rec, w);
    expect(edits.length).toBe(3);
    expect(edits[0].narration).toBeDefined(); // first half of the split
    expect(edits[1].narration).toBeUndefined();
    expect(edits[2].narration).toBeDefined(); // the merged scene
  });

  it('refuses a change once the operation cap is reached', () => {
    let w: WorkingPlan = fresh();
    for (let i = 0; i < MAX_SCENE_OPS; i++) w = ok(moveScene(w, 0, 1));
    expect(moveScene(w, 0, 1).ok).toBeFalse();
  });

  describe('pictures on a working list', () => {
    it('picking the saved picture undoes the change, by source scene', () => {
      const moved = ok(moveScene(fresh(), 0, 2));
      const same = pickFor(rec, moved, 2, asset({ id: '101' }));
      expect(same.scenes[2].pending).toBeUndefined();
      const other = pickFor(rec, moved, 2, asset({ id: '9' }));
      expect(buildWorkingEdits(rec, other)[2].media?.id).toBe('9');
    });

    it('removing clears a saved picture and just drops an unsaved choice', () => {
      expect(buildWorkingEdits(rec, removalFor(rec, fresh(), 0))[0].clearMedia).toBeTrue();
      expect(removalFor(rec, fresh(), 2).scenes[2].pending).toBeUndefined();
    });
  });
});
