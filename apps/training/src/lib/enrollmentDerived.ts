import type { PlanEnrollment } from '../api/types';

export interface EnrollmentDraft {
  priority: string;
  levelAsIs: string;
  levelToBe: string;
  plannedStart: string;
  plannedEnd: string;
  hoursPlanned: string;
  costPlanned: string;
  motivation: string;
  objective: string;
  notes: string;
}

export function costPerHour(cost: string, hours: string): number | undefined {
  const c = Number(cost);
  const h = Number(hours);
  if (!Number.isFinite(c) || !Number.isFinite(h) || h <= 0 || c < 0) return undefined;
  return c / h;
}

function numberDraftEqual(draft: string, value: number | undefined): boolean {
  const trimmed = draft.trim();
  if (trimmed === '') return value === undefined;
  if (value === undefined) return false;
  return Number(trimmed) === value;
}

function dateDraftEqual(draft: string, value: string | undefined): boolean {
  const left = draft ?? '';
  const right = value ? value.slice(0, 10) : '';
  return left === right;
}

function textDraftEqual(draft: string, value: string | undefined): boolean {
  return (draft ?? '') === (value ?? '');
}

export function isDirty(draft: EnrollmentDraft, enrollment: PlanEnrollment): boolean {
  if (!numberDraftEqual(draft.priority, enrollment.priority)) return true;
  if (!numberDraftEqual(draft.levelAsIs, enrollment.levelAsIs)) return true;
  if (!numberDraftEqual(draft.levelToBe, enrollment.levelToBe)) return true;
  if (!numberDraftEqual(draft.hoursPlanned, enrollment.hoursPlanned)) return true;
  if (!numberDraftEqual(draft.costPlanned, enrollment.costPlanned)) return true;
  if (!dateDraftEqual(draft.plannedStart, enrollment.plannedStart)) return true;
  if (!dateDraftEqual(draft.plannedEnd, enrollment.plannedEnd)) return true;
  if (!textDraftEqual(draft.motivation, enrollment.motivation)) return true;
  if (!textDraftEqual(draft.objective, enrollment.objective)) return true;
  if (!textDraftEqual(draft.notes, enrollment.notes)) return true;
  return false;
}

export function formatEuroCompact(value: number): string {
  return new Intl.NumberFormat('it-IT', { maximumFractionDigits: 0 }).format(Math.round(value));
}

export function formatEuro2(value: number): string {
  return new Intl.NumberFormat('it-IT', { minimumFractionDigits: 0, maximumFractionDigits: 2 }).format(value);
}
