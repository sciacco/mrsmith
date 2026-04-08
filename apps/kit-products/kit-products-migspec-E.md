# Kit and Products — Application Specification

## Summary

| Field | Value |
|-------|-------|
| **Application name** | Kit and Products |
| **Audit source** | `apps/kit-products/APPSMITH-AUDIT.md` (303-file Appsmith export) |
| **Spec status** | Complete — ready for implementation planning |
| **Last updated** | 2026-04-07 |
| **Coexistence constraint** | Must run alongside existing Appsmith app during transition — same DB, no schema changes |

---

## Current-State Evidence

- **Source:** 7 Appsmith pages, 3 datasources (Postgres, MSSQL, REST API), 4 JSObjects, ~45 queries
- **Entities:** 9 managed + 3 lookups + 1 computed view
- **Integrations:** db-mistra (Postgres), Alyante ERP (MSSQL), Mistra NG REST API (via GW internal CDLAN)
- **Audit gaps:** 1 dead modal (kit_details in Kit Discounts — no trigger), 14 dead code items, 7 bugs documented
- **Stored procedures:** All 6 found and analyzed in `docs/mistradb/mistra_products.json` and `docs/mistradb/mistra_common.json`

---

## Entity Catalog

### Entity: Kit

- **Purpose:** Product bundle definition — the central entity of the app
- **Source table:** `products.kit` + `products.kit_help` (1:1 separate table, kept for coexistence)
- **Primary key:** `id` (bigint, auto-generated from sequence)

**Operations:**

| Operation | API | Implementation |
|-----------|-----|----------------|
| List | `GET /kit-products/v1/kit` | Direct SQL: `SELECT * FROM products.kit ORDER BY is_active::int desc, internal_name` |
| GetById | `GET /kit-products/v1/kit/{id}` | Direct SQL with related translations, customer groups, help URL |
| Create | `POST /kit-products/v1/kit` | Call stored procedure `products.new_kit(json)` — atomic (kit + default translations). Returns new kit ID. |
| Update | `PUT /kit-products/v1/kit/{id}` | Call stored procedure `products.upd_kit(id, json)` — updates all fields + DELETE/re-INSERT of kit_customer_group associations |
| SoftDelete | `DELETE /kit-products/v1/kit/{id}` | `UPDATE products.kit SET is_active = false WHERE id = ...` |
| Clone | `POST /kit-products/v1/kit/{id}/clone` | Call stored procedure `products.clone_kit(id, name)` — deep-clones kit + kit_products + kit_custom_values. Does NOT clone kit_customer_group. bundle_prefix = new internal_name. |
| GetHelpUrl | Included in GetById response | `SELECT help_url FROM products.kit_help WHERE kit_id = ...` |
| UpsertHelpUrl | `PUT /kit-products/v1/kit/{id}/help` | `INSERT ... ON CONFLICT (kit_id) DO UPDATE SET help_url = ..., updated_at = CURRENT_TIMESTAMP` |

**Fields:**

| Field | Type | Nullable | Default | Editable | Notes |
|-------|------|----------|---------|----------|-------|
| id | bigint | no | sequence | no | PK |
| internal_name | varchar(255) | no | '' | yes | Unique |
| main_product_code | varchar(32) | no | — | yes | FK → Product.code |
| category_id | integer | no | — | yes | FK → ProductCategory.id |
| bundle_prefix | varchar(64) | yes | — | **create-only** | Immutable after creation |
| initial_subscription_months | smallint | no | 0 | yes | Form default: 12 |
| next_subscription_months | smallint | no | 0 | yes | Form default: 12 |
| activation_time_days | integer | no | 0 | yes | Form default: 30 |
| nrc | numeric(14,5) | no | 0 | yes | Non-recurring charge |
| mrc | numeric(14,5) | no | 0 | yes | Monthly recurring charge |
| translation_uuid | uuid | no | gen_random_uuid() | no | Auto-generated, links to Translation |
| ecommerce | boolean | no | true | yes | |
| is_active | boolean | no | true | yes | Soft-delete target |
| is_main_prd_sellable | boolean | yes | true | yes | |
| quotable | boolean | yes | true | yes | |
| billing_period | integer | no | 3 | yes | Static enum: 1,2,3,4,6,12,24 |
| sconto_massimo | numeric(5,2) | no | 0 | yes | Informational — no enforcement |
| variable_billing | boolean | no | false | yes | |
| h24_assurance | boolean | no | false | yes | |
| sla_resolution_hours | integer | no | 0 | yes | |
| notes | text | yes | '' | yes | |

