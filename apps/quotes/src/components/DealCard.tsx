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

function formatDate(dateStr: string | null | undefined): string {
  if (!dateStr) return '—';
  const d = new Date(dateStr);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleDateString('it-IT', { day: '2-digit', month: '2-digit', year: 'numeric' });
}

function dealTypeLabel(dealType: string | null | undefined): string | null {
  if (!dealType) return null;
  const normalized = dealType.trim().toLowerCase();
  if (normalized === 'newbusiness') return 'New';
  if (normalized === 'existingbusiness') return 'Existing';
  return null;
}

function ownerLabel(firstName: string | null | undefined, lastName: string | null | undefined): string | null {
  const parts = [firstName?.trim(), lastName?.trim()].filter((part): part is string => Boolean(part));
  if (parts.length === 0) return null;
  return parts.join(' ');
}

export function DealCard({ deal, selected, onClick }: DealCardProps) {
  const company = deal.company_name ?? '—';
  const dealType = dealTypeLabel(deal.dealtype);
  const owner = ownerLabel(deal.owner_firstname, deal.owner_lastname);
  const createdAt = formatDate(deal.created_at);
  const updatedAt = formatDate(deal.updated_at);
  const stageTone = deal.dealstage ? stageColor(deal.dealstage) : null;

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
        <span className={styles.meta}>
          {dealType && <span className={styles.metaType}>{dealType}</span>}
          {dealType && <span aria-hidden="true">•</span>}
          {owner && <span className={styles.metaOwner}>{owner}</span>}
          {owner && <span aria-hidden="true">•</span>}
          <span>Creata {createdAt}</span>
          <span aria-hidden="true">•</span>
          <span>Mod. {updatedAt}</span>
        </span>
      </div>
      <div className={styles.right}>
        {deal.dealstage && stageTone && (
          <span
            className={styles.stage}
            style={{ color: stageTone.color, background: stageTone.bg }}
          >
            {deal.dealstage}
          </span>
        )}
        {deal.pipeline && <span className={styles.pipeline}>{deal.pipeline}</span>}
      </div>
    </button>
  );
}
