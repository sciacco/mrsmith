-- Binocolo Target M&A saved-search lifecycle.
-- Additive: sessions can be archived or soft-deleted without deleting child data.

ALTER TABLE binocolo.ma_session
  ADD COLUMN IF NOT EXISTS archived_at timestamptz,
  ADD COLUMN IF NOT EXISTS archived_by_subject text,
  ADD COLUMN IF NOT EXISTS archived_by_email text,
  ADD COLUMN IF NOT EXISTS deleted_at timestamptz,
  ADD COLUMN IF NOT EXISTS deleted_by_subject text,
  ADD COLUMN IF NOT EXISTS deleted_by_email text;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'ma_session_single_lifecycle_state_check'
      AND conrelid = 'binocolo.ma_session'::regclass
  ) THEN
    ALTER TABLE binocolo.ma_session
      ADD CONSTRAINT ma_session_single_lifecycle_state_check
      CHECK (deleted_at IS NULL OR archived_at IS NULL);
  END IF;
END;
$$;

CREATE INDEX IF NOT EXISTS ma_session_active_updated_at_idx
  ON binocolo.ma_session (updated_at DESC, created_at DESC)
  WHERE archived_at IS NULL AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS ma_session_archived_updated_at_idx
  ON binocolo.ma_session (archived_at DESC, updated_at DESC)
  WHERE archived_at IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS ma_session_deleted_updated_at_idx
  ON binocolo.ma_session (deleted_at DESC, updated_at DESC)
  WHERE deleted_at IS NOT NULL;
