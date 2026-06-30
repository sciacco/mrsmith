-- Binocolo UC2 sector-classification: relative reject margin.
--
-- The reject gate moved from an ABSOLUTE threshold (sector_confirm_prob 0.25 + a 0.10
-- lead over the best target) to a RELATIVE, perimeter-aware margin: a company is
-- deterministically rejected (off-target, terminal scarta, no LLM) only when the best
-- off-target distractor outranks the best IN-PERIMETER target by this margin. The
-- reranker's softmax P(yes) is not a calibrated absolute probability for this corpus
-- (it tops out ~0.3 and shifts with document style), so the decisive signal is the
-- target-vs-distractor margin, not a fixed cutoff.
--
-- Validated offline against human labels (replay harness, session 391e7c67): at a
-- relative margin of 0.04 the deterministic reject reaches scarta-recall ~0.66 at ~0.96
-- precision with ZERO true-keep leakage, versus ~0.20 recall under the old absolute rule
-- (which dumped 28/35 true-scarta into forse). 0.02 was the in-sample optimum but tighter;
-- 0.04 trades a little recall for precision and headroom on unseen data.
--
-- Mirrors the Go default maSectorRejectRelMarginDefault in ma_sector_classification.go;
-- the code runs on the default even if this row is absent, so this seed only makes the
-- knob tunable in-DB like the other sector_* parameters.
--
-- Target database: Anisetta PostgreSQL (binocolo schema). Requires 063. Apply manually
-- on the database referenced by ANISETTA_DSN. Idempotent.

BEGIN;

INSERT INTO binocolo.ma_parameter (key, value, value_type, label, description) VALUES
  ('sector_reject_rel_margin', '0.04', 'number', 'Margine reject relativo settore', 'Distacco di probabilità rerank richiesto tra il miglior distrattore off-target e il miglior concetto in perimetro per il reject deterministico (scarta, senza LLM).')
ON CONFLICT (key) DO NOTHING;

COMMIT;
