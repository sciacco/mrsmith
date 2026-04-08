# LISTINI-IMPL Review Feedback

Review target: `apps/listini-e-sconti/LISTINI-IMPL.md`

## Findings

### 1. High — rack customer filtering is internally inconsistent (and may diverge from the audited Appsmith behavior)

- The rack-customer dropdown query explicitly preserves the audited behavior: it does **not** filter on `cli_fatturazione.stato` (`LISTINI-IMPL.md:1095-1104`).
- But the rack list query *does* require `cli_fatturazione.stato = 'attivo'` via a subselect (`LISTINI-IMPL.md:1084-1087`).

Why this matters:
- A customer can appear in the dropdown but return an empty rack list solely because of the extra `stato='attivo'` constraint.
- It also contradicts the “preserves exact behavior” note directly below the SQL.

Required revision:
- Either remove the `cli_fatturazione.stato = 'attivo'` constraint from the rack list query, or apply the same constraint consistently to the rack-customer dropdown (and treat it as a deliberate spec change).

### 2. Medium — spec vs plan endpoint prefix is still ambiguous

- The plan defines the API under `/listini/v1/...`.
- The spec (`apps/listini-e-sconti/SPEC.md`) still documents endpoints under `/api/v1/...`.

Why this matters:
- It’s easy for implementers (and later reviewers) to mix the two and accidentally implement or test against the wrong path shape.

Required revision:
- Add an explicit note near the top of the plan that `/api/v1/...` in the spec maps to `/listini/v1/...` in this repo, or update the spec to match the repo’s app-scoped prefix convention.

### 3. Medium — catalog test updates are not called out but will be required

- The plan changes the applaunch catalog entry for `listini-e-sconti` to a dedicated role (`app_listini_access`) and a new href (`/apps/listini-e-sconti/`).
- `backend/internal/platform/applaunch/catalog_test.go` has hard-coded expectations about which apps are “placeholders” vs role-gated.

Why this matters:
- Implementing the catalog changes without adjusting those tests will break `cd backend && go test ./...` (and likely CI).

Required revision:
- Add an explicit “update catalog tests” item to the infra wiring/verification sections, so it doesn’t get missed during execution.

## Summary

Most earlier execution drift is resolved. Remaining issues are concentrated around the rack filtering consistency, the spec-to-plan endpoint prefix mapping, and calling out the necessary catalog test updates.
