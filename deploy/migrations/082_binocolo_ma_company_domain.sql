-- 082: cross-session verified-domain registry for binocolo M&A targets.
--
-- Domain resolution today is session-scoped (ma_target_web_validation): the same
-- company re-pays Brave+scrape resolution in every session it appears in, and a
-- manual domain association does not survive the session. The 2026-07-02 eval on
-- persisted validations showed the failures RECUR identically across sessions
-- (same company, same wrong best-candidate).
--
-- This table stores one durable domain per company, written when:
--   - auto_verified: the target's P.IVA / codice fiscale was confirmed on-page
--     during resolution (the strongest identity signal), or
--   - manual: the operator associated the domain by hand (associate-domain remedy).
--
-- Resolution consults it first (zero retrieval cost). 'manual' entries are never
-- overwritten by 'auto_verified' ones (enforced by the application upsert).
--
-- Idempotent. Target: Anisetta (schema binocolo). Applied manually by the user.

CREATE TABLE IF NOT EXISTS binocolo.ma_company_domain (
    company_key        text PRIMARY KEY,
    vat_code           text NOT NULL DEFAULT '',
    tax_code           text NOT NULL DEFAULT '',
    company_name       text NOT NULL DEFAULT '',
    domain             text NOT NULL,
    method             text NOT NULL CHECK (method IN ('auto_verified', 'manual')),
    verified_at        timestamptz NOT NULL DEFAULT now(),
    created_by_subject text NOT NULL DEFAULT '',
    created_by_email   text NOT NULL DEFAULT '',
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

-- Secondary lookup: the same company can surface with a different company_key
-- derivation across sessions; the P.IVA / codice fiscale are the stable keys.
CREATE INDEX IF NOT EXISTS ma_company_domain_vat_idx
    ON binocolo.ma_company_domain (vat_code) WHERE vat_code <> '';
CREATE INDEX IF NOT EXISTS ma_company_domain_tax_idx
    ON binocolo.ma_company_domain (tax_code) WHERE tax_code <> '';
