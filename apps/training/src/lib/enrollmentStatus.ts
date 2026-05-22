export type EnrollmentStatus =
  | 'proposed'
  | 'approved'
  | 'in_progress'
  | 'completed'
  | 'failed'
  | 'cancelled'
  | 'expired';

export type EnrollmentStatusTone =
  | 'proposed'
  | 'approved'
  | 'in_progress'
  | 'completed'
  | 'failed'
  | 'cancelled'
  | 'expired';

export const ENROLLMENT_STATUS_LABEL: Record<EnrollmentStatus, string> = {
  proposed: 'Proposta',
  approved: 'Approvata',
  in_progress: 'In corso',
  completed: 'Completata',
  failed: 'Non superata',
  cancelled: 'Annullata',
  expired: 'Scaduta',
};

export const ENROLLMENT_STATUS_TONE: Record<EnrollmentStatus, EnrollmentStatusTone> = {
  proposed: 'proposed',
  approved: 'approved',
  in_progress: 'in_progress',
  completed: 'completed',
  failed: 'failed',
  cancelled: 'cancelled',
  expired: 'expired',
};

const ACTIVE_STATUSES: ReadonlySet<EnrollmentStatus> = new Set(['proposed', 'approved', 'in_progress']);

export function isEnrollmentStatus(value: string): value is EnrollmentStatus {
  return value in ENROLLMENT_STATUS_LABEL;
}

export function isActiveEnrollmentStatus(value: string): value is EnrollmentStatus {
  return isEnrollmentStatus(value) && ACTIVE_STATUSES.has(value);
}

export function enrollmentStatusLabel(value: string): string {
  return isEnrollmentStatus(value) ? ENROLLMENT_STATUS_LABEL[value] : value;
}

export function enrollmentStatusTone(value: string): EnrollmentStatusTone | null {
  return isEnrollmentStatus(value) ? ENROLLMENT_STATUS_TONE[value] : null;
}
