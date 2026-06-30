-- Binocolo Target M&A — UC2 sector-classification eval ground-truth labels.
-- Target database: Anisetta PostgreSQL. Apply after 065.
-- Human ground-truth label per (session, company) for the sector-eval harness.
-- Kept SEPARATE from the (re-runnable) web-validation prediction so it survives
-- KB changes and session re-validations. Keyed by company_key (vat>tax>vendor>nome,
-- see maTargetDedupeKey) like ma_target_rating, so it survives a session re-execute.
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
