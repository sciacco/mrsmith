-- Binocolo use-case-1 ATECO retrieval: query-side embedding instruction.
-- Qwen3 embeddings are instruction-aware — the query is wrapped
-- "Instruct: …\nQuery: …" while the curated KB documents are embedded raw, and
-- the instruction (not decoration) is what sharpens retrieval. This seeds that
-- instruction as a tunable mrsmith.llm_prompt (app=binocolo,
-- scope=ma_strategy_ateco_embed) so it can change without a code deploy; the
-- runtime falls back to a compiled default if the prompt is absent.
--
-- Target database: Anisetta PostgreSQL (mrsmith schema). Requires migration 047.
-- Apply manually on the database referenced by ANISETTA_DSN. Idempotent.

BEGIN;

INSERT INTO mrsmith.llm_prompt (app, scope, name, prompt, is_default)
VALUES (
  'binocolo',
  'ma_strategy_ateco_embed',
  'M&A ATECO embedding query instruction v1',
  'Data una descrizione di settore aziendale per una ricerca M&A, recupera il concetto di business corrispondente.',
  false
)
ON CONFLICT (app, scope, name) DO UPDATE
SET prompt     = EXCLUDED.prompt,
    is_default = false;

UPDATE mrsmith.llm_prompt
SET is_default = true
WHERE app = 'binocolo'
  AND scope = 'ma_strategy_ateco_embed'
  AND name = 'M&A ATECO embedding query instruction v1';

COMMIT;
