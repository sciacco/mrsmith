import type { Deal } from '../api/types';
import styles from './DealCard.module.css';

interface DealCardProps {
  deal: Deal;
  selected: boolean;
  onClick: () => void;
}

function initialsFromCompany(name: string | null | undefined): string {
  if (!name) return '?';
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return '?';
  if (parts.length === 1) return parts[0]!.slice(0, 2).toUpperCase();
  return (parts[0]![0]! + parts[1]![0]!).toUpperCase();
}

export function DealCard({ deal, selected, onClick }: DealCardProps) {
  const company = deal.company_name ?? '—';
  return (
    <button
      type="button"
      className={`${styles.card} ${selected ? styles.selected : ''}`}
      onClick={onClick}
      aria-pressed={selected}
    >
      <span className={styles.accent} aria-hidden="true" />
      <span className={styles.code}>{deal.name}</span>
      <div className={styles.center}>
        <span className={styles.company}>{company}</span>
        {deal.dealstage && <span className={styles.stage}>{deal.dealstage}</span>}
      </div>
      <div className={styles.right}>
        <span className={styles.avatar} aria-hidden="true">
          {initialsFromCompany(deal.company_name)}
        </span>
        {deal.pipeline && <span className={styles.pipeline}>{deal.pipeline}</span>}
      </div>
    </button>
  );
}
