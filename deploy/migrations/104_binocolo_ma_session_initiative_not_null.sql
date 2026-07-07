-- 104_binocolo_ma_session_initiative_not_null.sql
-- Final invariant closure for binocolo.ma_session initiative ownership.
-- Prerequisites: apply only after migrations 102 and 103, after the F1 frontend has
-- been deployed, and after an operator has verified there are no live orphan
-- sessions left. This migration intentionally does not update or delete data.
-- Verification query for live orphans (must be zero before applying/validating):
-- SELECT count(*) FROM binocolo.ma_session WHERE initiative_id IS NULL AND purged_at IS NULL;
-- Verification query for historical purged orphan tombstones (may be non-zero):
-- SELECT count(*) FROM binocolo.ma_session WHERE initiative_id IS NULL AND purged_at IS NOT NULL;
-- Robust option (b): enforce the product invariant only for non-purged sessions,
-- allowing historical purged orphan tombstones to remain auditable.

DO $$
DECLARE
  existing_expr text;
  normalized_existing_expr text;
  normalized_expected_expr text := 'CHECKinitiative_idISNOTNULLORpurged_atISNOTNULL';
BEGIN
  SELECT pg_get_constraintdef(c.oid, true)
    INTO existing_expr
  FROM pg_constraint c
  WHERE c.conrelid = 'binocolo.ma_session'::regclass
    AND c.conname = 'ma_session_initiative_required_unless_purged_check';

  IF existing_expr IS NULL THEN
    ALTER TABLE binocolo.ma_session
      ADD CONSTRAINT ma_session_initiative_required_unless_purged_check
      CHECK (initiative_id IS NOT NULL OR purged_at IS NOT NULL)
      NOT VALID;
  ELSE
    normalized_existing_expr := translate(existing_expr, ' ()' || chr(9) || chr(10) || chr(13), '');

    IF normalized_existing_expr <> normalized_expected_expr THEN
      RAISE EXCEPTION
        'Constraint ma_session_initiative_required_unless_purged_check already exists on binocolo.ma_session with unexpected definition: %',
        existing_expr;
    END IF;
  END IF;
END;
$$;

ALTER TABLE binocolo.ma_session
  VALIDATE CONSTRAINT ma_session_initiative_required_unless_purged_check;
