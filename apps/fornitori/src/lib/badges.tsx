import { StatusBadge, type StatusBadgeVariant } from '@mrsmith/ui';
import { stateLabel } from './reference';

const PROVIDER_STATE_VARIANT: Record<string, StatusBadgeVariant> = {
  DRAFT: 'neutral',
  ACTIVE: 'success',
  INACTIVE: 'neutral',
  CEASED: 'neutral',
};

const CATEGORY_STATE_VARIANT: Record<string, StatusBadgeVariant> = {
  NEW: 'neutral',
  NOT_QUALIFIED: 'warning',
  QUALIFIED: 'success',
};

const DOCUMENT_STATE_VARIANT: Record<string, StatusBadgeVariant> = {
  EXPIRED: 'danger',
  PENDING_VERIFY_ALL: 'warning',
  PENDING_VERIFY_DATE: 'warning',
  PENDING_VERIFY_DOC: 'warning',
};

function normalize(value?: string | null) {
  return (value ?? '').toUpperCase().replace(/[-\s]+/g, '_');
}

export function ProviderStateBadge({ state }: { state?: string | null }) {
  const key = normalize(state);
  if (!key) return null;
  const variant = PROVIDER_STATE_VARIANT[key] ?? 'neutral';
  return <StatusBadge value={key} label={stateLabel(state)} variant={variant} />;
}

export function CategoryStateBadge({ state }: { state?: string | null }) {
  const key = normalize(state);
  if (!key) return null;
  const variant = CATEGORY_STATE_VARIANT[key] ?? 'neutral';
  return <StatusBadge value={key} label={stateLabel(state)} variant={variant} />;
}

/**
 * Document state. Returns null for OK (la norma, non emette badge).
 */
export function DocumentStateBadge({ state }: { state?: string | null }) {
  const key = normalize(state);
  if (!key || key === 'OK') return null;
  const variant = DOCUMENT_STATE_VARIANT[key] ?? 'neutral';
  return <StatusBadge value={key} label={stateLabel(state)} variant={variant} />;
}

/**
 * Calcola giorni residui rispetto a oggi (mezzanotte locale).
 * Negativo se già scaduto, 0 se scade oggi.
 */
export function daysUntilExpiry(expireDate?: string | null): number | null {
  if (!expireDate) return null;
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const target = new Date(expireDate);
  if (Number.isNaN(target.getTime())) return null;
  target.setHours(0, 0, 0, 0);
  return Math.round((target.getTime() - today.getTime()) / 86_400_000);
}

function urgencyCopy(days: number): string {
  if (days < 0) return `Scaduto da ${Math.abs(days)}gg`;
  if (days === 0) return 'Scade oggi';
  return `Scade tra ${days}gg`;
}

/**
 * Cromia urgenza scadenza, derivata dalla data e indipendente dallo stato.
 * Ritorna null se data assente o oltre 30 giorni.
 */
export function DocumentUrgencyBadge({ expireDate, days }: { expireDate?: string | null; days?: number | null }) {
  const computed = days ?? daysUntilExpiry(expireDate);
  if (computed === null || computed === undefined) return null;
  if (computed > 30) return null;
  const variant: StatusBadgeVariant = computed <= 7 ? 'danger' : 'warning';
  return <StatusBadge value="urgency" label={urgencyCopy(computed)} variant={variant} />;
}