**Relationships:**
- Kit → 1 ProductCategory (category_id)
- Kit → 1 Product as main product (main_product_code)
- Kit → N KitProduct (kit items, CASCADE delete)
- Kit → N KitDiscount (customer group associations, CASCADE delete)
- Kit → N KitCustomValue (custom key-value pairs)
- Kit → 1 Translation (via translation_uuid)
- Kit → 0..1 HelpUrl (separate table `kit_help`)
- Kit → N SellableCustomerGroup (via `upd_kit` DELETE/re-INSERT pattern)

---

### Entity: KitProduct

- **Purpose:** Junction table linking kits to their component products with per-kit pricing overrides
- **Source table:** `products.kit_product`
- **Primary key:** `id` (bigint, auto-generated)

**Operations:**

| Operation | API | Implementation |
|-----------|-----|----------------|
| ListByKit | `GET /kit-products/v1/kit/{kitId}/products` | SQL join with `products.product` for internal_name, ordered by position/group/name |
| Create | `POST /kit-products/v1/kit/{kitId}/products` | Call `products.new_kit_product(json)` |
| Update | `PUT /kit-products/v1/kit/{kitId}/products/{id}` | Call `products.upd_kit_product(id, json)` |
| BatchUpdate | `PATCH /kit-products/v1/kit/{kitId}/products` | Loop `upd_kit_product` per item in a single DB transaction |
| Delete | `DELETE /kit-products/v1/kit/{kitId}/products/{id}` | `DELETE FROM products.kit_product WHERE id = ... AND kit_id = ...` (verify parent ownership) |

**Fields:**

| Field | Type | Nullable | Default | Notes |
|-------|------|----------|---------|-------|
| id | bigint | no | sequence | PK |
| kit_id | bigint | no | — | FK → Kit.id (CASCADE) |
| product_code | varchar(32) | no | — | FK → Product.code |
| group_name | varchar(64) | yes | — | From `common.vocabulary` (section=kit_product_group) |
| minimum | integer | no | 0 | |
| maximum | integer | no | -1 | -1 = unlimited |
| required | boolean | no | false | |
| nrc | double | no | 0 | Override price |
| mrc | double | no | 0 | Override price |
| position | integer | no | 0 | Display order |
| notes | text | yes | — | |

---

### Entity: Product

- **Purpose:** Individual product definition with ERP-synced translations
- **Source table:** `products.product`
- **Primary key:** `code` (varchar(32), **user-assigned** — not auto-generated)

**Operations:**

| Operation | API | Implementation |
|-----------|-----|----------------|
| List | `GET /kit-products/v1/product` | SQL join with category + `common.get_translations(uuid)` |
| Create | `POST /kit-products/v1/product` | INSERT product + INSERT IT/EN translation rows (short=empty, long=empty). No Alyante write on creation. |
| Update | `PUT /kit-products/v1/product/{code}` | UPDATE products.product. Frontend sends category_id and asset_flow directly (no name→id lookup). |
| UpdateTranslations | `PUT /kit-products/v1/product/{code}/translations` | **Dual-write (Postgres-first, Alyante best-effort):** 1) UPSERT common.translation for IT+EN. 2) UPDATE Alyante MG87_ARTDESC for short descriptions only (code padded 25 chars, lang ITA/ING). If Alyante fails: log error, return warning, Postgres commit stands. |

**Fields:**

| Field | Type | Nullable | Default | Notes |
|-------|------|----------|---------|-------|
| code | varchar(32) | no | — | PK, user-assigned. Padded to 25 chars for Alyante. |
| internal_name | varchar(255) | no | '' | |
| category_id | integer | no | — | FK → ProductCategory.id |
| translation_uuid | uuid | no | gen_random_uuid() | Auto-generated on insert |
| nrc | numeric(14,5) | no | 0 | List price |
| mrc | numeric(14,5) | no | 0 | List price |
| img_url | varchar(255) | yes | — | |
| erp_sync | boolean | yes | true | Controls external job behavior — no in-app side effect |
| asset_flow | varchar(50) | yes | — | FK → AssetFlow.name |

