-- 013 Training planning suggestion dismiss table.
-- Persists "skip" decisions per plan + suggestion signature so the queue
-- does not resurrect a dismissed suggestion until its underlying gap evolves.

CREATE TABLE IF NOT EXISTS training.planning_suggestion_dismiss (
  plan_id uuid NOT NULL REFERENCES training.training_plan(id) ON DELETE CASCADE,
  signature text NOT NULL,
  dismissed_at timestamptz NOT NULL DEFAULT now(),
  dismissed_by uuid REFERENCES training.employee(id),
  PRIMARY KEY (plan_id, signature)
);

CREATE INDEX IF NOT EXISTS idx_planning_dismiss_plan
  ON training.planning_suggestion_dismiss(plan_id);
