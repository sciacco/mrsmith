-- 124: Binocolo M&A — annotazioni azienda unificate (issue #90).
--
-- Espansione additiva compatibile con il binario corrente: gli outcome esistenti
-- restano leggibili e scrivibili, mentre il nuovo binario può modificare e
-- soft-eliminare soltanto gli eventi `nota`.
-- Target database: Anisetta PostgreSQL, schema binocolo. Apply once after 123.

BEGIN;
SET LOCAL lock_timeout = '5s';

ALTER TABLE binocolo.ma_target_outcome
  ADD COLUMN updated_at timestamptz NULL,
  ADD COLUMN updated_by_subject text NULL,
  ADD COLUMN updated_by_email text NULL,
  ADD COLUMN deleted_at timestamptz NULL,
  ADD COLUMN deleted_by_subject text NULL,
  ADD COLUMN deleted_by_email text NULL;

CREATE INDEX ma_target_outcome_company_created_idx
  ON binocolo.ma_target_outcome (company_key, created_at DESC);

COMMIT;
