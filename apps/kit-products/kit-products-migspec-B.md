# Phase B: UX Pattern Map

> Extracted from `APPSMITH-AUDIT.md` — Kit and Products application
> Status: **DRAFT — awaiting expert review**

---

## View Classification

The Appsmith app has 7 pages. After removing the hidden-page pattern (Edit Kit is a detail page navigated from Kit), the functional views are:

| Appsmith Page | Pattern | Primary Intent | Proposed View |
|---------------|---------|----------------|---------------|
| Kit | Master list + toolbar + modals | Browse/manage kits | **Kit List** |
| Edit Kit | Tabbed detail editor | Edit single kit (details, products, custom values) | **Kit Detail** |
| Products | Editable data table + modals | Browse/manage products | **Product List** |
| Discount groups | Editable data table | Browse/manage customer groups | **Customer Groups** |
| Kit discounts | Master-detail + modals | Manage per-kit discount rules | **Kit Discounts** |
| Kit Price Simulator | Cascading filter → master-detail | Explore customer-specific pricing | **Price Simulator** |
| Categories | Editable data table | Browse/manage categories | **Categories** |

---

## Per-View Analysis

### View 1: Kit List

**Pattern:** Data table with toolbar actions + modals
**User intent:** Find a kit, then take an action (edit, clone, create new, set help URL)
**Entry point:** App home / default page
**Exit points:** Navigate to Kit Detail (edit), stay (clone/create/help)

**Logical UI sections:**

| Section | Widgets | Role |
|---------|---------|------|
| Toolbar | ButtonGroup1 (Edit Kit, New Kit, More menu) | Primary actions |
| Kit table | Table1 (15+ columns, category color-coded, active-first sort) | Data browsing |
| New Kit modal | Modal1 → Form1 (name, prefix, category, main product, pricing, subscriptions, sellable groups, ecommerce) | Kit creation |
| Clone Kit modal | modal_clone (name input) | Kit duplication |
| Help URL modal | description_modal (URL input with HTTPS validation) | Help page management |

**Notes:**
- Table shows `internal_name (main_product_code)` as computed display (BR14)
- Category column is color-coded with `product_category.color` (BR15)
- Active kits sorted first (`is_active::int desc`)
- After New Kit submit → auto-navigate to Kit Detail
- After Clone → stay on list, refresh
- "Delete" exists in menu but is non-functional (D2)

**Questions:**
- **Q14.** The table shows 15+ columns. In the new app, **should all columns remain visible by default**, or should some be hidden with a column visibility toggle? The current layout is very wide.
- **Q15.** "More" menu groups Clone, Refresh, Delete, Help page. **Is this grouping logical, or should these actions be reorganized?** E.g., Help URL could be a field in Kit Detail instead.

---

### View 2: Kit Detail

**Pattern:** Tabbed detail editor (3 tabs)
**User intent:** Edit all aspects of a single kit
**Entry point:** Kit List (Edit button or after New Kit creation)
**Exit point:** Back button → Kit List

**Tab 1 — Details:**

| Section | Widgets | Role |
|---------|---------|------|
| Header | Text1 ("Kit # {id}"), IconButton4 (back/logout) | Identity + navigation |
| Kit form | Form1 (16+ fields: name, prefix, category, main product, pricing, subscriptions, billing period, flags, notes, sellable groups) | Edit kit metadata |
| Save button | btn_save_kit | Persist changes via `upd_kit` |
| Translations table | tbl_translations (inline edit: short/long per language) | Edit IT/EN translations |
| Save translations | btn_upd_transl (red button) | Persist translation changes |

**Tab 2 — Products:**

| Section | Widgets | Role |
|---------|---------|------|
| Toolbar | Add (+), Edit (pencil), Save (cloud-upload), Refresh, Delete (trash), Back | CRUD actions |
| Products table | tbl_related (inline edit: group, min, max, required, nrc, mrc, position) | Kit product management |
| Add/Edit modal | mdl_product → Form2 (product select, group, quantities, pricing, notes, required) | Product add/edit form |

**Tab 3 — Custom Value:**

| Section | Widgets | Role |
|---------|---------|------|
| Custom values table | tbl_custom_value (inline edit + add-new-row: key_name select, value JSON) | Custom field management |

**Notes:**
- `bundle_prefix` is **immutable** on existing kits (BR8)
- Billing period is a static select with Italian labels
- Delete kit-product has a **confirmation dialog** (only confirmed action in the app)
- "Back" button on Products tab navigates to Kit List (not to Details tab)
- `appsmith.store.v_kit_id` maps to a URL param in the new app (`/kits/:kitId`)

