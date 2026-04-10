[QUOTES][MEDIUM] Wire the list-page delete action to the quote deletion flow

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_02`
- Verification target: Quote deletion from the list page

## 3. Observed Behavior (Migrated Implementation)

- The migrated UI shows an `Elimina` action on the list page when the user has delete permission.
- The visible action is not wired to the deletion mutation.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith exposes a working delete flow.
- The documented sequence performs role checking, optional HubSpot quote deletion, database deletion, and list refresh.

## 5. Difference

- Appsmith executes a delete flow end to end.
- The migrated implementation renders the delete affordance but does not invoke the deletion logic from the visible menu action.

## 6. Impact

- Data correctness: intended deletions do not occur.
- User-facing behavior: the UI advertises a destructive action that is ineffective.
- System integrity: users may assume data was deleted when it was not.

## 7. Severity & Importance

- Severity: 3
- Importance: 4
- Priority: MEDIUM

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the list-page quote actions menu and the `Elimina` label.
- Search for the deletion mutation or delete-request code for quotes.
- Search for the table or list component that renders row-level actions.

2. Identify Faulty Logic
- Confirm that the visible delete action does not call the deletion handler.
- Identify the missing callback wiring between the menu item and the quote deletion flow.

3. Reference Correct Behavior
- Replicate the Appsmith user-visible behavior: when delete is available and selected, the quote deletion flow must execute and the list must refresh afterward.
- If HubSpot deletion is handled in the backend, ensure the frontend still triggers the full backend delete path rather than a no-op.

4. Apply Changes
- Pass a concrete delete handler from the list/table layer into the row action menu.
- Ensure the handler invokes the existing delete mutation for the selected quote.
- On success, refresh or invalidate the quote list so the deleted item disappears from the page.
- Preserve the existing permission guard for showing delete.

5. Validation Steps
- Input: open the list as a user with delete permission and trigger `Elimina` on a quote.
- Expected output: the delete request executes, the quote is removed from the visible list, and the list refreshes.
- Input: open the list as a user without delete permission.
- Expected output: the delete action remains unavailable.

6. Edge Cases
- Handle request failure by keeping the row visible and surfacing the error state already used by the app.
- Do not trigger deletion for the wrong row if the menu is reused across rows.

## 9. Acceptance Criteria

- Selecting `Elimina` triggers the quote deletion flow.
- The visible list refreshes after successful deletion.
- Permission-based visibility of the delete action is preserved.
- Verification for `out_02` would now result in `MATCH`.

## 10. Notes

- Confidence level: HIGH
- This mismatch is directly observable in the migrated UI wiring and is not a backend-only verification gap.
