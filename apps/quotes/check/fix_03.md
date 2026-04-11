# Fix Report 03

## Issue Summary

`issue_03.md` flagged that the migrated standard-quote creation flow only applied post-fetch client-side text filtering on the deal list returned from `/quotes/v1/deals`, while the Appsmith reference (`get_potentials`) constrained the dataset at query time with hardcoded pipeline/stage filters plus a non-empty `codice` condition. `out_03.md` recorded this as `PARTIAL MATCH` because the backend contract for `/quotes/v1/deals` was not visible from `apps/quotes/src` alone, so Appsmith eligibility parity could not be proven.

## Root Cause

The backend `handleListDeals` handler in `backend/internal/quotes/handler_reference.go` already enforced the Appsmith eligibility rules (the two pipeline IDs, their stage whitelists, and `d.codice <> ''`), but:

1. The filter IDs were defined as package-level `var`s that were then discarded via `_ = standardPipeline` stubs and re-hardcoded as string literals in the query. There was no test asserting the rules matched Appsmith, and the indirection between the declared constants and the SQL made the parity impossible to verify mechanically.
2. The `ORDER BY d.name` diverged from the Appsmith source which used `order by id desc`.

The frontend hook and `QuoteCreatePage` behavior is correct: `useDeals()` calls `/quotes/v1/deals`, the client-side text search only narrows the already-eligible server set (it cannot reintroduce excluded deals), and the acceptance criteria require nothing more than eligibility parity plus a preserved local search layer.

## Changes Made

- `backend/internal/quotes/handler_reference.go`
  - Added `"strings"` to the import list.
  - Removed the dead `_ = standardPipeline` / `standardStages` / `iaasPipeline` / `iaasStages` suppression block; those vars are now genuinely referenced.
  - Added `quoteStageInClause(stages []string) string` that renders a quoted SQL IN list (e.g. `('424443344','424502259',...)`) from a stage slice. The stage IDs are hardcoded backend constants (not user input), so direct interpolation is safe and keeps the query shape identical to the Appsmith source.
  - Promoted the deal-list SQL to a package-level `listDealsQuery` variable composed from the `standardPipeline` / `standardStages` / `iaasPipeline` / `iaasStages` constants. This ties the SQL to the constants so any future drift of the Appsmith IDs is a single-source-of-truth change, and it lets tests assert the resulting SQL directly.
  - Aligned the ordering with the Appsmith source: `ORDER BY d.id DESC` (was `ORDER BY d.name`). Ordering is not an eligibility rule, but matching it removes the last surface-level divergence from `get_potentials` and makes the SQL visibly round-trip to the Appsmith source.
  - `handleListDeals` now uses `listDealsQuery` via `h.db.QueryContext(r.Context(), listDealsQuery)`; all other handler behavior (scan struct, NULL handling, response shape) is unchanged.
- `backend/internal/quotes/handler_quotes_test.go`
  - Added `"strings"` to the import list.
  - Added `TestListDealsQueryMatchesAppsmithEligibility`, which:
    - Pins each pipeline/stage constant to its Appsmith `get_potentials.txt` value (fails loudly on any future drift).
    - Asserts that the assembled `listDealsQuery` contains each of the three Appsmith eligibility fragments verbatim: both pipeline comparisons, both stage `IN` lists, and `d.codice <> ''`.

No frontend changes were needed. `useDeals()` and `QuoteCreatePage.filteredDeals` already honour the contract: the server returns only eligible deals, and the local `dealSearch` layer is purely a narrowing text filter over `deal.name` / `deal.company_name`.

## Validation

- `cd backend && go build ./...` — clean, no output.
- `cd backend && go vet ./internal/quotes/...` — clean, no output.
- `cd backend && go test ./internal/quotes/...` — `ok  github.com/sciacco/mrsmith/internal/quotes  0.669s` (includes the new `TestListDealsQueryMatchesAppsmithEligibility`).
- `pnpm --filter mrsmith-quotes exec tsc --noEmit` — clean, no output.

## Acceptance Criteria Check

