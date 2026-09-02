-- 144_smartpassive_sdi_invoice.sql
-- Fatture elettroniche ricevute dallo SDI attraverso Fatture in Cloud, conservate
-- intere (XML) con le sole chiavi di testata che servono ad agganciarle alla
-- registrazione Alyante (partita IVA del fornitore, numero e data documento) e a
-- non caricarle due volte (identificativo del documento in Fatture in Cloud).
-- Riferimenti d'ordine, periodi di competenza e righe vengono letti dall'XML dal
-- backend. Idempotente.

BEGIN;

CREATE SCHEMA IF NOT EXISTS smartpassive;

CREATE TABLE IF NOT EXISTS smartpassive.sdi_invoice (
    id              bigserial PRIMARY KEY,
    source_id       bigint NOT NULL,
    file_name       text NOT NULL,
    supplier_vat    text NOT NULL,
    supplier_name   text NOT NULL,
    document_type   text NOT NULL,
    document_number text NOT NULL,
    document_date   date,
    total_amount    numeric(18, 2),
    xml             text NOT NULL,
    imported_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source_id)
);

CREATE INDEX IF NOT EXISTS sdi_invoice_link_idx
    ON smartpassive.sdi_invoice (supplier_vat, document_number, document_date);

COMMIT;
