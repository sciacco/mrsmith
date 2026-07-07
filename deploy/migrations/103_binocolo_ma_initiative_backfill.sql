-- 103_binocolo_ma_initiative_backfill.sql
-- Backfills one TGT initiative for each non-purged orphan ma_session and anchors it.
-- Prerequisite: apply manually only after purging the session trash from the UI.
-- Verification query before/after migration:
-- SELECT count(*) FROM binocolo.ma_session WHERE initiative_id IS NULL AND purged_at IS NULL;
-- Idempotent: once qualifying sessions have initiative_id set, reruns are no-ops.

WITH orphan_sessions AS MATERIALIZED (
  SELECT
    s.id AS session_id,
    gen_random_uuid() AS initiative_id,
    s.title,
    s.created_by_subject,
    s.created_by_email,
    s.created_at,
    s.updated_at,
    s.archived_at,
    s.archived_by_subject,
    s.archived_by_email,
    s.deleted_at,
    s.deleted_by_subject,
    s.deleted_by_email
  FROM binocolo.ma_session s
  WHERE s.initiative_id IS NULL
    AND s.purged_at IS NULL
), inserted_initiatives AS (
  INSERT INTO binocolo.ma_initiative (
    id,
    title,
    description,
    created_by_subject,
    created_by_email,
    created_at,
    updated_at,
    archived_at,
    archived_by_subject,
    archived_by_email,
    deleted_at,
    deleted_by_subject,
    deleted_by_email
  )
  SELECT
    os.initiative_id,
    'TGT: ' || left(os.title, 115),
    '',
    COALESCE(os.created_by_subject, ''),
    COALESCE(os.created_by_email, ''),
    os.created_at,
    os.updated_at,
    os.archived_at,
    os.archived_by_subject,
    os.archived_by_email,
    os.deleted_at,
    os.deleted_by_subject,
    os.deleted_by_email
  FROM orphan_sessions os
  RETURNING id
)
UPDATE binocolo.ma_session s
SET initiative_id = os.initiative_id
FROM orphan_sessions os
JOIN inserted_initiatives ii ON ii.id = os.initiative_id
WHERE s.id = os.session_id
  AND s.initiative_id IS NULL
  AND s.purged_at IS NULL;
