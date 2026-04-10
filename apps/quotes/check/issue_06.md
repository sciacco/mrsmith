[QUOTES][HIGH] Restore customer-specific default payment lookup in quote creation

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_06`
- Verification target: Customer-specific ERP default-payment lookup and fallback

## 3. Observed Behavior (Migrated Implementation)

- The migrated create flow uses a static default payment method of `402`.
- The customer-specific payment lookup is not active in the flow.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith looks up the ERP payment code for the selected customer.
- When no value is available, Appsmith falls back to `402`.

## 5. Difference

- Appsmith derives the payment method from the selected customer and only uses `402` as fallback.
- The migrated implementation always starts from the fallback and does not run the customer lookup.

## 6. Impact

- Data correctness: created quotes can store the wrong payment method.
- User-facing behavior: users may see an incorrect default payment method after selecting a customer.
- System integrity: downstream billing and contract generation can start from an incorrect payment setting.

## 7. Severity & Importance

- Severity: 4
- Importance: 4
- Priority: HIGH

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the standard quote creation flow and the initial payment method state.
- Search for the customer-selection logic in quote creation.
- Search for the customer-payment lookup path used by the migrated implementation.

2. Identify Faulty Logic
- Confirm that the create flow sets `402` statically and does not trigger a customer-specific payment lookup when the customer changes.

3. Reference Correct Behavior
- Replicate the Appsmith rule: when a customer is selected, fetch the ERP payment code for that customer; if no value is returned, keep or apply fallback `402`.

4. Apply Changes
- Trigger the customer-payment lookup immediately after customer selection becomes available in the create flow.
- When the lookup returns a valid payment code, update the form state to that code.
- When the lookup returns no value or an empty value, explicitly retain fallback `402`.
- Ensure the selected payment method stays editable afterward if the migrated UX allows manual override.

5. Validation Steps
- Input: select a customer that has a defined ERP payment code.
- Expected output: the payment method field updates to that customer-specific code.
- Input: select a customer with no ERP payment code.
- Expected output: the payment method remains `402`.
- Input: create a quote after both scenarios.
- Expected output: the submitted payload contains the resolved payment method actually shown in the form.

6. Edge Cases
- If the user changes customer multiple times, always apply the latest lookup result and discard stale responses.
- If the lookup request fails, keep `402` and surface the existing error handling instead of leaving the field undefined.

## 9. Acceptance Criteria

- Quote creation derives the default payment method from the selected customer.
- Fallback `402` is used only when the customer lookup has no value or fails.
- The submitted quote payload reflects the resolved payment method.
- Verification for `out_06` would now result in `MATCH`.

## 10. Notes

- Confidence level: HIGH
- This issue is directly documented as a feature drop in the migrated frontend.
