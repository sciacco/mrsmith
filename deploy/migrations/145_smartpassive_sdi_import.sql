-- 145_smartpassive_sdi_import.sql
-- Importazione periodica delle fatture SDI da Fatture in Cloud nel backend.
-- Alla tabella degli XML si aggiunge la data di ricezione, che e' la data su cui
-- l'API filtra l'elenco e da cui ogni giro riparte (ricezione massima presente,
-- meno la sovrapposizione). Il pregresso caricato dal CLI resta senza ricezione
-- finche' la prima passata completa non la riempie.
-- La tabella delle esecuzioni registra ogni giro, riuscito o no: mode =
-- incremental | full, outcome = ok | failed | stopped (arresto per riserva
-- oraria dell'API). In details va la diagnostica completa del giro, senza
-- omissioni: finestra letta, pagine e richieste API con i residui orario e
-- mensile, attese per limiti o errori di rete, documenti elencati, inseriti,
-- gia' presenti e saltati con identificativo, nome file e motivo, errore che
-- ha fermato il giro. Idempotente.

BEGIN;

ALTER TABLE smartpassive.sdi_invoice
    ADD COLUMN IF NOT EXISTS received_on date;

CREATE INDEX IF NOT EXISTS sdi_invoice_received_on_idx
    ON smartpassive.sdi_invoice (received_on);

CREATE TABLE IF NOT EXISTS smartpassive.sdi_import_run (
    id           bigserial PRIMARY KEY,
    started_at   timestamptz NOT NULL DEFAULT now(),
    finished_at  timestamptz,
    mode         text NOT NULL,
    outcome      text,
    inserted     integer NOT NULL DEFAULT 0,
    details      jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS sdi_import_run_started_at_idx
    ON smartpassive.sdi_import_run (started_at DESC);

COMMIT;
