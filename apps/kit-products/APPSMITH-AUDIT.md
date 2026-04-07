# Appsmith Audit: Kit and Products

> **Source:** `kit-and-products-main.zip` (Appsmith git export, 303 files)
> **Date:** 2026-04-07
> **Status:** Complete structural audit — ready for migration spec phase

---

## 1. Application Inventory

| Field | Value |
|-------|-------|
| **App name** | Kit and Products |
| **Layout** | FLUID, top-stacked navigation |
| **Pages** | 7 (Kit, Edit Kit, Products, Discount groups, Kit discounts, Kit Price Simulator, Categories) |
| **Default page** | Kit |
| **Hidden pages** | Edit Kit (navigated to programmatically) |
| **Datasources** | 3 (db-mistra, Alyante, GW internal CDLAN) |
| **JS Libraries** | xmlParser (fast-xml-parser 3.17.5) — appears unused by any page |
| **JSObjects** | 4 (Kit/utils, Products/utils, Discount groups/JSObject1, Kit discounts/utils) |
| **Total queries** | ~45 (SQL + REST) |

### Datasources

| Name | Plugin | Type | Used By |
|------|--------|------|---------|
| **db-mistra** | postgres-plugin | PostgreSQL | Kit, Edit Kit, Products, Discount groups, Kit discounts, Categories |
| **Alyante** | mssql-plugin | MS SQL Server (ERP) | Products (translation sync only) |
| **GW internal CDLAN** | restapi-plugin | REST API | Kit discounts, Kit Price Simulator |

### Navigation Flow

```
Kit (home) ──── "Edit Kit" button ──→ Edit Kit (hidden page, uses appsmith.store.v_kit_id)
     │
     ├── Products (tab in nav)
     ├── Discount groups (tab in nav)
     ├── Kit discounts (tab in nav)
     ├── Kit Price Simulator (tab in nav)
     └── Categories (tab in nav)
```

---

## 2. Database Schema Cross-Reference

### Primary Tables (products schema)

| Table | Purpose | Row Count | Used By Pages |
|-------|---------|-----------|---------------|
| `products.kit` | Kit (bundle) definitions | 83 | Kit, Edit Kit, Kit discounts |
| `products.product` | Individual products | 836 | Products, Edit Kit, Kit |
| `products.product_category` | Category lookup | 11 | Categories, Kit, Edit Kit, Products |
| `products.kit_product` | Kit ↔ Product junction (items) | 730 | Edit Kit |
| `products.kit_product_group` | Named product groups within kits | 4 | Edit Kit (vocabulary) |
| `products.kit_customer_group` | Kit ↔ Customer group discounts | 182 | Kit discounts |
| `products.kit_custom_value` | Custom key-value pairs on kits | 12 | Edit Kit |
| `products.kit_help` | Help URLs per kit | — | Kit |
| `products.asset_flow` | Asset flow type lookup | — | Products |

### Supporting Tables

| Table | Purpose | Used By |
|-------|---------|---------|
| `customers.customer_group` | Discount/commercial profile groups (5 rows) | Discount groups, Kit discounts, Kit, Edit Kit |
| `customers.customer` | Customer master (1,391 rows) | Kit discounts, Kit Price Simulator |
| `common.translation` | i18n texts (uuid, language, short, long) — 1,856 rows | Products, Edit Kit |
| `common.language` | Language codes (IT, EN) | Products, Edit Kit |
| `common.custom_field_key` | Custom field registry (3 rows) | Edit Kit |
| `common.vocabulary` | Lookup values by section | Edit Kit (kit_product_group) |

### Key Stored Functions

| Function | Purpose | Called From |
|----------|---------|------------|
| `products.new_kit(json)` | Create kit + translations + customer groups + products | Kit (new_kit query) |
| `products.clone_kit(id, name)` | Clone kit with all relationships | Kit (clone_kit query) |
| `products.upd_kit(id, json)` | Update kit metadata | Edit Kit |
| `products.upd_kit_product(id, json)` | Update kit-product relationship | Edit Kit |
| `products.new_kit_product(json)` | Add product to kit | Edit Kit |
| `common.get_translations(uuid)` | Return translations as JSON array | Products, Edit Kit |
| `common.upd_translation(uuid, json)` | Update translations | Edit Kit |

