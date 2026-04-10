[QUOTES][MEDIUM] Preserve Appsmith delete-row behavior while keeping migrated confirmation UX

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_18`
- Verification target: Deleting a kit row from a quote

## 3. Observed Behavior (Migrated Implementation)

- The migrated app deletes a row through a routed DELETE endpoint.
- The UI uses a 3-second inline confirmation state.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith deletes the selected quote row by id.

## 5. Difference

- The core delete action exists in both implementations.
- The migrated implementation changes the interaction model and delegates the delete operation to a backend endpoint, so exact delete semantics are not proven from the current evidence.

## 6. Impact

- Data correctness: delete behavior can diverge if the endpoint targets a different row or uses different constraints.
- User-facing behavior: confirmation UX differs from Appsmith, though this is acceptable if the underlying delete semantics remain correct.
- System integrity: row deletion relies on backend semantics that are not directly proven.

## 7. Severity & Importance

- Severity: 2
- Importance: 3
- Priority: MEDIUM

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the delete-row action on the quote detail page.
- Search for the DELETE request path used for row deletion.
- Search for the inline confirmation timer or state.

2. Identify Faulty Logic
- Confirm whether the routed DELETE endpoint deletes the selected row by id exactly as Appsmith does.
- Verify that the confirmation state does not allow accidental deletion of the wrong row.

3. Reference Correct Behavior
- Replicate the Appsmith underlying behavior: delete the selected quote row by id and refresh the visible row list.
- The migrated confirmation UX may remain if it does not change the underlying deletion semantics.

4. Apply Changes
- Align the backend or frontend delete flow so the selected row id is the only target of the delete request.
- Ensure the visible row list refreshes after successful deletion.
- Keep the inline confirmation only if it is scoped to the selected row and cannot leak to another row.

5. Validation Steps
- Input: trigger delete on a specific row and confirm within the allowed time window.
- Expected output: only that row is deleted and the row list refreshes.
- Input: let the confirmation expire or cancel the action.
- Expected output: no delete request is sent.

6. Edge Cases
- Handle rapid repeated clicks without sending duplicate deletes.
- Preserve row identity correctly when the list rerenders during the confirmation state.

## 9. Acceptance Criteria

- Deleting a row removes exactly the selected row.
- The row list refreshes after deletion.
- The migrated confirmation UX does not introduce incorrect-row deletes.
- Verification for `out_18` would now result in `MATCH`.

## 10. Notes

- Confidence level: HIGH
- Backend parity for the routed delete is not fully verifiable from the current summary, but the visible flow is present.
