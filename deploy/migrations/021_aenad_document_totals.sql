-- Aenad document row and header totals.
-- Target database: Mistra PostgreSQL.
-- Apply manually on the database referenced by MISTRA_DSN.

BEGIN;

CREATE OR REPLACE FUNCTION aenad.tdoc_discount_multiplier(discounts text)
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
      RAISE EXCEPTION 'Invalid Aenad discount value: %', discounts
        USING ERRCODE = '22023';
    END IF;

    percent := part::numeric;
    multiplier := multiplier * (1 - (percent / 100));
  END LOOP;

  RETURN multiplier;
END;
$$;

CREATE OR REPLACE FUNCTION aenad.tdoc_net_amount(
  price_net bigint,
  quantity bigint,
  discounts text
)
RETURNS bigint
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
  discount_multiplier numeric;
BEGIN
  discount_multiplier := aenad.tdoc_discount_multiplier(discounts);

  IF price_net IS NULL OR quantity IS NULL THEN
    RETURN NULL;
  END IF;

  RETURN round(price_net::numeric * quantity::numeric * discount_multiplier)::bigint;
END;
$$;

CREATE OR REPLACE FUNCTION aenad.tdoc_gross_amount(
  net_amount bigint,
  vat_code character varying
)
RETURNS bigint
LANGUAGE plpgsql
STABLE
AS $$
DECLARE
  vat_percent numeric;
BEGIN
  IF net_amount IS NULL OR vat_code IS NULL THEN
    RETURN NULL;
  END IF;

  SELECT "PercIva"::numeric
  INTO vat_percent
  FROM aenad."TIva"
  WHERE "CodIva" = vat_code;

  IF vat_percent IS NULL THEN
    RETURN NULL;
  END IF;

  RETURN round(net_amount::numeric * (1 + (vat_percent / 100)))::bigint;
END;
$$;

CREATE OR REPLACE FUNCTION aenad.tdoc_righe_set_totals()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW."ImportoAcquistoRiga" := NEW."PrezzoAcquisto" * NEW."Qta";
  NEW."ImportoNettoRiga" := aenad.tdoc_net_amount(NEW."PrezzoNetto", NEW."Qta", NEW."Sconti");
  NEW."GuadagnoRiga" := NEW."ImportoNettoRiga" - NEW."ImportoAcquistoRiga";
  NEW."ImportoIvatoRiga" := aenad.tdoc_gross_amount(NEW."ImportoNettoRiga", NEW."CodIva");

  RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION aenad.tdoc_recalculate_header(document_id integer)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
  IF document_id IS NULL THEN
    RETURN;
  END IF;

  UPDATE aenad."TDocTestate" header
  SET
    "TotNetto" = totals.tot_netto,
    "TotIva" = totals.tot_iva,
    "TotDoc" = totals.tot_doc,
    "TotPrezzoAcquisto" = totals.tot_prezzo_acquisto,
    "TotGuadagno" = totals.tot_guadagno
  FROM (
    SELECT
      SUM("ImportoNettoRiga")::bigint AS tot_netto,
      SUM("ImportoIvatoRiga" - "ImportoNettoRiga")::bigint AS tot_iva,
      SUM("ImportoIvatoRiga")::bigint AS tot_doc,
      SUM("ImportoAcquistoRiga")::bigint AS tot_prezzo_acquisto,
      SUM("GuadagnoRiga")::bigint AS tot_guadagno
    FROM aenad."TDocRighe"
    WHERE "IDDoc" = document_id
  ) totals
  WHERE header."IDDoc" = document_id;
END;
$$;

