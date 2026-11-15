import { PendingMedia } from './story-plan-edits';
import {
  MAX_NARRATION_CHARS,
  SceneEdit,
  StockAsset,
  StoryPlanRecord,
  charCount,
  mediaKey,
  toMediaRef,
} from './story-plan.models';

/**
 * Splitting, merging and reordering scenes (#326). The page keeps these as
 * unsaved edits, like narration and pictures, and sends them with the save.
 * `applyPlanChange` on the server runs the same operations in the same order;
 * keep the two in step (backend/internal/storyplan/ops.go).
 */

/** The operations the API accepts. Scene numbers are 1-based, in the list as it stands. */
export type SceneOp =
  | { op: 'move'; from: number; to: number }
  | { op: 'split'; scene: number }
  | { op: 'merge'; scene: number };

/** The API refuses a save with more operations than this. */
export const MAX_SCENE_OPS = 100;

/** One scene as the page shows it. */
export interface WorkScene {
  /** The saved scene it began as: where its picture and settings come from. */
  src: number;
  /** The narration as typed. */
  narration: string;
  /** A picture change not saved yet. */
  pending?: PendingMedia;
  /** Made by a split or a merge, so the save must send its narration. */
  touched: boolean;
}

/** The scenes in the order shown, and the operations that lead there from the saved plan. */
export interface WorkingPlan {
  scenes: WorkScene[];
  ops: SceneOp[];
}

export interface StructureLimits {
  minScenes: number;
  maxScenes: number;
}

/** Either the next state or the reason the change cannot be made. */
export type StructureResult = { ok: true; plan: WorkingPlan; announce: string } | { ok: false; reason: string };

/** The saved plan, unchanged. */
export function startWorking(rec: StoryPlanRecord): WorkingPlan {
  return {
    scenes: (rec.plan?.scenes ?? []).map((s, i) => ({ src: i, narration: s.narration, touched: false })),
    ops: [],
  };
}

const full = (w: WorkingPlan): StructureResult | null =>
  w.ops.length >= MAX_SCENE_OPS
    ? { ok: false, reason: 'That is too many changes to the scene order at once. Save, then continue.' }
    : null;

/** Moves the scene at `from` so it is at `to` (0-based). */
export function moveScene(w: WorkingPlan, from: number, to: number): StructureResult {
  const n = w.scenes.length;
  if (from < 0 || from >= n || to < 0 || to >= n || from === to) return { ok: false, reason: 'That scene cannot move there.' };
  const tooMany = full(w);
  if (tooMany) return tooMany;
  const scenes = [...w.scenes];
  const [s] = scenes.splice(from, 1);
  scenes.splice(to, 0, s);
  return {
    ok: true,
    plan: { scenes, ops: [...w.ops, { op: 'move', from: from + 1, to: to + 1 }] },
    announce: `Scene ${from + 1} moved to position ${to + 1} of ${n}.`,
  };
}

/**
 * Cuts scene `index` in two at character `at` of its narration. The second half
 * keeps the same picture and settings as the first.
 */
export function splitScene(w: WorkingPlan, index: number, at: number, limits: StructureLimits): StructureResult {
  const cur = w.scenes[index];
  if (!cur) return { ok: false, reason: 'That scene cannot be split.' };
  if (w.scenes.length >= limits.maxScenes) {
    return { ok: false, reason: `A video has at most ${limits.maxScenes} scenes, so this one cannot be split.` };
  }
  const tooMany = full(w);
  if (tooMany) return tooMany;
  const cut = Math.max(0, Math.min(at, cur.narration.length));
  const first = cur.narration.slice(0, cut).trim();
  const second = cur.narration.slice(cut).trim();
  if (!first || !second) {
    return { ok: false, reason: 'Put the cursor in the narration where the scene should end, with words on both sides.' };
  }
  const a: WorkScene = { ...cur, narration: first, touched: true };
  const b: WorkScene = { ...cur, narration: second, touched: true };
  const scenes = [...w.scenes];
  scenes.splice(index, 1, a, b);
  return {
    ok: true,
    plan: { scenes, ops: [...w.ops, { op: 'split', scene: index + 1 }] },
    announce: `Scene ${index + 1} split into scenes ${index + 1} and ${index + 2}.`,
  };
}

