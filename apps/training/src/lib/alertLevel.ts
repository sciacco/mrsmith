import type { PlanEnrollment } from '../api/types.js';
import { daysUntilPipelineReference } from './pipelineTiming.js';

export type AlertLevel = 'critical' | 'warning' | 'info';

export interface ClassifyOptions {
  now?: Date;
}

export function classifyAlertLevel(enrollment: PlanEnrollment, options: ClassifyOptions = {}): AlertLevel {
  const status = enrollment.status;
  const daysUntil = daysUntilPipelineReference(enrollment, options.now ?? new Date());

  if (status === 'expired' || status === 'failed') return 'critical';

  if (status === 'in_progress') {
    if (daysUntil !== null && daysUntil < 0) return 'critical';
    if (daysUntil !== null && daysUntil <= 30) return 'warning';
    return 'info';
  }

  if (status === 'approved') {
    if (daysUntil !== null && daysUntil < 0) return 'critical';
    if (daysUntil !== null && daysUntil <= 7) return 'warning';
    return 'info';
  }

  if (status === 'proposed') {
    if (daysUntil !== null && daysUntil < 0) return 'critical';
    if (daysUntil !== null && daysUntil <= 7) return 'warning';
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
