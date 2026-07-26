-- =============================================================================
-- Issue #81 — sonda sui dati reali per la scelta del modello di identità azienda
--
-- Target: Anisetta PostgreSQL, schema binocolo. SOLE SELECT: nessuna DDL, nessuna
-- DML, nessun lock oltre le letture MVCC normali. Eseguibili in qualunque ordine
-- e ripetibili.
--
-- -----------------------------------------------------------------------------
-- ESITO DELLA PRIMA ESECUZIONE — 2026-07-25 (baseline)
--
--   Q1  identità fiscale alla nascita ... P.IVA su 1.783 righe su 1.784 (1 solo CF)
--   Q2  chiave = vendor_id ............... 1.584/1.584 hanno vendor_id, 1.583 è la
--                                          chiave; ZERO target keyed sulla P.IVA
--   Q3  sibling key ...................... 1.367 identità, TUTTE con 1 sola chiave
--   Q6  conflitti P.IVA<->CF ............. vuota
--   Q7  chiave -> due identità ........... vuota (la mappa è una funzione)
--   Q10 forma del vendor_id .............. ObjectId MongoDB, 24 hex, len 24 fissa,
--                                          mai uguale alla P.IVA
--   Q11 riconciliazione .................. 1.507 ObjectId = 1.507 identità, ZERO
--                                          aziende con due ObjectId (142 target in
--                                          sessioni cancellate spiegano i 1.367)
--   Q12 timestamp degli ObjectId ......... 1.324 id su 1.507 con timestamp in
--                                          giugno 2021, il resto ~1 al mese fino a
--                                          giugno 2026. NON interpretare: cosa
--                                          significhi sugli interni di OpenAPI.it
--                                          non lo sappiamo. La prova che il
--                                          fornitore non ha rinumerato è Q11, non
--                                          questa distribuzione.
--   Q13 ragioni sociali .................. 1 nome per azienda, zero divergenze
--
-- CONCLUSIONE: nessuna frammentazione da riparare. company_key È stabile, ma la
-- stabilità è EREDITATA dall'ObjectId di OpenAPI.it, non garantita da noi — e
-- l'evidenza copre solo le 6 settimane 2026-06-15 -> 2026-07-24 (età del corpus).
-- Direzione decisa: adottare le chiavi esistenti come id interni opachi (UUID per
-- le nuove) + tabella di conversione per identità fiscale. Vedi il commento su
-- github.com/sciacco/mrsmith/issues/81 e docs/IMPLEMENTATION-KNOWLEDGE.md, voce
-- «Binocolo Internal Company Finder Uses Fiscal Identity Groups».
--
-- Rilanciare la sonda per misurare la deriva: Q3/Q11 sono anche il MONITOR
-- dell'invariante «una company_key per identità fiscale». Un valore > 0 in
-- `identita_con_piu_vendor_id` significa che il fornitore ha rinumerato.
-- -----------------------------------------------------------------------------
--
-- La normalizzazione è ricopiata dal codice, non reinventata:
--   clean(x)      = regexp_replace(upper(btrim(x)), '[[:space:].]', '', 'g')
--                   -> normalizeMAFiscalValue, ma_filing_store.go:49
--   stable_vat(x) = clean(x) senza il prefisso IT quando clean(x) ~ '^IT[0-9]{11}$'
--                   -> maStableVAT, ma_filing_store.go:55
--   fiscal_key    = stable_vat(vat) se non vuoto, altrimenti clean(tax)
--                   -> buildMAFiscalKey, ma_filing_store.go:66
--   company_key   = upper(btrim(vendor_id >> vat_code >> tax_code >> company_name))
--                   -> maTargetDedupeKey, ma_rules.go:960 (NB: solo upper+trim,
--                      nessuna rimozione di punti/spazi, nessun strip di IT)
--
-- Se hai poco tempo: la Q1 da sola decide fra "identità = P.IVA normalizzata" e
-- "identità = UUID + registro identificatori".
-- =============================================================================


