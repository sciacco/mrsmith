-- Binocolo M&A target discovery workspace.
-- Target database: Anisetta PostgreSQL.
-- Apply manually on the database referenced by ANISETTA_DSN before enabling
-- /binocolo/v1/ma saved sessions.

BEGIN;

CREATE SCHEMA IF NOT EXISTS binocolo;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS binocolo.llm_model (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  scope       text NOT NULL,
  name        text NOT NULL CHECK (btrim(name) <> ''),
  model       text NOT NULL CHECK (btrim(model) <> ''),
  is_default  boolean NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT llm_model_scope_check CHECK (scope ~ '^[a-z][a-z0-9_]*$')
);

CREATE UNIQUE INDEX IF NOT EXISTS llm_model_scope_model_unique
  ON binocolo.llm_model (scope, model);

CREATE UNIQUE INDEX IF NOT EXISTS llm_model_one_default_per_scope
  ON binocolo.llm_model (scope)
  WHERE is_default;

INSERT INTO binocolo.llm_model (id, scope, name, model, is_default)
VALUES
  ('00000000-0000-0000-0000-000000000101', 'default', 'Gemini Flash Lite', 'google/gemini-2.5-flash-lite-preview-09-2025', true),
  ('00000000-0000-0000-0000-000000000111', 'ma_strategy', 'Gemini Flash Lite', 'google/gemini-2.5-flash-lite-preview-09-2025', true),
  ('00000000-0000-0000-0000-000000000112', 'ma_strategy', 'Gemini Flash Lite precedente', 'google/gemini-2.5-flash-lite-preview-06-17', false),
  ('00000000-0000-0000-0000-000000000121', 'ma_sector_classification', 'Gemini Flash Lite', 'google/gemini-2.5-flash-lite-preview-09-2025', true),
  ('00000000-0000-0000-0000-000000000122', 'ma_sector_classification', 'Gemini Flash Lite precedente', 'google/gemini-2.5-flash-lite-preview-06-17', false)
ON CONFLICT (scope, model) DO UPDATE
SET name = EXCLUDED.name,
    is_default = EXCLUDED.is_default;

CREATE TABLE IF NOT EXISTS binocolo.llm_prompt (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  scope       text NOT NULL,
  name        text NOT NULL CHECK (btrim(name) <> ''),
  prompt      text NOT NULL CHECK (btrim(prompt) <> ''),
  is_default  boolean NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT llm_prompt_scope_check CHECK (scope ~ '^[a-z][a-z0-9_]*$')
);

CREATE UNIQUE INDEX IF NOT EXISTS llm_prompt_scope_name_unique
  ON binocolo.llm_prompt (scope, name);

CREATE UNIQUE INDEX IF NOT EXISTS llm_prompt_one_default_per_scope
  ON binocolo.llm_prompt (scope)
  WHERE is_default;

INSERT INTO binocolo.llm_prompt (id, scope, name, prompt, is_default)
VALUES (
  '00000000-0000-0000-0000-000000000211',
  'ma_strategy',
  'Strategia M&A v1',
  $prompt$
Sei un analista M&A per ricerche su aziende italiane.
Trasforma la richiesta dell'utente in una strategia JSON per interrogare Company IT-search.
Rispondi solo con JSON valido nel formato:
{
  "strategy": {
    "title": "titolo breve",
    "sectorDescription": "settore target",
    "territoryLabel": "territorio in parole",
    "provinces": ["MI"],
    "activityStatus": "ATTIVA",
    "searchLimit": 100,
    "turnoverAround": 5000000,
    "turnoverMin": 3500000,
    "turnoverMax": 6500000,
    "employeeMin": null,
    "employeeMax": null,
    "atecoCandidates": [{"code":"6201","description":"...","rationale":"..."}],
    "keywords": ["software"],
    "scoringCriteria": [
      {
        "id": "criterio_stabile_in_snake_case",
        "label": "condizione richiesta dall'utente",
        "description": "come valutare la condizione",
        "weight": 10,
        "evaluation": {
          "sourcePath": "campo target o percorso dati vendor",
          "operator": "exists|contains|eq|not_equals|gt|gte|lt|lte|between",
          "value": "valore atteso se serve",
          "min": null,
          "max": null,
          "tolerance": null,
          "match": "any"
        },
        "source": "richiesta utente"
      }
    ],
    "rationale": "sintesi della strategia",
    "missingCriteria": []
  }
}
Regole:
- usa activityStatus ATTIVA se non richiesto diversamente;
- searchLimit deve essere 100 salvo richiesta esplicita diversa; non superare mai 1000;
- se l'utente dice "intorno a" un fatturato, imposta turnoverAround e anche min/max a +/-30%;
- proponi codici ATECO plausibili con razionale, ma non inventare dati aziendali;
- separa i filtri di ricerca vendor dai criteri di valutazione: ogni condizione particolare richiesta dall'utente deve entrare in scoringCriteria, non in campi permanenti;
- usa sourcePath solo quando la condizione e' verificabile su un campo target o sul payload Company; se non e' verificabile, lascia sourcePath vuoto e spiega la condizione in description;
- se un criterio non e' derivabile dalla richiesta, lascialo vuoto e aggiungilo a missingCriteria;
- provinces deve contenere sigle italiane di due lettere quando il territorio e' provinciale.
$prompt$,
  true
)
ON CONFLICT (scope, name) DO UPDATE
SET prompt = EXCLUDED.prompt,
    is_default = EXCLUDED.is_default;

