-- Binocolo — alza il cap superficie gated-search a 1000.
--
-- Il valore 100 seed della 074 era = ceiling del batch gate (maWebValidationMaxLimit):
-- oltre 100, le aziende in eccesso restavano SENZA verdetto del gate e cadevano come
-- forse → Advanced pagato senza gate. Ora gatedSearchJobWork gata l'INTERA superficie
-- (gatePayload.Limit = len(targets)), quindi il cap può scalare oltre 100 in sicurezza.
--
-- La filosofia del funnel è superfici larghe: il gate (~€0.02/az) è un biglietto
-- d'ingresso e solo i sopravvissuti pagano l'Advanced (€0.10), quindi la larghezza si
-- disaccoppia dal costo. 1000 resta un freno anti-runaway (gate ~€20 a superficie piena),
-- non un via libera. Editabile a runtime da Config (ma_parameter).
--
-- UPSERT: aggiorna la riga esistente (se la 074 è stata applicata, era '100') oppure la
-- crea (se la 074 non è stata applicata). Allineato al default compilato
-- maGatedSurfaceCapDefault = 1000.
--
-- Target database: Anisetta PostgreSQL (schema binocolo). Idempotente. Apply after 076.

BEGIN;

INSERT INTO binocolo.ma_parameter (key, value, value_type, label, description) VALUES
  ('gated_surface_cap', '1000', 'number', 'Cap superficie gated-search (aziende)',
   'Massimo numero di aziende ammesse a una gated-search e freno di costo primario (il gate spende ~€0.02 su TUTTA la superficie; solo i sopravvissuti pagano l''Advanced €0.10). Il gate copre l''intera superficie, quindi il cap può superare le 100 aziende del batch standalone. Backstop anti-runaway, non un via libera.')
ON CONFLICT (key) DO UPDATE
  SET value = EXCLUDED.value,
      description = EXCLUDED.description;

COMMIT;
