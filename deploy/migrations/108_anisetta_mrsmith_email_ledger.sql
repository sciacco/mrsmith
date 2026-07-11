-- 108: email ledger — global tracking of direct (user-triggered) email sends.
-- One row per send attempt, correlated to the originating app entity via
-- (app, entity_type, entity_id, purpose). status 'accepted' means the SMTP
-- relay (Postal) accepted the message, not that it was delivered; message_id
-- is kept for a possible future bounce reconciliation.
-- Target database: Anisetta PostgreSQL (schema mrsmith). Idempotent. Apply after 107.

BEGIN;

CREATE SCHEMA IF NOT EXISTS mrsmith;

CREATE TABLE IF NOT EXISTS mrsmith.email_send (
    id             bigserial PRIMARY KEY,
    app            text NOT NULL,
    entity_type    text NOT NULL,
    entity_id      text NOT NULL,
    purpose        text NOT NULL,
    actor_subject  text NOT NULL DEFAULT '',
    actor_email    text NOT NULL DEFAULT '',
    recipients_to  text[] NOT NULL,
    recipients_cc  text[] NOT NULL DEFAULT '{}',
    recipients_bcc text[] NOT NULL DEFAULT '{}',
    subject        text NOT NULL,
    message_id     text NOT NULL DEFAULT '',
    status         text NOT NULL CHECK (status IN ('accepted', 'failed')),
    error          text NOT NULL DEFAULT '',
    sent_at        timestamptz NOT NULL DEFAULT now(),
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS email_send_correlation_idx
    ON mrsmith.email_send (app, entity_type, entity_id, purpose, sent_at DESC);

COMMIT;
