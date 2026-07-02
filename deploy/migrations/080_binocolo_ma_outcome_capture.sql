-- Binocolo M&A — cattura esiti: la ground truth per ogni calibrazione futura dello scoring.
--
-- Ogni giudizio dell'analista è un dato che oggi va perso:
--  1. ma_target_rating impara PERCHÉ un'azienda è stata esclusa (reason, compilato
--     sull'esclusione) e fotografa cosa mostrava la UI al momento del giudizio
--     (score_at_rating / confidence_at_rating, inviati dal client: sono
--     letteralmente ciò che l'analista vedeva). Quando lo scoring cambierà
--     (score_version, migrazione 081), il giudizio resterà interpretabile.
--  2. ma_target_outcome è il log append-only degli esiti reali a valle
--     (contattato / buon lead / no go), agganciato a company_key come il rating,
--     così sopravvive ai re-run della sessione.
--
-- Target database: Anisetta PostgreSQL (schema binocolo). Idempotente. Apply after 079.

BEGIN;

ALTER TABLE binocolo.ma_target_rating
  ADD COLUMN IF NOT EXISTS reason text,
  ADD COLUMN IF NOT EXISTS score_at_rating smallint,
  ADD COLUMN IF NOT EXISTS confidence_at_rating text;

CREATE TABLE IF NOT EXISTS binocolo.ma_target_outcome (
  id                  uuid PRIMARY KEY,
  session_id          uuid NOT NULL REFERENCES binocolo.ma_session(id) ON DELETE CASCADE,
  company_key         text NOT NULL,
  event               text NOT NULL,
  note                text,
  created_by_subject  text,
  created_by_email    text,
  created_at          timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT ma_target_outcome_company_key_not_blank CHECK (btrim(company_key) <> ''),
  CONSTRAINT ma_target_outcome_event_check CHECK (event IN ('contattato', 'buon_lead', 'no_go'))
);

CREATE INDEX IF NOT EXISTS ma_target_outcome_session_company_idx
  ON binocolo.ma_target_outcome (session_id, company_key);

COMMIT;
