# Phase A: Entity-Operation Model

> Extracted from `APPSMITH-AUDIT.md` — Kit and Products application
> Status: **DRAFT — awaiting expert review**

---

## Extracted Entities

### 1. Kit

**Source tables:** `products.kit`, `products.kit_help`
**Source pages:** Kit (list/create/clone), Edit Kit (detail editor)

**Inferred fields:**

| Field | Type | Nullable | Default | Notes |
|-------|------|----------|---------|-------|
| id | bigint | no | sequence | PK, auto-generated |
| internal_name | varchar(255) | no | '' | Unique |
| main_product_code | varchar(32) | no | — | FK → Product.code |
| category_id | integer | no | — | FK → ProductCategory.id |
| bundle_prefix | varchar(64) | yes | — | **Immutable after creation** (BR8) |
| initial_subscription_months | smallint | no | 0 | Form default: 12 |
| next_subscription_months | smallint | no | 0 | Form default: 12 |
| activation_time_days | integer | no | 0 | Form default: 30 |
| nrc | numeric(14,5) | no | 0 | Non-recurring charge |
| mrc | numeric(14,5) | no | 0 | Monthly recurring charge |
| translation_uuid | uuid | no | gen_random_uuid() | FK → Translation system |
| ecommerce | boolean | no | true | Visible in ecommerce? |
| is_active | boolean | no | true | |
| is_main_prd_sellable | boolean | yes | true | |
| quotable | boolean | yes | true | |
| billing_period | integer | no | 3 | Enum: 1,2,3,4,6,12,24 |
| sconto_massimo | numeric(5,2) | no | 0 | Max discount % |
| variable_billing | boolean | no | false | |
| h24_assurance | boolean | no | false | |
| sla_resolution_hours | integer | no | 0 | |
| notes | text | yes | '' | |
| help_url | varchar(255) | yes | — | From `kit_help` table, 1:1 |

**Operations:**

| Verb | Source | Notes |
|------|--------|-------|
| List | `get_kit` | All kits, active first, with category resolution |
| GetById | `get_kit_by_id` | Single kit with all fields |
| Create | `products.new_kit(json)` | Stored procedure; creates kit + translations + customer group associations in one call |
| Update | `products.upd_kit(id, json)` | Stored procedure; updates metadata from form data |
| Clone | `products.clone_kit(id, name)` | Stored procedure; deep-clones kit with all products and custom values |
| Delete | — | **Menu item exists but has no handler (D2). Was delete ever intended?** |
| UpsertHelpUrl | `upd_kit_help` | Upsert by kit_id |
| GetHelpUrl | `get_kit_help` | By kit_id |

**Relationships:**
- Kit → 1 ProductCategory (category_id)
- Kit → 1 Product as main product (main_product_code)
- Kit → N KitProduct (kit items)
- Kit → N KitDiscount (customer group associations)
- Kit → N KitCustomValue (custom key-value pairs)
- Kit → 1 Translation (via translation_uuid)
- Kit → 0..1 HelpUrl

---

### 2. KitProduct

**Source tables:** `products.kit_product`
**Source pages:** Edit Kit (Products tab)

**Inferred fields:**

| Field | Type | Nullable | Default | Notes |
|-------|------|----------|---------|-------|
| id | bigint | no | sequence | PK |
| kit_id | bigint | no | — | FK → Kit.id (CASCADE) |
| product_code | varchar(32) | no | — | FK → Product.code |
| group_name | varchar(64) | yes | — | From vocabulary `kit_product_group` |
| minimum | integer | no | 0 | Min quantity |
| maximum | integer | no | -1 | Max quantity (-1 = unlimited) |
| required | boolean | no | false | |
| nrc | double | no | 0 | Override NRC for this product in this kit |
| mrc | double | no | 0 | Override MRC for this product in this kit |
| position | integer | no | 0 | Display order |
| notes | text | yes | — | |

**Operations:**

