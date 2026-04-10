[QUOTES][HIGH] Expand per-product update payload to match Appsmith quote-row product semantics

## 1. Context

- This issue concerns the migration from Appsmith to `apps/quotes`.
- It was identified during factual verification of the migrated implementation.

## 2. Affected Check

- ID: `out_21`
- Verification target: Per-product update flow for quote-row products

## 3. Observed Behavior (Migrated Implementation)

- The migrated app updates products using reduced payloads.
- Product selection sends only `included: true` and `quantity`.
- Quantity editing sends only `quantity`.

## 4. Expected Behavior (Appsmith Reference)

- Appsmith updates product rows with a richer payload including product identity, prices, quantity, extended description, and included state.
- Appsmith also applies spot-quote `mrc = 0` adjustment and forces quantity `1` for included rows that would otherwise be zero.

## 5. Difference

- Appsmith sends a full product update payload and applies additional business rules.
- The migrated implementation sends only partial updates and does not mirror the documented Appsmith adjustments or the explicit `included: false` path.

## 6. Impact

- Data correctness: product row state can be saved incompletely or with incorrect pricing/quantity semantics.
- User-facing behavior: product selection and editing may not reflect the full Appsmith business logic.
- System integrity: quote-row product updates can drift from the source contract used by downstream calculations.

## 7. Severity & Importance

- Severity: 4
- Importance: 5
- Priority: HIGH

## 8. Remediation Instructions (LLM-Executable)

1. Locate Code
- Search for the product selection handler on quote-row products.
- Search for the quantity update handler.
- Search for the request payload sent when updating a quote-row product.

2. Identify Faulty Logic
- Confirm that current updates send only `included` and `quantity` fields.
- Confirm that the migrated flow does not expose or submit the Appsmith-equivalent fields for prices, description, and explicit included-state transitions.
- Confirm that the spot-quote `mrc = 0` rule and included-row quantity correction are absent from the observable migrated behavior.

3. Reference Correct Behavior
- Replicate the Appsmith product update behavior: submit the full product update payload and apply the documented price and quantity adjustments before persistence.

4. Apply Changes
- Expand the update payload builder so it includes all fields required by the Appsmith-equivalent product update contract.
- Add explicit handling for both `included: true` and `included: false` transitions if the product model supports deselection.
- Apply the Appsmith spot-quote rule that forces `mrc = 0` where required.
- Apply the Appsmith rule that forces quantity `1` when an included row would otherwise resolve to zero.
- If the backend endpoint already derives some fields, make the contract explicit and ensure the final persisted outcome still matches Appsmith.

5. Validation Steps
- Input: select a product variant that should become included.
- Expected output: the update payload and persisted state reflect the full Appsmith-equivalent row data, including corrected quantity if needed.
- Input: edit quantity on a standard quote and on a spot quote.
- Expected output: quantity persists correctly, and spot quotes apply the `mrc = 0` rule.
- Input: toggle included state off if supported.
- Expected output: the explicit deselection path persists correctly.

6. Edge Cases
- Preserve existing quantity values when they are valid and nonzero.
- If extended description is not currently editable in the migrated UI, still preserve or submit the correct value required by the Appsmith-equivalent contract rather than clearing it implicitly.

## 9. Acceptance Criteria

- Product updates use an Appsmith-equivalent payload and business rules.
- Spot-quote and included-row adjustments are preserved.
- Explicit included-state transitions are supported where applicable.
- Verification for `out_21` would now result in `MATCH`.

## 10. Notes

- Confidence level: HIGH
- Some server-side enforcement may already exist, but the current migrated frontend does not mirror the documented Appsmith update flow.