### External Systems

| System | Integration Point | Direction |
|--------|-------------------|-----------|
| **Alyante ERP** (MSSQL) | `MG87_ARTDESC` table — product short descriptions | Write-only (dual-write from Products page) |
| **GW internal CDLAN** (REST API) | `/products/v2/*`, `/customers/v2/*` endpoints | Read + Write (Kit discounts, Kit Price Simulator) |

---

## 3. Per-Page Audits

---

### 3.1 Kit (Home Page)

**Purpose:** Master list of all kits with buttons to create, edit, clone kits and manage help URLs.

**Queries (5 on load, 5 on demand):**

| Query | Datasource | SQL | On Load |
|-------|-----------|-----|---------|
| `get_kit` | db-mistra | `SELECT * FROM products.kit ORDER BY is_active::int desc, internal_name` | Yes |
| `get_category` | db-mistra | `SELECT name as label, id as value, color FROM products.product_category ORDER BY label` | Yes |
| `get_customer_group` | db-mistra | `SELECT name as label, id as value FROM customers.customer_group ORDER BY id` | Yes |
| `get_products` | db-mistra | `SELECT code as value, internal_name as label FROM products.product ORDER BY internal_name` | Yes |
| `get_kit_help` | db-mistra | `SELECT help_url, kit_id FROM products.kit_help WHERE kit_id = {{Table1.selectedRow.id \|\| -1}}` | Yes |
| `new_kit` | db-mistra | `SELECT products.new_kit('{{Form1.data}}')` | No |
| `clone_kit` | db-mistra | `SELECT products.clone_kit({{Table1.selectedRow.id}}, {{i_cloned_name.text}})` | No |
| `upd_kit_help` | db-mistra | `INSERT INTO products.kit_help ... ON CONFLICT (kit_id) DO UPDATE SET help_url = ...` | No |
| `get_product` | db-mistra | `SELECT * FROM products.product` | No — **UNUSED** |
| `get_product_category` | db-mistra | `SELECT * FROM products.product_category` | No — **UNUSED** |

**Widgets:**

| Widget | Type | Role |
|--------|------|------|
| `ButtonGroup1` | BUTTON_GROUP | Toolbar: Edit Kit / New Kit / More menu (Clone, Refresh, Delete, Help) |
| `Table1` | TABLE_V2 | Kit list, 15+ visible columns, category color-coded |
| `Modal1` | MODAL | New Kit form (name, prefix, category, main product, pricing, subscription terms, sellable groups, ecommerce) |
| `modal_clone` | MODAL | Clone kit (input for new name) |
| `description_modal` | MODAL | Help URL editor (input with HTTPS regex) |

**Event Flows:**
- **Page Load** → `get_kit`, `get_category`, `get_customer_group`, `get_products`, `get_kit_help` all execute
- **Row Select** → `get_kit_help.run()` fetches help URL for selected kit
- **Edit Kit button** → `storeValue('v_kit_id', id)` then `navigateTo('Edit Kit')`
- **New Kit submit** → `new_kit.run()` (stored procedure) → store returned ID → navigate to Edit Kit
- **Clone Kit confirm** → `clone_kit.run()` → refresh kit list
- **Help URL save** → `upd_kit_help.run()` (upsert)

**Hidden Logic:**
- `internal_name` column appends `(main_product_code)` via computed binding
- `category_id` column resolves to name + color via `get_category.data.find()` — **no null guard**
- `Edit Kit` and `Clone Kit` buttons disabled when `Table1.selectedRow.id == ''`

---

### 3.2 Edit Kit (Hidden Page)

**Purpose:** Detail editor for a single kit, navigated from Kit page. Three tabs: Details, Products, Custom Value.

**Entry:** Requires `appsmith.store.v_kit_id` (falls back to `id=2` if missing).

**JSObject: `utils`** — 7 functions:
- `saveRelatedProducts()` — batch-save inline table edits (sequential await loop)
- `updateKit()` — **dead code**, never called
- `ProductSelect()` — maps product data to select options
- `newKitProduct()` — add or edit product via modal (checks `v_kp_id == 'new'`)
- `writeKitCustomValues()` — save inline-edited custom value
- `newKitCustomValues()` — insert new custom value via add-row
- `populateDefaults()` — pre-fill modal fields from selected row
- `test1()` — **dead code**, debug function

