[QUOTES][MEDIUM] Ensure owner list excludes archived entries

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_04`
- Verification target: Owner-list loading

## 3. Observed Behavior (Migrated Implementation)

- The migrated app loads owners from `/quotes/v1/owners`.
- The owner list is also used to resolve the "Le mie proposte" preset by matching owner email to the authenticated user.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith explicitly filters the owner source to non-archived owners only.

## 5. Difference

- Appsmith defines the archived-owner exclusion explicitly.
- The migrated implementation uses a backend endpoint whose filter semantics are not proven from the current evidence.

## 6. Impact

- Data correctness: archived owners may appear in selection or matching flows.
- User-facing behavior: filters and presets may resolve against stale or invalid owners.
- System integrity: ownership-based filtering can become inconsistent with the source system.

## 7. Severity & Importance

- Severity: 2
- Importance: 4
- Priority: MEDIUM

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for all uses of `/quotes/v1/owners`.
- Search for the logic that resolves the "Le mie proposte" owner preset.

2. Identify Faulty Logic
- Determine whether archived owners can be returned by the current endpoint or retained in the owner list used by the UI.

3. Reference Correct Behavior
- Replicate the Appsmith rule that only non-archived owners are available for create/detail flows and owner-based presets.

4. Apply Changes
- Prefer enforcing archived-owner exclusion in the backend implementation of `/quotes/v1/owners`.
- If the backend already returns an archive flag, add a frontend filter so only active owners remain visible and selectable.
- Ensure the "Le mie proposte" preset matches only against the filtered owner list.

5. Validation Steps
- Input: load any owner-dependent flow and inspect the owner options.
- Expected output: archived owners are not present.
- Input: use the "Le mie proposte" preset.
- Expected output: it resolves only against active owners and still matches the authenticated user when appropriate.

6. Edge Cases
- Handle the case where the authenticated user maps only to an archived owner by leaving the preset unresolved rather than selecting an invalid owner.
- Preserve owner name rendering and email-based preset behavior for active owners.

## 9. Acceptance Criteria

- Archived owners are excluded from all migrated owner lists.
- The "Le mie proposte" preset continues to work for active owners.
- Verification for `out_04` would now result in `MATCH`.

## 10. Notes

- Confidence level: MEDIUM
- Backend behavior for `/quotes/v1/owners` is not fully verifiable from the current summary.
