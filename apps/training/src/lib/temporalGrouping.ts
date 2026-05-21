import type { PlanEnrollment } from '../api/types.js';

export type TemporalBucket = 'today' | 'this_week' | 'this_month' | 'later';

const DAY_MS = 24 * 60 * 60 * 1000;

function startOfDay(date: Date): Date {
  const copy = new Date(date);
  copy.setHours(0, 0, 0, 0);
  return copy;
}

function pickReferenceDate(enrollment: PlanEnrollment): Date | null {
  const candidate = enrollment.plannedStart ?? enrollment.plannedEnd;
  if (!candidate) return null;
  const stamped = candidate.length > 10 ? candidate : `${candidate}T00:00:00`;
  const date = new Date(stamped);
  return Number.isFinite(date.getTime()) ? startOfDay(date) : null;
}

export function bucketForEnrollment(
  enrollment: PlanEnrollment,
  now: Date = new Date(),
): TemporalBucket {
  const today = startOfDay(now);
  const reference = pickReferenceDate(enrollment);
  if (!reference) return 'later';

  if (reference.getTime() <= today.getTime()) return 'today';

  const diff = Math.floor((reference.getTime() - today.getTime()) / DAY_MS);
  if (diff <= 7) return 'this_week';
  if (diff <= 30) return 'this_month';
  return 'later';
}

export const TEMPORAL_BUCKET_LABEL: Record<TemporalBucket, string> = {
  today: 'Oggi',
  this_week: 'Questa settimana',
  this_month: 'Questo mese',
  later: 'Vedi tutte',
};

export const TEMPORAL_BUCKET_ORDER: TemporalBucket[] = ['today', 'this_week', 'this_month', 'later'];
