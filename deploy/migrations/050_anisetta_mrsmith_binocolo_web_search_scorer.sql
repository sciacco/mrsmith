-- Seed the binocolo "web_search_scorer" scope into the centralized LLM registry:
-- the small batched model that rates web-search results by relevance to the searched
-- terms (see web_search.go scoreWebSearchResults).
-- Target database: Anisetta PostgreSQL (mrsmith schema).
-- Apply manually on the database referenced by ANISETTA_DSN. Idempotent.
--
-- The model is bound to provider "Fireworks" by NAME (no hardcoded UUID). If your
-- mrsmith.llm_provider row is named differently, change 'Fireworks' below to match
-- before applying.

BEGIN;

-- Fail loudly if the provider is missing, so a silent no-op doesn't leave the scope
-- without a model binding (ResolveModel would then return ErrConfigNotFound).
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM mrsmith.llm_provider WHERE name = 'Fireworks') THEN
    RAISE EXCEPTION 'Provider "Fireworks" not found in mrsmith.llm_provider — adjust the name in this migration to match your registry';
  END IF;
END $$;

-- Model binding for (binocolo, web_search_scorer). Token-frugal: small max_tokens,
-- temperature 0; JSON mode on, tools off.
INSERT INTO mrsmith.llm_model
  (app, scope, provider_id, name, model, params, supports_tools, supports_json_mode, is_default)
SELECT
  'binocolo',
  'web_search_scorer',
  p.id,
  'Fireworks: DeepSeek V4 Flash',
  'accounts/fireworks/models/deepseek-v4-flash',
  '{"temperature":0,"max_tokens":512}'::jsonb,
  false,
  true,
  true
FROM mrsmith.llm_provider p
WHERE p.name = 'Fireworks'
ON CONFLICT (app, scope, model) DO UPDATE
SET provider_id        = EXCLUDED.provider_id,
    name               = EXCLUDED.name,
    params             = EXCLUDED.params,
    supports_tools     = EXCLUDED.supports_tools,
    supports_json_mode = EXCLUDED.supports_json_mode,
    is_default         = EXCLUDED.is_default;

-- Scoring prompt. Output is intentionally minimal (index + score only) to keep token
-- consumption low; no rationale.
INSERT INTO mrsmith.llm_prompt (app, scope, name, prompt, is_default)
VALUES (
  'binocolo',
  'web_search_scorer',
  'Web search relevance scorer v1',
  $prompt$Sei un valutatore di pertinenza per risultati di ricerca web.

Ricevi un oggetto JSON con:
- "terms": i termini cercati dall'utente
- "results": una lista di risultati, ognuno con "i" (indice), "title" e "text" (estratto)

Assegna a ogni risultato un punteggio intero da 0 a 100 che misura quanto il suo contenuto è pertinente ai termini cercati: presenza esplicita dei termini, vicinanza semantica e completezza rispetto a "terms".

Restituisci SOLO un oggetto JSON valido in questa forma, senza testo aggiuntivo:
{"scores": [{"i": 0, "score": 87}, {"i": 1, "score": 42}]}

Regole:
- un oggetto per ogni "i" ricevuto, con lo stesso indice.
- "score" intero 0-100.
- nessuna motivazione, nessun campo extra.
- valuta solo in base a "title" e "text" forniti; non inventare contenuti non presenti.$prompt$,
  true
)
ON CONFLICT (app, scope, name) DO UPDATE
SET prompt     = EXCLUDED.prompt,
    is_default = EXCLUDED.is_default;

COMMIT;
