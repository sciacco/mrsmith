-- Google Drive folder bindings for the Binocolo domain (#98 executing PRD #85;
-- first consumer of the shared Google Drive client #97).
-- Target database: Anisetta PostgreSQL (binocolo schema).
-- Idempotent: apply manually on the database referenced by ANISETTA_DSN.
-- The agent NEVER executes this migration; it is delivered for manual application.
--
-- Design decisions captured here:
--   * Bindings live in the Binocolo domain schema. There is no global
--     app+resource->folder registry in `mrsmith` (see migration 126 header);
--     each consumer domain keeps its own folder bindings.
--   * ON DELETE RESTRICT on both FKs: companies and initiative cards are never
--     physically deleted in the normal flow, so the binding rows are protected.
--   * Bindings are write-once in the first release: only the `created_*`
--     audit triplet is recorded. There are no `updated_*` or `deleted_*`
--     columns and no `set_updated_at` trigger.
--   * The UNIQUE/PK constraints are the safety net against duplicate bindings
--     under concurrent first-access: the Go store holds a row lock on
--     `ma_company` around the check->create->persist sequence.

BEGIN;
SET LOCAL lock_timeout = '5s';

-- ---------------------------------------------------------------------------
-- Company -> Google Drive root folder binding.
-- `company_key` is both the PRIMARY KEY and the UNIQUE constraint (it is the
-- opaque, immutable company identity); one folder per company.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binocolo.ma_company_drive_folder (
    company_key text PRIMARY KEY,
    drive_folder_id text NOT NULL CHECK (btrim(drive_folder_id) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by_subject text,
    created_by_email text,
    FOREIGN KEY (company_key) REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT
);

-- ---------------------------------------------------------------------------
-- Initiative card -> Google Drive subfolder binding.
-- The composite PRIMARY KEY (initiative_id, company_key) mirrors the PK of
-- `binocolo.ma_initiative_card`; one subfolder per (card, company) pair.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binocolo.ma_card_drive_folder (
    initiative_id uuid NOT NULL,
    company_key text NOT NULL,
    drive_folder_id text NOT NULL CHECK (btrim(drive_folder_id) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by_subject text,
    created_by_email text,
    PRIMARY KEY (initiative_id, company_key),
    FOREIGN KEY (initiative_id, company_key) REFERENCES binocolo.ma_initiative_card(initiative_id, company_key) ON DELETE RESTRICT
);

COMMIT;
