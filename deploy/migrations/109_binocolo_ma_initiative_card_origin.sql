-- Direct initiative cards: explicit provenance and domain-verification diary event.
-- Target: Anisetta PostgreSQL. Idempotent. Apply before the matching backend.

BEGIN;

ALTER TABLE binocolo.ma_initiative_card
  ADD COLUMN IF NOT EXISTS origin text NOT NULL DEFAULT 'search';
ALTER TABLE binocolo.ma_initiative_card DROP CONSTRAINT IF EXISTS ma_initiative_card_origin_check;
ALTER TABLE binocolo.ma_initiative_card ADD CONSTRAINT ma_initiative_card_origin_check
  CHECK (origin IN ('search', 'direct'));

ALTER TABLE binocolo.ma_target_outcome DROP CONSTRAINT IF EXISTS ma_target_outcome_event_check;
ALTER TABLE binocolo.ma_target_outcome ADD CONSTRAINT ma_target_outcome_event_check
  CHECK (event IN ('contattato', 'buon_lead', 'no_go', 'card_creata', 'card_rimossa', 'card_riaperta', 'stato', 'nota', 'chiusura', 'dominio_verificato'));

COMMIT;
