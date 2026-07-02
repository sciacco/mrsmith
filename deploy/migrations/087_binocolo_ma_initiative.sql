-- 087: Iniziativa entity (workstream D3, INIZIATIVE-PRD.md §3) and its
-- optional anchor on ma_session.
--
-- An Iniziativa is a light container (title, optional description, author,
-- archivable) — no budget/KPI/deadline (non-CRM constraint, PRD §10). A
-- session can be anchored to at most one initiative; the anchor is nullable
-- so exploratory sessions stay zero-friction (PRD §3.2). Nothing reads this
-- yet beyond the anchor itself — the card/log evolution comes in mig 088/089.
--
-- Idempotent. Target: Anisetta (schema binocolo). Applied manually by the user.

CREATE TABLE IF NOT EXISTS binocolo.ma_initiative (
    id uuid PRIMARY KEY,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_by_subject text NOT NULL DEFAULT '',
    created_by_email text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    archived_by_subject text,
    archived_by_email text
);

ALTER TABLE binocolo.ma_session
    ADD COLUMN IF NOT EXISTS initiative_id uuid REFERENCES binocolo.ma_initiative(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS ma_session_initiative_id_idx
    ON binocolo.ma_session (initiative_id)
    WHERE initiative_id IS NOT NULL;
