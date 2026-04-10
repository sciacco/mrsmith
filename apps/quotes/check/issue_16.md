[QUOTES][MEDIUM] Align kit picker source filtering with Appsmith eligibility rules

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_16`
- Verification target: Kit-picker source loading

## 3. Observed Behavior (Migrated Implementation)

- The migrated app loads kits from `/quotes/v1/kits`.
- The picker expects richer kit presentation data, including prices and category metadata.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith loads kits from source queries that exclude inactive and ecommerce kits.

## 5. Difference

- Appsmith defines kit eligibility filters explicitly.
- The migrated implementation uses a backend endpoint whose filter set is not proven from the current evidence.

## 6. Impact

- Data correctness: inactive or ecommerce kits may become selectable.
- User-facing behavior: the picker can show kits that Appsmith would have hidden.
- System integrity: quote rows may be created from kits outside the Appsmith-allowed set.

## 7. Severity & Importance

- Severity: 2
- Importance: 4
- Priority: MEDIUM

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the kit-picker request to `/quotes/v1/kits`.
- Search for the UI code that groups or renders kit options.

2. Identify Faulty Logic
- Determine whether the current kit source can include inactive or ecommerce kits.
- Confirm whether the frontend relies entirely on endpoint results without enforcing the Appsmith eligibility rules.

3. Reference Correct Behavior
- Replicate the Appsmith kit-picker behavior: only active, non-ecommerce kits are available for selection.

4. Apply Changes
- Prefer implementing the eligibility filter in the backend endpoint that serves `/quotes/v1/kits`.
- If the endpoint already includes the necessary flags, add a frontend safeguard that filters out inactive and ecommerce kits before rendering.
- Preserve the richer migrated presentation fields if they do not change the allowed kit set.

5. Validation Steps
- Input: open the kit picker with a dataset containing inactive, ecommerce, and eligible kits.
- Expected output: only eligible active, non-ecommerce kits are shown.
- Input: select a visible kit and add it to a quote.
- Expected output: add-row behavior continues to work with the filtered kit set.

6. Edge Cases
- If no eligible kits remain after filtering, show an explicit empty state.
- Preserve category grouping for the remaining valid kits.

## 9. Acceptance Criteria

- The kit picker exposes only the Appsmith-eligible kit set.
- Richer migrated display fields remain functional.
- Verification for `out_16` would now result in `MATCH`.

## 10. Notes

- Confidence level: MEDIUM
- Exact backend filtering behind `/quotes/v1/kits` is not fully verifiable from the current summary.
