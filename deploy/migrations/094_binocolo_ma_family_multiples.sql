-- Binocolo M&A — righe Damodaran per famiglia di business model (Fase 3).
-- Target: Anisetta PostgreSQL, schema binocolo. Applicata a mano dall'utente.
-- Idempotente (ON CONFLICT DO NOTHING).
--
-- Stessa tabella e stesso dataset gratuito della mig 039 (Damodaran NYU Stern
-- Europe, vintage 2026-01-05, confermata dall'utente = docs/psEurope.xls +
-- vebitdaEurope.xls; valori da docs/sector_valuation_multiples.json): si
-- aggiungono chiavi sintetiche FAMILY:* che la risoluzione prova PRIMA della
-- catena prefissi ATECO. La classificazione segue l'identità dell'azienda
-- (famiglia ratificata dall'analista o suggerita dal motore), non il codice
-- camerale: la software house codificata 6202 o l'MSP codificato 6201 smettono
-- di prendere il multiplo sbagliato.

BEGIN;

INSERT INTO binocolo.sector_valuation_multiple
  (ateco_prefix, damodaran_industry, ev_ebitda, ev_sales, n_firms, source, source_date)
VALUES
  ('FAMILY:servizi_ricorrenti',   'Computer Services',                12.03, 1.29, 210, 'Damodaran NYU Stern - Europe', '2026-01-05'),
  ('FAMILY:progetto_integrazione','Engineering/Construction',          9.71, 0.85, 160, 'Damodaran NYU Stern - Europe', '2026-01-05'),
  ('FAMILY:rivendita_var',        'Retail (Distributors)',            11.64, 1.30, 117, 'Damodaran NYU Stern - Europe', '2026-01-05'),
  ('FAMILY:software_prodotto',    'Software (System & Application)',  20.85, 5.31, 290, 'Damodaran NYU Stern - Europe', '2026-01-05')
ON CONFLICT (ateco_prefix) DO NOTHING;

COMMIT;