-- -----------------------------------------------------------------------------
-- Q1 — Copertura dell'identità fiscale alla nascita.  [LA DECISIVA]
--
-- Decide: esiste davvero l'azienda che nasce senza P.IVA? Se `solo_tax` e
-- `nessuna_identita` sono ~0, l'upgrade CF->P.IVA non è un caso reale e la P.IVA
-- normalizzata può essere l'identità canonica.
-- `ma_target` è spaccata per origin ed enrichment_level perché lo stadio Address
-- potrebbe restituire meno campi identitari dell'Advanced.
-- -----------------------------------------------------------------------------
SELECT
  'ma_target'                                                        AS sorgente,
  COALESCE(NULLIF(btrim(origin), ''), '(vuoto)')                     AS origin,
  COALESCE(NULLIF(btrim(enrichment_level), ''), '(vuoto)')           AS enrichment,
  COUNT(*)                                                           AS righe,
  COUNT(*) FILTER (WHERE regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g') <> '')  AS con_vat,
  COUNT(*) FILTER (WHERE regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g') =  ''
                     AND regexp_replace(upper(btrim(COALESCE(tax_code, ''))), '[[:space:].]', '', 'g') <> '')  AS solo_tax,
  COUNT(*) FILTER (WHERE regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g') =  ''
                     AND regexp_replace(upper(btrim(COALESCE(tax_code, ''))), '[[:space:].]', '', 'g') =  '')  AS nessuna_identita
FROM binocolo.ma_target
GROUP BY 1, 2, 3

UNION ALL
SELECT
  'ma_initiative_card',
  COALESCE(NULLIF(btrim(origin), ''), '(vuoto)'),
  '-',
  COUNT(*),
  COUNT(*) FILTER (WHERE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') <> ''),
  COUNT(*) FILTER (WHERE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') =  ''
                     AND regexp_replace(upper(btrim(tax_code)), '[[:space:].]', '', 'g') <> ''),
  COUNT(*) FILTER (WHERE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') =  ''
                     AND regexp_replace(upper(btrim(tax_code)), '[[:space:].]', '', 'g') =  '')
FROM binocolo.ma_initiative_card
GROUP BY 1, 2, 3

UNION ALL
SELECT
  'ma_company_domain', '-', '-',
  COUNT(*),
  COUNT(*) FILTER (WHERE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') <> ''),
  COUNT(*) FILTER (WHERE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') =  ''
                     AND regexp_replace(upper(btrim(tax_code)), '[[:space:].]', '', 'g') <> ''),
  COUNT(*) FILTER (WHERE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') =  ''
                     AND regexp_replace(upper(btrim(tax_code)), '[[:space:].]', '', 'g') =  '')
FROM binocolo.ma_company_domain

UNION ALL
SELECT
  'ma_deep_analysis', '-', '-',
  COUNT(*),
  COUNT(*) FILTER (WHERE regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g') <> ''),
  COUNT(*) FILTER (WHERE regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g') =  ''
                     AND regexp_replace(upper(btrim(COALESCE(tax_code, ''))), '[[:space:].]', '', 'g') <> ''),
  COUNT(*) FILTER (WHERE regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g') =  ''
                     AND regexp_replace(upper(btrim(COALESCE(tax_code, ''))), '[[:space:].]', '', 'g') =  '')
FROM binocolo.ma_deep_analysis
ORDER BY 1, 2, 3;


-- -----------------------------------------------------------------------------
-- Q2 — La P.IVA c'è nella riga ma non è la chiave.
--
-- Misura direttamente il meccanismo di frammentazione principale: vendor_id
-- scavalca una P.IVA presente. `vendor_vince` sono le righe che oggi producono
-- una company_key diversa dalla P.IVA che hanno in colonna.
-- -----------------------------------------------------------------------------
WITH t AS (
  SELECT
    upper(btrim(COALESCE(vendor_id, '')))                                                AS vendor_up,
    regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g')         AS vat_clean,
    regexp_replace(upper(btrim(COALESCE(tax_code, ''))), '[[:space:].]', '', 'g')         AS tax_clean,
    COALESCE(
      NULLIF(upper(btrim(COALESCE(vendor_id, ''))), ''),
      NULLIF(upper(btrim(COALESCE(vat_code, ''))), ''),
      NULLIF(upper(btrim(COALESCE(tax_code, ''))), ''),
      upper(btrim(company_name))
    )                                                                                    AS company_key
  FROM binocolo.ma_target
)
SELECT
  COUNT(*)                                                                    AS target_totali,
  COUNT(*) FILTER (WHERE vendor_up <> '')                                      AS con_vendor_id,
  COUNT(*) FILTER (WHERE vendor_up <> '' AND vat_clean <> '')                   AS vendor_vince_su_vat,
  COUNT(*) FILTER (WHERE vendor_up = ''  AND vat_clean <> '')                   AS chiave_e_la_vat,
  COUNT(*) FILTER (WHERE vendor_up = ''  AND vat_clean = '' AND tax_clean <> '') AS chiave_e_il_tax,
  COUNT(*) FILTER (WHERE company_key = upper(btrim(company_key))
                     AND vendor_up = '' AND vat_clean = '' AND tax_clean = '')  AS chiave_e_il_nome
FROM t;


-- -----------------------------------------------------------------------------
-- Q3 — Sibling key per identità fiscale: distribuzione.
--
-- Quante identità fiscali reali sono spezzate in 2, 3, N company_key. È la
-- dimensione del danno e insieme la dimensione della tabella di alias da
-- costruire. Il corpus è lo stesso del finder (ricerche + card + registro domini).
-- -----------------------------------------------------------------------------
WITH corpus AS (
  SELECT
    COALESCE(
      NULLIF(upper(btrim(COALESCE(t.vendor_id, ''))), ''),
      NULLIF(upper(btrim(COALESCE(t.vat_code, ''))), ''),
      NULLIF(upper(btrim(COALESCE(t.tax_code, ''))), ''),
      upper(btrim(t.company_name))
    ) AS company_key,
    COALESCE(t.vat_code, '') AS vat_code, COALESCE(t.tax_code, '') AS tax_code
  FROM binocolo.ma_target t
  JOIN binocolo.ma_session s ON s.id = t.session_id
  WHERE s.deleted_at IS NULL AND s.purged_at IS NULL
  UNION ALL
  SELECT upper(btrim(c.company_key)), c.vat_code, c.tax_code
  FROM binocolo.ma_initiative_card c
  JOIN binocolo.ma_initiative i ON i.id = c.initiative_id
  WHERE i.deleted_at IS NULL AND i.purged_at IS NULL
  UNION ALL
  SELECT upper(btrim(d.company_key)), d.vat_code, d.tax_code
  FROM binocolo.ma_company_domain d
), keyed AS (
  SELECT
    company_key,
    COALESCE(
      NULLIF(CASE WHEN regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
                  THEN substr(regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g'), 3)
                  ELSE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') END, ''),
      NULLIF(regexp_replace(upper(btrim(tax_code)), '[[:space:].]', '', 'g'), '')
    ) AS fiscal_key
  FROM corpus
  WHERE company_key <> ''
), grouped AS (
  SELECT fiscal_key, COUNT(DISTINCT company_key) AS n_keys
  FROM keyed
  WHERE fiscal_key IS NOT NULL
  GROUP BY fiscal_key
)
SELECT
  n_keys                             AS company_key_per_identita,
  COUNT(*)                           AS quante_identita,
  SUM(n_keys)                        AS company_key_coinvolte
FROM grouped
GROUP BY n_keys
ORDER BY n_keys;


-- -----------------------------------------------------------------------------
-- Q3b — Le 30 identità più frammentate, con le chiavi in chiaro.
--
-- Serve per capire il MOTIVO della frammentazione guardandola: id vendor vs
-- P.IVA, prefisso IT, punteggiatura, nome.
-- -----------------------------------------------------------------------------
WITH corpus AS (
  SELECT
    COALESCE(
      NULLIF(upper(btrim(COALESCE(t.vendor_id, ''))), ''),
      NULLIF(upper(btrim(COALESCE(t.vat_code, ''))), ''),
      NULLIF(upper(btrim(COALESCE(t.tax_code, ''))), ''),
      upper(btrim(t.company_name))
    ) AS company_key,
    COALESCE(t.vat_code, '') AS vat_code, COALESCE(t.tax_code, '') AS tax_code,
    COALESCE(t.company_name, '') AS company_name
  FROM binocolo.ma_target t
  JOIN binocolo.ma_session s ON s.id = t.session_id
  WHERE s.deleted_at IS NULL AND s.purged_at IS NULL
  UNION ALL
  SELECT upper(btrim(c.company_key)), c.vat_code, c.tax_code, c.company_name
  FROM binocolo.ma_initiative_card c
  JOIN binocolo.ma_initiative i ON i.id = c.initiative_id
  WHERE i.deleted_at IS NULL AND i.purged_at IS NULL
  UNION ALL
  SELECT upper(btrim(d.company_key)), d.vat_code, d.tax_code, d.company_name
  FROM binocolo.ma_company_domain d
), keyed AS (
  SELECT
    company_key, company_name,
    COALESCE(
      NULLIF(CASE WHEN regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
                  THEN substr(regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g'), 3)
                  ELSE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') END, ''),
      NULLIF(regexp_replace(upper(btrim(tax_code)), '[[:space:].]', '', 'g'), '')
    ) AS fiscal_key
  FROM corpus
  WHERE company_key <> ''
)
SELECT
  fiscal_key,
  MIN(company_name)                                AS un_nome,
  COUNT(DISTINCT company_key)                      AS n_keys,
  array_agg(DISTINCT company_key ORDER BY company_key) AS chiavi
FROM keyed
WHERE fiscal_key IS NOT NULL
GROUP BY fiscal_key
HAVING COUNT(DISTINCT company_key) > 1
ORDER BY COUNT(DISTINCT company_key) DESC, fiscal_key
LIMIT 30;


-- -----------------------------------------------------------------------------
-- Q4 — Il costo già pagato: dossier deep comprati due volte.
--
-- ma_deep_analysis è una cache per-azienda con company_key come PK. Se la stessa
-- identità fiscale ha più righe, abbiamo pagato più volte lo stesso dossier.
-- `euro_sprecati` = costo di tutte le righe oltre la prima di ogni identità.
-- L'identità viene dalla riga stessa quando ha vat/tax, altrimenti dal corpus
-- tramite company_key.
-- -----------------------------------------------------------------------------
WITH corpus AS (
  SELECT
    COALESCE(
      NULLIF(upper(btrim(COALESCE(t.vendor_id, ''))), ''),
      NULLIF(upper(btrim(COALESCE(t.vat_code, ''))), ''),
      NULLIF(upper(btrim(COALESCE(t.tax_code, ''))), ''),
      upper(btrim(t.company_name))
    ) AS company_key,
    COALESCE(t.vat_code, '') AS vat_code, COALESCE(t.tax_code, '') AS tax_code
  FROM binocolo.ma_target t
  UNION ALL
  SELECT upper(btrim(c.company_key)), c.vat_code, c.tax_code FROM binocolo.ma_initiative_card c
  UNION ALL
  SELECT upper(btrim(d.company_key)), d.vat_code, d.tax_code FROM binocolo.ma_company_domain d
), key_fiscal AS (
  SELECT
    company_key,
    MIN(COALESCE(
      NULLIF(CASE WHEN regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
                  THEN substr(regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g'), 3)
                  ELSE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') END, ''),
      NULLIF(regexp_replace(upper(btrim(tax_code)), '[[:space:].]', '', 'g'), '')
    )) AS fiscal_key
  FROM corpus
  WHERE company_key <> ''
  GROUP BY company_key
), deep AS (
  SELECT
    a.company_key,
    a.status,
    a.cost_eur,
    COALESCE(
      NULLIF(CASE WHEN regexp_replace(upper(btrim(COALESCE(a.vat_code, ''))), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
                  THEN substr(regexp_replace(upper(btrim(COALESCE(a.vat_code, ''))), '[[:space:].]', '', 'g'), 3)
                  ELSE regexp_replace(upper(btrim(COALESCE(a.vat_code, ''))), '[[:space:].]', '', 'g') END, ''),
      NULLIF(regexp_replace(upper(btrim(COALESCE(a.tax_code, ''))), '[[:space:].]', '', 'g'), ''),
      kf.fiscal_key
    ) AS fiscal_key
  FROM binocolo.ma_deep_analysis a
  LEFT JOIN key_fiscal kf ON kf.company_key = upper(btrim(a.company_key))
), dup AS (
  SELECT fiscal_key, COUNT(*) AS n_righe, SUM(cost_eur) AS costo_totale, MAX(cost_eur) AS costo_max
  FROM deep
  WHERE fiscal_key IS NOT NULL
  GROUP BY fiscal_key
  HAVING COUNT(*) > 1
)
SELECT
  (SELECT COUNT(*) FROM deep)                                      AS deep_righe_totali,
  (SELECT COUNT(*) FROM deep WHERE fiscal_key IS NULL)             AS deep_senza_identita,
  (SELECT COUNT(*) FROM dup)                                       AS identita_duplicate,
  (SELECT COALESCE(SUM(n_righe), 0) FROM dup)                      AS righe_coinvolte,
  (SELECT COALESCE(SUM(costo_totale - costo_max), 0) FROM dup)      AS euro_sprecati;


-- -----------------------------------------------------------------------------
-- Q5 — Frammentazione da normalizzazione, non da vendor_id.
--
-- company_key fa solo upper+trim: 'IT12345678901', '12345678901' e una P.IVA con
-- punti diventano tre chiavi diverse. Qui conto le chiavi non canoniche, cioè
-- quelle che cambierebbero passando alla normalizzazione fiscale.
-- -----------------------------------------------------------------------------
WITH corpus AS (
  SELECT
    COALESCE(
      NULLIF(upper(btrim(COALESCE(t.vendor_id, ''))), ''),
      NULLIF(upper(btrim(COALESCE(t.vat_code, ''))), ''),
      NULLIF(upper(btrim(COALESCE(t.tax_code, ''))), ''),
      upper(btrim(t.company_name))
    ) AS company_key
  FROM binocolo.ma_target t
  UNION
  SELECT upper(btrim(company_key)) FROM binocolo.ma_initiative_card
  UNION
  SELECT upper(btrim(company_key)) FROM binocolo.ma_company_domain
)
SELECT
  COUNT(*)                                                                  AS chiavi_distinte,
  COUNT(*) FILTER (WHERE company_key ~ '^[0-9]{11}$')                        AS forma_piva_pulita,
  COUNT(*) FILTER (WHERE company_key ~ '^IT[0-9]{11}$')                      AS forma_piva_con_IT,
  COUNT(*) FILTER (WHERE company_key ~ '[[:space:].]'
                     AND regexp_replace(company_key, '[[:space:].]', '', 'g') ~ '^(IT)?[0-9]{11}$') AS piva_con_punteggiatura,
  COUNT(*) FILTER (WHERE company_key ~ '^[0-9A-Z]{16}$')                     AS forma_cf_16,
  COUNT(*) FILTER (WHERE regexp_replace(company_key, '[[:space:].]', '', 'g') !~ '^(IT)?([0-9]{11}|[0-9A-Z]{16})$') AS ne_piva_ne_cf
FROM corpus;


-- -----------------------------------------------------------------------------
-- Q6 — Conflitti identitari: il caso che vieta la P.IVA come chiave.
--
-- (a) stessa P.IVA con codici fiscali diversi -> gruppo IVA o dato sbagliato: una
--     chiave = P.IVA FONDEREBBE aziende distinte (errore silenzioso).
-- (b) stesso CF con P.IVA diverse -> ditta individuale con più posizioni, oppure
--     dato sbagliato.
-- Se (a) è vuoto, la P.IVA come identità canonica è sicura sui dati che abbiamo.
-- -----------------------------------------------------------------------------
WITH corpus AS (
  SELECT COALESCE(t.vat_code, '') AS vat_code, COALESCE(t.tax_code, '') AS tax_code,
         COALESCE(t.company_name, '') AS company_name
  FROM binocolo.ma_target t
  UNION ALL
  SELECT vat_code, tax_code, company_name FROM binocolo.ma_initiative_card
  UNION ALL
  SELECT vat_code, tax_code, company_name FROM binocolo.ma_company_domain
), clean AS (
  SELECT
    CASE WHEN regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
         THEN substr(regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g'), 3)
         ELSE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') END AS vat,
    regexp_replace(upper(btrim(tax_code)), '[[:space:].]', '', 'g')               AS tax,
    company_name
  FROM corpus
)
SELECT 'a) una P.IVA, piu CF' AS conflitto, vat AS valore,
       array_agg(DISTINCT tax ORDER BY tax) AS controparti,
       array_agg(DISTINCT company_name ORDER BY company_name) AS nomi
FROM clean
WHERE vat <> '' AND tax <> ''
GROUP BY vat
HAVING COUNT(DISTINCT tax) > 1

UNION ALL
SELECT 'b) un CF, piu P.IVA', tax,
       array_agg(DISTINCT vat ORDER BY vat),
       array_agg(DISTINCT company_name ORDER BY company_name)
FROM clean
WHERE vat <> '' AND tax <> ''
GROUP BY tax
HAVING COUNT(DISTINCT vat) > 1
ORDER BY 1, 2
LIMIT 50;


-- -----------------------------------------------------------------------------
-- Q7 — La mappa legacy->canonico è una funzione?
--
-- Se una company_key risulta associata a DUE identità fiscali diverse, la tabella
-- di alias non può essere una semplice FK: serve arbitraggio. Questo caso è
-- possibile perché la chiave da nome collide fra omonimi e perché una chiave da
-- vendor_id potrebbe essere stata riusata con anagrafiche diverse.
-- -----------------------------------------------------------------------------
WITH corpus AS (
  SELECT
    COALESCE(
      NULLIF(upper(btrim(COALESCE(t.vendor_id, ''))), ''),
      NULLIF(upper(btrim(COALESCE(t.vat_code, ''))), ''),
      NULLIF(upper(btrim(COALESCE(t.tax_code, ''))), ''),
      upper(btrim(t.company_name))
    ) AS company_key,
    COALESCE(t.vat_code, '') AS vat_code, COALESCE(t.tax_code, '') AS tax_code,
    COALESCE(t.company_name, '') AS company_name
  FROM binocolo.ma_target t
  UNION ALL
  SELECT upper(btrim(company_key)), vat_code, tax_code, company_name FROM binocolo.ma_initiative_card
  UNION ALL
  SELECT upper(btrim(company_key)), vat_code, tax_code, company_name FROM binocolo.ma_company_domain
), keyed AS (
  SELECT company_key, company_name,
    COALESCE(
      NULLIF(CASE WHEN regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
                  THEN substr(regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g'), 3)
                  ELSE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') END, ''),
      NULLIF(regexp_replace(upper(btrim(tax_code)), '[[:space:].]', '', 'g'), '')
    ) AS fiscal_key
  FROM corpus
  WHERE company_key <> ''
)
SELECT
  company_key,
  COUNT(DISTINCT fiscal_key)                                AS n_identita,
  array_agg(DISTINCT fiscal_key ORDER BY fiscal_key)        AS identita,
  array_agg(DISTINCT company_name ORDER BY company_name)    AS nomi
FROM keyed
WHERE fiscal_key IS NOT NULL
GROUP BY company_key
HAVING COUNT(DISTINCT fiscal_key) > 1
ORDER BY 2 DESC, 1
LIMIT 50;


-- -----------------------------------------------------------------------------
-- Q8 — Bilanci: la fiscal_key è già a rischio di rekey?
--
-- ma_filing usa fiscal_key = vat_clean ?? tax_clean. Le righe la cui fiscal_key
-- NON è una P.IVA sono quelle che andrebbero rekeyate se la P.IVA arrivasse dopo:
-- è la verifica empirica se la domanda 4 della issue esiste nel sottosistema che
-- l'ha già affrontata.
-- -----------------------------------------------------------------------------
SELECT
  COUNT(*)                                                                       AS filing_totali,
  COUNT(DISTINCT fiscal_key)                                                     AS identita_distinte,
  COUNT(*) FILTER (WHERE COALESCE(vat_clean, '') <> '')                           AS con_vat,
  COUNT(*) FILTER (WHERE COALESCE(vat_clean, '') = '')                            AS senza_vat_rekey_a_rischio,
  COUNT(*) FILTER (WHERE fiscal_key !~ '^[0-9]{11}$')                             AS fiscal_key_non_piva,
  COUNT(*) FILTER (WHERE COALESCE(context_company_key, '') <> '')                 AS con_context_company_key
FROM binocolo.ma_filing;


-- -----------------------------------------------------------------------------
-- Q9 — Chi si perde la storia: card e ricerche su chiavi sorelle.
--
-- Una card in iniziativa e una ricerca sulla stessa azienda con chiavi diverse
-- sono la collisione cross-iniziativa citata nella issue. Qui conto le identità
-- che compaiono sia come card sia come target ma con company_key differente:
-- sono i casi in cui la Scheda azienda mostra oggi una storia parziale.
-- -----------------------------------------------------------------------------
WITH t AS (
  SELECT
    COALESCE(
      NULLIF(upper(btrim(COALESCE(vendor_id, ''))), ''),
      NULLIF(upper(btrim(COALESCE(vat_code, ''))), ''),
      NULLIF(upper(btrim(COALESCE(tax_code, ''))), ''),
      upper(btrim(company_name))
    ) AS company_key,
    COALESCE(
      NULLIF(CASE WHEN regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
                  THEN substr(regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g'), 3)
                  ELSE regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g') END, ''),
      NULLIF(regexp_replace(upper(btrim(COALESCE(tax_code, ''))), '[[:space:].]', '', 'g'), '')
    ) AS fiscal_key
  FROM binocolo.ma_target
), c AS (
  SELECT
    upper(btrim(company_key)) AS company_key,
    COALESCE(
      NULLIF(CASE WHEN regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
                  THEN substr(regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g'), 3)
                  ELSE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') END, ''),
      NULLIF(regexp_replace(upper(btrim(tax_code)), '[[:space:].]', '', 'g'), '')
    ) AS fiscal_key
  FROM binocolo.ma_initiative_card
)
SELECT
  COUNT(DISTINCT c.fiscal_key)                                                          AS identita_con_card,
  COUNT(DISTINCT c.fiscal_key) FILTER (WHERE t.fiscal_key IS NOT NULL)                   AS anche_in_ricerca,
  COUNT(DISTINCT c.fiscal_key) FILTER (WHERE t.company_key IS DISTINCT FROM c.company_key
                                         AND t.fiscal_key IS NOT NULL)                   AS con_chiave_diversa
FROM c
LEFT JOIN t ON t.fiscal_key = c.fiscal_key
WHERE c.fiscal_key IS NOT NULL;


-- -----------------------------------------------------------------------------
-- Q10 — Che cosa E' concretamente il vendor_id?
--
-- Serve prima dell'adozione: se `uguale_alla_vat` fosse ~1584, l'id del vendor
-- sarebbe la P.IVA stessa e le chiavi adottate sarebbero gia' P.IVA (con tutto
-- cio' che implica: identificatore esterno usato come id interno). Se invece e'
-- una numerazione propria del fornitore, l'adozione produce chiavi davvero opache.
-- `campioni` mostra tre valori reali per giudicare la forma a occhio.
-- -----------------------------------------------------------------------------
WITH t AS (
  SELECT
    upper(btrim(COALESCE(vendor_id, '')))                                          AS vendor_up,
    regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g')   AS vat_clean
  FROM binocolo.ma_target
)
SELECT
  COUNT(*)                                                              AS righe,
  COUNT(DISTINCT vendor_up)                                             AS vendor_id_distinti,
  COUNT(*) FILTER (WHERE vendor_up = vat_clean)                          AS uguale_alla_vat,
  COUNT(*) FILTER (WHERE vendor_up ~ '^[0-9]{11}$')                      AS forma_11_cifre,
  COUNT(*) FILTER (WHERE vendor_up ~ '^[0-9]+$')                         AS solo_cifre,
  MIN(length(vendor_up))                                                AS len_min,
  MAX(length(vendor_up))                                                AS len_max,
  (SELECT array_agg(v) FROM (SELECT DISTINCT vendor_up AS v FROM t ORDER BY 1 LIMIT 3) s) AS campioni
FROM t;


-- -----------------------------------------------------------------------------
-- Q11 — Riconciliazione 1.507 vs 1.367: la stessa azienda ha due ObjectId?
--
-- Q3 girava sul corpus FILTRATO (sessioni/iniziative non cancellate) e includeva
-- card e domini; Q10 gira su ma_target senza filtri. I 140 di differenza vanno
-- spiegati. `identita_con_piu_vendor_id` è il numero che conta: se è > 0 esiste
-- frammentazione reale che Q3 non poteva vedere.
-- -----------------------------------------------------------------------------
WITH t AS (
  SELECT
    upper(btrim(COALESCE(vendor_id, ''))) AS vendor_up,
    COALESCE(
      NULLIF(CASE WHEN regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
                  THEN substr(regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g'), 3)
                  ELSE regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g') END, ''),
      NULLIF(regexp_replace(upper(btrim(COALESCE(tax_code, ''))), '[[:space:].]', '', 'g'), '')
    ) AS fiscal_key,
    session_id
  FROM binocolo.ma_target
), per_identita AS (
  SELECT fiscal_key, COUNT(DISTINCT vendor_up) AS n_vendor
  FROM t WHERE fiscal_key IS NOT NULL AND vendor_up <> ''
  GROUP BY fiscal_key
)
SELECT
  (SELECT COUNT(*) FROM t)                                             AS target_totali,
  (SELECT COUNT(DISTINCT vendor_up) FROM t WHERE vendor_up <> '')       AS vendor_id_distinti,
  (SELECT COUNT(*) FROM per_identita)                                  AS identita_distinte,
  (SELECT COUNT(*) FROM per_identita WHERE n_vendor > 1)                AS identita_con_piu_vendor_id,
  (SELECT COALESCE(SUM(n_vendor), 0) FROM per_identita WHERE n_vendor > 1) AS objectid_coinvolti,
  (SELECT COUNT(*) FROM t JOIN binocolo.ma_session s ON s.id = t.session_id
    WHERE s.deleted_at IS NOT NULL OR s.purged_at IS NOT NULL)          AS target_in_sessioni_cancellate;


-- -----------------------------------------------------------------------------
-- Q11b — Le identità con più ObjectId, in chiaro (vuota se Q11 dà 0).
--
-- Se ci sono, i timestamp dei due ObjectId dicono se il secondo viene da un
-- import successivo del fornitore: è la firma del re-import.
-- -----------------------------------------------------------------------------
WITH t AS (
  SELECT
    upper(btrim(COALESCE(vendor_id, ''))) AS vendor_up,
    COALESCE(vat_code, '') AS vat_code, COALESCE(company_name, '') AS company_name,
    COALESCE(
      NULLIF(CASE WHEN regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
                  THEN substr(regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g'), 3)
                  ELSE regexp_replace(upper(btrim(COALESCE(vat_code, ''))), '[[:space:].]', '', 'g') END, ''),
      NULLIF(regexp_replace(upper(btrim(COALESCE(tax_code, ''))), '[[:space:].]', '', 'g'), '')
    ) AS fiscal_key
  FROM binocolo.ma_target
)
SELECT
  fiscal_key,
  MIN(company_name)                                            AS un_nome,
  COUNT(DISTINCT vendor_up)                                    AS n_objectid,
  array_agg(DISTINCT vendor_up ORDER BY vendor_up)             AS objectid,
  array_agg(DISTINCT to_char(to_timestamp(('x' || substr(lower(vendor_up), 1, 8))::bit(32)::int), 'YYYY-MM-DD')) AS creati_il
FROM t
WHERE fiscal_key IS NOT NULL AND vendor_up ~ '^[0-9A-F]{24}$'
GROUP BY fiscal_key
HAVING COUNT(DISTINCT vendor_up) > 1
ORDER BY 3 DESC, 1
LIMIT 40;


-- -----------------------------------------------------------------------------
-- Q12 — Quanti import ha fatto il fornitore?
--
-- Il timestamp nei primi 4 byte dell'ObjectId è la data di creazione del documento
-- presso OpenAPI.it. Serve a vedere SE gli id nascono tutti insieme o nel tempo.
-- ATTENZIONE all'interpretazione: la distribuzione dice quando il fornitore ha
-- creato i documenti, non perché. Non dedurne import, ricarichi o politiche di
-- rinumerazione: sono i suoi interni e non li conosciamo. La prova che non ha
-- rinumerato è Q11 (zero aziende con due ObjectId), non questa query.
-- Nota metodologica: non campionare gli ObjectId con ORDER BY sul valore per
-- giudicarne la distribuzione temporale — ordinarli per valore li ordina per
-- timestamp, quindi risulteranno adiacenti per costruzione.
-- -----------------------------------------------------------------------------
SELECT
  to_char(to_timestamp(('x' || substr(lower(upper(btrim(vendor_id))), 1, 8))::bit(32)::int), 'YYYY-MM') AS mese_creazione_objectid,
  COUNT(*)                                        AS righe,
  COUNT(DISTINCT upper(btrim(vendor_id)))          AS objectid_distinti,
  MIN(created_at)::date                            AS primo_uso_da_noi,
  MAX(created_at)::date                            AS ultimo_uso_da_noi
FROM binocolo.ma_target
WHERE upper(btrim(COALESCE(vendor_id, ''))) ~ '^[0-9A-F]{24}$'
GROUP BY 1
ORDER BY 1;


-- -----------------------------------------------------------------------------
-- Q13 — Quante aziende hanno più di una ragione sociale?
--
-- Il nome non è normalizzato in nessun punto (ma_vendor.go:85 lo prende grezzo da
-- IT-search o IT-advanced) e la scelta di quale mostrare segue due regole diverse
-- su finder e scheda. Qui misuro se la divergenza esiste nei dati: `con_piu_nomi`
-- sono le identità che possono mostrare nomi diversi a seconda della superficie.
-- `solo_maiuscole_o_spazi` isola i casi che differiscono per pura forma.
-- -----------------------------------------------------------------------------
WITH corpus AS (
  SELECT
    COALESCE(
      NULLIF(CASE WHEN regexp_replace(upper(btrim(COALESCE(t.vat_code, ''))), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
                  THEN substr(regexp_replace(upper(btrim(COALESCE(t.vat_code, ''))), '[[:space:].]', '', 'g'), 3)
                  ELSE regexp_replace(upper(btrim(COALESCE(t.vat_code, ''))), '[[:space:].]', '', 'g') END, ''),
      NULLIF(regexp_replace(upper(btrim(COALESCE(t.tax_code, ''))), '[[:space:].]', '', 'g'), '')
    ) AS fiscal_key,
    btrim(COALESCE(t.company_name, '')) AS company_name,
    'target:' || COALESCE(t.enrichment_level, '?') AS fonte
  FROM binocolo.ma_target t
  UNION ALL
  SELECT
    COALESCE(
      NULLIF(CASE WHEN regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
                  THEN substr(regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g'), 3)
                  ELSE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') END, ''),
      NULLIF(regexp_replace(upper(btrim(tax_code)), '[[:space:].]', '', 'g'), '')
    ), btrim(company_name), 'card'
  FROM binocolo.ma_initiative_card
  UNION ALL
  SELECT
    COALESCE(
      NULLIF(CASE WHEN regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
                  THEN substr(regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g'), 3)
                  ELSE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') END, ''),
      NULLIF(regexp_replace(upper(btrim(tax_code)), '[[:space:].]', '', 'g'), '')
    ), btrim(company_name), 'registro'
  FROM binocolo.ma_company_domain
), per_identita AS (
  SELECT
    fiscal_key,
    COUNT(DISTINCT company_name)                                                        AS n_nomi,
    COUNT(DISTINCT regexp_replace(upper(company_name), '[^0-9A-Z]', '', 'g'))            AS n_nomi_sostanziali
  FROM corpus
  WHERE fiscal_key IS NOT NULL AND company_name <> ''
  GROUP BY fiscal_key
)
SELECT
  COUNT(*)                                                    AS identita,
  COUNT(*) FILTER (WHERE n_nomi > 1)                           AS con_piu_nomi,
  COUNT(*) FILTER (WHERE n_nomi > 1 AND n_nomi_sostanziali = 1) AS solo_maiuscole_o_spazi,
  COUNT(*) FILTER (WHERE n_nomi_sostanziali > 1)                AS nomi_davvero_diversi
FROM per_identita;


-- -----------------------------------------------------------------------------
-- Q13b — I nomi divergenti in chiaro, con la fonte che li ha scritti.
-- -----------------------------------------------------------------------------
WITH corpus AS (
  SELECT
    COALESCE(
      NULLIF(CASE WHEN regexp_replace(upper(btrim(COALESCE(t.vat_code, ''))), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
                  THEN substr(regexp_replace(upper(btrim(COALESCE(t.vat_code, ''))), '[[:space:].]', '', 'g'), 3)
                  ELSE regexp_replace(upper(btrim(COALESCE(t.vat_code, ''))), '[[:space:].]', '', 'g') END, ''),
      NULLIF(regexp_replace(upper(btrim(COALESCE(t.tax_code, ''))), '[[:space:].]', '', 'g'), '')
    ) AS fiscal_key,
    btrim(COALESCE(t.company_name, '')) AS company_name,
    'target:' || COALESCE(t.enrichment_level, '?') || '/' || COALESCE(t.origin, '?') AS fonte
  FROM binocolo.ma_target t
  UNION ALL
  SELECT
    COALESCE(
      NULLIF(CASE WHEN regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
                  THEN substr(regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g'), 3)
                  ELSE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') END, ''),
      NULLIF(regexp_replace(upper(btrim(tax_code)), '[[:space:].]', '', 'g'), '')
    ), btrim(company_name), 'card'
  FROM binocolo.ma_initiative_card
  UNION ALL
  SELECT
    COALESCE(
      NULLIF(CASE WHEN regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
                  THEN substr(regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g'), 3)
                  ELSE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') END, ''),
      NULLIF(regexp_replace(upper(btrim(tax_code)), '[[:space:].]', '', 'g'), '')
    ), btrim(company_name), 'registro'
  FROM binocolo.ma_company_domain
)
SELECT
  fiscal_key,
  array_agg(DISTINCT company_name ORDER BY company_name) AS nomi,
  array_agg(DISTINCT fonte ORDER BY fonte)               AS fonti
FROM corpus
WHERE fiscal_key IS NOT NULL AND company_name <> ''
GROUP BY fiscal_key
HAVING COUNT(DISTINCT company_name) > 1
ORDER BY 1
LIMIT 40;


-- -----------------------------------------------------------------------------
-- Q14 — P.IVA e codice fiscale coincidono?
--
-- Aggiunta 2026-07-25 dopo la seconda revisione di #86, per sostituire con una
-- misura un'assunzione: avevo motivato il namespace fiscale unificato dicendo che
-- nelle società di capitali il CF è normalmente lo stesso numero a 11 cifre della
-- P.IVA. È conoscenza di dominio, non un dato nostro.
--
-- Dimensiona quante righe fiscali avrà ma_company_identifier: `coincidono` -> UNA
-- riga con entrambi i ruoli (is_vat + is_tax); `differiscono` -> DUE righe nel
-- namespace fiscale. `solo_vat` sono le aziende per cui non conosciamo il CF, cioè
-- quelle su cui DocuEngine non potrebbe cercare i bilanci senza prima risolverlo.
--
-- NB: non cambia il modello in nessuno dei due esiti — l'argomento per il namespace
-- unico è la collisione fra aziende diverse, che non dipende da questa frequenza.
-- -----------------------------------------------------------------------------
WITH corpus AS (
  SELECT COALESCE(vat_code, '') AS vat_code, COALESCE(tax_code, '') AS tax_code FROM binocolo.ma_target
  UNION ALL
  SELECT vat_code, tax_code FROM binocolo.ma_initiative_card
  UNION ALL
  SELECT vat_code, tax_code FROM binocolo.ma_company_domain
), clean AS (
  SELECT
    CASE WHEN regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
         THEN substr(regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g'), 3)
         ELSE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') END AS vat,
    regexp_replace(upper(btrim(tax_code)), '[[:space:].]', '', 'g')               AS tax
  FROM corpus
), per_identita AS (
  SELECT DISTINCT vat, tax FROM clean WHERE vat <> '' OR tax <> ''
)
SELECT
  COUNT(*)                                                      AS coppie_distinte,
  COUNT(*) FILTER (WHERE vat <> '' AND tax <> '' AND vat = tax)   AS coincidono,
  COUNT(*) FILTER (WHERE vat <> '' AND tax <> '' AND vat <> tax)  AS differiscono,
  COUNT(*) FILTER (WHERE vat <> '' AND tax =  '')                 AS solo_vat,
  COUNT(*) FILTER (WHERE vat =  '' AND tax <> '')                 AS solo_tax,
  COUNT(*) FILTER (WHERE vat <> '' AND tax <> '' AND vat <> tax
                     AND tax ~ '^[0-9A-Z]{16}$')                  AS differiscono_cf_persona_fisica
FROM per_identita;


-- =============================================================================
-- SEZIONE 2 — CUTOVER DEL REGISTRO IDENTITÀ (issue #86)
--
-- Q1..Q14 misurano lo stato PRIMA della decisione. Da qui in poi le query
-- servono a eseguire e sorvegliare l'adozione.
--
-- IL FILE NON SI ESEGUE IN BLOCCO. Metà di queste query legge oggetti che
-- CREA la migrazione 120 — la colonna ma_target.company_key, le tabelle del
-- registro, le funzioni di normalizzazione — e prima di applicarla fallisce con
-- «column/relation does not exist». Non è un guasto: è l'ordine del cutover.
--
--   PRIMA della 120        Q16  preflight del registro (deve essere VUOTA)
--                          Q19  chiavi a forma di P.IVA/CF (la causa nota)
--                          Q4   dossier comprati due volte (esito mai riportato)
--
--   DOPO la 120            Q15  parità fra le funzioni SQL e le espressioni
--
--   DOPO 120 + 121         Q17  gate: zero chiavi orfane, prima del deploy
--
--   dopo il cutover        Q18  monitor del registro, periodico
--
-- Q1..Q14 girano su qualunque schema.
--
-- Cambio di ruolo di Q3/Q11: dopo il cutover due vendor id per la stessa
-- identità NON sono più un invariante rotto — sono deriva del fornitore, e la
-- continuità è integra se entrambi puntano alla stessa ma_company. Restano come
-- SENSORE DI RINUMERAZIONE OpenAPI.it (alert), non come errore. L'invariante
-- canonico è Q18.
-- =============================================================================


-- -----------------------------------------------------------------------------
-- Q15 — Parità fra le funzioni SQL della 120 e la normalizzazione del codice.
--
-- binocolo.ma_normalize_fiscal / ma_stable_vat sono lo specchio SQL di
-- normalizeMAFiscalValue / maStableVAT (ma_filing_store.go). Qui si verifica che
-- producano esattamente il valore delle espressioni inline usate finora nel
-- codice e in questa sonda — cioè che promuovere le primitive non abbia
-- cambiato nulla. Tutte e tre le colonne di divergenza devono dare 0.
--
-- Eseguire DOPO la migrazione 120 (le funzioni non esistono prima).
-- -----------------------------------------------------------------------------
WITH corpus AS (
  SELECT COALESCE(vat_code, '') AS vat_code, COALESCE(tax_code, '') AS tax_code FROM binocolo.ma_target
  UNION ALL SELECT vat_code, tax_code FROM binocolo.ma_initiative_card
  UNION ALL SELECT vat_code, tax_code FROM binocolo.ma_company_domain
  UNION ALL SELECT COALESCE(vat_code, ''), COALESCE(tax_code, '') FROM binocolo.ma_deep_analysis
)
SELECT
  COUNT(*)                                                                          AS righe,
  COUNT(*) FILTER (WHERE binocolo.ma_normalize_fiscal(tax_code)
                      <> regexp_replace(upper(btrim(tax_code)), '[[:space:].]', '', 'g'))  AS divergenze_normalize,
  COUNT(*) FILTER (WHERE binocolo.ma_stable_vat(vat_code)
                      <> CASE WHEN regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
                              THEN substr(regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g'), 3)
                              ELSE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') END) AS divergenze_stable_vat,
  COUNT(*) FILTER (WHERE binocolo.ma_stable_vat(binocolo.ma_stable_vat(vat_code))
                      <> binocolo.ma_stable_vat(vat_code))                          AS non_idempotenti
FROM corpus;


-- -----------------------------------------------------------------------------
-- Q16 — Preflight del registro: un identificatore rivendica due aziende?
--
-- È l'ASSERZIONE che la migrazione 120 esegue e su cui SOLLEVA. Eseguirla prima
-- serve a sapere in anticipo se il cutover può partire: deve essere VUOTA.
--
-- Differisce da Q7 per direzione e granularità. Q7 chiedeva «una chiave, due
-- identità fiscali»; qui la domanda è «un VALORE, due chiavi», per ogni singolo
-- identificatore — la P.IVA e il CF contati separatamente, non collassati nella
-- fiscal_key composta. È la forma che il namespace fiscale unificato impone, e
-- l'unica che intercetta la P.IVA dell'azienda A uguale al CF dell'azienda B.
-- -----------------------------------------------------------------------------
WITH source AS (
  SELECT
    COALESCE(
      NULLIF(upper(btrim(COALESCE(t.vendor_id, ''))), ''),
      NULLIF(upper(btrim(COALESCE(t.vat_code, ''))), ''),
      NULLIF(upper(btrim(COALESCE(t.tax_code, ''))), ''),
      upper(btrim(COALESCE(t.company_name, '')))
    ) AS company_key,
    upper(btrim(COALESCE(t.vendor_id, ''))) AS vendor_id,
    COALESCE(t.vat_code, '') AS vat_code, COALESCE(t.tax_code, '') AS tax_code
  FROM binocolo.ma_target t
  UNION ALL
  SELECT upper(btrim(company_key)), '', COALESCE(vat_code, ''), COALESCE(tax_code, '') FROM binocolo.ma_initiative_card
  UNION ALL
  SELECT upper(btrim(company_key)), '', COALESCE(vat_code, ''), COALESCE(tax_code, '') FROM binocolo.ma_company_domain
  UNION ALL
  SELECT upper(btrim(company_key)), '', COALESCE(vat_code, ''), COALESCE(tax_code, '') FROM binocolo.ma_deep_analysis
  UNION ALL
  SELECT upper(btrim(company_key)), '', COALESCE(vat_code, ''), COALESCE(tax_code, '') FROM binocolo.ma_company_bm_family
), identifier AS (
  SELECT 'fiscal' AS namespace,
         CASE WHEN regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') ~ '^IT[0-9]{11}$'
              THEN substr(regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g'), 3)
              ELSE regexp_replace(upper(btrim(vat_code)), '[[:space:].]', '', 'g') END AS value,
         company_key
  FROM source WHERE btrim(COALESCE(company_key, '')) <> ''
  UNION ALL
  SELECT 'fiscal', regexp_replace(upper(btrim(tax_code)), '[[:space:].]', '', 'g'), company_key
  FROM source WHERE btrim(COALESCE(company_key, '')) <> ''
  UNION ALL
  SELECT 'vendor_openapiit', vendor_id, company_key
  FROM source WHERE btrim(COALESCE(company_key, '')) <> ''
)
SELECT namespace, value,
       COUNT(DISTINCT company_key)                             AS n_aziende,
       array_agg(DISTINCT company_key ORDER BY company_key)    AS aziende
FROM identifier
WHERE value <> ''
GROUP BY namespace, value
HAVING COUNT(DISTINCT company_key) > 1
ORDER BY 3 DESC, 1, 2
LIMIT 50;


-- -----------------------------------------------------------------------------
-- Q17 — GATE del cutover: zero company_key senza una ma_company.
--
-- Eseguire DOPO la 120 e la 121, PRIMA del deploy del binario nuovo. Copre
-- TUTTE le tabelle che portano una chiave, non i soli target: comprese quelle
-- che hanno chiavi presenti nel dettaglio SENZA una riga ma_target.
--
-- `orfane` deve essere 0 su OGNI riga; `senza_chiave` deve essere 0 dove
-- `chiave_obbligatoria`. Se non lo è, rieseguire la migrazione 121 a writer
-- fermi.
--
-- La distinzione fra chiave obbligatoria e facoltativa non è pedanteria:
-- `ma_filing_acquisition.context_company_key` è un riferimento CONTESTUALE
-- nullable (mig 113) — il filing è identificato dalla chiave fiscale, mai da
-- company_key — quindi un'acquisizione senza contesto è normale e non deve far
-- fallire il gate. Anche `ma_target.company_key` è nullable, ma lì il NULL è
-- esattamente ciò che il gate deve intercettare.
-- -----------------------------------------------------------------------------
WITH keyed AS (
  SELECT 'ma_target' AS tabella, true AS chiave_obbligatoria, company_key FROM binocolo.ma_target
  UNION ALL SELECT 'ma_target_rating', true, company_key FROM binocolo.ma_target_rating
  UNION ALL SELECT 'ma_target_web_validation', true, company_key FROM binocolo.ma_target_web_validation
  UNION ALL SELECT 'ma_sector_eval_label', true, company_key FROM binocolo.ma_sector_eval_label
  UNION ALL SELECT 'ma_target_outcome', true, company_key FROM binocolo.ma_target_outcome
  UNION ALL SELECT 'ma_initiative_card', true, company_key FROM binocolo.ma_initiative_card
  UNION ALL SELECT 'ma_company_domain', true, company_key FROM binocolo.ma_company_domain
  UNION ALL SELECT 'ma_deep_analysis', true, company_key FROM binocolo.ma_deep_analysis
  UNION ALL SELECT 'ma_deep_payload_vintage', true, company_key FROM binocolo.ma_deep_payload_vintage
  UNION ALL SELECT 'ma_company_bm_family', true, company_key FROM binocolo.ma_company_bm_family
  UNION ALL SELECT 'ma_company_fact', true, company_key FROM binocolo.ma_company_fact
  UNION ALL SELECT 'ma_company_note', true, company_key FROM binocolo.ma_company_note
  UNION ALL SELECT 'ma_card_thesis_reading', true, company_key FROM binocolo.ma_card_thesis_reading
  UNION ALL SELECT 'ma_session_thesis_reading', true, company_key FROM binocolo.ma_session_thesis_reading
  UNION ALL SELECT 'ma_card_irl_item', true, company_key FROM binocolo.ma_card_irl_item
  -- riferimento contestuale, nullable per contratto
  UNION ALL SELECT 'ma_filing_acquisition', false, context_company_key FROM binocolo.ma_filing_acquisition
)
SELECT
  tabella,
  bool_or(chiave_obbligatoria)                                                 AS chiave_obbligatoria,
  COUNT(*)                                                                     AS righe,
  COUNT(*) FILTER (WHERE chiave_obbligatoria AND btrim(COALESCE(company_key, '')) = '')
                                                                               AS senza_chiave,
  COUNT(*) FILTER (WHERE btrim(COALESCE(company_key, '')) <> '' AND NOT EXISTS (
                     SELECT 1 FROM binocolo.ma_company c WHERE c.company_key = keyed.company_key))
                                                                               AS orfane
FROM keyed
GROUP BY tabella
ORDER BY orfane DESC, senza_chiave DESC, tabella;


-- -----------------------------------------------------------------------------
-- Q18 — MONITOR del registro (periodico, dopo il cutover).
--
-- L'invariante canonico dopo l'adozione. Devono dare 0 le prime CINQUE righe,
-- compresa «aziende senza alcun identificatore»: il resolver non crea mai
-- un'entità senza identificatori — erra invece di inventarli da una ragione
-- sociale — quindi un valore > 0 può venire solo dal backfill, cioè da chiavi
-- storiche presenti unicamente nelle tabelle di dettaglio e prive della forma
-- ObjectId. Vanno guardate una per una: sono aziende che nessun identificatore
-- può più ritrovare.
--
-- Il controllo sulle chiavi che non risolvono copre TUTTE le tabelle con quella chiave:
-- finché F5 non introduce le FK verso ma_company, un writer che coni una chiave
-- fuori dal registro non incontra alcun vincolo, e restringere il monitor a
-- ma_target lo lascerebbe invisibile.
--
-- `aziende_vendor_only` > 0 non è un errore di integrità ma un DIFETTO da
-- chiudere: un'entità senza identità fiscale è precisamente ciò che non
-- sopravvive a un cambio fornitore.
--
-- Le ultime due righe non sono controlli ma dimensionamento: quante identità
-- fiscali conosciamo e quante aziende sono nate dopo il cutover.
-- -----------------------------------------------------------------------------
SELECT 'valori fiscali su due entità (impossibile: PK)' AS controllo,
       COUNT(*) AS valore
FROM (SELECT value FROM binocolo.ma_company_identifier
      WHERE namespace = 'fiscal' GROUP BY value HAVING COUNT(DISTINCT company_key) > 1) x
UNION ALL
-- Copre TUTTE le tabelle con quella chiave, non i soli target: finché F5 non introduce le
-- FK verso ma_company, nulla impedisce a un writer di scrivere una chiave che
-- non esiste, e un monitor ristretto a ma_target non lo vedrebbe. Riusa la
-- stessa lista di Q17.
SELECT 'chiavi che non risolvono, in qualunque tabella', COUNT(*)
FROM (
  SELECT company_key FROM binocolo.ma_target
  UNION ALL SELECT company_key FROM binocolo.ma_target_rating
  UNION ALL SELECT company_key FROM binocolo.ma_target_web_validation
  UNION ALL SELECT company_key FROM binocolo.ma_sector_eval_label
  UNION ALL SELECT company_key FROM binocolo.ma_target_outcome
  UNION ALL SELECT company_key FROM binocolo.ma_initiative_card
  UNION ALL SELECT company_key FROM binocolo.ma_company_domain
  UNION ALL SELECT company_key FROM binocolo.ma_deep_analysis
  UNION ALL SELECT company_key FROM binocolo.ma_deep_payload_vintage
  UNION ALL SELECT company_key FROM binocolo.ma_company_bm_family
  UNION ALL SELECT company_key FROM binocolo.ma_company_fact
  UNION ALL SELECT company_key FROM binocolo.ma_company_note
  UNION ALL SELECT company_key FROM binocolo.ma_card_thesis_reading
  UNION ALL SELECT company_key FROM binocolo.ma_session_thesis_reading
  UNION ALL SELECT company_key FROM binocolo.ma_card_irl_item
  UNION ALL SELECT context_company_key FROM binocolo.ma_filing_acquisition
) keyed
WHERE btrim(COALESCE(company_key, '')) <> ''
  AND NOT EXISTS (SELECT 1 FROM binocolo.ma_company c WHERE c.company_key = keyed.company_key)
UNION ALL
SELECT 'target senza chiave (obbligatoria)', COUNT(*)
FROM binocolo.ma_target WHERE company_key IS NULL
UNION ALL
SELECT 'conflitti identitari aperti', COUNT(*)
FROM binocolo.ma_company_identity_conflict WHERE state = 'open'
UNION ALL
SELECT 'aziende senza alcun identificatore', COUNT(*)
FROM binocolo.ma_company c
WHERE NOT EXISTS (SELECT 1 FROM binocolo.ma_company_identifier i WHERE i.company_key = c.company_key)
UNION ALL
SELECT 'aziende vendor_only (difetto, non stato normale)', COUNT(*)
FROM binocolo.ma_company WHERE identity_state = 'vendor_only'
UNION ALL
SELECT 'aziende con identità fiscale', COUNT(*)
FROM binocolo.ma_company WHERE identity_state = 'fiscal'
UNION ALL
SELECT 'aziende assegnate dopo il cutover (UUID)', COUNT(*)
FROM binocolo.ma_company WHERE origin = 'assigned';


-- -----------------------------------------------------------------------------
-- Q19 — Chiavi che sono valori fiscali: l'impronta del difetto.
--
-- Aggiunta 2026-07-25 da una ricognizione sistematica dei writer (issue #86).
--
-- Una company_key legittima ha una di due forme: l'ObjectId adottato (24 hex) o
-- un UUID assegnato. Una chiave a 11 cifre o a 16 caratteri NON è un id interno:
-- è una P.IVA o un codice fiscale usati come chiave, cioè un'identità coniata da
-- un payload. È l'impronta esatta della classe di difetto che la issue elimina, e
-- questa query la riconosce senza sapere quale writer l'abbia prodotta.
--
-- CAUSA NOTA: `companyDossier` (lo strumento standalone /azienda) accodava
-- l'analisi con EnqueueMADeepAnalysis(ctx, vat, vat, ...) — il primo argomento è
-- la company_key. Corretto nel codice, ma le righe già scritte restano.
--
-- PERCHÉ CONTA PRIMA DEL CUTOVER, e non solo come pulizia: se la stessa azienda
-- ha un dossier sotto la P.IVA E un target sotto l'ObjectId, quel valore fiscale
-- rivendica DUE company_key, e il preflight della migrazione 120 SOLLEVA. Q16 lo
-- vede; Q19 dice da dove viene. Q4 misurava già i dossier comprati due volte, ma
-- il suo esito non è mai stato riportato nella baseline: è la misura che manca.
--
-- Se `chiavi_fiscali` > 0 leggere i campioni e incrociare con Q16.
--
-- ESEGUIBILE PRIMA DELLA 120. Per ma_target la chiave è quella DERIVATA, perché
-- la colonna company_key non esiste ancora: è comunque il valore che il backfill
-- adotterà, quindi la misura vale. DOPO la 120, per misurare la colonna vera,
-- sostituire la prima riga della UNION con:
--
--   SELECT 'ma_target' AS tabella, company_key FROM binocolo.ma_target
--
-- Tutte le altre tabelle hanno già la loro colonna e non cambiano.
-- -----------------------------------------------------------------------------
WITH keyed AS (
  SELECT 'ma_target (chiave derivata)' AS tabella,
         COALESCE(
           NULLIF(upper(btrim(COALESCE(vendor_id, ''))), ''),
           NULLIF(upper(btrim(COALESCE(vat_code, ''))), ''),
           NULLIF(upper(btrim(COALESCE(tax_code, ''))), ''),
           upper(btrim(COALESCE(company_name, '')))
         ) AS company_key
  FROM binocolo.ma_target
  UNION ALL SELECT 'ma_deep_analysis', company_key FROM binocolo.ma_deep_analysis
  UNION ALL SELECT 'ma_deep_payload_vintage', company_key FROM binocolo.ma_deep_payload_vintage
  UNION ALL SELECT 'ma_company_bm_family', company_key FROM binocolo.ma_company_bm_family
  UNION ALL SELECT 'ma_initiative_card', company_key FROM binocolo.ma_initiative_card
  UNION ALL SELECT 'ma_company_domain', company_key FROM binocolo.ma_company_domain
  UNION ALL SELECT 'ma_target_rating', company_key FROM binocolo.ma_target_rating
  UNION ALL SELECT 'ma_target_web_validation', company_key FROM binocolo.ma_target_web_validation
  UNION ALL SELECT 'ma_sector_eval_label', company_key FROM binocolo.ma_sector_eval_label
  UNION ALL SELECT 'ma_target_outcome', company_key FROM binocolo.ma_target_outcome
  UNION ALL SELECT 'ma_company_fact', company_key FROM binocolo.ma_company_fact
  UNION ALL SELECT 'ma_company_note', company_key FROM binocolo.ma_company_note
  UNION ALL SELECT 'ma_card_thesis_reading', company_key FROM binocolo.ma_card_thesis_reading
  UNION ALL SELECT 'ma_session_thesis_reading', company_key FROM binocolo.ma_session_thesis_reading
  UNION ALL SELECT 'ma_card_irl_item', company_key FROM binocolo.ma_card_irl_item
  UNION ALL SELECT 'ma_filing_acquisition', context_company_key FROM binocolo.ma_filing_acquisition
), clean AS (
  SELECT tabella, upper(btrim(company_key)) AS k
  FROM keyed WHERE btrim(COALESCE(company_key, '')) <> ''
)
SELECT
  tabella,
  COUNT(DISTINCT k)                                                          AS chiavi_distinte,
  COUNT(DISTINCT k) FILTER (WHERE k ~ '^[0-9A-F]{24}$')                       AS objectid,
  COUNT(DISTINCT k) FILTER (WHERE k ~ '^[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}$') AS uuid,
  COUNT(DISTINCT k) FILTER (WHERE k ~ '^(IT)?[0-9]{11}$' OR k ~ '^[0-9A-Z]{16}$') AS chiavi_fiscali,
  COUNT(DISTINCT k) FILTER (WHERE k !~ '^[0-9A-F]{24}$'
                              AND k !~ '^[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}$'
                              AND k !~ '^(IT)?[0-9]{11}$'
                              AND k !~ '^[0-9A-Z]{16}$')                      AS altra_forma,
  (array_agg(DISTINCT k) FILTER (WHERE k ~ '^(IT)?[0-9]{11}$' OR k ~ '^[0-9A-Z]{16}$'))[1:5] AS campioni_fiscali
FROM clean
GROUP BY tabella
ORDER BY chiavi_fiscali DESC, tabella;
