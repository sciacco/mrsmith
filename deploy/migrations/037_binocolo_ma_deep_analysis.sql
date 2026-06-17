-- Binocolo Target M&A — veryshort: analisi approfondita (IT-full).
-- Target database: Anisetta PostgreSQL. Apply after 036.
-- Cache GLOBALE per azienda (company_key), condivisa tra sessioni: i bilanci non
-- cambiano con la tesi, quindi la stessa azienda promossa in due ricerche paga
-- IT-full una volta sola. Stato durevole per il resume del worker dopo un restart.
--   status: queued -> running -> ready | failed

BEGIN;

CREATE TABLE IF NOT EXISTS binocolo.ma_deep_analysis (
  company_key        text PRIMARY KEY,
  vat_code           text,
  tax_code           text,
  status             text NOT NULL,
  vendor_request_id  text,
  attempts           integer NOT NULL DEFAULT 0,
  itfull_payload     jsonb,
  scorecard          jsonb,
  valuation          jsonb,
  brief              jsonb,
  cost_eur           numeric NOT NULL DEFAULT 0,
  model_id           uuid REFERENCES binocolo.llm_model(id) ON DELETE SET NULL,
  prompt_id          uuid REFERENCES binocolo.llm_prompt(id) ON DELETE SET NULL,
  error_code         text,
  requested_at       timestamptz,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  refreshed_by_email text,

  CONSTRAINT ma_deep_analysis_company_key_not_blank CHECK (btrim(company_key) <> ''),
  CONSTRAINT ma_deep_analysis_status_check CHECK (status IN ('queued', 'running', 'ready', 'failed'))
);

-- Hot path for the worker: pending rows it must advance.
CREATE INDEX IF NOT EXISTS ma_deep_analysis_pending_idx
  ON binocolo.ma_deep_analysis (status, updated_at)
  WHERE status IN ('queued', 'running');

COMMIT;
