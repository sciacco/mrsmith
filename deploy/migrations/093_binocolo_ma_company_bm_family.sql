-- Binocolo M&A — famiglia di business model per azienda (Fase 3 del redesign
-- deep-dive, DEEP-DIVE-IMPLEMENTATION-PLAN.md). Target: Anisetta PostgreSQL,
-- schema binocolo. Applicata a mano dall'utente. Idempotente.
--
-- UNA classificazione, DUE consumatori: la famiglia alimenta le soglie RAG del
-- motore deep e la selezione della riga Damodaran (mig 094). NON vive in
-- ma_company_fact: la mig 090 ratifica che i fact sono presentation-only e
-- "NEVER read by gate/routing/scoring" — la famiglia è invece un input
-- analitico. Pattern: ma_company_domain (mig 082), company_key PK + snapshot.
--
-- suggested_* = proposta deterministica del motore (ATECO univoco o struttura
-- dei costi CE, con evidenza testuale); family = ratifica/override dell'analista
-- (NULL finché non ratificata). La famiglia EFFETTIVA usata dal motore è
-- family se ratificata, altrimenti suggested_family (con caveat in valuation).

BEGIN;

CREATE TABLE IF NOT EXISTS binocolo.ma_company_bm_family (
  company_key         text PRIMARY KEY,
  vat_code            text NOT NULL DEFAULT '',
  tax_code            text NOT NULL DEFAULT '',
  company_name        text NOT NULL DEFAULT '',

  suggested_family    text,
  suggested_source    text,
  suggested_evidence  text NOT NULL DEFAULT '',

  family              text,
  ratified_by_subject text,
  ratified_by_email   text,
  ratified_at         timestamptz,

  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT ma_bm_family_key_not_blank CHECK (btrim(company_key) <> ''),
  CONSTRAINT ma_bm_family_suggested_check CHECK (suggested_family IS NULL OR suggested_family IN
    ('servizi_ricorrenti', 'progetto_integrazione', 'rivendita_var', 'software_prodotto')),
  CONSTRAINT ma_bm_family_family_check CHECK (family IS NULL OR family IN
    ('servizi_ricorrenti', 'progetto_integrazione', 'rivendita_var', 'software_prodotto')),
  CONSTRAINT ma_bm_family_source_check CHECK (suggested_source IS NULL OR suggested_source IN
    ('ateco', 'cost_structure'))
);

COMMIT;
