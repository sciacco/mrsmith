-- Binocolo M&A — Lettura nota integrativa (issue #78, Fase 1). Run di lettura NI
-- (LLM) su un processing run, proposte di rettifica IMMUTABILI e decisioni
-- analista APPEND-ONLY (ratifica / rifiuto / revoca, con ri-conferma automatica).
--
-- Target database: Anisetta PostgreSQL, schema binocolo. Apply after 113.
-- Applicata a mano dall'utente con il suo processo (mai operazioni dirette sui
-- DB in env). Idempotente: CREATE TABLE/INDEX IF NOT EXISTS.
--
-- Stile allineato a ma_session_thesis_reading (mig 105): model_id/prompt_id sono
-- uuid nullable SENZA FK (soft-ref al registry mrsmith, come in 105).

BEGIN;

-- ---------------------------------------------------------------------------
-- Run di lettura NI = una passata LLM su un processing run. Immutabile:
-- re-lettura => nuovo run, il vecchio passa a 'superseded'.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binocolo.ma_ni_reading_run (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  filing_id         uuid NOT NULL REFERENCES binocolo.ma_filing(id) ON DELETE CASCADE,
  processing_run_id uuid NOT NULL REFERENCES binocolo.ma_filing_processing_run(id) ON DELETE CASCADE,
  prompt_id         uuid,   -- soft-ref a mrsmith.llm_prompt (come 105)
  model_id          uuid,   -- soft-ref a mrsmith.llm_model (come 105)
  request_id        text,
  status            text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'superseded')),
  created_at        timestamptz NOT NULL DEFAULT now()
);

-- Indice di supporto FK / risoluzione del run corrente per filing. NON è unico:
-- l'unicità del run 'active' resta responsabilità applicativa (coerente con
-- ma_filing_processing_run, che qui non ha un vincolo one-active).
CREATE INDEX IF NOT EXISTS ma_ni_reading_run_filing_idx
  ON binocolo.ma_ni_reading_run (filing_id);

-- ---------------------------------------------------------------------------
-- Proposta di rettifica NI = fatto invisibile ai prospetti osservato dall'LLM.
-- IMMUTABILE (nessun updated_at): una nuova lettura produce nuove proposte in un
-- nuovo run. fingerprint = chiave stabile del fatto (dedup entro il run).
-- Convenzioni di segno (dichiarate nel prompt): ΔEBITDA add-back > 0; ΔPFN
-- positiva = più debito, cassa vincolata cash-like => ΔPFN negativa.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binocolo.ma_ni_proposal (
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id                uuid NOT NULL REFERENCES binocolo.ma_ni_reading_run(id) ON DELETE CASCADE,
  exercise_date         date,
  fingerprint           text NOT NULL,
  fatto_osservato       text,
  importo_lordo         numeric,
  trattamento_candidato text CHECK (trattamento_candidato IN ('ebitda', 'pfn', 'dd_only')),
  direction             text CHECK (direction IN ('increase', 'decrease', 'uncertain')),
  importo_rettifica     numeric,   -- firmato; NULL se verso incerto
  incertezza            text CHECK (incertezza IN ('low', 'medium', 'high')),
  quote                 text,
  page_no               int,
  section               text,
  label                 text,
  rationale             text,
  created_at            timestamptz NOT NULL DEFAULT now(),
  UNIQUE (run_id, fingerprint)
);

-- ---------------------------------------------------------------------------
-- Decisione analista su una proposta = APPEND-ONLY (l'ultima riga per proposta
-- è lo stato corrente; lo storico non si riscrive). La promozione di 'dd_only'
-- in ratifica passa da ratified_treatment ('ebitda'|'pfn'). auto_reconfirmed_from
-- collega una ratifica auto-confermata (nuovo run con stesso fingerprint) alla
-- decisione umana originaria.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binocolo.ma_ni_decision (
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  proposal_id           uuid NOT NULL REFERENCES binocolo.ma_ni_proposal(id) ON DELETE CASCADE,
  action                text NOT NULL CHECK (action IN ('ratify', 'reject', 'revoke')),
  ratified_amount       numeric,   -- firmato; valorizzato solo su 'ratify'
  ratified_treatment    text CHECK (ratified_treatment IN ('ebitda', 'pfn')),
  reason                text,
  actor_subject         text,
  actor_email           text,
  created_at            timestamptz NOT NULL DEFAULT now(),
  auto_reconfirmed_from uuid REFERENCES binocolo.ma_ni_decision(id)
);

CREATE INDEX IF NOT EXISTS ma_ni_decision_proposal_idx
  ON binocolo.ma_ni_decision (proposal_id, created_at);

COMMIT;
