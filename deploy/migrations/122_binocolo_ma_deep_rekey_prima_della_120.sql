-- 122: Binocolo M&A — riporta i dossier con la P.IVA come chiave sulla chiave
-- canonica dell'azienda (issue #86).
--
-- ============================================================================
-- SI APPLICA PRIMA DELLA 120. Il numero è più alto per una ragione sola: 120 e
-- 121 sono già citate per numero nella issue, nei commenti del codice, nella
-- sonda e nel runbook, e rinumerarle desincronizzerebbe il registro scritto per
-- un guadagno estetico. L'ordine di esecuzione è 122 -> 120 -> 121, e il
-- runbook lo dice come passo 0.5.
-- ============================================================================
--
-- PERCHÉ ESISTE
--
-- Lo strumento standalone /azienda accodava il dossier con la P.IVA come
-- company_key: `EnqueueMADeepAnalysis(ctx, vat, vat, ...)`, dove il primo
-- argomento è la chiave. Il codice non lo fa più (commit c14f48b), ma le righe
-- scritte restano. Quando la stessa azienda esiste anche come target — con un
-- ObjectId per chiave — quel valore fiscale rivendica DUE company_key, e il
-- preflight della 120 solleva. È il gate che funziona, non un guasto: la 120
-- rifiuta di adottare un corpus in cui un identificatore non è una funzione.
--
-- MISURATO 2026-07-26 sull'Anisetta (sonda Q16 / Q19 / Q4):
--
--   Q16  4 identità fiscali con due chiavi ciascuna
--   Q19  6 chiavi a forma di P.IVA in ma_deep_analysis e
--        ma_deep_payload_vintage, 3 in ma_company_bm_family;
--        ZERO in target, card, rating, esiti, note, IRL, domini, web validation
--   Q4   0 identità con due dossier, 0 euro sprecati
--
-- Le tre misure insieme dicono che questa NON è la fusione rimandata dalla
-- issue. Quella presuppone storia su entrambi i lati — card, voti, esiti, URL
-- navigati — e va progettata. Qui la chiave sbagliata è usata SOLO da righe di
-- cache, la chiave giusta non ha un dossier concorrente (Q4), e non esiste
-- alcun contenuto dell'analista da riconciliare. Si sposta, non si fonde.
--
-- Le chiavi fiscali che NON collidono con nulla (2 delle 6: aziende viste solo
-- da /azienda, mai in una ricerca) restano come sono: la chiave è opaca, la
-- adotteremo com'è, e se un domani quell'azienda comparirà in una ricerca il
-- resolver la ritroverà dal suo identificatore fiscale. Q19 continuerà a
-- contarle, ed è corretto che lo faccia.
--
-- Target database: Anisetta PostgreSQL, schema binocolo.
-- Applicata a mano dall'utente. Idempotente: dopo lo spostamento la chiave
-- sorgente non esiste più in quelle tabelle e un rilancio non trova nulla.

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. Le identità da riportare: un valore fiscale che è ANCHE una company_key
--    in una tabella di cache, mentre la stessa identità ha un'altra chiave
--    nel corpus autoritativo (target, card, registro domini).
--
--    La normalizzazione è scritta a mano perché le funzioni della 120 non
--    esistono ancora: è la stessa espressione, ed è un colpo solo.
-- ---------------------------------------------------------------------------
CREATE TEMP TABLE ma_rekey ON COMMIT DROP AS
WITH clean AS (
  SELECT
    upper(btrim(company_key)) AS company_key,
    CASE WHEN regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
         THEN substr(regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g'), 3)
         ELSE regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g') END AS vat,
    regexp_replace(upper(btrim(COALESCE(tax_code, ''))), '[[:space:].]', '', 'g') AS tax,
    seen_at, authoritative
  FROM (
    SELECT
      COALESCE(
        NULLIF(upper(btrim(COALESCE(vendor_id, ''))), ''),
        NULLIF(upper(btrim(COALESCE(vat_code, ''))), ''),
        NULLIF(upper(btrim(COALESCE(tax_code, ''))), ''),
        upper(btrim(COALESCE(company_name, '')))
      ) AS company_key,
      vat_code, tax_code, created_at AS seen_at, true AS authoritative
    FROM binocolo.ma_target
    UNION ALL
    SELECT company_key, vat_code, tax_code, updated_at, true FROM binocolo.ma_initiative_card
    UNION ALL
    SELECT company_key, vat_code, tax_code, updated_at, true FROM binocolo.ma_company_domain
    UNION ALL
    SELECT company_key, vat_code, tax_code, updated_at, false FROM binocolo.ma_deep_analysis
  ) src
  WHERE btrim(COALESCE(company_key, '')) <> ''
), identity AS (
  SELECT company_key, authoritative, seen_at, v.value AS fiscal
  FROM clean, LATERAL (VALUES (clean.vat), (clean.tax)) AS v(value)
  WHERE v.value <> ''
), canonical AS (
  -- La chiave autoritativa per quell'identità: quella del corpus, non della cache.
  SELECT DISTINCT ON (fiscal) fiscal, company_key AS target_key
  FROM identity
  WHERE authoritative
  ORDER BY fiscal, seen_at DESC
)
SELECT DISTINCT c.fiscal, i.company_key AS source_key, c.target_key
FROM identity i
JOIN canonical c ON c.fiscal = i.fiscal
WHERE NOT i.authoritative
  AND i.company_key = c.fiscal          -- la chiave sorgente È il valore fiscale
  AND i.company_key <> c.target_key;

-- ---------------------------------------------------------------------------
-- 2. Asserzioni. Meglio fermarsi che spostare alla cieca.
-- ---------------------------------------------------------------------------
DO $$
DECLARE
  ambigue     integer;
  collisioni  integer;
  altrove     integer;
  sample      text;
BEGIN
  -- 2a. Una chiave sorgente che punterebbe a due destinazioni diverse.
  SELECT COUNT(*) INTO ambigue
  FROM (SELECT source_key FROM ma_rekey GROUP BY source_key HAVING COUNT(DISTINCT target_key) > 1) x;
  IF ambigue > 0 THEN
    RAISE EXCEPTION 'mig 122: % chiavi con destinazione ambigua — serve una decisione umana', ambigue
      USING ERRCODE = 'raise_exception';
  END IF;

  -- 2b. La destinazione ha già un dossier: sarebbe una fusione vera, fuori scope.
  SELECT COUNT(*), COALESCE(string_agg(format('  %s -> %s', source_key, target_key), E'\n'), '')
    INTO collisioni, sample
  FROM ma_rekey r
  WHERE EXISTS (SELECT 1 FROM binocolo.ma_deep_analysis d WHERE d.company_key = r.target_key);
  IF collisioni > 0 THEN
    RAISE EXCEPTION E'mig 122: % casi in cui la chiave di destinazione ha GIÀ un dossier.\nNon è uno spostamento ma una fusione, che questa migrazione non fa:\n%',
      collisioni, sample
      USING ERRCODE = 'raise_exception';
  END IF;

  -- 2c. La chiave sorgente è usata anche fuori dalle tre tabelle di cache:
  --     allora c'è storia dell'analista da riconciliare, ed è una fusione.
  SELECT COUNT(*), COALESCE(string_agg(DISTINCT format('  %s in %s', k, t), E'\n'), '')
    INTO altrove, sample
  FROM (
    SELECT r.source_key AS k, 'ma_target_rating' AS t FROM ma_rekey r JOIN binocolo.ma_target_rating x ON upper(btrim(x.company_key)) = r.source_key
    UNION ALL SELECT r.source_key, 'ma_target_web_validation' FROM ma_rekey r JOIN binocolo.ma_target_web_validation x ON upper(btrim(x.company_key)) = r.source_key
    UNION ALL SELECT r.source_key, 'ma_sector_eval_label' FROM ma_rekey r JOIN binocolo.ma_sector_eval_label x ON upper(btrim(x.company_key)) = r.source_key
    UNION ALL SELECT r.source_key, 'ma_target_outcome' FROM ma_rekey r JOIN binocolo.ma_target_outcome x ON upper(btrim(x.company_key)) = r.source_key
    UNION ALL SELECT r.source_key, 'ma_initiative_card' FROM ma_rekey r JOIN binocolo.ma_initiative_card x ON upper(btrim(x.company_key)) = r.source_key
    UNION ALL SELECT r.source_key, 'ma_company_domain' FROM ma_rekey r JOIN binocolo.ma_company_domain x ON upper(btrim(x.company_key)) = r.source_key
    UNION ALL SELECT r.source_key, 'ma_company_fact' FROM ma_rekey r JOIN binocolo.ma_company_fact x ON upper(btrim(x.company_key)) = r.source_key
    UNION ALL SELECT r.source_key, 'ma_company_note' FROM ma_rekey r JOIN binocolo.ma_company_note x ON upper(btrim(x.company_key)) = r.source_key
    UNION ALL SELECT r.source_key, 'ma_card_thesis_reading' FROM ma_rekey r JOIN binocolo.ma_card_thesis_reading x ON upper(btrim(x.company_key)) = r.source_key
    UNION ALL SELECT r.source_key, 'ma_session_thesis_reading' FROM ma_rekey r JOIN binocolo.ma_session_thesis_reading x ON upper(btrim(x.company_key)) = r.source_key
    UNION ALL SELECT r.source_key, 'ma_card_irl_item' FROM ma_rekey r JOIN binocolo.ma_card_irl_item x ON upper(btrim(x.company_key)) = r.source_key
    UNION ALL SELECT r.source_key, 'ma_filing_acquisition' FROM ma_rekey r JOIN binocolo.ma_filing_acquisition x ON upper(btrim(x.context_company_key)) = r.source_key
  ) y;
  IF altrove > 0 THEN
    RAISE EXCEPTION E'mig 122: la chiave sorgente porta storia fuori dalle tabelle di cache.\nÈ una fusione, non uno spostamento:\n%', sample
      USING ERRCODE = 'raise_exception';
  END IF;

  RAISE NOTICE 'mig 122: % dossier da riportare sulla chiave canonica', (SELECT COUNT(*) FROM ma_rekey);
END;
$$;

-- ---------------------------------------------------------------------------
-- 3. Lo spostamento. Tre tabelle, tutte cache per-azienda.
--    ma_deep_payload_vintage ha PK (company_key, balance_sheet_date): la
--    destinazione non ha righe (assunto 2b sul dossier, e le vintage seguono il
--    dossier), ma il DO NOTHING difende comunque dal caso limite.
-- ---------------------------------------------------------------------------
UPDATE binocolo.ma_deep_analysis d
SET company_key = r.target_key, updated_at = now()
FROM ma_rekey r
WHERE d.company_key = r.source_key;

UPDATE binocolo.ma_deep_payload_vintage v
SET company_key = r.target_key
FROM ma_rekey r
WHERE v.company_key = r.source_key
  AND NOT EXISTS (
    SELECT 1 FROM binocolo.ma_deep_payload_vintage x
    WHERE x.company_key = r.target_key AND x.balance_sheet_date = v.balance_sheet_date
  );
DELETE FROM binocolo.ma_deep_payload_vintage v
USING ma_rekey r
WHERE v.company_key = r.source_key;   -- eventuali residui: la destinazione li ha già

UPDATE binocolo.ma_company_bm_family f
SET company_key = r.target_key, updated_at = now()
FROM ma_rekey r
WHERE f.company_key = r.source_key
  AND NOT EXISTS (SELECT 1 FROM binocolo.ma_company_bm_family x WHERE x.company_key = r.target_key);
DELETE FROM binocolo.ma_company_bm_family f
USING ma_rekey r
WHERE f.company_key = r.source_key;

-- ---------------------------------------------------------------------------
-- 4. Gate: nessuna chiave sorgente sopravvive.
-- ---------------------------------------------------------------------------
DO $$
DECLARE residui integer;
BEGIN
  SELECT COUNT(*) INTO residui
  FROM ma_rekey r
  WHERE EXISTS (SELECT 1 FROM binocolo.ma_deep_analysis x WHERE x.company_key = r.source_key)
     OR EXISTS (SELECT 1 FROM binocolo.ma_deep_payload_vintage x WHERE x.company_key = r.source_key)
     OR EXISTS (SELECT 1 FROM binocolo.ma_company_bm_family x WHERE x.company_key = r.source_key);
  IF residui > 0 THEN
    RAISE EXCEPTION 'mig 122: % chiavi sorgente ancora presenti dopo lo spostamento', residui
      USING ERRCODE = 'raise_exception';
  END IF;
END;
$$;

COMMIT;

-- Dopo questa migrazione: rieseguire Q16. Deve essere VUOTA. Solo allora si
-- applica la 120.
