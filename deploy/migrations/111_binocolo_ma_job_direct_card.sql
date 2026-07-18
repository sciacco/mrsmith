-- Async verification jobs for direct initiative cards.
-- Target: Anisetta PostgreSQL. Idempotent. Apply after 110 and before backend deploy.

BEGIN;

ALTER TABLE binocolo.ma_job ALTER COLUMN session_id DROP NOT NULL;
ALTER TABLE binocolo.ma_job ALTER COLUMN strategy_version_id DROP NOT NULL;
ALTER TABLE binocolo.ma_job
  ADD COLUMN IF NOT EXISTS initiative_id uuid REFERENCES binocolo.ma_initiative(id) ON DELETE CASCADE;

ALTER TABLE binocolo.ma_job DROP CONSTRAINT IF EXISTS ma_job_type_check;
ALTER TABLE binocolo.ma_job ADD CONSTRAINT ma_job_type_check
  CHECK (job_type IN ('estimate', 'execute', 'web_validation', 'gated_search', 'associate_domain', 'manual_add', 'card_domain_verify'));

CREATE UNIQUE INDEX IF NOT EXISTS ma_job_direct_card_active_idx
  ON binocolo.ma_job (initiative_id, job_type, ((payload->>'companyKey')))
  WHERE job_type = 'card_domain_verify' AND status IN ('pending', 'processing');

COMMIT;
