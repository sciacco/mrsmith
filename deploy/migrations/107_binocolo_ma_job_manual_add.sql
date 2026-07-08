-- Binocolo ma_job — nuovo job_type 'manual_add'.
--
-- Permette a un analista di aggiungere manualmente una singola azienda a una
-- sessione MA esistente via P.IVA/CF. Il lavoro è asincrono e pre-leased come gli
-- altri job MA introdotti dopo la coda originale; usa stato pending/processing per
-- rollout sicuro.
--
-- Target database: Anisetta PostgreSQL (schema binocolo). Idempotente. Apply after 106.

BEGIN;

ALTER TABLE binocolo.ma_job DROP CONSTRAINT IF EXISTS ma_job_type_check;
ALTER TABLE binocolo.ma_job ADD CONSTRAINT ma_job_type_check
  CHECK (job_type IN ('estimate', 'execute', 'web_validation', 'gated_search', 'associate_domain', 'manual_add'));

COMMIT;
