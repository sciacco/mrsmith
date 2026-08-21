-- Training: anagrafica sincronizzata da directory esterna (Factorial).
-- Additiva: il POC resta operativo con questa migrazione applicata.

ALTER TABLE training.team ADD COLUMN IF NOT EXISTS external_id text;

CREATE UNIQUE INDEX IF NOT EXISTS idx_team_external_id
  ON training.team(external_id)
  WHERE external_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS training.directory_sync_run (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  started_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  status text NOT NULL CHECK (status IN ('running', 'ok', 'failed')),
  dry_run boolean NOT NULL,
  actor text NOT NULL,
  stats jsonb,
  error text
);

CREATE INDEX IF NOT EXISTS idx_directory_sync_run_started_at
  ON training.directory_sync_run(started_at DESC);