**Queries (12 on load, 5 on demand):**

| Query | SQL Summary | On Load |
|-------|-------------|---------|
| `get_kit_by_id` | `SELECT * FROM products.kit WHERE id = {{store.v_kit_id\|\|2}}` | Yes |
| `get_category` | Categories for dropdown | Yes |
| `get_customer_group` | Customer groups for multi-select | Yes |
| `get_products` | Products for dropdown (code, name) | Yes |
| `get_all_products` | `SELECT * FROM products.product` — **redundant with get_products** | Yes |
| `get_kit_product` | Kit's related products with join | Yes |
| `get_kit_translations` | `SELECT common.get_translations(uuid)` | Yes |
| `get_kit_custom_value` | Custom values for kit | Yes |
| `get_custom_field_keys` | Custom field key registry | Yes |
| `get_vocabulary_kit_group` | Vocabulary for product group names | Yes |
| `get_cg_id` | Sellable customer group IDs for kit | Yes |
| `upd_kit` | `SELECT products.upd_kit(id, json)` | No |
| `upd_kit_product` | `SELECT products.upd_kit_product(id, json)` | No |
| `new_kit_product` | `SELECT products.new_kit_product(json)` | No |
| `delete_kit_product` | `DELETE FROM products.kit_product WHERE id = ...` — **confirmBeforeExecute: true** | No |
| `upd_translation` | `SELECT common.upd_translation(uuid, json)` — **raw mustache** | No |
| `upd_kit_custom_value` | UPDATE products.kit_custom_value | No |
| `new_kit_custom_value` | INSERT INTO products.kit_custom_value | No |

**Tab 1 — Details:**
- Form with 16+ fields bound to `get_kit_by_id.data[0]`
- `bundle_prefix` always disabled on existing kits (`id > 0`)
- Billing period: static select (Mensile/Bimestrale/.../Biennale)
- `ms_sellable_to`: multi-select loaded from `get_cg_id` (sellable customer groups)
- Translation table with inline editing (short, long per language)
- **Save Kit** → `upd_kit.run({kit_id, kit_json: Form1.data})`
- **Save Translations** → `upd_translation.run({uuid, json: updatedRows})`

**Tab 2 — Products:**
- Table `tbl_related` with inline editing (group_name, min, max, required, nrc, mrc, position)
- Toolbar: Add (+), Edit (pencil), Save (cloud-upload), Refresh, Delete (trash), Back
- Add/Edit modal (`mdl_product`) with product select, group, quantities, pricing
- `newKitProduct()` handles both create and update based on `v_kp_id` store value

**Tab 3 — Custom Value:**
- Table with inline editing + add-new-row for key-value pairs
- `key_name` is a select from `custom_field_key`, `value` is JSON text

---

### 3.3 Products

**Purpose:** CRUD for individual products. Browse table with inline editing, create via modal, edit translations via modal with dual-write to Postgres + Alyante ERP.

**JSObject: `utils`** — 5 functions:
- `saveNewProduct()` — insert product + translations via `ins_product`
- `rigaSelezionata()` — extract IT/EN translations from selected row
- `salvaDescrizioni()` — upsert translations to Postgres + sync short descriptions to Alyante
- `salvaRiga()` — save inline table edits
- `test()` — **dead code**

**Queries (3 on load, 5 on demand):**

| Query | Datasource | SQL Summary | On Load |
|-------|-----------|-------------|---------|
| `get_products` | db-mistra | Products with category join + `common.get_translations()` | Yes |
| `get_category` | db-mistra | Categories for select | Yes |
| `get_asset_flow` | db-mistra | Asset flows for select | Yes |
| `get_translations` | db-mistra | `SELECT * FROM common.translation WHERE uuid = data[0].uuid` | No — **UNUSED** |
| `ins_product` | db-mistra | INSERT product + 2x INSERT translation (IT, EN) | No |
| `ins_translation` | db-mistra | UPSERT common.translation | No |
| `upd_products` | db-mistra | UPDATE products.product | No |
| `upd_translation_alyante` | Alyante | UPDATE MG87_ARTDESC (code padded to 25 chars, lang ITA/ING) | No |

