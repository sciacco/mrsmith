import type { MATargetOutcome } from '../../../api/types';
import { esitoLabel, stateLabel } from '../../../lib/cardStates';

/** Human label for persisted activity payloads. Historical payload keys are
 * resolved at presentation time and are never rewritten. */
export function eventLabel(event: MATargetOutcome, sessionMap: Map<string, string> = new Map()): string {
  const payload = event.payload as Record<string, unknown> | undefined;
  switch (event.event) {
    case 'stato': {
      const from = payload && typeof payload.from === 'string' ? stateLabel(payload.from) : null;
      const to = payload && typeof payload.to === 'string' ? stateLabel(payload.to) : null;
      return from && to ? `${from} → ${to}` : `Stato aggiornato${event.note ? ` — ${event.note}` : ''}`;
    }
    case 'chiusura': {
      const state = payload && typeof payload.stato === 'string' ? stateLabel(payload.stato) : 'Chiusura';
      const outcome = payload && typeof payload.esito === 'string' && payload.esito ? ` — ${esitoLabel(payload.esito)}` : '';
      return event.note ? `${state}${outcome}: ${event.note}` : `${state}${outcome}`;
    }
    case 'card_creata': {
      const rating = payload && typeof payload.rating === 'number' ? payload.rating : 0;
      const stars = rating > 0 ? ` ${'★'.repeat(Math.min(rating, 3))}` : '';
      const sessionId = payload && typeof payload.sessionId === 'string' ? payload.sessionId : null;
      const title = sessionId ? sessionMap.get(sessionId) ?? null : null;
      return `Aggiunta${title ? ` da ${title}` : ''}${stars}${event.note ? ` — ${event.note}` : ''}`;
    }
    case 'card_riaperta': return 'Rimessa in lavorazione';
    case 'card_rimossa': return 'Rimossa dalla lavorazione';
    case 'dominio_verificato': {
      const outcome = payload && typeof payload.esito === 'string' ? payload.esito : 'non_verificabile';
      if (outcome === 'confermato') return 'Verifica automatica dominio: confermata';
      if (outcome === 'non_confermato') return 'Verifica automatica dominio: non confermata';
      return 'Verifica automatica dominio: non disponibile';
    }
    case 'contattato': return event.note ? `Contattata — ${event.note}` : 'Contattata';
    case 'buon_lead': return event.note ? `Buon lead — ${event.note}` : 'Buon lead';
    case 'no_go': return event.note ? `No-go — ${event.note}` : 'No-go';
    case 'nota': return event.note ?? 'Annotazione';
    default: return event.note ? `${event.event} — ${event.note}` : event.event;
  }
}

export const TECHNICAL_ACTIVITY_EVENTS = new Set(['card_creata', 'dominio_verificato']);
