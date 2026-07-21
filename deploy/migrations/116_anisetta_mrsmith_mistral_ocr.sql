-- MrSmith LLM registry — provider Mistral + registro modelli OCR (issue #78,
-- Fase 1). L'OCR dei bilanci (Mistral OCR) è un servizio a sé: NON è una chat
-- completion, quindi ha un registro dedicato con RISOLUZIONE PROPRIA e NESSUN
-- fallback sulla cascata chat di mrsmith.llm_model. Modellato sul precedente
-- strutturale mrsmith.rerank_model (mig 062): sibling con (app, scope, is_default),
-- provider agganciato per NOME, un solo default per (app, scope).
--
-- Target database: Anisetta PostgreSQL, schema mrsmith (registry post-cutover,
-- mig 047). Apply after 115 (ordine globale) — dipende da mig 047 (llm_provider).
-- Applicata a mano dall'utente sul DB di ANISETTA_DSN. Idempotente.

BEGIN;

-- ---------------------------------------------------------------------------
-- Provider Mistral (OpenAI-compatible per la parte chat; qui serve come host
-- del servizio OCR). Chiave env-first come gli altri provider (mig 047):
-- api_key_env risolto per primo a runtime, api_key plaintext come fallback.
-- Guard idempotente sull'indice unico lower(name).
-- ---------------------------------------------------------------------------
INSERT INTO mrsmith.llm_provider (name, base_url, api_key_env)
SELECT 'Mistral', 'https://api.mistral.ai/v1', 'MISTRAL_API_KEY'
WHERE NOT EXISTS (
  SELECT 1 FROM mrsmith.llm_provider WHERE lower(name) = 'mistral'
);

-- ---------------------------------------------------------------------------
-- Registro modelli OCR. Colonne/stile allineati a rerank_model (mig 062):
-- risoluzione per (app, scope) con al più un default per (app, scope), provider
-- via FK. Nessuna cascata verso llm_model: un fascicolo si legge con l'OCR
-- dichiarato, mai con un modello chat di ripiego.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS mrsmith.ocr_model (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  app         text NOT NULL DEFAULT '_global',
  scope       text NOT NULL DEFAULT 'default',
  provider_id uuid NOT NULL REFERENCES mrsmith.llm_provider(id) ON DELETE RESTRICT,
  name        text NOT NULL,
  model       text NOT NULL,
  params      jsonb NOT NULL DEFAULT '{}'::jsonb,
  is_default  boolean NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (app, scope, model)
);

-- Al più un default per (app, scope) — mirror di rerank_model / llm_model.
CREATE UNIQUE INDEX IF NOT EXISTS ocr_model_one_default_per_app_scope
  ON mrsmith.ocr_model (app, scope) WHERE is_default;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM mrsmith.llm_provider WHERE lower(name) = 'mistral') THEN
    RAISE EXCEPTION 'Provider "Mistral" non trovato in mrsmith.llm_provider - riapplica la sezione provider di questa migrazione';
  END IF;
END $$;

-- Seed dell'OCR usato dall'ingest bilanci binocolo. Agganciato al provider
-- Mistral per nome (nessun UUID hardcodato). ON CONFLICT DO NOTHING: non
-- sovrascrive una configurazione già modificata a mano.
INSERT INTO mrsmith.ocr_model (app, scope, provider_id, name, model, params, is_default)
SELECT 'binocolo', 'ma_filing_ocr', p.id, 'Mistral OCR', 'mistral-ocr-4-0', '{}'::jsonb, true
FROM mrsmith.llm_provider p
WHERE lower(p.name) = 'mistral'
ON CONFLICT (app, scope, model) DO NOTHING;

COMMIT;
