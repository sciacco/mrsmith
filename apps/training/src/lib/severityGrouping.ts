import type { PlanEnrollment } from '../api/types.js';
import { classifyAlertLevel, type AlertLevel } from './alertLevel.js';

export type SeverityBucket = AlertLevel;

export const SEVERITY_BUCKET_ORDER: SeverityBucket[] = ['critical', 'warning', 'info'];

export const SEVERITY_BUCKET_LABEL: Record<SeverityBucket, string> = {
  critical: 'In ritardo',
  warning: 'Imminenti',
  info: 'In coda',
};

export const SEVERITY_BUCKET_DESCRIPTION: Record<SeverityBucket, string> = {
  critical: 'Azioni scadute o in ritardo',
  warning: 'Da gestire nei prossimi giorni',
  info: 'Senza urgenza immediata',
};

export function groupBySeverity(
  enrollments: PlanEnrollment[],
  now: Date = new Date(),
): Map<SeverityBucket, PlanEnrollment[]> {
  const map = new Map<SeverityBucket, PlanEnrollment[]>();
  for (const bucket of SEVERITY_BUCKET_ORDER) map.set(bucket, []);
  for (const enrollment of enrollments) {
    const level = classifyAlertLevel(enrollment, { now });
    map.get(level)!.push(enrollment);
  }
  return map;
}
