-- Binocolo OpenAPI.it company search cache.
-- Target database: Anisetta PostgreSQL.
-- Apply manually on the database referenced by ANISETTA_DSN before deploying
-- backend code that enables cached /binocolo/v1/companies/search calls.

BEGIN;

CREATE SCHEMA IF NOT EXISTS binocolo;

CREATE TABLE IF NOT EXISTS binocolo.company_search_cache (
  id                         bigserial PRIMARY KEY,
  cache_key                  text NOT NULL,
  params                     jsonb NOT NULL DEFAULT '{}'::jsonb,
  response                   jsonb NOT NULL,
  dry_run                    boolean NOT NULL,
  data_enrichment            text NOT NULL DEFAULT '',
  fetched_at                 timestamptz NOT NULL,
  expires_at                 timestamptz NOT NULL,
  last_served_at             timestamptz,
  served_count               bigint NOT NULL DEFAULT 0,
  created_at                 timestamptz NOT NULL DEFAULT now(),
  updated_at                 timestamptz NOT NULL DEFAULT now(),
  last_refreshed_by_subject  text,
  last_refreshed_by_email    text,

  CONSTRAINT company_search_cache_key_unique UNIQUE (cache_key)
);

CREATE INDEX IF NOT EXISTS company_search_cache_expires_at_idx
  ON binocolo.company_search_cache (expires_at);

CREATE INDEX IF NOT EXISTS company_search_cache_last_served_at_idx
  ON binocolo.company_search_cache (last_served_at);

CREATE INDEX IF NOT EXISTS company_search_cache_debug_idx
  ON binocolo.company_search_cache (data_enrichment, dry_run);

CREATE OR REPLACE FUNCTION binocolo.company_search_cache_set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS company_search_cache_set_updated_at
  ON binocolo.company_search_cache;
CREATE TRIGGER company_search_cache_set_updated_at
BEFORE UPDATE ON binocolo.company_search_cache
FOR EACH ROW
EXECUTE FUNCTION binocolo.company_search_cache_set_updated_at();

COMMIT;
