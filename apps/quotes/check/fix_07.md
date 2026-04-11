# Fix 07 — Replacement-order loaded selection for SOSTITUZIONE

## Issue Summary

Per `issue_07.md` (MEDIUM), the migrated `apps/quotes` app captured
`replace_orders` as a free-text input in both the detail page and the create
wizard, instead of replicating the Appsmith behaviour that loads the Alyante
order list and lets the user pick from that dataset. `out_07.md` confirmed
`MISMATCH`: the `useCustomerOrders` hook existed in `src/api/queries.ts` but
had no consumer, and `HeaderTab.tsx` rendered `replace_orders` as a plain
`<input type="text" ... />`.

## Root Cause

- `apps/quotes/src/components/HeaderTab.tsx` rendered the Sostituzione field as
  a free-text input and did not import `useCustomerOrders`.
- `apps/quotes/src/pages/QuoteCreatePage.tsx` had no UI at all for
  `replace_orders` during the Sostituzione flow (the field was silently kept
  at the initial empty string and posted on create).

The backing infrastructure was already in place:
- The backend endpoint `GET /quotes/v1/customer-orders/{customerId}` in
  `backend/internal/quotes/handler_reference.go` queries Alyante
  `Tsmi_Ordini` filtered by the resolved ERP `NUMERO_AZIENDA` and returns
  `[{name: string}]` (fixing Appsmith bug A7 by scoping to the customer).
- The client hook `useCustomerOrders(customerId)` in
  `apps/quotes/src/api/queries.ts` already targets that endpoint with the
  correct enable condition.

So only the UI wiring needed to be added.

## Changes Made

Appsmith reference (`quotes-main/pages/Dettaglio/widgets/Tabs1/frm_offerta/i_replace_orders.json`):
the widget is a `MULTI_SELECT_WIDGET_V2` bound to `cli_orders.data` with
`optionValue = NOME_TESTATA_ORDINE`. The persisted value is produced in
`mainForm.js` via `i_replace_orders.selectedOptionValues.join(';')`, so the
DB column is a `;`-separated string. The fix preserves that storage format.

### 1. `apps/quotes/src/components/HeaderTab.tsx`

- Imported `useCustomerOrders`.
- Resolved a `customerIdStr` from `quote.customer_id` (nullable number → string
  for the hook signature) and called `useCustomerOrders(customerIdStr)`.
- Parsed the stored `replace_orders` string on `;` (trimming empty entries)
  into `selectedOrders: string[]`.
- Replaced the free-text input with a native `<select multiple>` whose
  `value` is `selectedOrders`, populated from `customerOrders`. On change,
  the selected option values are joined with `;` and pushed through the
  existing `onChange('replace_orders', ...)` callback — no changes to the
  save pipeline were needed.
- Added a legacy-value preservation branch: any stored token that is not
  present in the loaded list is still rendered as a selectable option with a
  `(legacy)` suffix, so editing an existing quote never silently drops an
  unknown stored value.
- Handled the empty-list edge case by disabling the control and rendering a
  small inline hint ("Nessun ordine disponibile per il cliente.").

### 2. `apps/quotes/src/components/HeaderTab.module.css`

- Bumped the `revealSlide` keyframe `max-height` from `100px` to `200px` so
  the multi-select is not clipped by the reveal animation.
- Added an `.emptyHint` utility class used by the empty-state message.

### 3. `apps/quotes/src/pages/QuoteCreatePage.tsx`

- Imported `useCustomerOrders` and called it with the existing
  `selectedCustomerId` (already derived from the selected Deal's
  `company_id`).
- Added a conditional field block directly under the `Tipo proposta` radios
  that mirrors the Detail page: a native `<select multiple>` populated from
  the loaded orders, with the same `;`-join persistence and the same empty
  state hint. The wizard already initialised `replace_orders: ''` and already
  posted `replace_orders: state.replace_orders` in the create payload, so no
  changes to the save path were needed.

## Validation

