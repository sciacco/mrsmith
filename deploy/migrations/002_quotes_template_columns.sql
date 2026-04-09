-- Quotes template: add business-rule columns for new app.
-- Safe for Appsmith coexistence: all columns are nullable with defaults.
-- Appsmith reads only template_id, description, lang — ignores new columns.

BEGIN;

ALTER TABLE quotes.template
  ADD COLUMN IF NOT EXISTS template_type varchar(16) DEFAULT 'standard',
  ADD COLUMN IF NOT EXISTS kit_id bigint REFERENCES products.kit(id),
  ADD COLUMN IF NOT EXISTS service_category_id integer,
  ADD COLUMN IF NOT EXISTS is_colo boolean DEFAULT false,
  ADD COLUMN IF NOT EXISTS is_active boolean DEFAULT true;

-- Idempotent 13-row registry seed.
-- Uses INSERT ON CONFLICT so missing rows are created and existing rows are updated.
-- Source: quotes-migspec-phaseA.md "Full template registry" table.

INSERT INTO quotes.template (template_id, description, lang, template_type, kit_id, service_category_id, is_colo, is_active) VALUES
  ('105348827359', 'vecchio',          'it', 'legacy',   NULL, NULL, false, false),
  ('111577899484', 'COLO IT',          'it', 'standard', NULL, NULL, true,  true),
  ('111583049949', 'COLO EN',          'en', 'standard', NULL, NULL, true,  true),
  ('111583627969', 'NON COLO IT',      'it', 'standard', NULL, NULL, false, true),
  ('111583628251', 'NON COLO EN',      'en', 'standard', NULL, NULL, false, true),
  ('850825381069', 'IaaS Diretta EN',  'en', 'iaas',     62,   12,   false, true),
  ('853027287235', 'IaaS Diretta IT',  'it', 'iaas',     62,   12,   false, true),
  ('853237903587', 'VCLOUD IaaS IT',   'it', 'iaas',     116,  14,   false, true),
  ('853320143046', 'IaaS Indiretta EN','en', 'iaas',     63,   13,   false, true),
  ('853500178641', 'IaaS Indiretta IT','it', 'iaas',     63,   13,   false, true),
  ('853500899556', 'VCLOUD IaaS EN',   'en', 'iaas',     116,  14,   false, true),
  ('855439340792', 'VCLOUD DRaaS EN',  'en', 'iaas',     119,  15,   false, true),
  ('856380863697', 'VCLOUD DRaaS IT',  'it', 'iaas',     119,  15,   false, true)
ON CONFLICT (template_id) DO UPDATE SET
  template_type       = EXCLUDED.template_type,
  kit_id              = EXCLUDED.kit_id,
  service_category_id = EXCLUDED.service_category_id,
  is_colo             = EXCLUDED.is_colo,
  is_active           = EXCLUDED.is_active;

-- Hard verification gate: fail the transaction if the registry is incomplete.
DO $$
DECLARE
  row_count integer;
BEGIN
  SELECT count(*) INTO row_count FROM quotes.template;
  IF row_count <> 13 THEN
    RAISE EXCEPTION 'Template registry incomplete: expected 13 rows, got %', row_count;
  END IF;
END $$;

COMMIT;
