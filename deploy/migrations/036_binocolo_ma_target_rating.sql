-- Binocolo Target M&A — veryshort: rating preferiti (per azienda, per sessione).
-- Target database: Anisetta PostgreSQL. Apply after 035.
-- Il voto è agganciato all'identità azienda (company_key = vat>tax>vendor>nome,
-- vedi maTargetDedupeKey), NON a ma_target.id che viene ricreato a ogni execute.
-- Sopravvive quindi ai re-run della sessione.
--   rating: -1 = escluso, 1..3 = stelle. L'assenza di riga = non valutato (0).

BEGIN;

CREATE TABLE IF NOT EXISTS binocolo.ma_target_rating (
  session_id        uuid NOT NULL REFERENCES binocolo.ma_session(id) ON DELETE CASCADE,
  company_key       text NOT NULL,
  rating            smallint NOT NULL,
  rated_by_subject  text,
  rated_by_email    text,
  rated_at          timestamptz NOT NULL DEFAULT now(),

  PRIMARY KEY (session_id, company_key),
  CONSTRAINT ma_target_rating_company_key_not_blank CHECK (btrim(company_key) <> ''),
  CONSTRAINT ma_target_rating_value_check CHECK (rating IN (-1, 1, 2, 3))
);

COMMIT;
