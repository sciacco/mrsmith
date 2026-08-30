-- 139_training_trainers.sql
-- Formatore interno a due livelli, simmetrico dello schema corso/evento gia'
-- esistente per fornitore e prezzo: sul corso i formatori designati in
-- pianificazione (colleghi in anagrafica, anche piu' d'uno); alla
-- declinazione in evento il backend copia l'elenco del corso come valore di
-- partenza, poi ogni evento vive di formatori propri. Le sessioni non
-- portano formatori.

BEGIN;

CREATE TABLE IF NOT EXISTS training.course_trainer (
  course_id uuid NOT NULL REFERENCES training.course(id) ON DELETE CASCADE,
  employee_id uuid NOT NULL REFERENCES training.employee(id),
  PRIMARY KEY (course_id, employee_id)
);

CREATE INDEX IF NOT EXISTS idx_course_trainer_employee
  ON training.course_trainer(employee_id);

CREATE TABLE IF NOT EXISTS training.event_trainer (
  event_id uuid NOT NULL REFERENCES training.training_event(id) ON DELETE CASCADE,
  employee_id uuid NOT NULL REFERENCES training.employee(id),
  PRIMARY KEY (event_id, employee_id)
);

CREATE INDEX IF NOT EXISTS idx_event_trainer_employee
  ON training.event_trainer(employee_id);

COMMIT;
