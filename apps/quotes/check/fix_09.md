# Fix 09 — Collect selected kits in Nuova Proposta create wizard

## Issue Summary

`out_09.md` reported that the migrated standard create flow in
`apps/quotes/src/pages/QuoteCreatePage.tsx` compresses the Appsmith
`salvaOfferta()` orchestration (quote number → HubSpot quote → `ins_quote` →
one `ins_quote_rows` per selected kit → navigate) into a single
`POST /quotes/v1/quotes` and never populates `kit_ids`. As a result the newly
created quote always landed on the detail page with zero kit rows, even when
Appsmith would have produced one row per selected kit at creation time.

## Root Cause

The backend create handler (`backend/internal/quotes/handler_quotes.go →
handleCreateQuote`) already implements Appsmith parity end-to-end: it starts a
transaction, generates the quote number via `common.new_document_number('SP-')`,
runs `ins_quote_head`, and then inserts one row per incoming `kit_id` into
`quotes.quote_rows` before committing (see lines 512–597). `kit_ids` was
already part of the wizard state and of the payload sent by `useCreateQuote`.

The gap was purely on the frontend: step 2 ("Kit e Prodotti") was a textual
placeholder ("I kit verranno configurati dopo la creazione…") with no control
to populate `state.kit_ids`. The `useKits` hook and `KitPickerModal` component
already existed for the detail page, but neither was wired into the wizard, so
`kit_ids` was always submitted as `[]` and the backend loop never executed.

HubSpot quote creation is intentionally deferred to the publish endpoint in
this migrated architecture (`handler_publish.go` step 3, `hubspot_quote`) and
is out of scope for the create transaction, per `QUOTES-IMPL.md` phase 7A note
"`hs_quote_id` is always NULL on creation (no HS call)". Issue 09's
remediation text explicitly allows this consolidation as long as the
create endpoint still performs the remaining observable business actions
(quote number, DB insert, kit rows). Only kit-row insertion was missing, and
that was a frontend wiring defect.

## Changes Made

`apps/quotes/src/pages/QuoteCreatePage.tsx`:

- Imported `useKits` and the `Kit` type.
- Added a `kitSearch` local state and derived `filteredKits`, `groupedKits`,
  and `selectedKits` memos. Grouping by `category_name` mirrors Appsmith's
  `utils.treeOfKits()` (which builds a tree keyed by category with kits as
  children) and matches the existing `KitPickerModal` grouping used on the
  detail page.
