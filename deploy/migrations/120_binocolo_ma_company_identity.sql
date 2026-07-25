-- 120: Binocolo M&A — registro identità azienda (issue #86, sub-issue di #81).
--
-- Il lavoro è ADOTTARE, NON MIGRARE. Le company_key esistenti — oggi RICALCOLATE
-- a ogni lettura dal payload del vendor con la precedenza
-- vendor_id > vat_code > tax_code > company_name (maTargetDedupeKey,
-- ma_rules.go) — diventano identificatori NOSTRI, opachi e mai più ricalcolati.
-- Le aziende nuove ricevono un UUID. Un registro identificatori per identità
-- fiscale fa da deduplica e da ponte verso i fornitori, presenti e futuri.
--
-- NON si aggiunge company_key alle ~15 tabelle già chiavate: le chiavi non
-- cambiano, quindi non c'è nulla da rimappare. Si aggiunge UNA colonna, su
-- ma_target, perché quella è la radice dell'occorrenza e oggi NON persiste la
-- chiave (la riderivava a ogni lettura: senza la colonna l'UUID di un'azienda
-- nuova non sopravvivrebbe a un round-trip).
--
-- Target database: Anisetta PostgreSQL, schema binocolo. Apply after 119.
-- Applicata a mano dall'utente con il suo processo (mai operazioni dirette sui
-- DB in env). Idempotente: CREATE ... IF NOT EXISTS ovunque, backfill con
-- preflight + ON CONFLICT DO NOTHING (vedi il blocco «Backfill» in fondo).
--
-- MODELLO DI ESECUZIONE: hard cutover ad applicazione e worker fermi.
--
--   stop writer
--     -> migrazione 120 (questa: schema + registro + colonna nullable)
--     -> migrazione 121 (backfill ma_target.company_key, ri-eseguibile)
--     -> GATE: count(ma_target WHERE company_key IS NULL) = 0
--     -> prova di equivalenza (sonda IDENTITA-AZIENDA-PROBE.sql, Q15/Q16/Q17)
--     -> deploy
--     -> smoke
--     -> riapertura scritture
--
-- Tre confini di rollback, distinti:
--   1. SCHEMA — questa migrazione è additiva: un rollback applicativo non
--      richiede mai il ripristino di schema eliminato.
--   2. SEMANTICO — la prima azienda assegnata con UUID. Prima, il binario
--      vecchio è sicuro; dopo, ri-deriva l'ObjectId per quell'azienda e ne crea
--      una seconda identità.
--   3. SCRITTURE — ma_target.company_key nasce NULLABLE proprio per non spostare
--      questo confine: con NOT NULL il binario vecchio fallirebbe ogni INSERT di
--      target, non avendo la colonna nella propria lista. Il vincolo arriva in
--      F5, che è un lavoro separato.

BEGIN;

-- ---------------------------------------------------------------------------
-- F0 — Primitive di normalizzazione, esposte a SQL.
--
-- La stessa idea vive oggi in quattro implementazioni (normalizeMAFiscalValue /
-- maStableVAT in ma_filing_store.go, la CTE stable_key del finder, l'OR a tre
-- vie di GetMACompanyDomain). Qui si promuovono le PRIMITIVE, non la funzione
-- composta buildMAFiscalKey(vat, tax): quella risponde a «quale singolo valore
-- chiava questo filing» e scarta un CF distinto — corretta per il suo mestiere,
-- sbagliata per il registro, che ha bisogno dell'INSIEME degli identificatori.
--
-- Servono in SQL perché CTE e backfill non possono chiamare il Go; senza,
-- l'espressione resterebbe scritta a mano in più punti — il problema da cui si
-- esce. La parità Go<->SQL è verificata sul corpus (sonda, Q15).
-- ---------------------------------------------------------------------------

