-- Segnalazioni autonome in Binocolo: un utente registra rapidamente un'azienda
-- o un'opportunità con le informazioni che conosce; gli altri utenti la
-- ritrovano in una Kanban condivisa, ne integrano i contenuti e ne gestiscono
-- lo stato.
-- Database di destinazione: Anisetta PostgreSQL, schema binocolo.
-- Idempotente: da applicare a mano sul database indicato da ANISETTA_DSN.
-- L'agente non la esegue mai: sul database la applica l'utente.
--
-- Scelte di progetto registrate qui:
--   * Nessun prefisso `ma_`: la segnalazione nasce fuori dal dominio Target
--     M&A e non è collegata ad alcuna azienda o iniziativa; il collegamento,
--     se arriverà, sarà una migrazione successiva.
--   * I sei campi del form sono testo libero, individualmente facoltativi e
--     mai NULL (stringa vuota = non compilato). La regola «almeno uno tra nome,
--     sito web, identificativo fiscale e note» è dell'applicazione, non un
--     CHECK: vale su creazione e modifica contenuti, ma una riga esistente
--     non deve mai diventare invalida per un cambio di regola.
--   * Lo stato è il vocabolario di business della Kanban (quattro colonne):
--     il CHECK lo fissa, come `kind` in ma_company_agreement.
--   * Nessun trigger: `updated_at` lo scrive l'applicazione su ogni modifica
--     di contenuti o di stato, come per ma_company_agreement (migrazione 146).
--   * Nessun indice oltre la chiave primaria: volumi piccoli, elenco unico
--     ordinato per ultimo aggiornamento.

BEGIN;
SET LOCAL lock_timeout = '5s';

CREATE TABLE IF NOT EXISTS binocolo.segnalazione (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL DEFAULT '',
    website text NOT NULL DEFAULT '',
    location text NOT NULL DEFAULT '',
    fiscal_id text NOT NULL DEFAULT '',
    contacts text NOT NULL DEFAULT '',
    notes text NOT NULL DEFAULT '',
    state text NOT NULL DEFAULT 'da_gestire'
        CHECK (state IN ('da_gestire', 'in_gestione', 'chiusa', 'annullata')),
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by_subject text,
    created_by_email text,
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by_subject text,
    updated_by_email text
);

COMMIT;
