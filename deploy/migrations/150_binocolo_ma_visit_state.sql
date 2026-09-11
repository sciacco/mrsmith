-- Binocolo issue #201: stato operativo `visita` e relativa data informativa.
-- Database di destinazione: Anisetta PostgreSQL, schema binocolo.
--
-- Migrazione additiva e compatibile con il codice attualmente in esercizio:
-- amplia soltanto il vocabolario degli stati e aggiunge una colonna nullable.
-- Le scritture in `visita` iniziano con il rilascio della nuova interfaccia.
-- Nessuna card esistente viene modificata o riceve una data visita.
--
-- La applica l'utente attraverso il proprio processo. L'agente non la esegue
-- mai sui database configurati negli env.

BEGIN;
SET LOCAL lock_timeout = '5s';

ALTER TABLE binocolo.ma_initiative_card
  ADD COLUMN IF NOT EXISTS visit_on date;

ALTER TABLE binocolo.ma_initiative_card
  DROP CONSTRAINT IF EXISTS ma_initiative_card_state_check;

ALTER TABLE binocolo.ma_initiative_card
  ADD CONSTRAINT ma_initiative_card_state_check
  CHECK (state IN (
    'approfondimento',
    'da_contattare',
    'primo_contatto',
    'primo_incontro',
    'visita',
    'nda',
    'loi',
    'ricontattare',
    'won',
    'ko_nostro',
    'ko_target',
    'rimossa'
  ));

COMMIT;
