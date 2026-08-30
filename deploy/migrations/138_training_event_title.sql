-- 138_training_event_title.sql
-- Titolo sull'evento, ereditato dal corso alla creazione e poi indipendente
-- (stesso schema a due tempi di fornitore/prezzo). Abilita il pattern «un
-- corso, piu' eventi con ruoli diversi» (es. evento-corso ed evento-esame
-- sotto lo stesso titolo a catalogo). Gli eventi esistenti ereditano il
-- titolo del corso.

BEGIN;

ALTER TABLE training.training_event
  ADD COLUMN IF NOT EXISTS title text;

UPDATE training.training_event ev
SET title = c.title
FROM training.course c
WHERE ev.course_id = c.id AND ev.title IS NULL;

ALTER TABLE training.training_event
  ALTER COLUMN title SET NOT NULL;

COMMIT;
