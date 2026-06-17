-- Binocolo M&A dry-run probe count metadata.
-- Target database: Anisetta PostgreSQL. Apply after 043.
-- probe_count is diagnostic/audit metadata and may legitimately exceed the old
-- per-surface bound after expanded estimates are aggregated.

BEGIN;

ALTER TABLE IF EXISTS binocolo.ma_dry_run_estimate
  DROP CONSTRAINT IF EXISTS ma_dry_run_estimate_probe_count_check;

COMMIT;
