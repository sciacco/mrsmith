# Phase D: Integration and Data Flow

> Extracted from `APPSMITH-AUDIT.md` — Kit and Products application
> Status: **DRAFT — awaiting expert review**

---

## External Systems

### 1. db-mistra (PostgreSQL)

| Aspect | Details |
|--------|---------|
| **Current access** | Direct SQL from Appsmith browser (30 queries) |
| **Target access** | Go backend with `MistraDSN` connection (new) |
| **Schema** | `products`, `customers`, `common` |
| **Stored procedures** | 6 functions called directly |
| **Tables written** | `products.kit`, `products.kit_product`, `products.product`, `products.product_category`, `products.kit_customer_group` (via API), `products.kit_custom_value`, `products.kit_help`, `common.translation`, `customers.customer_group` |
| **Migration concern** | No existing DB connection in Go backend — needs `MistraDSN` env var, connection pool, migration pattern decision |

### 2. Alyante ERP (MS SQL Server)

| Aspect | Details |
|--------|---------|
| **Current access** | Direct MSSQL from Appsmith browser (1 query) |
| **Target access** | Go backend with dedicated ERP adapter |
| **Table** | `MG87_ARTDESC` (product descriptions) |
| **Operations** | UPDATE only (short descriptions for IT/EN) |
| **Data contract** | `MG87_DITTA_CG18 = 1`, `MG87_OPZIONE_MG5E = '                    '` (20 spaces), `MG87_LINGUA_MG52 = 'ITA'/'ING'`, `MG87_CODART_MG66 = code.padEnd(25, ' ')` |
| **Migration concern** | Must move server-side. Needs MSSQL driver in Go, `ALYANTE_DSN` env var, error handling strategy for ERP unavailability |

**Q33 — DECIDED: Opzione B (Postgres-first, Alyante best-effort).** Postgres writes always succeed. If Alyante fails, log error server-side + return warning to frontend. User sees "Salvato, ma sincronizzazione ERP fallita". Consider retry/sync-pending mechanism.

### 3. GW internal CDLAN (Mistra NG REST API)

| Aspect | Details |
|--------|---------|
| **Current access** | Direct REST calls from Appsmith browser (8 queries) |
| **Target access** | Go backend proxies via existing `internal/platform/arak/` client |
| **Base URL** | Configured as `ARAK_BASE_URL` (already in Go config) |
| **Auth** | Keycloak client_credentials (already in arak client) |
| **Endpoints used** | 5 under `/products/v2/`, 2 under `/customers/v2/` |

**Already handled:** The arak client and the proxy pattern from the budget app are directly reusable. No new infra work needed for these 7 endpoints.

### 4. Keycloak (Auth)

| Aspect | Details |
|--------|---------|
| **Current** | Appsmith handles auth internally |
| **Target** | `@mrsmith/auth-client` package (OAuth2/OIDC, already in monorepo) |
| **Role needed** | `app_kitproducts_access` (per CLAUDE.md naming convention) |
| **API auth** | Bearer token from Keycloak, validated by Go backend |

---

## End-to-End User Journeys

### Journey 1: Create a new kit (most complex)

```
Kit List
  → Click "New Kit"
  → Modal opens: fill name, prefix, category, main product, pricing, subscriptions, sellable groups
  → Submit
  → Backend: products.new_kit(json) [atomic: creates kit + translations + customer groups]
  → Returns new kit ID
  → Auto-navigate to Kit Detail (Edit Kit) page with new ID
  → User edits details in 3 tabs:
     Tab 1: Adjust metadata fields, save kit, edit/save translations
     Tab 2: Add products (modal), set quantities/pricing/groups, batch save
     Tab 3: Add custom values (inline)
  → Click Back → return to Kit List (now includes new kit)
```

**Cross-system writes:** Postgres only (translations go to Postgres; Alyante sync only happens for Products, not Kit translations)

### Journey 2: Edit product descriptions (ERP dual-write)

```
Product List
  → Select a product row
  → Click "Edit descriptions"
  → Modal opens: pre-filled with IT/EN short+long descriptions
  → Edit descriptions, submit
  → Backend:
     1. UPSERT common.translation (IT) → Postgres
     2. UPSERT common.translation (EN) → Postgres
     3. UPDATE MG87_ARTDESC (ITA) → Alyante MSSQL [short only, code padded 25 chars]
     4. UPDATE MG87_ARTDESC (ING) → Alyante MSSQL [short only, code padded 25 chars]
  → Refresh product list
  → Close modal
```

**Cross-system writes:** Postgres + Alyante ERP. This is the only journey that touches the ERP.

### Journey 3: Manage kit discount rules

