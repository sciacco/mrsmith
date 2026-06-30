-- Binocolo UC2 sector-classification: structural (threshold-free) reject gate.
--
-- Reverses migration 068's relative-margin reject. The deterministic reject is now
-- STRUCTURAL — no tunable margin: a company is terminally rejected (off-target, scarta,
-- no LLM) ONLY when the top-ranked KB concept is an off-target DISTRACTOR *and* nothing
-- in the strategy perimeter matched at all. A company with ANY in-perimeter signal is
-- escalated to the LLM (ambiguous) instead of being rejected, so the gate can no longer
-- leak a true keep.
--
-- Why: the relative margin (sector_reject_rel_margin = 0.04) was fit in-sample on one
-- session and did NOT generalize — on a second labeled session it terminally rejected
-- true keeps (keepLeak > 0). No single threshold on the reranker's relative softmax
-- generalizes across sessions; the structural rule has no number to overfit and rejected
-- 10/10 genuinely-scarta rows across both labeled sessions with zero keep loss. See
-- apps/binocolo/docs/UC2-SECTOR-MODEL-EVALUATION.md and ma_sector_replay.go.
--
-- The Go change (sectorVerdict in ma_sector_classification.go) no longer reads
-- sector_reject_rel_margin, so this parameter row is now dead config. Removing it keeps
-- ma_parameter honest. The code is unaffected whether the row is present or absent.
--
-- NOTE: existing persisted validations keep their old DeterministicVerdict until a
-- re-validation run ("Esegui validazione"); the structural gate only governs new runs.
--
-- Target database: Anisetta PostgreSQL (binocolo schema). Requires 063/068. Apply
-- manually on the database referenced by ANISETTA_DSN. Idempotent.

BEGIN;

DELETE FROM binocolo.ma_parameter WHERE key = 'sector_reject_rel_margin';

COMMIT;
