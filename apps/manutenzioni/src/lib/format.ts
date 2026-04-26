import { ApiError } from '@mrsmith/api-client';
import type { StatusCount } from '../api/types';
import {
  API_ERROR_MESSAGES,
  NOTICE_TYPE_LABELS,
  STATUS_LABELS,
  WINDOW_STATUS_LABELS,
} from './labels';
export { severityLabel } from './severity';

export const STATUS_OPTIONS = [
  'draft',
  'scheduled',
  'announced',
  'in_progress',
  'completed',
  'cancelled',
  'superseded',
] as const;

export function parsePositiveId(raw: string | undefined): number | null {
  if (!raw) return null;
  const id = Number(raw);
  return Number.isInteger(id) && id > 0 ? id : null;
}

export function statusLabel(status: string): string {
  return STATUS_LABELS[status] ?? status;
}

export function windowStatusLabel(status: string): string {
  return WINDOW_STATUS_LABELS[status] ?? status;
}

export function sourceLabel(source: string): string {
  const labels: Record<string, string> = {
    manual: 'Manuale',
    import: 'Importazione',
    rule: 'Regola',
    ai_extracted: 'AI',
    ai: 'AI',
    catalog_mapping: 'Catalogo',
    dependency_graph: 'Grafo servizi',
    hybrid: 'Ibrido',
  };
  return labels[source] ?? source;
}

export function serviceRoleLabel(role?: string | null): string {
  const labels: Record<string, string> = {
    operated: 'Operato',
    dependent: 'Impattato',
  };
  return role ? labels[role] ?? role : '-';
}

export function dependencyTypeLabel(value?: string | null): string {
  const labels: Record<string, string> = {
    runs_on: 'Ospita',
    connects_through: 'Transita da',
    consumes: 'Consuma',
    depends_on: 'Dipende da',
  };
  return value ? labels[value] ?? value : '-';
}

export function impactScopeLabel(scope: string): string {
  const labels: Record<string, string> = {
    direct: 'Diretto',
    indirect: 'Indiretto',
    possible: 'Possibile',
  };
  return labels[scope] ?? scope;
}

export function noticeTypeLabel(value: string): string {
  return NOTICE_TYPE_LABELS[value] ?? value;
}

export function audienceLabel(value: string): string {
  const labels: Record<string, string> = {
    internal: 'Interna',
    external: 'Esterna',
    both: 'Interna ed esterna',
    maintenance: 'Da valutare',
  };
  return labels[value] ?? value;
}

export function sendStatusLabel(value: string): string {
  const labels: Record<string, string> = {
    draft: 'Bozza',
    ready: 'Pronta',
    sent: 'Inviata',
    failed: 'Non riuscita',
    suppressed: 'Sospesa',
  };
  return labels[value] ?? value;
}

export function formatDateTime(value?: string | null): string {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  return new Intl.DateTimeFormat('it-IT', {
    dateStyle: 'short',
    timeStyle: 'short',
  }).format(date);
}

export function formatDateInput(value?: string | null): string {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const pad = (v: number) => String(v).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function minutesLabel(value?: number | null): string {
  if (value === null || value === undefined) return '-';
  return `${value} min`;
}

export function confidenceLabel(value?: number | null): string {
  if (value === null || value === undefined) return '-';
  return `${Math.round(value * 100)}%`;
}

export function noticesSummary(counts: StatusCount[]): string {
  if (!counts.length) return '-';
  return counts.map((item) => `${sendStatusLabel(item.status)} ${item.count}`).join(', ');
}

export function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body;
    if (typeof body === 'object' && body !== null && 'error' in body) {
      const code = String((body as { error: unknown }).error);
      return API_ERROR_MESSAGES[code] ?? fallback;
    }
    if (error.status === 403) return 'Non hai i permessi per questa azione.';
    if (error.status === 503) return 'Servizio non disponibile.';
  }
  return fallback;
}
