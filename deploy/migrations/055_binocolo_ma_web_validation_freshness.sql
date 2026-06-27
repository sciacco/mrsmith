-- Binocolo Target M&A — freshness metadata for persisted web/LLM validation.
-- Target database: Anisetta PostgreSQL. Apply after 054.

BEGIN;

ALTER TABLE binocolo.ma_target_web_validation
  ADD COLUMN IF NOT EXISTS pipeline_version text NOT NULL DEFAULT 'candidate-web-validation-v1',
  ADD COLUMN IF NOT EXISTS input_hash text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS keyword_set_hash text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS llm_model_id text,
  ADD COLUMN IF NOT EXISTS llm_prompt_id text,
  ADD COLUMN IF NOT EXISTS llm_model text,
  ADD COLUMN IF NOT EXISTS stale_after timestamptz NOT NULL DEFAULT (now() + interval '90 days'),
  ADD COLUMN IF NOT EXISTS expires_at timestamptz NOT NULL DEFAULT (now() + interval '180 days');

DO $$
BEGIN
  ALTER TABLE binocolo.ma_target_web_validation
    ADD CONSTRAINT ma_target_web_validation_pipeline_version_not_blank CHECK (btrim(pipeline_version) <> '');
EXCEPTION WHEN duplicate_object THEN
  NULL;
END $$;

DO $$
BEGIN
  ALTER TABLE binocolo.ma_target_web_validation
    ADD CONSTRAINT ma_target_web_validation_stale_before_expiry CHECK (stale_after <= expires_at);
EXCEPTION WHEN duplicate_object THEN
  NULL;
END $$;

CREATE INDEX IF NOT EXISTS ma_target_web_validation_session_freshness_idx
  ON binocolo.ma_target_web_validation (session_id, stale_after, expires_at);

COMMIT;
