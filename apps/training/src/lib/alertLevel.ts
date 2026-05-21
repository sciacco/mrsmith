import type { PlanEnrollment } from '../api/types.js';

export type AlertLevel = 'critical' | 'warning' | 'info';

const DAY_MS = 24 * 60 * 60 * 1000;

function startOfDay(date: Date): Date {
  const copy = new Date(date);
  copy.setHours(0, 0, 0, 0);
  return copy;
}

function daysBetween(later: Date, earlier: Date): number {
  return Math.floor((later.getTime() - earlier.getTime()) / DAY_MS);
}

function parseISO(value: string | undefined): Date | null {
  if (!value) return null;
  const stamped = value.length > 10 ? value : `${value}T00:00:00`;
  const date = new Date(stamped);
  return Number.isFinite(date.getTime()) ? startOfDay(date) : null;
}

export interface ClassifyOptions {
  now?: Date;
}

export function classifyAlertLevel(enrollment: PlanEnrollment, options: ClassifyOptions = {}): AlertLevel {
  const now = startOfDay(options.now ?? new Date());
  const status = enrollment.status;
  const plannedStart = parseISO(enrollment.plannedStart);
  const plannedEnd = parseISO(enrollment.plannedEnd);

  if (status === 'expired' || status === 'failed') return 'critical';

  if (status === 'in_progress') {
    if (plannedEnd && daysBetween(now, plannedEnd) > 0) return 'critical';
    if (plannedEnd && daysBetween(plannedEnd, now) <= 30) return 'warning';
    return 'info';
  }

  if (status === 'approved') {
    if (plannedStart && daysBetween(now, plannedStart) > 0) return 'critical';
    if (plannedStart && daysBetween(plannedStart, now) <= 7) return 'warning';
    return 'info';
  }

  if (status === 'proposed') {
    if (enrollment.mandatory) return 'warning';
    return 'info';
  }

  return 'info';
}

export const ALERT_LEVEL_LABEL: Record<AlertLevel, string> = {
  critical: 'Critico',
  warning: 'Attenzione',
  info: 'Info',
};

export const ALERT_LEVEL_SYMBOL: Record<AlertLevel, string> = {
  critical: '●',
  warning: '●',
  info: '○',
};
