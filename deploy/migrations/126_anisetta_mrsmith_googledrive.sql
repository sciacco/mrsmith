-- Shared Google Drive client configuration for the MrSmith portal (#97).
-- Target database: Anisetta PostgreSQL (mrsmith schema).
-- Apply manually on the database referenced by ANISETTA_DSN.
--
-- Design decisions captured here:
--   * One Service Account shared by the whole backend, stored as the complete
--     JSON key (private key included, plaintext) in a single-row table.
--     Protection is DB ACL + infrastructural encryption; rotation is a
--     controlled SQL UPDATE of credential_json (atomic, effective from the
--     next application request — the Go client cache is keyed on updated_at).
--   * No ENV, file or static Go configuration: the DB is the only source of
--     truth for every Google Drive parameter.
--   * Each (app, context) pair binds to one Shared Drive + one root folder,
--     both provisioned manually. The backend never creates, renames, moves or
--     deletes the configured roots. A missing/disabled row disables only that
--     context (not_configured), with no implicit fallback.
--   * No global app+resource->folder registry: domain bindings stay in the
--     consumer domains' own schemas.

BEGIN;

CREATE SCHEMA IF NOT EXISTS mrsmith;

-- ---------------------------------------------------------------------------
-- Global Service Account credential — single row enforced by the PK.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS mrsmith.googledrive_credential (
  single_row      boolean PRIMARY KEY DEFAULT true CHECK (single_row),
  credential_json jsonb NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);

DROP TRIGGER IF EXISTS googledrive_credential_set_updated_at ON mrsmith.googledrive_credential;
CREATE TRIGGER googledrive_credential_set_updated_at
  BEFORE UPDATE ON mrsmith.googledrive_credential
  FOR EACH ROW EXECUTE FUNCTION mrsmith.set_updated_at();

-- ---------------------------------------------------------------------------
-- Documentary contexts — app + context -> Shared Drive + hard root.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS mrsmith.googledrive_context (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  app             text NOT NULL CHECK (btrim(app) <> ''),
  context         text NOT NULL CHECK (btrim(context) <> ''),
  shared_drive_id text NOT NULL CHECK (btrim(shared_drive_id) <> ''),
  root_folder_id  text NOT NULL CHECK (btrim(root_folder_id) <> ''),
  enabled         boolean NOT NULL DEFAULT true,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (app, context)
);

DROP TRIGGER IF EXISTS googledrive_context_set_updated_at ON mrsmith.googledrive_context;
CREATE TRIGGER googledrive_context_set_updated_at
  BEFORE UPDATE ON mrsmith.googledrive_context
  FOR EACH ROW EXECUTE FUNCTION mrsmith.set_updated_at();

COMMIT;

-- ---------------------------------------------------------------------------
-- Controlled inserts (run separately, NOT part of the migration):
--
-- Credential (paste the full Service Account JSON key):
--   INSERT INTO mrsmith.googledrive_credential (credential_json)
--   VALUES ('<service-account-key-json>'::jsonb)
--   ON CONFLICT (single_row)
--   DO UPDATE SET credential_json = EXCLUDED.credential_json;
--
-- Context (example for the first consumer, Binocolo #85):
--   INSERT INTO mrsmith.googledrive_context (app, context, shared_drive_id, root_folder_id)
--   VALUES ('binocolo', 'documenti', '<shared-drive-id>', '<root-folder-id>');
-- ---------------------------------------------------------------------------
