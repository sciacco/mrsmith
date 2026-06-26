-- Centralized multi-provider LLM registry for the MrSmith portal.
-- Target database: Anisetta PostgreSQL (mrsmith schema).
-- Apply manually on the database referenced by ANISETTA_DSN.
--
-- PHASE 1 (this migration): additive schema + seeds ONLY. There is no backend
-- wiring yet — existing binocolo.llm_model / maintenance.llm_model remain the
-- authoritative config until a later cutover migration + Go change. A management
-- frontend (and its RBAC) is a future phase. Nothing here is dropped.
--
-- Design decisions captured here:
--   * OpenAI-compatible providers only (base_url + key + headers differ).
--   * Key resolution at runtime: os.Getenv(api_key_env) first, else api_key
--     (plaintext fallback). Uniform across prod and dev. A provider is "usable"
--     iff it resolves a key AND some llm_model row references it — no enabled flag.
--   * Shared registry under the mrsmith schema; apps on other DBs read it via
--     the injected anisetta pool.
--   * Per-call params (temperature, max_tokens, ...) live on the (app,scope)
--     model binding (llm_model.params jsonb), NOT on the provider.
--   * Per-call audit is generic (mrsmith.llm_call_audit, no domain FKs — soft
--     refs in context jsonb), records BOTH successes and failures. Binocolo's
--     rich operation trace stays in-app, correlated via request_id.
--   * Model resolution cascade (app,scope) -> (app,'default') -> ('_global','default').
--   * Prompt resolution cascade (app,scope) -> (app,'default'); no global.
--   * app/scope are exact-match lookup keys; no format CHECK (a valid-format typo
--     would pass it anyway, so it only gives false protection). Normalization is
--     handled in Go at cutover.

BEGIN;

CREATE SCHEMA IF NOT EXISTS mrsmith;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Generic updated_at trigger for mrsmith config tables.
CREATE OR REPLACE FUNCTION mrsmith.set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$;

