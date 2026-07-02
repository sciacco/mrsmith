-- 088: card di lavorazione (workstream D3, INIZIATIVE-PRD.md §4.1-4.2, §5).
--
-- La prima >=1 stella su un'azienda in una sessione agganciata crea la card,
-- stato iniziale "da_contattare" (PRD §4.1). Dopo il primo evento a diario o
-- avanzamento di stato la stella non governa più la card (autonomia, §4.2):
-- lo stato operativo è l'unica verità corrente. Chiave primaria unica
-- (iniziativa, azienda): la riapertura riusa la stessa card, mai una seconda
-- (§4.3).
--
-- Idempotente. Target: Anisetta (schema binocolo). Applicare prima del deploy
-- del codice che referenzia questa tabella (l'hook su setTargetRating, mig
-- 089 companion).

CREATE TABLE IF NOT EXISTS binocolo.ma_initiative_card (
    initiative_id uuid NOT NULL REFERENCES binocolo.ma_initiative(id) ON DELETE CASCADE,
    company_key text NOT NULL,
    company_name text NOT NULL DEFAULT '',
    vat_code text NOT NULL DEFAULT '',
    tax_code text NOT NULL DEFAULT '',
    province text NOT NULL DEFAULT '',
    state text NOT NULL DEFAULT 'da_contattare'
        CHECK (state IN ('da_contattare', 'contattata', 'in_dialogo', 'approfondimento', 'offerta', 'chiusa', 'rimossa')),
    esito text CHECK (esito IN ('conclusa', 'no_go', 'non_idonea', 'sfumata', 'rimandata')),
    created_from_session uuid REFERENCES binocolo.ma_session(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    closed_at timestamptz,
    PRIMARY KEY (initiative_id, company_key)
);

CREATE INDEX IF NOT EXISTS ma_initiative_card_company_key_idx
    ON binocolo.ma_initiative_card (company_key);
