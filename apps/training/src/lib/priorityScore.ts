import type { PlanEnrollment } from '../api/types.js';
import { classifyAlertLevel, type AlertLevel } from './alertLevel.js';

const SEVERITY_WEIGHT: Record<AlertLevel, number> = {
  critical: 1000,
  warning: 200,
  info: 10,
};

const DAY_MS = 24 * 60 * 60 * 1000;

function parseDate(value: string | undefined): Date | null {
  if (!value) return null;
  const stamped = value.length > 10 ? value : `${value}T00:00:00`;
  const date = new Date(stamped);
  return Number.isFinite(date.getTime()) ? date : null;
}

export interface PriorityOptions {
  now?: Date;
}

export function priorityScore(enrollment: PlanEnrollment, options: PriorityOptions = {}): number {
  const now = options.now ?? new Date();
  const level = classifyAlertLevel(enrollment, { now });
  let score = SEVERITY_WEIGHT[level];

  const plannedStart = parseDate(enrollment.plannedStart);
  if (plannedStart) {
    const ageDays = (now.getTime() - plannedStart.getTime()) / DAY_MS;
    score += Math.max(0, ageDays) * 2;
    if (ageDays < 0) {
      const daysUntil = -ageDays;
      score += Math.max(0, 30 - daysUntil);
    }
  }

  if (enrollment.mandatory) score += 50;
  if (enrollment.priority && enrollment.priority > 0) score += (10 - Math.min(enrollment.priority, 10)) * 2;

  return score;
}

export function comparePriorityDesc(
  a: PlanEnrollment,
  b: PlanEnrollment,
  options: PriorityOptions = {},
): number {
  return priorityScore(b, options) - priorityScore(a, options);
}
