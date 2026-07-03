-- Binocolo M&A — soglie RAG per famiglia di business model + haircut PMI
-- graduato per taglia (Fase 3). Target: Anisetta PostgreSQL, schema binocolo.
-- Applicata a mano dall'utente. Idempotente (ON CONFLICT DO NOTHING).
--
-- Le soglie settoriali toccano SOLO le metriche strutturalmente dipendenti dal
-- modello di business (margine EBITDA, ROS, ciclo finanziario) — leva e
-- liquidita' restano uniformi (e dal redesign sono metriche "di contorno").
-- Valori dalla tabella di DEEP-DIVE-DECISIONS.md §5.3 (prassi, tarabili qui).
-- Haircut graduato: sostituisce il flat 30% (sme_haircut_pct resta come
-- fallback quando il fatturato non e' noto): <5M -> tier1, 5-20M -> tier2
-- (= status quo 30), >20M -> tier3.

BEGIN;

INSERT INTO binocolo.ma_parameter (key, value, value_type, label, description) VALUES
  -- Haircut graduato per taglia (fatturato)
  ('haircut_pct_tier1',    '35',       'percent', 'Haircut PMI tier 1 (%)',            'Sconto sui multipli per fatturato sotto la soglia tier 1 (micro/small: illiquidita'' massima).'),
  ('haircut_pct_tier2',    '30',       'percent', 'Haircut PMI tier 2 (%)',            'Sconto sui multipli per fatturato tra le soglie tier 1 e tier 2 (= flat storico).'),
  ('haircut_pct_tier3',    '20',       'percent', 'Haircut PMI tier 3 (%)',            'Sconto sui multipli per fatturato sopra la soglia tier 2 (mid-market).'),
  ('haircut_tier1_max_eur','5000000',  'money',   'Soglia fatturato tier 1 (€)',       'Sotto questo fatturato si applica haircut tier 1.'),
  ('haircut_tier2_max_eur','20000000', 'money',   'Soglia fatturato tier 2 (€)',       'Sotto questo fatturato (e sopra tier 1) si applica haircut tier 2; oltre, tier 3.'),

  -- Servizi ricorrenti (MSP, hosting, TLC gestite)
  ('rag_margin_ok_servizi_ricorrenti',   '10', 'percent', 'Margine EBITDA ok — servizi ricorrenti (%)',  'Sotto = rosso, sopra = ambra fino alla soglia good.'),
  ('rag_margin_good_servizi_ricorrenti', '18', 'percent', 'Margine EBITDA good — servizi ricorrenti (%)','Sopra = verde.'),
  ('rag_ros_ok_servizi_ricorrenti',      '6',  'percent', 'ROS ok — servizi ricorrenti (%)',             'Sotto = rosso.'),
  ('rag_ros_good_servizi_ricorrenti',    '12', 'percent', 'ROS good — servizi ricorrenti (%)',           'Sopra = verde.'),
  ('rag_ciclo_good_servizi_ricorrenti',  '30', 'number',  'Ciclo finanziario good — servizi ricorrenti (gg)', 'Sotto = verde (canoni anticipati).'),
  ('rag_ciclo_ok_servizi_ricorrenti',    '75', 'number',  'Ciclo finanziario ok — servizi ricorrenti (gg)',   'Sopra = rosso.'),

  -- Progetto & integrazione (SI, impiantistica tecnologica)
  ('rag_margin_ok_progetto_integrazione',   '6',  'percent', 'Margine EBITDA ok — progetto/integrazione (%)',  'Sotto = rosso.'),
  ('rag_margin_good_progetto_integrazione', '12', 'percent', 'Margine EBITDA good — progetto/integrazione (%)','Sopra = verde.'),
  ('rag_ros_ok_progetto_integrazione',      '4',  'percent', 'ROS ok — progetto/integrazione (%)',             'Sotto = rosso.'),
  ('rag_ros_good_progetto_integrazione',    '8',  'percent', 'ROS good — progetto/integrazione (%)',           'Sopra = verde.'),
  ('rag_ciclo_good_progetto_integrazione',  '90', 'number',  'Ciclo finanziario good — progetto/integrazione (gg)', 'Commesse: fisiologicamente lungo.'),
  ('rag_ciclo_ok_progetto_integrazione',    '150','number',  'Ciclo finanziario ok — progetto/integrazione (gg)',   'Sopra = rosso.'),

  -- Rivendita / VAR (distribuzione HW/SW)
  ('rag_margin_ok_rivendita_var',   '3',  'percent', 'Margine EBITDA ok — rivendita/VAR (%)',  'Sotto = rosso.'),
  ('rag_margin_good_rivendita_var', '7',  'percent', 'Margine EBITDA good — rivendita/VAR (%)','Sopra = verde.'),
  ('rag_ros_ok_rivendita_var',      '2',  'percent', 'ROS ok — rivendita/VAR (%)',             'Sotto = rosso.'),
  ('rag_ros_good_rivendita_var',    '5',  'percent', 'ROS good — rivendita/VAR (%)',           'Sopra = verde.'),
  ('rag_ciclo_good_rivendita_var',  '45', 'number',  'Ciclo finanziario good — rivendita/VAR (gg)', 'Rotazione merce.'),
  ('rag_ciclo_ok_rivendita_var',    '90', 'number',  'Ciclo finanziario ok — rivendita/VAR (gg)',   'Sopra = rosso.'),

  -- Software prodotto (ISV, SaaS)
  ('rag_margin_ok_software_prodotto',   '15', 'percent', 'Margine EBITDA ok — software prodotto (%)',  'Sotto = rosso.'),
  ('rag_margin_good_software_prodotto', '25', 'percent', 'Margine EBITDA good — software prodotto (%)','Sopra = verde.'),
  ('rag_ros_ok_software_prodotto',      '10', 'percent', 'ROS ok — software prodotto (%)',             'Sotto = rosso.'),
  ('rag_ros_good_software_prodotto',    '18', 'percent', 'ROS good — software prodotto (%)',           'Sopra = verde.'),
  ('rag_ciclo_good_software_prodotto',  '15', 'number',  'Ciclo finanziario good — software prodotto (gg)', 'Ricavi anticipati: ciclo corto o negativo.'),
  ('rag_ciclo_ok_software_prodotto',    '60', 'number',  'Ciclo finanziario ok — software prodotto (gg)',   'Sopra = rosso.')
ON CONFLICT (key) DO NOTHING;

COMMIT;
