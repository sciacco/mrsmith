-- Binocolo M&A — Bilanci depositati (issue #78, Fase 1). Storage del fascicolo
-- di bilancio: blob deduplicati, filing con identità fiscale, acquisizioni e
-- ricerche DocuEngine come state machine durevoli, run di processing immutabili
-- (OCR/parse) con pagine ed estratti CEE.
--
-- Target database: Anisetta PostgreSQL, schema binocolo. Apply after 112.
-- Applicata a mano dall'utente con il suo processo (mai operazioni dirette sui
-- DB in env). Idempotente: CREATE TABLE/INDEX IF NOT EXISTS; l'unico ALTER con
-- ADD CONSTRAINT è preceduto da DROP CONSTRAINT IF EXISTS.
--
-- Ordine di creazione vincolato dalle dipendenze FK:
--   ma_filing_blob -> ma_filing -> ma_filing_acquisition -> ma_filing_search
--   -> ma_filing_processing_run -> ma_filing_page/ma_filing_extract
--   -> ALTER ma_deep_analysis, ALTER ma_filing (FK active_processing_run_id).
--
-- Nota identità: NIENTE colonna `origin` su ma_filing — l'origine vive solo sulle
-- acquisizioni (ma_filing_acquisition.origin). Il filing è identificato dalla
-- chiave fiscale, MAI da company_key: fiscal_key = vat_clean se presente,
-- altrimenti tax_clean, dove *_clean = regexp_replace(upper(btrim(x)),
-- '[[:space:].]','','g') (stessa normalizzazione di ma_store.go).

BEGIN;