**Widgets:**

| Widget | Type | Role |
|--------|------|------|
| `tbl_products` | TABLE_V2 | Main table, inline editing (name, nrc, mrc, category select, asset_flow select, erp_sync, img_url) |
| `IconButton2` | ICON_BUTTON | "+" New product → opens `mdl_product` |
| `Button3` | BUTTON | "Edit descriptions" → opens `mdl_descriptions` (disabled when no row selected) |
| `mdl_product` | MODAL | New product form (code, name, category, nrc, mrc, erp_sync, img_url, descriptions IT/EN, asset_flow) |
| `mdl_descriptions` | MODAL | Edit descriptions form (short/long IT/EN) with dual-write |

**Key Bindings:**
- Table `category` select options: `value: i.name` (name-based), form `category` options: `value: i.id` (ID-based) — asymmetry handled by `salvaRiga()` lookup
- `translations` column hidden but used by `rigaSelezionata()` to populate description form
- `category_id`, `translation_uuid` columns hidden

**Alyante Dual-Write Flow:**
```
salvaDescrizioni() →
  ins_translation(uuid, 'it', short, long)    → Postgres
  ins_translation(uuid, 'en', short, long)    → Postgres
  upd_translation_alyante(code, 'ITA', short) → Alyante MSSQL
  upd_translation_alyante(code, 'ING', short) → Alyante MSSQL
```
Note: Only short descriptions sync to Alyante; long descriptions are Postgres-only. Initial `ins_product` does NOT write to Alyante.

---

### 3.4 Discount Groups

**Purpose:** CRUD for customer discount groups (`customers.customer_group`). Inline editing with batch save, add-new-row for inserts.

**JSObject: `JSObject1`** — 1 function:
- `salvaModifiche()` — loop over `updatedRows`, call `upd_customer_group` sequentially, refresh

**Queries:**

| Query | SQL | On Load |
|-------|-----|---------|
| `get_customer_groups` | `SELECT id, name, is_default, is_partner, read_only FROM customers.customer_group ORDER BY name` | Yes |
| `upd_customer_group` | `UPDATE customers.customer_group SET name = ..., is_partner = ... WHERE id = ...` | No |
| `ins_customer_group` | `INSERT INTO customers.customer_group (name, is_partner) VALUES (...)` | No |

**Widgets:**
- `tbl_groups` — TABLE_V2 with inline editing. `name` and `is_partner` editable only when `!currentRow["read_only"]`. Hidden: `id`, `is_default`
- `IconButton1` — Save button (cloud-upload), disabled when no pending edits

**Business Rule:** `read_only` flag prevents editing protected groups.

---

### 3.5 Kit Discounts

**Purpose:** Manage per-kit, per-customer-group discount rules. Master-detail: left table = kits, right table = discount associations. Add/edit discounts via modals.

**JSObject: `utils`** — 3 functions + 1 variable:
- `gruppi_non_presenti: []` — reactive state for unassigned groups
- `salvaModifiche()` — **dead code** (hidden save button, references non-existent fields)
- `nuovoGruppo()` — filter out already-assigned groups, open add modal
- `setDiscount()` — auto-fill MRC discount from group's `base_discount`

**Queries (mix of SQL + REST):**

| Query | Type | Purpose | On Load |
|-------|------|---------|---------|
| `GetAllKit` | REST GET | `/products/v2/kit` — kit list | Yes |
| `get_customer_groups` | SQL | Customer groups for dropdowns | Yes |
| `get_customer` | SQL | `SELECT * FROM customers.customer` (for kit details modal) | Yes |
| `GetAllKitDiscountsById` | REST GET | `/products/v2/kit-discount?kit_id=...` — discounts for selected kit | Yes |
| `GetDiscountedKitById` | REST GET | `/products/v2/discounted-kit/{id}?customer_id=...` — price preview | Yes |
| `NewKitDiscount` | REST POST | `/products/v2/kit-discount` — create/update (upsert) | No |
| `get_kit` | SQL | — **UNUSED** | No |
| `GetAllKitDiscounts` | REST GET | — **UNUSED** | No |
| `GetKit` | REST GET | — **UNUSED** | No |
| `get_kit_customer_group` | SQL | — **UNUSED** (only in dead salvaModifiche) | No |
| `ins_kit_group_discount` | SQL | — **UNUSED** (legacy, replaced by REST) | No |
| `upd_kit_group_discount` | SQL | — **UNUSED** (legacy, replaced by REST) | No |

