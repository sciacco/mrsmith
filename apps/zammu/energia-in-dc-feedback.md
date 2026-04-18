# Energia in DC Plan Review

Status: approved

## Findings

No blocking findings remain after the updates to:

- `apps/zammu/energia-in-dc-impl.md`
- `apps/zammu/energia-in-dc-spec.md`

## Feedback

The planning package is now materially stronger. The implementation plan adds an explicit Appsmith contract-pinning gate, so handler logic and UI rendering are no longer allowed to drift ahead of verified legacy contracts.

The date/time story is also clearer and simpler now: Europe/Rome local time end-to-end, one pinned wire format, inclusive bounds, and no timezone-conversion layer. That matches the product reality better than adding unnecessary transport complexity.

The lookup chain is safer because the nested-resource invariants are now explicit in the plan. `site + customer -> rooms` and `room + customer -> racks` are treated as backend-owned rules instead of assumptions inherited from the frontend cascade.

The spec and plan now agree on the repo API namespace (`/api/energia-dc/v1/...`), which removes an avoidable source of doc/test/implementation drift.

The validation scope is now more proportionate to the app. The docs no longer imply a heavy “fixture phase”; they use `docs/grappa` as the schema source of truth and keep only a few narrow regression checks for the places where drift would actually hurt.

## Residual Risks

- The lightweight validation checks still need to be executed before signoff: power-readings pagination/order, ID-keyed no-variable detail loading, low-consumption optional-customer behavior, and the local datetime parsing contract.
- This is still a pre-gate artifact; `portal-miniapp-ui-review` approval is a separate required step before implementation handoff.

## Optional Improvements

- Add `apps/reports/src/pages/OrdiniPage.tsx` as one more cited comparable if you want the archetype evidence to align even more directly with the `data_workspace` reference in the mini-app skill.
- Consider logging unknown `magnetotermico` values when the backend falls back to breaker capacity `32`, so new hardware types do not drift silently.
