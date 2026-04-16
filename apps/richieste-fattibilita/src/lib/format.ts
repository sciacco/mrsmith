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
  return `Bozza: ${counts.bozza} Inv: ${counts.inviata} Soll: ${counts.sollecitata} Comp: ${counts.completata} Ann: ${counts.annullata}`;
}

export function budgetLabel(score: number): string {
  return BUDGET_OPTIONS.find((item) => item.value === score)?.label ?? 'Non valutato';
}

export function copyErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body as { error?: string } | undefined;
    return body?.error ?? error.statusText ?? fallback;
  }
  if (error instanceof Error) return error.message;
  return fallback;
}

export function parsePositiveId(value: string | undefined): number | null {
  if (!value) return null;
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) return null;
  return parsed;
}
