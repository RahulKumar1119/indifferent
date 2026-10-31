import {
  MediaRef,
  SceneEdit,
  StockAsset,
  StoryPlanRecord,
  mediaKey,
  toMediaRef,
} from './story-plan.models';

/** A picture change the user has made but not saved yet. */
export type PendingMedia = { kind: 'set'; asset: StockAsset } | { kind: 'clear' };

/** What one scene's picture block shows. */
export type SceneMediaView =
  | { state: 'none' }
  | { state: 'asset'; asset: StockAsset; changed: boolean }
  /** The provider no longer has it. Nothing to retry; choose another. */
  | { state: 'unavailable'; ref: MediaRef }
  /** A temporary failure looking it up. */
  | { state: 'error'; ref: MediaRef };

/** The picture a scene shows now: a pending change if there is one, else what is saved. */
export function sceneMediaView(
  record: StoryPlanRecord,
  index: number,
  pending: PendingMedia | undefined,
): SceneMediaView {
  if (pending?.kind === 'set') return { state: 'asset', asset: pending.asset, changed: true };
  if (pending?.kind === 'clear') return { state: 'none' };

  const ref = record.plan?.scenes[index]?.media;
  if (!ref) return { state: 'none' };
  const item = record.media?.[index];
  if (item?.asset) return { state: 'asset', asset: item.asset, changed: false };
  if (item?.status === 'unavailable') return { state: 'unavailable', ref };
  // A saved pick the API did not describe (lookups are off, or it failed).
  return { state: 'error', ref };
}

/** The reference the scene points at now, for marking the current choice in the picker. */
export function currentRef(record: StoryPlanRecord, index: number, pending: PendingMedia | undefined): MediaRef | null {
  if (pending?.kind === 'set') return toMediaRef(pending.asset);
  if (pending?.kind === 'clear') return null;
  return record.plan?.scenes[index]?.media ?? null;
}

/**
 * Records a chosen asset for a scene. Choosing what is already saved undoes
 * any pending change instead of counting as an edit.
 */
export function withPick(
  record: StoryPlanRecord,
  pending: (PendingMedia | undefined)[],
  index: number,
  asset: StockAsset,
): (PendingMedia | undefined)[] {
  const next = [...pending];
  const saved = record.plan?.scenes[index]?.media;
  next[index] = saved && mediaKey(saved) === mediaKey(asset) ? undefined : { kind: 'set', asset };
  return next;
}

/** Removes a scene's picture. Removing one that was never saved just drops the pending choice. */
export function withRemoval(
  record: StoryPlanRecord,
  pending: (PendingMedia | undefined)[],
  index: number,
): (PendingMedia | undefined)[] {
  const next = [...pending];
  next[index] = record.plan?.scenes[index]?.media ? { kind: 'clear' } : undefined;
  return next;
}

/** One entry per scene, as PUT /story-plans/:id requires. Unchanged scenes are `{}`. */
export function buildEdits(
  record: StoryPlanRecord,
  narrations: string[],
  pending: (PendingMedia | undefined)[],
): SceneEdit[] {
  const scenes = record.plan?.scenes ?? [];
  return scenes.map((scene, i) => {
    const edit: SceneEdit = {};
    if (narrations[i] !== undefined && narrations[i].trim() !== scene.narration.trim()) {
      edit.narration = narrations[i];
    }
    const p = pending[i];
    if (p?.kind === 'set') edit.media = toMediaRef(p.asset);
    else if (p?.kind === 'clear') edit.clearMedia = true;
    return edit;
  });
}

/** How many scenes have a change waiting to be saved. */
export function changedSceneCount(edits: SceneEdit[]): number {
  return edits.filter((e) => e.narration !== undefined || e.media !== undefined || e.clearMedia).length;
}