**Widgets:**

| Widget | Type | Role |
|--------|------|------|
| `tbl_kit` | TABLE_V2 | Left panel — kit list from API (visible: internal_name, category) |
| `tbl_cgroups` | TABLE_V2 | Right panel — discount groups for selected kit (derived columns for nested mrc/nrc) |
| `IconButton1` | ICON_BUTTON | Save — **HIDDEN** (dead UI) |
| `IconButton3` | ICON_BUTTON | Add new group association (+) |
| `mdl_associate_new_grp` | MODAL | Add new discount: group select, MRC/NRC percentage + sign (+/-), rounding switch |
| `mdl_edit_discount` | MODAL | Edit existing discount: same fields, pre-populated from selected row |
| `kit_details` | MODAL | Price preview: customer select → discounted related products table |

**Key Bindings:**
- NRC defaults sync to MRC values (both sign and percentage)
- Max discount validation: capped at 100% for discounts (sign=-), uncapped for surcharges (sign=+)
- `NewKitDiscount` body uses structured JSON: `{kit_id, customer_group_id, sellable, use_int_rounding, mrc: {percentage, sign}, nrc: {percentage, sign}}`

---

### 3.6 Kit Price Simulator

**Purpose:** Read-only price simulation. Select customer → view all discounted kits → select kit → view related products with per-product pricing. No write operations.

**Queries (all REST, datasource: GW internal CDLAN):**

| Query | Method | Path | On Load |
|-------|--------|------|---------|
| `GetAllCustomer` | GET | `/customers/v2/customer?disable_pagination=true` | Yes |
| `GetAllDiscountedKit` | GET | `/products/v2/discounted-kit?customer_id={{sl_customer.selectedOptionValue}}` | Yes |
| `GetDiscountedKitDetails` | GET | `/products/v2/discounted-kit/{{tbl_discounted_kits.selectedRow.id}}?customer_id=...` | Yes |

**Widgets:**

| Widget | Type | Role |
|--------|------|------|
| `sl_customer` | SELECT | Customer dropdown (filterable), `onOptionChange → GetAllDiscountedKit.run()` |
| `tbl_discounted_kits` | TABLE_V2 | Kit list with derived nrc/mrc columns from nested `base_price` object |
| `Text1` | TEXT | "Related Products for Kit '{{selectedRow.internal_name}}'" |
| `tbl_discounted_rp` | TABLE_V2 | Related products flattened from `related_products[].products[]` with group denormalization |

**Key Bindings:**
- `base_price` column hidden; derived columns extract `.nrc` and `.mrc` via IIFE pattern
- Related products table uses `.flatMap()` to flatten nested groups → products structure
- `group_required` and `img_url` fetched but hidden

---

### 3.7 Categories

**Purpose:** Simple CRUD for product categories (`products.product_category`). Inline editing + add-new-row. No delete.

**Queries:**

| Query | SQL | On Load |
|-------|-----|---------|
| `get_category` | `SELECT * FROM products.product_category ORDER BY name` | Yes |
| `ins_category` | `INSERT INTO products.product_category (name, color) VALUES (...)` | No |
| `upd_category` | `UPDATE products.product_category SET name = ..., color = ... WHERE id = ...` | No |

**Widgets:**
- `Table1` — TABLE_V2, inline editing on `name` (required) and `color`, hidden `id` column
- `EditActions1` — Save/Discard per row, disabled via `updatedRowIndices.includes()` check
- `allowAddNewRow: true` with `onAddNewRowSave → ins_category.run() → get_category.run()`

---

## 4. Datasource & Query Catalog

### 4.1 db-mistra (PostgreSQL) — 30 queries

