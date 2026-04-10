[QUOTES][MEDIUM] Align detail-page save payload and IaaS rewrite behavior with Appsmith

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_13`
- Verification target: Detail-page quote save flow

## 3. Observed Behavior (Migrated Implementation)

- The migrated detail page saves by sending the current local quote object through `PUT /quotes/v1/quotes/:id`.
- Template and services are read-only in the visible UI.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith builds an explicit update payload field by field.
- Appsmith also applies template and services remapping for specific IaaS template cases before saving.

## 5. Difference

- Appsmith performs an explicit payload assembly and conditional rewrite step.
- The migrated implementation sends the whole local object and does not reproduce the IaaS-specific remapping logic in the observable frontend behavior.

## 6. Impact

- Data correctness: saved quote data may not reflect Appsmith field mapping rules.
- User-facing behavior: updates to IaaS-related quotes can preserve stale or incompatible template/service values.
- System integrity: the save contract is looser than the source implementation and can drift from expected field semantics.

## 7. Severity & Importance

- Severity: 3
- Importance: 5
- Priority: MEDIUM

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the detail-page save action and the `PUT /quotes/v1/quotes/:id` request assembly.
- Search for where the local quote object is constructed and updated.
- Search for any current IaaS-specific save logic.

2. Identify Faulty Logic
- Confirm that the save payload is the raw local quote object rather than an explicit Appsmith-equivalent payload.
- Confirm that no IaaS template/services rewrite occurs before save.

3. Reference Correct Behavior
- Replicate the Appsmith save behavior: build the update payload field by field and apply the documented IaaS-specific remapping rules before submission.

4. Apply Changes
- Introduce an explicit save-payload builder so only the intended fields are submitted.
- Add an IaaS rewrite step before submission for the template/service combinations documented by Appsmith.
- Keep read-only UI controls if that is a product decision, but still ensure the saved payload matches the Appsmith-equivalent derived values.

5. Validation Steps
- Input: edit a standard quote and save.
- Expected output: only the intended fields are sent and persisted.
- Input: save an IaaS quote that triggers Appsmith remapping.
- Expected output: the submitted payload contains the Appsmith-equivalent rewritten template/services values.

6. Edge Cases
- Preserve unsaved-change tracking if the payload builder derives fields from local state.
- Avoid sending unrelated local-only fields that the backend does not own.

## 9. Acceptance Criteria

- Detail-page save uses an explicit Appsmith-equivalent payload.
- IaaS-specific remapping runs before save where required.
- Existing editable fields still save successfully.
- Verification for `out_13` would now result in `MATCH`.

## 10. Notes

- Confidence level: MEDIUM
- Final parity also depends on backend semantics for `PUT /quotes/v1/quotes/:id`, which are not fully documented in the current summary.