| Verb | Source | Notes |
|------|--------|-------|
| ListByKit | `get_kit_product` | Joined with product.internal_name, ordered by position, group, name |
| Create | `products.new_kit_product(json)` | Stored procedure |
| Update | `products.upd_kit_product(id, json)` | Stored procedure |
| BatchUpdate | `utils.saveRelatedProducts()` | Sequential loop over inline edits |
| Delete | `delete_kit_product` | Direct DELETE with confirmation dialog |

**Relationships:**
- KitProduct → 1 Kit (kit_id)
- KitProduct → 1 Product (product_code)

---

### 3. Product

**Source tables:** `products.product`, `products.asset_flow`
**Source pages:** Products

**Inferred fields:**

| Field | Type | Nullable | Default | Notes |
|-------|------|----------|---------|-------|
| code | varchar(32) | no | — | PK, **user-assigned** (not auto-generated) |
| internal_name | varchar(255) | no | '' | |
| category_id | integer | no | — | FK → ProductCategory.id |
| translation_uuid | uuid | no | gen_random_uuid() | FK → Translation system |
| nrc | numeric(14,5) | no | 0 | List price NRC |
| mrc | numeric(14,5) | no | 0 | List price MRC |
| img_url | varchar(255) | yes | — | |
| erp_sync | boolean | yes | true | Whether to sync with Alyante ERP |
| asset_flow | varchar(50) | yes | — | FK → AssetFlow.name |

**Operations:**

| Verb | Source | Notes |
|------|--------|-------|
| List | `get_products` | With category join + embedded translations JSON |
| Create | `ins_product` | Inserts product + creates IT/EN translation rows; **bug B1: asset_flow not saved** |
| Update | `upd_products` | Inline table edits; category resolved by name→id, asset_flow by label→name |
| UpdateTranslations | `ins_translation` + `upd_translation_alyante` | Dual-write to Postgres + Alyante ERP (BR1, BR2) |
| Delete | — | **No delete operation exists** |

**Relationships:**
- Product → 1 ProductCategory (category_id)
- Product → 0..1 AssetFlow (asset_flow)
- Product → 1 Translation (via translation_uuid)
- Product → N KitProduct (used in kits)

**Note:** `code` is a user-assigned identifier (max 25 chars in Alyante, 32 in Postgres). Per IMPLEMENTATION-PLANNING.md: "Resolve identifier strategy before defining CRUD" — this is a meaningful string key, not auto-generated.

---

### 4. ProductCategory

**Source tables:** `products.product_category`
**Source pages:** Categories

**Inferred fields:**

| Field | Type | Nullable | Default | Notes |
|-------|------|----------|---------|-------|
| id | integer | no | sequence | PK |
| name | varchar(64) | no | — | |
| color | varchar(12) | no | '#231F20' | Used for table cell background |

**Operations:**

| Verb | Source | Notes |
|------|--------|-------|
| List | `get_category` | Sorted by name; used as dropdown across 4 pages |
| Create | `ins_category` | Via add-new-row |
| Update | `upd_category` | Inline editing |
| Delete | — | **No delete. Categories are FK-referenced by Kit and Product.** |

**Relationships:**
- ProductCategory → N Kit (category_id)
- ProductCategory → N Product (category_id)

---

### 5. Translation

**Source tables:** `common.translation`, `common.language`
**Source pages:** Products (descriptions), Edit Kit (translations tab)

**Inferred fields:**

| Field | Type | Nullable | Default | Notes |
|-------|------|----------|---------|-------|
| translation_uuid | uuid | no | — | PK (composite with language) |
| language | char(2) | no | — | PK (composite). Values: 'it', 'en' |
| short | varchar(255) | no | '' | Short text / title |
| long | text | no | '' | Long text / description |

**Operations:**

