-- MrSmith-owned runtime configuration and support requests on Anisetta.
-- Apply manually on the database referenced by ANISETTA_DSN before enabling
-- contextual support requests.

BEGIN;

CREATE SCHEMA IF NOT EXISTS mrsmith;

CREATE TABLE IF NOT EXISTS mrsmith.runtime_config (
  namespace text NOT NULL,
  key text NOT NULL,
  value jsonb NOT NULL,
  description text NOT NULL DEFAULT '',
  is_sensitive boolean NOT NULL DEFAULT false,
  updated_by text NOT NULL DEFAULT '',
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (namespace, key)
);

CREATE OR REPLACE FUNCTION mrsmith.set_runtime_config_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS runtime_config_set_updated_at ON mrsmith.runtime_config;
CREATE TRIGGER runtime_config_set_updated_at
BEFORE UPDATE ON mrsmith.runtime_config
FOR EACH ROW
EXECUTE FUNCTION mrsmith.set_runtime_config_updated_at();

INSERT INTO mrsmith.runtime_config (namespace, key, value, description)
VALUES (
  'support',
  'notification.email_to',
  '[]'::jsonb,
  'Email recipients for contextual support request notifications.'
)
ON CONFLICT (namespace, key) DO NOTHING;

CREATE TABLE IF NOT EXISTS mrsmith.support_request (
  id bigserial PRIMARY KEY,
  status text NOT NULL DEFAULT 'open',
  priority text NOT NULL DEFAULT 'normal',
  app_id text NOT NULL,
  app_name text NOT NULL DEFAULT '',
  page_url text NOT NULL DEFAULT '',
  page_path text NOT NULL DEFAULT '',
  message text NOT NULL,
  requester_subject text NOT NULL DEFAULT '',
  requester_name text NOT NULL DEFAULT '',
  requester_email text NOT NULL DEFAULT '',
  requester_roles jsonb NOT NULL DEFAULT '[]'::jsonb,
  technical_context_included boolean NOT NULL DEFAULT true,
  email_notification_status text NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT support_request_status_check
    CHECK (status IN ('open', 'in_progress', 'closed', 'cancelled')),
  CONSTRAINT support_request_priority_check
    CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
  CONSTRAINT support_request_email_status_check
    CHECK (email_notification_status IN ('pending', 'sent', 'skipped', 'failed'))
);

CREATE INDEX IF NOT EXISTS support_request_created_idx
  ON mrsmith.support_request (created_at DESC);

CREATE INDEX IF NOT EXISTS support_request_status_priority_idx
  ON mrsmith.support_request (status, priority, created_at DESC);

CREATE INDEX IF NOT EXISTS support_request_app_idx
  ON mrsmith.support_request (app_id, created_at DESC);

CREATE TABLE IF NOT EXISTS mrsmith.support_request_context (
  request_id bigint PRIMARY KEY
    REFERENCES mrsmith.support_request(id) ON DELETE CASCADE,
  context jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS mrsmith.support_request_event (
  id bigserial PRIMARY KEY,
  request_id bigint NOT NULL
    REFERENCES mrsmith.support_request(id) ON DELETE CASCADE,
  event_type text NOT NULL,
  actor_subject text NOT NULL DEFAULT '',
  actor_name text NOT NULL DEFAULT '',
  actor_email text NOT NULL DEFAULT '',
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS support_request_event_request_idx
  ON mrsmith.support_request_event (request_id, created_at);

COMMIT;