-- Specchio esatto di normalizeMAFiscalValue (ma_filing_store.go:48): maiuscolo,
-- trim, rimozione di spazi e punti.
CREATE OR REPLACE FUNCTION binocolo.ma_normalize_fiscal(value text)
RETURNS text
LANGUAGE sql
IMMUTABLE
PARALLEL SAFE
AS $$
  SELECT regexp_replace(upper(btrim(COALESCE(value, ''))), '[[:space:].]', '', 'g')
$$;

-- Specchio esatto di maStableVAT (ma_filing_store.go:55): il valore pulito senza
-- il prefisso IT quando è IT + 11 cifre. Idempotente su input già pulito.
CREATE OR REPLACE FUNCTION binocolo.ma_stable_vat(value text)
RETURNS text
LANGUAGE sql
IMMUTABLE
PARALLEL SAFE
AS $$
  SELECT CASE
    WHEN binocolo.ma_normalize_fiscal(value) ~ '^IT[0-9]{11}$'
      THEN substr(binocolo.ma_normalize_fiscal(value), 3)
    ELSE binocolo.ma_normalize_fiscal(value)
  END
$$;

-- ---------------------------------------------------------------------------
-- F1 — ma_company: l'entità azienda.
--
-- company_key è l'id interno. Per le esistenti è l'ObjectId OpenAPI.it adottato
-- (origin = 'adopted'); per le nuove un UUID (origin = 'assigned'). In nessuno
-- dei due casi va MAI ricalcolato da un payload esterno: è il punto dell'intera
-- issue, ed è imposto dal trigger di immutabilità più sotto.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binocolo.ma_company (
  company_key      text PRIMARY KEY,
  -- Nome LETTERALE come osservato, mai normalizzato: il name-match della
  -- verifica dominio via scrape confronta la ragione sociale così com'è.
  company_name     text,
  -- Istante della CHIAMATA AL FORNITORE che ha prodotto il nome, non della
  -- scrittura: rileggere una riga già a DB non deve avanzarlo.
  name_observed_at timestamptz,
  name_source      text,
  -- fiscal      = ha almeno un identificatore fiscale (P.IVA o CF).
  -- vendor_only = conosciamo solo l'id del fornitore. È lo stato che NON
  --               sopravvive a un cambio fornitore: entra nel monitor, e un
  --               conteggio > 0 è un difetto da chiudere, non uno stato normale.
  identity_state   text NOT NULL DEFAULT 'vendor_only',
  origin           text NOT NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT ma_company_key_not_blank CHECK (btrim(company_key) <> ''),
  CONSTRAINT ma_company_identity_state_check CHECK (identity_state IN ('fiscal', 'vendor_only')),
  CONSTRAINT ma_company_origin_check CHECK (origin IN ('adopted', 'assigned'))
);

COMMENT ON COLUMN binocolo.ma_company.company_key IS
  'Id interno opaco. Adottato (ObjectId OpenAPI.it) o assegnato (UUID). NON VA MAI RICALCOLATO DA UN PAYLOAD ESTERNO: il valore è la chiave, non una derivazione.';

-- ---------------------------------------------------------------------------
-- ma_company_identifier: quali valori identificano questa azienda.
--
-- Il namespace fiscale è UNIFICATO, non spaccato in 'vat' e 'tax'. La ragione
-- che regge da sola: con kind separati, la P.IVA dell'azienda A uguale al CF
-- dell'azienda B sarebbe SILENZIOSAMENTE permessa, perché la PK non vedrebbe
-- collisione fra namespace diversi. Con un namespace unico quella collisione
-- fallisce. Ragione secondaria (assunzione di dominio, misurata da Q14 della
-- sonda): nelle società di capitali il CF è tipicamente lo stesso numero a 11
-- cifre della P.IVA, quindi con kind separati la stessa azienda produrrebbe due
-- righe. Il modello non cambia in nessuno dei due esiti della misura.
--
-- is_vat / is_tax sono METADATO DI RUOLO, non identità: si aggiornano in OR,
-- senza mai perdere un ruolo già osservato.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binocolo.ma_company_identifier (
  namespace     text NOT NULL,
  value         text NOT NULL,
  company_key   text NOT NULL REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT,
  is_vat        boolean NOT NULL DEFAULT false,
  is_tax        boolean NOT NULL DEFAULT false,
  first_seen_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at  timestamptz NOT NULL DEFAULT now(),

  PRIMARY KEY (namespace, value),
  CONSTRAINT ma_company_identifier_namespace_check
    CHECK (namespace IN ('fiscal', 'vendor_openapiit')),
  CONSTRAINT ma_company_identifier_value_not_blank CHECK (btrim(value) <> ''),
  -- I ruoli esistono solo nel namespace fiscale.
  CONSTRAINT ma_company_identifier_role_scope_check
    CHECK (namespace = 'fiscal' OR (is_vat = false AND is_tax = false)),
  -- Una riga fiscale con entrambi i flag falsi sarebbe priva di significato.
  CONSTRAINT ma_company_identifier_fiscal_role_check
    CHECK (namespace <> 'fiscal' OR is_vat OR is_tax)
);

