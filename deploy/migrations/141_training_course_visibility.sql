-- 141_training_course_visibility.sql
-- Platea di visibilita' sul corso, con lo stesso vocabolario delle platee
-- delle regole (tutti / team / area / gruppo / persone). Facoltativa:
-- assente = corso riservato a People (default alla nascita, embrioni e
-- import inclusi); «all» = pubblico; una cerchia = selezionabile da quella
-- cerchia. Distinta dalla platea delle regole: la regola dice «dovete», il
-- corso dice «potete scegliermi». People vede sempre tutto; is_active resta
-- il solo interruttore di vita. Per kind «people» i destinatari stanno nel
-- ponte course_visibility_person.

BEGIN;

ALTER TABLE training.course
  ADD COLUMN IF NOT EXISTS visibility_target jsonb;

ALTER TABLE training.course
  DROP CONSTRAINT IF EXISTS chk_course_visibility_kind,
  ADD CONSTRAINT chk_course_visibility_kind
    CHECK (
      visibility_target IS NULL
      OR (
        jsonb_typeof(visibility_target) = 'object'
        AND visibility_target->>'kind' IN ('all', 'team', 'skill_area', 'custom_group', 'people')
      )
    );

CREATE TABLE IF NOT EXISTS training.course_visibility_person (
  course_id uuid NOT NULL REFERENCES training.course(id) ON DELETE CASCADE,
  employee_id uuid NOT NULL REFERENCES training.employee(id) ON DELETE CASCADE,
  PRIMARY KEY (course_id, employee_id)
);

CREATE INDEX IF NOT EXISTS idx_course_visibility_person_employee
  ON training.course_visibility_person(employee_id);

COMMIT;
