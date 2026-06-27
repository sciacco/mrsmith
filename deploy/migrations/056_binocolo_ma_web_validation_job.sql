-- Binocolo M&A — estende la coda job asincrona per l'arricchimento web
-- dei target di sessione. Apply after 055.

BEGIN;

ALTER TABLE binocolo.ma_job DROP CONSTRAINT IF EXISTS ma_job_type_check;
ALTER TABLE binocolo.ma_job ADD CONSTRAINT ma_job_type_check
  CHECK (job_type IN ('estimate', 'execute', 'web_validation'));

COMMIT;
