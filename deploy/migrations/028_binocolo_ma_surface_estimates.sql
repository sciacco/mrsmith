-- Binocolo M&A surface dry-run metadata.
-- Target database: Anisetta PostgreSQL.

BEGIN;

ALTER TABLE binocolo.ma_dry_run_estimate
  ADD COLUMN IF NOT EXISTS surface_status text NOT NULL DEFAULT 'exact',
  ADD COLUMN IF NOT EXISTS execution_limit integer NOT NULL DEFAULT 100,
  ADD COLUMN IF NOT EXISTS probe_count integer NOT NULL DEFAULT 1;

UPDATE binocolo.ma_dry_run_estimate
SET execution_limit = COALESCE(NULLIF((params ->> 'limit')::integer, 0), execution_limit)
WHERE params ? 'limit'
  AND (params ->> 'limit') ~ '^[0-9]+$';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'ma_dry_run_estimate_surface_status_check'
      AND conrelid = 'binocolo.ma_dry_run_estimate'::regclass
  ) THEN
    ALTER TABLE binocolo.ma_dry_run_estimate
      ADD CONSTRAINT ma_dry_run_estimate_surface_status_check
      CHECK (surface_status IN ('exact', 'too_broad'));
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'ma_dry_run_estimate_execution_limit_check'
      AND conrelid = 'binocolo.ma_dry_run_estimate'::regclass
  ) THEN
    ALTER TABLE binocolo.ma_dry_run_estimate
      ADD CONSTRAINT ma_dry_run_estimate_execution_limit_check
      CHECK (execution_limit BETWEEN 1 AND 1000);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'ma_dry_run_estimate_probe_count_check'
      AND conrelid = 'binocolo.ma_dry_run_estimate'::regclass
  ) THEN
    ALTER TABLE binocolo.ma_dry_run_estimate
      ADD CONSTRAINT ma_dry_run_estimate_probe_count_check
      CHECK (probe_count BETWEEN 1 AND 2);
  END IF;
END $$;

COMMIT;
