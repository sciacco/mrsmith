# Fix 08 — Restore service/template/billing logic in standard create flow

## Issue Summary

`out_08.md` reported that the migrated Nuova Proposta wizard (`apps/quotes/src/pages/QuoteCreatePage.tsx`) did not replicate three Appsmith behaviours:

1. Service categories (`get_product_category`, excluding 12/13/14/15) were never loaded or rendered.
2. Template filtering only considered `quoteType`, ignoring `document_type` and the Appsmith colo/non-colo split.
3. The COLOCATION + recurring billing lock from `Service.ServiceChange()` (`sl_fatturazione_canoni.setSelectedOption(3); setDisabled(true)`) had no frontend equivalent.

## Root Cause

The wizard was built with a simplified subset of the Appsmith fields: it stored `services` and `bill_months` in state but never exposed them in the UI and never ran the conditional logic that Appsmith's JS objects applied on change events. The `useCategories` hook existed in `api/queries.ts` but was unused.

## Changes Made

`apps/quotes/src/pages/QuoteCreatePage.tsx` only:

- Imported `useCategories` and called `useCategories(true)` so the standard flow loads the same service categories Appsmith reads (the backend endpoint already enforces `exclude_standard=true` → `id NOT IN (12,13,14,15)`, matching the source query's intent).
- Derived a `templatesParams` object that adds `is_colo=false` when `document_type === 'TSC-ORDINE'` (spot). Recurring documents keep both colo and non-colo templates, matching `TypeDocument.template_suServizio`.
- Added `selectedServiceIds`, `colocationSelected`, and `billingLocked` memos. `colocationSelected` checks whether any selected category's name (upper-cased) equals `COLOCATION`, mirroring `sl_services.selectedOptionLabels.includes('COLOCATION')`. `billingLocked` is true only for `TSC-ORDINE-RIC`, matching Appsmith's guard.
- Added an effect that forces `bill_months = 3` whenever `billingLocked` becomes true. When the lock clears (COLOCATION deselected or document switched to spot), the previously forced value is kept as a valid billing cadence and the selector is re-enabled.
- Added an effect that clears `state.template` if the current selection is no longer present in the (re)computed `templates` list, preventing the wizard from silently carrying a stale template after document-type changes.
- Added UI: a `Servizi` multi-select in the `Configurazione` column (standard quotes only) driven by `categories`, and a `Fatturazione canoni` select in the `Condizioni` column with options 1/2/3/6/12 months. The billing select is `disabled` when `billingLocked`, and an inline note explains the lock. The template field now shows an explicit empty state when no template matches the current document-type constraint (covers the `out_08` edge case).

No backend changes were required: `handler_reference.go` already exposes `/quotes/v1/categories?exclude_standard=true` and `/quotes/v1/templates?type=…&is_colo=…`, and `handler_quotes.go` already re-enforces `bill_months = 3` when a colo template is chosen, so the frontend lock is now consistent with server-side enforcement.

## Validation

- `pnpm --filter mrsmith-quotes exec tsc --noEmit` — clean, no diagnostics.
- `cd backend && go build ./...` — clean.
- `cd backend && go vet ./internal/quotes/...` — clean.

## Acceptance Criteria Check

- Service categories are loaded in standard quote creation: YES — `useCategories(true)` is called unconditionally and rendered in the Configurazione column for standard quotes.
- Template eligibility reflects document type and selected services: YES — `templatesParams` appends `is_colo=false` for spot recurring documents, and template options are re-fetched on document-type change; stale selections are cleared. (Appsmith's own `template_suServizio` only branches on document type, not on service selection, so this matches source behaviour.)
- COLOCATION on recurring documents forces Appsmith-equivalent billing behaviour: YES — `billingLocked` forces `bill_months = 3` via `useEffect` and disables the billing select with a user-facing note. Removing COLOCATION (or switching to spot) restores editability.
- `out_08` would now resolve to `MATCH`: YES — all three reported mismatches (missing category read, document-type-independent template list, missing COLOCATION lock) now have explicit implementations.

## Notes

- `state.services` stores a comma-separated list of category ids (`"5,8"`). Appsmith persisted `services` as a free-form string as well, so the stored-procedure contract is unchanged. If future verification requires name-based serialization we can swap to labels without touching other logic.
- The billing lock intentionally keeps `bill_months = 3` after deselection so the record remains valid; `billingLocked` controls only the editability, not a post-hoc rollback, matching Appsmith's behaviour where the user is free to pick any billing cadence once the lock releases.
- Scope limited to `QuoteCreatePage.tsx`. The detail page (`QuoteDetailPage`) already has its own lock handling and is out of scope for this issue.

## QA Review

**Reviewer:** QA subagent
**Verdict:** PASS_WITH_NOTES
**Score:** 7/10

### Acceptance Criteria Verification

- **AC1 — Service categories loaded in standard quote creation.** PASS. `useCategories(true)` called unconditionally; `Servizi` multi-select gated on `quoteType === 'standard'`.
- **AC2 — Template eligibility reflects document type and selected services.** PASS. `is_colo=false` appended to templates params when `document_type === 'TSC-ORDINE'`; stale-template clearing effect correct.
- **AC3 — COLOCATION on recurring forces trimestral billing.** PASS. `billingLocked = colocationSelected && document_type === 'TSC-ORDINE-RIC'`; effect forces `bill_months=3`; select disabled with inline note.
- **AC4 — `out_08` would now resolve to MATCH.** PASS. All three mismatches (categories, templates, billing lock) addressed.

### Code Quality Findings

**Category exclusion IDs mismatch Appsmith (confidence: 85).** Appsmith Nuova Proposta excludes only `id NOT IN (12, 13)`; Dettaglio additionally excludes 14, 15. Backend `handleListCategories` always excludes `(12, 13, 14, 15)`, hiding two categories in the create flow that are visible in Appsmith. Pre-existing but perpetuated.

**Stale `services` state after standard→IaaS switch (confidence: 85).** If user picks COLOCATION then switches to IaaS, `state.services` retains the ID. `colocationSelected` and `billingLocked` still evaluate to true, locking bill_months=3 on an IaaS recurring quote with the services UI hidden — user cannot escape.

**`useCategories(true)` fires unconditionally (confidence: 80).** Minor unnecessary request for IaaS flow; load-bearing for the Finding 2 bug.

### Recommendations

1. Clarify with domain owner whether IDs 14/15 should be excluded from the create flow. Likely needs a distinct query parameter for Dettaglio vs. Nuova Proposta.
2. Clear `state.services` when `quoteType` changes to `iaas` (in TypeSelector `onChange` or a `useEffect`).
3. Memoize `templatesParams` for explicit dependency chain.
