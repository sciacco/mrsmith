-- 142_training_request_course_embryo.sql
-- Il titolo libero sparisce dalla richiesta: un titolo e' l'embrione di un
-- corso, ogni richiesta aggancia sempre un corso (creato contestualmente
-- come embrione — solo nome — quando serve). I vincoli di completezza della
-- scheda si spostano dalla nascita alla maturazione del corso.
-- Le richieste esistenti con solo titolo libero vengono agganciate a un
-- corso con quel titolo (riusato se gia' presente, altrimenti creato come
-- embrione); poi la colonna e il vincolo storico decadono.

BEGIN;

-- Il CHECK di 012 (course_id oppure free_text_title) e' anonimo: si rimuove
-- cercandolo per definizione.
DO $$
DECLARE c record;
BEGIN
  FOR c IN
    SELECT conname
    FROM pg_constraint
    WHERE conrelid = 'training.training_request'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) ILIKE '%free_text_title%'
  LOOP
    EXECUTE format('ALTER TABLE training.training_request DROP CONSTRAINT %I', c.conname);
  END LOOP;
END $$;

INSERT INTO training.course (title)
SELECT DISTINCT r.free_text_title
FROM training.training_request r
WHERE r.course_id IS NULL
  AND r.free_text_title IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM training.course c WHERE c.title = r.free_text_title
  );

UPDATE training.training_request r
SET course_id = c.id
FROM training.course c
WHERE r.course_id IS NULL
  AND c.title = r.free_text_title;

ALTER TABLE training.training_request
  ALTER COLUMN course_id SET NOT NULL;

ALTER TABLE training.training_request
  DROP COLUMN IF EXISTS free_text_title;

CREATE INDEX IF NOT EXISTS idx_training_request_course
  ON training.training_request(course_id);

COMMIT;