| Verb | Source | Notes |
|------|--------|-------|
| GetByUuid | `common.get_translations(uuid)` | Returns JSON array of {language, short, long} |
| Upsert | `ins_translation` | ON CONFLICT DO UPDATE |
| BatchUpdate | `common.upd_translation(uuid, json)` | Stored procedure, used by Edit Kit |

**Key business rules:**
- On product creation, IT + EN rows auto-created with empty `long` (BR11)
- On product translation save, short descriptions dual-written to Alyante ERP (BR1)
- Language mapping: Postgres `it`/`en` → Alyante `ITA`/`ING` (BR2)
- Product code padded to 25 chars with spaces for Alyante (BR2)

---

### 6. CustomerGroup

**Source tables:** `customers.customer_group`
**Source pages:** Discount groups

**Inferred fields:**

| Field | Type | Nullable | Default | Notes |
|-------|------|----------|---------|-------|
| id | integer | no | sequence | PK |
| name | varchar(255) | no | — | |
| is_default | boolean | no | false | Hidden in UI, informational |
| is_partner | boolean | no | false | Editable |
| read_only | boolean | yes | false | **Guards editability** (BR7) |
| base_discount | integer | yes | 0 | **Fetched by DB but NOT in Appsmith SELECT. Used by Kit discounts `setDiscount()` to auto-fill MRC.** |

**Operations:**

| Verb | Source | Notes |
|------|--------|-------|
| List | `get_customer_groups` | Sorted by name |
| Create | `ins_customer_group` | Inserts name + is_partner only |
| BatchUpdate | `JSObject1.salvaModifiche()` | Sequential loop; updates name + is_partner |
| Delete | — | **No delete. Groups are FK-referenced by KitDiscount and Customer.** |

**Business rules:**
- Rows with `read_only = true` cannot be edited inline (BR7)
- `base_discount` is passed in update payload but **never written** (D14) — is this intentional?

---

### 7. KitDiscount

**Source tables:** `products.kit_customer_group`
**Source pages:** Kit discounts

**Inferred fields (from REST API `NewKitDiscount` body):**

| Field | Type | Nullable | Default | Notes |
|-------|------|----------|---------|-------|
| kit_id | bigint | no | — | PK (composite), FK → Kit.id |
| group_id (customer_group_id) | integer | no | — | PK (composite), FK → CustomerGroup.id |
| sellable | boolean | no | true | "Vendibile" |
| use_int_rounding | boolean | yes | false | "Arrotondamento" |
| discount_mrc | numeric(6,3) | yes | 0 | Stored as percentage+sign in API |
| discount_nrc | numeric(6,3) | no | 0 | Stored as percentage+sign in API |

**API representation (REST):**
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

**Operations:**

| Verb | Source | Notes |
|------|--------|-------|
| ListByKit | `GET /products/v2/kit-discount?kit_id=...` | Via REST API |
| CreateOrUpdate | `POST /products/v2/kit-discount` | Upsert semantics (BR10) |
| Delete | — | **No delete operation exists** |

**Business rules:**
- NRC defaults sync to MRC values on creation (BR3)
- Max discount capped at 100% for sign="-", uncapped for sign="+" (BR4)
- Only unassigned groups shown when adding new association (BR5)
- MRC auto-filled from group's `base_discount` (BR6)
- DB trigger `trg_validate_discount` ensures discount >= -1

---

### 8. KitCustomValue

**Source tables:** `products.kit_custom_value`, `common.custom_field_key`
**Source pages:** Edit Kit (Custom Value tab)

**Inferred fields:**

| Field | Type | Nullable | Default | Notes |
|-------|------|----------|---------|-------|
| id | bigint | no | sequence | PK |
| kit_id | bigint | no | — | FK → Kit.id |
| key_name | varchar(32) | no | — | FK → CustomFieldKey.key_name |
| value | jsonb | yes | — | Arbitrary JSON value |

**Operations:**

| Verb | Source | Notes |
|------|--------|-------|
| ListByKit | `get_kit_custom_value` | With `jsonb_pretty(value)` |
| Create | `new_kit_custom_value` | Via add-new-row |
| Update | `upd_kit_custom_value` | Inline editing |
| Delete | — | **No delete** |

