[QUOTES][MEDIUM] Preserve Appsmith deal-list filtering in standard quote creation

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_03`
- Verification target: Deal-list loading for standard quote creation

## 3. Observed Behavior (Migrated Implementation)

- The migrated app loads deals from `/quotes/v1/deals`.
- It applies only client-side text filtering on deal name and company name after retrieval.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith uses explicit deal-selection constraints in the source query.
- The documented behavior includes pipeline and stage filters plus exclusion of empty `codice` values.

## 5. Difference

- Appsmith constrains the dataset at query time.
- The migrated implementation only text-filters the returned dataset, so parity for the underlying deal eligibility rules is not proven.

## 6. Impact

- Data correctness: ineligible deals may be shown or eligible deals may be handled differently.
- User-facing behavior: quote creation may start from a different deal population than Appsmith.
- System integrity: downstream quote creation may attach to deals that would not have been selectable in the source implementation.

## 7. Severity & Importance

- Severity: 3
- Importance: 5
- Priority: MEDIUM

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the standard quote creation flow and the code that requests `/quotes/v1/deals`.
- Search for the local text-filter logic applied to the returned deal list.

2. Identify Faulty Logic
- Verify that the migrated implementation relies only on post-fetch text filtering and does not enforce the Appsmith eligibility rules for pipeline, stage, and non-empty code.

3. Reference Correct Behavior
- Replicate the Appsmith deal eligibility behavior before the user selects a deal.
- The allowed deal set must honor the documented pipeline/stage constraints and the exclusion of empty-code deals.

4. Apply Changes
- Prefer implementing the Appsmith eligibility rules in the backend endpoint that serves `/quotes/v1/deals`.
- If backend changes are not available, add a temporary frontend guard that excludes deals failing the Appsmith conditions, but only if the necessary fields are already present in the response.
- Keep the existing text-filter search as an additional narrowing layer, not as the primary eligibility filter.

5. Validation Steps
- Input: open standard quote creation with the unfiltered deal picker.
- Expected output: only deals satisfying the Appsmith constraints are selectable.
- Input: search by deal name or company name.
- Expected output: search narrows the already-eligible set and does not reintroduce excluded deals.

6. Edge Cases
- Preserve behavior when the endpoint returns a small or empty set.
- If the current response does not include the fields needed for frontend filtering, do not infer them; fix the backend contract instead.

## 9. Acceptance Criteria

- The selectable deal list for standard quote creation matches Appsmith eligibility rules.
- Local text search still works on the resulting allowed set.
- Verification for `out_03` would now result in `MATCH`.

## 10. Notes

- Confidence level: MEDIUM
- Backend parity for `/quotes/v1/deals` is not fully verifiable from the current summary, so validation must confirm the final contract explicitly.