CREATE TABLE IF NOT EXISTS binocolo.ma_session (
  id                    uuid PRIMARY KEY,
  title                 text NOT NULL CHECK (btrim(title) <> ''),
  prompt                text NOT NULL CHECK (btrim(prompt) <> ''),
  status                text NOT NULL DEFAULT 'draft',
  selected_strategy     text,
  active_strategy_id    uuid,
  created_by_subject    text,
  created_by_email      text,
  last_estimated_at     timestamptz,
  last_executed_at      timestamptz,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT ma_session_status_check CHECK (status IN ('draft', 'estimated', 'running', 'completed', 'failed')),
  CONSTRAINT ma_session_selected_strategy_check CHECK (selected_strategy IS NULL OR selected_strategy IN ('ateco', 'expanded'))
);

CREATE TABLE IF NOT EXISTS binocolo.ma_strategy_version (
  id                 uuid PRIMARY KEY,
  session_id         uuid NOT NULL REFERENCES binocolo.ma_session(id) ON DELETE CASCADE,
  version            integer NOT NULL CHECK (version > 0),
  strategy           jsonb NOT NULL,
  created_by_email   text,
  created_at         timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT ma_strategy_version_unique UNIQUE (session_id, version)
);

ALTER TABLE binocolo.ma_session
  DROP CONSTRAINT IF EXISTS ma_session_active_strategy_fkey;
ALTER TABLE binocolo.ma_session
  ADD CONSTRAINT ma_session_active_strategy_fkey
  FOREIGN KEY (active_strategy_id) REFERENCES binocolo.ma_strategy_version(id);

CREATE TABLE IF NOT EXISTS binocolo.ma_dry_run_estimate (
  id                   uuid PRIMARY KEY,
  session_id           uuid NOT NULL REFERENCES binocolo.ma_session(id) ON DELETE CASCADE,
  strategy_version_id  uuid NOT NULL REFERENCES binocolo.ma_strategy_version(id) ON DELETE CASCADE,
  strategy_type        text NOT NULL,
  ateco_code           text,
  ateco_description    text,
  province             text,
  estimated_count      integer NOT NULL DEFAULT 0 CHECK (estimated_count >= 0),
  estimated_cost       numeric(12, 4) NOT NULL DEFAULT 0 CHECK (estimated_cost >= 0),
  selected             boolean NOT NULL DEFAULT false,
  params               jsonb NOT NULL DEFAULT '{}'::jsonb,
  vendor_response      jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at           timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT ma_dry_run_estimate_strategy_check CHECK (strategy_type IN ('ateco', 'expanded'))
);

CREATE TABLE IF NOT EXISTS binocolo.ma_execution_run (
  id                   uuid PRIMARY KEY,
  session_id           uuid NOT NULL REFERENCES binocolo.ma_session(id) ON DELETE CASCADE,
  strategy_version_id  uuid NOT NULL REFERENCES binocolo.ma_strategy_version(id) ON DELETE CASCADE,
  strategy_type        text NOT NULL,
  status               text NOT NULL DEFAULT 'running',
  estimated_count      integer NOT NULL DEFAULT 0 CHECK (estimated_count >= 0),
  result_count         integer NOT NULL DEFAULT 0 CHECK (result_count >= 0),
  error_code           text,
  started_at           timestamptz NOT NULL DEFAULT now(),
  completed_at         timestamptz,

  CONSTRAINT ma_execution_run_strategy_check CHECK (strategy_type IN ('ateco', 'expanded')),
  CONSTRAINT ma_execution_run_status_check CHECK (status IN ('running', 'completed', 'failed'))
);

