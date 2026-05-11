-- RDA comment mention notifications on Anisetta.
-- Apply manually on the database referenced by ANISETTA_DSN after
-- deploy/migrations/006_anisetta_mrsmith_notifications.sql.

BEGIN;

INSERT INTO mrsmith.notification_type (
  type_key,
  app_id,
  title_template,
  body_template,
  severity,
  default_policy
)
VALUES (
  'rda_comment_mention',
  'rda',
  'RDA comment mention',
  'You were mentioned in an RDA comment.',
  'info',
  '{
    "portal": {
      "enabled": true
    },
    "email": {
      "enabled": true,
      "steps": [
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