---

### Entity: ProductCategory

- **Purpose:** Lookup table for product/kit categories with display color
- **Source table:** `products.product_category`
- **Primary key:** `id` (integer, auto-generated)

**Operations:**

| Operation | API | Implementation |
|-----------|-----|----------------|
| List | `GET /kit-products/v1/category` | `SELECT * FROM products.product_category ORDER BY name` |
| Create | `POST /kit-products/v1/category` | INSERT (name, color) |
| Update | `PUT /kit-products/v1/category/{id}` | UPDATE name, color |

**Fields:** `id` (PK), `name` (varchar(64), required), `color` (varchar(12), default '#231F20')

No delete — FK-referenced by Kit and Product.

---

### Entity: Translation

- **Purpose:** Centralized i18n texts for products and kits
- **Source table:** `common.translation` + `common.language`
- **Primary key:** composite (`translation_uuid`, `language`)
- **Languages:** IT, EN (static, 2 rows in `common.language`)

**Operations:**

| Operation | API | Implementation |
|-----------|-----|----------------|
| GetByUuid | Embedded in parent entity responses | `common.get_translations(uuid)` → JSON array |
| Upsert | Part of Product.UpdateTranslations | `INSERT ... ON CONFLICT DO UPDATE` |
| BatchUpdate | Part of Kit.UpdateTranslations | Call `common.upd_translation(uuid, json)` — iterates JSON array, updates existing rows only |

**Business rules:**
- On product creation: IT + EN rows auto-created with empty short/long
- On kit creation: IT + EN rows auto-created with short=internal_name, long=''
- Product short descriptions dual-written to Alyante (Postgres-first, best-effort)
- Long descriptions are Postgres-only (never sent to Alyante)

---

### Entity: CustomerGroup

- **Purpose:** Discount/commercial profile groups
- **Source table:** `customers.customer_group`
- **Primary key:** `id` (integer, auto-generated)

**Operations:**

| Operation | API | Implementation |
|-----------|-----|----------------|
| List | `GET /kit-products/v1/customer-group` | `SELECT id, name, is_default, is_partner, read_only, base_discount FROM customers.customer_group ORDER BY name` |
| Create | `POST /kit-products/v1/customer-group` | INSERT (name, is_partner) |
| BatchUpdate | `PATCH /kit-products/v1/customer-group` | UPDATE name, is_partner per item — single transaction, all-or-nothing. Backend rejects updates to read_only groups. |

**Fields:** `id` (PK), `name`, `is_default` (read-only display), `is_partner` (editable), `read_only` (guards editability), `base_discount` (read-only in this app — used by Kit Discounts to auto-fill MRC)

No delete — FK-referenced by KitDiscount and Customer.

---

### Entity: KitDiscount

- **Purpose:** Per-kit, per-customer-group discount rules
- **Source table:** `products.kit_customer_group`
- **Primary key:** composite (`kit_id`, `group_id`)

**Operations:**

| Operation | API | Implementation |
|-----------|-----|----------------|
| ListByKit | `GET /products/v2/kit-discount?kit_id=...` | **Existing Mistra REST API** (proxy via arak) |
| CreateOrUpdate | `POST /products/v2/kit-discount` | **Existing Mistra REST API** (upsert semantics) |

**API payload (existing):**
```json
{
  "kit_id": 1,
  "customer_group_id": 2,
  "sellable": true,
  "use_int_rounding": false,
  "mrc": { "percentage": "20", "sign": "-" },
  "nrc": { "percentage": "20", "sign": "-" }
}
```

**Business rules:**
- Max discount: 100% for sign="-", uncapped for sign="+" (frontend + backend validation)
- NRC defaults to MRC values on creation (frontend convenience)
- Auto-fill MRC from group's `base_discount` (frontend reads from CustomerGroup list)
- DB trigger `trg_validate_discount` ensures discount >= -1
- No delete operation

