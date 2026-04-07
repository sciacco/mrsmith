# Phase C: Logic Placement

> Extracted from `APPSMITH-AUDIT.md` — Kit and Products application
> Status: **DRAFT — awaiting expert review**

---

## Classification Key

| Category | Definition | Target |
|----------|-----------|--------|
| **Domain** | Business rules, data integrity, cross-system contracts | Backend (Go) |
| **Orchestration** | Multi-step flows, chaining queries, state management | Backend API or frontend service layer |
| **Presentation** | Display formatting, UI state, widget behavior | Frontend (React) |

---

## JSObject Methods

### Products/utils

| Method | Current Logic | Classification | Recommended Placement | Notes |
|--------|--------------|----------------|----------------------|-------|
| `saveNewProduct()` | Collects form data, calls `ins_product.run()` | Orchestration | **Backend API**: `POST /kit-products/v1/product` | Backend should handle product + translation auto-creation atomically. **Fix bug B1** (asset_flow param mismatch). Backend should also close the "no refresh" gap (B2) by having the API return the created entity. |
| `rigaSelezionata()` | Extracts IT/EN translations from `selectedRow.translations` JSON array | Presentation | **Frontend** | Pure data extraction for form population. Stays in React component. |
| `salvaDescrizioni()` | Upserts IT/EN to Postgres, then syncs short descriptions to Alyante with code-padding and language mapping | **Domain** | **Backend API**: `PUT /kit-products/v1/product/{code}/translations` | **Critical**: Alyante dual-write (BR1, BR2) must move server-side. Backend handles: Postgres upsert + Alyante sync + code padding (25 chars) + language mapping (it→ITA, en→ING). Frontend sends `{translations: [{language, short, long}]}`. |
| `salvaRiga()` | Reads updatedRow, resolves category name→id and asset_flow label→name, calls `upd_products.run()` | Orchestration | **Backend API**: `PUT /kit-products/v1/product/{code}` | Backend accepts IDs directly (not names). Frontend sends `{category_id, asset_flow}` as proper identifiers. Eliminates the name→id lookup hack (BR12). |
| `test()` | Debug function, pads string to 25 chars | Dead code | **Delete** | |

### Edit Kit/utils

| Method | Current Logic | Classification | Recommended Placement | Notes |
|--------|--------------|----------------|----------------------|-------|
| `saveRelatedProducts()` | Loops over `updatedRows`, calls `upd_kit_product` sequentially per row | Orchestration | **Backend API**: `PATCH /kit-products/v1/kit/{kitId}/products` (batch) | Replace sequential loop with single batch endpoint. Backend handles transaction. |
| `updateKit()` | Escapes single quotes in notes, calls `upd_kit` | Dead code | **Delete** | Never called (D3). |
| `ProductSelect()` | Maps product rows to `{label, value}` for select widget | Presentation | **Frontend** | Select option mapping stays in React. But eliminate redundant `get_all_products` query — use the same product list endpoint. |
| `newKitProduct()` | Checks `v_kp_id == 'new'` to decide create vs update, calls appropriate stored procedure | Orchestration | **Backend API**: `POST /kit-products/v1/kit/{kitId}/products` (create) + `PUT /kit-products/v1/kit/{kitId}/products/{id}` (update) | Separate create/update endpoints. No more client-side routing by store flag. **Fix `.then(await ...)` bug (B6)**. |
| `writeKitCustomValues()` | Reads updatedRow, calls `upd_kit_custom_value` | Orchestration | **Backend API**: `PUT /kit-products/v1/kit/{kitId}/custom-values/{id}` | Simple proxy to DB update. |
| `newKitCustomValues()` | Reads newRow, calls `new_kit_custom_value` | Orchestration | **Backend API**: `POST /kit-products/v1/kit/{kitId}/custom-values` | Simple proxy to DB insert. |
| `populateDefaults()` | Pre-fills 9 form fields from `tbl_related.selectedRow` | Presentation | **Frontend** | Form default population. Stays in React component. |
| `test1()` | Maps custom field keys to select options | Dead code | **Delete** | |

### Discount groups/JSObject1

