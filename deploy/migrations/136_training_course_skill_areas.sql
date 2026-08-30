-- 136_training_course_skill_areas.sql
-- Il legame con le aree di competenza passa da singolo a multiplo per corso
-- e richiesta formativa (la richiesta mappa sui corsi): un corso sviluppa
-- piu' competenze. I valori esistenti delle colonne singole vengono
-- travasati nelle tabelle ponte prima della rimozione.

BEGIN;

CREATE TABLE IF NOT EXISTS training.course_skill_area (
  course_id uuid NOT NULL REFERENCES training.course(id) ON DELETE CASCADE,
  skill_area_id uuid NOT NULL REFERENCES training.skill_area(id),
  PRIMARY KEY (course_id, skill_area_id)
);

INSERT INTO training.course_skill_area (course_id, skill_area_id)
SELECT id, skill_area_id FROM training.course
WHERE skill_area_id IS NOT NULL
ON CONFLICT DO NOTHING;

ALTER TABLE training.course DROP COLUMN IF EXISTS skill_area_id;

CREATE TABLE IF NOT EXISTS training.training_request_skill_area (
  request_id uuid NOT NULL REFERENCES training.training_request(id) ON DELETE CASCADE,
  skill_area_id uuid NOT NULL REFERENCES training.skill_area(id),
  PRIMARY KEY (request_id, skill_area_id)
);

INSERT INTO training.training_request_skill_area (request_id, skill_area_id)
SELECT id, skill_area_id FROM training.training_request
WHERE skill_area_id IS NOT NULL
ON CONFLICT DO NOTHING;

ALTER TABLE training.training_request DROP COLUMN IF EXISTS skill_area_id;

COMMIT;
