CREATE TABLE binocolo.ma_company_contact (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_key text NOT NULL REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT,
    name text NOT NULL CHECK (btrim(name) <> ''),
    relationship text,
    contact_details text,
    note text,
    is_primary boolean NOT NULL DEFAULT false,
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

CREATE INDEX ma_company_contact_active_company_idx
    ON binocolo.ma_company_contact (company_key, is_primary DESC, lower(name), created_at)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX ma_company_contact_one_primary_idx
    ON binocolo.ma_company_contact (company_key)
    WHERE is_primary AND deleted_at IS NULL;