---

### 9. Customer (read-only reference)

**Source tables:** `customers.customer`
**Source pages:** Kit discounts (detail modal), Kit Price Simulator

**Used in the app only for:**
- Selecting a customer in Kit Price Simulator to view discounted pricing
- Selecting a customer in Kit discounts detail modal for price preview

**Operations:**

| Verb | Source | Notes |
|------|--------|-------|
| List | `GET /customers/v2/customer` (REST) or `SELECT * FROM customers.customer` (SQL) | Two different sources in different pages |

**Not managed by this app.** Read-only reference entity.

---

### 10. AssetFlow (lookup)

**Source tables:** `products.asset_flow`
**Source pages:** Products (dropdown)

**Inferred fields:**

| Field | Type |
|-------|------|
| name | varchar (PK) |
| label | varchar |

**Operations:** List only. Not managed by this app — pure lookup.

---

### 11. CustomFieldKey (lookup)

**Source tables:** `common.custom_field_key`
**Source pages:** Edit Kit (Custom Value tab dropdown)

**Inferred fields:**

| Field | Type |
|-------|------|
| key_name | varchar(32) (PK) |
| key_description | varchar |
| get_values | text (SQL for valid values) |
| schema | text |

**Operations:** List only. Not managed by this app — pure lookup.

---

### 12. Vocabulary (lookup)

**Source tables:** `common.vocabulary`
**Source pages:** Edit Kit (group_name dropdown)

Used only for `section = 'kit_product_group'` to populate the group name select.

**Operations:** List only, filtered by section.

---

## Computed / Derived Entities (from REST API)

### DiscountedKit (read-only, computed)

**Source:** `GET /products/v2/discounted-kit` and `GET /products/v2/discounted-kit/{id}`
**Pages:** Kit Price Simulator, Kit discounts (detail modal)

Returns kits with pricing adjusted for a specific customer's discount group:

```
{
  id, internal_name, category, ecommerce,
  base_price: { nrc, mrc },
  starter_subscription_time, regular_subscription_time, activation_time,
  title,
  related_products: [
    {
      group_name, required,
      products: [
        { id, title, price: { nrc, mrc }, min_qty, max_qty, img_url }
      ]
    }
  ]
}
```

This is a **computed view**, not a stored entity. It combines Kit + KitProduct + KitDiscount data, applying discount calculations server-side.

---

## Summary of Entity Relationships

```
ProductCategory ──1:N──→ Kit
ProductCategory ──1:N──→ Product
Product ──1:N──→ KitProduct ←──N:1── Kit
CustomerGroup ──1:N──→ KitDiscount ←──N:1── Kit
Kit ──1:N──→ KitCustomValue ←──N:1── CustomFieldKey
Kit ──1:1──→ Translation (via uuid)
Product ──1:1──→ Translation (via uuid)
Product ──0:1──→ AssetFlow
Customer ──N:1──→ CustomerGroup (primary)
Customer ──M:N──→ CustomerGroup (via group_association)
```

---

## Questions for Expert Review

### Entity completeness

**Q1.** The Kit "Delete" menu item exists in Appsmith but has no handler (D2). **Should Kit Delete be part of the new app?** If yes, should it be soft-delete (is_active=false) or hard-delete? Note: kits are referenced by `orders.order.kit_id`.

**Q2.** There is **no Product Delete** operation. **Should the new app support product deletion?** Products are referenced by KitProduct and orders.order_row.

**Q3.** There is **no KitDiscount Delete** operation. **Can a discount association be removed once created?** Currently only sellable=false can disable it.

**Q4.** There is **no KitCustomValue Delete** operation. **Should custom values be deletable?**

### Field ambiguities

