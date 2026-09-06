-- Accordi sottoscritti dall'azienda nel dominio Binocolo (primo tipo: NDA).
-- Database di destinazione: Anisetta PostgreSQL, schema binocolo.
-- Idempotente: da applicare a mano sul database indicato da ANISETTA_DSN.
-- L'agente non la esegue mai: sul database la applica l'utente.
--
-- Scelte di progetto registrate qui:
--   * Un'azienda può avere più accordi, anche dello stesso tipo (es. NDA
--     rinnovati nel tempo): nessun indice UNIQUE su (company_key, kind).
--   * Cancellazione logica via `deleted_at`: le righe rimosse escono dai filtri
--     applicativi ma restano in tabella; gli eventi di diario collegati non
--     vengono toccati.
--   * Nessun trigger su questa tabella: `updated_at` lo scrive l'applicazione
--     sugli aggiornamenti, come per ma_company_contact (migrazione 125).
--   * `expires_on` è facoltativo: NULL significa nessuna scadenza nota.
--   * L'evento `accordo` nel diario (`ma_target_outcome`) registrerà azione,
--     date e stato prima/dopo; l'allargamento del CHECK qui sotto è additivo
--     e compatibile con l'applicazione attualmente in esercizio, che non
--     scrive mai `accordo` finché non viene rilasciato il backend corrispondente.

BEGIN;
SET LOCAL lock_timeout = '5s';

-- ---------------------------------------------------------------------------
-- Accordi per azienda: oggi solo NDA, il vocabolario `kind` è aperto a futuri
-- tipi tramite una migrazione successiva che allarghi il CHECK.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binocolo.ma_company_agreement (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_key text NOT NULL REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT,
    kind text NOT NULL CHECK (kind IN ('nda')),
    signed_on date NOT NULL,
    expires_on date, -- NULL = nessuna scadenza
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by_subject text,
    created_by_email text,
    updated_at timestamptz,
    updated_by_subject text,
    updated_by_email text,
    deleted_at timestamptz,
    deleted_by_subject text,
    deleted_by_email text
);

CREATE INDEX IF NOT EXISTS ma_company_agreement_active_company_idx
    ON binocolo.ma_company_agreement (company_key, signed_on DESC, created_at DESC)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- Diario: aggiunge l'evento `accordo` al vocabolario esistente.
-- ---------------------------------------------------------------------------
ALTER TABLE binocolo.ma_target_outcome DROP CONSTRAINT IF EXISTS ma_target_outcome_event_check;
ALTER TABLE binocolo.ma_target_outcome ADD CONSTRAINT ma_target_outcome_event_check
  CHECK (event IN ('contattato', 'buon_lead', 'no_go', 'card_creata', 'card_rimossa', 'card_riaperta', 'stato', 'nota', 'chiusura', 'dominio_verificato', 'accordo'));

COMMIT;
