-- 014 Training business-facing plan history.
-- Retains plan_deleted events after a draft plan is removed.

CREATE TABLE IF NOT EXISTS training.plan_audit_log (
  id bigserial PRIMARY KEY,
  plan_id uuid NOT NULL,
  actor_id text NOT NULL,
  event_type text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_training_plan_audit_log_plan_created
  ON training.plan_audit_log(plan_id, created_at DESC);
