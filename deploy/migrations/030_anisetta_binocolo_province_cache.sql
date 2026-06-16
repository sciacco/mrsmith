-- Binocolo OpenAPI.it CAP province cache.
-- Target database: Anisetta PostgreSQL.

BEGIN;

CREATE SCHEMA IF NOT EXISTS binocolo;

CREATE TABLE IF NOT EXISTS binocolo.province_cache (
  cache_key       text PRIMARY KEY,
  response        jsonb NOT NULL,
  fetched_at      timestamptz NOT NULL,
  expires_at      timestamptz NOT NULL,
  last_served_at  timestamptz,
  served_count    bigint NOT NULL DEFAULT 0,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS province_cache_expires_at_idx
  ON binocolo.province_cache (expires_at);

CREATE INDEX IF NOT EXISTS province_cache_last_served_at_idx
  ON binocolo.province_cache (last_served_at);

DROP TRIGGER IF EXISTS province_cache_set_updated_at
  ON binocolo.province_cache;
CREATE TRIGGER province_cache_set_updated_at
BEFORE UPDATE ON binocolo.province_cache
FOR EACH ROW
EXECUTE FUNCTION binocolo.set_updated_at();

COMMIT;
