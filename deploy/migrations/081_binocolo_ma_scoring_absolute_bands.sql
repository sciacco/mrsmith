-- Binocolo M&A — scoring: bande assolute al posto dei percentili + versione dello score.
--
-- I segnali trend fatturato e produttività erano percentili calcolati sul pool dei
-- sopravvissuti del run: stesso bilancio → punteggio diverso a seconda di chi altro
-- c'era nella ricerca (non stazionario, non confrontabile cross-run, degenerato sui
-- pool piccoli: n<=1 → 0.5 fisso). Diventano rampe ASSOLUTE ancorate: stesso input →
-- stesso punteggio, sempre. Le ancore sono parametri dichiarati (default PROVVISORI
-- per PMI ICT, ritoccabili da UI senza deploy); i punteggi ai capi delle rampe sono
-- fissi in codice (floor 0.1, zero-crescita 0.5, top 1.0).
--
-- trend_cagr_decline_floor_pct è il MODULO del declino (10 = -10%/anno): il parser
-- dei parametri rifiuta i valori negativi.
--
-- ma_target.score_version etichetta i punteggi persistiti: NULL = versioni precedenti
-- (percentili pool-relativi, viability che puniva il dato assente), 3 = catalogo v2
-- con bande assolute e viability assente≠distressed. Punteggi con versione diversa
-- NON sono confrontabili tra loro.
--
-- Target database: Anisetta PostgreSQL (schema binocolo). Idempotente. Apply after 080.

BEGIN;

ALTER TABLE binocolo.ma_target
  ADD COLUMN IF NOT EXISTS score_version smallint;

INSERT INTO binocolo.ma_parameter (key, value, value_type, label, description) VALUES
  ('trend_cagr_decline_floor_pct', '10',     'percent', 'Trend: declino al floor (%/anno)',      'Declino annuo (mediana YoY, in modulo) a cui il sotto-score trend tocca il minimo 0.1. Tra -floor e 0 il punteggio sale lineare fino a 0.5.'),
  ('trend_cagr_top_pct',           '15',     'percent', 'Trend: crescita al massimo (%/anno)',   'Crescita annua (mediana YoY) a cui il sotto-score trend raggiunge 1.0. Tra 0 e top il punteggio sale lineare da 0.5.'),
  ('productivity_floor_eur',       '50000',  'money',   'Produttività: floor (€/dipendente)',    'Fatturato per dipendente a cui il sotto-score produttività tocca il minimo 0.1. Ancora provvisoria (PMI ICT), da calibrare su dati reali.'),
  ('productivity_top_eur',         '200000', 'money',   'Produttività: massimo (€/dipendente)',  'Fatturato per dipendente a cui il sotto-score produttività raggiunge 1.0. Rampa lineare tra floor e top. Ancora provvisoria (PMI ICT).')
ON CONFLICT (key) DO NOTHING;

COMMIT;
