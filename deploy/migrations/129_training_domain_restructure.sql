-- Training domain restructure: reset POC data and rebuild schema on the
-- consolidated domain contract (#136). Operational plan: #137. Perimeter: #138.
--
-- This migration is DESTRUCTIVE and intentional. It is the explicit exception
-- to the additive rule of docs/DATABASE-MIGRATIONS.md: the POC must NOT remain
-- operational during the refactor, no parallel implementation is maintained,
-- and all POC data is deleted without conversion.
--
-- Before applying: verify that Training jobs are disabled
-- (TRAINING_JOBS_ENABLED=false in backend/internal/platform/config/config.go
-- and backend/.env.example). After applying, the legacy Training runtime is no
-- longer compatible and Training stays unavailable until the backend tasks of
-- the #137 sequence are completed.
--
-- Delivered as a file only. The Agent does NOT apply it and does NOT connect to
-- any database configured in the env files.

BEGIN;

-- ===========================================================================
-- 1. Drop legacy views, trigger, function and plan-unique index first, so they
--    do not block the table changes below.
-- ===========================================================================

DROP VIEW IF EXISTS training.v_plan_budget;
DROP VIEW IF EXISTS training.v_mandatory_compliance_gap;
DROP VIEW IF EXISTS training.v_mandatory_rule_population;

DROP TRIGGER IF EXISTS trg_enrollment_state_guard ON training.enrollment;
DROP FUNCTION IF EXISTS training.validate_enrollment_transition();

DROP INDEX IF EXISTS training.idx_training_one_open_plan;

-- ===========================================================================
-- 2. Reset all POC data. Every table created by migrations 012-017 and 128 is
--    emptied. No data is preserved, transformed or carried over.
--    Safe to CASCADE: every REFERENCES training.* is intra-schema (verified on
--    the repo), so no external table is affected.
-- ===========================================================================

TRUNCATE TABLE
  training.team,
  training.employee,
  training.team_membership,
  training.vendor,
  training.skill_area,
  training.certification,
  training.course,
  training.training_plan,
  training.enrollment,
  training.certification_award,
  training.skill_assessment,
  training.document,
  training.mandatory_assignment_rule,
  training.training_request,
  training.learning_path,
  training.learning_path_step,
  training.employee_learning_path,
  training.audit_log,
  training.planning_suggestion_dismiss,
  training.plan_audit_log,
  training.custom_groups,
  training.custom_group_members,
  training.mandatory_rules,
  training.directory_sync_run
RESTART IDENTITY CASCADE;

-- ===========================================================================
-- 3. Drop legacy tables and the enrollment columns that couple to them.
--    Indices on the dropped columns are removed first (explicit, then the
--    columns), then the tables themselves.
-- ===========================================================================

DROP INDEX IF EXISTS training.idx_enrollment_mandatory_rule;
DROP INDEX IF EXISTS training.idx_enrollment_source_custom_group;

ALTER TABLE training.enrollment
  DROP COLUMN IF EXISTS mandatory_rule_id,
  DROP COLUMN IF EXISTS source_custom_group_id;

DROP TABLE IF EXISTS training.planning_suggestion_dismiss;
DROP TABLE IF EXISTS training.plan_audit_log;
DROP TABLE IF EXISTS training.mandatory_assignment_rule;
DROP TABLE IF EXISTS training.mandatory_rules;

-- ===========================================================================
-- 4. Adjust kept catalogs.
-- ===========================================================================

-- course: recurrence moves to the training rule; add the Factorial training id
-- for the educational sync only (no generic mapping table, no recurrence,
-- dates or actual costs on the course).
ALTER TABLE training.course
  DROP COLUMN IF EXISTS recurrence_interval,
  ADD COLUMN IF NOT EXISTS factorial_training_id text;

CREATE UNIQUE INDEX IF NOT EXISTS idx_course_factorial_training_id
  ON training.course(factorial_training_id)
  WHERE factorial_training_id IS NOT NULL;

COMMENT ON COLUMN training.course.is_compliance_course IS
  'Se TRUE, il corso e collegato a un framework compliance. L''obbligatorieta per persona deriva da training.training_rule.';

-- employee: the TL derives from team leads, not from a parallel manager
-- hierarchy.
ALTER TABLE training.employee
  DROP COLUMN IF EXISTS manager_id;

-- ===========================================================================
-- 5. Restructure enrollment as a person<->event relation.
--    The course always comes from the event; no enrollment without event and
--    no legacy fields for the old backend are kept.
-- ===========================================================================

-- Drop legacy columns and the indices that reference them.
DROP INDEX IF EXISTS training.idx_enrollment_plan;
DROP INDEX IF EXISTS training.idx_enrollment_course;
DROP INDEX IF EXISTS training.idx_enrollment_active;

ALTER TABLE training.enrollment
  DROP COLUMN IF EXISTS course_id,
  DROP COLUMN IF EXISTS training_plan_id,
  DROP COLUMN IF EXISTS status,
  DROP COLUMN IF EXISTS priority,
  DROP COLUMN IF EXISTS level_as_is,
  DROP COLUMN IF EXISTS level_to_be,
  DROP COLUMN IF EXISTS planned_start,
  DROP COLUMN IF EXISTS planned_end,
  DROP COLUMN IF EXISTS hours_planned,
  DROP COLUMN IF EXISTS cost_planned,
  DROP COLUMN IF EXISTS cost_actual,
  DROP COLUMN IF EXISTS course_title_snapshot,
  DROP COLUMN IF EXISTS vendor_name_snapshot,
  DROP COLUMN IF EXISTS motivation;

-- Add the new domain columns. Foreign keys that target tables not yet created
-- (event, training_request) are added later, once those tables exist.
ALTER TABLE training.enrollment
  ADD COLUMN IF NOT EXISTS event_id uuid NOT NULL,
  ADD COLUMN IF NOT EXISTS delivery_status text NOT NULL DEFAULT 'planned',
  ADD COLUMN IF NOT EXISTS learning_outcome text,
  ADD COLUMN IF NOT EXISTS origin text NOT NULL,
  ADD COLUMN IF NOT EXISTS source_rule_id uuid,
  ADD COLUMN IF NOT EXISTS source_request_id uuid,
  ADD COLUMN IF NOT EXISTS cancellation_reason text,
  ADD COLUMN IF NOT EXISTS factorial_training_membership_id text;

ALTER TABLE training.enrollment
  ADD CONSTRAINT chk_enrollment_delivery_status
    CHECK (delivery_status IN (
      'planned', 'in_progress', 'completed',
      'partially_completed', 'not_attended', 'cancelled')),
  ADD CONSTRAINT chk_enrollment_learning_outcome
    CHECK (learning_outcome IS NULL OR learning_outcome IN (
      'passed', 'failed', 'not_taken', 'not_required')),
  ADD CONSTRAINT chk_enrollment_origin
    CHECK (origin IN ('rule', 'request', 'direct', 'factorial_import'));

-- Origin/source consistency and cancellation workflow are enforced by the
-- backend, not duplicated as cross-column database constraints.

CREATE UNIQUE INDEX IF NOT EXISTS idx_enrollment_employee_event
  ON training.enrollment(employee_id, event_id);

-- ===========================================================================
-- 6. Restructure training_request: original request data stays separate from
--    the data accepted by People.
-- ===========================================================================

DROP INDEX IF EXISTS training.idx_training_request_open;

ALTER TABLE training.training_request
  DROP COLUMN IF EXISTS desired_year,
  DROP COLUMN IF EXISTS status,
  DROP COLUMN IF EXISTS converted_to_enrollment_id,
  DROP COLUMN IF EXISTS review_notes,
  DROP COLUMN IF EXISTS reviewed_by,
  DROP COLUMN IF EXISTS reviewed_at;

ALTER TABLE training.training_request
  ADD COLUMN IF NOT EXISTS selected_team_id uuid NOT NULL REFERENCES training.team(id),
  ADD COLUMN IF NOT EXISTS desired_start date,
  ADD COLUMN IF NOT EXISTS desired_end date,
  ADD COLUMN IF NOT EXISTS tl_opinion text,
  ADD COLUMN IF NOT EXISTS tl_opinion_by uuid REFERENCES training.employee(id),
  ADD COLUMN IF NOT EXISTS tl_opinion_at timestamptz,
  ADD COLUMN IF NOT EXISTS tl_opinion_reason text,
  ADD COLUMN IF NOT EXISTS people_decision text,
  ADD COLUMN IF NOT EXISTS people_decision_by uuid REFERENCES training.employee(id),
  ADD COLUMN IF NOT EXISTS people_decision_at timestamptz,
  ADD COLUMN IF NOT EXISTS people_decision_reason text,
  ADD COLUMN IF NOT EXISTS outcome text,
  ADD COLUMN IF NOT EXISTS closed_at timestamptz,
  ADD COLUMN IF NOT EXISTS accepted_course_id uuid REFERENCES training.course(id),
  ADD COLUMN IF NOT EXISTS accepted_event_id uuid,
  ADD COLUMN IF NOT EXISTS accepted_vendor_id uuid REFERENCES training.vendor(id),
  ADD COLUMN IF NOT EXISTS accepted_period_start date,
  ADD COLUMN IF NOT EXISTS accepted_period_end date,
  ADD COLUMN IF NOT EXISTS accepted_notes text,
  ADD COLUMN IF NOT EXISTS resulting_enrollment_id uuid;

ALTER TABLE training.training_request
  ADD CONSTRAINT chk_request_tl_opinion_values
    CHECK (tl_opinion IS NULL OR tl_opinion IN ('favorable', 'unfavorable')),
  ADD CONSTRAINT chk_request_people_decision_values
    CHECK (people_decision IS NULL OR people_decision IN ('accepted', 'rejected')),
  ADD CONSTRAINT chk_request_outcome_values
    CHECK (outcome IS NULL OR outcome IN ('accepted', 'rejected', 'withdrawn')),
  ADD CONSTRAINT chk_request_desired_period
    CHECK (
      desired_start IS NULL OR desired_end IS NULL OR desired_end >= desired_start
    ),
  ADD CONSTRAINT chk_request_accepted_period
    CHECK (
      accepted_period_start IS NULL
      OR accepted_period_end IS NULL
      OR accepted_period_end >= accepted_period_start
    );

-- Opinion/decision completeness and request lifecycle transitions are
-- enforced by the backend, not duplicated as cross-column database constraints.

CREATE INDEX IF NOT EXISTS idx_training_request_open
  ON training.training_request(created_at DESC)
  WHERE outcome IS NULL;

-- ===========================================================================
-- 7. Training rule: a plannable need referring to a course, with a population
--    or a number of positions, optional mandatoriness, deadline and optional
--    recurrence.
-- ===========================================================================

CREATE TABLE IF NOT EXISTS training.training_rule (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  course_id uuid NOT NULL REFERENCES training.course(id),
  population_target jsonb,
  seat_count integer,
  is_mandatory boolean NOT NULL DEFAULT false,
  deadline date NOT NULL,
  recurrence_months integer,
  recurrence_anchor text,
  is_active boolean NOT NULL DEFAULT true,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT chk_rule_population_xor_seat
    CHECK (
      (population_target IS NOT NULL AND seat_count IS NULL)
      OR (population_target IS NULL AND seat_count IS NOT NULL)
    ),
  CONSTRAINT chk_rule_seat_count
    CHECK (seat_count IS NULL OR seat_count > 0),
  CONSTRAINT chk_rule_recurrence_pair
    CHECK (
      (recurrence_months IS NULL AND recurrence_anchor IS NULL)
      OR (recurrence_months IS NOT NULL AND recurrence_anchor IS NOT NULL)
    ),
  CONSTRAINT chk_rule_recurrence_months
    CHECK (recurrence_months IS NULL OR recurrence_months > 0),
  CONSTRAINT chk_rule_recurrence_anchor
    CHECK (
      recurrence_anchor IS NULL OR recurrence_anchor IN ('calendar', 'completion')
    ),
  CONSTRAINT chk_rule_population_kind
    CHECK (
      population_target IS NULL
      OR (
        jsonb_typeof(population_target) = 'object'
        AND population_target->>'kind' IS NOT NULL
        AND population_target ? 'kind'
        AND population_target->>'kind' IN ('all', 'team', 'skill_area', 'custom_group', 'people')
      )
    ),
  CONSTRAINT chk_rule_population_id
    CHECK (
      population_target IS NULL
      OR (
        (population_target->>'kind' IN ('all', 'people')
         AND NOT (population_target ? 'id'))
        OR (
          population_target->>'kind' IN ('team', 'skill_area', 'custom_group')
          AND COALESCE(population_target->>'id', '') ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
        )
      )
    )
);

CREATE INDEX IF NOT EXISTS idx_training_rule_course
  ON training.training_rule(course_id);

CREATE INDEX IF NOT EXISTS idx_training_rule_active_deadline
  ON training.training_rule(deadline)
  WHERE is_active;

CREATE TABLE IF NOT EXISTS training.training_rule_person (
  rule_id uuid NOT NULL REFERENCES training.training_rule(id) ON DELETE CASCADE,
  employee_id uuid NOT NULL REFERENCES training.employee(id) ON DELETE CASCADE,
  PRIMARY KEY (rule_id, employee_id)
);

CREATE INDEX IF NOT EXISTS idx_training_rule_person_employee
  ON training.training_rule_person(employee_id);

-- ===========================================================================
-- 8. Wire enrollment.source_rule_id to the now-existing training_rule.
-- ===========================================================================

ALTER TABLE training.enrollment
  DROP CONSTRAINT IF EXISTS fk_enrollment_source_rule,
  ADD CONSTRAINT fk_enrollment_source_rule
    FOREIGN KEY (source_rule_id) REFERENCES training.training_rule(id);

-- ===========================================================================
-- 9. Training event: the concrete organisational/economic initiative for a
--    course. source_request_id FK is added later (circular: request -> event).
-- ===========================================================================

CREATE TABLE IF NOT EXISTS training.training_event (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  course_id uuid NOT NULL REFERENCES training.course(id),
  vendor_id uuid REFERENCES training.vendor(id),
  agreed_price numeric(12,2),
  agreed_conditions text,
  origin text NOT NULL,
  source_rule_id uuid REFERENCES training.training_rule(id),
  source_request_id uuid,
  rule_deadline date,
  factorial_class_id text,
  cancelled_at timestamptz,
  cancelled_by uuid REFERENCES training.employee(id),
  cancellation_reason text,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT chk_event_agreed_price
    CHECK (agreed_price IS NULL OR agreed_price >= 0),
  CONSTRAINT chk_event_origin
    CHECK (origin IN ('rule', 'request', 'direct', 'factorial_import'))
);

-- Origin/source consistency and cancellation workflow are enforced by the
-- backend, not duplicated as cross-column database constraints.

CREATE INDEX IF NOT EXISTS idx_training_event_course
  ON training.training_event(course_id);

CREATE INDEX IF NOT EXISTS idx_training_event_source_rule
  ON training.training_event(source_rule_id)
  WHERE source_rule_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_training_event_source_request
  ON training.training_event(source_request_id)
  WHERE source_request_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_training_event_factorial_class_id
  ON training.training_event(factorial_class_id)
  WHERE factorial_class_id IS NOT NULL;

-- ===========================================================================
-- 10. Training session: an appointment or fruition window of an event.
-- ===========================================================================

CREATE TABLE IF NOT EXISTS training.training_session (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id uuid NOT NULL REFERENCES training.training_event(id) ON DELETE CASCADE,
  schedule_type text,
  starts_at timestamptz,
  ends_at timestamptz,
  due_at timestamptz,
  max_capacity integer,
  factorial_session_id text,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT chk_session_schedule_type
    CHECK (schedule_type IS NULL OR schedule_type IN ('scheduled', 'self_paced')),
  CONSTRAINT chk_session_max_capacity
    CHECK (max_capacity IS NULL OR max_capacity > 0),
  CONSTRAINT chk_session_ends_after_starts
    CHECK (
      starts_at IS NULL OR ends_at IS NULL OR ends_at >= starts_at
    )
);

CREATE INDEX IF NOT EXISTS idx_training_session_event
  ON training.training_session(event_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_training_session_factorial_session_id
  ON training.training_session(factorial_session_id)
  WHERE factorial_session_id IS NOT NULL;

-- ===========================================================================
-- 11. Wire enrollment.event_id to the now-existing training_event.
-- ===========================================================================

ALTER TABLE training.enrollment
  DROP CONSTRAINT IF EXISTS fk_enrollment_event,
  ADD CONSTRAINT fk_enrollment_event
    FOREIGN KEY (event_id) REFERENCES training.training_event(id);

-- ===========================================================================
-- 12. enrollment_session: assignment and attendance of an enrollment to a
--     session. The absence of a row means "not assigned", not "absent".
--     The same-event constraint is enforced by the backend in the next task;
--     no DB trigger and no duplicated event_id here.
-- ===========================================================================

CREATE TABLE IF NOT EXISTS training.enrollment_session (
  enrollment_id uuid NOT NULL REFERENCES training.enrollment(id) ON DELETE CASCADE,
  session_id uuid NOT NULL REFERENCES training.training_session(id) ON DELETE CASCADE,
  participation_status text NOT NULL DEFAULT 'assigned',
  assigned_at timestamptz NOT NULL DEFAULT now(),
  assigned_by uuid REFERENCES training.employee(id),
  factorial_access_membership_id text,
  factorial_attendance_id text,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (enrollment_id, session_id),
  CONSTRAINT chk_enrollment_session_participation
    CHECK (participation_status IN ('assigned', 'in_progress', 'completed', 'not_attended'))
);

CREATE INDEX IF NOT EXISTS idx_enrollment_session_session
  ON training.enrollment_session(session_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_enrollment_session_factorial_access
  ON training.enrollment_session(factorial_access_membership_id)
  WHERE factorial_access_membership_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_enrollment_session_factorial_attendance
  ON training.enrollment_session(factorial_attendance_id)
  WHERE factorial_attendance_id IS NOT NULL;

-- ===========================================================================
-- 13. Event expenses and RDA link. rda_id has no SQL foreign key: RDA lives on
--     a different database. The RDA task backend verifies the RDA exists and
--     that expense and enrollments belong to the same event.
-- ===========================================================================

CREATE TABLE IF NOT EXISTS training.event_expense (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id uuid NOT NULL REFERENCES training.training_event(id) ON DELETE CASCADE,
  rda_id bigint NOT NULL,
  created_by uuid REFERENCES training.employee(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT chk_event_expense_rda_id CHECK (rda_id > 0),
  CONSTRAINT uq_event_expense_event_rda UNIQUE (event_id, rda_id)
);

CREATE INDEX IF NOT EXISTS idx_event_expense_event
  ON training.event_expense(event_id);

CREATE INDEX IF NOT EXISTS idx_event_expense_rda
  ON training.event_expense(rda_id);

CREATE TABLE IF NOT EXISTS training.event_expense_enrollment (
  expense_id uuid NOT NULL REFERENCES training.event_expense(id) ON DELETE CASCADE,
  enrollment_id uuid NOT NULL REFERENCES training.enrollment(id) ON DELETE CASCADE,
  PRIMARY KEY (expense_id, enrollment_id)
);

CREATE INDEX IF NOT EXISTS idx_event_expense_enrollment_enrollment
  ON training.event_expense_enrollment(enrollment_id);

-- ===========================================================================
-- 14. Wire the remaining circular foreign keys now that all tables exist.
-- ===========================================================================

ALTER TABLE training.training_event
  DROP CONSTRAINT IF EXISTS fk_training_event_source_request,
  ADD CONSTRAINT fk_training_event_source_request
    FOREIGN KEY (source_request_id) REFERENCES training.training_request(id);

ALTER TABLE training.training_request
  DROP CONSTRAINT IF EXISTS fk_request_accepted_event,
  ADD CONSTRAINT fk_request_accepted_event
    FOREIGN KEY (accepted_event_id) REFERENCES training.training_event(id);

ALTER TABLE training.training_request
  DROP CONSTRAINT IF EXISTS fk_request_resulting_enrollment,
  ADD CONSTRAINT fk_request_resulting_enrollment
    FOREIGN KEY (resulting_enrollment_id) REFERENCES training.enrollment(id);

ALTER TABLE training.enrollment
  DROP CONSTRAINT IF EXISTS fk_enrollment_source_request,
  ADD CONSTRAINT fk_enrollment_source_request
    FOREIGN KEY (source_request_id) REFERENCES training.training_request(id);

-- ===========================================================================
-- 15. Drop the annual plan table and the legacy enums. No remaining
--     references exist at this point.
-- ===========================================================================

DROP TABLE IF EXISTS training.training_plan;

DROP TYPE IF EXISTS training.enrollment_status;
DROP TYPE IF EXISTS training.plan_status;

-- ===========================================================================
-- 16. updated_at triggers on the new tables, reusing the existing function.
-- ===========================================================================

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'training_rule', 'training_event', 'training_session',
    'enrollment_session', 'event_expense'
  ]
  LOOP
    EXECUTE format('DROP TRIGGER IF EXISTS trg_%I_updated_at ON training.%I', t, t);
    EXECUTE format(
      'CREATE TRIGGER trg_%I_updated_at BEFORE UPDATE ON training.%I
       FOR EACH ROW EXECUTE FUNCTION training.set_updated_at()', t, t);
  END LOOP;
END $$;

-- ===========================================================================
-- 17. The certification views depend only on kept tables and enums and remain
--     valid; no action needed. v_employee_certifications and
--     v_expiring_certifications are unchanged.
-- ===========================================================================

COMMIT;
