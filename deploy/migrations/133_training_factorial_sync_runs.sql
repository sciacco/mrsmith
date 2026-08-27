-- Training: persistenza delle run del sync Factorial e dei loro esiti
-- completi (#154, task 6.3 di #151; parent #141). Emendamento ratificato
-- alla #141: nel database si traccia tutto, dati personali compresi; la
-- sanitizzazione (campioni <=20, soli id tecnici, niente nomi/email) resta
-- solo nei log strutturati, che non cambiano. Nessuna logica applicativa
-- qui, ne' retention/purge: additiva pura.
--
-- Delivered as a file only. The Agent does NOT apply it and does NOT connect
-- to any database configured in the env files.

BEGIN;

CREATE TABLE IF NOT EXISTS training.factorial_sync_run (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  started_at timestamptz NOT NULL,
  finished_at timestamptz,
  duration_ms bigint,
  actor text NOT NULL,
  dry_run boolean NOT NULL,
  outcome text NOT NULL CHECK (outcome IN ('ok', 'failed')),
  error text,
  sessions_without_class int NOT NULL DEFAULT 0,
  counters jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE training.factorial_sync_run IS
  'Una riga per run del sync formativo Factorial (RunFactorialSync), dry-run comprese; persistita a fine run (finish), anche su esito fallito.';
COMMENT ON COLUMN training.factorial_sync_run.counters IS
  'Contatori aggregati di tombstone/inbound/outbound della run (stessi numeri del log strutturato).';

CREATE INDEX IF NOT EXISTS idx_factorial_sync_run_started_at
  ON training.factorial_sync_run(started_at DESC);

CREATE TABLE IF NOT EXISTS training.factorial_sync_finding (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id uuid NOT NULL REFERENCES training.factorial_sync_run(id) ON DELETE CASCADE,
  phase text NOT NULL CHECK (phase IN ('tombstone', 'inbound', 'outbound')),
  severity text NOT NULL CHECK (severity IN ('conflict', 'warning')),
  kind text NOT NULL,
  ref text NOT NULL,
  local_entity text,
  local_id uuid,
  employee_id uuid REFERENCES training.employee(id),
  detail jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE training.factorial_sync_finding IS
  'Conflitti e warning di una run (tombstone/inbound/outbound), dati personali compresi. Le eccezioni RDA (#141 D7) sono kind=rda_exception, severity=warning.';
COMMENT ON COLUMN training.factorial_sync_finding.ref IS
  'Miglior identificatore disponibile lato Factorial (id tecnico) o, quando il ramo non ne ha uno, lato locale.';
COMMENT ON COLUMN training.factorial_sync_finding.local_id IS
  'Id locale (course/training_event/training_session/enrollment) quando gia'' risolto al momento della run, senza lookup aggiuntive.';

CREATE INDEX IF NOT EXISTS idx_factorial_sync_finding_run_id
  ON training.factorial_sync_finding(run_id);

CREATE INDEX IF NOT EXISTS idx_factorial_sync_finding_employee_id
  ON training.factorial_sync_finding(employee_id) WHERE employee_id IS NOT NULL;

COMMIT;
