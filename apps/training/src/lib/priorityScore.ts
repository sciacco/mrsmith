import type { PlanEnrollment } from '../api/types.js';
import { classifyAlertLevel, type AlertLevel } from './alertLevel.js';
import { daysUntilPipelineReference } from './pipelineTiming.js';

const SEVERITY_WEIGHT: Record<AlertLevel, number> = {
  critical: 1000,
  warning: 200,
  info: 10,
};

export interface PriorityOptions {
  now?: Date;
}

export function priorityScore(enrollment: PlanEnrollment, options: PriorityOptions = {}): number {
  const now = options.now ?? new Date();
  const level = classifyAlertLevel(enrollment, { now });
  let score = SEVERITY_WEIGHT[level];

  const daysUntil = daysUntilPipelineReference(enrollment, now);
  if (daysUntil !== null) {
    score += Math.max(0, -daysUntil) * 2;
    if (daysUntil > 0) score += Math.max(0, 30 - daysUntil);
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
