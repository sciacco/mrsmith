import type { ReactNode } from 'react';
import { Tooltip } from '../Tooltip/Tooltip';
import styles from './StatusBadge.module.css';

export type StatusBadgeVariant = 'success' | 'warning' | 'danger' | 'accent' | 'neutral';

interface StatusBadgeProps {
  value: string | null | undefined;
  label?: ReactNode;
  variant?: StatusBadgeVariant;
  dot?: boolean;
  tooltip?: ReactNode;
  className?: string;
}

const DEFAULT_MAP: Record<string, StatusBadgeVariant> = {
  ATTIVO: 'success',
  INVIATO: 'accent',
  ANNULLATO: 'danger',
  EVASO: 'neutral',
  CHIUSO: 'neutral',
  BOZZA: 'warning',
};

const DEFAULT_TOOLTIP: Record<string, string> = {
  ATTIVO: 'Ordine attivo in produzione',
  INVIATO: 'Inviato, in attesa di evasione',
  ANNULLATO: 'Ordine annullato',
  EVASO: 'Ordine completato',
  CHIUSO: 'Ordine chiuso',
  BOZZA: 'Ordine in preparazione',
};

export function StatusBadge({
  value,
  label,
  variant,
  dot = true,
  tooltip,
  className,
}: StatusBadgeProps) {
  const text = value ?? '';
  const upper = text.toUpperCase();
  const resolved = variant ?? DEFAULT_MAP[upper] ?? 'neutral';
  const tip = tooltip ?? DEFAULT_TOOLTIP[upper];

  const badge = (
    <span className={`${styles.badge} ${styles[resolved]} ${className ?? ''}`} tabIndex={tip ? 0 : undefined}>
      {dot && <span className={styles.dot} aria-hidden="true" />}
      <span className={styles.label}>{label ?? text}</span>
    </span>
  );

  if (tip) {
    return <Tooltip content={tip} placement="top">{badge}</Tooltip>;
  }
  return badge;
}
