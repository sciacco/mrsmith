-- 089: ma_target_outcome evolve nel log unico degli eventi di lavorazione
-- (workstream D3, INIZIATIVE-PRD.md §5). Un sistema solo, non due verità: il
-- diario È la ground truth che validerà lo score (scopo dichiarato della 080).
--
-- - session_id diventa nullable: resta come provenienza quando l'evento nasce
--   in contesto sessione, ma il diario sopravvive alla cancellazione della
--   sessione (FK da ON DELETE CASCADE a ON DELETE SET NULL).
-- - initiative_id è il nuovo ancoraggio primario dei nuovi eventi di card.
-- - payload jsonb per i dettagli evento (from/to di stato, esito, ecc.).
-- - vocabolario eventi esteso: i tre storici (contattato/buon_lead/no_go)
--   restano validi, si aggiungono gli eventi di card
--   (card_creata/card_rimossa/card_riaperta/stato/nota/chiusura).
--
-- Idempotente. Target: Anisetta (schema binocolo). Applicare prima del deploy
-- del codice che scrive gli eventi di card (hook setTargetRating, B4).

ALTER TABLE binocolo.ma_target_outcome
    ALTER COLUMN session_id DROP NOT NULL;

DO $$
DECLARE
    existing_fk text;
BEGIN
    SELECT conname
    INTO existing_fk
    FROM pg_constraint
    WHERE conrelid = 'binocolo.ma_target_outcome'::regclass
      AND contype = 'f'
      AND pg_get_constraintdef(oid) LIKE '%session_id%ma_session%'
    LIMIT 1;

    IF existing_fk IS NOT NULL THEN
        EXECUTE format('ALTER TABLE binocolo.ma_target_outcome DROP CONSTRAINT %I', existing_fk);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'binocolo.ma_target_outcome'::regclass
          AND conname = 'ma_target_outcome_session_id_fkey'
    ) THEN
        ALTER TABLE binocolo.ma_target_outcome
            ADD CONSTRAINT ma_target_outcome_session_id_fkey
            FOREIGN KEY (session_id) REFERENCES binocolo.ma_session(id) ON DELETE SET NULL;
    END IF;
END $$;

ALTER TABLE binocolo.ma_target_outcome
    ADD COLUMN IF NOT EXISTS initiative_id uuid REFERENCES binocolo.ma_initiative(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS payload jsonb NOT NULL DEFAULT '{}'::jsonb;

DO $$
DECLARE
    existing_check text;
BEGIN
    SELECT conname
    INTO existing_check
    FROM pg_constraint
    WHERE conrelid = 'binocolo.ma_target_outcome'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) LIKE '%event%'
    LIMIT 1;

    IF existing_check IS NOT NULL THEN
        EXECUTE format('ALTER TABLE binocolo.ma_target_outcome DROP CONSTRAINT %I', existing_check);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'binocolo.ma_target_outcome'::regclass
          AND conname = 'ma_target_outcome_event_check'
    ) THEN
        ALTER TABLE binocolo.ma_target_outcome
            ADD CONSTRAINT ma_target_outcome_event_check
            CHECK (event IN ('contattato', 'buon_lead', 'no_go', 'card_creata', 'card_rimossa', 'card_riaperta', 'stato', 'nota', 'chiusura'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS ma_target_outcome_initiative_company_idx
    ON binocolo.ma_target_outcome (initiative_id, company_key)
    WHERE initiative_id IS NOT NULL;
