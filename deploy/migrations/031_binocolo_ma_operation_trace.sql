-- Binocolo M&A operation traces.
-- Target database: Anisetta PostgreSQL.

BEGIN;

CREATE SCHEMA IF NOT EXISTS binocolo;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS binocolo.ma_operation_trace (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  request_id           text NOT NULL DEFAULT '',
  operation            text NOT NULL CHECK (btrim(operation) <> ''),
  method               text NOT NULL DEFAULT '',
  path                 text NOT NULL DEFAULT '',
  status               text NOT NULL DEFAULT 'running',
  session_id           uuid REFERENCES binocolo.ma_session(id) ON DELETE SET NULL,
  strategy_version_id  uuid REFERENCES binocolo.ma_strategy_version(id) ON DELETE SET NULL,
  execution_run_id     uuid REFERENCES binocolo.ma_execution_run(id) ON DELETE SET NULL,
  created_by_subject   text NOT NULL DEFAULT '',
  created_by_email     text NOT NULL DEFAULT '',
  request              jsonb NOT NULL DEFAULT '{}'::jsonb,
  http_status          integer,
  error_code           text NOT NULL DEFAULT '',
  error_message        text NOT NULL DEFAULT '',
  started_at           timestamptz NOT NULL DEFAULT now(),
  completed_at         timestamptz,
  duration_ms          integer,
  created_at           timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT ma_operation_trace_status_check
    CHECK (status IN ('running', 'succeeded', 'failed'))
);

CREATE TABLE IF NOT EXISTS binocolo.ma_operation_trace_event (
  id               bigserial PRIMARY KEY,
  trace_id         uuid NOT NULL REFERENCES binocolo.ma_operation_trace(id) ON DELETE CASCADE,
  event_order      integer NOT NULL CHECK (event_order > 0),
  event_type       text NOT NULL CHECK (btrim(event_type) <> ''),
  round            integer,
  tool_name        text NOT NULL DEFAULT '',
  external_system  text NOT NULL DEFAULT '',
  status           text NOT NULL DEFAULT 'info',
  duration_ms      integer,
  request          jsonb NOT NULL DEFAULT '{}'::jsonb,
  response         jsonb NOT NULL DEFAULT '{}'::jsonb,
  metadata         jsonb NOT NULL DEFAULT '{}'::jsonb,
  error            text NOT NULL DEFAULT '',
  created_at       timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT ma_operation_trace_event_status_check
    CHECK (status IN ('info', 'started', 'succeeded', 'failed')),
  CONSTRAINT ma_operation_trace_event_order_unique
    UNIQUE (trace_id, event_order)
);

CREATE INDEX IF NOT EXISTS ma_operation_trace_request_idx
  ON binocolo.ma_operation_trace (request_id)
  WHERE request_id <> '';

CREATE INDEX IF NOT EXISTS ma_operation_trace_session_idx
  ON binocolo.ma_operation_trace (session_id, started_at DESC)
  WHERE session_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ma_operation_trace_operation_status_idx
  ON binocolo.ma_operation_trace (operation, status, started_at DESC);

CREATE INDEX IF NOT EXISTS ma_operation_trace_started_idx
  ON binocolo.ma_operation_trace (started_at DESC);

CREATE INDEX IF NOT EXISTS ma_operation_trace_event_trace_idx
  ON binocolo.ma_operation_trace_event (trace_id, event_order);

COMMIT;
