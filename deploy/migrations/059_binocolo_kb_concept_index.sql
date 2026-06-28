-- Binocolo curated KB tables: business concepts + concept->ATECO fit mapping +
-- ATECO ICT supplement nodes. These back the embedding-based ATECO retrieval
-- (use case 1) and the web sector classification (use case 2).
--
-- Storage is PLAIN Postgres (no Qdrant, no pgvector): the corpus is tiny
-- (~72 vectors) so the runtime loads vectors into memory and does brute-force
-- cosine in Go. Vectors live in the `embedding real[]` column.
--
-- DDL only. Curated rows + vectors are populated by the `atego` loader
-- (reads apps/binocolo/docs/business-maps/*.json, embeds embedding_text via
-- Fireworks, UPSERTs rows + fills `embedding`). embedding/source_text_hash/
-- embedding_model_id stay NULL until the loader runs.
--
-- Target database: Anisetta PostgreSQL (binocolo schema; FK into mrsmith schema,
-- same database). Requires migration 058 first. Apply manually on ANISETTA_DSN.
-- Idempotent.

BEGIN;

-- Business concepts (incl. off-target "distractor" concepts for UC2).
CREATE TABLE IF NOT EXISTS binocolo.kb_concept (
  id                 text PRIMARY KEY,                       -- slug, e.g. 'cloud_infrastructure'
  name               text NOT NULL,
  domain             text,
  kind               text NOT NULL DEFAULT 'target'
                       CHECK (kind IN ('target','distractor')),
  aliases            text[] NOT NULL DEFAULT '{}',
  embedding_text     text NOT NULL,                          -- the text the loader embeds (raw, no instruction)
  embedding          real[],                                 -- 4096-dim vector; NULL until loader runs
  source_text_hash   text,                                   -- sha256(embedding_text); loader idempotency
  embedding_model_id uuid REFERENCES mrsmith.embedding_model(id) ON DELETE RESTRICT,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now()
);

-- Concept -> ATECO fit mapping. relation is in_kb|excluded only; the core/weak
-- split is intent-relative (decided at query time from the matched concept's
-- cosine), NOT a property of the edge. ateco_code has NO FK so it tolerates
-- excluded prefixes that are not catalog leaves (e.g. '6310.2').
CREATE TABLE IF NOT EXISTS binocolo.kb_concept_ateco (
  concept_id text NOT NULL REFERENCES binocolo.kb_concept(id) ON DELETE CASCADE,
  ateco_code text NOT NULL,
  relation   text NOT NULL CHECK (relation IN ('in_kb','excluded')),
  position   integer,                                        -- order within the source list (future use)
  PRIMARY KEY (concept_id, ateco_code, relation)
);

CREATE INDEX IF NOT EXISTS kb_concept_ateco_code_idx
  ON binocolo.kb_concept_ateco (ateco_code);

-- ATECO ICT supplement: the embeddable text + ICT metadata for the 37 ICT nodes
-- (codice references the full catalog binocolo.codici_ateco_2025). Recall is OFF
-- in v1, so the runtime does not query these yet; present for consistency and as
-- the recall substrate.
CREATE TABLE IF NOT EXISTS binocolo.kb_ateco_node (
  codice             text PRIMARY KEY,
  relevance_tier     text,                                   -- 'core' | 'extended'
  keywords           text[] NOT NULL DEFAULT '{}',
  seed_concepts      text[] NOT NULL DEFAULT '{}',
  path_text          text,
  embedding_text     text NOT NULL,
  embedding          real[],
  source_text_hash   text,
  embedding_model_id uuid REFERENCES mrsmith.embedding_model(id) ON DELETE RESTRICT,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now()
);

COMMIT;
