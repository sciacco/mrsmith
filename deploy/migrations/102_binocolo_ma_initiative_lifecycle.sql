-- 102_binocolo_ma_initiative_lifecycle.sql
-- Adds trash/purge tombstones to binocolo.ma_initiative, mirroring the
-- ma_session lifecycle columns from migrations 035 and 053.
-- Deployment note: apply this migration before deploying the matching backend;
-- initiative list/get SELECTs depend on these columns being present.
-- Idempotent. Purged initiatives remain as audit tombstones but are hidden from
-- every visibility, including deleted/trash.

ALTER TABLE binocolo.ma_initiative
  ADD COLUMN IF NOT EXISTS deleted_at         timestamptz,
  ADD COLUMN IF NOT EXISTS deleted_by_subject text,
  ADD COLUMN IF NOT EXISTS deleted_by_email   text,
  ADD COLUMN IF NOT EXISTS purged_at          timestamptz,
  ADD COLUMN IF NOT EXISTS purged_by_subject  text,
  ADD COLUMN IF NOT EXISTS purged_by_email    text;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'ma_initiative_single_lifecycle_state_check'
      AND conrelid = 'binocolo.ma_initiative'::regclass
  ) THEN
    ALTER TABLE binocolo.ma_initiative
      ADD CONSTRAINT ma_initiative_single_lifecycle_state_check
      CHECK (deleted_at IS NULL OR archived_at IS NULL);
  END IF;
END;
$$;

ALTER TABLE binocolo.ma_initiative
  DROP CONSTRAINT IF EXISTS ma_initiative_purge_requires_delete_check;

ALTER TABLE binocolo.ma_initiative
  ADD CONSTRAINT ma_initiative_purge_requires_delete_check
  CHECK (purged_at IS NULL OR deleted_at IS NOT NULL);

CREATE INDEX IF NOT EXISTS ma_initiative_active_updated_at_idx
  ON binocolo.ma_initiative (updated_at DESC, created_at DESC)
  WHERE archived_at IS NULL AND deleted_at IS NULL AND purged_at IS NULL;

CREATE INDEX IF NOT EXISTS ma_initiative_archived_updated_at_idx
  ON binocolo.ma_initiative (archived_at DESC, updated_at DESC)
  WHERE archived_at IS NOT NULL AND deleted_at IS NULL AND purged_at IS NULL;

CREATE INDEX IF NOT EXISTS ma_initiative_deleted_updated_at_idx
  ON binocolo.ma_initiative (deleted_at DESC, updated_at DESC)
  WHERE deleted_at IS NOT NULL AND purged_at IS NULL;
