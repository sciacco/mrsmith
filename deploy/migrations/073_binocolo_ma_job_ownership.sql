-- Binocolo ma_job — OWNERSHIP per la coda condivisa (step 0 gated-search pipeline).
--
-- Problema: binocolo.ma_job vive nel DB Anisetta CONDIVISO. Il lease per-riga
-- (lease_until/locked_by) impedisce a due worker di elaborare la STESSA riga
-- insieme, ma NON impedisce a un worker estraneo (istanza dev/staging altrui,
-- magari con codice vecchio) di ACQUISIRE il lease e girare il job col proprio
-- codice → le ricorrenti "doppie esecuzioni". Su una pipeline che spende davvero
-- (Address + scrape + Advanced) una doppia esecuzione = doppi soldi.
--
-- Fix (parte 1/2, schema): una colonna `owner` stabile. L'enqueue (lato Go)
-- stampa owner = InstanceOwner (default hostname) e PRE-LEASA già la riga a
-- quell'owner (locked_by = owner, lease_until = now()+lease). Così, senza che il
-- worker vecchio sappia nulla di `owner`, il SUO predicato di claim immutato
-- (lease_until < now() OR locked_by = <suo uuid>) risulta falso e salta la riga.
-- Il claim del worker nuovo filtra su owner e fa CAS del lease sul proprio uuid
-- (parte 2/2, in codice: EnqueueMAJob / ListMAJobs / AcquireMAJobLease).
--
-- Le righe pre-migrazione hanno owner NULL: il codice nuovo le tratta come
-- legacy (branch `owner IS NULL`), quindi restano processabili.
--
-- Target database: Anisetta PostgreSQL (schema binocolo). Idempotente.
-- Apply after 072.

BEGIN;

ALTER TABLE binocolo.ma_job
  ADD COLUMN IF NOT EXISTS owner text;

-- Nuovo tipo di job: la gated-search pipeline (surface → address → gate UC2 →
-- advanced+score sui sopravvissuti) gira come singolo job multi-stadio.
ALTER TABLE binocolo.ma_job DROP CONSTRAINT IF EXISTS ma_job_type_check;
ALTER TABLE binocolo.ma_job ADD CONSTRAINT ma_job_type_check
  CHECK (job_type IN ('estimate', 'execute', 'web_validation', 'gated_search'));

-- Hot path del worker nuovo: righe pendenti del proprio owner (le legacy owner
-- NULL restano coperte dall'indice pending esistente e dal branch owner IS NULL).
CREATE INDEX IF NOT EXISTS ma_job_pending_owner_idx
  ON binocolo.ma_job (owner, status, updated_at)
  WHERE status IN ('queued', 'running', 'pending', 'processing');

COMMIT;