CREATE INDEX IF NOT EXISTS ma_company_identifier_company_idx
  ON binocolo.ma_company_identifier (company_key);

-- ---------------------------------------------------------------------------
-- ma_company_identity_conflict: il ledger delle collisioni.
--
-- Un conflitto NON può essere insieme registrato e annullato: se stesse nella
-- transazione del resolver, il rollback porterebbe via la riga. Il contratto è
-- quindi a due transazioni — rollback + errore tipizzato, poi scrittura del
-- conflitto in una SECONDA transazione, di proprietà del livello service.
--
-- NESSUNA FK, nemmeno verso sessione o run. Due ragioni: un conflitto aborta il
-- batch SENZA scrivere, quindi il target non esiste e una FK obbligatoria
-- farebbe fallire l'inserimento; e un ledger che perde righe quando una sessione
-- viene purgata perderebbe proprio la storia che deve conservare.
--
-- Unicità PARZIALE su (namespace, value) WHERE state = 'open': se l'analista
-- rilancia la ricerca lo stesso conflitto si ripresenta, e un'unicità secca
-- farebbe fallire la scrittura diagnostica — che per contratto non deve mai
-- essere ciò che rompe.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binocolo.ma_company_identity_conflict (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  namespace            text NOT NULL,
  value                text NOT NULL,
  company_key_existing text NOT NULL,
  company_key_incoming text NOT NULL,
  state                text NOT NULL DEFAULT 'open',
  detected_at          timestamptz NOT NULL DEFAULT now(),
  last_detected_at     timestamptz NOT NULL DEFAULT now(),
  detections           integer NOT NULL DEFAULT 1,
  resolved_at          timestamptz,
  resolved_by_email    text,
  note                 text,
  -- Colonne diagnostiche dell'occorrenza, libere e senza FK.
  session_id           text,
  run_id               text,
  target_id            text,
  company_name         text,

  CONSTRAINT ma_company_identity_conflict_state_check CHECK (state IN ('open', 'resolved')),
  CONSTRAINT ma_company_identity_conflict_namespace_check
    CHECK (namespace IN ('fiscal', 'vendor_openapiit'))
);

CREATE UNIQUE INDEX IF NOT EXISTS ma_company_identity_conflict_open_idx
  ON binocolo.ma_company_identity_conflict (namespace, value)
  WHERE state = 'open';