/** Joins scene `index` with the one after it. The result keeps the first scene's picture. */
export function mergeScenes(w: WorkingPlan, index: number, limits: StructureLimits): StructureResult {
  const a = w.scenes[index];
  const b = w.scenes[index + 1];
  if (!a || !b) return { ok: false, reason: 'There is no scene after this one to merge with.' };
  if (w.scenes.length <= limits.minScenes) {
    return { ok: false, reason: `A video needs at least ${limits.minScenes} scenes, so these cannot be merged.` };
  }
  const tooMany = full(w);
  if (tooMany) return tooMany;
  const narration = `${a.narration.trim()} ${b.narration.trim()}`.trim();
  if (charCount(narration) > MAX_NARRATION_CHARS) {
    return { ok: false, reason: 'Together these narrations are too long for one scene. Shorten one first.' };
  }
  const scenes = [...w.scenes];
  scenes.splice(index, 2, { ...a, narration, touched: true });
  return {
    ok: true,
    plan: { scenes, ops: [...w.ops, { op: 'merge', scene: index + 1 }] },
    announce: `Scenes ${index + 1} and ${index + 2} merged into scene ${index + 1}.`,
  };
}

/** True when the order or the number of scenes differs from what is saved. */
export function structureChanged(w: WorkingPlan, savedCount: number): boolean {
  return w.scenes.length !== savedCount || w.scenes.some((s, i) => s.src !== i || s.touched);
}

/** Records a chosen asset for scene `index`; choosing what is saved for it undoes the change. */
export function pickFor(rec: StoryPlanRecord, w: WorkingPlan, index: number, asset: StockAsset): WorkingPlan {
  const cur = w.scenes[index];
  if (!cur) return w;
  const saved = rec.plan?.scenes[cur.src]?.media;
  const pending: PendingMedia | undefined = saved && mediaKey(saved) === mediaKey(asset) ? undefined : { kind: 'set', asset };
  return replace(w, index, { ...cur, pending });
}

/** Removes the picture of scene `index`. */
export function removalFor(rec: StoryPlanRecord, w: WorkingPlan, index: number): WorkingPlan {
  const cur = w.scenes[index];
  if (!cur) return w;
  const pending: PendingMedia | undefined = rec.plan?.scenes[cur.src]?.media ? { kind: 'clear' } : undefined;
  return replace(w, index, { ...cur, pending });
}

export function withNarration(w: WorkingPlan, index: number, text: string): WorkingPlan {
  const cur = w.scenes[index];
  return cur ? replace(w, index, { ...cur, narration: text }) : w;
}

function replace(w: WorkingPlan, index: number, s: WorkScene): WorkingPlan {
  const scenes = [...w.scenes];
  scenes[index] = s;
  return { ...w, scenes };
}

/** One entry per scene as shown, as PUT /story-plans/:id requires after the operations. */
export function buildWorkingEdits(rec: StoryPlanRecord, w: WorkingPlan): SceneEdit[] {
  return w.scenes.map((s) => {
    const saved = rec.plan?.scenes[s.src];
    const edit: SceneEdit = {};
    if (s.touched || (saved && s.narration.trim() !== saved.narration.trim())) edit.narration = s.narration;
    if (s.pending?.kind === 'set') edit.media = toMediaRef(s.pending.asset);
    else if (s.pending?.kind === 'clear') edit.clearMedia = true;
    return edit;
  });
}

/** The operations to send: none when the structure is back to what is saved. */
export function opsToSend(rec: StoryPlanRecord, w: WorkingPlan): SceneOp[] {
  return structureChanged(w, rec.plan?.scenes.length ?? 0) ? w.ops : [];
}

/** Whether scene `index` has narration or picture changes, or was made by a split or merge. */
export function sceneEdited(edit: SceneEdit | undefined, w: WorkScene | undefined): boolean {
  return !!w?.touched || !!edit && (edit.narration !== undefined || edit.media !== undefined || !!edit.clearMedia);
}
