-- MrSmith shared HubSpot async request queue.
-- Target database: Anisetta PostgreSQL.
-- Apply manually on the database referenced by ANISETTA_DSN.
-- Requires 004_anisetta_mrsmith_support.sql (mrsmith schema + mrsmith.runtime_config).
--
-- Companion to 023_raenad_schema.sql (parent #56). HubSpot writes triggered by raenad
-- (and other apps) are persisted here and processed asynchronously with retry/backoff,
-- so saving a quote never depends on a synchronous HubSpot response.
--
-- V1 contract:
--   * On first quote save the backend persists raenad.quote and enqueues
--     operation='hubspot.create_deal' with dedupe_key='raenad:quote:{id}:deal:create'.
--   * Relevant saves after create/update enqueue operation='hubspot.update_deal' with
--     dedupe_key='raenad:quote:{id}:deal:update'. The backend overwrites the pending/failed
--     update payload with the latest quote-owned state (latest wins).
--     Payloads carry the source quote updated_at so the worker can detect stale locked
--     updates and re-arm a latest update when the quote changed during processing.
--   * raenad.quote.hubspot_deal_id stays NULL until the worker completes the request.
--     If the update request runs before the deal id exists, the worker defers it.
--   * dedupe_key is unique, guaranteeing idempotent enqueue.
--   * A reconciler can re-enqueue quotes with hubspot_sync_status='pending' that have no
--     matching live request (e.g. dropped/cancelled), using the same dedupe_key.

BEGIN;

CREATE SCHEMA IF NOT EXISTS mrsmith;

CREATE TABLE IF NOT EXISTS mrsmith.hubspot_request (
  id              bigserial PRIMARY KEY,
  status          text NOT NULL DEFAULT 'pending',
  operation       text NOT NULL,
  entity_type     text NOT NULL,
  entity_id       text NOT NULL,
  dedupe_key      text NOT NULL,
  payload         jsonb NOT NULL DEFAULT '{}'::jsonb,
  response        jsonb,
  attempt_count   integer NOT NULL DEFAULT 0,
  max_attempts    integer NOT NULL DEFAULT 8,
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  locked_at       timestamptz,
  locked_by       text,
  last_error      text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT hubspot_request_dedupe_key_unique UNIQUE (dedupe_key),
  CONSTRAINT hubspot_request_status_check
    CHECK (status IN ('pending', 'locked', 'succeeded', 'failed', 'dead', 'cancelled'))
);

-- Poll index for the worker: due, not-yet-terminal requests.
CREATE INDEX IF NOT EXISTS hubspot_request_due_idx
  ON mrsmith.hubspot_request (next_attempt_at)
  WHERE status = 'pending';

-- Reconciler lookup by source entity.
CREATE INDEX IF NOT EXISTS hubspot_request_entity_idx
  ON mrsmith.hubspot_request (entity_type, entity_id);

CREATE INDEX IF NOT EXISTS hubspot_request_status_idx
  ON mrsmith.hubspot_request (status, next_attempt_at);

CREATE OR REPLACE FUNCTION mrsmith.hubspot_request_set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS hubspot_request_set_updated_at ON mrsmith.hubspot_request;
CREATE TRIGGER hubspot_request_set_updated_at
BEFORE UPDATE ON mrsmith.hubspot_request
FOR EACH ROW
EXECUTE FUNCTION mrsmith.hubspot_request_set_updated_at();

CREATE TABLE IF NOT EXISTS mrsmith.hubspot_request_attempt (
  id             bigserial PRIMARY KEY,
  request_id     bigint NOT NULL REFERENCES mrsmith.hubspot_request(id) ON DELETE CASCADE,
  attempt_number integer NOT NULL,
  status         text NOT NULL,
  started_at     timestamptz NOT NULL DEFAULT now(),
  finished_at    timestamptz,
  http_status    integer,
  error          text,
  response       jsonb
);

CREATE INDEX IF NOT EXISTS hubspot_request_attempt_request_idx
  ON mrsmith.hubspot_request_attempt (request_id, attempt_number);

-- Worker runtime switch (kept disabled until the backend worker ships).
INSERT INTO mrsmith.runtime_config (namespace, key, value, description)
VALUES (
  'raenad',
  'hubspot_queue_worker',
  '{"enabled": false, "interval_seconds": 30, "batch_size": 20, "max_attempts": 8, "backoff_base_seconds": 30, "backoff_max_seconds": 3600}'::jsonb,
  'Controls the shared HubSpot async request queue worker (mrsmith.hubspot_request).'
)
ON CONFLICT (namespace, key) DO NOTHING;

-- HubSpot pipeline used when creating deals for new Aenad quotes.
INSERT INTO mrsmith.runtime_config (namespace, key, value, description)
VALUES (
  'raenad',
  'hubspot_deal_pipeline',
  '{"pipeline_id": "3883934920", "initial_dealstage_id": "5521139911"}'::jsonb,
  'HubSpot pipeline and initial deal stage for new Aenad quote deals.'
)
ON CONFLICT (namespace, key) DO NOTHING;

-- Owner mapping used when creating HubSpot deals for new Aenad quotes.
INSERT INTO mrsmith.runtime_config (namespace, key, value, description)
VALUES (
  'raenad',
  'hubspot_deal_owner',
  '{"fallback_owner_email": "service01@cdlan.it"}'::jsonb,
  'HubSpot owner fallback for new Aenad quote deals when the authenticated user has no matching owner.'
)
ON CONFLICT (namespace, key) DO NOTHING;

-- Default values used when creating/editing new Aenad quote lines.
INSERT INTO mrsmith.runtime_config (namespace, key, value, description)
VALUES (
  'aenad',
  'quote_defaults',
  '{"cod_iva": "22"}'::jsonb,
  'Default values for new Aenad quote lines.'
)
ON CONFLICT (namespace, key) DO NOTHING;

COMMIT;
