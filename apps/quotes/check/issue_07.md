[QUOTES][MEDIUM] Replace free-text replacement orders with loaded order selection

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_07`
- Verification target: Replacement-order loading for `SOSTITUZIONE`

## 3. Observed Behavior (Migrated Implementation)

- The migrated implementation captures replacement orders as free text.
- The dedicated replacement-order loading path is not used.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith loads an Alyante order list for replacement orders.
- Users choose from loaded order values instead of relying on plain free text.

## 5. Difference

- Appsmith uses a dedicated data read for replacement orders.
- The migrated implementation replaces that constrained selection with a plain text field.

## 6. Impact

- Data correctness: invalid or inconsistent replacement order values can be entered.
- User-facing behavior: users lose guided selection and may input non-existent orders.
- System integrity: replacement flows can store values that would not have been selectable in Appsmith.

## 7. Severity & Importance

- Severity: 3
- Importance: 3
- Priority: MEDIUM

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the `SOSTITUZIONE` quote path and the UI field that captures replacement orders.
- Search for the migrated replacement-order data-loading path or request code.

2. Identify Faulty Logic
- Confirm that the current UI uses free text and does not load the replacement-order source list.

3. Reference Correct Behavior
- Replicate the Appsmith behavior: load the replacement-order list and let the user choose from that dataset for `SOSTITUZIONE`.

4. Apply Changes
- Re-enable or implement the replacement-order read path in the migrated flow.
- Replace the plain text input with a selection control backed by the loaded replacement-order dataset.
- Preserve existing stored values only if they are part of the allowed options or intentionally supported as legacy data.

5. Validation Steps
- Input: open a `SOSTITUZIONE` flow.
- Expected output: the UI loads and renders replacement-order options rather than a free-text-only field.
- Input: select an option and save.
- Expected output: the selected replacement order is persisted and rendered consistently after reload.

6. Edge Cases
- Handle an empty order list gracefully by disabling selection and surfacing a clear empty state.
- If multiple orders share similar labels, ensure the stored value remains unambiguous.

## 9. Acceptance Criteria

- Replacement orders for `SOSTITUZIONE` are loaded from the source dataset.
- The migrated UI no longer relies on unrestricted free text for this field.
- Verification for `out_07` would now result in `MATCH`.

## 10. Notes

- Confidence level: HIGH
- The current issue is directly observable in the migrated frontend.
