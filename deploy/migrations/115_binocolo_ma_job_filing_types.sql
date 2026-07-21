-- Binocolo M&A — job type per la pipeline bilanci (issue #78, Fase 1). Aggiunge
-- 'filing_search', 'filing_acquire', 'filing_ingest' al CHECK di ma_job.job_type
-- e gli indici inflight UNICI parziali che serializzano i job per chiave di
-- business (ma_job NON ha colonne di dominio: le chiavi vivono nel payload jsonb).
--
-- Target database: Anisetta PostgreSQL, schema binocolo. Apply after 114.
-- Applicata a mano dall'utente con il suo processo (mai operazioni dirette sui
-- DB in env). Idempotente: DROP CONSTRAINT IF EXISTS + ADD; CREATE INDEX IF NOT
-- EXISTS. La lista base dei 7 job type è quella corrente della mig 111 (l'ultima
-- a toccare ma_job_type_check), estesa coi 3 nuovi tipi.

BEGIN;

ALTER TABLE binocolo.ma_job DROP CONSTRAINT IF EXISTS ma_job_type_check;
ALTER TABLE binocolo.ma_job ADD CONSTRAINT ma_job_type_check
  CHECK (job_type IN (
    'estimate', 'execute', 'web_validation', 'gated_search', 'associate_domain',
    'manual_add', 'card_domain_verify',
    'filing_search', 'filing_acquire', 'filing_ingest'
  ));

-- Stati inflight coerenti coi job sidecar (mig 057/111): pending/processing.
-- Le espressioni sul payload replicano il pattern di ma_job_direct_card_active_idx.

-- filing_search: al più una ricerca inflight per identità fiscale.
CREATE UNIQUE INDEX IF NOT EXISTS ma_job_filing_search_active_idx
  ON binocolo.ma_job (((payload->>'fiscalKey')))
  WHERE job_type = 'filing_search' AND status IN ('pending', 'processing');

-- filing_acquire: dedup business-scoped su (identità fiscale, balanceSheetId) —
-- il filing NON esiste ancora all'enqueue (il job è accodato per acquisitionId),
-- quindi si serializza sulla coppia che identifica il fascicolo richiesto.
CREATE UNIQUE INDEX IF NOT EXISTS ma_job_filing_acquire_active_idx
  ON binocolo.ma_job (((payload->>'fiscalKey')), ((payload->>'balanceSheetId')))
  WHERE job_type = 'filing_acquire' AND status IN ('pending', 'processing');

-- filing_ingest: al più un ingest inflight per filing.
CREATE UNIQUE INDEX IF NOT EXISTS ma_job_filing_ingest_active_idx
  ON binocolo.ma_job (((payload->>'filingId')))
  WHERE job_type = 'filing_ingest' AND status IN ('pending', 'processing');

COMMIT;
