-- 053_binocolo_ma_session_purge.sql
-- Adds a "purged" lifecycle state to binocolo.ma_session: a tombstone that hides
-- a trashed session from every UI view (including the trash) permanently. No data
-- is deleted — child rows, audit, cost and operation traces are all preserved.
-- A session can only be purged from the trash, so purged_at implies deleted_at.

ALTER TABLE binocolo.ma_session
  ADD COLUMN IF NOT EXISTS purged_at         timestamptz,
  ADD COLUMN IF NOT EXISTS purged_by_subject text,
  ADD COLUMN IF NOT EXISTS purged_by_email   text;

ALTER TABLE binocolo.ma_session
  DROP CONSTRAINT IF EXISTS ma_session_purge_requires_delete_check;

ALTER TABLE binocolo.ma_session
  ADD CONSTRAINT ma_session_purge_requires_delete_check
  CHECK (purged_at IS NULL OR deleted_at IS NOT NULL);
