-- Companion to 047: populate per-scope call params and seed the Manutenzioni
-- system prompt into the centralized registry, ahead of the backend cutover.
-- Target database: Anisetta PostgreSQL (mrsmith schema).
-- Apply manually on the database referenced by ANISETTA_DSN.
--
-- Additive / inert until the backend reads these tables (cutover phase). Safe to
-- apply anytime; idempotent.

BEGIN;

-- ---------------------------------------------------------------------------
-- Per-call params on the (app,scope) model bindings, for the scopes that map
-- 1:1 to a known Go call-site. Other scopes keep params '{}' and the Go caller
-- falls back to its current hardcoded defaults (e.g. deep_brief max_tokens 2200,
-- which has no dedicated scope row yet).
-- ---------------------------------------------------------------------------

-- binocolo ma_strategy: Temperature 0, MaxTokens 1800 (ma_service.go draftStrategy).
UPDATE mrsmith.llm_model
SET params = '{"temperature":0,"max_tokens":1800}'::jsonb
WHERE app = 'binocolo' AND scope = 'ma_strategy';

-- manutenzioni assistance_draft: Temperature 0.2, MaxTokens 4096 (assistance.go).
UPDATE mrsmith.llm_model
SET params = '{"temperature":0.2,"max_tokens":4096}'::jsonb
WHERE app = 'manutenzioni' AND scope = 'assistance_draft';

-- ---------------------------------------------------------------------------
-- Manutenzioni system prompt — moved from the Go constant
-- maintenanceAssistanceSystemPrompt (assistance.go) into the registry, so it
-- resolves like binocolo's prompts. The Go constant is removed at cutover.
-- ---------------------------------------------------------------------------
INSERT INTO mrsmith.llm_prompt (app, scope, name, prompt, is_default)
VALUES (
  'manutenzioni',
  'assistance_draft',
  'Assistenza manutenzioni v1',
  $prompt$Sei un assistente per manutenzioni tecniche interne. Devi proporre testi e classificazioni a partire dal contesto disponibile e dalle opzioni di riferimento.

Se la manutenzione non ha ancora id ne titolo e user_note contiene un brief libero, usa user_note come fonte primaria per inferire titolo, descrizione e classificazioni.
Se la manutenzione esiste gia con dati propri, integra e affina senza inventare informazioni non supportate.

Restituisci solo un oggetto JSON valido con questa forma:
{
  "texts": {
    "title_it": "titolo operativo in italiano",
    "title_en": "English title",
    "description_it": "descrizione operativa in italiano",
    "description_en": "English description",
    "reason_en": "English reason, only if reason_it exists",
    "residual_service_en": "English residual service, only if residual_service_it exists"
  },
  "service_taxonomy": [{"reference_id": 1, "confidence": 0.85, "rationale": "motivo sintetico"}],
  "reason_classes": [{"reference_id": 1, "confidence": 0.85, "rationale": "motivo sintetico"}],
  "impact_effects": [{"reference_id": 1, "confidence": 0.85, "rationale": "motivo sintetico"}],
  "quality_flags": [{"reference_id": 1, "confidence": 0.85, "rationale": "motivo sintetico"}],
  "summary": "sintesi sintetica delle proposte"
}

Regole:
- Usa solo reference_id presenti in reference_options.
- Per service_taxonomy scegli solo servizi coerenti con il technical_domain della manutenzione.
- Non inventare clienti, target, ordini, circuiti o asset puntuali.
- Non applicare automaticamente nulla: produci solo proposte.
- Mantieni un tono operativo, sintetico e adatto a comunicazioni interne.
- Se un dato non e supportato dal contesto, lascia il campo vuoto o ometti la proposta.$prompt$,
  true
)
ON CONFLICT (app, scope, name) DO UPDATE
SET prompt = EXCLUDED.prompt,
    is_default = EXCLUDED.is_default;

COMMIT;
