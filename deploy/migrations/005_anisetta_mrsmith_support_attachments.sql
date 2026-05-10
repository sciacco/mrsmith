-- Persist user-uploaded files attached to contextual support requests.

BEGIN;

CREATE TABLE IF NOT EXISTS mrsmith.support_request_attachment (
  id bigserial PRIMARY KEY,
  request_id bigint NOT NULL
    REFERENCES mrsmith.support_request(id) ON DELETE CASCADE,
  ordinal integer NOT NULL,
  filename text NOT NULL,
  content_type text NOT NULL,
  size_bytes bigint NOT NULL,
  content_sha256 text NOT NULL,
  content bytea NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT support_request_attachment_size_check
    CHECK (size_bytes > 0),
  CONSTRAINT support_request_attachment_hash_check
    CHECK (length(content_sha256) = 64),
  CONSTRAINT support_request_attachment_ordinal_check
    CHECK (ordinal > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS support_request_attachment_request_ordinal_idx
  ON mrsmith.support_request_attachment (request_id, ordinal);

CREATE INDEX IF NOT EXISTS support_request_attachment_request_idx
  ON mrsmith.support_request_attachment (request_id, created_at);

COMMIT;