| Page | Query | Operation | Table(s) | Rewrite Recommendation |
|------|-------|-----------|----------|----------------------|
| Kit | get_kit | SELECT | products.kit | Backend API |
| Kit | get_category | SELECT | products.product_category | Backend API (shared) |
| Kit | get_customer_group | SELECT | customers.customer_group | Backend API (shared) |
| Kit | get_products | SELECT | products.product | Backend API (shared) |
| Kit | get_kit_help | SELECT | products.kit_help | Backend API |
| Kit | new_kit | FUNCTION | products.new_kit() | Backend API |
| Kit | clone_kit | FUNCTION | products.clone_kit() | Backend API |
| Kit | upd_kit_help | UPSERT | products.kit_help | Backend API |
| Edit Kit | get_kit_by_id | SELECT | products.kit | Backend API |
| Edit Kit | get_kit_product | SELECT | products.kit_product + product | Backend API |
| Edit Kit | get_kit_translations | FUNCTION | common.get_translations() | Backend API |
| Edit Kit | get_kit_custom_value | SELECT | products.kit_custom_value | Backend API |
| Edit Kit | get_custom_field_keys | SELECT | common.custom_field_key | Backend API |
| Edit Kit | get_vocabulary_kit_group | SELECT | common.vocabulary | Backend API |
| Edit Kit | get_cg_id | SELECT | products.kit_customer_group | Backend API |
| Edit Kit | upd_kit | FUNCTION | products.upd_kit() | Backend API |
| Edit Kit | upd_kit_product | FUNCTION | products.upd_kit_product() | Backend API |
| Edit Kit | new_kit_product | FUNCTION | products.new_kit_product() | Backend API |
| Edit Kit | delete_kit_product | DELETE | products.kit_product | Backend API |
| Edit Kit | upd_translation | FUNCTION | common.upd_translation() | Backend API |
| Edit Kit | upd_kit_custom_value | UPDATE | products.kit_custom_value | Backend API |
| Edit Kit | new_kit_custom_value | INSERT | products.kit_custom_value | Backend API |
| Products | get_products | SELECT | products.product + category + translations | Backend API |
| Products | ins_product | INSERT | products.product + common.translation | Backend API |
| Products | ins_translation | UPSERT | common.translation | Backend API |
| Products | upd_products | UPDATE | products.product | Backend API |
| Discount groups | get_customer_groups | SELECT | customers.customer_group | Backend API |
| Discount groups | upd_customer_group | UPDATE | customers.customer_group | Backend API |
| Discount groups | ins_customer_group | INSERT | customers.customer_group | Backend API |
| Categories | get_category | SELECT | products.product_category | Backend API |
| Categories | ins_category | INSERT | products.product_category | Backend API |
| Categories | upd_category | UPDATE | products.product_category | Backend API |

### 4.2 Alyante (MSSQL) — 1 query

| Page | Query | Operation | Table | Rewrite Recommendation |
|------|-------|-----------|-------|----------------------|
| Products | upd_translation_alyante | UPDATE | MG87_ARTDESC | Backend API (ERP sync should be server-side) |

### 4.3 GW internal CDLAN (REST API) — 8 queries

| Page | Query | Method | Endpoint | Rewrite Recommendation |
|------|-------|--------|----------|----------------------|
| Kit discounts | GetAllKit | GET | /products/v2/kit | Frontend → existing API |
| Kit discounts | GetAllKitDiscountsById | GET | /products/v2/kit-discount | Frontend → existing API |
| Kit discounts | GetDiscountedKitById | GET | /products/v2/discounted-kit/{id} | Frontend → existing API |
| Kit discounts | NewKitDiscount | POST | /products/v2/kit-discount | Frontend → existing API |
| Kit Price Simulator | GetAllCustomer | GET | /customers/v2/customer | Frontend → existing API |
| Kit Price Simulator | GetAllDiscountedKit | GET | /products/v2/discounted-kit | Frontend → existing API |
| Kit Price Simulator | GetDiscountedKitDetails | GET | /products/v2/discounted-kit/{id} | Frontend → existing API |

---

## 5. Findings Summary

### 5.1 Bugs

