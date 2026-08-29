-- Training: attestati e documenti salvati nel database.
-- Sostituisce lo storage su filesystem locale del POC: i byte dei file vivono
-- in training.document_blob, indirizzati dalla stessa storage key gia' usata
-- da training.document.storage_key. Nessun backfill: il dominio e' vuoto
-- post-129 e non esistono file da migrare. Nessuna foreign key verso
-- training.document: lo storage resta un deposito chiave->contenuto con il
-- ciclo di vita gestito dal backend (Put prima della riga documento, Delete
-- alla rimozione), come l'adapter che sostituisce.
--
-- Delivered as a file only. The Agent does NOT apply it and does NOT connect
-- to any database configured in the env files.

BEGIN;

CREATE TABLE IF NOT EXISTS training.document_blob (
  storage_key text PRIMARY KEY,
  content bytea NOT NULL,
  sha256 text NOT NULL,
  size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
  created_at timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE training.document_blob IS
  'Contenuto binario di attestati e documenti Training: chiave = training.document.storage_key.';

COMMIT;
