# Fix Report 02

## Issue Summary

`issue_02.md` reported that the list-page `Elimina` action in `apps/quotes` was visually rendered (and correctly gated by the `app_quotes_delete` role) but not wired to the existing `useDeleteQuote` mutation. Selecting the menu item was a no-op, so the migrated implementation diverged from Appsmith's `eliminaOfferta()` flow (role check → optional HubSpot delete → DB delete → list refresh).

## Root Cause

`QuoteTable` rendered `<KebabMenu quoteId={q.id} canDelete={canDelete} />` without passing the optional `onDelete` prop. `KebabMenu` already called `onDelete?.()` on click, but because no callback was provided no network request was made. The `useDeleteQuote` hook existed in `apps/quotes/src/api/queries.ts` but had no caller.

## Changes Made

- `apps/quotes/src/components/QuoteTable.tsx`
  - Imported `useDeleteQuote`.
  - Instantiated the mutation in the component, added a `handleDelete(id)` callback that short-circuits when a delete is already pending and otherwise invokes `deleteQuote.mutate(id)`.
  - Passed `onDelete={() => handleDelete(q.id)}` to each row's `KebabMenu`, so the correct quote ID is always deleted regardless of menu reuse across rows.
  - Added an inline error banner above the table that appears when the mutation fails, displays the error message (or a localized fallback), and resets the mutation state via a dismiss button. The failing row remains visible because the list is only refreshed on success (through the existing `['quotes']` query invalidation inside `useDeleteQuote`).
- `apps/quotes/src/components/QuoteTable.module.css`
  - Added `.errorBar` and `.errorDismiss` styles for the new inline error banner.

No backend changes were needed: `handleDeleteQuote` in `backend/internal/quotes/handler_quotes.go` already enforces the `app_quotes_delete` role and performs the optional HubSpot delete before the DB delete, so the frontend only had to invoke the existing `DELETE /quotes/v1/quotes/:id` endpoint.

## Validation

- `pnpm --filter mrsmith-quotes exec tsc --noEmit` — clean, no output.
- `cd backend && go build ./...` — clean, no output.
- `cd backend && go vet ./internal/quotes/...` — clean, no output.

## Acceptance Criteria Check

- Selecting `Elimina` triggers the quote deletion flow. — PASS (`KebabMenu` invokes `onDelete`, which calls `deleteQuote.mutate(q.id)` bound to the current row's id).
- The visible list refreshes after successful deletion. — PASS (`useDeleteQuote.onSuccess` already invalidates `['quotes']`, so React Query refetches and the deleted quote disappears).
- Permission-based visibility of the delete action is preserved. — PASS (`canDelete` is still derived from `user.roles.includes('app_quotes_delete')` and `KebabMenu` still renders the `Elimina` item only when `canDelete` is true).
- Verification for `out_02` would now result in `MATCH`. — PASS (role-gated visibility + DB delete + list refresh are now wired; HubSpot deletion is handled atomically by the backend, which is a strict improvement over Appsmith's client-side sequencing).

## Notes

- The backend already handles the HubSpot delete (`handler_quotes.go:650`) before the DB delete, so there is no need to expose two separate endpoints from the frontend.
- On failure, the row stays visible because no optimistic removal is performed; the mutation's `error` is surfaced in a dismissable banner above the table. No toast infrastructure was introduced (none exists in the app today).
- No confirmation dialog was added because the Appsmith reference (`eliminaOfferta`) deletes immediately without confirmation; adding one would be an unrelated behavioral change.
- Concurrent multi-delete is guarded by the `deleteQuote.isPending` short-circuit; queued second clicks are ignored until the in-flight mutation resolves.

## QA Review

**Reviewer:** QA subagent
**Verdict:** PASS_WITH_NOTES
**Score:** 9/10

### Acceptance Criteria Verification

- **Selecting `Elimina` triggers the quote deletion flow** — PASS. `KebabMenu` calls `onDelete?.()` then `close()` on click. `QuoteTable` now passes `onDelete={() => handleDelete(q.id)}` to every row's `KebabMenu`. `handleDelete` calls `deleteQuote.mutate(id)`, which invokes `DELETE /quotes/v1/quotes/:id`. The closure correctly binds each row's `q.id` at render time.
- **The visible list refreshes after successful deletion** — PASS. `useDeleteQuote.onSuccess` invalidates `['quotes']`, so React Query refetches the list and the deleted row disappears.
- **Permission-based visibility of the delete action is preserved** — PASS. `canDelete` is still derived from `user.roles.includes('app_quotes_delete')` and passed through to `KebabMenu`, which renders `Elimina` only when true.
- **Verification for `out_02` would now result in `MATCH`** — PASS. End-to-end flow (role gate → network delete → list refresh) matches Appsmith's `eliminaOfferta()` sequence.

### Code Quality Findings

**Single mutation instance silently blocks concurrent row deletes (confidence: 82).** `QuoteTable` instantiates one `useDeleteQuote()` shared across rows. The `if (deleteQuote.isPending) return` guard prevents concurrent deletes but is silent: clicking `Elimina` on a second row while the first is in flight closes the menu and does nothing — no feedback, no error. This is a UX gap, not a correctness bug. The fix notes acknowledge the no-toast constraint and Appsmith does not handle this either; acceptable tradeoff.

No other high-confidence issues. Dead-code risk is zero, error handling via the dismissible `.errorBar` is correct (`deleteQuote.reset()` clears state), CSS additions minimal.

### Recommendations

- Consider an `aria-disabled` state on the `Elimina` button while `deleteQuote.isPending` is true, making the guard observable without toast infrastructure. Future improvement, not a blocker.
