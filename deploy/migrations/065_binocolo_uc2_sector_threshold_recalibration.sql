-- Binocolo UC2 sector-classification threshold recalibration.
--
-- After the reranker score was fixed to a true two-way softmax P(yes) (normalize:false
-- + softmax in llm.Client.Rerank), the score scale is genuine but LOW: the reranker is
-- a strong ranker but a conservative absolute scorer on the concept-membership task,
-- topping out ~0.3 for a strong match. The original thresholds (migration 063:
-- confirm 0.65, no_signal_floor 0.40) were set for a non-existent scale, so every
-- company escalated to the LLM and clear positives were mislabeled no_signal.
--
-- Recalibrated on two anchors (corrected scale, 2026-06-29):
--   NETX64 (IT system integrator, true positive): best in-perimeter concept 0.296
--   Coherency (IoT/software dev, off-perimeter):   best in-perimeter concept 0.142
-- confirm_prob 0.25 confirms NETX64 deterministically (no LLM) while leaving Coherency
-- to escalate; no_signal_floor 0.08 reserves no_signal for genuinely flat results.
-- Mirrors the Go defaults in ma_sector_classification.go.
--
-- Target database: Anisetta PostgreSQL (binocolo schema). Requires 063. Apply manually
-- on the database referenced by ANISETTA_DSN. Idempotent.

BEGIN;

UPDATE binocolo.ma_parameter SET value = '0.25' WHERE key = 'sector_confirm_prob';
UPDATE binocolo.ma_parameter SET value = '0.08' WHERE key = 'sector_no_signal_floor';

COMMIT;