CREATE OR REPLACE FUNCTION aenad.tdoc_righe_recalculate_header()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF current_setting('aenad.skip_header_recalc', true) = 'on' THEN
    IF TG_OP = 'DELETE' THEN
      RETURN OLD;
    END IF;
    RETURN NEW;
  END IF;

  IF TG_OP = 'DELETE' THEN
    PERFORM aenad.tdoc_recalculate_header(OLD."IDDoc");
    RETURN OLD;
  END IF;

  PERFORM aenad.tdoc_recalculate_header(NEW."IDDoc");

  IF TG_OP = 'UPDATE' AND OLD."IDDoc" IS DISTINCT FROM NEW."IDDoc" THEN
    PERFORM aenad.tdoc_recalculate_header(OLD."IDDoc");
  END IF;

  RETURN NEW;
END;
$$;

CREATE OR REPLACE PROCEDURE aenad.recalculate_tdoc_totals(p_id_doc integer DEFAULT NULL)
LANGUAGE plpgsql
AS $$
DECLARE
  rec record;
BEGIN
  PERFORM set_config('aenad.skip_header_recalc', 'on', true);

  WITH calculated AS (
    SELECT
      row_data."IDDocRiga",
      row_data."PrezzoAcquisto" * row_data."Qta" AS importo_acquisto_riga,
      net.importo_netto_riga,
      net.importo_netto_riga - (row_data."PrezzoAcquisto" * row_data."Qta") AS guadagno_riga,
      aenad.tdoc_gross_amount(net.importo_netto_riga, row_data."CodIva") AS importo_ivato_riga
    FROM aenad."TDocRighe" row_data
    CROSS JOIN LATERAL (
      SELECT aenad.tdoc_net_amount(row_data."PrezzoNetto", row_data."Qta", row_data."Sconti") AS importo_netto_riga
    ) net
    WHERE p_id_doc IS NULL OR row_data."IDDoc" = p_id_doc
  )
  UPDATE aenad."TDocRighe" row_data
  SET
    "ImportoAcquistoRiga" = calculated.importo_acquisto_riga,
    "ImportoNettoRiga" = calculated.importo_netto_riga,
    "GuadagnoRiga" = calculated.guadagno_riga,
    "ImportoIvatoRiga" = calculated.importo_ivato_riga
  FROM calculated
  WHERE row_data."IDDocRiga" = calculated."IDDocRiga"
    AND (
      row_data."ImportoAcquistoRiga" IS DISTINCT FROM calculated.importo_acquisto_riga
      OR row_data."ImportoNettoRiga" IS DISTINCT FROM calculated.importo_netto_riga
      OR row_data."GuadagnoRiga" IS DISTINCT FROM calculated.guadagno_riga
      OR row_data."ImportoIvatoRiga" IS DISTINCT FROM calculated.importo_ivato_riga
    );

  PERFORM set_config('aenad.skip_header_recalc', 'off', true);

  FOR rec IN
    SELECT "IDDoc"
    FROM aenad."TDocTestate"
    WHERE p_id_doc IS NULL OR "IDDoc" = p_id_doc
  LOOP
    PERFORM aenad.tdoc_recalculate_header(rec."IDDoc");
  END LOOP;
END;
$$;

DROP TRIGGER IF EXISTS tdoc_righe_set_totals ON aenad."TDocRighe";
CREATE TRIGGER tdoc_righe_set_totals
BEFORE INSERT OR UPDATE OF "Qta", "PrezzoNetto", "PrezzoAcquisto", "Sconti", "CodIva"
ON aenad."TDocRighe"
FOR EACH ROW
EXECUTE FUNCTION aenad.tdoc_righe_set_totals();

DROP TRIGGER IF EXISTS tdoc_righe_recalculate_header ON aenad."TDocRighe";
CREATE TRIGGER tdoc_righe_recalculate_header
AFTER INSERT OR DELETE OR UPDATE OF
  "IDDoc",
  "Qta",
  "PrezzoNetto",
  "PrezzoAcquisto",
  "Sconti",
  "CodIva",
  "ImportoAcquistoRiga",
  "ImportoNettoRiga",
  "ImportoIvatoRiga",
  "GuadagnoRiga"
ON aenad."TDocRighe"
FOR EACH ROW
EXECUTE FUNCTION aenad.tdoc_righe_recalculate_header();

COMMIT;
