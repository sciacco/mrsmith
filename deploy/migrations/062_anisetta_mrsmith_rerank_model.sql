-- Centralized reranker registry (sibling to mrsmith.llm_model / embedding_model).
-- A reranker scores a (query, document) pair as a yes/no relevance question; on
-- Fireworks it is served via the /embeddings endpoint with return_logits over the
-- "no"/"yes" token ids. Those ids are tokenizer constants of the specific model,
-- so they are first-class columns (no_token_id / yes_token_id) — like dimension is
-- for embedding_model. Resolution is by (app, scope) with the same cascade as
-- llm_model: (app,scope) -> (app,'default') -> ('_global','default').
--
-- Reuses mrsmith.llm_provider for provider + key resolution (Fireworks).
-- Target database: Anisetta PostgreSQL (mrsmith schema). Requires migration 047
-- (llm_provider) and 058 (Fireworks provider seeded). Apply manually on the
-- database referenced by ANISETTA_DSN. Idempotent.

BEGIN;

CREATE TABLE IF NOT EXISTS mrsmith.rerank_model (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  app          text NOT NULL DEFAULT '_global',
  scope        text NOT NULL DEFAULT 'default',
  provider_id  uuid NOT NULL REFERENCES mrsmith.llm_provider(id) ON DELETE RESTRICT,
  name         text NOT NULL,
  model        text NOT NULL,
  no_token_id  integer NOT NULL,
  yes_token_id integer NOT NULL,
  params       jsonb NOT NULL DEFAULT '{}'::jsonb,
  is_default   boolean NOT NULL DEFAULT false,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (app, scope, model)
);

-- At most one default per (app, scope) — mirrors llm_model / llm_prompt.
CREATE UNIQUE INDEX IF NOT EXISTS rerank_model_one_default_per_app_scope
  ON mrsmith.rerank_model (app, scope) WHERE is_default;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM mrsmith.llm_provider WHERE name = 'Fireworks') THEN
    RAISE EXCEPTION 'Provider "Fireworks" not found in mrsmith.llm_provider - apply migration 058 or adjust the name in this migration';
  END IF;
END $$;

-- Seed the UC2 sector-classification reranker. Token ids are the Qwen3 tokenizer
-- constants for "no" (2753) and "yes" (9454).
INSERT INTO mrsmith.rerank_model
  (app, scope, provider_id, name, model, no_token_id, yes_token_id, params, is_default)
SELECT
  'binocolo',
  'ma_sector_classification',
  p.id,
  'Fireworks: Qwen3 Reranker 8B',
  'accounts/fireworks/models/qwen3-reranker-8b',
  2753,
  9454,
  '{}'::jsonb,
  false
FROM mrsmith.llm_provider p
WHERE p.name = 'Fireworks'
ON CONFLICT (app, scope, model) DO UPDATE
SET provider_id  = EXCLUDED.provider_id,
    name         = EXCLUDED.name,
    no_token_id  = EXCLUDED.no_token_id,
    yes_token_id = EXCLUDED.yes_token_id,
    params       = EXCLUDED.params,
    updated_at   = now();

UPDATE mrsmith.rerank_model
SET is_default = true
WHERE app = 'binocolo'
  AND scope = 'ma_sector_classification'
  AND model = 'accounts/fireworks/models/qwen3-reranker-8b';

COMMIT;
