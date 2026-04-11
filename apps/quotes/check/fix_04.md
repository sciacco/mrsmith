# Fix Report 04

## Issue Summary

`issue_04.md` (MEDIUM) — the migrated `/quotes/v1/owners` endpoint did not replicate the Appsmith query's `archived = FALSE` filter, so archived owners could leak into create/detail owner pickers and into the "Le mie proposte" preset resolution (which matches owner email to the authenticated user).

## Root Cause

`backend/internal/quotes/handler_reference.go` (`handleListOwners`) issued:

```sql
SELECT id, first_name, last_name, email FROM loader.hubs_owner ORDER BY last_name, first_name
```

The Appsmith source query (`quotes-main/pages/Nuova Proposta/queries/get_owners/get_owners.txt`) is:

```sql
select * from loader.hubs_owner where archived = FALSE
```

The migrated backend was missing the `WHERE archived = FALSE` predicate. Schema confirms `loader.hubs_owner.archived boolean NOT NULL` (see `docs/mistradb/mistra_loader.json`).

## Changes Made

- `backend/internal/quotes/handler_reference.go` — added `WHERE archived = FALSE` to the `handleListOwners` query, preserving the existing column list and `ORDER BY last_name, first_name` ordering.

No frontend changes were necessary: `apps/quotes/src/api/queries.ts` (`useOwners`) and consumers (`QuoteCreatePage.tsx`, `HeaderTab.tsx`, `FilterBar.tsx`) already treat the backend response as the canonical owner list. The "Le mie proposte" preset in `FilterBar.tsx` resolves by matching the authenticated user's email against this list; with archived owners no longer returned, the preset is automatically scoped to active owners and naturally remains unresolved if the authenticated user only maps to an archived owner.

## Validation

- `cd backend && go build ./...` — clean.
- `cd backend && go vet ./internal/quotes/...` — clean.
- `pnpm --filter mrsmith-quotes exec tsc --noEmit` — clean (no TS changes needed, but run for completeness).

## Acceptance Criteria Check

- Archived owners are excluded from all migrated owner lists — **PASS** (enforced at `/quotes/v1/owners` backend query).
- The "Le mie proposte" preset continues to work for active owners — **PASS** (email match still operates over the same response shape; active owners are retained).
- Verification for `out_04` would now result in `MATCH` — **PASS** (backend SQL now matches Appsmith semantics: `select ... from loader.hubs_owner where archived = FALSE`).

## Notes

- Edge case: if the authenticated user maps only to an archived owner, the preset will leave the owner filter unresolved rather than selecting an invalid owner — behavior identical to Appsmith under the same filter.
- Minimal scoped change; no unrelated code or test modifications. No new imports introduced.

## QA Review

**Reviewer:** QA subagent
**Verdict:** PASS
**Score:** 10/10

### Acceptance Criteria Verification

- **AC1 — Archived owners excluded from all migrated owner lists.** PASS. `handleListOwners` in `handler_reference.go` now reads `SELECT ... FROM loader.hubs_owner WHERE archived = FALSE ORDER BY last_name, first_name`. Schema confirms `archived` is `boolean NOT NULL`. Filter at DB layer covers all consumers.
- **AC2 — "Le mie proposte" preset continues to work for active owners.** PASS. `FilterBar.tsx` resolves preset via `owners.find(o => o.email === user?.email)` over the filtered response. Archived-only user → `matchedOwner` undefined → button disabled (correct edge case).
- **AC3 — `out_04` would now result in MATCH.** PASS. Appsmith SQL and migrated query are semantically identical.

### Code Quality Findings

None. Single-predicate addition, zero collateral changes, `archived` is NOT NULL (no NULL concern), `Owner` type correctly omits `archived`, no dead code.

### Recommendations

None. Change is minimal, correctly scoped, matches Appsmith exactly.
