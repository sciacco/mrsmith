// Unica fonte del copy degli stati Training (#155). Mappe italiane pure:
// nessuna logica, nessun ricalcolo di dominio — solo etichette.

import type { EventFlags } from '../api/types';

export const DELIVERY_STATUS_LABELS: Record<string, string> = {
  planned: 'Pianificata',
  in_progress: 'In corso',
  completed: 'Completata',
  partially_completed: 'Completata parzialmente',
  not_attended: 'Non frequentata',
  cancelled: 'Annullata',
};

export const PARTICIPATION_STATUS_LABELS: Record<string, string> = {
  assigned: 'Assegnata',
  in_progress: 'In corso',
  completed: 'Completata',
  not_attended: 'Non frequentata',
};

export const LEARNING_OUTCOME_LABELS: Record<string, string> = {
  passed: 'Superato',
  failed: 'Non superato',
  not_taken: 'Non sostenuto',
  not_required: 'Non richiesto',
};

export const ECONOMIC_STATE_LABELS: Record<string, string> = {
  approved: 'Approvata',
  pending: 'In attesa',
  rejected: 'Rifiutata',
};

// Condizioni evento: etichette dei tre raggruppamenti della coda "Eventi da
// sistemare" (flag di EventListRow).
export const EVENT_CONDITION_LABELS: Record<
  'needsReconciliation' | 'withoutSessions' | 'unassignedEnrollments',
  string
> = {
  needsReconciliation: 'Da riconciliare',
  withoutSessions: 'Senza sessioni',
  unassignedEnrollments: 'Iscrizioni non assegnate',
};

// Natura del bisogno di una regola formativa.
export const NEED_LABELS: Record<string, string> = {
  attendance: 'Frequenza',
  certification: 'Certificazione',
};

export const TL_OPINION_LABELS: Record<string, string> = {
  favorable: 'Favorevole',
  unfavorable: 'Sfavorevole',
};

// Badge compatti di condizione operativa evento (lista /eventi, dettaglio
// evento). Copre tutti i flag di EventFlags — distinta da
// EVENT_CONDITION_LABELS, che resta il copy delle sezioni della coda.
export const EVENT_CONDITION_BADGE_LABELS: Record<keyof EventFlags, string> = {
  cancelled: 'Annullato',
  withoutSessions: 'Senza sessioni',
  unassignedEnrollments: 'Non collocati',
  inProgress: 'In corso',
  needsReconciliation: 'Da riconciliare',
  concluded: 'Concluso',
};

export const SCHEDULE_TYPE_LABELS: Record<string, string> = {
  scheduled: 'Con date',
  self_paced: 'Autoapprendimento',
};

export const ENROLL_SKIP_REASON_LABELS: Record<string, string> = {
  already_enrolled: 'Già iscritta',
  inactive: 'Non attiva',
};

export const EVENT_ORIGIN_LABELS: Record<string, string> = {
  direct: 'Diretto',
  rule: 'Da regola',
  request: 'Da richiesta',
  factorial_import: 'Import Factorial',
};

export const BULK_ASSIGN_MODE_LABELS: Record<string, string> = {
  all_to_all: 'Assegna tutti a tutte',
  distribute: 'Distribuisci automaticamente',
  fill_session: 'Colloca i non assegnati',
};

// Esito e stato delle richieste formative (#157).
export const REQUEST_STATE_LABELS: Record<'open' | 'closed' | 'all', string> = {
  open: 'Aperte',
  closed: 'Chiuse',
  all: 'Tutte',
};

export const REQUEST_OUTCOME_LABELS: Record<string, string> = {
  accepted: 'Accolta',
  rejected: 'Respinta',
  withdrawn: 'Ritirata',
};

// Platea delle regole formative (#157).
export const POPULATION_KIND_LABELS: Record<string, string> = {
  all: 'Organizzazione',
  team: 'Team',
  skill_area: 'Area di competenza',
  custom_group: 'Gruppo locale',
  people: 'Singole persone',
};

export const RECURRENCE_ANCHOR_LABELS: Record<string, string> = {
  calendar: 'Da calendario',
  completion: 'Da completamento',
};