---

### Entity: KitCustomValue

- **Purpose:** Arbitrary key-value pairs on kits
- **Source table:** `products.kit_custom_value`
- **Primary key:** `id` (bigint, auto-generated)

**Operations:**

| Operation | API | Implementation |
|-----------|-----|----------------|
| ListByKit | `GET /kit-products/v1/kit/{kitId}/custom-values` | `SELECT id, kit_id, key_name, jsonb_pretty(value) FROM products.kit_custom_value WHERE kit_id = ...` |
| Create | `POST /kit-products/v1/kit/{kitId}/custom-values` | INSERT (kit_id, key_name, value) |
| Update | `PUT /kit-products/v1/kit/{kitId}/custom-values/{id}` | UPDATE key_name, value WHERE id = ... AND kit_id = ... |
| Delete | `DELETE /kit-products/v1/kit/{kitId}/custom-values/{id}` | DELETE WHERE id = ... AND kit_id = ... |

**Fields:** `id` (PK), `kit_id` (FK), `key_name` (FK → CustomFieldKey), `value` (jsonb)

---

### Entity: Customer (read-only reference)

- **Not managed by this app.** Read-only reference for price simulation and discount preview.
- **Source:** `GET /customers/v2/customer` (existing Mistra REST API, proxy via arak)

---

### Lookup Entities (read-only)

| Entity | Source | API | Used By |
|--------|--------|-----|---------|
| AssetFlow | `products.asset_flow` | `GET /kit-products/v1/lookup/asset-flow` | Product form (select dropdown) |
| CustomFieldKey | `common.custom_field_key` | `GET /kit-products/v1/lookup/custom-field-key` | KitCustomValue form (key select) |
| Vocabulary | `common.vocabulary` | `GET /kit-products/v1/lookup/vocabulary?section=kit_product_group` | KitProduct form (group_name select) |

---

### Computed View: DiscountedKit (read-only)

- **Source:** Existing Mistra REST API
- **Endpoints:**
  - `GET /products/v2/discounted-kit?customer_id=...` — list with discounts applied
  - `GET /products/v2/discounted-kit/{kitId}?customer_id=...` — detail with related products
- **Used by:** Price Simulator view

---

## View Specifications

### View: Kit List

- **Route:** `/kit`
- **User intent:** Browse all kits, take actions (edit, create, clone, soft-delete)
- **Interaction pattern:** Data table with toolbar + modals

**Main data:** Kit list (83 rows)

**Default visible columns (8-9):** internal_name (with main_product_code appended), bundle_prefix, nrc, mrc, category (color-coded), is_active, billing_period, sconto_massimo. Column visibility toggle for remaining ~7 columns.

**Toolbar actions:**
- "Edit Kit" → navigate to `/kit/:id`
- "New Kit" → modal (name, prefix, category, main product, pricing, subscriptions, sellable groups, ecommerce) → on submit: navigate to `/kit/:id`
- "More" dropdown: Clone Kit (modal with name input), Refresh, Soft-Delete (set is_active=false)

**Key behaviors:**
- Active kits sorted first
- Category column color-coded by `product_category.color`
- internal_name display: `{name} ({main_product_code})`
- Edit/Clone/Delete disabled when no row selected

---

### View: Kit Detail

- **Route:** `/kit/:id`
- **User intent:** Edit all aspects of a single kit
- **Interaction pattern:** Tabbed editor (3 tabs)
- **Entry:** Kit List (Edit button or after Create/Clone)
- **Exit:** Back arrow → `/kit` (consistent across all tabs)
- **Header:** `← Tutti i Kit / KIT #{id} - {internal_name}`

**Tab 1 — Dettagli:**
- Form with 16+ fields bound to kit data
- `bundle_prefix` disabled on existing kits (backend also rejects changes)
- Billing period: static select (Mensile/Bimestrale/.../Biennale)
- `ms_sellable_to`: multi-select of customer groups
- Help URL field (moved from Kit List "More" menu)
- **Save Kit button** → `PUT /kit-products/v1/kit/{id}`
- Translation table (inline editing: short/long per IT/EN language)
- **Save Translations button** (separate) → `PUT /kit-products/v1/kit/{id}/translations`

