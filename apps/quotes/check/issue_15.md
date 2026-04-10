[QUOTES][MEDIUM] Preserve Appsmith row ordering semantics when loading quote rows

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_15`
- Verification target: Quote-row list loading on the detail page

## 3. Observed Behavior (Migrated Implementation)

- The migrated app loads rows from `/quotes/v1/quotes/:id/rows`.
- The UI renders rows in the order returned by that endpoint.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith loads quote rows with explicit ordering by position.

## 5. Difference

- Appsmith defines row ordering explicitly.
- The migrated implementation does not prove that `/quotes/v1/quotes/:id/rows` returns rows in Appsmith-equivalent position order.

## 6. Impact

- Data correctness: row order can differ from the source system.
- User-facing behavior: kits can appear in a different sequence on the detail page.
- System integrity: reorder operations and subsequent edits may act on a different visible order than expected.

## 7. Severity & Importance

- Severity: 2
- Importance: 4
- Priority: MEDIUM

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the row-loading request to `/quotes/v1/quotes/:id/rows`.
- Search for the detail-page component that maps returned rows into the UI.

2. Identify Faulty Logic
- Determine whether the backend guarantees position ordering.
- If not, confirm that the frontend currently renders rows without applying an explicit sort by position.

3. Reference Correct Behavior
- Replicate the Appsmith row-loading behavior: rows must be displayed in position order.

4. Apply Changes
- Prefer enforcing position ordering in the backend response for `/quotes/v1/quotes/:id/rows`.
- If the backend contract cannot be changed immediately, add a frontend sort by position before rendering.
- Keep the existing row structure and UI composition unchanged apart from ordering.

5. Validation Steps
- Input: open a quote with multiple rows in nontrivial positions.
- Expected output: rows render in ascending Appsmith-equivalent position order.
- Input: reload the detail page after a reorder.
- Expected output: the displayed row order remains stable and position-based.

6. Edge Cases
- Handle rows with missing or duplicate positions deterministically, for example by preserving stable secondary order.
- Do not mutate stored positions during display-only sorting.

## 9. Acceptance Criteria

- Quote rows are rendered in Appsmith-equivalent position order.
- The order remains stable across reloads.
- Verification for `out_15` would now result in `MATCH`.

## 10. Notes

- Confidence level: MEDIUM
- Backend ordering semantics are not fully verifiable from the current summary.