-- ---------------------------------------------------------------------------
-- Blob del PDF, deduplicato per digest. Chiave = md5 in HEX MINUSCOLO canonico.
-- Canonicalizzazione: il valore restituito da DocuEngine (Download.md5) è
-- presumibilmente in base64 e va DECODIFICATO -> hex minuscolo PRIMA di ogni
-- confronto o INSERT; il digest calcolato localmente sul PDF scaricato è già
-- hex. sha256 è ridondante ma tenuto per verifica incrociata.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binocolo.ma_filing_blob (
  md5        text PRIMARY KEY,   -- hex lowercase canonico
  sha256     text,
  size_bytes bigint,
  pdf        bytea,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Filing = fascicolo di bilancio per (identità fiscale, esercizio più recente).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binocolo.ma_filing (
  id                           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  fiscal_key                   text NOT NULL,   -- vat_clean se presente, altrimenti tax_clean
  vat_clean                    text,
  tax_clean                    text,
  closing_date                 date NOT NULL,   -- chiusura dell'esercizio più recente del fascicolo (dal documento)
  balance_sheet_id             text,
  balance_sheet_type           text,
  taxonomy_version             text,
  identity_status              text NOT NULL DEFAULT 'pending_validation'
    CHECK (identity_status IN ('pending_validation', 'validated', 'mismatch', 'override')),
  -- Override set-once dell'identità (mismatch fiscale accettato manualmente):
  -- l'unicità del set è imposta applicativamente, non da vincolo.
  identity_override_by_subject text,
  identity_override_by_email   text,
  identity_override_reason     text,
  identity_override_at         timestamptz,
  status                       text NOT NULL DEFAULT 'queued'
    CHECK (status IN ('queued', 'ocr', 'parse', 'ni_reading', 'ready', 'degraded', 'failed', 'identity_blocked')),
  blob_md5                     text REFERENCES binocolo.ma_filing_blob(md5),
  active_processing_run_id     uuid,   -- FK aggiunta in coda, dopo ma_filing_processing_run
  page_count                   int,
  error                        text,
  created_by_subject           text,
  created_by_email             text,
  created_at                   timestamptz NOT NULL DEFAULT now()
);

-- Un solo fascicolo per (identità, esercizio, balance sheet) quando il vendor
-- fornisce un balanceSheetId opaco; i NULL non partecipano al vincolo.
CREATE UNIQUE INDEX IF NOT EXISTS ma_filing_fiscal_closing_bsid_uidx
  ON binocolo.ma_filing (fiscal_key, closing_date, balance_sheet_id)
  WHERE balance_sheet_id IS NOT NULL;

-- Stesso blob non duplicato per la stessa identità (dedup post-download). I NULL
-- su blob_md5 sono distinti, quindi filing senza blob non collidono.
CREATE UNIQUE INDEX IF NOT EXISTS ma_filing_fiscal_blob_uidx
  ON binocolo.ma_filing (fiscal_key, blob_md5);

CREATE INDEX IF NOT EXISTS ma_filing_fiscal_key_idx
  ON binocolo.ma_filing (fiscal_key);

-- ---------------------------------------------------------------------------
-- Acquisizione = un tentativo di procurarsi un fascicolo (upload manuale o
-- download DocuEngine). La riga NASCE PRIMA della chiamata al vendor (status
-- 'intent'); `origin` vive QUI, non su ma_filing. Lo stato 'unknown' (esito
-- vendor indeterminato) NON viene mai risolto euristicamente.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binocolo.ma_filing_acquisition (
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  filing_id             uuid REFERENCES binocolo.ma_filing(id),   -- NULL finché il filing non esiste
  origin                text NOT NULL CHECK (origin IN ('upload', 'docuengine')),
  status                text NOT NULL DEFAULT 'intent'
    CHECK (status IN ('intent', 'requested', 'downloaded', 'done', 'failed', 'unknown')),
  docuengine_request_id text,
  result_id             text,
  balance_sheet_id      text,
  context_company_key   text,
  actor_subject         text,
  actor_email           text,
  error                 text,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ma_filing_acquisition_filing_idx
  ON binocolo.ma_filing_acquisition (filing_id);

-- ---------------------------------------------------------------------------
-- Ricerca DocuEngine per identità fiscale = state machine durevole. La
-- consumazione (una ricerca genera un'acquisizione) è un claim CAS
-- 'results' -> 'consumed' su consumed_by_acquisition_id. Lo stato 'unknown'
-- (esito vendor indeterminato) non viene risolto euristicamente.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binocolo.ma_filing_search (
  id                         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  fiscal_key                 text NOT NULL,
  status                     text NOT NULL DEFAULT 'intent'
    CHECK (status IN ('intent', 'requested', 'unknown', 'results', 'consumed', 'failed')),
  consumed_by_acquisition_id uuid REFERENCES binocolo.ma_filing_acquisition(id),
  docuengine_request_id      text,
  results                    jsonb,   -- result_id opachi + balanceSheetId + tipo + closing date
  searched_by_subject        text,
  searched_by_email          text,
  created_at                 timestamptz NOT NULL DEFAULT now(),
  updated_at                 timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ma_filing_search_fiscal_key_idx
  ON binocolo.ma_filing_search (fiscal_key);

-- ---------------------------------------------------------------------------
-- Processing run = una passata OCR+parse IMMUTABILE. Re-OCR o re-parse creano
-- un NUOVO run; il vecchio passa a 'superseded'. ma_filing.active_processing_run_id
-- punta al run 'active' corrente.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binocolo.ma_filing_processing_run (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  filing_id          uuid NOT NULL REFERENCES binocolo.ma_filing(id) ON DELETE CASCADE,
  ocr_model          text,
  ocr_params_version text,
  parse_version      text,
  status             text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'superseded')),
  request_id         text,
  created_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ma_filing_processing_run_filing_idx
  ON binocolo.ma_filing_processing_run (filing_id);

-- ---------------------------------------------------------------------------
-- Pagine OCR del run (markdown per pagina + extra strutturati opzionali).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binocolo.ma_filing_page (
  processing_run_id uuid NOT NULL REFERENCES binocolo.ma_filing_processing_run(id) ON DELETE CASCADE,
  page_no           int NOT NULL,
  markdown          text,
  extras            jsonb,
  PRIMARY KEY (processing_run_id, page_no)
);

-- ---------------------------------------------------------------------------
-- Estratti CEE per esercizio (uno per closing date presente nel fascicolo):
-- stato patrimoniale (sp), conto economico (ce), quadrature (checks), lettura
-- Document AI (docai) e diff docai vs parse deterministico (docai_diff).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binocolo.ma_filing_extract (
  processing_run_id uuid NOT NULL REFERENCES binocolo.ma_filing_processing_run(id) ON DELETE CASCADE,
  exercise_date     date NOT NULL,
  sp                jsonb,
  ce                jsonb,
  checks            jsonb,
  docai             jsonb,
  docai_diff        jsonb,
  PRIMARY KEY (processing_run_id, exercise_date)
);

-- ---------------------------------------------------------------------------
-- ma_deep_analysis: marcatempo del brief, aggiornato SOLO al salvataggio del
-- brief (serve alla staleness del brief vs nuovi esercizi allineati).
-- ---------------------------------------------------------------------------
ALTER TABLE binocolo.ma_deep_analysis
  ADD COLUMN IF NOT EXISTS brief_generated_at timestamptz;

-- ---------------------------------------------------------------------------
-- FK di ma_filing.active_processing_run_id -> ma_filing_processing_run(id),
-- aggiunta ora che la tabella referenziata esiste. DROP IF EXISTS + ADD per
-- idempotenza (CREATE TABLE IF NOT EXISTS non riesegue l'ALTER).
-- ---------------------------------------------------------------------------
ALTER TABLE binocolo.ma_filing DROP CONSTRAINT IF EXISTS ma_filing_active_processing_run_id_fkey;
ALTER TABLE binocolo.ma_filing
  ADD CONSTRAINT ma_filing_active_processing_run_id_fkey
  FOREIGN KEY (active_processing_run_id) REFERENCES binocolo.ma_filing_processing_run(id);

COMMIT;
