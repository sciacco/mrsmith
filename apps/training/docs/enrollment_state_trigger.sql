-- =============================================================================
-- CDLAN Training Tool — Enrollment state machine guard
--
-- Rete di sicurezza al livello DB. Impedisce transizioni di enrollment.status
-- non consentite, indipendentemente da chi fa l'UPDATE (applicazione, psql, dba).
--
-- Le precondizioni "esterne" (chi è l'attore, esiste la reason, lo stato del
-- piano) NON sono verificate qui: vivono nell'application layer, dove c'è
-- contesto sufficiente. Qui blocchiamo solo transizioni meccanicamente assurde.
--
-- Per bypass legittimi (migrazione storico), usare:
--   SET LOCAL training.allow_status_override = 'true';
-- dentro la stessa transazione.
-- =============================================================================

SET search_path TO training, public;

CREATE OR REPLACE FUNCTION training.validate_enrollment_transition()
RETURNS trigger AS $$
DECLARE
  v_override text;
  v_allowed  boolean := false;
BEGIN
  -- Niente da validare se lo stato non cambia
  IF NEW.status = OLD.status THEN
    RETURN NEW;
  END IF;

  -- Bypass esplicito (es. import storico, fix dati)
  BEGIN
    v_override := current_setting('training.allow_status_override', true);
  EXCEPTION WHEN OTHERS THEN
    v_override := NULL;
  END;

  IF v_override = 'true' THEN
    RETURN NEW;
  END IF;

  -- Matrice meccanica consentita. Le regole business complete vivono nel backend Go.
  v_allowed := CASE
    -- da proposed
    WHEN OLD.status = 'proposed'    AND NEW.status IN ('approved','cancelled','expired') THEN true
    -- da approved
    WHEN OLD.status = 'approved'    AND NEW.status IN ('proposed','in_progress','cancelled','expired') THEN true
    -- da in_progress
    WHEN OLD.status = 'in_progress' AND NEW.status IN ('completed','failed','cancelled') THEN true
    -- reopen da uno qualunque dei terminali
    WHEN OLD.status IN ('completed','failed','cancelled','expired') AND NEW.status = 'in_progress' THEN true
    ELSE false
  END;

  IF NOT v_allowed THEN
    RAISE EXCEPTION 'Transizione enrollment.status non consentita: % -> % (enrollment_id=%)',
      OLD.status, NEW.status, NEW.id
      USING ERRCODE = 'check_violation',
            HINT    = 'Usare il service layer Training oppure SET LOCAL training.allow_status_override=''true'' solo per import storico.';
  END IF;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_enrollment_state_guard ON training.enrollment;
CREATE TRIGGER trg_enrollment_state_guard
  BEFORE UPDATE OF status ON training.enrollment
  FOR EACH ROW
  EXECUTE FUNCTION training.validate_enrollment_transition();

COMMENT ON FUNCTION training.validate_enrollment_transition() IS
  'Guard meccanica delle transizioni di enrollment.status; le regole business complete vivono nel backend Go.';
