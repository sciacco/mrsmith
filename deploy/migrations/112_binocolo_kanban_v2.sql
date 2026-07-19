-- 112_binocolo_kanban_v2.sql
-- Kanban v2: nuovo set a 10 stati operativi + `rimossa`, esito a testo libero,
-- data di ricontatto informativa. KANBAN-V2-PLAN.md §3.
--
-- Ordine obbligato: si DROPPA prima il CHECK stati vecchio, POI si rimappa (un
-- UPDATE verso un valore del nuovo set violerebbe altrimenti il CHECK vecchio),
-- POI si aggiunge il CHECK nuovo (a quel punto tutte le righe sono nel nuovo set).
-- La applica l'utente con il suo processo (mai operazioni dirette sui DB in env).

BEGIN;

-- 1) Drop dei vincoli vecchi PRIMA del remapping.
--    a) CHECK stati (nome inline auto-generato da 088) con fallback su pg_constraint.
ALTER TABLE binocolo.ma_initiative_card DROP CONSTRAINT IF EXISTS ma_initiative_card_state_check;
DO $$
DECLARE
  c text;
BEGIN
  SELECT conname INTO c
  FROM pg_constraint
  WHERE conrelid = 'binocolo.ma_initiative_card'::regclass
    AND contype = 'c'
    AND pg_get_constraintdef(oid) ILIKE '%state%'
    AND pg_get_constraintdef(oid) ILIKE '%approfondimento%';
  IF c IS NOT NULL THEN
    EXECUTE format('ALTER TABLE binocolo.ma_initiative_card DROP CONSTRAINT %I', c);
  END IF;
END $$;

--    b) CHECK esito: si toglie (il vocabolario resta suggerimento applicativo,
--       obbligo non-vuoto solo per i KO imposto dal backend).
ALTER TABLE binocolo.ma_initiative_card DROP CONSTRAINT IF EXISTS ma_initiative_card_esito_check;
DO $$
DECLARE
  c text;
BEGIN
  SELECT conname INTO c
  FROM pg_constraint
  WHERE conrelid = 'binocolo.ma_initiative_card'::regclass
    AND contype = 'c'
    AND pg_get_constraintdef(oid) ILIKE '%esito%';
  IF c IS NOT NULL THEN
    EXECUTE format('ALTER TABLE binocolo.ma_initiative_card DROP CONSTRAINT %I', c);
  END IF;
END $$;

-- 2) Mapping stati pregressi → nuovo set (idempotente, ora senza CHECK sugli stati).
UPDATE binocolo.ma_initiative_card SET state = 'primo_contatto' WHERE state = 'contattata';
UPDATE binocolo.ma_initiative_card SET state = 'primo_incontro' WHERE state = 'in_dialogo';
UPDATE binocolo.ma_initiative_card SET state = 'loi'            WHERE state = 'offerta';
UPDATE binocolo.ma_initiative_card SET state = 'ko_target'      WHERE state = 'chiusa';
-- `approfondimento`, `da_contattare`, `rimossa` restano invariati.
-- L'esito esistente resta com'è (il vocabolario diventa suggerimento applicativo).

-- 3) Nuovo CHECK stati (a questo punto tutte le righe sono nel nuovo set).
ALTER TABLE binocolo.ma_initiative_card ADD CONSTRAINT ma_initiative_card_state_check
  CHECK (state IN ('approfondimento','da_contattare','primo_contatto','primo_incontro',
                   'nda','loi','ricontattare','won','ko_nostro','ko_target','rimossa'));

-- 4) Data di ricontatto (informativa, nullable): chip su `ricontattare`, azzerata
--    quando la card lascia quello stato. Nessun reminder engine (vincolo non-CRM).
ALTER TABLE binocolo.ma_initiative_card ADD COLUMN IF NOT EXISTS recontact_on date;

-- Il DEFAULT 'da_contattare' (088) resta valido nel nuovo set: non si tocca.
-- ma_target_outcome_event_check NON si tocca: i nuovi passaggi restano nel
-- vocabolario eventi esistente (`stato` con payload from/to, `chiusura` con
-- payload esito+stato terminale). I payload storici del diario NON si riscrivono.

COMMIT;
