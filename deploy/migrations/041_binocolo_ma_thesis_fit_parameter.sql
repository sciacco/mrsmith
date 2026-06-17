-- Binocolo M&A — parametro: sconto "controllo holding" sotto tesi successione.
-- Target database: Anisetta PostgreSQL. Apply after 038.
-- Leva di business: quanto demolire lo score di un'azienda controllata da una
-- holding quando la tesi e' successione (un socio di controllo societario e'
-- l'antitesi di un proprietario-persona prossimo all'uscita). E' uno sconto sul
-- punteggio, NON un knockout: l'azienda resta visibile in shortlist, solo piu' in
-- basso. scoreMATargetsV2 legge il valore via loadPricing (factor = 1 - pct/100).

BEGIN;

INSERT INTO binocolo.ma_parameter (key, value, value_type, label, description) VALUES
  ('thesis_fit_holding_haircut_pct', '60', 'percent', 'Sconto controllo holding — successione (%)', 'Riduzione del punteggio per aziende controllate da una holding quando la tesi e'' successione. 0% disattiva; valori vicini al 100% le affossano (resta comunque una demozione, non una rimozione).')
ON CONFLICT (key) DO NOTHING;

COMMIT;