| Method | Current Logic | Classification | Recommended Placement | Notes |
|--------|--------------|----------------|----------------------|-------|
| `salvaModifiche()` | Loops over `updatedRows`, calls `upd_customer_group` per row, includes unused `base_discount` | Orchestration | **Backend API**: `PATCH /kit-products/v1/customer-groups` (batch) or individual `PUT /kit-products/v1/customer-groups/{id}` | Remove dead `base_discount` param (D14). **Fix `.then(showAlert)` bug (B7)** — frontend handles toast after API response. |

### Kit discounts/utils

| Method | Current Logic | Classification | Recommended Placement | Notes |
|--------|--------------|----------------|----------------------|-------|
| `salvaModifiche()` | Loops over updatedRows, calls `NewKitDiscount` per row | Dead code | **Delete** | Hidden save button (D9), references non-existent fields. |
| `nuovoGruppo()` | Filters customer groups to find unassigned ones for a kit | **Domain** | **Backend API** (preferred) or **Frontend** | Backend should return unassigned groups: `GET /kit-products/v1/kit/{kitId}/available-groups`. Avoids downloading all groups to client then filtering. But could stay frontend if data is small (5 groups). |
| `setDiscount()` | Auto-fills MRC discount from group's `base_discount` | **Domain** | **Frontend** (with backend data) | The auto-fill rule (BR6) is a UX convenience. Frontend reads `base_discount` from group data and pre-fills the form. Backend provides the field in the customer-group list response. |

---

## Inline Binding Expressions

### Business rules embedded in UI

| # | Location | Expression | Classification | Recommended Placement |
|---|----------|-----------|----------------|----------------------|
| BR3 | Kit discounts: `sl_nrc_sign` default | `sl_mrc_sign.selectedOptionValue` | Domain | **Frontend** — NRC defaulting to MRC is a form convenience, not a constraint. Backend should validate independently. |
| BR4 | Kit discounts: `i_mrc_discount` maxNum | `(sl_mrc_sign.selectedOptionValue == '+') ? null : 100` | Domain | **Frontend** validation + **Backend** validation. Max 100% for discounts is a business rule; backend must enforce it. The DB trigger `trg_validate_discount` already ensures >= -1. |
| BR7 | Discount groups: `tbl_groups` isCellEditable | `!currentRow["read_only"]` | Domain | **Backend**: read_only groups should return a flag; **Frontend** disables editing based on flag. Backend rejects updates to read_only groups. |
| BR8 | Edit Kit: `f_bundle_prefix` isDisabled | `get_kit_by_id.data[0].id > 0` | Domain | **Backend**: reject bundle_prefix changes on existing kits. **Frontend**: disable field. |

### Presentation-only expressions

| # | Location | Expression | Classification |
|---|----------|-----------|----------------|
| BR14 | Kit list: `internal_name` computedValue | `currentRow["internal_name"] + ' (' + currentRow["main_product_code"] + ')'` | Presentation — frontend only |
| BR15 | Kit list: `category_id` cellBackground | `get_category.data.find(cat => cat.value === currentRow["category_id"]).color` | Presentation — frontend only |
| — | Kit discounts: `tbl_cgroups` derived columns | `currentRow["mrc"].percentage`, `currentRow["customer_group"].name` | Presentation — frontend only |
| — | Kit Price Simulator: `tbl_discounted_rp` tableData | `.flatMap(group => group.products.map(...))` | Presentation — frontend only (flattening nested API response) |
| — | Kit Price Simulator: `tbl_discounted_kits` customColumns | IIFE to extract `base_price.nrc` / `base_price.mrc` | Presentation — frontend only |

### Save/Discard button state

| Location | Expression | Classification |
|----------|-----------|----------------|
| Products, Categories: `isSaveDisabled` | `!Table.updatedRowIndices.includes(currentIndex)` | Presentation — React dirty-tracking |
| Discount groups: `IconButton1` isDisabled | `tbl_groups.updatedRows.length == 0` | Presentation — React dirty-tracking |
| Edit Kit: `IconButton7` isDisabled | `tbl_related.updatedRowIndices.length == 0` | Presentation — React dirty-tracking |

All save/discard state management is pure presentation — React form/table dirty-state tracking.

---

## Stored Procedures — Logic to Audit

These PG functions contain business logic that is **not visible in the Appsmith export**. They must be audited before backend implementation:

