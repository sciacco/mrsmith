import type { ReactNode } from 'react';
import styles from './StatusPill.module.css';

export type StatusTone = 'neutral' | 'accent' | 'success' | 'warning' | 'danger';

interface StatusPillProps {
  tone?: StatusTone;
  children: ReactNode;
  'aria-label'?: string;
}

const toneClass: Record<StatusTone, string | undefined> = {
  neutral: undefined,
  accent: styles.accent,
  success: styles.success,
  warning: styles.warning,
  danger: styles.danger,
};

export function StatusPill({ tone = 'neutral', children, ...rest }: StatusPillProps) {
  const className = [styles.pill, toneClass[tone]].filter(Boolean).join(' ');
  return (
    <span className={className} {...rest}>
      {children}
    </span>
  );
}

const richiestaToneMap: Record<string, StatusTone> = {
  nuova: 'accent',
  bozza: 'accent',
  'in corso': 'warning',
  inviata: 'warning',
  sollecitata: 'warning',
  completata: 'success',
  annullata: 'danger',
};

export function statusTone(value: string | null | undefined): StatusTone {
  if (!value) return 'neutral';
  return richiestaToneMap[value.toLowerCase()] ?? 'neutral';
}
