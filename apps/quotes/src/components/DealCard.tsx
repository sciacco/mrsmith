import type { Deal } from '../api/types';
import styles from './DealCard.module.css';

interface DealCardProps {
  deal: Deal;
  selected: boolean;
  onClick: () => void;
}

export function DealCard({ deal, selected, onClick }: DealCardProps) {
  return (
    <div className={`${styles.card} ${selected ? styles.selected : ''}`} onClick={onClick}>
      <div className={styles.dealName}>{deal.name}</div>
      <div className={styles.company}>{deal.company_name ?? '—'}</div>
    </div>
  );
}
