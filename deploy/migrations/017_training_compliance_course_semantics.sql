-- Split course compliance metadata from per-person mandatory rule coverage.

BEGIN;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'training'
      AND table_name = 'course'
      AND column_name = 'is_mandatory'
  ) AND NOT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'training'
      AND table_name = 'course'
      AND column_name = 'is_compliance_course'
  ) THEN
    ALTER TABLE training.course RENAME COLUMN is_mandatory TO is_compliance_course;
  END IF;
END $$;

COMMENT ON COLUMN training.course.is_compliance_course IS
  'Se TRUE, il corso e collegato a un framework compliance. L''obbligatorieta per persona deriva da training.mandatory_rules.';

CREATE OR REPLACE VIEW training.v_mandatory_compliance_gap AS
WITH required AS (
  SELECT
    population.rule_id,
    employee.id AS employee_id,
    employee.last_name,
    employee.first_name,
    course.id AS course_id,
    course.title AS course_title,
    course.leads_to_cert_id,
    course.compliance_framework,
    course.recurrence_interval
  FROM training.v_mandatory_rule_population population
  JOIN training.mandatory_rules rule ON rule.id = population.rule_id
  JOIN training.course course ON course.id = rule.course_id AND course.is_active
  JOIN training.employee employee ON employee.id = population.employee_id
),
coverage AS (
  SELECT
    req.*,
    (
      SELECT MAX(award.awarded_on)
      FROM training.certification_award award
      WHERE req.leads_to_cert_id IS NOT NULL
        AND award.employee_id = req.employee_id
        AND award.certification_id = req.leads_to_cert_id
        AND award.outcome = 'passed_exam'
        AND (award.expires_on IS NULL OR award.expires_on > CURRENT_DATE)
        AND (req.recurrence_interval IS NULL OR award.awarded_on + req.recurrence_interval > CURRENT_DATE)
    ) AS last_valid_awarded_on,
    (
      SELECT MAX(COALESCE(enrollment.actual_end, enrollment.planned_end))
      FROM training.enrollment enrollment
      WHERE req.leads_to_cert_id IS NULL
        AND enrollment.employee_id = req.employee_id
        AND enrollment.course_id = req.course_id
        AND enrollment.status = 'completed'
        AND COALESCE(enrollment.actual_end, enrollment.planned_end) IS NOT NULL
        AND (
          req.recurrence_interval IS NULL
          OR COALESCE(enrollment.actual_end, enrollment.planned_end) + req.recurrence_interval > CURRENT_DATE
        )
    ) AS last_valid_completed_on
  FROM required req
)
SELECT
  coverage.rule_id,
  coverage.employee_id,
  coverage.last_name,
  coverage.first_name,
  coverage.course_id,
  coverage.course_title,
  coverage.compliance_framework,
  coverage.last_valid_awarded_on,
  coverage.last_valid_completed_on,
  CASE
    WHEN coverage.leads_to_cert_id IS NOT NULL AND coverage.last_valid_awarded_on IS NOT NULL THEN 'compliant'
    WHEN coverage.leads_to_cert_id IS NULL AND coverage.last_valid_completed_on IS NOT NULL THEN 'compliant'
    ELSE 'missing_or_expired'
  END AS compliance_status
FROM coverage;

COMMIT;
