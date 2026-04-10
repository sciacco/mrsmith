[QUOTES][MEDIUM] Align quote list retrieval behavior with Appsmith defaults

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_01`
- Verification target: Main quote-list retrieval for the list page

## 3. Observed Behavior (Migrated Implementation)

- The migrated app retrieves quotes through `/quotes/v1/quotes`.
- The request supports pagination, filters, and sort parameters.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith uses one fixed query for the main list view.
- The documented behavior includes explicit ordering by quote number descending and a hard limit of 2000 rows.

## 5. Difference

- Appsmith has fixed retrieval semantics.
- The migrated implementation exposes a parameterized request surface, and parity for the default ordering and row limit is not proven.

## 6. Impact

- Data correctness: the list may return a different set or order of quotes than Appsmith.
- User-facing behavior: users may see different default ordering or a different initial result size.
- System integrity: downstream list actions may operate on a different visible dataset than the source system.

## 7. Severity & Importance

- Severity: 3
- Importance: 5
- Priority: MEDIUM

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the main quote-list page and the code that builds requests to `/quotes/v1/quotes`.
- Search for the default request parameters for list loading, especially pagination, sorting, and empty-filter behavior.

2. Identify Faulty Logic
- Determine whether the migrated default list request produces the same default behavior as Appsmith.
- Specifically check whether the initial request preserves quote-number descending ordering and whether the effective row cap matches the Appsmith limit of 2000.

3. Reference Correct Behavior
- Replicate the Appsmith default behavior for the main list view: fixed list retrieval semantics with quote-number descending ordering and a 2000-row limit.
- If the migrated design intentionally keeps pagination and filters, the default unfiltered view must still resolve to the same result set and order as Appsmith.

4. Apply Changes
- Update the frontend defaults sent to `/quotes/v1/quotes` so that the initial list load matches Appsmith ordering.
- If the backend endpoint owns the default semantics, update the backend implementation instead of adding frontend-only assumptions.
- If both layers contribute, make the contract explicit so the frontend default state and the backend fallback behavior agree.

5. Validation Steps
- Input: open the list page with no filters and no explicit sort override.
- Expected output: the first page or returned result should reflect quote-number descending order and should not exceed the Appsmith-equivalent row cap behavior.
- Input: apply filters and sorting controls after the fix.
- Expected output: enhanced migrated controls continue to work without breaking the Appsmith-equivalent default state.

6. Edge Cases
- Preserve migrated pagination and filter features if they are intentional.
- Do not change filtered or user-sorted behavior when the issue is only in the default load path.

## 9. Acceptance Criteria

- The default quote-list load matches the Appsmith reference behavior.
- Sorting and row-limit behavior for the unfiltered list are aligned with Appsmith.
- Migrated filters and pagination still work.
- Verification for `out_01` would now result in `MATCH`.

## 10. Notes

- Confidence level: MEDIUM
- Part of the parity remains unverifiable from the current summary because backend query semantics behind `/quotes/v1/quotes` are not directly documented there.
