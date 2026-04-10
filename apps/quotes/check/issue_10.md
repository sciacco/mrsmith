[QUOTES][HIGH] Implement Appsmith-specific IaaS quote creation behavior

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_10`
- Verification target: IaaS quote creation flow

## 3. Observed Behavior (Migrated Implementation)

- The migrated app reuses the generic create page for IaaS.
- It switches only the quote type and loads templates by type.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith uses an IaaS-specific creation flow.
- The documented behavior includes language-filtered templates, template-to-kit and template-to-services mappings, fixed recurring term behavior, trial text generation, and insertion of one derived kit row.

## 5. Difference

- Appsmith has dedicated IaaS business rules in the create flow.
- The migrated implementation treats IaaS as a generic variant and omits the documented mappings and generated content.

## 6. Impact

- Data correctness: IaaS quotes can be created with the wrong template, services, term, or missing derived kit data.
- User-facing behavior: IaaS users do not receive the same guided flow or generated content as Appsmith.
- System integrity: IaaS records may diverge substantially from source-system expectations.

## 7. Severity & Importance

- Severity: 4
- Importance: 4
- Priority: HIGH

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the IaaS create entry point and the generic create flow it currently reuses.
- Search for the template-loading logic used during IaaS creation.
- Search for where the create submission assembles kits, services, term values, and generated notes or text.

2. Identify Faulty Logic
- Confirm that IaaS creation currently differs from standard creation only by quote type.
- Confirm that language-filtered templates, template-derived kits/services, fixed term behavior, and trial text generation are absent.

3. Reference Correct Behavior
- Replicate the Appsmith IaaS flow: language-filtered template selection, template-to-kit mapping, template-to-services mapping, fixed recurring-term behavior, trial text generation, and creation of exactly one derived kit row.

4. Apply Changes
- Add IaaS-specific state and rules rather than treating IaaS as only a generic type variant.
- Filter available templates by the language rule used by Appsmith.
- Add deterministic mappings from the selected IaaS template to the required kit and services.
- Enforce the Appsmith-equivalent fixed term model for IaaS.
- Generate the Appsmith-equivalent trial text during create when the triggering conditions are met.
- Ensure the create transaction inserts the single derived kit row expected for IaaS.

5. Validation Steps
- Input: create an IaaS quote for each supported template/language combination.
- Expected output: only Appsmith-allowed templates are selectable for that language, and the chosen template deterministically sets the expected services, term behavior, and derived kit row.
- Input: create an IaaS quote in a trial scenario.
- Expected output: the generated trial text matches the Appsmith behavior and appears in the created record.

6. Edge Cases
- If a selected language produces no valid templates, show a clear empty state and block submission.
- Ensure standard quote creation is unaffected by IaaS-only rules.

## 9. Acceptance Criteria

- IaaS quote creation follows a dedicated Appsmith-equivalent business flow.
- Template, language, services, term, trial text, and derived kit behavior are aligned with Appsmith.
- Verification for `out_10` would now result in `MATCH`.

## 10. Notes

- Confidence level: HIGH
- The current summary identifies this as a direct mismatch against documented Appsmith IaaS behavior.
