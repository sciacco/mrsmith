-- Binocolo M&A — vintage dei payload IT-full (Fase 0 del redesign deep-dive,
-- DEEP-DIVE-IMPLEMENTATION-PLAN.md). Target: Anisetta PostgreSQL, schema
-- binocolo. Applicata a mano dall'utente. Idempotente (doppia apply ok).
--
-- Problema: ma_deep_analysis tiene UN solo itfull_payload per company_key e il
-- worker lo SOVRASCRIVE a ogni refresh — ogni bilancio nuovo distruggerebbe il
-- precedente. Ogni payload porta un solo esercizio di dettaglio CEE (le
-- divisioni IIC/IPL non sono anni), quindi l'archivio pluriennale si costruisce
-- SOLO conservando una riga per (azienda, data bilancio): ~€0.30/azienda/anno.
--
-- Chiave = data di chiusura bilancio presa LETTERALMENTE dalla stringa vendor
-- (left(...,10)): niente cast timestamptz, che con offset +01:00 e sessione UTC
-- sposterebbe il 31/12 al 30/12.
--
-- Il backfill mette al sicuro le vintage già in cache PRIMA di qualsiasi
-- refresh futuro; righe senza data bilancio leggibile vengono saltate (il
-- payload resta comunque in ma_deep_analysis). fetched_at usa updated_at come
-- proxy dell'epoca di acquisto (è l'istante del save ready; le manutenzioni
-- successive lo toccano, ma per vintage già uniche per data resta indicativo).

BEGIN;

CREATE TABLE IF NOT EXISTS binocolo.ma_deep_payload_vintage (
  company_key        text NOT NULL,
  balance_sheet_date date NOT NULL,
  turnover_year      integer,
  payload            jsonb NOT NULL,
  fetched_at         timestamptz NOT NULL DEFAULT now(),

  PRIMARY KEY (company_key, balance_sheet_date),
  CONSTRAINT ma_deep_vintage_company_key_not_blank CHECK (btrim(company_key) <> '')
);

INSERT INTO binocolo.ma_deep_payload_vintage
  (company_key, balance_sheet_date, turnover_year, payload, fetched_at)
SELECT
  company_key,
  left(COALESCE(itfull_payload->'data'->'ecofin'->>'balanceSheetDate',
                itfull_payload->'ecofin'->>'balanceSheetDate'), 10)::date,
  NULLIF(COALESCE(itfull_payload->'data'->'ecofin'->>'turnoverYear',
                  itfull_payload->'ecofin'->>'turnoverYear'), '')::numeric::integer,
  itfull_payload,
  updated_at
FROM binocolo.ma_deep_analysis
WHERE status = 'ready'
  AND itfull_payload IS NOT NULL
  AND itfull_payload <> 'null'::jsonb
  AND COALESCE(itfull_payload->'data'->'ecofin'->>'balanceSheetDate',
               itfull_payload->'ecofin'->>'balanceSheetDate') ~ '^\d{4}-\d{2}-\d{2}'
ON CONFLICT (company_key, balance_sheet_date) DO NOTHING;

COMMIT;
