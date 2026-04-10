[QUOTES][HIGH] Restore Appsmith service, template, and billing-lock logic in standard quote creation

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_08`
- Verification target: Standard create-flow reads and frontend logic for services, templates, and billing locks

## 3. Observed Behavior (Migrated Implementation)

- The migrated standard create flow loads templates by quote type only.
- It does not load service categories.
- It does not apply the documented COLOCATION-driven billing lock behavior.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith reads service categories.
- Appsmith derives allowed templates from document type and service selection.
- Appsmith forces trimestral billing when COLOCATION is selected for recurring documents.

## 5. Difference

- Appsmith contains explicit service-driven and document-driven creation logic.
- The migrated implementation reduces the flow to template loading by type and omits the service and billing-lock rules.

## 6. Impact

- Data correctness: created quotes can carry the wrong template or billing cadence.
- User-facing behavior: users are not guided by the same service-dependent constraints as Appsmith.
- System integrity: the create flow can produce records that violate the original source logic.

## 7. Severity & Importance

- Severity: 4
- Importance: 5
- Priority: HIGH

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the standard quote creation flow.
- Search for the template-loading request that currently filters only by quote type.
- Search for any existing service-category loading path and billing-period state management.

2. Identify Faulty Logic
- Confirm that service categories are not loaded in the standard create flow.
- Confirm that template eligibility depends only on quote type.
- Confirm that COLOCATION does not trigger the documented billing lock for recurring documents.

3. Reference Correct Behavior
- Replicate the Appsmith flow: load service categories, constrain templates using document type plus service selection, and force trimestral billing when COLOCATION is selected for recurring documents.

4. Apply Changes
- Add the service-category read to the standard create flow.
- Introduce service selection state if it is missing.
- Recompute allowed templates whenever document type or selected services change.
- Add the COLOCATION rule so recurring documents automatically switch to trimestral billing and prevent conflicting manual selection while the condition is active.
- If migrated backend endpoints already expose derived template options, consume that contract instead of duplicating incompatible frontend logic.

5. Validation Steps
- Input: open standard quote creation for a recurring document and select COLOCATION.
- Expected output: billing is forced to the Appsmith-equivalent trimestral setting and conflicting choices are disabled.
- Input: change selected services and document type.
- Expected output: template options update to the Appsmith-allowed set.
- Input: create a quote after these selections.
- Expected output: the submitted data reflects the constrained template and billing state shown in the UI.

6. Edge Cases
- When COLOCATION is removed, restore normal billing editability and keep a valid billing value.
- If no template remains valid for the current service/document combination, show an explicit empty state instead of silently retaining an invalid selection.

## 9. Acceptance Criteria

- Service categories are loaded in standard quote creation.
- Template eligibility reflects document type and selected services.
- COLOCATION on recurring documents forces the Appsmith-equivalent billing behavior.
- Verification for `out_08` would now result in `MATCH`.

## 10. Notes

- Confidence level: HIGH
- This is a direct mismatch in both data reads and conditional execution logic.