- **The selectable deal list for standard quote creation matches Appsmith eligibility rules.** — PASS. `listDealsQuery` filters on `(d.pipeline = '255768766' AND d.dealstage IN ('424443344','424502259','424502261','424502262')) OR (d.pipeline = '255768768' AND d.dealstage IN ('424443381','424443586','424443588','424443587','424443589')) AND d.codice <> ''`, which is the exact eligibility predicate from `quotes-main/pages/Nuova Proposta/queries/get_potentials/get_potentials.txt`. A unit test now locks this in.
- **Local text search still works on the resulting allowed set.** — PASS. `QuoteCreatePage.filteredDeals` still `.filter`s the server-returned deals on `name` / `company_name`; because the server set is already restricted by the Appsmith eligibility predicate, the client filter can only narrow that set — it cannot reintroduce excluded deals.
- **Verification for `out_03` would now result in `MATCH`.** — PASS. The backend contract now (a) composes its SQL from the same pipeline/stage constants the Appsmith source uses and (b) is asserted by a test that would fail on any divergence, so a re-run of `check_03.md` can point at `listDealsQuery` and `TestListDealsQueryMatchesAppsmithEligibility` as explicit parity evidence.

## Notes

- The Appsmith source also selects extra columns (`d.codice`, `p.label as pipeline`, `ds.label as stage`, `o.email as owner`) and joins `loader.hubs_pipeline`, `loader.hubs_stages`, `loader.hubs_owner`. None of those are consumed by `QuoteCreatePage` / `DealCard` — only `id`, `name`, `company_id`, `company_name` are used — so the backend deliberately keeps the projection minimal. This is a projection-surface difference, not an eligibility-rule difference, and is outside the scope of this issue.
- `quoteStageInClause` builds the SQL `IN (...)` list via plain string concatenation. Every input is a compile-time constant defined in this file, so there is no user input reaching the concatenation and no SQL-injection surface. I did not switch to `fmt.Sprintf` because the task constraint requires literal format strings for `Sprintf`, and `strings.Join` keeps the intent obvious.
- `ORDER BY d.id DESC` is a behavioral change from `ORDER BY d.name`, but only in how the list is ordered on the wire. The frontend already re-sorts visually via the search layout, and this aligns with Appsmith's `order by id desc`, which is the authoritative contract.
- I did not touch the pre-existing unused `wantIaasStages`-style warnings, the Go 1.21 `mapsloop`/`minmax` lints, or any unrelated `fmt.Sprintf` call sites, per the task constraints.

## QA Review

**Reviewer:** QA subagent
**Verdict:** PASS_WITH_NOTES
**Score:** 8/10

### Acceptance Criteria Verification

- **AC1 — Selectable deal list matches Appsmith eligibility rules.** PASS. `listDealsQuery` is composed from `standardPipeline`/`standardStages` and `iaasPipeline`/`iaasStages` constants, includes `d.codice <> ''`, and wraps both pipelines in outer parens.
- **AC2 — Local text search still works on the resulting allowed set.** PASS. `QuoteCreatePage.filteredDeals` narrows server-returned deals on `name` / `company_name` only.
- **AC3 — `out_03` would now result in MATCH.** PASS. The backend contract is pinned via `listDealsQuery` + `TestListDealsQueryMatchesAppsmithEligibility`.

### Code Quality Findings

**Test does not assert ORDER BY alignment or outer-paren structure (confidence: 82).** `TestListDealsQueryMatchesAppsmithEligibility` checks five eligibility fragments but not `ORDER BY d.id DESC` or the outer double-paren wrapping both pipeline conditions. If the outer parens were accidentally removed, `d.codice <> ''` would only apply to the iaas pipeline. Not a current bug, but a test-coverage gap.

**Misleading WHERE clause in fix_03.md line 42 (confidence: 80).** The human-readable description omits the outer parens. Actual SQL is correct; documentation-only issue.

### Recommendations

1. Add `"ORDER BY d.id DESC"` to the `mustContain` slice.
2. Add a fragment check for `"((d.pipeline"` to catch outer-paren regressions.
3. Correct the WHERE clause description in fix_03.md to include outer parens.

None are blockers.
