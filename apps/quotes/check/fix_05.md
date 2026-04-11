# Fix Report: issue_05 — Align payment-method list filtering and ordering with Appsmith

## Issue Summary

The migrated `/quotes/v1/payment-methods` endpoint did not provably mirror the Appsmith `get_payment_method` query, which restricts rows to selectable-only payment methods and orders them by description. Appsmith SQL:

```sql
SELECT cod_pagamento, desc_pagamento FROM loader.erp_metodi_pagamento
where selezionabile is true
order by desc_pagamento;
```

Severity 2, Importance 4, Priority MEDIUM.

## Root Cause

`backend/internal/quotes/handler_reference.go` `handleListPaymentMethods` issued:

```sql
SELECT codice, descrizione FROM loader.erp_metodi_pagamento ORDER BY descrizione
```

Two defects:

1. **Wrong column names**: `loader.erp_metodi_pagamento` exposes `cod_pagamento` and `desc_pagamento` per the Mistra schema (`docs/mistradb/mistra_loader.json`, entry `erp_metodi_pagamento`). The previous identifiers `codice`/`descrizione` do not exist on that table.
2. **Missing selectable filter**: no `WHERE selezionabile IS TRUE` clause, so non-selectable methods could be returned and the result set would differ from Appsmith semantics.

Ordering by description was already intended but it was sorting on a non-existent column.

## Changes Made

- `backend/internal/quotes/handler_reference.go` (`handleListPaymentMethods`): replaced the query literal with the Appsmith-aligned form:

  ```sql
  SELECT cod_pagamento, desc_pagamento FROM loader.erp_metodi_pagamento
  WHERE selezionabile IS TRUE
  ORDER BY desc_pagamento
  ```

The response shape is unchanged: rows are still scanned into `{code, description}` and serialized as the normalized `PaymentMethod[]` JSON contract consumed by `apps/quotes/src/api/queries.ts` `usePaymentMethods` and by `QuoteCreatePage.tsx` / `HeaderTab.tsx`. No frontend changes were required; column ordinals already matched the scan targets.

No imports were added or removed. No `fmt.Sprintf` usage was introduced; the SQL is a literal constant.

## Validation

- `pnpm --filter mrsmith-quotes exec tsc --noEmit` — clean (no output).
- `go build ./...` (from `backend/`) — clean.
- `go vet ./internal/quotes/...` — clean.

## Acceptance Criteria Check

- [x] Payment-method options match Appsmith selectable-only behavior — backend now applies `WHERE selezionabile IS TRUE`.
- [x] Option ordering matches Appsmith description ordering — backend now orders by `desc_pagamento` (the real column), which is the field serialized as `description` and rendered by the UI.
- [x] `out_05` verification would now result in `MATCH` — SQL fields, filter, and ordering all mirror the Appsmith query verbatim.

## Notes

- The fix is backend-only and preserves the normalized `{code, description}` API contract, so callers in `apps/quotes/src/api/queries.ts`, `QuoteCreatePage.tsx`, and `HeaderTab.tsx` remain unchanged.
- The previous query with non-existent column names would have errored at runtime on any real `loader` database; correcting to the schema-accurate identifiers is a necessary side-effect of the fix and not a scope expansion.
- No frontend safeguard filter/sort was added; the remediation guidance prefers backend enforcement and the backend is now authoritative.

## QA Review

**Reviewer:** QA subagent
**Verdict:** PASS
**Score:** 9/10

### Acceptance Criteria Verification

- **AC1 — Payment-method options match Appsmith selectable-only behavior.** PASS. `handleListPaymentMethods` now includes `WHERE selezionabile IS TRUE`. Schema confirms `selezionabile` is a real boolean column on `loader.erp_metodi_pagamento`.
- **AC2 — Option ordering matches Appsmith description ordering.** PASS. Query orders by `desc_pagamento` (actual column name per schema). Previous query was ordering by non-existent alias `descrizione` — double correction.
- **AC3 — `out_05` would now result in MATCH.** PASS. Migrated SQL is character-equivalent to Appsmith's `select cod_pagamento, desc_pagamento from loader.erp_metodi_pagamento where selezionabile is true order by desc_pagamento`.

### Code Quality Findings

- Schema-verified column names (`cod_pagamento`, `desc_pagamento`); scan targets align positionally.
- No frontend changes required: `{code, description}` contract preserved.
- No new imports, no new `fmt.Sprintf`. Pure SQL literal replacement.
- `IS TRUE` is correct for nullable boolean in PostgreSQL — excludes `false` AND `NULL`, matching Appsmith intent.

### Recommendations

None blocking. Future hardening: `selezionabile` is nullable with default `false`; a `NOT NULL` schema constraint would make intent explicit, but this is out-of-scope schema evolution.
