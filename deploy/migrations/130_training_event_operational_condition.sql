-- Training: operational condition of an event (#139, follows #138/129).
-- Ordinary view with one row per training_event and seven independent,
-- cumulable flags. No RDA signals here: the backend integrates them in the
-- RDA task. No automation reads this view to change state.
--
-- Also adds the event-first index on training.enrollment(event_id): the only
-- existing index on enrollment is employee-first (employee_id, event_id).
--
-- Delivered as a file only. The Agent does NOT apply it and does NOT connect
-- to any database configured in the env files.

BEGIN;

CREATE INDEX IF NOT EXISTS idx_enrollment_event
  ON training.enrollment(event_id);

DROP VIEW IF EXISTS training.v_event_operational_condition;

CREATE VIEW training.v_event_operational_condition AS
SELECT
  ev.id AS event_id,

  -- The event has been cancelled by People.
  ev.cancelled_at IS NOT NULL AS cancelled,

  -- The event has no sessions at all.
  NOT EXISTS (
    SELECT 1
    FROM training.training_session s
    WHERE s.event_id = ev.id
  ) AS without_sessions,

  -- At least one non-cancelled enrollment has no relation with any session.
  EXISTS (
    SELECT 1
    FROM training.enrollment en
    WHERE en.event_id = ev.id
      AND en.delivery_status <> 'cancelled'
      AND NOT EXISTS (
        SELECT 1
        FROM training.enrollment_session es
        WHERE es.enrollment_id = en.id
      )
  ) AS unassigned_enrollments,

  -- At least one enrollment is being delivered.
  EXISTS (
    SELECT 1
    FROM training.enrollment en
    WHERE en.event_id = ev.id
      AND en.delivery_status = 'in_progress'
  ) AS in_progress,

  -- The agenda says finished, the register does not: a typed session is past
  -- its end (scheduled: ends_at; self_paced: due_at) and still holds
  -- participations 'assigned'/'in_progress' of non-cancelled enrollments.
  -- Sessions without schedule_type (possible only via import) never trigger.
  EXISTS (
    SELECT 1
    FROM training.training_session s
    JOIN training.enrollment_session es ON es.session_id = s.id
    JOIN training.enrollment en ON en.id = es.enrollment_id
    WHERE s.event_id = ev.id
      AND en.delivery_status <> 'cancelled'
      AND es.participation_status IN ('assigned', 'in_progress')
      AND (
        (s.schedule_type = 'scheduled' AND s.ends_at IS NOT NULL AND s.ends_at < now())
        OR (s.schedule_type = 'self_paced' AND s.due_at IS NOT NULL AND s.due_at < now())
      )
  ) AS needs_reconciliation,

  -- Non-cancelled event with at least one terminal delivery and nothing still
  -- planned or running. An event whose enrollments are all cancelled is not
  -- concluded.
  (
    ev.cancelled_at IS NULL
    AND EXISTS (
      SELECT 1
      FROM training.enrollment en
      WHERE en.event_id = ev.id
        AND en.delivery_status IN ('completed', 'partially_completed', 'not_attended')
    )
    AND NOT EXISTS (
      SELECT 1
      FROM training.enrollment en
      WHERE en.event_id = ev.id
        AND en.delivery_status IN ('planned', 'in_progress')
    )
  ) AS concluded
FROM training.training_event ev;

COMMIT;
