[QUOTES][MEDIUM] Align payment-method list filtering and ordering with Appsmith

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_05`
- Verification target: Payment-method list loading

## 3. Observed Behavior (Migrated Implementation)

- The migrated app loads payment methods from `/quotes/v1/payment-methods`.
- It consumes a normalized response shape containing `code` and `description`.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith loads only selectable payment methods.
- The documented behavior sorts payment methods by description.

## 5. Difference

- Appsmith explicitly constrains and orders the source data.
- The migrated implementation does not prove that `/quotes/v1/payment-methods` preserves the same selectable-only filter and ordering.

## 6. Impact

- Data correctness: non-selectable payment methods may be exposed.
- User-facing behavior: the ordering of options may differ from Appsmith.
- System integrity: payment method selection may drift from the source contract used by quote flows.

## 7. Severity & Importance

- Severity: 2
- Importance: 4
- Priority: MEDIUM

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for all uses of `/quotes/v1/payment-methods`.
- Search for the UI code that renders payment method options in create and detail flows.

2. Identify Faulty Logic
- Confirm whether the current endpoint or UI layer can return non-selectable methods or a different sort order than Appsmith.

3. Reference Correct Behavior
- Replicate the Appsmith behavior: expose only selectable payment methods and sort them by description.

4. Apply Changes
- Prefer implementing selectable-only filtering and description ordering in the backend endpoint.
- If the response already contains the necessary fields, add a frontend safeguard that filters non-selectable entries and sorts by description before rendering.
- Preserve the normalized `code` and `description` API shape if that is the adopted migrated contract.

5. Validation Steps
- Input: load create and detail flows that render payment methods.
- Expected output: only selectable methods are present and they appear in description order.
- Input: select a payment method and save.
- Expected output: existing save behavior continues to work without schema regressions.

6. Edge Cases
- Preserve rendering when descriptions differ only by case or locale-specific characters.
- Do not drop valid methods solely because the migrated contract normalizes field names.

## 9. Acceptance Criteria

- Payment-method options match the Appsmith selectable-only behavior.
- Option ordering matches the Appsmith description ordering.
- Verification for `out_05` would now result in `MATCH`.

## 10. Notes

- Confidence level: MEDIUM
- Backend semantics for `/quotes/v1/payment-methods` are not fully verifiable from the current summary.
