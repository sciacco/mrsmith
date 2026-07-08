import type { MAConfidence } from '../../../../api/types';
import { deepRagClassName, formatDeepCompactEuro } from '../../../../components/deep/DeepComponents';
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
  return formatDeepCompactEuro(value);
}

export function ragClass(rag?: string): string {
  return deepRagClassName(rag);
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

// Costruisce l'URL della Scheda azienda con lente iniziativa (il card-dossier
// è stato assorbito dalla scheda, FUSIONE F5).
export function dossierHref(t: DossierTarget): string | null {
  if (!t.initiativeId || !t.companyKey) return null;
  return `/aziende/${encodeURIComponent(t.companyKey)}?iniziativa=${encodeURIComponent(t.initiativeId)}`;
}