**Q5.** `CustomerGroup.base_discount` is fetched from DB and used by `setDiscount()` to auto-fill MRC discount, but it's **not in the Discount Groups SELECT** and not updatable from that page. **Is `base_discount` managed elsewhere, or is it a dead field?**

**Q6.** `Kit.sconto_massimo` (max discount) is shown in the Kit list table but **not enforced anywhere** in the discount modals. **Should the new app validate that kit discounts don't exceed this limit?**

**Q7.** `Product.erp_sync` flag is editable in the Products table. **Does this flag actually control whether `upd_translation_alyante` runs, or is it purely informational?** The audit shows `salvaDescrizioni()` always writes to Alyante regardless of this flag.

### Entity merges / splits

**Q8.** `Kit.help_url` comes from a separate table (`kit_help`) with its own upsert. **Should HelpUrl be a first-class field on Kit, or remain a separate entity?** The 1:1 relationship suggests merging into Kit.

**Q9.** The audit found **KitProductGroup** as a separate table (4 rows) but the Appsmith app uses `common.vocabulary` for group names, not the table directly. **Which is the authoritative source for product group names?** Should groups be managed as a proper entity or remain a vocabulary lookup?

### Operation design

**Q10.** Kit Create currently calls `products.new_kit(json)` which does everything atomically in a stored procedure. **Should the backend API replicate this atomicity, or decompose into separate endpoints?**

**Q11.** The "Clone Kit" operation uses `products.clone_kit(id, name)`. **Is clone still a required feature?** If yes, should it deep-clone KitProducts and KitCustomValues as it does now?

**Q12.** Kit discounts page uses the REST API (`/products/v2/kit-discount`) while direct-SQL pages access the same data differently. **Should the new app exclusively use the existing REST API, or do some operations need new backend endpoints?** (API coverage analysis pending)

### Billing period values

**Q13.** `Kit.billing_period` uses a static list: Mensile(1), Bimestrale(2), Trimestrale(3), Quadrimestrale(4), Semestrale(6), Annuale(12), Biennale(24). **Is this list complete and stable, or should it be configurable?**

---

## API Coverage Analysis

### Existing Mistra REST API (reusable via Go arak proxy)

| Endpoint | Method | Covers |
|----------|--------|--------|
| `/products/v2/kit` | GET | Kit list (brief schema — may lack some fields) |
| `/products/v2/kit/{kitId}` | GET | Kit detail (may partially cover full-field needs) |
| `/products/v2/discounted-kit` | GET | Discounted kit list by customer |
| `/products/v2/discounted-kit/{kitId}` | GET | Discounted kit detail by customer |
| `/products/v2/kit-discount` | GET | Kit discount list |
| `/products/v2/kit-discount` | POST | Kit discount create/upsert |
| `/customers/v2/customer` | GET | Customer list |
| `/customers/v2/customer-group` | GET | Customer group list |
| `/customers/v2/customer-group/{id}` | PUT | Customer group edit |

### Operations requiring NEW backend endpoints (direct DB)

**Kit lifecycle** (stored procedures): Create, Clone, Update, Help URL upsert
**KitProduct CRUD**: List by kit, Add, Update, Delete (stored procedures)
**Product CRUD**: List (with translations), Create (+ auto-create translations), Update
**ProductCategory CRUD**: List, Create, Update
**CustomerGroup**: Create (Mistra only has edit)
**Translation**: Get by UUID, Update (stored procedure)
**Kit Custom Values CRUD**: List, Create, Update
**Lookups**: AssetFlow, Vocabulary, CustomFieldKey
**ERP sync**: Alyante dual-write (must move server-side)

### Go backend current state

- `internal/platform/arak/` — generic HTTP client for Mistra API, ready to proxy
- `internal/budget/` — working proxy pattern example
- **No products/kits code exists** — entire module needs building
- No `MistraDSN` configured — will need direct Postgres access for ~30 operations

### Frontend client current state

- `@mrsmith/api-client` — bare HTTP client with auth. No domain types or methods for kit-products.
