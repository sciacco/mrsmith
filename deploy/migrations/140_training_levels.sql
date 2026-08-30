-- 140_training_levels.sql
-- Livelli sulla scala 0-5 delle valutazioni (skill_assessment resta la
-- sorgente unica del livello posseduto):
--   * la certificazione dichiara quale livello attesta sulla sua area
--     (facoltativo); al conseguimento con esame superato il backend lo
--     travasa in una valutazione con fonte «documento verificato»;
--   * as-is/to-be come attributi dell'esigenza, per area: la coppia livello
--     attuale -> atteso vive sul legame richiesta<->area, facoltativa,
--     scritta nel gesto di registrazione.

BEGIN;

ALTER TABLE training.certification
  ADD COLUMN IF NOT EXISTS attested_level smallint
    CHECK (attested_level IS NULL OR attested_level BETWEEN 0 AND 5);

ALTER TABLE training.training_request_skill_area
  ADD COLUMN IF NOT EXISTS level_current smallint
    CHECK (level_current IS NULL OR level_current BETWEEN 0 AND 5),
  ADD COLUMN IF NOT EXISTS level_target smallint
    CHECK (level_target IS NULL OR level_target BETWEEN 0 AND 5);

COMMIT;
