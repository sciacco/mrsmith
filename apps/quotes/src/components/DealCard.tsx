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

const stageColors: [string, string][] = [
  ['#635bff', 'rgba(99,91,255,0.10)'],   // indigo
  ['#10b981', 'rgba(16,185,129,0.10)'],   // green
  ['#f59e0b', 'rgba(245,158,11,0.12)'],   // amber
  ['#3b82f6', 'rgba(59,130,246,0.10)'],   // blue
  ['#ec4899', 'rgba(236,72,153,0.10)'],   // pink
  ['#8b5cf6', 'rgba(139,92,246,0.10)'],   // violet
  ['#06b6d4', 'rgba(6,182,212,0.10)'],    // cyan
];

function stageColor(label: string): { color: string; bg: string } {
  let h = 0;
  for (let i = 0; i < label.length; i++) h = ((h << 5) - h + label.charCodeAt(i)) | 0;
  const idx = ((h % stageColors.length) + stageColors.length) % stageColors.length;
  return { color: stageColors[idx]![0], bg: stageColors[idx]![1] };
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
      <span className={styles.avatar} aria-hidden="true">
        {initialsFromCompany(deal.company_name)}
      </span>
      <div className={styles.center}>
        <span className={styles.dealName}>{deal.name}</span>
        <span className={styles.company}>{company}</span>
      </div>
      <div className={styles.right}>
        {deal.dealstage && (
          <span
            className={styles.stage}
            style={{ color: stageColor(deal.dealstage).color, background: stageColor(deal.dealstage).bg }}
          >
            {deal.dealstage}
          </span>
        )}
        {deal.pipeline && <span className={styles.pipeline}>{deal.pipeline}</span>}
      </div>
    </button>
  );
}
