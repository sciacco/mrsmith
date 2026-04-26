export const STATUS_LABELS: Record<string, string> = {
  draft: 'Bozza',
  approved: 'Approvata',
  scheduled: 'Pianificata',
  announced: 'Annunciata',
  in_progress: 'In corso',
  completed: 'Completata',
  cancelled: 'Annullata',
  superseded: 'Superata',
};

export const WINDOW_STATUS_LABELS: Record<string, string> = {
  planned: 'Schedulata',
  cancelled: 'Annullata',
  superseded: 'Sostituita',
  executed: 'Eseguita',
};

export const NOTICE_TYPE_LABELS: Record<string, string> = {
  announcement: 'Annuncio',
  reminder: 'Promemoria',
  reschedule: 'Rischedulazione',
  cancellation: 'Annullamento',
  start: 'Avvio',
  completion: 'Completamento',
  internal_update: 'Aggiornamento interno',
};

export const MAINTENANCE_EVENT_LABELS: Record<string, string> = {
  created: 'Manutenzione creata',
  updated: 'Manutenzione aggiornata',
  announced: 'Manutenzione annunciata',
  started: 'Manutenzione avviata',
  completed: 'Manutenzione completata',
  cancelled: 'Manutenzione annullata',
  rescheduled: 'Finestra rischedulata',
};

export const WINDOW_ACTION_LABELS = {
  reschedule: 'Rischedula',
  rescheduleTitle: 'Rischedula finestra',
  rescheduleSuccess: 'Finestra rischedulata.',
  rescheduleFailure: 'Rischedulazione non riuscita.',
  cancel: 'Annulla',
  cancelTitle: 'Annulla finestra',
  cancelSuccess: 'Finestra annullata.',
  cancelFailure: 'Annullamento finestra non riuscito.',
} as const;

export const API_ERROR_MESSAGES: Record<string, string> = {
  manutenzioni_database_not_configured: 'Registro non configurato.',
  customer_lookup_not_configured: 'Ricerca clienti non disponibile.',
  maintenance_not_found: 'Manutenzione non trovata.',
  status_transition_not_allowed: 'Cambio stato non consentito.',
  customer_scope_required: "Definisci l'ambito clienti prima di continuare.",
  invalid_customer_scope: 'Ambito clienti non valido.',
  maintenance_window_required: 'Aggiungi una finestra prima di continuare.',
  invalid_window_range: 'La fine della finestra deve essere successiva all’inizio.',
  invalid_window: 'Verifica i dati della finestra.',
  cancellation_reason_required: 'Indica il motivo dell’annullamento.',
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
