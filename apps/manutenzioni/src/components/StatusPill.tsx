import styles from './StatusPill.module.css';

export function statusTone(status: string): 'neutral' | 'info' | 'success' | 'warning' | 'danger' {
  switch (status) {
    case 'scheduled':
      return 'info';
    case 'announced':
    case 'in_progress':
      return 'warning';
    case 'completed':
    case 'ready':
    case 'sent':
      return 'success';
    case 'cancelled':
    case 'failed':
      return 'danger';
    default:
      return 'neutral';
  }
}

interface StatusPillProps {
  children: string;
  tone?: 'neutral' | 'info' | 'success' | 'warning' | 'danger';
}

export function StatusPill({ children, tone = 'neutral' }: StatusPillProps) {
  return <span className={`${styles.pill} ${styles[tone]}`}>{children}</span>;
}
