import { ProjectStatus } from '../../shared/models/project.model';

/** Plain words for a quiz project's status, shared by the list and the project page. */
export function projectStatusLabel(status: ProjectStatus): string {
  switch (status) {
    case 'created':
      return 'Needs questions';
    case 'parsing':
      return 'Reading questions';
    case 'generating_slides':
      return 'Making slides';
    case 'narrating':
      return 'Narrating';
    case 'rendering':
      return 'Rendering';
    case 'completed':
      return 'Ready';
    case 'failed':
      return 'Failed';
    default:
      return status;
  }
}

/** Colour class of the status pill: ready, failed, waiting on the user, or still working. */
export function projectStatusTone(status: ProjectStatus): string {
  if (status === 'completed') return 't-ok';
  if (status === 'failed') return 't-bad';
  if (status === 'created') return 't-new';
  return 't-wait';
}
