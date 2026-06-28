-- Binocolo use-case-1 ATECO retrieval calibration (ma_parameter).
--   * ateco_retrieval_method: kill switch. 'embedding' (default) uses the curated-KB
--     concept retrieval; 'llm' forces the legacy hierarchy resolver. With 'embedding'
--     the fallback to the resolver is automatic when KB/embedder are unavailable.
--   * thresholds are RELATIVE to the top concept's cosine, never absolute — validated
--     retrieval shows the bullseye cosine drifting per query (~0.58 vs ~0.73), so a
--     fixed floor would reject good matches. rel_threshold keeps a concept when
--     cosine >= top*rel; core_ratio splits core vs weak at top*core_ratio; floor is a
--     small absolute guard that flags "no real match".
-- Defaults mirror the Go constants in ma_ateco_retrieval.go; the loader degrades to
-- those if a row is missing, so this seed is for visibility/tunability.
--
-- Target database: Anisetta PostgreSQL (binocolo schema). Requires migration 038
-- (ma_parameter table) and 052 (value_type 'text'). Apply manually on the database
-- referenced by ANISETTA_DSN. Idempotent (existing operator-tuned values preserved).

BEGIN;

INSERT INTO binocolo.ma_parameter (key, value, value_type, label, description) VALUES
  ('ateco_retrieval_method',        'embedding', 'text',   'Metodo retrieval ATECO',           'embedding: retrieval deterministico sulla KB curata (use case 1). llm: forza il resolver gerarchico legacy. Con embedding il fallback al resolver è automatico se KB/embedder non disponibili.'),
  ('ateco_retrieval_rel_threshold', '0.75',      'number', 'Soglia relativa retrieval ATECO',  'Mantiene un concetto se coseno >= top*soglia. Relativa al miglior coseno, non assoluta.'),
  ('ateco_retrieval_core_ratio',    '0.92',      'number', 'Soglia core/weak retrieval ATECO', 'Un concetto è core se coseno >= top*ratio, altrimenti weak.'),
  ('ateco_retrieval_floor',         '0.30',      'number', 'Pavimento assoluto retrieval ATECO','Se il miglior coseno è sotto questo valore nessun match è affidabile (settore non riconosciuto).'),
  ('ateco_retrieval_cap',           '12',        'number', 'Massimo concetti retrieval ATECO',  'Numero massimo di concetti considerati oltre la soglia relativa.')
ON CONFLICT (key) DO NOTHING;

COMMIT;