- `pnpm --filter mrsmith-quotes exec tsc --noEmit` — clean (no output).
- `go build ./...` — clean.
- `go vet ./internal/quotes/...` — clean.

No unrelated files were touched; no existing Go handlers, SQL, or types were
modified.

## Acceptance Criteria Check

1. **Replacement orders for `SOSTITUZIONE` are loaded from the source dataset.**
   Both `HeaderTab` (detail) and `QuoteCreatePage` (wizard) now call
   `useCustomerOrders(customerId)` and populate the selection control from
   the returned list, which comes from the backend's Alyante-backed
   `GET /quotes/v1/customer-orders/{customerId}`.
2. **The migrated UI no longer relies on unrestricted free text for this field.**
   The free-text input in `HeaderTab` was replaced with a multi-select, and
   the create wizard gained a multi-select (it previously had no UI at all
   for this field and kept the initial empty string). Users can no longer
   type arbitrary values.
3. **Verification for `out_07` would now result in `MATCH`.**
   The Appsmith `cli_orders` read is replicated via `useCustomerOrders`, and
   the UI mirrors the Appsmith `MULTI_SELECT_WIDGET_V2` behaviour with the
   same `;`-separated persistence format.

## Notes

- Stored values are preserved verbatim: the code round-trips the
  `;`-separated string without normalising or re-ordering it, and any
  stored token absent from the loaded list is surfaced as a selectable
  `(legacy)` option so existing quotes remain editable without data loss.
- The backend filter scopes orders by customer (`NUMERO_AZIENDA`), which
  intentionally improves on Appsmith's unscoped query (bug A7 in the spec)
  while still returning values the user could have selected for this
  customer in Alyante.
- A proper styled multi-select UI (search, chips) was intentionally avoided
  to keep scope minimal; the native `<select multiple>` reuses the existing
  `.input` class for visual consistency with neighbouring controls.

## QA Review

**Reviewer:** QA subagent
**Verdict:** PASS_WITH_NOTES
**Score:** 8/10

### Acceptance Criteria Verification

- **AC1 — Replacement orders for `SOSTITUZIONE` loaded from source dataset.** PASS. Both `HeaderTab.tsx` and `QuoteCreatePage.tsx` call `useCustomerOrders(customerId)` and populate `<select multiple>` from the backend.
- **AC2 — No unrestricted free text.** PASS. Both locations now use a constrained multi-select.
- **AC3 — `out_07` would now result in MATCH.** PASS. `cli_orders` read path and `MULTI_SELECT_WIDGET_V2` behavior replicated; `;`-join persistence matches Appsmith.

### Code Quality Findings

**Backend column name divergence `NOME` vs `NOME_TESTATA_ORDINE` (confidence: 85).** `handler_reference.go` queries `NOME` but Appsmith SQL uses `NOME_TESTATA_ORDINE`, and the `STATO_ORDINE IN ('Evaso','Confermato')` filter is also absent. If these are distinct columns, stored Appsmith values will not match loaded options and always render as `(legacy)`. Pre-existing in the handler but not acknowledged in fix_07.

**`replace_orders` not cleared when `proposal_type` changes away from `SOSTITUZIONE` (confidence: 82).** In `QuoteCreatePage.tsx`, if a user picks orders then switches to `NUOVO`/`RINNOVO`, the multi-select disappears but `state.replace_orders` retains its value and is submitted. Data correctness concern.

**Inline style in wizard empty-hint (confidence: 80).** `QuoteCreatePage.tsx` uses raw `style={{...}}` for the empty-state text while `HeaderTab.tsx` uses the `styles.emptyHint` CSS module class. Inconsistent.

### Recommendations

1. Verify `Tsmi_Ordini` column — if `NOME_TESTATA_ORDINE` is correct, update the backend query and restore the `STATO_ORDINE` filter.
2. Reset `replace_orders` to `''` when `proposal_type` changes away from `SOSTITUZIONE`.
3. Replace inline style with a CSS module class.
