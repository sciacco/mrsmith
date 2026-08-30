-- 135_training_factorial_import_fields.sql
-- Campi di destinazione per l'import Factorial completo deciso ad agosto 2026:
--   * corso: tag liberi (dalle categorie Factorial, testuali, anche multipli);
--   * sessione: argomento, modalita', durata in ore, luogo (dati operativi
--     reali presenti su Factorial e finora scartati).
-- La data di completamento storica non richiede colonne: atterra su
-- enrollment.actual_end (gia' esistente), valorizzata dal sync solo se vuota.
-- Nessun dato viene toccato: solo colonne nuove, tutte opzionali.

BEGIN;

ALTER TABLE training.course
  ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}';

ALTER TABLE training.training_session
  ADD COLUMN IF NOT EXISTS topic text,
  ADD COLUMN IF NOT EXISTS modality text,
  ADD COLUMN IF NOT EXISTS duration_hours numeric(6,2)
    CHECK (duration_hours IS NULL OR duration_hours >= 0),
  ADD COLUMN IF NOT EXISTS location text;

ALTER TABLE training.enrollment_session
  ADD COLUMN IF NOT EXISTS completed_hours numeric(6,2)
    CHECK (completed_hours IS NULL OR completed_hours >= 0);

COMMIT;