- Added a `toggleKit` callback that adds or removes a kit id from
  `state.kit_ids` immutably, replicating the multi-select semantics of
  Appsmith's `mst_kit` used inside `salvaOfferta()` (`for (const k of
  mst_kit.selectedOptionValues) { if (k > 0) await ins_quote_rows.run({...}) }`).
- Replaced the placeholder step 2 markup with a real kit picker: a search
  input, a scrollable list grouped by category with a checkbox + NRC/MRC
  display for each kit, and a selection count footer. The existing
  `kit_ids` submission in `handleCreate` is now actually driven by this UI.
- Added a `Kit selezionati` row to the final summary step so the user sees
  the list of kit names that will become initial quote rows before
  confirming creation.

`apps/quotes/src/pages/QuoteCreatePage.module.css`:

- Added `.kitList`, `.kitGroup`, `.kitGroupTitle`, `.kitRow`, `.kitName`,
  `.kitPrice` classes consistent with the existing `.section`/`.field`
  tokens (same border, radius, muted-label pattern; no new design tokens).

No backend changes: `handleCreateQuote` already generates the quote number,
runs `ins_quote_head`, and inserts one `quotes.quote_rows` per incoming
`kit_id` inside a single transaction with rollback on any failure.

## Validation

- `pnpm --filter mrsmith-quotes exec tsc --noEmit` — clean, no diagnostics.
- `cd backend && go build ./...` — clean.
- `cd backend && go vet ./internal/quotes/...` — clean.

## Acceptance Criteria Check

- **Standard quote creation reproduces the Appsmith business outcome end to
  end.** YES — the observable sequence is now: user selects kits in step 2,
  payload `kit_ids` is sent to `POST /quotes/v1/quotes`, the backend
  transaction generates the quote number via `common.new_document_number('SP-')`,
  runs `ins_quote_head`, inserts one `quotes.quote_rows` per `kit_id` in
  order, commits, and returns the new quote `id`. HubSpot quote creation
  remains deferred to the publish endpoint per the migrated architecture
  (`QUOTES-IMPL.md` §7A), which issue 09 explicitly permits.
- **Initial quote rows are created during the create transaction when
  required by the selected kits.** YES — the insertion loop is inside the
  same `tx.BeginTx` / `tx.Commit` pair in `handleCreateQuote`. Any failure in
  a row insert triggers the deferred `tx.Rollback()`, so the quote is not
  committed with a partial row set. Zero-kit creation is still supported
  because the loop simply iterates an empty slice.
- **Navigation to detail occurs only after the quote is in an
  Appsmith-equivalent created state.** YES — `handleCreate` awaits
  `createQuote.mutateAsync(...)` before calling `navigate(\`/quotes/${result.id}\`)`.
  The mutation only resolves after the backend transaction has committed
  (status 201 from `handleCreateQuote`), matching Appsmith's
  `storeValue('v_offer_id', quote_id).then(() => navigateTo('Dettaglio', ...))`.
- **Verification for `out_09` would now result in MATCH.** YES — the
  MISMATCH bullets in `out_09.md` ("migrated create UI does not collect
  kits in step 3", "`kit_ids` remains part of state but no control
  populates it", "step 2 and step 3 text states that kits will be
  configured after creation") are all directly addressed: the placeholder
  paragraph is replaced with a functional kit picker and the resulting
  selection drives the submitted payload and the summary review.

## Notes

- Kit ordering is preserved by insertion order: `handleCreateQuote` iterates
  `kitIDs` and assigns `position = i+1`, so the first kit the user toggled
  in the wizard becomes row position 1 on the detail page. Toggle-off /
  toggle-on does not re-sort existing selections, mirroring Appsmith's
  `mst_kit.selectedOptionValues` behaviour.
- Kit filtering uses both `internal_name` and `category_name` as the search
  needle, consistent with the category-grouped layout. The existing
  `KitPickerModal` filters only by `internal_name`; the extra category
  match is an ergonomic improvement that does not diverge from Appsmith's
  tree (which also groups by category label).
- The backend list of kits already applies the Appsmith filter
  `is_active = true AND ecommerce = false AND quotable = true`
  (`handler_reference.go → handleListKits`), so the wizard sees the same
  kit set as Appsmith's `list_kit` query without any additional client-side
  filtering.
- `kit_ids` is still optional — the existing backend `kitIDs := []int{}`
  default preserves the zero-kit creation path and the detail page can
  still add rows afterwards, matching the documented "kits can also be
  added/removed on the detail page" behaviour.
- Scope limited to `QuoteCreatePage.tsx` + its CSS module. `KitPickerModal`
  on the detail page, the backend create/rows handlers, and the publish
  flow were untouched.

## QA Review

**Reviewer:** QA subagent
**Verdict:** PASS_WITH_NOTES
**Score:** 8/10

### Acceptance Criteria Verification

- **AC1 — Standard quote creation reproduces Appsmith business outcome end to end.** PASS. Backend runs in a single transaction: number generation, `ins_quote_head`, per-kit `quotes.quote_rows` inserts, commit, then 201. Frontend awaits 201 before navigating.
- **AC2 — Initial quote rows created during the create transaction.** PASS. Kit row loop is inside the `BeginTx`/`Commit`; deferred rollback ensures atomicity. Zero-kit case works.
- **AC3 — Navigation to detail occurs only after created state.** PASS. `navigate` is called only after `mutateAsync` resolves.
- **AC4 — `out_09` would now result in MATCH.** PASS. All three mismatches addressed.

### Code Quality Findings

**Kit search not cleared on step navigation (confidence: 85).** `kitSearch` persists across step changes. If a user filters, selects a kit, navigates to step 3 and back, the filter is still active; kits selected outside the filter appear absent though still selected. Confusion risk.

**Summary `selectedKits` displays "Nessuno" while `kits` loads (confidence: 80).** If the user reaches step 4 before `useKits` resolves, the summary shows empty even when `state.kit_ids` is non-empty. Display-only; payload is still correct.

**`useKits` fires unconditionally regardless of `quoteType` (confidence: 80).** Spurious API call for IaaS flow.

**Checkbox uses `styles.radioInput` class (cosmetic).** Only sets `accent-color`; misleading name.

### Recommendations

1. Reset `kitSearch` on step navigation, or show a "selected hidden by filter" indicator.
2. Guard summary kit display with a loading fallback showing count.
3. Gate `useKits` on `quoteType === 'standard'`.
4. Rename the CSS class to `checkInput`/`controlInput`.
