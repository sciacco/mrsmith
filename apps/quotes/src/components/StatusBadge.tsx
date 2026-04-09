import type { QuoteStatus } from '../api/types';
import styles from './StatusBadge.module.css';

const statusConfig: Record<QuoteStatus, { label: string; className: string }> = {
  DRAFT: { label: 'Bozza', className: styles.draft ?? '' },
  PENDING_APPROVAL: { label: 'In approvazione', className: styles.pendingApproval ?? '' },
  APPROVED: { label: 'Approvata', className: styles.approved ?? '' },
  APPROVAL_NOT_NEEDED: { label: 'Pronta', className: styles.approvalNotNeeded ?? '' },
  ESIGN_COMPLETED: { label: 'Firmata', className: styles.esignCompleted ?? '' },
};

export function StatusBadge({ status }: { status: QuoteStatus }) {
  const config = statusConfig[status] ?? statusConfig.DRAFT;
  return (
    <span className={`${styles.badge ?? ''} ${config.className}`}>
      {config.label}
    </span>
  );
}
