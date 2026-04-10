[QUOTES][MEDIUM] Replace drag-and-drop swap writes with Appsmith-equivalent row position updates

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_19`
- Verification target: Quote-row position updates

## 3. Observed Behavior (Migrated Implementation)

- The migrated app updates row order through drag and drop.
- A drop operation sends two position updates, swapping the dragged row and the target row.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith updates the position of one edited row with one write.

## 5. Difference

- Appsmith uses single-row position updates.
- The migrated implementation changes both the trigger model and the write semantics by performing a two-row swap.

## 6. Impact

- Data correctness: row positions can diverge from Appsmith if swap semantics do not match intended ordering behavior.
- User-facing behavior: reorder outcomes differ from the source inline-edit model.
- System integrity: the migrated flow performs extra writes and may produce different stored position sequences.

## 7. Severity & Importance

- Severity: 3
- Importance: 3
- Priority: MEDIUM

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the row reorder logic on the quote detail page.
- Search for the mutation that updates row positions.
- Search for the drag-and-drop event handler that triggers two writes.

2. Identify Faulty Logic
- Confirm that a single reorder action currently issues two position updates and implements swap behavior instead of a single-row position update.

3. Reference Correct Behavior
- Replicate the Appsmith behavior: update the position of the intended row directly with one effective position assignment per edited row.

4. Apply Changes
- Refactor the reorder flow so a user action results in the Appsmith-equivalent position update semantics.
- If drag and drop remains the UI model, translate the drag result into the final target position for the moved row and persist positions in a way that matches Appsmith outcomes, not raw swap semantics.
- Avoid issuing unnecessary extra writes when one definitive position update is sufficient.

5. Validation Steps
- Input: move one row to a new position.
- Expected output: persisted positions after reload match the Appsmith-equivalent final order for that move.
- Input: repeat several reorder operations in sequence.
- Expected output: no duplicated swaps or inconsistent positions appear.

6. Edge Cases
- Handle adjacent and non-adjacent moves consistently.
- If multiple rows require renumbering to preserve a contiguous order, ensure the final stored positions are deterministic and match the intended moved-row result.

## 9. Acceptance Criteria

- Reordering rows no longer relies on raw two-row swap semantics.
- Persisted row positions match Appsmith-equivalent outcomes after reload.
- Verification for `out_19` would now result in `MATCH`.

## 10. Notes

- Confidence level: HIGH
- This is a factual change in write behavior, not only a UI difference.
