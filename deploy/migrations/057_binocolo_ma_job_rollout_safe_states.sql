-- Binocolo M&A — stati ma_job rollout-safe per job type aggiunti dopo il
-- worker originale. I worker vecchi leggono solo queued/running; i nuovi job
-- sidecar possono quindi usare pending/processing per restare invisibili durante
-- deploy progressivi finche' non li prende una replica aggiornata.
--
-- Target database: Anisetta PostgreSQL (schema binocolo). Apply after 056.

BEGIN;

ALTER TABLE binocolo.ma_job DROP CONSTRAINT IF EXISTS ma_job_status_check;
ALTER TABLE binocolo.ma_job ADD CONSTRAINT ma_job_status_check
  CHECK (status IN ('queued', 'running', 'pending', 'processing', 'ready', 'failed'));

-- Compatibile con l'indice storico ma_job_inflight_idx: lo lasciamo in piedi
-- per i binari vecchi che fanno ON CONFLICT sul predicato queued/running.
CREATE UNIQUE INDEX IF NOT EXISTS ma_job_inflight_active_idx
  ON binocolo.ma_job (session_id, job_type)
  WHERE status IN ('queued', 'running', 'pending', 'processing');

CREATE INDEX IF NOT EXISTS ma_job_active_idx
  ON binocolo.ma_job (status, updated_at)
  WHERE status IN ('queued', 'running', 'pending', 'processing');

COMMIT;
