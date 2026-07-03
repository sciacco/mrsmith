-- Binocolo M&A — parametri del bridge EV→equity e dei flag di qualità EBITDA
-- (Fase 2 del redesign deep-dive, DEEP-DIVE-IMPLEMENTATION-PLAN.md).
-- Target: Anisetta PostgreSQL, schema binocolo. Applicata a mano dall'utente.
-- Idempotente (ON CONFLICT DO NOTHING). Convenzioni = policy di team, non scelte
-- per-analista: si cambiano di rado, con audit, e valgono per tutti i dossier.
--
-- vendor_cee_tolerance_pct: calibrata dall'inspect di Fase 0 su n=10 (rumore max
-- osservato 0,03% da rounding dei ratio; un mismatch definitorio vero è in scala
-- percentuale piena → 1% dà 30x di margine).

BEGIN;

INSERT INTO binocolo.ma_parameter (key, value, value_type, label, description) VALUES
  ('tfr_bridge_pct',                '100', 'percent', 'TFR nel bridge equity (%)',            'Quota del fondo TFR sottratta come debt-like nel ponte EV -> equity (prassi 80-100; screening prudente = 100).'),
  ('a5_ebitda_flag_pct',            '20',  'percent', 'Soglia flag altri ricavi A.5 (%)',     'Sopra questa quota di A.5 sull''EBITDA scatta il flag qualita'' margine (composizione altri ricavi da chiarire in DD).'),
  ('b8_revenue_flag_pct',           '8',   'percent', 'Soglia flag godimento beni terzi (%)', 'Sopra questa quota di B.8 sui ricavi scatta il flag informativo (struttura in affitto/noleggio: impegni da ereditare).'),
  ('participation_assets_flag_pct', '25',  'percent', 'Soglia flag partecipazioni/attivo (%)','Sopra questa quota di partecipazioni sull''attivo scatta il flag perimetro standalone (la banda non vede le controllate).'),
  ('participation_income_flag_pct', '20',  'percent', 'Soglia flag proventi C.15/EBITDA (%)', 'Sopra questa quota di proventi da partecipazioni sull''EBITDA scatta il flag perimetro standalone.'),
  ('vendor_cee_tolerance_pct',      '1',   'percent', 'Tolleranza scarto vendor vs CEE (%)',  'Oltre questo scarto tra KPI vendor e rilettura CEE scatta il flag di riconciliazione dati (rumore osservato: 0,03%).')
ON CONFLICT (key) DO NOTHING;

COMMIT;
