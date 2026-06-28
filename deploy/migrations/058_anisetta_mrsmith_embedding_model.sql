-- Centralized embedding-model registry (sibling to mrsmith.llm_model, but for
-- embedding models). Embeddings differ from chat models in three ways that make
-- a dedicated table cleaner than extending llm_model:
--   1. dimension is intrinsic and first-class (drives the drift check);
--   2. NO cascade/fallback — falling back to a different embedding model yields
--      vectors from an incompatible space (silent garbage), so the binding is exact;
--   3. resolution is row-driven (a stored vector set names its model), not
--      scope-driven with fallback.
-- Reuses mrsmith.llm_provider for provider + key resolution (Fireworks).
--
-- Target database: Anisetta PostgreSQL (mrsmith schema).
-- Apply manually on the database referenced by ANISETTA_DSN. Idempotent.
--
-- The embedding model is bound to provider "Fireworks" by NAME (no hardcoded
-- UUID). If your mrsmith.llm_provider row is named differently, change 'Fireworks'
-- below before applying.

BEGIN;

CREATE TABLE IF NOT EXISTS mrsmith.embedding_model (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  provider_id uuid NOT NULL REFERENCES mrsmith.llm_provider(id) ON DELETE RESTRICT,
  name        text NOT NULL,
  model       text NOT NULL,
  dimension   integer NOT NULL CHECK (dimension > 0),
  distance    text NOT NULL DEFAULT 'Cosine' CHECK (distance IN ('Cosine','Dot','Euclid')),
  params      jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider_id, model)
);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM mrsmith.llm_provider WHERE name = 'Fireworks') THEN
    RAISE EXCEPTION 'Provider "Fireworks" not found in mrsmith.llm_provider - adjust the name in this migration to match your registry';
  END IF;
END $$;

-- Seed the embedding model used to build the binocolo KB. dimension MUST match
-- the vectors actually produced by this model; the runtime checks the stored
-- vector length against it.
INSERT INTO mrsmith.embedding_model
  (provider_id, name, model, dimension, distance, params)
SELECT
  p.id,
  'Fireworks: Qwen3 Embedding 8B',
  'accounts/fireworks/models/qwen3-embedding-8b',
  4096,
  'Cosine',
  '{}'::jsonb
FROM mrsmith.llm_provider p
WHERE p.name = 'Fireworks'
ON CONFLICT (provider_id, model) DO UPDATE
SET name       = EXCLUDED.name,
    dimension  = EXCLUDED.dimension,
    distance   = EXCLUDED.distance,
    params     = EXCLUDED.params,
    updated_at = now();

COMMIT;
