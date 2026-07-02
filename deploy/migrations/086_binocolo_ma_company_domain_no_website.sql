-- 086: allow durable "no official website" declarations for Binocolo M&A.
--
-- The operator can declare that a company has no official website. The registry
-- stores this as method='no_website' with domain='', so future sessions skip
-- web resolution and the gated flow continues on structured data only.
-- Apply manually on Anisetta before deploying code that writes method='no_website'.
--
-- Idempotent. Target: Anisetta (schema binocolo).

DO $$
DECLARE
    existing_constraint text;
BEGIN
    SELECT conname
    INTO existing_constraint
    FROM pg_constraint
    WHERE conrelid = 'binocolo.ma_company_domain'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) LIKE '%method%'
    LIMIT 1;

    IF existing_constraint IS NOT NULL THEN
        EXECUTE format('ALTER TABLE binocolo.ma_company_domain DROP CONSTRAINT %I', existing_constraint);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'binocolo.ma_company_domain'::regclass
          AND conname = 'ma_company_domain_method_check'
    ) THEN
        ALTER TABLE binocolo.ma_company_domain
            ADD CONSTRAINT ma_company_domain_method_check
            CHECK (method IN ('auto_verified', 'manual', 'no_website'));
    END IF;
END $$;
