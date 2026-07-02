-- 085: flag operator-confirmed group sites in the Binocolo M&A domain registry.
--
-- The manual domain registry already stores durable company->domain remedies.
-- This additive column marks the specific case where the operator confirms that
-- the chosen domain is a group site rather than the company's own legal-entity
-- site. Apply manually on Anisetta before deploying code that reads/writes it.
--
-- Idempotent. Target: Anisetta (schema binocolo).

ALTER TABLE binocolo.ma_company_domain
    ADD COLUMN IF NOT EXISTS group_site boolean NOT NULL DEFAULT false;
