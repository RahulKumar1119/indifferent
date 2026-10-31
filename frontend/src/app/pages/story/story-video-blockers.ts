import { StoryPlanRecord } from './story-plan.models';

/** "Scene 3", "Scenes 1 and 3", "Scenes 1, 2 and 4". */
function sceneList(numbers: number[]): string {
  if (numbers.length === 1) return `Scene ${numbers[0]}`;
  const head = numbers.slice(0, -1).join(', ');
  return `Scenes ${head} and ${numbers[numbers.length - 1]}`;
}

/**
 * Why a video cannot be made from this plan right now, in words that say what
 * to do; empty when it can. The video is made from the plan as saved, so unsaved
 * edits come first. The same checks the API makes, ahead of time, so a person
 * is told before a render starts instead of after it fails.
 */
export function videoBlocker(record: StoryPlanRecord, hasUnsavedChanges: boolean): string {
  if (hasUnsavedChanges) {
    return 'Save your changes first. The video is made from the saved plan, not from what is on screen.';
  }
  const scenes = record.plan?.scenes ?? [];
  const without = scenes.flatMap((s, i) => (s.media ? [] : [i + 1]));
  if (without.length > 0) {
    const verb = without.length === 1 ? 'has' : 'have';
    const each = without.length === 1 ? 'one' : 'one for each';
    return `${sceneList(without)} ${verb} no picture. Choose ${each}, then save.`;
  }
  const gone = scenes.flatMap((s, i) => (s.media && record.media?.[i]?.status === 'unavailable' ? [i + 1] : []));
  if (gone.length > 0) {
    const one = gone.length === 1;
    return `The ${one ? 'picture' : 'pictures'} for ${sceneList(gone).toLowerCase()} can't be used any more: the photo library no longer has ${one ? 'it' : 'them'}. Choose another, then save.`;
  }
  return '';
}