**Tab 2 — Prodotti:**
- Table with inline editing: group_name (select), min, max, required (checkbox), nrc, mrc, position
- Toolbar: Add (+) → modal, Edit (pencil) → modal pre-filled, Save (batch) → `PATCH`, Refresh, Delete (with confirmation)
- Add/Edit modal: product select, group_name select, quantities, pricing, notes, required toggle
- **Save button** for batch inline edits (all-or-nothing transaction)

**Tab 3 — Valori Custom:**
- Table with inline editing + add-new-row: key_name (select from CustomFieldKey), value (JSON text)
- Per-row save for edits
- Add-new-row for creation
- Delete supported (per row)

---

### View: Product List

- **Route:** `/products`
- **User intent:** Browse and manage individual products, edit descriptions with ERP sync
- **Interaction pattern:** Data table with inline editing + modals

**Main data:** Product list (836 rows) with category join + embedded translations

**Inline-editable columns (7):** internal_name, nrc, mrc, category (select), asset_flow (select), erp_sync (checkbox), img_url

**Actions:**
- "+" button → New Product modal (code [required, user-assigned], name, category, nrc, mrc, erp_sync, img_url, descriptions IT/EN, asset_flow [select, not free text])
- "Edit descriptions" button (disabled when no row selected) → Descriptions modal (short/long IT, short/long EN — multiline text, short required)
- Per-row save/discard for inline edits

**Key behaviors:**
- Per-row save (not batch)
- Description save triggers dual-write (Postgres-first, Alyante best-effort with warning on failure)
- Product code is user-assigned, max 25 chars (Alyante constraint)
- No delete operation

---

### View: Kit Discounts

- **Route:** `/discounts`
- **User intent:** Manage which customer groups can buy which kits and at what discount
- **Interaction pattern:** Master-detail (side-by-side tables) + modal

**Left panel:** Kit list from `GET /products/v2/kit` (visible: internal_name, category)
**Right panel:** Discount groups for selected kit from `GET /products/v2/kit-discount?kit_id=...`

**Right panel columns:** group_name (derived), sellable (checkbox), use_int_rounding (checkbox), mrc_sign, mrc_percentage, nrc_sign, nrc_percentage

**Actions:**
- "+" button → Discount modal (create mode)
- Row click → Discount modal (edit mode, pre-populated)

**Discount modal (single modal, create + edit):**
- Group select (create: only unassigned groups; edit: current group, disabled)
- MRC section: sign select (+/-), percentage input (max 100 for "-")
- NRC section: sign select (defaults to MRC sign), percentage input (defaults to MRC percentage)
- Rounding switch ("Arrotondamento")
- Sellable toggle (only in edit mode — hardcoded true on create)
- Submit → `POST /products/v2/kit-discount` (upsert) → refresh right panel

**Business rules (frontend):**
- NRC defaults sync to MRC values (convenience, not enforced)
- MRC auto-filled from group's `base_discount` on group selection
- Max 100% for discount (sign="-"), uncapped for surcharge (sign="+")

---

### View: Price Simulator

- **Route:** `/simulator`
- **User intent:** Explore customer-specific pricing across all kits
- **Interaction pattern:** Cascading filter → master → detail (read-only)
- **Audience:** Same users who manage kits/products

**Sections:**
1. Customer dropdown (filterable, from `GET /customers/v2/customer`)
2. Discounted kits table (from `GET /products/v2/discounted-kit?customer_id=...`)
   - Columns: id, internal_name, nrc, mrc (extracted from nested base_price), category, ecommerce, subscription times, activation time
3. Section header: "Prodotti correlati per il Kit '{name}'"
4. Related products table (flattened from nested groups→products via `.flatMap()`)
   - Columns: group_name, id, title, price_nrc, price_mrc, min_qty, max_qty

**Flow:** Customer change → refresh kits → auto-select row 0 → refresh products

---

### View: Settings — Categories

- **Route:** `/settings/categories`
- **User intent:** Manage product category definitions
- **Interaction pattern:** Simple editable table

**Columns:** name (required, editable), color (editable, **color picker widget**)
**Actions:** Per-row save/discard, add-new-row. No delete.