-- ---------------------------------------------------------------------------
-- Trigger: le due regole che non devono dipendere da chi scrive.
--
-- La PK impedisce i duplicati, NON gli UPDATE: senza trigger, company_key
-- sarebbe modificabile e l'intera issue sarebbe una convenzione, non un
-- meccanismo. La transizione fiscal -> vendor_only sarebbe una perdita di
-- identità silenziosa.
--
-- La promozione INVERSA (vendor_only -> fiscal all'attach di un identificatore
-- fiscale) NON è qui: la fa il resolver nella sua transazione. Un trigger che
-- aggiornasse ma_company a ogni inserimento di identificatore sarebbe un
-- effetto collaterale nascosto.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION binocolo.ma_company_guard()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.company_key IS DISTINCT FROM OLD.company_key THEN
    RAISE EXCEPTION 'binocolo.ma_company.company_key è immutabile (% -> %)',
      OLD.company_key, NEW.company_key
      USING ERRCODE = 'raise_exception';
  END IF;
  IF OLD.identity_state = 'fiscal' AND NEW.identity_state = 'vendor_only' THEN
    RAISE EXCEPTION 'binocolo.ma_company %: identity_state non torna da fiscal a vendor_only',
      OLD.company_key
      USING ERRCODE = 'raise_exception';
  END IF;
  NEW.updated_at := now();
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS ma_company_guard_trg ON binocolo.ma_company;
CREATE TRIGGER ma_company_guard_trg
  BEFORE UPDATE ON binocolo.ma_company
  FOR EACH ROW EXECUTE FUNCTION binocolo.ma_company_guard();

-- ---------------------------------------------------------------------------
-- ma_target.company_key — la radice dell'occorrenza persiste la chiave.
--
-- NULLABLE di proposito: vedi il confine di rollback 3 in testa al file. Il
-- NOT NULL è F5, un lavoro separato.
-- ---------------------------------------------------------------------------
ALTER TABLE binocolo.ma_target
  ADD COLUMN IF NOT EXISTS company_key text;

COMMENT ON COLUMN binocolo.ma_target.company_key IS
  'Identità azienda risolta dal registro (binocolo.ma_company). NON VA MAI RICALCOLATA DA UN PAYLOAD ESTERNO: prima della mig 120 era riderivata a ogni lettura con COALESCE(vendor_id, vat_code, tax_code, company_name), e riportare quella derivazione ri-aggancerebbe l''identità al fornitore. La colonna si scrive solo con il valore restituito dal resolver.';

CREATE INDEX IF NOT EXISTS ma_target_company_key_idx
  ON binocolo.ma_target (company_key);

-- ===========================================================================
-- Backfill del registro.
--
-- Ordine obbligato: PREFLIGHT che SOLLEVA se la mappa identificatore -> chiave
-- non è una funzione, POI INSERT ... ON CONFLICT DO NOTHING. In questo ordine il
-- DO NOTHING non può nascondere una collisione vera — dopo il preflight i soli
-- conflitti che può assorbire sono re-inserimenti identici, che è ciò che serve
-- per l'idempotenza. Un INSERT stretto da solo NON sarebbe idempotente:
-- fallirebbe anche su un rilancio con righe perfettamente identiche.
--
-- Q7 della sonda è vuota, quindi il preflight passa oggi; ma l'asserzione deve
-- stare nella migrazione, non nella memoria di una query eseguita una volta.
-- ===========================================================================

-- Sorgenti che portano identificatori (chiave + P.IVA/CF/vendor_id + nome).
--
-- Per ma_target la COLONNA PERSISTITA VIENE PRIMA, e la derivazione legacy resta
-- solo per le righe che non l'hanno ancora. È ciò che rende vera la ri-entranza
-- dichiarata in testa al file: rieseguendo la 120 dopo che esistono aziende con
-- UUID, derivare dal vendor id reinterpreterebbe quelle righe con la chiave
-- vecchia e inserirebbe una seconda ma_company 'adopted' — un'entità fantasma
-- senza identificatori, perché i loro sarebbero già assegnati all'UUID e il DO
-- NOTHING li lascerebbe dove sono.
CREATE TEMP TABLE ma_identity_source ON COMMIT DROP AS
SELECT
  COALESCE(
    NULLIF(upper(btrim(COALESCE(t.company_key, ''))), ''),
    NULLIF(upper(btrim(COALESCE(t.vendor_id, ''))), ''),
    NULLIF(upper(btrim(COALESCE(t.vat_code, ''))), ''),
    NULLIF(upper(btrim(COALESCE(t.tax_code, ''))), ''),
    upper(btrim(COALESCE(t.company_name, '')))
  )                                     AS company_key,
  upper(btrim(COALESCE(t.vendor_id, ''))) AS vendor_id,
  COALESCE(t.vat_code, '')              AS vat_code,
  COALESCE(t.tax_code, '')              AS tax_code,
  COALESCE(t.company_name, '')          AS company_name,
  t.created_at                          AS seen_at
