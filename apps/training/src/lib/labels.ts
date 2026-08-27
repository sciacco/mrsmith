// Unica fonte del copy degli stati Training (#155). Mappe italiane pure:
// nessuna logica, nessun ricalcolo di dominio — solo etichette.

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
