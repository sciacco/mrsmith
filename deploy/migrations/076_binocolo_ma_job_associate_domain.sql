-- Binocolo ma_job — nuovo job_type 'associate_domain' (remedy manual_review).
--
-- La remedy "associa dominio" — l'operatore fornisce un dominio ufficiale per un'azienda
-- tenuta in manual_review perché il dominio non era stato risolto — è codice di PRODUZIONE
-- e SPENDE (re-gate scrape + un Advanced €0.10 sull'azienda che sopravvive). Va quindi
-- sulla coda durabile/ownership come la gated-search reale, NON inline: un crash a metà si
-- riprende (retry idempotente, l'Advanced già pagato è riusato via enrichment_level) e la
-- riga pre-leasata all'owner evita il furto-lease del worker estraneo sul DB condiviso.
--
-- Estende il CHECK di ma_job.job_type per ammettere 'associate_domain' (dopo 073, che ha
-- aggiunto 'gated_search').
--
-- Target database: Anisetta PostgreSQL (schema binocolo). Idempotente. Apply after 075.

BEGIN;

ALTER TABLE binocolo.ma_job DROP CONSTRAINT IF EXISTS ma_job_type_check;
ALTER TABLE binocolo.ma_job ADD CONSTRAINT ma_job_type_check
  CHECK (job_type IN ('estimate', 'execute', 'web_validation', 'gated_search', 'associate_domain'));

COMMIT;
