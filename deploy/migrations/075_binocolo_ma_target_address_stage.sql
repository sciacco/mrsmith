-- Binocolo — ma_target per lo stadio address della gated-search pipeline (step 2).
--
-- La nuova pipeline popola ma_target in DUE momenti:
--   1) stadio address: identità (nome/P.IVA/CF/sede/GPS/ATECO) per TUTTA la
--      superficie, SENZA punteggio (Address €0.01, nessuno scoring);
--   2) stadio enrich_score: Advanced €0.10 + scoring SOLO sui sopravvissuti
--      keep/forse, che valorizza score/match_state e alza enrichment_level.
--
-- Oggi score/match_state sono NOT NULL (li scrive sempre lo scoring). Per lo
-- stadio address vanno resi NULL-abili. enrichment_level marca il livello di
-- arricchimento della riga ('address' identity-only | 'advanced' scorata) e
-- default 'advanced' per compat con le righe execute esistenti.
--
-- I CHECK esistenti (score BETWEEN 0 AND 100, match_state IN (...)) restano
-- validi: un CHECK passa quando l'espressione è NULL, quindi tollerano già le
-- righe address non scorate senza modifiche.
--
-- Target database: Anisetta PostgreSQL (schema binocolo). Idempotente. Apply after 074.

BEGIN;

ALTER TABLE binocolo.ma_target ALTER COLUMN score DROP NOT NULL;
ALTER TABLE binocolo.ma_target ALTER COLUMN match_state DROP NOT NULL;

ALTER TABLE binocolo.ma_target
  ADD COLUMN IF NOT EXISTS enrichment_level text NOT NULL DEFAULT 'advanced';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'ma_target_enrichment_level_check'
  ) THEN
    ALTER TABLE binocolo.ma_target
      ADD CONSTRAINT ma_target_enrichment_level_check
      CHECK (enrichment_level IN ('address', 'advanced'));
  END IF;
END $$;

COMMIT;
