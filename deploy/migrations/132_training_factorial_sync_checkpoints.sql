-- Training: Factorial sync checkpoints for session schedule and enrollment
-- participation status (#143, task 5.1 of #137; parent #141). No application
-- logic here.
--
-- Delivered as a file only. The Agent does NOT apply it and does NOT connect
-- to any database configured in the env files.

BEGIN;

ALTER TABLE training.training_session
  ADD COLUMN IF NOT EXISTS factorial_synced_state jsonb;

COMMENT ON COLUMN training.training_session.factorial_synced_state IS
  'Proiezione canonica {schedule_type, starts_at, ends_at, due_date} dell''ultimo stato sincronizzato con Factorial.';

ALTER TABLE training.enrollment_session
  ADD COLUMN IF NOT EXISTS factorial_synced_status text;

COMMENT ON COLUMN training.enrollment_session.factorial_synced_status IS
  'participation_status (vocabolario locale) dell''ultimo stato sincronizzato con Factorial.';

COMMIT;
