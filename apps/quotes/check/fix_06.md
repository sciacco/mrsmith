# Fix 06 — Customer-specific ERP default-payment lookup restored

## Issue Summary

Per `issue_06.md` (HIGH), the migrated `apps/quotes` create flow always started
from the static `payment_method: '402'` initial state and never invoked the
customer-specific ERP lookup. Appsmith's `metodoPagDefault` pattern instead
fetches the ERP payment code for the selected customer via
`get_pagamento_anagrCli` and only falls back to `402` when the lookup returns
no value.

`out_06.md` confirmed `MISMATCH`: the `useCustomerPayment` hook was defined in
`src/api/queries.ts` but had no consumer anywhere under `apps/quotes/src`.

## Root Cause

`QuoteCreatePage.tsx` initialized wizard state with `payment_method: '402'`
and never reacted to customer selection. The `useCustomerPayment` hook and its
backend endpoint (`GET /quotes/v1/customer-payment/{customerId}` in
`internal/quotes/handler_reference.go`) were fully implemented server-side and
in the client API layer, but the create flow never called the hook.

The backend endpoint already implements the authoritative fallback semantics:
when Alyante is unconfigured, when the HubSpot→ERP bridge row is missing, or
when `CODICE_PAGAMENTO` is null, it returns `{"payment_code": "402"}`. So the
client only needs to wire the hook into the create wizard and react to its
result.

## Changes Made

Single file touched: `apps/quotes/src/pages/QuoteCreatePage.tsx`.

1. Imported `useCustomerPayment` alongside the other query hooks.
2. Derived `selectedCustomerId` from `state.selectedDeal?.company_id`
   (converted to string because the hook signature expects `string | null`,
   while `Deal.company_id` is `number | null`).
3. Called `useCustomerPayment(selectedCustomerId)` — React Query automatically
   keys by the customer id, so switching deals re-runs the lookup and stale
   results are discarded by the normal query lifecycle.
4. Added a `useEffect` that, when a customer is selected and the query is no
   longer pending, updates `state.payment_method`:
   - when `data.payment_code` is present and non-empty, apply it;
   - otherwise (no data, empty string, or query error) explicitly fall back
     to `'402'`.
   The setter is guarded with a `prev.payment_method === nextCode` short-circuit
   to avoid redundant re-renders.

Initial state still sets `payment_method: '402'` so the field has a sensible
default before any deal is chosen (no deal selected = no lookup to run).
The payment-method `<select>` remains user-editable, so manual override after
the lookup still works.

No other files changed. No backend, type, or test changes were required.

## Validation

```
pnpm --filter mrsmith-quotes exec tsc --noEmit   # clean
cd backend && go build ./...                      # clean
cd backend && go vet ./internal/quotes/...        # clean
```

All three commands completed with no output (success).

## Acceptance Criteria Check

- [x] Quote creation derives the default payment method from the selected
      customer — `useCustomerPayment(selectedCustomerId)` is now wired into
      `QuoteCreatePage` and its result is applied to form state.
- [x] Fallback `402` is used only when the customer lookup has no value or
      fails — the effect checks `resolved && resolved !== ''` and otherwise
      writes `'402'`; `isError` is included in the dependency array so query
      failures also collapse to the fallback.
- [x] The submitted quote payload reflects the resolved payment method — the
      submit path already reads `state.payment_method` (unchanged), which now
      holds the resolved value.
- [x] Verification for `out_06` would now result in `MATCH`: the hook has an
      active consumer in the create flow, the trigger condition matches
      Appsmith (runs on customer selection), and the fallback behavior is
      preserved.

Edge cases covered:

- Multiple customer changes: React Query keys on `customerId`, so each new
  selection starts a fresh query and the effect always writes the latest
  result's code; stale responses from previous customers do not clobber the
  current form value.
- Lookup request failure: `isError` is a dependency; on failure the effect
  falls back to `'402'`, leaving the field defined and editable.
- No deal selected: the effect early-returns, leaving the initial `'402'`
  default untouched.

## Notes

- The backend endpoint already returns `{"payment_code": "402"}` as its
  documented fallback for missing ERP bridge, missing ERP row, and null
  `CODICE_PAGAMENTO`, so the frontend fallback is belt-and-braces. The
  frontend still explicitly coerces empty/missing values to `'402'` in case
  the payload ever comes back without a payment code.
- `selectedCustomerId` is stringified because `useCustomerPayment` expects
  `string | null` and constructs the URL path from it; `Deal.company_id` is
  typed as `number | null`. The backend path param is a HubSpot company id
  resolved against `loader.hubs_company.id`.
- No changes to `HeaderTab.tsx` (the detail page already binds the payment
  method to persisted quote data; only the create flow was missing the
  lookup).

## QA Review

**Reviewer:** QA subagent
**Verdict:** PASS
**Score:** 9/10

### Acceptance Criteria Verification

- **AC1 — Quote creation derives the default payment method from the selected customer.** PASS. `useCustomerPayment(selectedCustomerId)` is now wired into `QuoteCreatePage` and consumed in the effect.
- **AC2 — Fallback `402` used only when lookup is empty or fails.** PASS. Effect evaluates `resolved && resolved \!== ''`; falsy/empty/error collapses to `'402'`. `isError` is in deps to force re-run on failure.
- **AC3 — Submitted payload reflects resolved payment method.** PASS. `handleCreate` reads `state.payment_method`; the effect writes the resolved code before the user advances.
- **AC4 — `out_06` would now result in MATCH.** PASS. Sole cause of previous MISMATCH was zero consumers for `useCustomerPayment`; fix introduces the correct consumer with fallback semantics.

### Code Quality Findings

- `isError` dependency appears unused in the effect body but is intentional (forces re-run on failure). May trigger ESLint `exhaustive-deps` warning; no runtime impact.
- Race condition on rapid deal switching is handled correctly by React Query's keyed cache + `isPending` guard.
- Manual override is intentionally overwritten on customer resolution, matching issue_06 edge-case rule.

### Recommendations

- Add an inline comment on the `isError` dependency documenting why it's present despite not being read in the body.
- Pre-existing: the step-1 payment-method `<select>` has no `— Seleziona —` fallback, so if `'402'` is absent from `paymentMethods` the visible selection and state can diverge. Out-of-scope for this fix.
