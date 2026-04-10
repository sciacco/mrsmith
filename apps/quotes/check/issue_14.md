[QUOTES][MEDIUM] Reintroduce Appsmith publish prechecks and orchestration guarantees

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_14`
- Verification target: Publish orchestration and client-side prechecks

## 3. Observed Behavior (Migrated Implementation)

- The migrated detail page blocks publish only when the quote is dirty.
- Publish is then delegated to a single `/publish` request and the UI displays returned step statuses.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith saves before publish.
- Appsmith checks HubSpot id state, signed-quote state, and required products.
- Appsmith performs a larger publish-side orchestration with multiple steps and side effects.

## 5. Difference

- Appsmith includes explicit prechecks and orchestration in the observable client flow.
- The migrated implementation delegates almost all publish logic to a single backend call and exposes fewer client-side guarantees.

## 6. Impact

- Data correctness: publish can run without the same validated preconditions as Appsmith.
- User-facing behavior: users receive weaker pre-publish feedback and may hit backend failures later.
- System integrity: publish behavior depends on undocumented backend orchestration rather than observable Appsmith-equivalent safeguards.

## 7. Severity & Importance

- Severity: 3
- Importance: 5
- Priority: MEDIUM

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the detail-page publish action and the `/publish` request path.
- Search for dirty-state gating and any existing publish prechecks.

2. Identify Faulty Logic
- Confirm that the migrated flow blocks publish only on unsaved changes and does not enforce the Appsmith-equivalent prechecks before calling publish.

3. Reference Correct Behavior
- Replicate the Appsmith publish guarantees: save first, verify HubSpot linkage and signature state, validate required products, then run publish.
- If the migrated architecture centralizes orchestration in the backend, the frontend must still enforce or surface the same preconditions before reporting success.

4. Apply Changes
- Add explicit pre-publish checks for the Appsmith-documented conditions before the publish request is allowed.
- If those checks already exist in the backend, expose their results clearly in the frontend and block publish until they pass.
- Ensure the save-before-publish behavior remains guaranteed.
- Preserve step-status rendering if it accurately reflects the Appsmith-equivalent workflow.

5. Validation Steps
- Input: attempt publish with unsaved changes.
- Expected output: publish remains blocked until save succeeds.
- Input: attempt publish when HubSpot linkage, signature state, or required-product conditions fail.
- Expected output: publish is blocked with a clear reason before final success is reported.
- Input: attempt publish when all conditions pass.
- Expected output: publish succeeds and the step-state output remains coherent.

6. Edge Cases
- Prevent duplicate publish requests while checks are in progress.
- If the backend is authoritative for some checks, do not show contradictory frontend pass states; surface the backend failure result directly.

## 9. Acceptance Criteria

- Publish enforces the same preconditions documented in Appsmith.
- Save-before-publish is guaranteed.
- Users receive explicit feedback when a publish precondition fails.
- Verification for `out_14` would now result in `MATCH`.

## 10. Notes

- Confidence level: MEDIUM
- Some final parity remains dependent on backend publish logic, which is not fully observable from the current summary.
