-- Binocolo UC2: register the analyst concept-grounding knob, defaulted OFF.
--
-- The LLM analyst no longer receives the embed+rerank concept matches by default. The 10x
-- snippet-faithful ablation (apps/binocolo/docs/UC2-SECTOR-MODEL-EVALUATION.md §7.1) found
-- the concept-match grounding net-harmful as analyst input — it biases toward keeping (more
-- false-keeps and more keep-leaks) and decisively hurts gpt-oss (accuracy -0.098,
-- scartaRecall -0.150, t≈-12). The analyst still gets the distilled description, the
-- strategy perimeter, and the web snippets.
--
-- The Go default (maSectorAnalystGroundingDefault) is already false, so the code drops the
-- grounding even if this row is absent. This row only makes the knob visible/tunable in-DB:
-- set value '1' to restore the old grounded payload.
--
-- Target database: Anisetta PostgreSQL (binocolo schema). Requires 063. Apply manually on
-- the database referenced by ANISETTA_DSN. Idempotent.

BEGIN;

INSERT INTO binocolo.ma_parameter (key, value, value_type, label, description) VALUES
  ('sector_analyst_concept_grounding', '0', 'number', 'Grounding concept-match per analista LLM', 'Se >0 l''analista riceve i concept match embed+rerank; default 0 (off). L''ablation 10x ha mostrato il grounding netto-dannoso (UC2-SECTOR-MODEL-EVALUATION.md §7.1).')
ON CONFLICT (key) DO NOTHING;

COMMIT;