FROM binocolo.ma_target t
UNION ALL
SELECT upper(btrim(c.company_key)), '', COALESCE(c.vat_code, ''), COALESCE(c.tax_code, ''),
       COALESCE(c.company_name, ''), c.updated_at
FROM binocolo.ma_initiative_card c
UNION ALL
SELECT upper(btrim(d.company_key)), '', COALESCE(d.vat_code, ''), COALESCE(d.tax_code, ''),
       COALESCE(d.company_name, ''), d.updated_at
FROM binocolo.ma_company_domain d
UNION ALL
SELECT upper(btrim(a.company_key)), '', COALESCE(a.vat_code, ''), COALESCE(a.tax_code, ''),
       '', a.updated_at
FROM binocolo.ma_deep_analysis a
UNION ALL
SELECT upper(btrim(f.company_key)), '', COALESCE(f.vat_code, ''), COALESCE(f.tax_code, ''),
       COALESCE(f.company_name, ''), f.updated_at
FROM binocolo.ma_company_bm_family f;

DELETE FROM ma_identity_source WHERE btrim(COALESCE(company_key, '')) = '';

-- Tutte le chiavi che esistono da qualche parte, comprese quelle presenti SOLO
-- nel dettaglio (rating, esiti, note, IRL...) senza una riga con identificatori.
-- Il gate del cutover è «zero company_key in qualunque tabella senza una
-- ma_company corrispondente»: questa è la lista che lo rende vero.
CREATE TEMP TABLE ma_identity_key ON COMMIT DROP AS
SELECT DISTINCT company_key FROM (
  SELECT company_key FROM ma_identity_source
  UNION ALL SELECT upper(btrim(company_key)) FROM binocolo.ma_target_rating
  UNION ALL SELECT upper(btrim(company_key)) FROM binocolo.ma_target_web_validation
  UNION ALL SELECT upper(btrim(company_key)) FROM binocolo.ma_sector_eval_label
  UNION ALL SELECT upper(btrim(company_key)) FROM binocolo.ma_target_outcome
  UNION ALL SELECT upper(btrim(company_key)) FROM binocolo.ma_company_fact
  UNION ALL SELECT upper(btrim(company_key)) FROM binocolo.ma_company_note
  UNION ALL SELECT upper(btrim(company_key)) FROM binocolo.ma_deep_payload_vintage
  UNION ALL SELECT upper(btrim(company_key)) FROM binocolo.ma_card_thesis_reading
  UNION ALL SELECT upper(btrim(company_key)) FROM binocolo.ma_card_irl_item
  UNION ALL SELECT upper(btrim(company_key)) FROM binocolo.ma_session_thesis_reading
  UNION ALL SELECT upper(btrim(context_company_key)) FROM binocolo.ma_filing_acquisition
) all_keys
WHERE btrim(COALESCE(company_key, '')) <> '';

-- Identificatori grezzi, prima dell'aggregazione dei ruoli.
--
-- L'ultima UNION merita una spiegazione: per una chiave che compare solo come
-- card / registro domini / deep (nessuna riga ma_target, quindi nessuna colonna
-- vendor_id da cui leggere) la chiave STESSA è l'ObjectId che la generò. Q10
-- della sonda ha misurato la forma — 24 hex, lunghezza fissa, mai uguale alla
-- P.IVA — e nessun valore fiscale può assumerla (P.IVA 11 cifre, CF 16). Senza
-- questa riga, il giorno in cui un target ripresenta quel vendor_id la
-- risoluzione lo mancherebbe. Non può introdurre collisioni: il valore mappa
-- sulla chiave che è già se stessa.
CREATE TEMP TABLE ma_identity_identifier_raw ON COMMIT DROP AS
SELECT 'fiscal'::text AS namespace, binocolo.ma_stable_vat(vat_code) AS value,
       company_key, true AS is_vat, false AS is_tax, seen_at
