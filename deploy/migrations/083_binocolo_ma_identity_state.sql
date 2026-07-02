-- 083: persist the domain-identity certainty level on web validations.
--
-- Phase (a) of the "asimmetria identitaria" design (2026-07-02): a gate verdict
-- is only as valid as the premise "this site belongs to this company", and the
-- consequence of judging the WRONG site is asymmetric — a false keep costs one
-- €0.10 enrichment and self-corrects on real financials, a false reject hides
-- the company forever (the TAYLA case: name-matched onto an unrelated site and
-- rejected on someone else's business).
--
-- This migration only records the FACT per validation:
--   verified — target's P.IVA / codice fiscale confirmed on-page
--   vouched  — operator associated the domain (or manual registry entry)
--   assumed  — accepted via name-match / brand-trust / score-only
--   NULL     — legacy rows (pre-083) and unresolved domains
--
-- No behavior changes here. Phase (b) measures reject × identity_state on real
-- runs; phase (c) — if the data supports it — applies the read-time rule
-- "reject may suppress only from verified/vouched" in the bucket mappings.
--
-- Idempotent. Target: Anisetta (schema binocolo). Applied manually by the user.

ALTER TABLE binocolo.ma_target_web_validation
    ADD COLUMN IF NOT EXISTS identity_state text;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ma_target_web_validation_identity_state_check'
          AND conrelid = 'binocolo.ma_target_web_validation'::regclass
    ) THEN
        ALTER TABLE binocolo.ma_target_web_validation
            ADD CONSTRAINT ma_target_web_validation_identity_state_check
            CHECK (identity_state IS NULL OR identity_state IN ('verified', 'vouched', 'assumed'));
    END IF;
END $$;