CREATE TABLE IF NOT EXISTS binocolo.ma_target (
  id                 uuid PRIMARY KEY,
  session_id         uuid NOT NULL REFERENCES binocolo.ma_session(id) ON DELETE CASCADE,
  run_id             uuid NOT NULL REFERENCES binocolo.ma_execution_run(id) ON DELETE CASCADE,
  vendor_id          text,
  company_name       text NOT NULL CHECK (btrim(company_name) <> ''),
  vat_code           text,
  tax_code           text,
  province           text,
  town               text,
  activity_status    text,
  turnover           integer,
  turnover_year      integer,
  employees          integer,
  ateco_code         text,
  ateco_description  text,
  score              integer NOT NULL CHECK (score BETWEEN 0 AND 100),
  match_state        text NOT NULL,
  rationale          text NOT NULL DEFAULT '',
  missing_criteria   jsonb NOT NULL DEFAULT '[]'::jsonb,
  vendor_payload     jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at         timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT ma_target_match_state_check CHECK (match_state IN ('match', 'match_parziale', 'fuori_criterio'))
);

CREATE TABLE IF NOT EXISTS binocolo.ma_evidence (
  id           uuid PRIMARY KEY,
  target_id    uuid NOT NULL REFERENCES binocolo.ma_target(id) ON DELETE CASCADE,
  criterion    text NOT NULL,
  status       text NOT NULL,
  label        text NOT NULL,
  value        text,
  source_path  text,
  created_at   timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT ma_evidence_status_check CHECK (status IN ('match', 'match_parziale', 'criterio_mancante', 'fuori_criterio'))
);

CREATE TABLE IF NOT EXISTS binocolo.ma_model_audit (
  id                   uuid PRIMARY KEY,
  session_id           uuid REFERENCES binocolo.ma_session(id) ON DELETE CASCADE,
  strategy_version_id  uuid REFERENCES binocolo.ma_strategy_version(id) ON DELETE SET NULL,
  scope                text NOT NULL,
  model_id             uuid REFERENCES binocolo.llm_model(id) ON DELETE SET NULL,
  prompt_id            uuid REFERENCES binocolo.llm_prompt(id) ON DELETE SET NULL,
  model                text NOT NULL,
  prompt               jsonb NOT NULL DEFAULT '{}'::jsonb,
  response             jsonb NOT NULL DEFAULT '{}'::jsonb,
  usage                jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at           timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS binocolo.ma_export (
  id                uuid PRIMARY KEY,
  session_id        uuid NOT NULL REFERENCES binocolo.ma_session(id) ON DELETE CASCADE,
  format            text NOT NULL,
  row_count         integer NOT NULL DEFAULT 0 CHECK (row_count >= 0),
  created_by_email  text,
  created_at        timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT ma_export_format_check CHECK (format IN ('xlsx'))
);

CREATE INDEX IF NOT EXISTS ma_session_updated_at_idx
  ON binocolo.ma_session (updated_at DESC);

CREATE INDEX IF NOT EXISTS ma_strategy_session_idx
  ON binocolo.ma_strategy_version (session_id, version DESC);

CREATE INDEX IF NOT EXISTS ma_estimate_session_idx
  ON binocolo.ma_dry_run_estimate (session_id, selected, strategy_type);

CREATE INDEX IF NOT EXISTS ma_run_session_idx
  ON binocolo.ma_execution_run (session_id, started_at DESC);

CREATE INDEX IF NOT EXISTS ma_target_session_score_idx
  ON binocolo.ma_target (session_id, score DESC);

CREATE INDEX IF NOT EXISTS ma_target_identity_idx
  ON binocolo.ma_target (vat_code, tax_code, vendor_id);

CREATE OR REPLACE FUNCTION binocolo.set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS ma_session_set_updated_at ON binocolo.ma_session;
CREATE TRIGGER ma_session_set_updated_at
BEFORE UPDATE ON binocolo.ma_session
FOR EACH ROW
EXECUTE FUNCTION binocolo.set_updated_at();

DROP TRIGGER IF EXISTS llm_model_set_updated_at ON binocolo.llm_model;
CREATE TRIGGER llm_model_set_updated_at
BEFORE UPDATE ON binocolo.llm_model
FOR EACH ROW
EXECUTE FUNCTION binocolo.set_updated_at();

DROP TRIGGER IF EXISTS llm_prompt_set_updated_at ON binocolo.llm_prompt;
CREATE TRIGGER llm_prompt_set_updated_at
BEFORE UPDATE ON binocolo.llm_prompt
FOR EACH ROW
EXECUTE FUNCTION binocolo.set_updated_at();

COMMIT;
