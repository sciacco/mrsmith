-- MrSmith portal notifications on Anisetta.
-- Apply manually on the database referenced by ANISETTA_DSN before implementing
-- or enabling the notifications backend module.

BEGIN;

CREATE SCHEMA IF NOT EXISTS mrsmith;

CREATE OR REPLACE FUNCTION mrsmith.set_runtime_config_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$;

CREATE TABLE IF NOT EXISTS mrsmith.notification_type (
  type_key text PRIMARY KEY,
  app_id text NOT NULL,
  title_template text NOT NULL,
  body_template text NOT NULL DEFAULT '',
  severity text NOT NULL,
  default_policy jsonb NOT NULL DEFAULT '{}'::jsonb,
  enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT notification_type_severity_check
    CHECK (severity IN ('info', 'success', 'warning', 'critical')),
  CONSTRAINT notification_type_default_policy_object_check
    CHECK (jsonb_typeof(default_policy) = 'object')
);

DROP TRIGGER IF EXISTS notification_type_set_updated_at ON mrsmith.notification_type;
CREATE TRIGGER notification_type_set_updated_at
BEFORE UPDATE ON mrsmith.notification_type
FOR EACH ROW
EXECUTE FUNCTION mrsmith.set_runtime_config_updated_at();

CREATE INDEX IF NOT EXISTS notification_type_app_idx
  ON mrsmith.notification_type (app_id, enabled, type_key);

CREATE TABLE IF NOT EXISTS mrsmith.notification (
  id bigserial PRIMARY KEY,
  type_key text NOT NULL
    REFERENCES mrsmith.notification_type(type_key),
  app_id text NOT NULL,
  severity text NOT NULL,
  title text NOT NULL,
  body text NOT NULL DEFAULT '',
  entity_type text NOT NULL DEFAULT '',
  entity_id text NOT NULL DEFAULT '',
  dedupe_key text NOT NULL,
  deep_link text NOT NULL DEFAULT '',
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  policy_override jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_by_subject text NOT NULL DEFAULT '',
  created_by_email text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT notification_severity_check
    CHECK (severity IN ('info', 'success', 'warning', 'critical')),
  CONSTRAINT notification_metadata_object_check
    CHECK (jsonb_typeof(metadata) = 'object'),
  CONSTRAINT notification_policy_override_object_check
    CHECK (jsonb_typeof(policy_override) = 'object')
);

CREATE UNIQUE INDEX IF NOT EXISTS notification_dedupe_key_idx
  ON mrsmith.notification (dedupe_key);

CREATE INDEX IF NOT EXISTS notification_entity_idx
  ON mrsmith.notification (entity_type, entity_id, created_at DESC);

CREATE INDEX IF NOT EXISTS notification_app_created_idx
  ON mrsmith.notification (app_id, created_at DESC);

CREATE INDEX IF NOT EXISTS notification_type_created_idx
  ON mrsmith.notification (type_key, created_at DESC);

CREATE TABLE IF NOT EXISTS mrsmith.notification_recipient (
  id bigserial PRIMARY KEY,
  notification_id bigint NOT NULL
    REFERENCES mrsmith.notification(id) ON DELETE CASCADE,
  recipient_subject text NOT NULL DEFAULT '',
  recipient_email text NOT NULL,
  recipient_name text NOT NULL DEFAULT '',
  read_at timestamptz,
  archived_at timestamptz,
  resolved_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT notification_recipient_email_not_blank_check
    CHECK (btrim(recipient_email) <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS notification_recipient_unique_idx
  ON mrsmith.notification_recipient (notification_id, lower(recipient_email));

CREATE INDEX IF NOT EXISTS notification_recipient_email_created_idx
  ON mrsmith.notification_recipient (lower(recipient_email), created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS notification_recipient_unread_idx
  ON mrsmith.notification_recipient (lower(recipient_email), created_at DESC, id DESC)
  WHERE read_at IS NULL
    AND archived_at IS NULL
    AND resolved_at IS NULL;

CREATE INDEX IF NOT EXISTS notification_recipient_notification_idx
  ON mrsmith.notification_recipient (notification_id);

CREATE TABLE IF NOT EXISTS mrsmith.notification_delivery (
  id bigserial PRIMARY KEY,
  recipient_id bigint NOT NULL
    REFERENCES mrsmith.notification_recipient(id) ON DELETE CASCADE,
  channel text NOT NULL,
  policy_step text NOT NULL,
  status text NOT NULL,
  due_at timestamptz NOT NULL,
  locked_at timestamptz,
  locked_by text NOT NULL DEFAULT '',
  attempt_count integer NOT NULL DEFAULT 0,
  sent_at timestamptz,
  skipped_at timestamptz,
  failed_at timestamptz,
  last_error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT notification_delivery_channel_check
    CHECK (channel IN ('portal', 'email')),
  CONSTRAINT notification_delivery_status_check
    CHECK (status IN ('pending', 'locked', 'sent', 'skipped', 'failed', 'cancelled')),
  CONSTRAINT notification_delivery_attempt_count_check
    CHECK (attempt_count >= 0),
  CONSTRAINT notification_delivery_policy_step_not_blank_check
    CHECK (btrim(policy_step) <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS notification_delivery_unique_step_idx
  ON mrsmith.notification_delivery (recipient_id, channel, policy_step);

CREATE INDEX IF NOT EXISTS notification_delivery_due_idx
  ON mrsmith.notification_delivery (status, due_at, id);

CREATE INDEX IF NOT EXISTS notification_delivery_pending_due_idx
  ON mrsmith.notification_delivery (due_at, id)
  WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS notification_delivery_recipient_idx
  ON mrsmith.notification_delivery (recipient_id, created_at DESC);

CREATE TABLE IF NOT EXISTS mrsmith.notification_delivery_attempt (
  id bigserial PRIMARY KEY,
  delivery_id bigint NOT NULL
    REFERENCES mrsmith.notification_delivery(id) ON DELETE CASCADE,
  status text NOT NULL,
  error text NOT NULL DEFAULT '',
  attempted_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT notification_delivery_attempt_status_check
    CHECK (status IN ('sent', 'skipped', 'failed', 'cancelled'))
);

CREATE INDEX IF NOT EXISTS notification_delivery_attempt_delivery_idx
  ON mrsmith.notification_delivery_attempt (delivery_id, attempted_at DESC);

INSERT INTO mrsmith.notification_type (
  type_key,
  app_id,
  title_template,
  body_template,
  severity,
  default_policy
)
VALUES (
  'rda_approval_requested',
  'rda',
  'RDA approval requested',
  'A purchase request is waiting for your approval.',
  'warning',
  '{
    "portal": {
      "enabled": true
    },
    "email": {
      "enabled": true,
      "steps": [
        {
          "step": "unread_after_4h",
          "delay": "4h"
        },
        {
          "step": "unread_after_24h",
          "delay": "24h"
        },
        {
          "step": "unread_after_72h",
          "delay": "72h"
        }
      ]
    }
  }'::jsonb
)
ON CONFLICT (type_key) DO UPDATE
SET app_id = EXCLUDED.app_id,
    title_template = EXCLUDED.title_template,
    body_template = EXCLUDED.body_template,
    severity = EXCLUDED.severity,
    default_policy = EXCLUDED.default_policy,
    enabled = true;

COMMIT;
