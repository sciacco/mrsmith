-- 151_training_need.sql
-- Esigenza formativa (#202): entita' master dell'istruttoria, con la rosa dei
-- corsi candidati, le aree facoltative e il collegamento a N richieste. La
-- richiesta guadagna una descrizione propria e torna a poter vivere senza
-- corso: nessun embrione nasce piu' dalla richiesta (decade la regola della
-- 142). Backfill unico: descrizione delle richieste esistenti = titolo del
-- corso. Coerenza fra stato e corso definitivo resta al backend, come le
-- transizioni di stato (stessa scelta della 129).

BEGIN;
SET LOCAL lock_timeout = '5s';

-- 1. Richiesta: descrizione propria, corso facoltativo.
ALTER TABLE training.training_request
  ADD COLUMN IF NOT EXISTS description text;

UPDATE training.training_request r
SET description = c.title
FROM training.course c
WHERE c.id = r.course_id AND r.description IS NULL;

ALTER TABLE training.training_request
  ALTER COLUMN description SET NOT NULL,
  ALTER COLUMN course_id DROP NOT NULL;

-- 2. Esigenza formativa. Stato a kanban mosso a mano; soft delete separato
--    dallo stato, cosi' l'eliminazione conserva lo stato in cui era.
CREATE TABLE IF NOT EXISTS training.training_need (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  description text NOT NULL,
  status text NOT NULL DEFAULT 'new'
    CHECK (status IN ('new', 'scouting', 'finalizing', 'closed', 'cancelled')),
  final_course_id uuid REFERENCES training.course(id),
  notes text,
  reminder_text text,
  reminder_at date,
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_training_need_live
  ON training.training_need(status, updated_at DESC)
  WHERE deleted_at IS NULL;

-- 3. Rosa dei candidati, con nota e rango facoltativi per candidato.
CREATE TABLE IF NOT EXISTS training.training_need_course (
  need_id uuid NOT NULL REFERENCES training.training_need(id) ON DELETE CASCADE,
  course_id uuid NOT NULL REFERENCES training.course(id),
  notes text,
  rank integer,
  added_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (need_id, course_id)
);

CREATE INDEX IF NOT EXISTS idx_training_need_course_course
  ON training.training_need_course(course_id);

-- Il corso definitivo deve essere uno della rosa.
ALTER TABLE training.training_need
  DROP CONSTRAINT IF EXISTS fk_training_need_final_in_rosa,
  ADD CONSTRAINT fk_training_need_final_in_rosa
    FOREIGN KEY (id, final_course_id)
    REFERENCES training.training_need_course(need_id, course_id);

-- 4. Aree facoltative dell'esigenza (utili quando nasce prima delle richieste).
CREATE TABLE IF NOT EXISTS training.training_need_skill_area (
  need_id uuid NOT NULL REFERENCES training.training_need(id) ON DELETE CASCADE,
  skill_area_id uuid NOT NULL REFERENCES training.skill_area(id),
  PRIMARY KEY (need_id, skill_area_id)
);

-- 5. Richieste collegate, senza univocita'.
CREATE TABLE IF NOT EXISTS training.training_need_request (
  need_id uuid NOT NULL REFERENCES training.training_need(id) ON DELETE CASCADE,
  request_id uuid NOT NULL REFERENCES training.training_request(id) ON DELETE CASCADE,
  added_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (need_id, request_id)
);

CREATE INDEX IF NOT EXISTS idx_training_need_request_request
  ON training.training_need_request(request_id);

COMMIT;
