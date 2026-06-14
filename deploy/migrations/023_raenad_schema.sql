-- Raenad V1 operational schema for new Aenad quotes (preventivi).
-- Target database: Mistra PostgreSQL.
-- Apply manually on the database referenced by MISTRA_DSN.
--
-- Raenad is the operational write model for the new quote-creation flow (parent #56),
-- kept separate from the legacy read-only `aenad` archive. Conventions:
--   * lowercase-only objects and columns;
--   * monetary/quantity fields are numeric(18,4), exposed as decimal strings by the API;
--   * totals are DB-owned, replicating the verified aenad logic in 021_aenad_document_totals.sql;
--   * quote numbering reuses the shared common.new_document_number('AE-') sequence;
--   * HubSpot stays the source of truth of commercial state; raenad only snapshots/syncs it.
--
-- V1 EXCLUSIONS (intentionally not modelled): qta_shown (use qta), eco-contributi,
-- ritenute, mandatory contact phone. The shared VAT master remains aenad."TIva".

BEGIN;

CREATE SCHEMA IF NOT EXISTS raenad;

COMMENT ON SCHEMA raenad IS
  'Operational schema for new Aenad quotes (preventivi), separate from the legacy aenad archive. '
  'V1 exclusions: qta_shown (use qta), eco-contributi, ritenute, mandatory contact phone.';

-- ---------------------------------------------------------------------------
-- Tables
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS raenad.quote (
  id                      bigserial PRIMARY KEY,
  quote_number            text NOT NULL DEFAULT common.new_document_number('AE-'),
  created_at              timestamptz NOT NULL DEFAULT now(),
  updated_at              timestamptz NOT NULL DEFAULT now(),
  created_by              text NOT NULL DEFAULT '',
  updated_by              text NOT NULL DEFAULT '',

  -- authoring + technical sync (NOT a parallel commercial taxonomy)
  authoring_status        text NOT NULL DEFAULT 'draft',
  hubspot_sync_status     text NOT NULL DEFAULT 'pending',
  hubspot_sync_error      text,
  hubspot_synced_at       timestamptz,

  -- HubSpot references / snapshot of commercial state
  hubspot_company_id      text,
  hubspot_contact_id      text,
  hubspot_deal_id         text,
  hubspot_pipeline_id     text,
  hubspot_pipeline_label  text,
  hubspot_dealstage_id    text,
  hubspot_dealstage_label text,

  -- printable customer snapshot
  customer_name           text,
  customer_vat            text,
  customer_tax_code       text,
  customer_pec            text,
  customer_email          text,
  customer_address        text,
  customer_zip            text,
  customer_city           text,
  customer_province       text,
  customer_country        text,
  customer_language       text,
  numero_azienda_snapshot text,

  -- printable contact snapshot (V1 set)
  contact_first_name      text,
  contact_last_name       text,
  contact_full_name       text,
  contact_email           text,
  contact_role            text,

  -- document fields
  document_date           date,
  payment_method_code     text,
  payment_method_label    text,
  payment_bank_details    text,
  description             text,
  internal_notes          text,

  -- DB-owned totals
  total_net               numeric(18,4) NOT NULL DEFAULT 0,
  total_vat               numeric(18,4) NOT NULL DEFAULT 0,
  total_gross             numeric(18,4) NOT NULL DEFAULT 0,
  total_purchase          numeric(18,4) NOT NULL DEFAULT 0,
  total_gain              numeric(18,4) NOT NULL DEFAULT 0,

  CONSTRAINT quote_number_unique UNIQUE (quote_number),
  CONSTRAINT quote_authoring_status_check
    CHECK (authoring_status IN ('draft', 'ready', 'archived')),
  CONSTRAINT quote_hubspot_sync_status_check
    CHECK (hubspot_sync_status IN ('pending', 'succeeded', 'failed'))
);

COMMENT ON COLUMN raenad.quote.numero_azienda_snapshot IS
  'Snapshot of loader.hubs_company.numero_azienda (Alyante ERP id) at quote time. No physical FK to the mirror.';

CREATE TABLE IF NOT EXISTS raenad.quote_line (
  id                   bigserial PRIMARY KEY,
  quote_id             bigint NOT NULL REFERENCES raenad.quote(id) ON DELETE CASCADE,
  position             integer NOT NULL DEFAULT 0,
  line_type            text NOT NULL,

  -- commercial content (item lines); spacer/description rows need no economics
  item_code            text,
  item_description     text,
  description          text,
  unit_of_measure      text,
  qta                  numeric(18,4),
  unit_price           numeric(18,4),
  discounts            text,
  cod_iva              text,
  iva_percent_snapshot numeric(9,4),
  purchase_unit_price  numeric(18,4),

  -- DB-owned line totals (NULL for non-item rows)
  line_net             numeric(18,4),
  line_vat             numeric(18,4),
  line_gross           numeric(18,4),
  line_purchase        numeric(18,4),
  line_gain            numeric(18,4),

  CONSTRAINT quote_line_type_check
    CHECK (line_type IN ('spacer', 'description', 'item'))
);

COMMENT ON COLUMN raenad.quote_line.qta IS
  'Canonical quantity. Legacy qta_shown is intentionally not modelled in V1.';
COMMENT ON COLUMN raenad.quote_line.iva_percent_snapshot IS
  'VAT percent snapshot for calc/export stability. Resolved from aenad."TIva".PercIva via cod_iva when not provided. '
  'Sticky: once set it is NOT re-resolved when cod_iva changes; set this column to NULL to force re-resolution from the new cod_iva.';
COMMENT ON COLUMN raenad.quote_line.item_description IS
  'Description imported from the Alyante article; remains editable alongside the free-text description column.';

CREATE TABLE IF NOT EXISTS raenad.quote_pdf_export (
  id                        bigserial PRIMARY KEY,
  quote_id                  bigint NOT NULL REFERENCES raenad.quote(id) ON DELETE CASCADE,
  revision                  integer NOT NULL,
  filename                  text NOT NULL DEFAULT '',
  content_type              text NOT NULL DEFAULT 'application/pdf',
  checksum_sha256           text,
  render_payload            jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at                timestamptz NOT NULL DEFAULT now(),
  created_by                text NOT NULL DEFAULT '',

  -- HubSpot attachment status (mutable; updating it must not rewrite history)
  hubspot_attachment_status text NOT NULL DEFAULT 'pending',
  hubspot_file_id           text,
  hubspot_note_id           text,
  hubspot_deal_id           text,
  hubspot_attached_at       timestamptz,
  hubspot_error             text,

  CONSTRAINT quote_pdf_export_revision_unique UNIQUE (quote_id, revision),
  CONSTRAINT quote_pdf_export_attachment_status_check
    CHECK (hubspot_attachment_status IN ('pending', 'attached', 'failed', 'skipped'))
);

COMMENT ON TABLE raenad.quote_pdf_export IS
  'Immutable PDF export revisions. Content columns are immutable (guarded by trigger); '
  'only the hubspot_* attachment status fields may be updated.';

CREATE TABLE IF NOT EXISTS raenad.quote_event (
  id            bigserial PRIMARY KEY,
  quote_id      bigint NOT NULL REFERENCES raenad.quote(id) ON DELETE CASCADE,
  event_type    text NOT NULL,
  actor_subject text NOT NULL DEFAULT '',
  payload       jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at    timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Indexes
-- ---------------------------------------------------------------------------

CREATE INDEX IF NOT EXISTS quote_hubspot_deal_idx     ON raenad.quote (hubspot_deal_id);
CREATE INDEX IF NOT EXISTS quote_hubspot_company_idx  ON raenad.quote (hubspot_company_id);
CREATE INDEX IF NOT EXISTS quote_hubspot_sync_idx     ON raenad.quote (hubspot_sync_status);
CREATE INDEX IF NOT EXISTS quote_document_date_idx    ON raenad.quote (document_date);
CREATE INDEX IF NOT EXISTS quote_line_quote_pos_idx   ON raenad.quote_line (quote_id, position);
CREATE INDEX IF NOT EXISTS quote_event_quote_idx      ON raenad.quote_event (quote_id, created_at);

-- ---------------------------------------------------------------------------
-- Calculation helpers (lowercase replica of aenad 021 logic)
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION raenad.discount_multiplier(discounts text)
RETURNS numeric
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
  cleaned text;
  part text;
  percent numeric;
  multiplier numeric := 1;
BEGIN
  cleaned := regexp_replace(coalesce(discounts, ''), '[[:space:]]+', '', 'g');
  cleaned := replace(cleaned, ',', '.');

  IF cleaned = '' THEN
    RETURN 1;
  END IF;

  FOREACH part IN ARRAY string_to_array(cleaned, '+')
  LOOP
    part := regexp_replace(part, '%$', '');

    IF part = '' OR part !~ '^[+-]?[0-9]+([.][0-9]+)?$' THEN
      RAISE EXCEPTION 'Invalid raenad discount value: %', discounts
        USING ERRCODE = '22023';
    END IF;

    percent := part::numeric;
    multiplier := multiplier * (1 - (percent / 100));
  END LOOP;

  RETURN multiplier;
END;
$$;

CREATE OR REPLACE FUNCTION raenad.line_net(
  unit_price numeric,
  quantity numeric,
  discounts text
)
RETURNS numeric
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
  multiplier numeric;
BEGIN
  multiplier := raenad.discount_multiplier(discounts);

  IF unit_price IS NULL OR quantity IS NULL THEN
    RETURN NULL;
  END IF;

  RETURN (unit_price * quantity * multiplier)::numeric(18,4);
END;
$$;

-- VAT fallback resolution: aenad."TIva" is the shared VAT master on the same Mistra DB.
CREATE OR REPLACE FUNCTION raenad.resolve_iva_percent(cod_iva text)
RETURNS numeric
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
  vat_percent numeric;
BEGIN
  IF cod_iva IS NULL THEN
    RETURN NULL;
  END IF;

  SELECT "PercIva"::numeric
  INTO vat_percent
  FROM aenad."TIva"
  WHERE "CodIva" = cod_iva;

  RETURN vat_percent;
END;
$$;

-- ---------------------------------------------------------------------------
-- Line totals + header recalculation triggers
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION raenad.quote_line_set_totals()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  pct numeric;
BEGIN
  -- Non-economic rows (spacer/description) carry no amounts.
  IF NEW.line_type <> 'item' THEN
    NEW.iva_percent_snapshot := NULL;
    NEW.line_net := NULL;
    NEW.line_vat := NULL;
    NEW.line_gross := NULL;
    NEW.line_purchase := NULL;
    NEW.line_gain := NULL;
    RETURN NEW;
  END IF;

  -- Snapshot-first VAT resolution; fall back to the shared aenad VAT master.
  IF NEW.iva_percent_snapshot IS NULL AND NEW.cod_iva IS NOT NULL THEN
    NEW.iva_percent_snapshot := raenad.resolve_iva_percent(NEW.cod_iva);
  END IF;
  pct := NEW.iva_percent_snapshot;

  NEW.line_purchase := (NEW.purchase_unit_price * NEW.qta)::numeric(18,4);
  NEW.line_net := raenad.line_net(NEW.unit_price, NEW.qta, NEW.discounts);
  NEW.line_gain := NEW.line_net - NEW.line_purchase;

  IF NEW.line_net IS NULL THEN
    NEW.line_vat := NULL;
    NEW.line_gross := NULL;
  ELSIF pct IS NULL THEN
    RAISE EXCEPTION
      'Cannot calculate quote line VAT/gross: missing or unresolved VAT code %',
      COALESCE(NEW.cod_iva, '<NULL>')
      USING
        ERRCODE = '23514',
        DETAIL = 'Provide iva_percent_snapshot or a cod_iva present in aenad."TIva".';
  ELSE
    NEW.line_vat := (NEW.line_net * pct / 100)::numeric(18,4);
    NEW.line_gross := (NEW.line_net + NEW.line_vat)::numeric(18,4);
  END IF;

  RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION raenad.quote_recalculate_header(p_quote_id bigint)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  IF p_quote_id IS NULL THEN
    RETURN;
  END IF;

  UPDATE raenad.quote header
  SET
    total_net = totals.total_net,
    total_vat = totals.total_vat,
    total_gross = totals.total_gross,
    total_purchase = totals.total_purchase,
    total_gain = totals.total_gain
  FROM (
    SELECT
      COALESCE(SUM(line_net), 0)::numeric(18,4)      AS total_net,
      COALESCE(SUM(line_vat), 0)::numeric(18,4)      AS total_vat,
      COALESCE(SUM(line_gross), 0)::numeric(18,4)    AS total_gross,
      COALESCE(SUM(line_purchase), 0)::numeric(18,4) AS total_purchase,
      COALESCE(SUM(line_gain), 0)::numeric(18,4)     AS total_gain
    FROM raenad.quote_line
    WHERE quote_id = p_quote_id
  ) totals
  WHERE header.id = p_quote_id;
END;
$$;

CREATE OR REPLACE FUNCTION raenad.quote_line_recalculate_header()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF current_setting('raenad.skip_header_recalc', true) = 'on' THEN
    IF TG_OP = 'DELETE' THEN
      RETURN OLD;
    END IF;
    RETURN NEW;
  END IF;

  IF TG_OP = 'DELETE' THEN
    PERFORM raenad.quote_recalculate_header(OLD.quote_id);
    RETURN OLD;
  END IF;

  PERFORM raenad.quote_recalculate_header(NEW.quote_id);

  IF TG_OP = 'UPDATE' AND OLD.quote_id IS DISTINCT FROM NEW.quote_id THEN
    PERFORM raenad.quote_recalculate_header(OLD.quote_id);
  END IF;

  RETURN NEW;
END;
$$;

-- Generic updated_at touch.
CREATE OR REPLACE FUNCTION raenad.set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$;

-- PDF export content is immutable; only hubspot_* status fields may change.
CREATE OR REPLACE FUNCTION raenad.quote_pdf_export_guard_immutable()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.quote_id IS DISTINCT FROM OLD.quote_id
     OR NEW.revision IS DISTINCT FROM OLD.revision
     OR NEW.filename IS DISTINCT FROM OLD.filename
     OR NEW.content_type IS DISTINCT FROM OLD.content_type
     OR NEW.checksum_sha256 IS DISTINCT FROM OLD.checksum_sha256
     OR NEW.render_payload IS DISTINCT FROM OLD.render_payload
     OR NEW.created_at IS DISTINCT FROM OLD.created_at
     OR NEW.created_by IS DISTINCT FROM OLD.created_by
  THEN
    RAISE EXCEPTION 'raenad.quote_pdf_export revisions are immutable; only hubspot_* status fields may be updated'
      USING ERRCODE = '23514';
  END IF;

  RETURN NEW;
END;
$$;

-- Bulk recompute for backfills/data fixes (re-fires the line trigger, then headers).
CREATE OR REPLACE PROCEDURE raenad.recalculate_quote_totals(p_quote_id bigint DEFAULT NULL)
LANGUAGE plpgsql
AS $$
DECLARE
  rec record;
BEGIN
  PERFORM set_config('raenad.skip_header_recalc', 'on', true);

  UPDATE raenad.quote_line
  SET qta = qta
  WHERE p_quote_id IS NULL OR quote_id = p_quote_id;

  PERFORM set_config('raenad.skip_header_recalc', 'off', true);

  FOR rec IN
    SELECT id
    FROM raenad.quote
    WHERE p_quote_id IS NULL OR id = p_quote_id
  LOOP
    PERFORM raenad.quote_recalculate_header(rec.id);
  END LOOP;
END;
$$;

-- ---------------------------------------------------------------------------
-- Trigger wiring (drop + recreate so the script is re-runnable)
-- ---------------------------------------------------------------------------

DROP TRIGGER IF EXISTS quote_line_set_totals ON raenad.quote_line;
CREATE TRIGGER quote_line_set_totals
BEFORE INSERT OR UPDATE OF qta, unit_price, purchase_unit_price, discounts, cod_iva, iva_percent_snapshot, line_type
ON raenad.quote_line
FOR EACH ROW
EXECUTE FUNCTION raenad.quote_line_set_totals();

DROP TRIGGER IF EXISTS quote_line_recalculate_header ON raenad.quote_line;
CREATE TRIGGER quote_line_recalculate_header
AFTER INSERT OR DELETE OR UPDATE OF
  quote_id,
  qta,
  unit_price,
  purchase_unit_price,
  discounts,
  cod_iva,
  iva_percent_snapshot,
  line_type,
  line_net,
  line_vat,
  line_gross,
  line_purchase,
  line_gain
ON raenad.quote_line
FOR EACH ROW
EXECUTE FUNCTION raenad.quote_line_recalculate_header();

DROP TRIGGER IF EXISTS quote_set_updated_at ON raenad.quote;
CREATE TRIGGER quote_set_updated_at
BEFORE UPDATE ON raenad.quote
FOR EACH ROW
EXECUTE FUNCTION raenad.set_updated_at();

DROP TRIGGER IF EXISTS quote_pdf_export_guard_immutable ON raenad.quote_pdf_export;
CREATE TRIGGER quote_pdf_export_guard_immutable
BEFORE UPDATE ON raenad.quote_pdf_export
FOR EACH ROW
EXECUTE FUNCTION raenad.quote_pdf_export_guard_immutable();

COMMIT;