**Questions:**
- **Q16.** Kit Detail has **two separate save buttons** — one for kit metadata, one for translations. **Should these be unified into a single save**, or kept separate? Current UX requires the user to remember which section they edited.
- **Q17.** Products tab has both **inline editing AND a modal form**. The modal handles add+edit, inline handles batch quick-edits with a separate save button. **Should we keep both interaction modes, or simplify to one?**
- **Q18.** The "Back" button on the Products tab navigates to Kit List. **Should all tabs share the same back behavior**, or should back navigate between tabs?

---

### View 3: Product List

**Pattern:** Data table with inline editing + modals
**User intent:** Browse products, edit fields inline, manage descriptions (with ERP sync)
**Entry point:** Navigation tab
**Exit points:** None (stays on page)

**Logical UI sections:**

| Section | Widgets | Role |
|---------|---------|------|
| Toolbar | IconButton2 (+), Button3 (Edit descriptions) | Create product, edit translations |
| Products table | tbl_products (inline edit: name, nrc, mrc, category select, asset_flow select, erp_sync, img_url) | Data browsing + inline edit |
| New Product modal | mdl_product → jsform_product (code, name, category, pricing, erp_sync, img_url, descriptions IT/EN, asset_flow) | Product creation |
| Descriptions modal | mdl_descriptions → JSONForm1 (short/long IT, short/long EN) | Translation editing with ERP dual-write |

**Notes:**
- Product `code` is **user-assigned** on creation (not auto-generated)
- "Edit descriptions" button disabled when no row selected
- Per-row save/discard via EditActions column
- Description save triggers **dual-write**: Postgres + Alyante ERP (BR1, BR2)
- New product form: `asset_flow` is free text (bug: should be a select like in the table)
- After new product insert: **no refresh, no modal close** (bug B2)

**Questions:**
- **Q19.** The Products table has **7 inline-editable columns**. This is a lot of inline editing. **Is this the preferred workflow, or would a detail panel/modal be better for editing?** Per the UI/UX doc, master-detail is the established pattern for the budget app.
- **Q20.** `erp_sync` is a boolean column visible and editable in the table. **What should happen when a user toggles it?** Currently it's just a stored flag with no side effects.

---

### View 4: Customer Groups

**Pattern:** Editable data table with batch save
**User intent:** Manage customer discount group definitions
**Entry point:** Navigation tab
**Exit points:** None

**Logical UI sections:**

| Section | Widgets | Role |
|---------|---------|------|
| Groups table | tbl_groups (inline edit: name, is_partner; add-new-row) | Group management |
| Save button | IconButton1 (cloud-upload, disabled when no pending edits) | Batch persist |

**Notes:**
- `read_only` rows cannot be edited (BR7) — editability controlled per-row
- `is_default` column fetched but hidden
- Batch save: loops over all changed rows, saves sequentially
- Add-new-row saves immediately (not batched)
- **No delete** operation

**Questions:**
- **Q21.** This is a very simple page — 3 columns, no complex interactions. **Could it be merged into a settings/admin section** alongside Categories, or does it warrant its own top-level nav item?

---

### View 5: Kit Discounts

**Pattern:** Master-detail (side-by-side tables) + modals
**User intent:** Manage which customer groups can buy which kits and at what discount
**Entry point:** Navigation tab
**Exit points:** None

**Logical UI sections:**

| Section | Widgets | Role |
|---------|---------|------|
| Header text | Text1 ("Selezionare il kit a sinistra...") | Instructions |
| Kit list (left) | tbl_kit (API-driven, visible: name + category) | Kit selection |
| Discount groups (right) | tbl_cgroups (API-driven, derived columns for nested mrc/nrc) | Discount rules for selected kit |
| Add group button | IconButton3 (+) | Open add modal |
| Add discount modal | mdl_associate_new_grp (group select, MRC/NRC percentage+sign, rounding, sellable) | Create association |
| Edit discount modal | mdl_edit_discount (same fields, pre-populated) | Edit existing association |
| Kit details modal | kit_details (customer select → discounted products table) | Price preview |

**Notes:**
- Left table from REST API (`GetAllKit`), right table from REST API (`GetAllKitDiscountsById`)
- Add and Edit both use `POST /products/v2/kit-discount` (upsert) (BR10)
- NRC defaults sync to MRC values (BR3)
- Max discount 100% for sign="-", uncapped for sign="+" (BR4)
- Auto-fill MRC from group's `base_discount` (BR6)
- Only unassigned groups shown in add modal (BR5)
- Near-identical add/edit modals (DUP6) — should be unified in new app
- Kit details modal has a dead "Confirm" button (D13)
- Hidden save button + dead `salvaModifiche()` (D9) — legacy dead code

