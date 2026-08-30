-- 137_training_planning_annotations.sql
-- Annotazioni di pianificazione decise nel brainstorm di agosto 2026:
--   * nota libera su richiesta e corso (l'evento la possiede gia');
--   * promemoria «in attesa di / prossimo passo» su richiesta, corso ed
--     evento: testo breve + data di richiamo facoltativa (le viste ordinano
--     ed evidenziano i richiami scaduti);
--   * sospensione motivata reversibile su richiesta e corso (non
--     sull'evento, che ha gia' l'annullamento): riattivazione = azzeramento;
--   * priorita' facoltativa sulla richiesta (1 = piu' importante, senza
--     unicita' ne' obblighi).
-- Solo colonne nuove, tutte opzionali. Nessun dato toccato.

BEGIN;

ALTER TABLE training.training_request
  ADD COLUMN IF NOT EXISTS notes text,
  ADD COLUMN IF NOT EXISTS reminder_text text,
  ADD COLUMN IF NOT EXISTS reminder_at date,
  ADD COLUMN IF NOT EXISTS suspended_at timestamptz,
  ADD COLUMN IF NOT EXISTS suspended_by uuid REFERENCES training.employee(id),
  ADD COLUMN IF NOT EXISTS suspension_reason text,
  ADD COLUMN IF NOT EXISTS priority integer;

ALTER TABLE training.course
  ADD COLUMN IF NOT EXISTS notes text,
  ADD COLUMN IF NOT EXISTS reminder_text text,
  ADD COLUMN IF NOT EXISTS reminder_at date,
  ADD COLUMN IF NOT EXISTS suspended_at timestamptz,
  ADD COLUMN IF NOT EXISTS suspended_by uuid REFERENCES training.employee(id),
  ADD COLUMN IF NOT EXISTS suspension_reason text;

ALTER TABLE training.training_event
  ADD COLUMN IF NOT EXISTS reminder_text text,
  ADD COLUMN IF NOT EXISTS reminder_at date;

COMMIT;
