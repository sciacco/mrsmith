import type { MAConfidence } from '../../../../api/types';
import styles from '../Inspector.module.css';

export const dateTimeFormat = new Intl.DateTimeFormat('it-IT', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
});

export const numberFormat = new Intl.NumberFormat('it-IT');

export function formatDateTime(iso?: string): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return dateTimeFormat.format(d);
}

export function formatEuro(value?: number): string {
  if (value == null) return '—';
  return `${numberFormat.format(value)} €`;
}

export function formatCompactEuro(value?: number): string {
  if (value == null) return '—';
  if (value >= 1_000_000) return `${(value / 1_000_000).toLocaleString('it-IT', { maximumFractionDigits: 1 })} M€`;
  if (value >= 1_000) return `${Math.round(value / 1000)} k€`;
  return `${numberFormat.format(value)} €`;
}

export function ragClass(rag?: string): string {
  switch ((rag ?? '').toLowerCase()) {
    case 'green': return styles.ragGreen ?? '';
    case 'amber': return styles.ragAmber ?? '';
    case 'red': return styles.ragRed ?? '';
    default: return styles.ragNa ?? '';
  }
}

export function freshnessClass(f?: string): string {
  switch (f) {
    case 'fresh': return styles.freshFresh ?? '';
    case 'stale': return styles.freshStale ?? '';
    case 'expired': return styles.freshExpired ?? '';
    default: return styles.freshUnknown ?? '';
  }
}

export function freshnessLabel(f?: string): string {
  return f ?? 'n/d';
}

export function confidenceText(c?: MAConfidence | string): string {
  return c ?? '—';
}

export interface DossierTarget {
  initiativeId?: string;
  initiativeTitle?: string;
  companyKey?: string;
}

// Costruisce l'URL del card-dossier (valido solo se la sessione ha initiativeId).
export function dossierHref(t: DossierTarget): string | null {
  if (!t.initiativeId || !t.companyKey) return null;
  return `/iniziative/${t.initiativeId}/dossier/${encodeURIComponent(t.companyKey)}`;
}