---

### View: Settings — Customer Groups

- **Route:** `/settings/customer-groups`
- **User intent:** Manage customer discount group definitions
- **Interaction pattern:** Simple editable table with batch save

**Visible columns:** name (editable when !read_only), is_partner (checkbox, editable when !read_only)
**Hidden columns:** id, is_default
**Actions:** Batch save button (disabled when no pending edits), add-new-row
**Business rule:** `read_only` flag prevents editing — backend enforces this too

---

## Logic Allocation

### Backend (Go) Responsibilities

- All database access (Postgres via MistraDSN)
- Stored procedure calls (`new_kit`, `upd_kit`, `clone_kit`, `new_kit_product`, `upd_kit_product`, `upd_translation`)
- Alyante ERP dual-write with best-effort strategy + error logging + warning response
- Mistra REST API proxy via existing arak client
- Batch operations in single DB transactions (all-or-nothing)
- Business rule enforcement:
  - `read_only` guard on customer group updates
  - `bundle_prefix` immutability on existing kits
  - Nested resource ownership verification (kit_product.kit_id, kit_custom_value.kit_id)
  - Discount validation (DB trigger exists; backend should also validate)
- Product code validation (max 25 chars for Alyante compatibility)
- Translation auto-creation on product/kit creation
- Alyante data contract: code padded to 25 chars, language mapping it→ITA / en→ING, 20-space MG87_OPZIONE, MG87_DITTA=1

### Frontend (React) Responsibilities

- Form state management, dirty tracking, save/discard state
- Select option mapping from lookup data
- Display formatting:
  - Category color-coding in table cells
  - `internal_name (main_product_code)` display
  - Nested API response flattening (`.flatMap()` for related products)
  - Derived columns from nested objects (mrc.percentage, customer_group.name)
- NRC-defaults-to-MRC convenience in discount modal
- MRC auto-fill from group's `base_discount`
- Column visibility toggle (Kit List)
- Color picker widget (Categories)
- Form population from selected row (Kit Detail modals)
- URL-based navigation (`/kit/:id` via React Router)

### Shared

- Entity TypeScript types (generated from or matching API contract)
- Field validation schemas (lengths, required fields, numeric ranges)

---

## Integrations and Data Flow

### External Systems

| System | Connection | Direction | Managed By |
|--------|-----------|-----------|------------|
| db-mistra (Postgres) | Go backend via `MistraDSN` (new) | Read + Write | Go backend (direct SQL + stored procedures) |
| Alyante ERP (MSSQL) | Go backend via `ALYANTE_DSN` (new) | Write-only | Go backend ERP adapter (product short descriptions) |
| Mistra NG REST API | Go backend via existing arak client (`ARAK_BASE_URL`) | Read + Write | Go backend proxy (kit discounts, discounted kits, customers) |
| Keycloak | `@mrsmith/auth-client` (existing) | Auth | Role: `app_kitproducts_access` |

### Data Flow

```
                                    ┌─────────────┐
                                    │   Keycloak   │
                                    └──────┬───────┘
                                           │ Bearer token
                                           ▼
┌──────────────────┐              ┌──────────────────┐
│   React Frontend │──── /api ───→│   Go Backend     │
│   (kit-products) │              │                  │
│                  │              │  ┌────────────┐  │
│  /kit            │              │  │ kit-products│  │──── SQL ────→ db-mistra (Postgres)
│  /kit/:id        │              │  │  handlers   │  │                 products.*
│  /products       │              │  └────────────┘  │                 customers.*
│  /discounts      │              │                  │                 common.*
│  /simulator      │              │  ┌────────────┐  │
│  /settings/*     │              │  │ arak proxy  │  │──── REST ───→ Mistra NG API
│                  │              │  └────────────┘  │                 /products/v2/*
│                  │              │                  │                 /customers/v2/*
│                  │              │  ┌────────────┐  │
│                  │              │  │ ERP adapter │  │──── MSSQL ──→ Alyante ERP
│                  │              │  └────────────┘  │                 MG87_ARTDESC
└──────────────────┘              └──────────────────┘
```

