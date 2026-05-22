-- 015 Enforce at most one open training plan.

CREATE UNIQUE INDEX IF NOT EXISTS idx_training_one_open_plan
  ON training.training_plan(status)
  WHERE status = 'open';
