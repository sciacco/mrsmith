# Fix Report 01

## Issue Summary

The quotes list default load did not explicitly preserve Appsmith parity for
the unfiltered view. Appsmith uses one fixed query with
`ORDER BY quote_number DESC LIMIT 2000`, while the migrated app used backend
defaults plus a paginated request surface, so the default row-cap behavior was
not guaranteed.

## Files Changed

- `apps/quotes/src/api/queries.ts`
- `backend/internal/quotes/handler_quotes.go`
- `backend/internal/quotes/handler_quotes_test.go`
- `apps/quotes/check/fix_01.md`

## Implementation Summary

- Updated the list page to preserve the distinction between an omitted `page`
  parameter and an explicit `page=1`.
- Updated the quotes frontend hook to serialize an explicit `page=1` instead of
  dropping it unconditionally.
- Added backend detection for the true Appsmith-equivalent default request:
  no page, no filters, no explicit sort, no explicit direction.
- Kept the existing backend default sort of `quote_number DESC`.
- Applied the Appsmith `2000` row cap only for that default request shape and
  capped `total` accordingly, while leaving filtered, user-sorted, and
  explicitly paginated requests on the migrated paginated behavior.
- Added a focused Go test for the default-request detector.

## Verification Performed

- `cd backend && go test ./internal/quotes`
- `pnpm --filter mrsmith-quotes build`
- `cd backend && go build ./...` — clean after diagnostic cleanup.
- `cd backend && go vet ./internal/quotes/...` — clean.

## Diagnostic Cleanup

- Removed the trailing `fmt.Sprintf(" ORDER BY %s %s LIMIT $%d OFFSET $%d", …)`
  used when assembling `selectQuery` in `handleListQuotes`. The query string is
  now built by plain string concatenation plus `strconv.Itoa` for the numbered
  placeholders, so no `fmt.Sprintf` in this path carries a dynamic format
  string and the `printf` warnings on the `selectQuery` construct are gone.
- Verified `appsmithRowCap` is still used (pageSize default for the Appsmith
  path, and the `total > appsmithRowCap` cap), so it is not dead code.
- Unrelated pre-existing linter findings (`mapsloop`, `minmax`, unused
  `requireAlyante` in `handler.go`) were intentionally left untouched.

## Remaining Risks

- The parity path still depends on the frontend leaving `page` unset on the
  untouched initial load. If a future change starts always serializing
  `page=1`, the backend will treat that as explicit pagination and use the
  migrated 25-row page size.
- Verification here is focused. I did not run an end-to-end browser check
  against live quote data.

## QA Review

**Reviewer:** QA subagent
**Verdict:** PASS_WITH_NOTES
**Score:** 8/10

### Acceptance Criteria Verification

- **AC 1 — Default quote-list load matches Appsmith reference.** PASS. `QuoteListPage.tsx` now passes `page: undefined` only when the URL truly has no `page` param, so the untouched default request stays parameterless and the backend keeps the Appsmith 2000-row path.
- **AC 2 — Sorting and row-limit aligned.** PASS. Sort defaults to `q.quote_number DESC` (handler_quotes.go:53-58); 2000-row LIMIT applied and `total` capped at 2000 (handler_quotes.go:126-128).
- **AC 3 — Migrated filters/pagination still work.** PASS. Once pagination is explicit, `useQuotes()` now keeps `page=1` instead of collapsing back to the Appsmith path, so returning from page 2 to page 1 remains on the 25-row migrated pagination contract.
- **AC 4 — `out_01` would now result in MATCH.** PASS by construction.

### Code Quality Findings

- **Finding 1 — Mid-session flip to Appsmith path.** RESOLVED. `QuoteListPage.tsx` now preserves whether `page` was explicitly present in the URL, and `useQuotes()` no longer strips `page=1`. Returning from page 2 to page 1 stays on the paginated backend path.
- **Finding 2 — Total count capping clarity (confidence: 80).** Logic is correct (filters are guaranteed empty in the Appsmith path), but a brief comment tying the cap to "mirrors Appsmith LIMIT" would improve readability. Minor.

### Recommendations

1. Add a handler integration test (httptest + sqlmock) verifying `page_size=2000` for empty params and `page_size=25` for explicit `page=1` or filtered requests.
2. Promote the remaining frontend/backed contract dependency about untouched default `page` omission into a tracked TODO if this behavior is likely to evolve again.
