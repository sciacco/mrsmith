-- 090: company-level registry (INIZIATIVE-PRD.md §6) — typed facts + free
-- notes about a company, independent of any Iniziativa/session/card. Pattern
-- mirrors binocolo.ma_company_domain (mig 082): company_key primary key,
-- vat/tax/name snapshot for readability, created_by audit trail.
--
-- Two separate concerns, deliberately not merged (PRD §2/§6):
--   - ma_company_fact: closed-vocabulary, revocable, one active fact per
--     (company_key, kind) at a time (unique partial index on revoked_at IS NULL).
--   - ma_company_note: append-only free text, no revocation.
--
-- Effects are presentation-only (badges/marker in tables, dossier section).
-- NEVER read by gate/routing/scoring (PRD §6, ratified).
--
-- Idempotent. Target: Anisetta (schema binocolo). Applied manually by the user.

CREATE TABLE IF NOT EXISTS binocolo.ma_company_fact (
    id                  uuid PRIMARY KEY,
    company_key         text NOT NULL,
    vat_code            text NOT NULL DEFAULT '',
    tax_code            text NOT NULL DEFAULT '',
    company_name        text NOT NULL DEFAULT '',
    kind                text NOT NULL CHECK (kind IN ('non_vende', 'in_trattativa_altrui', 'da_evitare', 'gia_cliente', 'partner')),
    note                text NOT NULL DEFAULT '',
    created_by_subject  text NOT NULL DEFAULT '',
    created_by_email    text NOT NULL DEFAULT '',
    created_at          timestamptz NOT NULL DEFAULT now(),
    revoked_at          timestamptz,
    revoked_by_subject  text,
    revoked_by_email    text,
    revoke_note         text NOT NULL DEFAULT ''
);

-- Only one active fact per (company, kind) at a time; revoked ones are kept
-- for history and excluded from this constraint.
CREATE UNIQUE INDEX IF NOT EXISTS ma_company_fact_active_idx
    ON binocolo.ma_company_fact (company_key, kind) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS ma_company_fact_company_idx
    ON binocolo.ma_company_fact (company_key);

CREATE TABLE IF NOT EXISTS binocolo.ma_company_note (
    id                  uuid PRIMARY KEY,
    company_key         text NOT NULL,
    body                text NOT NULL,
    created_by_subject  text NOT NULL DEFAULT '',
    created_by_email    text NOT NULL DEFAULT '',
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ma_company_note_company_idx
    ON binocolo.ma_company_note (company_key);