### ERP Dual-Write Strategy (Postgres-first, Alyante best-effort)

```
PUT /kit-products/v1/product/{code}/translations
  1. BEGIN transaction
  2. UPSERT common.translation (IT) → Postgres    ✓ always
  3. UPSERT common.translation (EN) → Postgres    ✓ always
  4. COMMIT
  5. UPDATE MG87_ARTDESC (ITA) → Alyante          ⚠ best-effort
  6. UPDATE MG87_ARTDESC (ING) → Alyante          ⚠ best-effort
  7. If step 5 or 6 fails:
     - Log error server-side (full details)
     - Return 200 with warning: { "warning": "erp_sync_failed", "message": "Salvato, ma sincronizzazione ERP fallita" }
```

### End-to-End User Journeys

**Create Kit:** Kit List → New Kit modal → submit → `POST /kit` → navigate to `/kit/:id` → edit details/products/custom values → back to `/kit`

**Edit Product Descriptions (ERP):** Product List → select row → "Edit descriptions" → modal → submit → Postgres upsert + Alyante best-effort → refresh + close modal (warning toast if ERP fails)

**Manage Kit Discounts:** `/discounts` → select kit (left) → view groups (right) → add/edit via single modal → `POST /products/v2/kit-discount` → refresh

**Simulate Pricing:** `/simulator` → select customer → browse discounted kits → select kit → view per-product pricing

**Clone Kit:** Kit List → select kit → More → Clone → name modal → `POST /kit/{id}/clone` → refresh list

---

## API Contract Summary

