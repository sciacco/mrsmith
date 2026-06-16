-- Binocolo M&A scoring v2: graded ranking, confidence axis, flags, legal-form acquisition tiers.
-- Target database: Anisetta PostgreSQL.
-- Apply after 032. Additive: ma_target / ma_evidence gain columns; company_legal_forms gains a tier.
-- match_state is kept (derived from confidence) for backward compatibility.

BEGIN;

-- Per-target confidence axis (how much of the intended signal set was evaluable).
ALTER TABLE binocolo.ma_target
  ADD COLUMN IF NOT EXISTS confidence text;

ALTER TABLE binocolo.ma_target
  DROP CONSTRAINT IF EXISTS ma_target_confidence_check;
ALTER TABLE binocolo.ma_target
  ADD CONSTRAINT ma_target_confidence_check
  CHECK (confidence IS NULL OR confidence IN ('alta', 'media', 'bassa'));

-- Per-target non-scoring annotations (deal profile + warnings). Array of {code,label,severity}.
ALTER TABLE binocolo.ma_target
  ADD COLUMN IF NOT EXISTS flags jsonb NOT NULL DEFAULT '[]'::jsonb;

-- Evidence rows gain the graded breakdown: family + normalized points + applied weight.
ALTER TABLE binocolo.ma_evidence
  ADD COLUMN IF NOT EXISTS family text;
ALTER TABLE binocolo.ma_evidence
  ADD COLUMN IF NOT EXISTS points numeric;
ALTER TABLE binocolo.ma_evidence
  ADD COLUMN IF NOT EXISTS weight numeric;

-- Acquisition tier for the legal-form ranking signal (1 = cleanest to acquire, 4 = rarely a target).
ALTER TABLE binocolo.company_legal_forms
  ADD COLUMN IF NOT EXISTS acquisition_tier integer NOT NULL DEFAULT 4;

ALTER TABLE binocolo.company_legal_forms
  DROP CONSTRAINT IF EXISTS company_legal_forms_acquisition_tier_check;
ALTER TABLE binocolo.company_legal_forms
  ADD CONSTRAINT company_legal_forms_acquisition_tier_check
  CHECK (acquisition_tier BETWEEN 1 AND 4);

-- T1 — capital companies, transferable shares/quotas.
UPDATE binocolo.company_legal_forms
  SET acquisition_tier = 1
  WHERE code IN ('SP', 'AU', 'SR', 'SU', 'RR', 'RS', 'SD', 'SV');

-- T2 — partnerships (acquirable, partner liability, no clean shares).
UPDATE binocolo.company_legal_forms
  SET acquisition_tier = 2
  WHERE code IN ('SN', 'AS', 'AA', 'SE', 'SF', 'SI');

-- T3 — individual / natural person (asset deal, not quotas).
UPDATE binocolo.company_legal_forms
  SET acquisition_tier = 3
  WHERE code IN ('DI', 'PF', 'IF', 'CE');

-- Everything else (co-ops, consortia, public bodies, foundations, foreign/other) stays T4 by default.
UPDATE binocolo.company_legal_forms
  SET acquisition_tier = 4
  WHERE acquisition_tier IS NULL;

COMMIT;
