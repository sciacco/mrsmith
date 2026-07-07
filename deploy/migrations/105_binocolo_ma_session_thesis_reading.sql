-- Binocolo M&A — lettura di tesi per (sessione, azienda).
-- Target: Anisetta PostgreSQL, schema binocolo. Applicata a mano dall'utente.
-- Idempotente: crea il nuovo storage session-scoped e backfilla dalla tabella
-- legacy card-scoped. Le righe legacy senza sessione risolvibile sono saltate.
-- La tabella legacy 096 NON viene droppata: le rotte backend diventano adapter
-- e leggono/scrivono solo binocolo.ma_session_thesis_reading.

BEGIN;

CREATE TABLE IF NOT EXISTS binocolo.ma_session_thesis_reading (
  session_id           uuid NOT NULL REFERENCES binocolo.ma_session(id) ON DELETE CASCADE,
  company_key          text NOT NULL,
  thesis_snapshot      text NOT NULL DEFAULT '',
  reading              jsonb NOT NULL,
  web_evidence_date    timestamptz,
  model_id             uuid,
  prompt_id            uuid,
  generated_by_subject text NOT NULL DEFAULT '',
  generated_by_email   text NOT NULL DEFAULT '',
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),

  PRIMARY KEY (session_id, company_key),
  CONSTRAINT ma_session_thesis_reading_key_not_blank CHECK (btrim(company_key) <> '')
);

INSERT INTO binocolo.ma_session_thesis_reading (
  session_id,
  company_key,
  thesis_snapshot,
  reading,
  web_evidence_date,
  model_id,
  prompt_id,
  generated_by_subject,
  generated_by_email,
  created_at,
  updated_at
)
SELECT
  src.session_id,
  src.company_key,
  src.thesis_snapshot,
  src.reading,
  src.web_evidence_date,
  src.model_id,
  src.prompt_id,
  src.generated_by_subject,
  src.generated_by_email,
  src.created_at,
  src.updated_at
FROM (
  SELECT
    COALESCE(old.session_id, card.created_from_session) AS session_id,
    old.company_key,
    old.thesis_snapshot,
    old.reading,
    old.web_evidence_date,
    old.model_id,
    old.prompt_id,
    old.generated_by_subject,
    old.generated_by_email,
    old.created_at,
    old.updated_at
  FROM binocolo.ma_card_thesis_reading old
  LEFT JOIN binocolo.ma_initiative_card card
    ON card.initiative_id = old.initiative_id
   AND card.company_key = old.company_key
) src
WHERE src.session_id IS NOT NULL
ON CONFLICT (session_id, company_key) DO NOTHING;

COMMIT;
