[QUOTES][MEDIUM] Align quote detail header loading with Appsmith field semantics

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_11`
- Verification target: Quote detail header loading

## 3. Observed Behavior (Migrated Implementation)

- The migrated app loads quote detail through `/quotes/v1/quotes/:id`.
- The returned object is consumed directly by the detail UI.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith loads detail with an explicit field projection.
- The documented source behavior includes a visible transformation of `replace_orders`.

## 5. Difference

- Appsmith defines field-level behavior explicitly.
- The migrated implementation does not prove equivalent field projection or the `replace_orders` transformation in the observable frontend behavior.

## 6. Impact

- Data correctness: some displayed header values can differ from Appsmith.
- User-facing behavior: replacement-order text formatting and related header fields may not match the source system.
- System integrity: detail consumers may rely on fields whose migrated semantics are not aligned.

## 7. Severity & Importance

- Severity: 2
- Importance: 4
- Priority: MEDIUM

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the detail-page request to `/quotes/v1/quotes/:id`.
- Search for the header, notes, and contact sections that consume the returned quote object.

2. Identify Faulty Logic
- Determine whether the migrated detail response already includes Appsmith-equivalent field formatting.
- Specifically verify whether `replace_orders` is transformed to match the Appsmith-visible format.

3. Reference Correct Behavior
- Replicate the Appsmith field semantics for quote detail loading, including the visible `replace_orders` transformation.

4. Apply Changes
- If the backend detail endpoint should own field projection and formatting, align that contract with the Appsmith behavior.
- If the backend intentionally returns raw values, add a frontend transformation layer before rendering so the displayed values match Appsmith.
- Keep route-param based loading if that is the migrated navigation model; only align the resulting visible behavior.

5. Validation Steps
- Input: open a quote whose `replace_orders` value exercises the Appsmith transformation.
- Expected output: the rendered header value matches the Appsmith-visible format.
- Input: verify core header fields after the change.
- Expected output: the detail page still renders quote owner, payment method, notes, and contact information without regression.

6. Edge Cases
- Handle null or empty `replace_orders` values without rendering placeholder corruption.
- Preserve raw stored values if the transformation is display-only.

## 9. Acceptance Criteria

- Quote detail displays Appsmith-equivalent field semantics for the header area.
- `replace_orders` formatting matches the Appsmith reference where applicable.
- Verification for `out_11` would now result in `MATCH`.

## 10. Notes

- Confidence level: MEDIUM
- Full backend field projection parity is not fully verifiable from the current summary.
