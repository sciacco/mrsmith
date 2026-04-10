[QUOTES][MEDIUM] Align grouped product loading with Appsmith response semantics

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_20`
- Verification target: Grouped product loading for a selected quote row

## 3. Observed Behavior (Migrated Implementation)

- The migrated app loads products from `/quotes/v1/quotes/:quoteId/rows/:rowId/products`.
- The frontend groups a flat product list by `group_name`.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith returns grouped product data from the source query.
- The documented source behavior includes grouped rows plus included-item helper fields.

## 5. Difference

- Appsmith performs grouping and helper extraction in the data source.
- The migrated implementation moves grouping to the frontend and does not prove that the returned response includes Appsmith-equivalent helper semantics.

## 6. Impact

- Data correctness: grouping and included-item behavior can diverge from the source system.
- User-facing behavior: product groups may render differently or lose helper-derived state.
- System integrity: product-selection logic can depend on a response shape that no longer encodes the Appsmith group semantics directly.

## 7. Severity & Importance

- Severity: 2
- Importance: 4
- Priority: MEDIUM

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the request to `/quotes/v1/quotes/:quoteId/rows/:rowId/products`.
- Search for the frontend grouping logic keyed by `group_name`.

2. Identify Faulty Logic
- Determine whether the flat response omits helper fields that Appsmith relied on for grouped rendering or included-item handling.
- Confirm whether the frontend grouping produces the same observable groups as Appsmith.

3. Reference Correct Behavior
- Replicate the Appsmith product-loading semantics: grouped product data must expose the same effective grouping and included-item behavior seen in the source implementation.

4. Apply Changes
- Prefer moving grouping and helper derivation into the backend response if the migrated contract should mirror Appsmith more closely.
- If the flat response is retained, extend it so the frontend can reconstruct the exact Appsmith-equivalent grouped state deterministically.
- Update the grouping logic only as needed to restore Appsmith-visible behavior.

5. Validation Steps
- Input: open a quote row with multiple product groups and included-item relationships.
- Expected output: the grouped UI matches the Appsmith grouping and included-item behavior.
- Input: reload the same row after the change.
- Expected output: grouping remains stable and deterministic.

6. Edge Cases
- Handle products with empty or duplicate group names consistently.
- Preserve rendering for rows that contain only a single group or a single product.

## 9. Acceptance Criteria

- Product groups render with Appsmith-equivalent grouping semantics.
- Included-item helper behavior is preserved where required.
- Verification for `out_20` would now result in `MATCH`.

## 10. Notes

- Confidence level: MEDIUM
- The summary does not fully document backend response shape, so helper-field parity must be validated explicitly.
