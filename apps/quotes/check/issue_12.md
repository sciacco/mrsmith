[QUOTES][HIGH] Restore Appsmith-equivalent HubSpot status contract and detail-page handling

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_12`
- Verification target: HubSpot status read and response handling on the detail page

## 3. Observed Behavior (Migrated Implementation)

- The migrated app reads HubSpot status from `/quotes/v1/quotes/:id/hs-status`.
- The visible contract is reduced to `hs_quote_id`, `status`, and `pdf_url`.
- The detail page uses `pdf_url` for the "Apri su HS" action.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith reads a broader HubSpot quote object.
- The documented behavior includes distinct quote and PDF links plus signature-related fields, including sign status.

## 5. Difference

- Appsmith distinguishes multiple HubSpot properties needed for detail-page decisions and links.
- The migrated implementation reduces the contract and reuses `pdf_url` for a user action that Appsmith ties to a separate quote link.

## 6. Impact

- Data correctness: detail-page status data can be incomplete or mapped incorrectly.
- User-facing behavior: "Apri su HS" can send users to the wrong target.
- System integrity: publish and signature-dependent flows may lack the fields required for Appsmith-equivalent behavior.

## 7. Severity & Importance

- Severity: 4
- Importance: 4
- Priority: HIGH

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the detail-page HubSpot status request to `/quotes/v1/quotes/:id/hs-status`.
- Search for the code that renders the "Apri su HS" action.
- Search for any logic that depends on HubSpot status during detail or publish flows.

2. Identify Faulty Logic
- Confirm that the current response model lacks the Appsmith-equivalent quote link, PDF link, and sign-status fields.
- Confirm that the UI uses `pdf_url` where Appsmith expects the HubSpot quote link for the open action.

3. Reference Correct Behavior
- Replicate the Appsmith contract closely enough to expose separate quote-link and PDF-link semantics plus the signature-related status needed by downstream flows.

4. Apply Changes
- Expand the migrated HubSpot status contract to include the distinct fields required by the Appsmith behavior.
- Update the detail page so "Apri su HS" uses the Appsmith-equivalent quote link rather than the PDF download link.
- Preserve a separate PDF action if the migrated UI needs it, but do not conflate the two URLs.
- Make the sign-status field available wherever publish gating or signature-state rendering depends on it.

5. Validation Steps
- Input: open a quote that has both a HubSpot quote page and a PDF download link.
- Expected output: "Apri su HS" opens the quote page, not the PDF URL.
- Input: load a quote with signature state populated.
- Expected output: the migrated detail/publish flow can access the Appsmith-equivalent sign status.

6. Edge Cases
- Handle missing HubSpot linkage by disabling or hiding actions rather than linking to an invalid target.
- Preserve behavior when only one of the quote-link or PDF-link values is available.

## 9. Acceptance Criteria

- The migrated HubSpot status contract exposes the fields required for Appsmith-equivalent detail behavior.
- "Apri su HS" uses the correct HubSpot quote link.
- Signature-related status is available for downstream logic.
- Verification for `out_12` would now result in `MATCH`.

## 10. Notes

- Confidence level: HIGH
- This mismatch affects both the read contract and the detail-page response handling.
