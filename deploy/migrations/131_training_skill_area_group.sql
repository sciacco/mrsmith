-- Training: link a skill area to the local group that gathers its members
-- (#140, follows #136 as amended on 2026-08-25; operational plan: #137).
--
-- A training rule with population kind 'skill_area' resolves its population
-- through this link: the members of the linked local group. An area without
-- a linked group cannot be used as a rule population and is rejected by the
-- backend with an explicit message. Additive and nullable; the link is
-- managed by the existing skill-area upsert.
--
-- Delivered as a file only. The Agent does NOT apply it and does NOT connect
-- to any database configured in the env files.

BEGIN;

ALTER TABLE training.skill_area
  ADD COLUMN IF NOT EXISTS custom_group_id uuid REFERENCES training.custom_groups(id);

COMMENT ON COLUMN training.skill_area.custom_group_id IS
  'Gruppo locale che raccoglie gli appartenenti all''area: platea delle regole con kind skill_area.';

COMMIT;
