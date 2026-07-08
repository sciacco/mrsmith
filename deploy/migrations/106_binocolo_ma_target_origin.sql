-- Binocolo M&A target origin.
-- Marks whether a target entered the session through the search funnel or by an
-- explicit analyst/manual insertion. Existing rows are search-created by default.
--
-- Target database: Anisetta PostgreSQL (schema binocolo). Idempotente. Apply after 105.

BEGIN;

ALTER TABLE binocolo.ma_target
  ADD COLUMN IF NOT EXISTS origin text NOT NULL DEFAULT 'search';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'ma_target_origin_check'
  ) THEN
    ALTER TABLE binocolo.ma_target
      ADD CONSTRAINT ma_target_origin_check
      CHECK (origin IN ('search', 'manual'));
  END IF;
END $$;

COMMIT;
