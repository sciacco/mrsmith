-- Binocolo M&A — coda job asincrona generica per le operazioni lunghe della
-- sessione (stima e, in seguito, esecuzione). Sostituisce l'esecuzione sincrona
-- dentro la richiesta HTTP: il POST accoda un job e ritorna 202, un worker
-- DB-backed lo elabora in background. Così la connessione non può andare in
-- timeout mentre il backend lavora, e un disconnect del client non uccide più il
-- lavoro a metà (girava su r.Context()).
--
-- Stato: queued -> running -> ready | failed. Stato durevole per il resume del
-- worker dopo un restart; lease per-riga per repliche/dev multipli sullo stesso DB
-- (stesso modello del deep worker, vedi 037/046).
--
-- Target database: Anisetta PostgreSQL (schema binocolo). Apply after 048.

BEGIN;

CREATE TABLE IF NOT EXISTS binocolo.ma_job (
  id                  uuid PRIMARY KEY,
  job_type            text NOT NULL,
  session_id          uuid NOT NULL REFERENCES binocolo.ma_session(id) ON DELETE CASCADE,
  strategy_version_id uuid NOT NULL REFERENCES binocolo.ma_strategy_version(id) ON DELETE CASCADE,
  status              text NOT NULL DEFAULT 'queued',
  payload             jsonb NOT NULL DEFAULT '{}'::jsonb,
  attempts            integer NOT NULL DEFAULT 0,
  error_code          text,
  -- Correlazione soft con ma_operation_trace (la trace è creata dal worker a
  -- inizio elaborazione). Nessuna FK: la trace può essere potata senza orfanare il job.
  trace_id            uuid,
  created_by_subject  text,
  created_by_email    text,
  lease_until         timestamptz,
  locked_by           text,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT ma_job_type_check   CHECK (job_type IN ('estimate', 'execute')),
  CONSTRAINT ma_job_status_check CHECK (status IN ('queued', 'running', 'ready', 'failed'))
);

-- Hot path del worker: le righe pendenti che deve far avanzare.
CREATE INDEX IF NOT EXISTS ma_job_pending_idx
  ON binocolo.ma_job (status, updated_at)
  WHERE status IN ('queued', 'running');

-- Anti-doppione: al massimo un job in volo per (sessione, tipo). Un secondo
-- submit mentre la stima è in coda/in corso viene rifiutato dall'INSERT
-- (ON CONFLICT DO NOTHING) invece di lanciare lavoro duplicato.
CREATE UNIQUE INDEX IF NOT EXISTS ma_job_inflight_idx
  ON binocolo.ma_job (session_id, job_type)
  WHERE status IN ('queued', 'running');

-- Nuovo stato di sessione in volo per la stima asincrona: draft -> estimating ->
-- estimated. La execute continua a usare 'running', già ammesso.
ALTER TABLE binocolo.ma_session DROP CONSTRAINT IF EXISTS ma_session_status_check;
ALTER TABLE binocolo.ma_session ADD CONSTRAINT ma_session_status_check
  CHECK (status IN ('draft', 'estimating', 'estimated', 'running', 'completed', 'failed'));

COMMIT;
