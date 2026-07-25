-- 121: Binocolo M&A — backfill di ma_target.company_key (issue #86).
--
-- STEP SEPARATO E RI-ESEGUIBILE, distinto dal backfill del registro (mig 120),
-- proprio per poter girare da solo.
--
-- Perché serve un file a parte: ma_target.company_key nasce NULLABLE per non
-- spostare il confine di rollback delle scritture (con NOT NULL il binario
-- vecchio fallirebbe ogni INSERT di target). Il prezzo di quella scelta è
-- questo: se un rollback applicativo rimette in servizio il binario vecchio
-- prima del confine semantico, quello crea target con company_key IS NULL, e al
-- tentativo di cutover successivo il backfill va rieseguito a writer fermi.
--
-- Naturalmente idempotente: è un UPDATE ... WHERE company_key IS NULL.
--
-- Ordine: applicare DOPO la 120, a writer fermi, PRIMA di mettere in servizio il
-- binario nuovo. Le guardie applicative — che dopo il cutover ERRANO invece di
-- riderivare la chiave — sono raggiungibili solo dopo il gate finale di questo
-- file.
--
-- Target database: Anisetta PostgreSQL, schema binocolo.
-- Applicata a mano dall'utente con il suo processo (mai operazioni dirette sui
-- DB in env).

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. Riempi i NULL con la derivazione storica.
--
-- Questa è l'UNICA volta in cui la derivazione dal payload del vendor è ancora
-- legittima: è l'atto di adozione: fotografa la chiave che il codice avrebbe
-- comunque calcolato a lettura, e da qui in poi il valore è nostro. Il codice
-- applicativo non la ricalcola mai più — vedi il COMMENT sulla colonna.
-- ---------------------------------------------------------------------------
UPDATE binocolo.ma_target
SET company_key = COALESCE(
      NULLIF(upper(btrim(COALESCE(vendor_id, ''))), ''),
      NULLIF(upper(btrim(COALESCE(vat_code, ''))), ''),
      NULLIF(upper(btrim(COALESCE(tax_code, ''))), ''),
      NULLIF(upper(btrim(COALESCE(company_name, ''))), '')
    )
WHERE company_key IS NULL;

-- ---------------------------------------------------------------------------
-- 2. Registra le identità comparse dopo la 120.
--
-- Se il passo 1 ha adottato righe che la 120 non aveva visto (target creati da
-- un binario vecchio rimesso in servizio), quelle chiavi non hanno una
-- ma_company e il gate del cutover fallirebbe. Stessa logica della 120, ristretta
-- ai soli target: preflight che solleva, poi INSERT ... ON CONFLICT DO NOTHING.
-- ---------------------------------------------------------------------------
CREATE TEMP TABLE ma_target_identity_delta ON COMMIT DROP AS
SELECT
  t.company_key,
  upper(btrim(COALESCE(t.vendor_id, ''))) AS vendor_id,
  COALESCE(t.vat_code, '')                AS vat_code,
  COALESCE(t.tax_code, '')                AS tax_code,
  COALESCE(t.company_name, '')            AS company_name,
  t.created_at                            AS seen_at
FROM binocolo.ma_target t
WHERE COALESCE(btrim(t.company_key), '') <> '';

CREATE TEMP TABLE ma_target_identifier_delta ON COMMIT DROP AS
SELECT 'fiscal'::text AS namespace, binocolo.ma_stable_vat(vat_code) AS value,
       company_key, true AS is_vat, false AS is_tax, seen_at
FROM ma_target_identity_delta
WHERE binocolo.ma_stable_vat(vat_code) <> ''
UNION ALL
SELECT 'fiscal', binocolo.ma_normalize_fiscal(tax_code), company_key, false, true, seen_at
FROM ma_target_identity_delta
WHERE binocolo.ma_normalize_fiscal(tax_code) <> ''
UNION ALL
SELECT 'vendor_openapiit', vendor_id, company_key, false, false, seen_at
FROM ma_target_identity_delta
WHERE vendor_id <> '';

