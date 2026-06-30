-- Binocolo UC2: promote Cerebras gpt-oss-120b to the default candidate_match_analyst model.
--
-- Why: the 10× model comparison (apps/binocolo/docs/UC2-SECTOR-MODEL-EVALUATION.md) found
-- gpt-oss-120b tied on quality with the DeepSeek analyst (current default) but ~15x faster
-- and ~25% cheaper, on a different provider (Cerebras vs Fireworks) for outage resilience.
--
-- This DEMOTES the current default at (binocolo, candidate_match_analyst) — it stays as a
-- non-default row, still resolvable by id — and sets gpt-oss-120b as the new default. The
-- prompt binding is unchanged. Idempotent (safe to re-run).
--
-- The model is bound to provider "Cerebras" by NAME (no hardcoded UUID); adjust below if
-- your mrsmith.llm_provider row is named differently. Params set max_tokens=5000 (the
-- analyst JSON is short; this is the value validated in the comparison runs).
--
-- Target database: Anisetta PostgreSQL (mrsmith schema). Requires 047 + 053. Apply manually
-- on the database referenced by ANISETTA_DSN.

BEGIN;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM mrsmith.llm_provider WHERE name = 'Cerebras') THEN
    RAISE EXCEPTION 'Provider "Cerebras" not found in mrsmith.llm_provider - adjust the name in this migration to match your registry';
  END IF;
END $$;

-- Demote whatever is currently default at this scope (kept as a non-default fallback).
UPDATE mrsmith.llm_model
SET is_default = false
WHERE app = 'binocolo' AND scope = 'candidate_match_analyst' AND is_default;

-- Promote Cerebras gpt-oss-120b to the default analyst.
INSERT INTO mrsmith.llm_model
  (app, scope, provider_id, name, model, params, supports_tools, supports_json_mode, is_default)
SELECT
  'binocolo',
  'candidate_match_analyst',
  p.id,
  'Cerebras: gpt-oss-120b',
  'gpt-oss-120b',
  '{"max_tokens":5000}'::jsonb,
  false,
  true,
  true
FROM mrsmith.llm_provider p
WHERE p.name = 'Cerebras'
ON CONFLICT (app, scope, model) DO UPDATE
SET provider_id        = EXCLUDED.provider_id,
    name               = EXCLUDED.name,
    params             = EXCLUDED.params,
    supports_tools     = EXCLUDED.supports_tools,
    supports_json_mode = EXCLUDED.supports_json_mode,
    is_default         = true;

COMMIT;