```
Kit Discounts
  → Left table: browse kits (from REST API)
  → Select a kit row
  → Right table: shows discount groups for that kit (from REST API)
  → To add: Click "+", modal shows only unassigned groups
     → Select group, set MRC/NRC percentages and signs, rounding, sellable
     → Submit → POST /products/v2/kit-discount (upsert) → refresh right table
  → To edit: Click row, modal pre-fills current values
     → Modify, submit → same POST endpoint → refresh
```

**Cross-system writes:** Mistra REST API only (the API handles DB writes internally)

### Journey 4: Simulate customer pricing (read-only)

```
Price Simulator
  → Select customer from dropdown
  → Kit table refreshes with discounted kits for that customer (REST API)
  → Select a kit row
  → Products table shows per-product pricing with discounts applied (REST API)
```

**No writes.** Pure REST API consumption.

### Journey 5: Clone a kit

```
Kit List
  → Select a kit row
  → More menu → "Clone Kit"
  → Modal: name pre-filled as "{name}-Copy"
  → Edit name, confirm
  → Backend: products.clone_kit(id, name) [deep-clones kit + products + custom values]
  → Refresh kit list
```

---

## Data Flow Diagram

```
                                    ┌─────────────┐
                                    │   Keycloak   │
                                    │  (OAuth2)    │
                                    └──────┬───────┘
                                           │ Bearer token
                                           ▼
┌──────────────────┐              ┌──────────────────┐
│   React Frontend │──── /api ───→│   Go Backend     │
│   (kit-products) │              │                  │
│                  │              │  ┌────────────┐  │
│  Kit List        │              │  │ kit-products│  │──── SQL ────→ db-mistra (Postgres)
│  Kit Detail      │              │  │  handlers   │  │                 products.*
│  Products        │              │  └────────────┘  │                 customers.*
│  Discounts       │              │                  │                 common.*
│  Simulator       │              │  ┌────────────┐  │
│  Categories      │              │  │ arak proxy  │  │──── REST ───→ Mistra NG API
│  Customer Groups │              │  └────────────┘  │                 /products/v2/*
│                  │              │                  │                 /customers/v2/*
│                  │              │  ┌────────────┐  │
│                  │              │  │ ERP adapter │  │──── MSSQL ──→ Alyante ERP
│                  │              │  └────────────┘  │                 MG87_ARTDESC
└──────────────────┘              └──────────────────┘
```

---

## Hidden Triggers and Automation

The audit found **no background processes, timers, cron jobs, or event-driven automation**. All operations are user-initiated.

The only implicit trigger is **cascading queries on widget state change**:
- Customer dropdown change → refresh kit list → auto-select row 0 → refresh products (Price Simulator)
- Kit row select → refresh discount groups (Kit Discounts)
- Kit row select → refresh help URL (Kit List)

These translate to standard React `useEffect` dependencies or event handlers — no background automation needed.

---

## Data Ownership Boundaries

| Data | Owner | kit-products App Access |
|------|-------|----------------------|
| `products.*` tables | **This app** (primary manager) | Full CRUD |
| `customers.customer_group` | **Shared** (also used by customer management, orders) | CRUD here, but changes affect other apps |
| `customers.customer` | **Customer management** (not this app) | Read-only |
| `common.translation` | **Shared** (used by any entity with translations) | CRUD for product/kit translations only |
| `common.vocabulary`, `common.custom_field_key` | **Shared** (admin/config) | Read-only |
| `orders.*` | **Order management** (not this app) | Not accessed, but references products.kit/product |
| Alyante `MG87_ARTDESC` | **ERP** (external system of record) | Write-only (sync short descriptions) |
| Mistra REST API | **Mistra NG platform** | Read + write for kit-discounts; read for pricing |

---

## Questions for Expert Review

**Q33.** (Repeated from above) Alyante failure strategy: **succeed-and-retry-later, or fail-atomically?**

**Q34.** The Go backend currently has **no Postgres connection** for the Mistra DB (only the arak HTTP client and the Anisetta DSN for compliance). **Should kit-products introduce a `MistraDSN` config**, or should all DB operations be wrapped in new Mistra API endpoints? Adding direct DB access is a significant architectural decision for the Go backend.

**Q35.** Orders reference `products.kit.id` and `products.product.code` without FK enforcement (for kit) or with FK (for product). **Should kit/product deletion (if added) check for order references before allowing delete?**

**Q36.** `customers.customer_group` is shared with other apps. **Are there other apps currently managing customer groups**, or is the Discount Groups page the only place they're edited? This affects whether we can change the data model.

**Q37.** The xmlParser JS library (fast-xml-parser 3.17.5) was loaded but appears unused. **Was it ever used for XML responses from Alyante or another system?** Safe to confirm it can be dropped.
