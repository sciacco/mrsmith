[QUOTES][HIGH] Reintroduce Appsmith quote creation orchestration for standard quotes

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_09`
- Verification target: Standard quote creation write flow

## 3. Observed Behavior (Migrated Implementation)

- The migrated standard create flow performs a single POST to `/quotes/v1/quotes`.
- Kits are not populated during creation.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith generates a quote number.
- Appsmith creates the HubSpot quote.
- Appsmith inserts the quote record.
- Appsmith inserts one row per selected kit.
- Appsmith stores the new quote id and navigates to detail.

## 5. Difference

- Appsmith uses a multi-step orchestration with several side effects before navigation.
- The migrated implementation compresses the flow to one backend request and omits the documented kit-row creation behavior from the create flow.

## 6. Impact

- Data correctness: created quotes may miss required generated identifiers or row data.
- User-facing behavior: the resulting quote detail can differ materially from the Appsmith-created record.
- System integrity: the migrated flow may not preserve the original create transaction semantics.

## 7. Severity & Importance

- Severity: 5
- Importance: 5
- Priority: HIGH

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the standard create flow submission path and the POST to `/quotes/v1/quotes`.
- Search for where kit selection is collected during creation, if present at all.
- Search for navigation to the new quote detail page after creation.

2. Identify Faulty Logic
- Confirm that the current flow performs only one create request.
- Confirm that the create flow does not generate or verify quote numbers, does not ensure HubSpot creation, and does not insert quote rows for selected kits within the creation sequence.

3. Reference Correct Behavior
- Replicate the Appsmith orchestration for standard quote creation: generate the quote number, create HubSpot data, insert the quote, insert one row per selected kit, store the new quote id, then navigate to detail.
- If the migrated backend consolidates these steps into one endpoint, that endpoint must still perform the same observable business actions before returning success.

4. Apply Changes
- Decide whether parity belongs in the backend create endpoint or in a multi-step frontend orchestration. Prefer one authoritative backend transaction if that is the migrated architecture.
- Ensure the create contract accepts the selected kits needed to create the initial quote rows.
- Ensure the success response is returned only after the quote number, HubSpot quote, quote record, and initial rows have been created or explicitly handled.
- Update the frontend create flow so it collects and submits the data required by the Appsmith-equivalent behavior.

5. Validation Steps
- Input: create a standard quote with one or more selected kits.
- Expected output: the resulting quote has a generated quote number, a valid created record, and initial quote rows corresponding to the selected kits before navigation completes.
- Input: open the created quote immediately after navigation.
- Expected output: the detail page shows the same initial row state that Appsmith would have produced at creation time.

6. Edge Cases
- Handle create failure atomically: if one required step fails, do not return success with a partially created quote unless that partial state is an explicit migrated design.
- Preserve correct behavior for zero-kit creation only if that is intentionally supported and aligned with Appsmith.

## 9. Acceptance Criteria

- Standard quote creation reproduces the Appsmith business outcome end to end.
- Initial quote rows are created during the create transaction when required by the selected kits.
- Navigation to detail occurs only after the quote is in an Appsmith-equivalent created state.
- Verification for `out_09` would now result in `MATCH`.

## 10. Notes

- Confidence level: HIGH
- The current summary documents this as a core orchestration mismatch.