FROM ma_identity_source
WHERE binocolo.ma_stable_vat(vat_code) <> ''
UNION ALL
SELECT 'fiscal', binocolo.ma_normalize_fiscal(tax_code), company_key, false, true, seen_at
FROM ma_identity_source
WHERE binocolo.ma_normalize_fiscal(tax_code) <> ''
UNION ALL
SELECT 'vendor_openapiit', vendor_id, company_key, false, false, seen_at
FROM ma_identity_source
WHERE vendor_id <> ''
UNION ALL
SELECT 'vendor_openapiit', company_key, company_key, false, false, now()
FROM ma_identity_key
WHERE company_key ~ '^[0-9A-F]{24}$';

-- PREFLIGHT. Solleva se un identificatore rivendica due aziende.
--
-- Il confronto include il registro GIÀ SCRITTO, non solo la sorgente: a un
-- rilancio della migrazione la collisione può stare fra ciò che si sta per
-- inserire e ciò che c'è, e in quel caso l'ON CONFLICT DO NOTHING la
-- assorbirebbe in silenzio invece di farla vedere.
DO $$
DECLARE
  offenders integer;
  sample    text;
BEGIN
  WITH claimed AS (
    SELECT namespace, value, company_key FROM ma_identity_identifier_raw
    UNION
    SELECT namespace, value, company_key FROM binocolo.ma_company_identifier
  ), bad AS (
    SELECT format('  %s/%s -> %s', namespace, value,
                  array_to_string(array_agg(DISTINCT company_key ORDER BY company_key), ', ')) AS descr
    FROM claimed
    GROUP BY namespace, value
    HAVING COUNT(DISTINCT company_key) > 1
  )
  SELECT (SELECT COUNT(*) FROM bad),
         COALESCE((SELECT string_agg(descr, E'\n') FROM (SELECT descr FROM bad ORDER BY descr LIMIT 20) top), '')
    INTO offenders, sample;

  IF offenders > 0 THEN
    RAISE EXCEPTION E'mig 120: la mappa identificatore -> company_key non è una funzione (% casi, primi 20):\n%',
      offenders, sample
      USING ERRCODE = 'raise_exception';
  END IF;
END;
$$;

-- Entità. Il nome è l'osservazione non vuota più recente; name_observed_at è il
-- created_at/updated_at della riga che lo portava — il miglior proxy
-- disponibile dell'istante della chiamata al fornitore, non una sua misura.
INSERT INTO binocolo.ma_company (company_key, company_name, name_observed_at, name_source, identity_state, origin)
SELECT
  k.company_key,
  named.company_name,
  named.seen_at,
  CASE WHEN named.company_name IS NULL THEN NULL ELSE 'backfill_120' END,
  CASE WHEN EXISTS (
    SELECT 1 FROM ma_identity_identifier_raw i
    WHERE i.company_key = k.company_key AND i.namespace = 'fiscal'
  ) THEN 'fiscal' ELSE 'vendor_only' END,
  'adopted'
FROM ma_identity_key k
LEFT JOIN LATERAL (
  SELECT NULLIF(btrim(s.company_name), '') AS company_name, s.seen_at
  FROM ma_identity_source s
  WHERE s.company_key = k.company_key AND NULLIF(btrim(s.company_name), '') IS NOT NULL
  ORDER BY s.seen_at DESC
  LIMIT 1
) named ON true
ON CONFLICT (company_key) DO NOTHING;

-- Identificatori, con i ruoli aggregati in OR.
INSERT INTO binocolo.ma_company_identifier (namespace, value, company_key, is_vat, is_tax, first_seen_at, last_seen_at)
SELECT namespace, value, company_key,
       bool_or(is_vat), bool_or(is_tax),
       MIN(seen_at), MAX(seen_at)
FROM ma_identity_identifier_raw
GROUP BY namespace, value, company_key
ON CONFLICT (namespace, value) DO NOTHING;

COMMIT;
