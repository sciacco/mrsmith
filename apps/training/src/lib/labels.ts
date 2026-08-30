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

// Valori Factorial osservati sulla modalità di sessione.
export const MODALITY_LABELS: Record<string, string> = {
  inperson: 'In presenza',
  online: 'Online',
  mixed: 'Mista',
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

// Persone, catalogo e sync formativo (#158).
export const PERSON_STATUS_LABELS: Record<string, string> = {
  active: 'Attiva',
  on_leave: 'In aspettativa',
  terminated: 'Cessata',
};

export const DELIVERY_MODE_LABELS: Record<string, string> = {
  classroom: 'Aula',
  online_live: 'Online in diretta',
  online_self: 'Online in autonomia',
  on_the_job: 'On the job',
  mixed: 'Mista',
};

export const PROVIDER_KIND_LABELS: Record<string, string> = {
  internal: 'Interna',
  external: 'Esterna',
};

export const FACTORIAL_SYNC_PHASE_LABELS: Record<string, string> = {
  tombstone: 'Pulizia cessati',
  inbound: 'Da Factorial',
  outbound: 'Verso Factorial',
};

export const FACTORIAL_SYNC_SEVERITY_LABELS: Record<string, string> = {
  conflict: 'Conflitto',
  warning: 'Avviso',
};

export const FACTORIAL_SYNC_OUTCOME_LABELS: Record<string, string> = {
  ok: 'Riuscita',
  failed: 'Non riuscita',
};

// Entita' locale collegata a un finding del sync formativo (localEntity,
// backend factorial_sync_*.go).
export const FACTORIAL_SYNC_ENTITY_LABELS: Record<string, string> = {
  course: 'Corso',
  training_event: 'Evento',
  training_session: 'Sessione',
  enrollment: 'Iscrizione',
  enrollment_session: 'Iscrizione a sessione',
};

// Tipo di finding del sync formativo (kind, colonna testo libera —
// backend/internal/training/factorial_sync_inbound.go,
// factorial_sync_outbound.go, factorial_sync_tombstones.go). Copre i kind
// noti dei tre rami (tombstone/inbound/outbound); un kind non mappato resta
// visibile col valore grezzo (fallback nel chiamante).
export const FACTORIAL_SYNC_KIND_LABELS: Record<string, string> = {
  // inbound
  course_title_missing: 'Titolo corso mancante',
  event_reopen_blocked: 'Riapertura evento bloccata',
  session_ends_before_starts: "Sessione con fine prima dell'inizio",
  session_conflict: 'Conflitto sessione',
  session_provenance_unknown: 'Provenienza sessione sconosciuta',
  employee_unresolved: 'Dipendente non risolto',
  enrollment_reopen_blocked: 'Riapertura iscrizione bloccata',
  attendance_status_unknown: 'Stato di presenza sconosciuto',
  attendance_conflict: 'Conflitto di presenza',
  rda_exception: 'Eccezione RDA',
  // outbound
  training_company_mismatch: 'Corso su azienda diversa',
  training_correlator_conflict: 'Conflitto di aggancio corso',
  training_adopt: 'Aggancio corso esistente',
  training_create: 'Creazione corso',
  training_update_missing_year: 'Aggiornamento corso senza anno',
  training_propagate: 'Propagazione corso',
  class_no_dates: 'Evento senza date',
  class_no_exportable_sessions: 'Evento senza sessioni esportabili',
  class_correlator_conflict: 'Conflitto di aggancio evento',
  class_adopt: 'Aggancio evento esistente',
  class_create: 'Creazione evento',
  class_update_missing_costs: 'Aggiornamento evento senza costi',
  class_propagate: 'Propagazione evento',
  session_correlator_conflict: 'Conflitto di aggancio sessione',
  session_adopt: 'Aggancio sessione esistente',
  session_create: 'Creazione sessione',
  session_propagate: 'Propagazione sessione',
  session_type_missing: 'Sessione senza tipo di calendario',
  session_multiday_scheduled: 'Sessione con date su più giorni',
  employee_not_active: 'Persone non attive escluse',
  membership_backfill: 'Recupero iscrizione mancante',
  membership_bulk_create: 'Creazione massiva iscrizioni',
  access_bulk_create: 'Creazione massiva accessi',
  attendance_missing_after_access_create: 'Presenza assente dopo creazione accesso',
  attendance_propagate: 'Propagazione presenza',
  training_create_failed: 'Creazione corso non riuscita',
  training_update_failed: 'Aggiornamento corso non riuscito',
  class_create_failed: 'Creazione evento non riuscita',
  class_update_failed: 'Aggiornamento evento non riuscito',
  session_create_failed: 'Creazione sessione non riuscita',
  session_update_failed: 'Aggiornamento sessione non riuscito',
  membership_bulk_create_failed: 'Creazione massiva iscrizioni non riuscita',
  access_bulk_create_failed: 'Creazione massiva accessi non riuscita',
  attendance_reread_failed: 'Rilettura presenza non riuscita',
  attendance_update_failed: 'Aggiornamento presenza non riuscito',
  // tombstone
  session_tombstone_missing_remote_id: 'Sessione cessata senza id remoto',
  access_tombstone_missing_remote_id: 'Accesso cessato senza id remoto',
  access_bulk_destroy_failed: 'Cancellazione massiva accessi non riuscita',
  session_delete_failed: 'Cancellazione sessione non riuscita',
  access_missing_remote_provenance_unknown: 'Accesso remoto assente, provenienza sconosciuta',
  access_missing_remote_imported: 'Accesso remoto assente, importato da Factorial',
  access_missing_remote_inactive: 'Accesso remoto assente, persona non attiva',
  attendance_missing_remote_provenance_unknown: 'Presenza remota assente, provenienza sconosciuta',
  attendance_missing_remote_imported: 'Presenza remota assente, importata da Factorial',
  attendance_missing_remote_inactive: 'Presenza remota assente, persona non attiva',
  access_missing_check_failed: 'Verifica accesso remoto non riuscita',
  attendance_missing_check_failed: 'Verifica presenza remota non riuscita',
};

// Conseguimenti, valutazioni e percorsi nella scheda persona e nel catalogo
// (#162). Vocabolario allineato agli enum Postgres di
// deploy/migrations/012_anisetta_training.sql (award_outcome,
// validation_source) e alla vista current_status della stessa migrazione.
export const AWARD_OUTCOME_LABELS: Record<string, string> = {
  passed_exam: 'Esame superato',
  attendance_only: 'Attestato di frequenza',
};

export const VALIDATION_SOURCE_LABELS: Record<string, string> = {
  document_verified: 'Documento verificato',
  declared_survey: 'Dichiarata da questionario',
  declared_verbal: 'Dichiarata a voce',
  declared_cv: 'Dichiarata da CV',
  imported_legacy: 'Importata da storico',
};

export const AWARD_STATUS_LABELS: Record<string, string> = {
  valid: 'Valida',
  valid_no_expiry: 'Valida senza scadenza',
  expired: 'Scaduta',
};

// Scala descrittiva 0-5 delle valutazioni di competenza: copy presentazionale
// (nessuna rubrica di dominio dichiarata altrove), scala standard L&D.
export const ASSESSMENT_LEVEL_LABELS: Record<number, string> = {
  0: 'Nessuna conoscenza',
  1: 'Consapevolezza',
  2: 'Base',
  3: 'Intermedio',
  4: 'Avanzato',
  5: 'Esperto',
};

// Contatori aggregati della run (factorialSyncCounters, backend), mostrati
// solo quando positivi.
export const FACTORIAL_SYNC_COUNTER_LABELS: Record<string, string> = {
  tombstoneAccessDestroyed: 'accessi cessati rimossi',
  tombstoneSessionsDeleted: 'sessioni di cessati eliminate',
  tombstoneReset: 'agganci di cessati azzerati',
  tombstoneWarnings: 'avvisi pulizia cessati',
  inboundConflicts: 'conflitti da Factorial',
  inboundWarnings: 'avvisi da Factorial',
  inboundRDAExceptions: 'eccezioni RDA',
  outboundConflicts: 'conflitti verso Factorial',
  outboundWarnings: 'avvisi verso Factorial',
  outboundPlanned: 'operazioni pianificate verso Factorial',
};

// Pannello storia (#164): vocabolario di prodotto per le entità e le azioni
// realmente scritte in audit_log (allow-list a specchio di
// backend/internal/training/types_audit.go). Niente termini tecnici (nomi di
// tabella, nomi di azione grezzi) nella riga "chi · quando · cosa".
export const AUDIT_ENTITY_TYPE_LABELS: Record<string, string> = {
  certification_award: 'Conseguimento',
  certification: 'Certificazione',
  course: 'Corso',
  custom_groups: 'Gruppo',
  document: 'Attestato',
  employee: 'Persona',
  employee_learning_path: 'Percorso assegnato',
  enrollment: 'Iscrizione',
  enrollment_session: 'Partecipazione',
  event_expense: 'Spesa',
  learning_path: 'Percorso formativo',
  skill_area: 'Area di competenza',
  skill_assessment: 'Valutazione di competenza',
  team: 'Team',
  team_membership: 'Appartenenza',
  training_event: 'Evento',
  training_request: 'Richiesta',
  training_rule: 'Regola',
  training_session: 'Sessione',
  vendor: 'Fornitore',
};

export const AUDIT_ACTION_LABELS: Record<string, string> = {
  create: 'Creazione',
  create_from_rule: 'Creazione da regola',
  update: 'Modifica',
  update_facts: 'Modifica esiti',
  update_participation: 'Modifica partecipazione',
  cancel: 'Annullamento',
  complete_historical: 'Chiusura storica',
  reopen: 'Riapertura',
  assign: 'Assegnazione',
  remove: 'Rimozione',
  reconcile_delivery_status: 'Riconciliazione stato',
  replace_po: 'Sostituzione PO',
  replace_enrollments: 'Sostituzione iscrizioni collegate',
  replace_steps: 'Modifica passi percorso',
  delete: 'Eliminazione',
  feed: 'Alimentazione platea',
  set_people: 'Aggiornamento platea',
  set_members: 'Aggiornamento membri',
  archive: 'Archiviazione',
  tl_opinion: 'Parere del lead',
  decision: 'Decisione',
  withdraw: 'Ritiro',
  validate: 'Validazione',
  close: 'Chiusura appartenenza',
  directory_adopt: 'Adozione da directory',
  directory_rename: 'Rinomina da directory',
  directory_create: 'Creazione da directory',
  directory_unlink: 'Scollegamento da directory',
  directory_open: 'Riapertura da directory',
  directory_close: 'Chiusura da directory',
  directory_lead: 'Aggiornamento lead da directory',
  directory_update: 'Aggiornamento da directory',
  directory_terminate: 'Cessazione da directory',
  factorial_import: 'Importazione da Factorial',
  factorial_export: 'Esportazione verso Factorial',
  factorial_missing_reset: 'Scollegamento (assente su Factorial)',
  activate: 'Attivazione',
  deactivate: 'Disattivazione',
  cancel_by_event: 'Annullamento per evento annullato',
};