| Function | Called From | What We Know | What We Don't Know |
|----------|------------|-------------|-------------------|
| `products.new_kit(json)` | Kit (create) | Creates kit + translations + customer group associations atomically | Exact field mapping, validation rules, default generation |
| `products.clone_kit(id, name)` | Kit (clone) | Deep-clones kit + kit_products + custom_values | Whether it clones translations, customer groups, help_url |
| `products.upd_kit(id, json)` | Edit Kit (save) | Updates kit metadata from form JSON | Which fields it accepts, validation, side effects |
| `products.upd_kit_product(id, json)` | Edit Kit (save product) | Updates a kit-product relationship | Accepted fields, constraints |
| `products.new_kit_product(json)` | Edit Kit (add product) | Creates a kit-product relationship | Duplicate handling, position auto-assignment |
| `common.upd_translation(uuid, json)` | Edit Kit (save translations) | Updates translation rows from JSON array | Whether it handles add/delete or only update |

**Q29.** These stored procedures are black boxes from the audit perspective. **Can you provide their source code**, or should the backend re-implement the logic from the field/table evidence?

---

## Logic Being Revised (Not Just Ported)

These are current behaviors that should **change** in the new app:

| Current Behavior | Problem | Proposed Change |
|-----------------|---------|-----------------|
| Sequential `for...in` loop for batch saves (DUP5) | Slow, no transaction, partial failure leaves inconsistent state | Single batch API call with server-side transaction |
| Alyante dual-write from browser (S4) | Security, no error handling, no retry | Server-side ERP sync with error handling |
| Direct DB access from browser (S3) | 30+ queries directly against production DB | All access through Go backend API |
| Raw SQL interpolation (S1, S2) | SQL injection risk | Parameterized queries in backend |
| Name→ID lookups in client (BR12) | Fragile, breaks if names change | Frontend sends IDs directly; backend accepts IDs |
| `appsmith.store` for navigation state (R4) | Global mutable state, breaks on direct URL | React Router URL params (`/kits/:kitId`) |
| `asset_flow` free text on create, select on edit | Inconsistent UX + possible invalid values | Always use select/dropdown from lookup data |
| No post-create refresh/close (B2) | Table stale after insert | API returns created entity; frontend refreshes + closes modal |

---

## Summary: Logic Allocation

| Layer | Responsibilities |
|-------|-----------------|
| **Backend (Go)** | All DB access, stored procedure calls, Alyante ERP sync, batch operations in transactions, business rule validation (read_only guard, bundle_prefix immutability, discount limits, nested resource ownership), translation auto-creation on product insert |
| **Frontend (React)** | Form state management, dirty tracking, select option mapping, display formatting (category colors, name+code display), NRC-defaults-to-MRC convenience, form population from selected row, table column rendering, `.flatMap()` for nested API responses |
| **Shared** | Entity types/interfaces (TypeScript types generated from API contract), validation schemas (field lengths, required fields, numeric ranges) |

---

## Questions for Expert Review

**Q29.** (Repeated from above) Stored procedures are black boxes. **Can you provide their source code** for `new_kit`, `clone_kit`, `upd_kit`, `upd_kit_product`, `new_kit_product`, `upd_translation`? Or should the backend re-implement from evidence?

**Q30.** The Alyante dual-write currently only syncs **short descriptions** and only on the "Edit descriptions" flow (not on product creation). **Is this intentional, or should product creation also trigger an Alyante write?** And should long descriptions ever sync to Alyante?

### Expert Decisions Recorded

- **Q33 → Opzione B (Postgres-first, Alyante best-effort):** Postgres writes always succeed. If Alyante is unreachable, log the error and surface a warning to the user ("Salvato, ma sincronizzazione ERP fallita"). A retry/sync-pending mechanism should be considered. The user is never blocked by ERP unavailability.
- **Q10 → Opzione A (single atomic endpoint):** Kit creation stays as a single `POST /kit-products/v1/kit` that creates the kit + default translations atomically (replicating or calling `new_kit`). The existing flow "create → navigate to Edit Kit" is confirmed.

**Q31.** The batch save pattern (DUP5) currently allows **partial success** — if row 3 of 5 fails, rows 1-2 are already committed. **Should the new app use all-or-nothing transactions?**

**Q32.** `nuovoGruppo()` filters available groups client-side. With only 5 customer groups this is fine. **But will the number of customer groups grow significantly?** If yes, the filtering should move to a backend endpoint.
