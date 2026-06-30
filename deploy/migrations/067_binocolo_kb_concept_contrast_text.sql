-- Add binocolo.kb_concept.contrast_text: the curated DISCRIMINATIVE text of a
-- concept (sibling_contrast_notes in the source JSON) — the distinctive
-- vocabulary + "vs sibling" boundaries that separate confusable concepts
-- (cloud vs hosting, managed_services vs system_integration, software vs
-- web_agency, ...).
--
-- WHY: today the UC2 reranker scores (company_description, embedding_text) — the
-- concept's POSITIVE self-description only — so it cannot separate siblings that
-- share vocabulary. The discriminative material exists and is curated, but the
-- loader drops it. This column surfaces it so:
--   - the LLM analyst can read the discriminators (handles negation), and
--   - the reranker document can be enriched (positive distinctive vocabulary).
--
-- NOT embedded: contrast_text feeds the cross-encoder document / LLM payload, not
-- the cosine-recall vector. embedding_text stays the sole recall substrate, so
-- source_text_hash (= sha256(embedding_text)) is unchanged and curating
-- contrast_text alone does NOT force a re-embed. The atego loader writes this
-- column on the metadata UPSERT path (every sync), independent of the embed gate.
--
-- Target database: Anisetta PostgreSQL (binocolo schema). Requires migration 059.
-- Apply manually on ANISETTA_DSN. Idempotent.

BEGIN;

ALTER TABLE binocolo.kb_concept
  ADD COLUMN IF NOT EXISTS contrast_text text;

COMMIT;