-- Preflight: contro il registro già scritto E contro se stesso.
DO $$
DECLARE
  offenders integer;
  sample    text;
BEGIN
  WITH claimed AS (
    SELECT namespace, value, company_key FROM ma_target_identifier_delta
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
    RAISE EXCEPTION E'mig 121: un identificatore rivendica due aziende (% casi, primi 20):\n%',
      offenders, sample
      USING ERRCODE = 'raise_exception';
  END IF;
END;
$$;

INSERT INTO binocolo.ma_company (company_key, company_name, name_observed_at, name_source, identity_state, origin)
SELECT
  k.company_key,
  named.company_name,
  named.seen_at,
  CASE WHEN named.company_name IS NULL THEN NULL ELSE 'backfill_121' END,
  CASE WHEN EXISTS (
    SELECT 1 FROM ma_target_identifier_delta i
    WHERE i.company_key = k.company_key AND i.namespace = 'fiscal'
  ) THEN 'fiscal' ELSE 'vendor_only' END,
  'adopted'
FROM (SELECT DISTINCT company_key FROM ma_target_identity_delta) k
LEFT JOIN LATERAL (
  SELECT NULLIF(btrim(d.company_name), '') AS company_name, d.seen_at
  FROM ma_target_identity_delta d
  WHERE d.company_key = k.company_key AND NULLIF(btrim(d.company_name), '') IS NOT NULL
  ORDER BY d.seen_at DESC
  LIMIT 1
) named ON true
ON CONFLICT (company_key) DO NOTHING;

INSERT INTO binocolo.ma_company_identifier (namespace, value, company_key, is_vat, is_tax, first_seen_at, last_seen_at)
SELECT namespace, value, company_key, bool_or(is_vat), bool_or(is_tax), MIN(seen_at), MAX(seen_at)
FROM ma_target_identifier_delta
GROUP BY namespace, value, company_key
ON CONFLICT (namespace, value) DO UPDATE SET
  is_vat        = binocolo.ma_company_identifier.is_vat OR EXCLUDED.is_vat,
  is_tax        = binocolo.ma_company_identifier.is_tax OR EXCLUDED.is_tax,
  first_seen_at = LEAST(binocolo.ma_company_identifier.first_seen_at, EXCLUDED.first_seen_at),
  last_seen_at  = GREATEST(binocolo.ma_company_identifier.last_seen_at, EXCLUDED.last_seen_at)
WHERE binocolo.ma_company_identifier.company_key = EXCLUDED.company_key;

-- Promuovi le entità che hanno acquisito un identificatore fiscale.
UPDATE binocolo.ma_company c
SET identity_state = 'fiscal'
WHERE c.identity_state = 'vendor_only'
  AND EXISTS (
    SELECT 1 FROM binocolo.ma_company_identifier i
    WHERE i.company_key = c.company_key AND i.namespace = 'fiscal'
  );

-- ---------------------------------------------------------------------------
-- 3. GATE pre-deploy. Solleva invece di lasciar proseguire un cutover parziale.
-- ---------------------------------------------------------------------------
DO $$
DECLARE
  null_keys    integer;
  orphan_keys  integer;
BEGIN
  SELECT COUNT(*) INTO null_keys
  FROM binocolo.ma_target WHERE company_key IS NULL;

  IF null_keys > 0 THEN
    RAISE EXCEPTION 'mig 121: % target con company_key IS NULL dopo il backfill (righe senza vendor_id, P.IVA, CF né ragione sociale)', null_keys
      USING ERRCODE = 'raise_exception';
  END IF;

  SELECT COUNT(*) INTO orphan_keys
  FROM binocolo.ma_target t
  WHERE NOT EXISTS (SELECT 1 FROM binocolo.ma_company c WHERE c.company_key = t.company_key);

  IF orphan_keys > 0 THEN
    RAISE EXCEPTION 'mig 121: % target la cui company_key non ha una ma_company', orphan_keys
      USING ERRCODE = 'raise_exception';
  END IF;
END;
$$;

COMMIT;
