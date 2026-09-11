-- Le richieste formative possono non avere un team quando la persona non ha
-- appartenenze attive. L'obbligo condizionato e la validazione
-- dell'appartenenza restano responsabilita' del backend (issue #200).
--
-- Migrazione idempotente, senza backfill: i team gia' registrati restano
-- invariati. Da applicare a mano dall'utente; l'agente non la esegue mai.

BEGIN;
SET LOCAL lock_timeout = '5s';

ALTER TABLE training.training_request
  ALTER COLUMN selected_team_id DROP NOT NULL;

COMMIT;
