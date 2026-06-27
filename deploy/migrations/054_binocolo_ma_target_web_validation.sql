-- Binocolo Target M&A — latest web/LLM validation artifact per candidate.
-- Target database: Anisetta PostgreSQL. Apply after 053.
--
-- Like ma_target_rating, this is keyed by (session_id, company_key) rather than
-- ma_target.id, so the analyst validation can survive a session re-execute.

BEGIN;

CREATE TABLE IF NOT EXISTS binocolo.ma_target_web_validation (
  session_id                 uuid NOT NULL REFERENCES binocolo.ma_session(id) ON DELETE CASCADE,
  company_key                text NOT NULL,
  target_id                  uuid REFERENCES binocolo.ma_target(id) ON DELETE SET NULL,
  run_id                     uuid REFERENCES binocolo.ma_execution_run(id) ON DELETE SET NULL,

  selected_domain            text,
  domain_confidence          text,
  domain_score               integer,
  web_score                  integer NOT NULL,
  web_confidence             text,
  web_validation_state       text NOT NULL,
  final_action               text NOT NULL,
  analyst_verdict            text,
  analyst_action             text,
  analyst_confidence         text,

  summary                    jsonb NOT NULL DEFAULT '{}'::jsonb,
  keyword_set                jsonb NOT NULL DEFAULT '{}'::jsonb,
  selected_domain_payload    jsonb NOT NULL DEFAULT '{}'::jsonb,
  domain_response            jsonb NOT NULL DEFAULT '{}'::jsonb,
  evidence_runs              jsonb NOT NULL DEFAULT '[]'::jsonb,
  candidate_match_analysis   jsonb NOT NULL DEFAULT '{}'::jsonb,
  candidate_match_error      text,
  final_decision             jsonb NOT NULL DEFAULT '{}'::jsonb,

  created_by_subject         text,
  created_by_email           text,
  created_at                 timestamptz NOT NULL DEFAULT now(),
  updated_by_subject         text,
  updated_by_email           text,
  updated_at                 timestamptz NOT NULL DEFAULT now(),

  PRIMARY KEY (session_id, company_key),
  CONSTRAINT ma_target_web_validation_company_key_not_blank CHECK (btrim(company_key) <> ''),
  CONSTRAINT ma_target_web_validation_domain_score_check CHECK (domain_score IS NULL OR domain_score BETWEEN 0 AND 100),
  CONSTRAINT ma_target_web_validation_web_score_check CHECK (web_score BETWEEN 0 AND 100),
  CONSTRAINT ma_target_web_validation_state_check CHECK (
    web_validation_state IN ('confirmed', 'deprioritized', 'domain_unresolved', 'analysis_unavailable', 'rejected', 'unclear')
  ),
  CONSTRAINT ma_target_web_validation_action_check CHECK (
    final_action IN ('confirm', 'deprioritize', 'reject', 'needs_domain_review', 'needs_business_validation')
  )
);

CREATE INDEX IF NOT EXISTS ma_target_web_validation_session_action_idx
  ON binocolo.ma_target_web_validation (session_id, final_action, updated_at DESC);

CREATE INDEX IF NOT EXISTS ma_target_web_validation_session_state_idx
  ON binocolo.ma_target_web_validation (session_id, web_validation_state, updated_at DESC);

COMMIT;
