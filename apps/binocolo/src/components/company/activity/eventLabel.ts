import { formatLocalDate } from '@mrsmith/format';
import type { MATargetOutcome } from '../../../api/types';
import { esitoLabel, stateLabel } from '../../../lib/cardStates';

const AGREEMENT_KIND_LABELS: Record<string, string> = { nda: 'NDA' };
const AGREEMENT_FIELD_LABELS: Record<string, string> = { kind: 'Tipo', signedOn: 'Sottoscrizione', expiresOn: 'Scadenza' };

const dateLabel = (value?: string | null) => formatLocalDate(value) ?? '';

/** Valore di una data civile di payload: `YYYY-MM-DD` formattata senza conversioni
 *  di fuso (mai `new Date("YYYY-MM-DD")`); null/vuoto restano null. */
function visitDateValue(value: unknown): string | null {
  if (typeof value !== 'string' || value === '') return null;
  return formatLocalDate(value) ?? value;
}

/** Dettaglio leggibile della transizione di `visit_on` nel diario (issue #201).
 *  I campi compaiono solo quando la data è cambiata; un null esplicito racconta
 *  l'azzeramento. Nei `stato` la permanenza in `visita` (`to`) distingue la
 *  cancellazione manuale dall'azzeramento automatico in uscita; per chiusura e
 *  rimozione la data è sempre azzerata dall'uscita. `null` quando l'evento non
 *  porta i campi (payload storici: resa invariata). */
function visitDateDetail(payload: Record<string, unknown>): string | null {
  if (!('visitOnFrom' in payload) && !('visitOnTo' in payload)) return null;
  const from = visitDateValue(payload.visitOnFrom);
  const to = visitDateValue(payload.visitOnTo);
  if (to) return from ? `Data visita: ${from} → ${to}` : `Data visita: ${to}`;
  if (from) {
    return payload.to === 'visita' ? `Data visita: ${from} → cancellata` : `Data visita azzerata (era ${from})`;
  }
  return 'Data visita aggiornata';
}

function agreementKindLabel(kind: unknown): string {
  return typeof kind === 'string' ? AGREEMENT_KIND_LABELS[kind] ?? kind : 'Accordo';
}

function agreementFieldValueLabel(field: unknown, value: unknown): string {
  if (field === 'kind') return agreementKindLabel(value);
  if (value === null || value === undefined) return field === 'expiresOn' ? 'Senza scadenza' : '—';
  if (typeof value !== 'string') return '—';
  const formatted = formatLocalDate(value);
  if (formatted) return formatted;
  return value !== '' ? value : '—';
}

/** Human label for persisted activity payloads. Historical payload keys are
 * resolved at presentation time and are never rewritten. */
export function eventLabel(event: MATargetOutcome, sessionMap: Map<string, string> = new Map()): string {
  const payload = event.payload as Record<string, unknown> | undefined;
  switch (event.event) {
    case 'stato': {
      const from = payload && typeof payload.from === 'string' ? stateLabel(payload.from) : null;
      const to = payload && typeof payload.to === 'string' ? stateLabel(payload.to) : null;
      const base = from && to ? `${from} → ${to}` : `Stato aggiornato${event.note ? ` — ${event.note}` : ''}`;
      const detail = payload ? visitDateDetail(payload) : null;
      return detail ? `${base} · ${detail}` : base;
    }
    case 'chiusura': {
      const state = payload && typeof payload.stato === 'string' ? stateLabel(payload.stato) : 'Chiusura';
      const outcome = payload && typeof payload.esito === 'string' && payload.esito ? ` — ${esitoLabel(payload.esito)}` : '';
      const base = event.note ? `${state}${outcome}: ${event.note}` : `${state}${outcome}`;
      const detail = payload ? visitDateDetail(payload) : null;
      return detail ? `${base} · ${detail}` : base;
    }
    case 'card_creata': {
      const rating = payload && typeof payload.rating === 'number' ? payload.rating : 0;
      const stars = rating > 0 ? ` ${'★'.repeat(Math.min(rating, 3))}` : '';
      const sessionId = payload && typeof payload.sessionId === 'string' ? payload.sessionId : null;
      const title = sessionId ? sessionMap.get(sessionId) ?? null : null;
      return `Aggiunta${title ? ` da ${title}` : ''}${stars}${event.note ? ` — ${event.note}` : ''}`;
    }
    case 'card_riaperta': return 'Rimessa in lavorazione';
    case 'card_rimossa': {
      const detail = payload ? visitDateDetail(payload) : null;
      return detail ? `Rimossa dalla lavorazione · ${detail}` : 'Rimossa dalla lavorazione';
    }
    case 'dominio_verificato': {
      const outcome = payload && typeof payload.esito === 'string' ? payload.esito : 'non_verificabile';
      if (outcome === 'confermato') return 'Verifica automatica dominio: confermata';
      if (outcome === 'non_confermato') return 'Verifica automatica dominio: non confermata';
      return 'Verifica automatica dominio: non disponibile';
    }
    case 'contattato': return event.note ? `Contattata — ${event.note}` : 'Contattata';
    case 'buon_lead': return event.note ? `Buon lead — ${event.note}` : 'Buon lead';
    case 'no_go': return event.note ? `No-go — ${event.note}` : 'No-go';
    case 'accordo': {
      const action = payload && typeof payload.action === 'string' ? payload.action : '';
      const kind = agreementKindLabel(payload?.kind);
      const signedOn = payload && typeof payload.signedOn === 'string' ? dateLabel(payload.signedOn) : '';
      const signedOnFragment = signedOn ? ` il ${signedOn}` : '';
      if (action === 'created') {
        const expiresOn = payload?.expiresOn;
        const expiryText = expiresOn && typeof expiresOn === 'string' ? `scade il ${dateLabel(expiresOn)}` : 'senza scadenza';
        return `${kind} sottoscritto${signedOnFragment} · ${expiryText}`;
      }
      if (action === 'updated') {
        const changes = Array.isArray(payload?.changes) ? payload.changes : [];
        const validChanges = changes.filter((change) => {
          const raw = (change && typeof change === 'object' ? change : {}) as Record<string, unknown>;
          return typeof raw.field === 'string' && raw.field !== '';
        });
        if (validChanges.length === 0) return `${kind} modificato`;
        const parts = validChanges.map((change) => {
          const raw = change as Record<string, unknown>;
          const field = raw.field as string;
          const label = AGREEMENT_FIELD_LABELS[field] ?? field;
          const from = agreementFieldValueLabel(field, raw.from);
          const to = agreementFieldValueLabel(field, raw.to);
          return `${label}: ${from} → ${to}`;
        });
        return `${kind} modificato — ${parts.join(' · ')}`;
      }
      if (action === 'removed') return signedOn ? `${kind} rimosso (sottoscritto il ${signedOn})` : `${kind} rimosso`;
      return 'Accordo';
    }
    case 'nota': return event.note ?? 'Annotazione';
    default: return event.note ? `${event.event} — ${event.note}` : event.event;
  }
}

export const TECHNICAL_ACTIVITY_EVENTS = new Set(['card_creata', 'dominio_verificato']);
