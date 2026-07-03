-- Binocolo M&A — lettura di tesi context-scoped sulla card (Fase 5 del redesign
-- deep-dive, DEEP-DIVE-IMPLEMENTATION-PLAN.md). Target: Anisetta PostgreSQL,
-- schema binocolo. Applicata a mano dall'utente. Idempotente.
--
-- Il dossier deep resta thesis-neutral e globale (cache per company_key); la
-- lettura di tesi è il secondo strato — il "memo" per (iniziativa, azienda) —
-- ancorato alla TESI DELLA SESSIONE DI PROVENIENZA (provenienza col rating più
-- recente, fallback created_from_session; la tesi NON vive sull'iniziativa,
-- che resta un contenitore pratico). Generazione ON-DEMAND (pattern B6),
-- rigenerazione esplicita: thesis_snapshot serve al confronto di staleness con
-- la tesi corrente della sessione.

BEGIN;

CREATE TABLE IF NOT EXISTS binocolo.ma_card_thesis_reading (
  initiative_id       uuid NOT NULL REFERENCES binocolo.ma_initiative(id) ON DELETE CASCADE,
  company_key         text NOT NULL,
  session_id          uuid,
  thesis_snapshot     text NOT NULL DEFAULT '',
  reading             jsonb NOT NULL,
  web_evidence_date   timestamptz,
  model_id            uuid,
  prompt_id           uuid,
  generated_by_subject text NOT NULL DEFAULT '',
  generated_by_email  text NOT NULL DEFAULT '',
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),

  PRIMARY KEY (initiative_id, company_key),
  CONSTRAINT ma_thesis_reading_key_not_blank CHECK (btrim(company_key) <> '')
);

COMMIT;
