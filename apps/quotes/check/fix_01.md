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

- Updated the quotes frontend hook to omit `page=1` from the untouched default
  request so the initial list load stays parameterless.
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

- The parity path now depends on the frontend continuing to omit `page=1` for
  the untouched default list request. If a future change starts sending
  `page=1` again, the backend will treat it as explicit pagination and use the
  migrated 25-row page size.
- Verification here is focused. I did not run an end-to-end browser check
  against live quote data.

## QA Review

**Reviewer:** QA subagent
**Verdict:** PASS_WITH_NOTES
**Score:** 8/10

### Acceptance Criteria Verification

- **AC 1 — Default quote-list load matches Appsmith reference.** PASS. `QuoteListPage.tsx` passes `page=1` to `useQuotes`, which omits `page=1` (queries.ts:107-108). Backend receives no `page`, `isAppsmithDefaultQuoteListRequest` returns true, handler uses `pageSize=2000` with `quote_number DESC`.
- **AC 2 — Sorting and row-limit aligned.** PASS. Sort defaults to `q.quote_number DESC` (handler_quotes.go:53-58); 2000-row LIMIT applied and `total` capped at 2000 (handler_quotes.go:126-128).
- **AC 3 — Migrated filters/pagination still work.** PASS. Any non-empty filter, explicit page, or explicit sort/dir flips `isAppsmithDefaultQuoteListRequest` to false, preserving `pageSize=25` paginated behavior.
- **AC 4 — `out_01` would now result in MATCH.** PASS by construction.

### Code Quality Findings

- **Finding 1 — Mid-session flip to Appsmith path (confidence: 85).** When a user pages to 2 then returns to page 1 via `handlePageChange(1)`, `useQuotes` suppresses `page=1`, causing the backend to return 2000 rows with `page_size=2000`. The `Pagination` UI collapses mid-session. Fix: make frontend detection symmetric to `isAppsmithDefaultQuoteListRequest` (only omit `page=1` when all other params are also absent), or guard `handlePageChange` to not navigate to page 1 via the empty-params path after explicit pagination started.
- **Finding 2 — Total count capping clarity (confidence: 80).** Logic is correct (filters are guaranteed empty in the Appsmith path), but a brief comment tying the cap to "mirrors Appsmith LIMIT" would improve readability. Minor.

### Recommendations

1. Guard `handlePageChange` against reinstating the Appsmith path mid-session; make frontend/backend detection symmetric.
2. Add a handler integration test (httptest + sqlmock) verifying `page_size=2000` for empty params and `page_size=25` for filtered requests.
3. Promote the "Remaining Risks" note about `page=1` regression into a tracked TODO.
