-- Binocolo Target M&A — veryshort: rating preferiti (per azienda, per sessione).
-- Target database: Anisetta PostgreSQL. Apply after 035.
-- Il voto è agganciato all'identità azienda (company_key), NON a ma_target.id che
-- viene ricreato a ogni execute. Sopravvive quindi ai re-run della sessione.
--   rating: -1 = escluso, 1..3 = stelle. L'assenza di riga = non valutato (0).
--
-- RETTIFICA 2026-07-25 (issue #81): questo commento diceva "company_key =
-- vat>tax>vendor>nome". La precedenza reale di maTargetDedupeKey è l'opposta —
-- vendor_id > P.IVA > codice fiscale > ragione sociale — e sui dati reali solo il
-- primo ramo si è mai attivato (1.583 chiavi su 1.584 sono l'ObjectId OpenAPI.it,
-- zero sono la P.IVA). NON allineare il codice a questo commento: invertire la
-- precedenza ricodificherebbe l'identità di tutto il corpus e orfanerebbe ogni
-- riga qui e nelle altre tabelle che usano company_key come chiave.

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
