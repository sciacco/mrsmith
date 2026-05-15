-- RDF creation notifications on Anisetta.
-- Apply manually on the database referenced by ANISETTA_DSN after
-- deploy/migrations/009_anisetta_rdf_comments.sql.

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
  'rdf_richiesta_created',
  'richieste-fattibilita',
  'Nuova RDF',
  'è stata inserita una nuova richiesta di fattibilita.',
  'info',
  '{
    "portal": {
      "enabled": true
    },
    "email": {
      "enabled": true,
      "steps": []
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
