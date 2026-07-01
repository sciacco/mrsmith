-- Binocolo — parametri costo per la gated-search pipeline (step 1).
--
-- La nuova pipeline paga, per azienda su TUTTA la superficie, un enrichment
-- Address (identità+sede) invece dell'Advanced, più lo scrape/crawl e la search
-- di dominio del gate; l'Advanced (€0.10) lo pagano solo i sopravvissuti keep+forse.
-- L'estimate v2 proietta: costo CERTO del gate (N × per-azienda) + costo ATTESO
-- Advanced (N × tasso_sopravvivenza × cost_advanced), come banda. Con la spesa
-- "tutto automatico", questa proiezione a monte + il cap sulla superficie sono
-- l'unico freno di costo.
--
-- Oggi in ma_parameter c'è solo cost_advanced_eur/full/dryrun. Qui si aggiungono
-- i costi unitari mancanti e il tasso di sopravvivenza atteso (calibrabile dai run).
-- I costi fastcrw sono in $ ma a questa scala si trattano ≈ €.
--
-- Target database: Anisetta PostgreSQL (schema binocolo). Idempotente. Apply after 073.

BEGIN;

INSERT INTO binocolo.ma_parameter (key, value, value_type, label, description) VALUES
  ('cost_address_eur',      '0.01',  'money',  'Costo enrichment address (€/azienda)',   'Prezzo OpenAPI.it IT-address (solo identità+sede) per azienda: primo passo del gate.'),
  ('cost_scrape_page_eur',  '0.001', 'money',  'Costo scrape/crawl (€/pagina)',          'Prezzo fastcrw per pagina scrapeata/crawlata nella risoluzione dominio + evidenza del gate.'),
  ('cost_search_eur',       '0.001', 'money',  'Costo search dominio (€/ricerca)',       'Prezzo fastcrw per ricerca (5 risultati), fallback nella risoluzione dominio.'),
  ('survivor_rate_default', '0.35',  'number', 'Tasso sopravvivenza keep+forse atteso',  'Frazione attesa (0-1) di aziende che il gate promuove a keep/forse e che pagano Advanced. Stima iniziale, calibrata dai run reali.')
ON CONFLICT (key) DO NOTHING;

COMMIT;
