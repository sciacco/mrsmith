-- Training mini-app domain schema on Anisetta.
-- Apply after the shared mrsmith notification migrations.

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE SCHEMA IF NOT EXISTS training;
SET search_path TO training, public;

DO $$ BEGIN
  CREATE TYPE training.employee_status AS ENUM ('active', 'on_leave', 'terminated');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE training.hr_source AS ENUM ('factorial', 'manual', 'successor');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE training.course_delivery_mode AS ENUM ('classroom', 'online_live', 'online_self', 'on_the_job', 'mixed');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE training.course_provider_kind AS ENUM ('internal', 'external');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE training.enrollment_status AS ENUM ('proposed', 'approved', 'in_progress', 'completed', 'failed', 'cancelled', 'expired');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE training.award_outcome AS ENUM ('passed_exam', 'attendance_only', 'failed_exam', 'in_progress');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE training.validation_source AS ENUM ('document_verified', 'declared_survey', 'declared_verbal', 'declared_cv', 'imported_legacy');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE training.plan_status AS ENUM ('draft', 'open', 'frozen', 'closed');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS training.team (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code text NOT NULL UNIQUE,
  name text NOT NULL,
  description text,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS training.employee (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  external_id text NOT NULL,
  external_source training.hr_source NOT NULL DEFAULT 'factorial',
  first_name text NOT NULL,
  last_name text NOT NULL,
  email citext NOT NULL,
  hire_date date,
  termination_date date,
  status training.employee_status NOT NULL DEFAULT 'active',
  manager_id uuid REFERENCES training.employee(id) ON DELETE SET NULL,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (external_source, external_id),
  UNIQUE (email)
);

CREATE TABLE IF NOT EXISTS training.team_membership (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id uuid NOT NULL REFERENCES training.employee(id) ON DELETE CASCADE,
  team_id uuid NOT NULL REFERENCES training.team(id),
  role text,
  start_date timestamptz NOT NULL DEFAULT now(),
  end_date timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_team_membership_active_uniq
  ON training.team_membership (employee_id, team_id)
  WHERE (end_date IS NULL);

CREATE TABLE IF NOT EXISTS training.vendor (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  name_normalized citext NOT NULL,
  website text,
  notes text,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (name_normalized)
);

CREATE TABLE IF NOT EXISTS training.skill_area (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code text NOT NULL UNIQUE,
  name text NOT NULL,
  parent_id uuid REFERENCES training.skill_area(id) ON DELETE SET NULL,
  description text,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS training.certification (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code text NOT NULL UNIQUE,
  name text NOT NULL,
  issuer_vendor_id uuid REFERENCES training.vendor(id),
  skill_area_id uuid REFERENCES training.skill_area(id),
  typical_validity interval,
  description text,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS training.course (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  title text NOT NULL,
  vendor_id uuid REFERENCES training.vendor(id),
  skill_area_id uuid REFERENCES training.skill_area(id),
  leads_to_cert_id uuid REFERENCES training.certification(id),
  delivery_mode training.course_delivery_mode NOT NULL DEFAULT 'mixed',
  provider_kind training.course_provider_kind NOT NULL DEFAULT 'external',
  default_hours integer CHECK (default_hours IS NULL OR default_hours > 0),
  default_cost numeric(10,2) CHECK (default_cost IS NULL OR default_cost >= 0),
  course_url text,
  description text,
  is_mandatory boolean NOT NULL DEFAULT false,
  recurrence_interval interval,
  compliance_framework text,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS training.training_plan (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  year integer NOT NULL UNIQUE CHECK (year BETWEEN 2020 AND 2100),
  status training.plan_status NOT NULL DEFAULT 'draft',
  budget_total numeric(12,2) CHECK (budget_total IS NULL OR budget_total >= 0),
  opened_at timestamptz,
  frozen_at timestamptz,
  closed_at timestamptz,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS training.enrollment (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id uuid NOT NULL REFERENCES training.employee(id),
  course_id uuid NOT NULL REFERENCES training.course(id),
  training_plan_id uuid NOT NULL REFERENCES training.training_plan(id),
  status training.enrollment_status NOT NULL DEFAULT 'proposed',
  priority smallint CHECK (priority IS NULL OR priority BETWEEN 1 AND 5),
  level_as_is smallint CHECK (level_as_is IS NULL OR level_as_is BETWEEN 0 AND 5),
  level_to_be smallint CHECK (level_to_be IS NULL OR level_to_be BETWEEN 0 AND 5),
  planned_start date,
  planned_end date,
  actual_start date,
  actual_end date,
  hours_planned integer CHECK (hours_planned IS NULL OR hours_planned > 0),
  hours_actual integer CHECK (hours_actual IS NULL OR hours_actual >= 0),
  cost_planned numeric(10,2) CHECK (cost_planned IS NULL OR cost_planned >= 0),
  cost_actual numeric(10,2) CHECK (cost_actual IS NULL OR cost_actual >= 0),
  course_title_snapshot text,
  vendor_name_snapshot text,
  motivation text,
  objective text,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (planned_end IS NULL OR planned_start IS NULL OR planned_end >= planned_start),
  CHECK (actual_end IS NULL OR actual_start IS NULL OR actual_end >= actual_start),
  CHECK (level_to_be IS NULL OR level_as_is IS NULL OR level_to_be >= level_as_is)
);

CREATE INDEX IF NOT EXISTS idx_enrollment_employee ON training.enrollment(employee_id);
CREATE INDEX IF NOT EXISTS idx_enrollment_plan ON training.enrollment(training_plan_id);
CREATE INDEX IF NOT EXISTS idx_enrollment_course ON training.enrollment(course_id);
CREATE INDEX IF NOT EXISTS idx_enrollment_active ON training.enrollment(employee_id)
  WHERE status IN ('proposed', 'approved', 'in_progress');

CREATE TABLE IF NOT EXISTS training.certification_award (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id uuid NOT NULL REFERENCES training.employee(id),
  certification_id uuid NOT NULL REFERENCES training.certification(id),
  enrollment_id uuid REFERENCES training.enrollment(id),
  outcome training.award_outcome NOT NULL,
  awarded_on date NOT NULL,
  expires_on date,
  validity daterange GENERATED ALWAYS AS (daterange(awarded_on, expires_on, '[)')) STORED,
  validation_source training.validation_source NOT NULL DEFAULT 'document_verified',
  external_credential_id text,
  external_credential_url text,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (expires_on IS NULL OR expires_on > awarded_on)
);

CREATE INDEX IF NOT EXISTS idx_cert_award_employee ON training.certification_award(employee_id);
CREATE INDEX IF NOT EXISTS idx_cert_award_certification ON training.certification_award(certification_id);
CREATE INDEX IF NOT EXISTS idx_cert_award_validity ON training.certification_award USING gist (validity);
CREATE INDEX IF NOT EXISTS idx_cert_award_passed_by_employee
  ON training.certification_award(employee_id, certification_id, expires_on)
  WHERE outcome = 'passed_exam';

CREATE TABLE IF NOT EXISTS training.skill_assessment (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id uuid NOT NULL REFERENCES training.employee(id),
  skill_area_id uuid NOT NULL REFERENCES training.skill_area(id),
  level smallint NOT NULL CHECK (level BETWEEN 0 AND 5),
  assessed_on date NOT NULL DEFAULT CURRENT_DATE,
  source training.validation_source NOT NULL DEFAULT 'declared_survey',
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (employee_id, skill_area_id, assessed_on)
);
CREATE INDEX IF NOT EXISTS idx_assessment_employee_area
  ON training.skill_assessment(employee_id, skill_area_id, assessed_on DESC);

CREATE TABLE IF NOT EXISTS training.document (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  enrollment_id uuid REFERENCES training.enrollment(id) ON DELETE CASCADE,
  certification_award_id uuid REFERENCES training.certification_award(id) ON DELETE CASCADE,
  filename text NOT NULL,
  storage_key text NOT NULL,
  sha256 text NOT NULL,
  mime text NOT NULL,
  size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
  uploaded_by uuid REFERENCES training.employee(id),
  uploaded_at timestamptz NOT NULL DEFAULT now(),
  is_validated boolean NOT NULL DEFAULT false,
  validated_by uuid REFERENCES training.employee(id),
  validated_at timestamptz,
  CHECK ((enrollment_id IS NOT NULL)::int + (certification_award_id IS NOT NULL)::int = 1),
  UNIQUE (sha256, storage_key)
);
CREATE INDEX IF NOT EXISTS idx_document_enrollment ON training.document(enrollment_id)
  WHERE enrollment_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_document_award ON training.document(certification_award_id)
  WHERE certification_award_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS training.mandatory_assignment_rule (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  course_id uuid NOT NULL REFERENCES training.course(id) ON DELETE CASCADE,
  team_id uuid REFERENCES training.team(id) ON DELETE CASCADE,
  role_filter text,
  is_active boolean NOT NULL DEFAULT true,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (course_id, team_id, role_filter)
);

CREATE TABLE IF NOT EXISTS training.training_request (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id uuid NOT NULL REFERENCES training.employee(id),
  course_id uuid REFERENCES training.course(id),
  free_text_title text,
  skill_area_id uuid REFERENCES training.skill_area(id),
  motivation text NOT NULL,
  desired_year integer CHECK (desired_year IS NULL OR desired_year BETWEEN 2024 AND 2100),
  status text NOT NULL DEFAULT 'submitted'
    CHECK (status IN ('submitted', 'under_review', 'accepted', 'rejected', 'converted')),
  converted_to_enrollment_id uuid REFERENCES training.enrollment(id),
  review_notes text,
  reviewed_by uuid REFERENCES training.employee(id),
  reviewed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (course_id IS NOT NULL OR free_text_title IS NOT NULL),
  CHECK (status <> 'converted' OR converted_to_enrollment_id IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS idx_training_request_employee ON training.training_request(employee_id);
CREATE INDEX IF NOT EXISTS idx_training_request_open ON training.training_request(status, created_at DESC)
  WHERE status IN ('submitted', 'under_review');

CREATE TABLE IF NOT EXISTS training.learning_path (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code text NOT NULL UNIQUE,
  name text NOT NULL,
  skill_area_id uuid REFERENCES training.skill_area(id),
  description text,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS training.learning_path_step (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  path_id uuid NOT NULL REFERENCES training.learning_path(id) ON DELETE CASCADE,
  step_order smallint NOT NULL,
  course_id uuid REFERENCES training.course(id),
  certification_id uuid REFERENCES training.certification(id),
  is_required boolean NOT NULL DEFAULT true,
  notes text,
  UNIQUE (path_id, step_order),
  CHECK (course_id IS NOT NULL OR certification_id IS NOT NULL)
);

CREATE TABLE IF NOT EXISTS training.employee_learning_path (
  employee_id uuid NOT NULL REFERENCES training.employee(id) ON DELETE CASCADE,
  path_id uuid NOT NULL REFERENCES training.learning_path(id),
  started_on date NOT NULL DEFAULT CURRENT_DATE,
  target_completion date,
  completed_on date,
  notes text,
  PRIMARY KEY (employee_id, path_id),
  CHECK (target_completion IS NULL OR target_completion >= started_on),
  CHECK (completed_on IS NULL OR completed_on >= started_on)
);

CREATE TABLE IF NOT EXISTS training.audit_log (
  id bigserial PRIMARY KEY,
  occurred_at timestamptz NOT NULL DEFAULT now(),
  actor_id uuid REFERENCES training.employee(id),
  entity_type text NOT NULL,
  entity_id uuid NOT NULL,
  action text NOT NULL,
  before_state jsonb,
  after_state jsonb,
  correlation_id uuid
);
CREATE INDEX IF NOT EXISTS idx_audit_entity ON training.audit_log(entity_type, entity_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_actor ON training.audit_log(actor_id, occurred_at DESC);

CREATE OR REPLACE FUNCTION training.set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$;

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'team','employee','vendor','skill_area','certification','course',
    'training_plan','enrollment','certification_award',
    'mandatory_assignment_rule','training_request','learning_path'
  ]
  LOOP
    EXECUTE format('DROP TRIGGER IF EXISTS trg_%I_updated_at ON training.%I', t, t);
    EXECUTE format(
      'CREATE TRIGGER trg_%I_updated_at BEFORE UPDATE ON training.%I
       FOR EACH ROW EXECUTE FUNCTION training.set_updated_at()', t, t);
  END LOOP;
END $$;

CREATE OR REPLACE FUNCTION training.validate_enrollment_transition()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  v_override text;
  v_allowed boolean := false;
BEGIN
  IF NEW.status = OLD.status THEN
    RETURN NEW;
  END IF;

  BEGIN
    v_override := current_setting('training.allow_status_override', true);
  EXCEPTION WHEN OTHERS THEN
    v_override := NULL;
  END;

  IF v_override = 'true' THEN
    RETURN NEW;
  END IF;

  v_allowed := CASE
    WHEN OLD.status = 'proposed' AND NEW.status IN ('approved','cancelled','expired') THEN true
    WHEN OLD.status = 'approved' AND NEW.status IN ('proposed','in_progress','cancelled','expired') THEN true
    WHEN OLD.status = 'in_progress' AND NEW.status IN ('completed','failed','cancelled') THEN true
    WHEN OLD.status IN ('completed','failed','cancelled','expired') AND NEW.status = 'in_progress' THEN true
    ELSE false
  END;

  IF NOT v_allowed THEN
    RAISE EXCEPTION 'Transizione enrollment.status non consentita: % -> % (enrollment_id=%)',
      OLD.status, NEW.status, NEW.id
      USING ERRCODE = 'check_violation',
            HINT = 'Usare il service layer Training oppure SET LOCAL training.allow_status_override=''true'' solo per import storico.';
  END IF;

  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_enrollment_state_guard ON training.enrollment;
CREATE TRIGGER trg_enrollment_state_guard
  BEFORE UPDATE OF status ON training.enrollment
  FOR EACH ROW
  EXECUTE FUNCTION training.validate_enrollment_transition();

DROP VIEW IF EXISTS training.v_employee_certifications;
CREATE VIEW training.v_employee_certifications AS
SELECT
  ca.id AS award_id,
  e.id AS employee_id,
  e.last_name,
  e.first_name,
  c.code AS cert_code,
  c.name AS cert_name,
  ca.outcome,
  ca.awarded_on,
  ca.expires_on,
  CASE
    WHEN ca.outcome <> 'passed_exam' THEN 'not_certified'
    WHEN ca.expires_on IS NULL THEN 'valid_no_expiry'
    WHEN ca.expires_on > CURRENT_DATE THEN 'valid'
    ELSE 'expired'
  END AS current_status,
  ca.validation_source
FROM training.certification_award ca
JOIN training.employee e ON e.id = ca.employee_id
JOIN training.certification c ON c.id = ca.certification_id;

CREATE OR REPLACE VIEW training.v_plan_budget AS
SELECT
  tp.year,
  t.code AS team_code,
  COUNT(*) AS enrollments_count,
  SUM(COALESCE(en.cost_actual, en.cost_planned, c.default_cost)) AS cost_total,
  SUM(COALESCE(en.hours_actual, en.hours_planned, c.default_hours)) AS hours_total
FROM training.enrollment en
JOIN training.training_plan tp ON tp.id = en.training_plan_id
JOIN training.course c ON c.id = en.course_id
LEFT JOIN training.team_membership tm
  ON tm.employee_id = en.employee_id
 AND tm.start_date <= CURRENT_TIMESTAMP
 AND (tm.end_date IS NULL OR tm.end_date >= CURRENT_TIMESTAMP)
LEFT JOIN training.team t ON t.id = tm.team_id
GROUP BY tp.year, t.code;

CREATE OR REPLACE VIEW training.v_expiring_certifications AS
SELECT
  e.id AS employee_id,
  e.last_name,
  e.first_name,
  e.email,
  c.code AS cert_code,
  c.name AS cert_name,
  ca.expires_on,
  (ca.expires_on - CURRENT_DATE) AS days_to_expiry
FROM training.certification_award ca
JOIN training.employee e ON e.id = ca.employee_id
JOIN training.certification c ON c.id = ca.certification_id
WHERE ca.outcome = 'passed_exam'
  AND ca.expires_on IS NOT NULL
  AND ca.expires_on > CURRENT_DATE
  AND e.status = 'active';

CREATE OR REPLACE VIEW training.v_mandatory_compliance_gap AS
WITH active_employees AS (
  SELECT e.id, e.last_name, e.first_name, tm.team_id
  FROM training.employee e
  LEFT JOIN training.team_membership tm
    ON tm.employee_id = e.id
   AND tm.start_date <= CURRENT_TIMESTAMP
   AND (tm.end_date IS NULL OR tm.end_date >= CURRENT_TIMESTAMP)
  WHERE e.status = 'active'
),
required AS (
  SELECT
    ae.id AS employee_id,
    ae.last_name,
    ae.first_name,
    c.id AS course_id,
    c.title AS course_title,
    c.leads_to_cert_id,
    c.compliance_framework,
    c.recurrence_interval
  FROM training.mandatory_assignment_rule r
  JOIN training.course c ON c.id = r.course_id AND c.is_active AND c.is_mandatory
  JOIN active_employees ae ON (r.team_id IS NULL OR r.team_id = ae.team_id)
  WHERE r.is_active
)
SELECT
  req.employee_id,
  req.last_name,
  req.first_name,
  req.course_id,
  req.course_title,
  req.compliance_framework,
  (
    SELECT MAX(ca.awarded_on)
    FROM training.certification_award ca
    WHERE ca.employee_id = req.employee_id
      AND ca.certification_id = req.leads_to_cert_id
      AND ca.outcome = 'passed_exam'
      AND (ca.expires_on IS NULL OR ca.expires_on > CURRENT_DATE)
      AND (req.recurrence_interval IS NULL OR ca.awarded_on + req.recurrence_interval > CURRENT_DATE)
  ) AS last_valid_awarded_on,
  CASE
    WHEN req.leads_to_cert_id IS NULL THEN 'no_cert_linked'
    WHEN EXISTS (
      SELECT 1
      FROM training.certification_award ca
      WHERE ca.employee_id = req.employee_id
        AND ca.certification_id = req.leads_to_cert_id
        AND ca.outcome = 'passed_exam'
        AND (ca.expires_on IS NULL OR ca.expires_on > CURRENT_DATE)
        AND (req.recurrence_interval IS NULL OR ca.awarded_on + req.recurrence_interval > CURRENT_DATE)
    ) THEN 'compliant'
    ELSE 'missing_or_expired'
  END AS compliance_status
FROM required req;

INSERT INTO mrsmith.notification_type (
  type_key,
  app_id,
  title_template,
  body_template,
  severity,
  default_policy
)
VALUES
  (
    'training.certificate_expiring',
    'training',
    'Certificazione in scadenza',
    'Una certificazione richiede attenzione.',
    'warning',
    '{"email":[{"delay":"0s"}]}'::jsonb
  ),
  (
    'training.compliance_gap',
    'training',
    'Formazione obbligatoria da pianificare',
    'Una formazione obbligatoria deve essere assegnata o aggiornata.',
    'warning',
    '{"email":[{"delay":"0s"}]}'::jsonb
  )
ON CONFLICT (type_key) DO UPDATE
SET app_id = EXCLUDED.app_id,
    title_template = EXCLUDED.title_template,
    body_template = EXCLUDED.body_template,
    severity = EXCLUDED.severity,
    default_policy = EXCLUDED.default_policy,
    enabled = true,
    updated_at = now();

COMMIT;