### New endpoints (Go backend, direct DB)

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/kit-products/v1/kit` | List all kits |
| GET | `/kit-products/v1/kit/{id}` | Get kit detail (+ translations, help URL, sellable groups) |
| POST | `/kit-products/v1/kit` | Create kit (atomic: kit + default translations) |
| PUT | `/kit-products/v1/kit/{id}` | Update kit (+ re-create customer group associations) |
| DELETE | `/kit-products/v1/kit/{id}` | Soft-delete (is_active=false) |
| POST | `/kit-products/v1/kit/{id}/clone` | Clone kit |
| PUT | `/kit-products/v1/kit/{id}/help` | Upsert help URL |
| PUT | `/kit-products/v1/kit/{id}/translations` | Update kit translations |
| GET | `/kit-products/v1/kit/{id}/products` | List kit products |
| POST | `/kit-products/v1/kit/{id}/products` | Add product to kit |
| PUT | `/kit-products/v1/kit/{id}/products/{pid}` | Update kit product |
| PATCH | `/kit-products/v1/kit/{id}/products` | Batch update kit products (transaction) |
| DELETE | `/kit-products/v1/kit/{id}/products/{pid}` | Delete kit product |
| GET | `/kit-products/v1/kit/{id}/custom-values` | List kit custom values |
| POST | `/kit-products/v1/kit/{id}/custom-values` | Create custom value |
| PUT | `/kit-products/v1/kit/{id}/custom-values/{cvid}` | Update custom value |
| DELETE | `/kit-products/v1/kit/{id}/custom-values/{cvid}` | Delete custom value |
| GET | `/kit-products/v1/product` | List products (with translations) |
| POST | `/kit-products/v1/product` | Create product (+ auto-create translations) |
| PUT | `/kit-products/v1/product/{code}` | Update product |
| PUT | `/kit-products/v1/product/{code}/translations` | Update translations (Postgres + Alyante dual-write) |
| GET | `/kit-products/v1/category` | List categories |
| POST | `/kit-products/v1/category` | Create category |
| PUT | `/kit-products/v1/category/{id}` | Update category |
| GET | `/kit-products/v1/customer-group` | List customer groups |
| POST | `/kit-products/v1/customer-group` | Create customer group |
| PATCH | `/kit-products/v1/customer-group` | Batch update customer groups (transaction) |
| GET | `/kit-products/v1/lookup/asset-flow` | Asset flow types |
| GET | `/kit-products/v1/lookup/custom-field-key` | Custom field keys |
| GET | `/kit-products/v1/lookup/vocabulary?section=...` | Vocabulary by section |

### Proxied endpoints (Go backend → Mistra REST API via arak)

| Method | Path | Proxies To |
|--------|------|-----------|
| GET | `/kit-products/v1/mistra/kit` | `GET /products/v2/kit` |
| GET | `/kit-products/v1/mistra/kit-discount` | `GET /products/v2/kit-discount` |
| POST | `/kit-products/v1/mistra/kit-discount` | `POST /products/v2/kit-discount` |
| GET | `/kit-products/v1/mistra/discounted-kit` | `GET /products/v2/discounted-kit` |
| GET | `/kit-products/v1/mistra/discounted-kit/{id}` | `GET /products/v2/discounted-kit/{id}` |
| GET | `/kit-products/v1/mistra/customer` | `GET /customers/v2/customer` |

---

## Constraints and Non-Functional Requirements

### Coexistence

- **Critical:** The new app must coexist with the existing Appsmith version during transition
- Same database, same schema, same stored procedures
- No schema migrations, no column renames, no data model changes
- Both apps may run simultaneously reading/writing the same data

### Security

- All DB access through Go backend — no direct browser-to-DB queries (eliminates S1-S4 from audit)
- Parameterized queries only — no string interpolation in SQL
- Bearer auth on all endpoints via Keycloak
- Nested resource ownership verification on all sub-resource endpoints
- Keycloak role: `app_kitproducts_access`

### Performance

- Product list: 836 rows — single-page load acceptable, client-side search
- Kit list: 83 rows — trivial
- Customer list: 1,391 rows — paginated from Mistra API (disable_pagination=true currently; consider pagination if slow)
- Customer groups: 5 rows — client-side filtering for "unassigned groups" is fine

### Operational

- Structured logging for all backend operations
- Request correlation IDs
- Alyante ERP failures logged with full details (code, language, error) for diagnosis
- Panic recovery middleware (existing pattern from budget/compliance)

---

## Open Questions and Deferred Decisions

| # | Question | Needed Input | Defer To |
|---|----------|-------------|----------|
| Q35 | Should kit soft-delete / future product delete check for order references? | Domain expert | Implementation time |
| — | Exact Vite port for kit-products dev server | Repo check | Implementation planning |
| — | Docker output path for production build | Repo check | Implementation planning |
| — | Whether to add pagination to product list if it grows beyond ~1000 rows | Performance testing | Post-MVP |
| — | Whether Alyante sync failures should trigger a retry mechanism or just log | Ops team | Post-MVP |

---

## Acceptance Notes

### What the audit proved directly
- All 7 page structures, 45 queries, 4 JSObjects with full SQL/JS code
- All widget bindings, event flows, hidden logic, and dependencies
- 7 bugs, 14 dead code items, 4 security concerns documented
- 6 stored procedure bodies extracted from schema dumps

### What the expert confirmed
- Soft-delete for Kit (is_active=false)
- No Product/KitDiscount delete for now; KitCustomValue deletable
- Clone Kit preserved as-is
- Navigation: 4 tabs + gear menu (Kit, Prodotti, Sconti Kit, Simulatore Prezzi, ⚙ Settings)
- Kit Detail on separate route `/kit/:id`
- Postgres-first, Alyante best-effort for ERP dual-write
- Single atomic endpoint for kit creation
- All-or-nothing transactions for batch saves
- Coexistence with Appsmith — no schema changes
- Column visibility toggle for Kit List (8-9 default columns)
- Two separate save buttons in Kit Detail
- Both inline + modal editing in Products tab
- Single modal for add/edit discount
- Color picker for category color
- `erp_sync` controls external job, no in-app side effect
- `sconto_massimo` informational only, no enforcement
- `base_discount` maintained for compatibility
- Lookups from `common.vocabulary` for group names
- No Alyante write on product creation
- Static billing period list
- xmlParser safe to drop

### What still needs validation
- Q35: Order reference checks on delete (deferred)
- Repo-fit checklist (IMPLEMENTATION-PLANNING.md) to be run before implementation plan approval
- Runtime fit: Vite port, base path, dev proxy, Docker output path
- Auth fit: Keycloak role creation, 401/403 behavior
- Actual Alyante connectivity from Go backend (MSSQL driver, DSN configuration)
