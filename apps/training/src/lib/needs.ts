import type { Need, NeedInput, NeedStatus } from '../api/types';

export const NEED_STATES: { value: NeedStatus; label: string }[] = [
  { value: 'new', label: 'Nuova' },
  { value: 'scouting', label: 'Scouting' },
  { value: 'finalizing', label: 'Finalizzare' },
  { value: 'closed', label: 'Chiusa' },
  { value: 'cancelled', label: 'Annullata' },
];

export function needInput(need: Need): NeedInput {
  return {
    description: need.description,
    status: need.status,
    finalCourseId: need.finalCourseId,
    skillAreaIds: need.skillAreas.map((area) => area.id),
    notes: need.notes,
    reminderText: need.reminderText,
    reminderAt: need.reminderAt,
  };
}
