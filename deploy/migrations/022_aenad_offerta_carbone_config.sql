-- Runtime config for the aenad offer PDF Carbone template.
-- Carbone assigns a new template id on every upload, so the id lives in
-- mrsmith.runtime_config (read per request) instead of an env var that would
-- require an application restart. Apply manually on the database referenced
-- by ANISETTA_DSN.

BEGIN;

INSERT INTO mrsmith.runtime_config (namespace, key, value, description)
VALUES (
  'aenad',
  'carbone_offerta',
  '{"template_id": "59e783c74239854856f6af5e27dd0eb996bdbe2b576cab683675fc5f23abf74f"}'::jsonb,
  'Carbone template id for aenad offer PDFs (regenerated via scripts/carbone/aenad-offerta/).'
)
ON CONFLICT (namespace, key) DO NOTHING;

COMMIT;
