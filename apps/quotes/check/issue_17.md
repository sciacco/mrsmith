[QUOTES][MEDIUM] Ensure add-row behavior matches Appsmith quote-row creation semantics

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_17`
- Verification target: Adding a kit row to an existing quote

## 3. Observed Behavior (Migrated Implementation)

- The migrated app adds a row by posting `{ kit_id }` to `/quotes/v1/quotes/:id/rows`.
- The visible row list is refreshed after the mutation.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith inserts a quote row directly for the selected kit and refreshes the result.

## 5. Difference

- Appsmith writes directly from the frontend.
- The migrated implementation delegates insertion to a backend endpoint, so exact insert semantics are not proven from the current evidence.

## 6. Impact

- Data correctness: inserted rows may differ from the Appsmith-created shape if backend defaults diverge.
- User-facing behavior: row creation can appear successful while storing different derived values.
- System integrity: the add-row flow depends on opaque backend behavior rather than documented Appsmith insert semantics.

## 7. Severity & Importance

- Severity: 2
- Importance: 4
- Priority: MEDIUM

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the add-row mutation that posts to `/quotes/v1/quotes/:id/rows`.
- Search for the row refresh or query invalidation that runs after success.

2. Identify Faulty Logic
- Determine whether the backend endpoint creates a row with the same effective semantics Appsmith used for quote-row insertion.

3. Reference Correct Behavior
- Replicate the Appsmith behavior: selecting a kit creates the corresponding quote row for that quote and refreshes the visible row list.

4. Apply Changes
- If the backend endpoint is already the canonical insertion path, align its insert behavior with the Appsmith-equivalent row creation semantics.
- Keep the frontend request minimal if `{ kit_id }` is sufficient, but expand it only if additional explicit fields are required to match Appsmith behavior.
- Preserve query invalidation or refresh after successful insertion.

5. Validation Steps
- Input: add a kit row to an existing quote.
- Expected output: one new quote row is created for the selected kit and appears in the refreshed row list.
- Input: reload the quote detail page after the add.
- Expected output: the inserted row persists and retains the expected initial values.

6. Edge Cases
- Prevent duplicate submissions when the add action is triggered repeatedly before completion.
- If the same kit can be added multiple times by design, ensure each add still creates exactly one row per confirmed action.

## 9. Acceptance Criteria

- Adding a kit creates the correct quote row and refreshes the visible list.
- The created row matches Appsmith-equivalent insertion semantics.
- Verification for `out_17` would now result in `MATCH`.

## 10. Notes

- Confidence level: MEDIUM
- Exact backend insert semantics remain partially unverifiable from the current summary.
