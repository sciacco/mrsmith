-- Binocolo — abbassa la soglia relativa del retrieval ATECO da 0.75 a 0.65.
--
-- La soglia è RELATIVA al miglior coseno (relCut = top × soglia): tiene un concetto KB
-- se coseno >= relCut. A 0.75 una tesi multi-faccia collassa sul concetto dominante:
-- quando il top è un outlier (es. managed_services 0.68 vs il cluster reale 0.42–0.49),
-- relCut cade nel vuoto e azzera le facce adiacenti (cloud/datacenter/cyber), che quindi
-- non entrano mai nella rete di ricerca (divisione singola). Il gate a valle non può
-- recuperarle: ciò che non si cerca non si filtra.
--
-- 0.65 validato con un fan-out di 10 tesi indipendenti su tutte le 8 divisioni della KB
-- (26/27/33/46/58/61/62/63) più casi di confine:
--   - regime OUTLIER-TOP (data-ai/software-house/msp, gap 0.14–0.19): a 0.75 la rete
--     collassa a 1–2 concetti; a 0.65 recupera le facce secondarie legittime restando
--     nella divisione corretta.
--   - regime CLUSTER-COERENTE (gap piccolo): a 0.65 allarga in modo modesto e pertinente
--     alle divisioni adiacenti reali; nessun settore improbabile entra.
--   - invarianti: i concetti distractor non contribuiscono MAI codici ATECO (esclusione
--     dei settori improbabili intatta); il cap=12 (ateco_retrieval_cap) resta il freno
--     dominante; 0.55 è troppo largo (deriva), la banda sicura è 0.62–0.68.
--
-- UPSERT: aggiorna la riga esistente (seed 061 = '0.75') o la crea. Allineato al default
-- compilato maAtecoRetrievalRelThresholdDefault = 0.65. Editabile a runtime da Config.
--
-- Target database: Anisetta PostgreSQL (schema binocolo). Idempotente. Apply after 077.

BEGIN;

INSERT INTO binocolo.ma_parameter (key, value, value_type, label, description) VALUES
  ('ateco_retrieval_rel_threshold', '0.65', 'number', 'Soglia relativa retrieval ATECO',
   'Mantiene un concetto KB se coseno >= top*soglia (relativa al miglior coseno, non assoluta). Abbassata da 0.75 a 0.65 per recuperare le facce adiacenti di una tesi multi-faccia quando il concetto top è un outlier; il gate a valle filtra la precisione e il cap (ateco_retrieval_cap) limita l''ampiezza. Validata su fan-out 10 settori.')
ON CONFLICT (key) DO UPDATE
  SET value = EXCLUDED.value,
      description = EXCLUDED.description;

COMMIT;
