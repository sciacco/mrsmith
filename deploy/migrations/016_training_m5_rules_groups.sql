-- Training M5 rule populations and custom groups.
-- Keeps mandatory populations owned by the Training domain.

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS training.custom_groups (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  description text,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS training.custom_group_members (
  group_id uuid NOT NULL REFERENCES training.custom_groups(id) ON DELETE CASCADE,
  employee_id uuid NOT NULL REFERENCES training.employee(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (group_id, employee_id)
);

CREATE INDEX IF NOT EXISTS idx_custom_group_members_employee
  ON training.custom_group_members(employee_id);

CREATE TABLE IF NOT EXISTS training.mandatory_rules (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  course_id uuid NOT NULL REFERENCES training.course(id),
  population_target jsonb NOT NULL DEFAULT '{"kind":"all"}'::jsonb,
  is_active boolean NOT NULL DEFAULT true,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT mandatory_rules_population_kind_chk CHECK (
    population_target ? 'kind'
    AND population_target->>'kind' IN ('all', 'team', 'skill_area', 'custom_group')
  ),
  CONSTRAINT mandatory_rules_population_id_chk CHECK (
    (
      population_target->>'kind' = 'all'
      AND NOT (population_target ? 'id')
    )
    OR (
      population_target->>'kind' IN ('team', 'skill_area', 'custom_group')
      AND COALESCE(population_target->>'id', '') ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
    )
  )
);

CREATE INDEX IF NOT EXISTS idx_mandatory_rules_course
  ON training.mandatory_rules(course_id);

CREATE INDEX IF NOT EXISTS idx_mandatory_rules_population_target
  ON training.mandatory_rules USING gin(population_target);

CREATE INDEX IF NOT EXISTS idx_mandatory_rules_active
  ON training.mandatory_rules(is_active)
  WHERE is_active;

ALTER TABLE training.enrollment
  ADD COLUMN IF NOT EXISTS mandatory_rule_id uuid REFERENCES training.mandatory_rules(id),
  ADD COLUMN IF NOT EXISTS source_custom_group_id uuid REFERENCES training.custom_groups(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_enrollment_mandatory_rule
  ON training.enrollment(mandatory_rule_id)
  WHERE mandatory_rule_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_enrollment_source_custom_group
  ON training.enrollment(source_custom_group_id)
  WHERE source_custom_group_id IS NOT NULL;

INSERT INTO training.mandatory_rules (
  id,
  name,
  course_id,
  population_target,
  is_active,
  notes,
  created_at,
  updated_at
)
SELECT
  old.id,
  CASE
    WHEN old.team_id IS NOT NULL THEN course.title || ' - ' || team.name
    ELSE course.title || ' - Tutti'
  END,
  old.course_id,
  CASE
    WHEN old.team_id IS NOT NULL THEN jsonb_build_object('kind', 'team', 'id', old.team_id::text)
    ELSE '{"kind":"all"}'::jsonb
  END,
  CASE WHEN NULLIF(BTRIM(old.role_filter), '') IS NULL THEN old.is_active ELSE false END,
  NULLIF(
    CONCAT_WS(
      E'\n',
      NULLIF(old.notes, ''),
      CASE
        WHEN NULLIF(BTRIM(old.role_filter), '') IS NOT NULL
          THEN 'Disattivata in migrazione: filtro ruolo legacy non supportato nelle popolazioni M5. Rivedere prima di riattivare.'
        ELSE NULL
      END
    ),
    ''
  ),
  old.created_at,
  old.updated_at
FROM training.mandatory_assignment_rule old
JOIN training.course course ON course.id = old.course_id
LEFT JOIN training.team team ON team.id = old.team_id
ON CONFLICT (id) DO NOTHING;

CREATE OR REPLACE VIEW training.v_mandatory_rule_population AS
SELECT DISTINCT
  rule.id AS rule_id,
  employee.id AS employee_id
FROM training.mandatory_rules rule
JOIN training.employee employee ON employee.status = 'active'
WHERE rule.is_active
  AND (
    rule.population_target->>'kind' = 'all'
    OR (
      rule.population_target->>'kind' = 'team'
      AND EXISTS (
        SELECT 1
        FROM training.team_membership tm
        WHERE tm.employee_id = employee.id
          AND tm.team_id = (rule.population_target->>'id')::uuid
          AND tm.start_date <= now()
          AND (tm.end_date IS NULL OR tm.end_date >= now())
      )
    )
    OR (
      rule.population_target->>'kind' = 'skill_area'
      AND (
        EXISTS (
          SELECT 1
          FROM training.skill_assessment assessment
          WHERE assessment.employee_id = employee.id
            AND assessment.skill_area_id = (rule.population_target->>'id')::uuid
        )
        OR EXISTS (
          SELECT 1
          FROM training.enrollment enrollment
          JOIN training.course course ON course.id = enrollment.course_id
          WHERE enrollment.employee_id = employee.id
            AND course.skill_area_id = (rule.population_target->>'id')::uuid
        )
        OR EXISTS (
          SELECT 1
          FROM training.certification_award award
          JOIN training.certification certification ON certification.id = award.certification_id
          WHERE award.employee_id = employee.id
            AND certification.skill_area_id = (rule.population_target->>'id')::uuid
        )
      )
    )
    OR (
      rule.population_target->>'kind' = 'custom_group'
      AND EXISTS (
        SELECT 1
        FROM training.custom_group_members member
        WHERE member.group_id = (rule.population_target->>'id')::uuid
          AND member.employee_id = employee.id
      )
    )
  );

DROP VIEW IF EXISTS training.v_mandatory_compliance_gap;

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

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['custom_groups','mandatory_rules']
  LOOP
    EXECUTE format('DROP TRIGGER IF EXISTS trg_%I_updated_at ON training.%I', t, t);
    EXECUTE format(
      'CREATE TRIGGER trg_%I_updated_at BEFORE UPDATE ON training.%I
       FOR EACH ROW EXECUTE FUNCTION training.set_updated_at()', t, t);
  END LOOP;
END $$;

COMMIT;
