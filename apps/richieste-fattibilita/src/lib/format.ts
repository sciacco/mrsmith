import { ApiError } from '@mrsmith/api-client';
import { hasAnyRole } from '@mrsmith/auth-client';
import type { FattibilitaCounts } from '../api/types';

export const MANAGER_ROLES = ['app_rdf_manager'];
export const DEFAULT_LIST_STATES = ['nuova', 'in corso'];
export const RICHIESTA_STATES = ['nuova', 'in corso', 'completata', 'annullata'];
export const FATTIBILITA_STATES = ['bozza', 'inviata', 'sollecitata', 'completata', 'annullata'];
export const BUDGET_OPTIONS = [
  { value: 0, label: 'Non valutato' },
  { value: 1, label: 'Pessima' },
  { value: 2, label: 'Fuori budget' },
  { value: 3, label: 'Nella norma' },
  { value: 4, label: 'Ottima' },
  { value: 5, label: 'Eccezionale' },
] as const;

export function isManager(roles: readonly string[] | undefined): boolean {
  return hasAnyRole(roles, MANAGER_ROLES);
}

export function formatDate(value: string | null | undefined): string {
  if (!value) return 'Non disponibile';
  return new Intl.DateTimeFormat('it-IT', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
  }).format(new Date(value));
}

export function formatDateTime(value: string | null | undefined): string {
  if (!value) return 'Non disponibile';
  return new Intl.DateTimeFormat('it-IT', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));
}

export function formatCounts(counts: FattibilitaCounts): string {
  return `Bozza ${counts.bozza} · Inviate ${counts.inviata} · Sollecitate ${counts.sollecitata} · Completate ${counts.completata} · Annullate ${counts.annullata}`;
}

export function formatCountsBreakdown(counts: FattibilitaCounts): string {
  const parts: string[] = [];
  if (counts.bozza) parts.push(`${counts.bozza} bozza`);
  if (counts.inviata) parts.push(`${counts.inviata} inviate`);
  if (counts.sollecitata) parts.push(`${counts.sollecitata} sollecitate`);
  if (counts.completata) parts.push(`${counts.completata} completate`);
  if (counts.annullata) parts.push(`${counts.annullata} annullate`);
  return parts.length ? parts.join(' · ') : 'Nessuna RDF';
}

export function stripCompanyPrefix(
  dealName: string | null | undefined,
  companyName: string | null | undefined,
): string {
  if (!dealName) return '';
  if (!companyName) return dealName;
  for (const sep of [' – ', ' - ']) {
    const prefix = `${companyName}${sep}`;
    if (dealName.startsWith(prefix)) return dealName.slice(prefix.length);
  }
  return dealName;
}

export function compactAddress(address: string | null | undefined): string {
  if (!address) return '—';
  const match = address.match(/([A-Za-zÀ-ÿ'\- ]+?)\s*\(([A-Z]{2})\)/);
  const city = match?.[1];
  const province = match?.[2];
  if (city && province) return `${city.trim()} (${province})`;
  return address.length > 48 ? `${address.slice(0, 48).trimEnd()}…` : address;
}

export function budgetLabel(score: number): string {
  return BUDGET_OPTIONS.find((item) => item.value === score)?.label ?? 'Non valutato';
}

function isTechnicalErrorCopy(value: string | undefined): boolean {
  if (!value) return false;
  return /\b(unauthorized|forbidden|bad gateway|gateway timeout|internal server error|failed to fetch|network(?:error| error)|timeout|json|http|fetch)\b/i.test(value);
}

export function copyErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body as { error?: string; message?: string } | undefined;
    const candidate = body?.message ?? body?.error;
    if (candidate && !isTechnicalErrorCopy(candidate)) return candidate;
    if (error.status === 401 || error.status === 403) return 'Non hai accesso a questa operazione.';
    if (error.status >= 500 || isTechnicalErrorCopy(error.statusText)) return fallback;
    return error.statusText || fallback;
  }
  if (error instanceof Error) {
    return isTechnicalErrorCopy(error.message) ? fallback : error.message;
  }
  return fallback;
}

export function parsePositiveId(value: string | undefined): number | null {
  if (!value) return null;
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) return null;
  return parsed;
}
