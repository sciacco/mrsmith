-- Persist low-volume backend warning/error diagnostics for support debugging.
-- Apply manually on the database referenced by ANISETTA_DSN.

BEGIN;

CREATE SCHEMA IF NOT EXISTS mrsmith;

CREATE TABLE IF NOT EXISTS mrsmith.diagnostic_event (
  id bigserial PRIMARY KEY,
  observed_at timestamptz NOT NULL DEFAULT now(),
  level text NOT NULL,
  message text NOT NULL DEFAULT '',
  component text NOT NULL DEFAULT '',
  operation text NOT NULL DEFAULT '',
  request_id text NOT NULL DEFAULT '',
  method text NOT NULL DEFAULT '',
  path text NOT NULL DEFAULT '',
  status integer,
  auth_subject text NOT NULL DEFAULT '',
  error text NOT NULL DEFAULT '',
  source_file text NOT NULL DEFAULT '',
  source_line integer NOT NULL DEFAULT 0,
  source_function text NOT NULL DEFAULT '',
  attrs jsonb NOT NULL DEFAULT '{}'::jsonb,
  stack text NOT NULL DEFAULT '',
  dropped_before bigint NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT diagnostic_event_level_check
    CHECK (level IN ('WARN', 'ERROR'))
);

CREATE INDEX IF NOT EXISTS diagnostic_event_observed_idx
  ON mrsmith.diagnostic_event (observed_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS diagnostic_event_level_observed_idx
  ON mrsmith.diagnostic_event (level, observed_at DESC);

CREATE INDEX IF NOT EXISTS diagnostic_event_component_observed_idx
  ON mrsmith.diagnostic_event (component, observed_at DESC);

CREATE INDEX IF NOT EXISTS diagnostic_event_request_idx
  ON mrsmith.diagnostic_event (request_id)
  WHERE request_id <> '';

CREATE INDEX IF NOT EXISTS diagnostic_event_path_observed_idx
  ON mrsmith.diagnostic_event (path, observed_at DESC)
  WHERE path <> '';

COMMIT;
