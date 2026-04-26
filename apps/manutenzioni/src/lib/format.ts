import { ApiError } from '@mrsmith/api-client';
import type { StatusCount } from '../api/types';
export { severityLabel } from './severity';

export const STATUS_OPTIONS = [
  'draft',
  'approved',
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
  const labels: Record<string, string> = {
    draft: 'Bozza',
    approved: 'Approvata',
    scheduled: 'Pianificata',
    announced: 'Annunciata',
    in_progress: 'In corso',
    completed: 'Completata',
    cancelled: 'Annullata',
    superseded: 'Superata',
  };
  return labels[status] ?? status;
}

export function windowStatusLabel(status: string): string {
  const labels: Record<string, string> = {
    planned: 'Pianificata',
    cancelled: 'Annullata',
    superseded: 'Sostituita',
    executed: 'Eseguita',
  };
  return labels[status] ?? status;
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
  const labels: Record<string, string> = {
    announcement: 'Annuncio',
    reminder: 'Promemoria',
    reschedule: 'Riprogrammazione',
    cancellation: 'Annullamento',
    start: 'Avvio',
    completion: 'Completamento',
    internal_update: 'Aggiornamento interno',
  };
  return labels[value] ?? value;
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
      const known: Record<string, string> = {
        manutenzioni_database_not_configured: 'Registro non configurato.',
        customer_lookup_not_configured: 'Ricerca clienti non disponibile.',
        maintenance_not_found: 'Manutenzione non trovata.',
        status_transition_not_allowed: 'Cambio stato non consentito.',
        customer_scope_required: "Definisci l'ambito clienti prima di continuare.",
        invalid_customer_scope: 'Ambito clienti non valido.',
        maintenance_window_required: 'Aggiungi una finestra prima di continuare.',
        invalid_window_range: 'La fine della finestra deve essere successiva all’inizio.',
        invalid_window: 'Verifica i dati della finestra.',
        notice_content_required: 'Completa i testi richiesti prima di cambiare stato.',
        sent_at_required: 'Indica la data di invio.',
        assistance_not_configured: 'Assistenza non disponibile. Puoi completare la bozza manualmente.',
        assistance_generation_failed: 'Assistenza non riuscita. Riprova o completa la bozza manualmente.',
        invalid_llm_model_scope: 'Ambito non valido.',
        llm_model_model_required: 'Indica il modello.',
        llm_model_already_exists: 'Esiste già un modello con questo ambito.',
        llm_model_scope_immutable: "L'ambito non può essere modificato.",
        llm_model_not_found: 'Modello non trovato.',
      };
      return known[code] ?? fallback;
    }
    if (error.status === 403) return 'Non hai i permessi per questa azione.';
    if (error.status === 503) return 'Servizio non disponibile.';
  }
  return fallback;
}
