-- Binocolo Target M&A — UC2 sector-classification eval ground-truth labels.
-- Target database: Anisetta PostgreSQL. Apply after 065.
-- Human ground-truth label per (session, company) for the sector-eval harness.
-- Kept SEPARATE from the (re-runnable) web-validation prediction so it survives
-- KB changes and session re-validations. Keyed by company_key like ma_target_rating,
-- so it survives a session re-execute.
-- CORRECTION 2026-07-25 (issue #81): this comment used to read "company_key
-- (vat>tax>vendor>nome)". maTargetDedupeKey's real precedence is the reverse —
-- vendor_id > P.IVA > codice fiscale > company name — and on real data only the
-- first branch has ever fired (1,583 of 1,584 keys are the OpenAPI.it ObjectId,
-- zero are the P.IVA). Do NOT "fix" the code to match the old comment: flipping the
-- precedence would re-key the whole corpus and orphan every row keyed on company_key.
--   label: keep = nel settore-obiettivo; forse = incerto/da rivedere; scarta = off-target.
--   L'assenza di riga = non etichettato.

BEGIN;

CREATE TABLE IF NOT EXISTS binocolo.ma_sector_eval_label (
  session_id         uuid NOT NULL REFERENCES binocolo.ma_session(id) ON DELETE CASCADE,
  company_key        text NOT NULL,
  label              text NOT NULL,
  note               text,
  labeled_by_subject text,
  labeled_by_email   text,
  updated_at         timestamptz NOT NULL DEFAULT now(),

  PRIMARY KEY (session_id, company_key),
  CONSTRAINT ma_sector_eval_label_company_key_not_blank CHECK (btrim(company_key) <> ''),
  CONSTRAINT ma_sector_eval_label_value_check CHECK (label IN ('keep', 'forse', 'scarta'))
);

COMMIT;