**Questions:**
- **Q22.** The kit details modal shows a price preview for a specific customer. **Is this a frequently used feature?** It's somewhat hidden (no visible trigger in the main page audit; may be accessed from another path). Should it be a dedicated view or remain a modal?
- **Q23.** The "Add" and "Edit" discount modals are nearly identical (DUP6). **In the new app, should there be a single modal for both create and edit?**

---

### View 6: Price Simulator

**Pattern:** Cascading filter → master → detail (read-only)
**User intent:** Explore what a specific customer would pay for any kit, including per-product pricing
**Entry point:** Navigation tab
**Exit points:** None

**Logical UI sections:**

| Section | Widgets | Role |
|---------|---------|------|
| Customer filter | sl_customer (filterable dropdown) | Select customer |
| Kit table | tbl_discounted_kits (derived nrc/mrc from nested base_price) | Browse discounted kits |
| Section header | Text1 ("Related Products for Kit '{name}'") | Context label |
| Products table | tbl_discounted_rp (flattened from nested groups→products) | Per-product pricing |

**Notes:**
- Fully **read-only** — no write operations
- All data from REST API (3 GET calls)
- Customer change → refresh kit list → auto-select row 0 → refresh products
- Related products flattened from nested structure with `.flatMap()` (DUP4)
- Hidden columns: `base_price`, `group_required`, `img_url`

**Questions:**
- **Q24.** This is the only read-only page. **Is it used primarily by sales teams, or by the same users who manage kits/products?** Understanding the audience might influence whether it stays as a tab or becomes a separate tool.

---

### View 7: Categories

**Pattern:** Simple editable data table
**User intent:** Manage product category definitions (name + color)
**Entry point:** Navigation tab
**Exit points:** None

**Logical UI sections:**

| Section | Widgets | Role |
|---------|---------|------|
| Categories table | Table1 (inline edit: name, color; add-new-row; per-row save/discard) | Category management |

**Notes:**
- Minimal page: one table, two editable columns, no modals
- `id` hidden, auto-generated
- Per-row save (not batch)
- **No delete** — categories are FK-referenced
- Color is free text (not a color picker)

**Questions:**
- **Q25.** Like Discount Groups, this is very simple. **Should Categories be merged with Discount Groups into a "Settings" or "Configuration" section?**
- **Q26.** Category `color` is currently free text. **Should the new app use a color picker widget?**

---

## Navigation Structure

### Current (Appsmith)

```
[Kit] [Products] [Discount groups] [Kit discounts] [Kit Price Simulator] [Categories]
                                                          ↕ (hidden)
                                                      [Edit Kit]
```

7 tabs, 1 hidden page. All siblings at the same level.

### Proposed restructuring (for expert review)

**Option A — Keep flat tabs** (closest to current):
```
[Kit] [Products] [Discount Groups] [Kit Discounts] [Price Simulator] [Categories]
```
Pro: Minimal change, users keep existing mental model.
Con: 6 tabs may feel crowded; simple pages (Categories, Discount Groups) don't warrant top-level nav.

**Option B — Group by function:**
```
[Kit]  [Products]  [Pricing]  [Settings]
                      ├── Kit Discounts
                      └── Price Simulator
                                  ├── Categories
                                  ├── Discount Groups
                                  └── (future lookups)
```
Pro: Cleaner navigation, groups related pages.
Con: Extra click to reach sub-pages; different from what users know.

**Option C — Primary + secondary nav:**
```
Primary tabs:  [Kit]  [Products]  [Kit Discounts]  [Price Simulator]
Secondary:     Categories and Discount Groups accessible from a gear icon / settings area
```
Pro: Main workflow front-and-center; admin pages tucked away.
Con: Users who frequently edit groups/categories lose quick access.

**Q27.** Which navigation structure do you prefer? Or a different layout entirely?

---

## Cross-View UI Patterns Summary

| Pattern | Used In | Frequency |
|---------|---------|-----------|
| Data table with inline editing | Products, Categories, Discount Groups, Edit Kit (Products tab, Custom Values tab, Translations) | 6 instances |
| Data table with per-row save/discard | Products, Categories | 2 instances |
| Data table with batch save button | Discount Groups, Edit Kit (Products tab) | 2 instances |
| Modal form for create/edit | Kit (New/Clone/Help), Edit Kit (Product), Products (New/Descriptions), Kit Discounts (Add/Edit) | 8 modals |
| Master-detail side-by-side | Kit Discounts | 1 instance |
| Cascading filter → tables | Price Simulator | 1 instance |
| Tabbed detail editor | Edit Kit | 1 instance |
| Category color-coding in table cells | Kit list | 1 instance |

**Q28.** The existing budget app uses a **master-detail pattern** (left list + right sticky panel). **Should kit-products adopt the same pattern** for Kit List → Kit Detail, or is the current navigate-to-separate-page approach preferred?