| # | Page | Severity | Description |
|---|------|----------|-------------|
| B1 | Products | **High** | `saveNewProduct()` passes `asset_flow_name` but `ins_product` binds `{{this.params.asset_flow}}` — new products get NULL asset_flow |
| B2 | Products | Medium | `saveNewProduct()` does not refresh table or close modal after insert |
| B3 | Kit discounts | Medium | `edit_discount_group.onOptionChange` calls `utils.setDiscount()` which references `sl_group` (wrong modal's widget) |
| B4 | Kit | Low | Category column `get_category.data.find(...)` has no null guard — throws if category_id missing |
| B5 | Kit | Low | Help URL regex `^https://:*` is malformed (`:*` = zero or more colons) |
| B6 | Edit Kit | Low | `.then(await get_kit_product.run())` in `newKitProduct()` executes immediately instead of chaining |
| B7 | Discount groups | Low | `showAlert` passed directly to `.then()` instead of as callback — fires before query resolves |

### 5.2 Dead Code

| # | Page | Item | Notes |
|---|------|------|-------|
| D1 | Kit | `get_product`, `get_product_category` queries | Never referenced by any widget or JS |
| D2 | Kit | "Delete" menu item | onClick is empty string |
| D3 | Edit Kit | `utils.updateKit()` | Never called; `btn_save_kit` calls `upd_kit.run()` directly |
| D4 | Edit Kit | `utils.test1()` | Debug function |
| D5 | Edit Kit | `get_all_products` | Redundant with `get_products`, only used by `ProductSelect()` |
| D6 | Edit Kit | `Text3` (hidden debug widget) | Shows `Form1.data` — leftover |
| D7 | Products | `get_translations` query | Hardcoded to `data[0]`, never called |
| D8 | Products | `utils.test()` | Debug function |
| D9 | Kit discounts | `IconButton1` (save button) | `isVisible: false`, `utils.salvaModifiche()` references non-existent fields |
| D10 | Kit discounts | `get_kit`, `GetAllKitDiscounts`, `GetKit` queries | Never referenced |
| D11 | Kit discounts | `ins_kit_group_discount`, `upd_kit_group_discount` SQL queries | Legacy — replaced by REST API `NewKitDiscount` |
| D12 | Kit discounts | `get_kit_customer_group` SQL query | Only called from dead `salvaModifiche()` |
| D13 | Kit discounts | `Button4` ("Confirm") in kit_details modal | No onClick handler |
| D14 | Discount groups | `base_discount` in JSObject payload | Passed but never used in SQL |

### 5.3 Security Concerns

| # | Page | Risk | Description |
|---|------|------|-------------|
| S1 | Kit | SQL Injection | `new_kit` uses raw mustache interpolation (`pluginSpecifiedTemplates.value: false`) — `Form1.data` embedded directly in SQL string |
| S2 | Edit Kit | SQL Injection | `upd_translation` uses raw mustache — translation JSON with single quotes breaks/injects SQL |
| S3 | All | Direct DB Access | 30+ queries execute directly against production database from the browser — no backend validation layer |
| S4 | Products | ERP Direct Access | Alyante MSSQL writes happen directly from UI without server-side validation |

### 5.4 Embedded Business Rules

| # | Location | Rule | Classification |
|---|----------|------|---------------|
| BR1 | Products `salvaDescrizioni()` | Dual-write: translations saved to Postgres AND Alyante ERP; only short descriptions sync to ERP | Business logic |
| BR2 | Products `upd_translation_alyante` | Product code padded to 25 chars with spaces for Alyante; language mapped IT→ITA, EN→ING | Business logic |
| BR3 | Kit discounts `mdl_associate_new_grp` | NRC defaults sync to MRC values (sign + percentage) | Business logic |
| BR4 | Kit discounts modal | Max discount capped at 100% for discounts, uncapped for surcharges | Business logic |
| BR5 | Kit discounts `nuovoGruppo()` | Filter out already-assigned groups when adding new association | Business logic |
| BR6 | Kit discounts `setDiscount()` | Auto-fill MRC discount from group's `base_discount` | Business logic |
| BR7 | Discount groups | `read_only` flag prevents editing protected customer groups | Business logic |
| BR8 | Edit Kit | `bundle_prefix` not editable after kit creation | Business logic |
| BR9 | Kit `new_kit` | On kit creation, store returned ID and navigate to Edit Kit | UI orchestration |
| BR10 | Kit discounts `NewKitDiscount` | Both create and update use same POST endpoint (upsert semantics) | Business logic |
| BR11 | Products `ins_product` | On product creation, auto-create IT+EN translation rows with empty long description | Business logic |
| BR12 | Products `salvaRiga()` | Category resolved by name→id lookup; asset_flow resolved by label→name lookup | UI orchestration |
| BR13 | Edit Kit `newKitProduct()` | Add vs edit determined by `appsmith.store.v_kp_id == 'new'` | UI orchestration |
| BR14 | Kit list `Table1` | internal_name display appends `(main_product_code)` | Presentation |
| BR15 | Kit list `Table1` | Category cells color-coded by `product_category.color` | Presentation |

### 5.5 Duplication

| # | Description |
|---|-------------|
| DUP1 | `get_category` query duplicated across Kit, Edit Kit, Products, Categories (4 copies) |
| DUP2 | `get_customer_group(s)` query duplicated across Kit, Edit Kit, Discount groups, Kit discounts (4 copies) |
| DUP3 | `get_products` query duplicated across Kit and Edit Kit |
| DUP4 | Products flattening logic (`related_products.flatMap(group → products)`) duplicated in Kit Price Simulator and Kit discounts |
| DUP5 | Sequential batch-save pattern (`for...in updatedRows { await query.run() }`) duplicated across Edit Kit, Discount groups, Kit discounts |
| DUP6 | `mdl_associate_new_grp` and `mdl_edit_discount` modals in Kit discounts are near-identical layouts |

### 5.6 Migration Blockers & Risks

| # | Risk | Impact | Mitigation |
|---|------|--------|------------|
| R1 | Stored procedure dependency | `new_kit`, `clone_kit`, `upd_kit`, `upd_kit_product`, `new_kit_product`, `upd_translation` all call PG functions — business logic lives in the database | Must audit stored procedures for business rules before backend API design |
| R2 | Alyante ERP dual-write | Product descriptions sync to external MSSQL — must replicate in backend with proper error handling and possible retry/queue | Backend service with ERP adapter |
| R3 | REST API already exists | Kit discounts and Kit Price Simulator already use `/products/v2/*` and `/customers/v2/*` REST APIs | These APIs should be reused; verify coverage for direct-SQL pages |
| R4 | `appsmith.store` for navigation state | `v_kit_id` and `v_kp_id` stored globally — must map to URL params or React state | Use React Router params |
| R5 | xmlParser JS library | Loaded but appears unused — verify no runtime dependency | Safe to drop |
| R6 | Translation system is dual-DB | `common.translation` (Postgres) + `MG87_ARTDESC` (Alyante) must stay in sync | Server-side sync service recommended |

---

## 6. Candidate Domain Entities

Based on the audit, the following domain entities emerge:

| Entity | Source Tables | CRUD Pages |
|--------|-------------|------------|
| **Kit** | `products.kit`, `products.kit_help` | Kit (list/create/clone), Edit Kit (detail) |
| **KitProduct** | `products.kit_product`, `products.kit_product_group` | Edit Kit (Products tab) |
| **Product** | `products.product`, `products.asset_flow` | Products |
| **ProductCategory** | `products.product_category` | Categories |
| **Translation** | `common.translation`, `common.language` | Products (descriptions), Edit Kit (translations) |
| **CustomerGroup** | `customers.customer_group` | Discount groups |
| **KitDiscount** | `products.kit_customer_group` | Kit discounts |
| **KitCustomValue** | `products.kit_custom_value`, `common.custom_field_key` | Edit Kit (Custom Value tab) |
| **Customer** | `customers.customer` | Kit discounts, Kit Price Simulator (read-only) |

---

## 7. Recommended Next Steps

1. **Hand off to `appsmith-migration-spec`** — this audit is the input for the expert-led specification phase
2. **Audit stored procedures** — `products.new_kit`, `upd_kit`, `clone_kit`, `upd_kit_product`, `new_kit_product`, `common.upd_translation` contain business logic not visible in the UI
3. **Verify REST API coverage** — check if `/products/v2/*` and `/customers/v2/*` endpoints already cover the direct-SQL operations, reducing backend work
4. **Design ERP sync strategy** — the Alyante dual-write must move server-side with proper error handling
5. **Consolidate shared lookups** — categories, customer groups, products, and translations are fetched by multiple pages; design shared API endpoints
6. **Fix bugs before migration** — B1 (asset_flow param mismatch) and B3 (wrong modal reference) should be fixed in the Appsmith app or noted as known issues to avoid replicating