-- ---------------------------------------------------------------------------
-- Providers — OpenAI-compatible endpoints.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS mrsmith.llm_provider (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name            text NOT NULL CHECK (btrim(name) <> ''),
  base_url        text NOT NULL CHECK (btrim(base_url) <> ''),
  api_key_env     text NOT NULL DEFAULT '',   -- env var name, e.g. 'OPENROUTER_API_KEY'
  api_key         text NOT NULL DEFAULT '',   -- plaintext fallback (used only if env empty)
  default_headers jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS llm_provider_name_unique
  ON mrsmith.llm_provider (lower(name));

DROP TRIGGER IF EXISTS llm_provider_set_updated_at ON mrsmith.llm_provider;
CREATE TRIGGER llm_provider_set_updated_at
BEFORE UPDATE ON mrsmith.llm_provider
FOR EACH ROW EXECUTE FUNCTION mrsmith.set_updated_at();

-- ---------------------------------------------------------------------------
-- Models — model->scope bindings per app (+ '_global' fallback tier).
-- params jsonb holds per-call request params (e.g. {"temperature":0,"max_tokens":1800}).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS mrsmith.llm_model (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  app                text NOT NULL,
  scope              text NOT NULL,
  provider_id        uuid NOT NULL REFERENCES mrsmith.llm_provider(id) ON DELETE RESTRICT,
  name               text NOT NULL CHECK (btrim(name) <> ''),
  model              text NOT NULL CHECK (btrim(model) <> ''),
  params             jsonb NOT NULL DEFAULT '{}'::jsonb,
  supports_tools     boolean NOT NULL DEFAULT false,
  supports_json_mode boolean NOT NULL DEFAULT false,
  is_default         boolean NOT NULL DEFAULT false,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS llm_model_app_scope_model_unique
  ON mrsmith.llm_model (app, scope, model);
CREATE UNIQUE INDEX IF NOT EXISTS llm_model_one_default_per_app_scope
  ON mrsmith.llm_model (app, scope) WHERE is_default;

DROP TRIGGER IF EXISTS llm_model_set_updated_at ON mrsmith.llm_model;
CREATE TRIGGER llm_model_set_updated_at
BEFORE UPDATE ON mrsmith.llm_model
FOR EACH ROW EXECUTE FUNCTION mrsmith.set_updated_at();

-- ---------------------------------------------------------------------------
-- Prompts — per app+scope. Not portable across apps; no global tier.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS mrsmith.llm_prompt (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  app         text NOT NULL,
  scope       text NOT NULL,
  name        text NOT NULL CHECK (btrim(name) <> ''),
  prompt      text NOT NULL CHECK (btrim(prompt) <> ''),
  is_default  boolean NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS llm_prompt_app_scope_name_unique
  ON mrsmith.llm_prompt (app, scope, name);
CREATE UNIQUE INDEX IF NOT EXISTS llm_prompt_one_default_per_app_scope
  ON mrsmith.llm_prompt (app, scope) WHERE is_default;

DROP TRIGGER IF EXISTS llm_prompt_set_updated_at ON mrsmith.llm_prompt;
CREATE TRIGGER llm_prompt_set_updated_at
BEFORE UPDATE ON mrsmith.llm_prompt
FOR EACH ROW EXECUTE FUNCTION mrsmith.set_updated_at();

-- ---------------------------------------------------------------------------
-- Generic per-call audit (successes AND failures). Config FKs are real (same
-- schema); domain refs are soft (context jsonb), so audit survives deletion of
-- domain entities. Append-only: no updated_at / trigger.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS mrsmith.llm_call_audit (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  app           text NOT NULL,
  scope         text NOT NULL,
  provider_id   uuid REFERENCES mrsmith.llm_provider(id) ON DELETE SET NULL,
  model_id      uuid REFERENCES mrsmith.llm_model(id)    ON DELETE SET NULL,
  prompt_id     uuid REFERENCES mrsmith.llm_prompt(id)   ON DELETE SET NULL,
  model         text NOT NULL DEFAULT '',          -- snapshot of model string at call time
  request       jsonb NOT NULL DEFAULT '{}'::jsonb, -- messages+tools+config (secrets redacted)
  response      jsonb NOT NULL DEFAULT '{}'::jsonb,
  usage         jsonb NOT NULL DEFAULT '{}'::jsonb, -- {prompt_tokens, completion_tokens, total_tokens}
  context       jsonb NOT NULL DEFAULT '{}'::jsonb, -- soft domain refs, e.g. {"session_id": "..."}
  actor_subject text NOT NULL DEFAULT '',
  actor_email   text NOT NULL DEFAULT '',
  request_id    text NOT NULL DEFAULT '',           -- correlation with in-app operation trace
  status        text NOT NULL DEFAULT 'succeeded' CHECK (status IN ('succeeded', 'failed')),
  error_code    text NOT NULL DEFAULT '',
  error_message text NOT NULL DEFAULT '',           -- secrets redacted (provider error bodies can echo keys)
  duration_ms   integer,
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS llm_call_audit_app_scope_created_idx
  ON mrsmith.llm_call_audit (app, scope, created_at DESC);
CREATE INDEX IF NOT EXISTS llm_call_audit_provider_created_idx
  ON mrsmith.llm_call_audit (provider_id, created_at DESC);
CREATE INDEX IF NOT EXISTS llm_call_audit_request_idx
  ON mrsmith.llm_call_audit (request_id) WHERE request_id <> '';

-- ===========================================================================
-- SEED DATA
-- ===========================================================================

-- Providers. Only OpenRouter has a key in the environment today (it is the
-- active provider). The others are seeded as documentation of intended
-- providers and become usable once a key is set — via their api_key_env in the
-- environment, or via api_key. NOTE: base_url for DeepSeek / Cerebras / MiniMax
-- / ZAI are best-effort and region-dependent — VERIFY before use.
INSERT INTO mrsmith.llm_provider (id, name, base_url, api_key_env) VALUES
  ('00000000-0000-0000-0000-00000000a001', 'OpenRouter', 'https://openrouter.ai/api/v1',         'OPENROUTER_API_KEY'),
  ('00000000-0000-0000-0000-00000000a002', 'OpenAI',     'https://api.openai.com/v1',            'OPENAI_API_KEY'),
  ('00000000-0000-0000-0000-00000000a003', 'Together',   'https://api.together.xyz/v1',          'TOGETHER_API_KEY'),
  ('00000000-0000-0000-0000-00000000a004', 'Fireworks',  'https://api.fireworks.ai/inference/v1','FIREWORKS_API_KEY'),
  ('00000000-0000-0000-0000-00000000a005', 'DeepSeek',   'https://api.deepseek.com',             'DEEPSEEK_API_KEY'),  -- VERIFY base_url
  ('00000000-0000-0000-0000-00000000a006', 'Cerebras',   'https://api.cerebras.ai/v1',           'CEREBRAS_API_KEY'),  -- VERIFY base_url
  ('00000000-0000-0000-0000-00000000a007', 'MiniMax',    'https://api.minimax.io/v1',            'MINIMAX_API_KEY'),   -- VERIFY base_url (region-dependent)
  ('00000000-0000-0000-0000-00000000a008', 'ZAI',        'https://api.z.ai/api/paas/v4',         'ZAI_API_KEY')        -- VERIFY base_url (region-dependent)
ON CONFLICT (id) DO NOTHING;

-- Binocolo models — copied faithfully from the live binocolo.llm_model (same DB).
-- params left '{}' here; per-scope params are populated at backend cutover from
-- the authoritative Go call-sites. Capability flags true/true (all current rows
-- are Gemini Flash Lite, which supports tools + json mode).
INSERT INTO mrsmith.llm_model (app, scope, provider_id, name, model, supports_tools, supports_json_mode, is_default)
SELECT 'binocolo', m.scope, '00000000-0000-0000-0000-00000000a001'::uuid, m.name, m.model, true, true, m.is_default
FROM binocolo.llm_model m
ON CONFLICT (app, scope, model) DO NOTHING;

-- Binocolo prompts — copied faithfully from the live binocolo.llm_prompt (same DB).
INSERT INTO mrsmith.llm_prompt (app, scope, name, prompt, is_default)
SELECT 'binocolo', p.scope, p.name, p.prompt, p.is_default
FROM binocolo.llm_prompt p
ON CONFLICT (app, scope, name) DO NOTHING;

-- Manutenzioni models — literal seed (maintenance.llm_model lives on another DB,
-- cannot be SELECTed here). Mirrors docs/manutenzioni_migrations/002_llm_models.sql.
-- Its system prompt is a Go constant today and is intentionally NOT seeded — it
-- moves to llm_prompt at backend cutover.
INSERT INTO mrsmith.llm_model (app, scope, provider_id, name, model, supports_tools, supports_json_mode, is_default) VALUES
  ('manutenzioni', 'default',          '00000000-0000-0000-0000-00000000a001', 'Gemini Flash Lite', 'google/gemini-2.5-flash-lite-preview-06-17', true, true, true),
  ('manutenzioni', 'assistance_draft', '00000000-0000-0000-0000-00000000a001', 'Gemini Flash Lite', 'google/gemini-2.5-flash-lite-preview-06-17', true, true, true)
ON CONFLICT (app, scope, model) DO NOTHING;

-- Portal-wide default model — final fallback tier for any app/scope without its
-- own default. Points to OpenRouter + Gemini Flash Lite.
INSERT INTO mrsmith.llm_model (app, scope, provider_id, name, model, supports_tools, supports_json_mode, is_default) VALUES
  ('_global', 'default', '00000000-0000-0000-0000-00000000a001', 'Gemini Flash Lite (portal default)', 'google/gemini-2.5-flash-lite-preview-09-2025', true, true, true)
ON CONFLICT (app, scope, model) DO NOTHING;

COMMIT;
