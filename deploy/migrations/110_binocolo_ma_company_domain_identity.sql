-- Global domain registry: persist the confidence of the identity association.
-- Target: Anisetta PostgreSQL. Idempotent. Apply after 109.

BEGIN;

ALTER TABLE binocolo.ma_company_domain
  ADD COLUMN IF NOT EXISTS identity_state text;
ALTER TABLE binocolo.ma_company_domain DROP CONSTRAINT IF EXISTS ma_company_domain_identity_state_check;
ALTER TABLE binocolo.ma_company_domain ADD CONSTRAINT ma_company_domain_identity_state_check
  CHECK (identity_state IS NULL OR identity_state IN ('verified', 'vouched', 'assumed'));

UPDATE binocolo.ma_company_domain
SET identity_state = CASE method
  WHEN 'auto_verified' THEN 'verified'
  WHEN 'manual' THEN 'vouched'
  ELSE NULL
END
WHERE identity_state IS NULL;

COMMIT;
