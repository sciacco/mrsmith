# Kit and Products — Implementation Plan

> **Spec source:** `apps/kit-products/kit-products-migspec-E.md`
> **Date:** 2026-04-07
> **Status:** Draft — awaiting approval

---

## Repo-Fit Checklist

### 1. Runtime Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| **Route/base path** | `/apps/kit-products/` (build), `/` (dev) | Budget pattern: `vite.config.ts` base conditional on mode |
| **Deep links** | SPA fallback handled by `staticspa` handler — auto-discovers `/apps/kit-products/index.html` | `backend/internal/platform/staticspa/handler.go` — generic, no changes needed |
| **Dev split-server** | `KIT_PRODUCTS_APP_URL` env var, default `http://localhost:5176` | Budget/compliance pattern in `main.go` lines 89-99 |
| **Catalog entry** | Update existing `kit-e-prodotti` entry: href → `/apps/kit-products/`, add dedicated access role, set status ready | `catalog.go:95-101` — already exists with placeholder href `/apps/mkt-sales/kit-e-prodotti` |

### 2. Dev Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| **Vite port** | `5176` (next after compliance 5175) | `docker-compose.dev.yaml` port assignments |
| **API proxy** | `/api` and `/config` → `http://localhost:8080` | Budget `vite.config.ts` |
| **Root scripts** | Add `dev:kit-products` to root `package.json` | Existing: `dev:budget`, `dev:compliance` |
| **Makefile** | Add `dev-kit-products` target + `.PHONY` entry | Existing: `dev-budget` pattern |
| **CORS** | Add port 5176 to `config.go` default CORS origins | `config.go:37` — currently has 5173,5174,5175 |
| **Docker compose** | Add `kit-products` service + named volume | `docker-compose.dev.yaml` — follow budget/compliance pattern |

### 3. Auth Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| **Keycloak role** | `app_kitproducts_access` | Convention: `app_{appname}_access` per CLAUDE.md |
| **Bearer auth** | All `/kit-products/v1/*` endpoints wrapped in `acl.RequireRole()` | Compliance pattern: `protect` closure in `RegisterRoutes` |
| **401/403** | Handled by existing `authMiddleware.Handler` on `/api/` mount | `main.go` middleware chain |
| **Frontend auth** | Same pattern as budget: fetch `/config` → init AuthProvider → Bearer on all API calls | `apps/budget/src/main.tsx` |

### 4. Data-Contract Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| **Kit PK** | `id` bigint, auto-generated (sequence) | `products.kit` table |
| **Product PK** | `code` varchar(32), **user-assigned** — creation API must accept client-provided ID | `products.product` table |
| **Active-only vs all** | Kit list: all (sorted active-first). Product list: all. Lookups: all. | Spec: no active-only filtering needed |
| **Nested resource ownership** | All sub-resource endpoints verify parent ownership: `kit_product.kit_id`, `kit_custom_value.kit_id` | IMPLEMENTATION-PLANNING.md requirement |

### 5. Deployment Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| **Dockerfile COPY** | `COPY --from=frontend /app/apps/kit-products/dist /static/apps/kit-products` | Existing pattern for budget/compliance |
| **Env vars** | `MISTRA_DSN` (new, Postgres), `ALYANTE_DSN` (new, MSSQL). Arak vars already exist. | `config.go` — add two new fields |
| **DB driver** | pgx v5 (already imported in `main.go`). MSSQL: add `github.com/denisenkom/go-mssqldb` or `github.com/microsoft/go-mssqldb` | Budget uses pgx already |
| **Migration story** | **No migrations** — coexistence constraint means zero schema changes | Expert decision: same DB, same schema as Appsmith |

### 6. Verification Fit

| Item | Decision |
|------|----------|
| **Transaction rollback** | Batch operations (kit products, customer groups) use `BeginTx` + deferred rollback. Compliance pattern. |
| **Deep-link refresh** | `staticspa` handler covers this automatically |
| **ERP failure** | Postgres commits, Alyante best-effort. Warning returned to frontend. Logged server-side. |
| **Structured logging** | `logging.FromContext(r.Context())` with component=kitproducts, operation name per handler |
| **Panic recovery** | Existing `middleware.Recover(logger)` on `/api/` mount |
| **Error sanitization** | `httputil.InternalError` pattern — log real error, return generic 500 to client |

---

## Implementation Sequence

### Phase 1 — Scaffolding (foundation)

**1.1 Frontend app scaffold**

Create `apps/kit-products/` with:
- `package.json` (name: `mrsmith-kit-products`, deps: `@mrsmith/ui`, `@mrsmith/auth-client`, `@mrsmith/api-client`, `react-router-dom`, `@tanstack/react-query`)
- `vite.config.ts` (port 5176, base `/apps/kit-products/` in build, proxy `/api` + `/config`)
- `tsconfig.json` (extends `../../tsconfig.base.json`)
- `index.html` (lang=it, data-theme=clean, DM Sans + JetBrains Mono fonts)
- `src/main.tsx` (auth bootstrap from `/config`, router basename from `BASE_URL`)
- `src/App.tsx` (AppShell + TabNav with 4 tabs + gear menu)
- `src/routes.tsx` (route definitions)
- `src/styles/global.css` (import clean theme)

**1.2 Backend module scaffold**

Create `backend/internal/kitproducts/`:
- `handler.go` — `Handler` struct with `db *sql.DB` + `arak *arak.Client`, `RegisterRoutes(mux, db, arakCli)`, shared helpers (`requireDB`, `dbFailure`, `rowError`, `rowsDone`, `rollbackTx`)
- Empty handler files per domain area

**1.3 Infra wiring**

| File | Changes |
|------|---------|
| `backend/internal/platform/config/config.go` | Add `MistraDSN`, `AlyanteDSN`, `KitProductsAppURL` fields |
| `backend/cmd/server/main.go` | Open mistraDB if `MistraDSN` set. Open alyante MSSQL if `AlyanteDSN` set. Call `kitproducts.RegisterRoutes(api, mistraDB, alyantDB, arakCli)`. Add href override for kit-products. |
| `backend/internal/platform/applaunch/catalog.go` | Add `KitProductsAppID = "kit-e-prodotti"`, `kitProductsAccessRoles`, `KitProductsAccessRoles()`. Update catalog entry: href → `/apps/kit-products/`, AccessRoles → dedicated. |
| `backend/internal/platform/config/config.go` | Add port 5176 to CORS origins default |
| `deploy/Dockerfile` | Add COPY line for kit-products dist |
| `docker-compose.dev.yaml` | Add kit-products service (port 5176) + volume |
| Root `package.json` | Add `dev:kit-products` script |
| `Makefile` | Add `dev-kit-products` target, update `.PHONY` |
| `go.mod` | Add MSSQL driver dependency |

**Verification:** `make dev` starts all apps. Navigating to `http://localhost:5176` shows empty app shell with auth. Portal card for "Kit e Prodotti" links to `/apps/kit-products/`.

---

### Phase 2 — Lookup endpoints + Categories + Customer Groups (simplest views first)

**2.1 Backend: Lookup endpoints**

| Endpoint | SQL |
|----------|-----|
| `GET /kit-products/v1/lookup/asset-flow` | `SELECT name, label FROM products.asset_flow ORDER BY name` |
| `GET /kit-products/v1/lookup/custom-field-key` | `SELECT key_name, key_description FROM common.custom_field_key ORDER BY key_description` |
| `GET /kit-products/v1/lookup/vocabulary?section=...` | `SELECT name as label, name as value FROM common.vocabulary WHERE section = $1 ORDER BY label` |

**2.2 Backend: Category CRUD**

| Endpoint | Implementation |
|----------|---------------|
| `GET /kit-products/v1/category` | SELECT * ORDER BY name |
| `POST /kit-products/v1/category` | INSERT (name, color) |
| `PUT /kit-products/v1/category/{id}` | UPDATE name, color WHERE id |

**2.3 Backend: Customer Group CRUD**

| Endpoint | Implementation |
|----------|---------------|
| `GET /kit-products/v1/customer-group` | SELECT id, name, is_default, is_partner, read_only, base_discount ORDER BY name |
| `POST /kit-products/v1/customer-group` | INSERT (name, is_partner) |
| `PATCH /kit-products/v1/customer-group` | Batch UPDATE in transaction. Reject updates to read_only rows. |

**2.4 Frontend: Settings views**

- `/settings/categories` — table with inline editing (name + color picker), add-new-row, per-row save
- `/settings/customer-groups` — table with inline editing (name, is_partner), add-new-row, batch save button. `read_only` disables editing.

**Verification:** Both settings views fully functional with CRUD. Gear menu navigates correctly.

---

### Phase 3 — Product CRUD + Alyante ERP adapter

**3.1 Backend: Alyante ERP adapter**

Create `backend/internal/kitproducts/alyante.go`:
- `type AlyanteAdapter struct { db *sql.DB }`
- `SyncTranslation(ctx, code, lang, shortDescription) error` — UPDATE MG87_ARTDESC with code padding (25 chars) and language mapping (it→ITA, en→ING)
- Graceful nil handling: if `db == nil`, log warning and return nil (ERP not configured)

**3.2 Backend: Product endpoints**

| Endpoint | Implementation |
|----------|---------------|
| `GET /kit-products/v1/product` | SELECT with category join + `common.get_translations(uuid)` |
| `POST /kit-products/v1/product` | INSERT product + INSERT IT/EN translations (empty). No Alyante write. |
| `PUT /kit-products/v1/product/{code}` | UPDATE product fields. Accepts category_id and asset_flow as IDs. |
| `PUT /kit-products/v1/product/{code}/translations` | Transaction: UPSERT IT + EN in common.translation. Then best-effort Alyante sync for short descriptions. Return warning on ERP failure. |

**3.3 Frontend: Product List view**

- `/products` — table with 7 inline-editable columns, per-row save/discard
- New Product modal (code user-assigned, asset_flow as select)
- Edit Descriptions modal (short/long IT/EN, multiline)
- Toast warning on ERP sync failure

**Verification:** Full product CRUD. Create product, edit inline, edit descriptions. Verify dual-write to Postgres + Alyante (if configured). Verify warning toast when Alyante unavailable.

---

### Phase 4 — Kit List + Kit Create + Clone

**4.1 Backend: Kit list and creation endpoints**

| Endpoint | Implementation |
|----------|---------------|
| `GET /kit-products/v1/kit` | SELECT * with category name/color resolution, ORDER BY is_active desc, internal_name |
| `POST /kit-products/v1/kit` | Call `products.new_kit(json)`. Map form fields to stored procedure JSON keys (f_internal_name, s_main_product, etc.). Return new kit ID. |
| `DELETE /kit-products/v1/kit/{id}` | UPDATE is_active = false |
| `POST /kit-products/v1/kit/{id}/clone` | Call `products.clone_kit(id, name)`. Return new kit ID. |

**4.2 Frontend: Kit List view**

- `/kit` — table with 8-9 default columns + column visibility toggle
- Category color-coded cells
- internal_name display: `{name} ({main_product_code})`
- Toolbar: Edit Kit → `/kit/:id`, New Kit → modal, More → Clone/Refresh/Soft-Delete
- New Kit modal (name, prefix, category, main product, pricing, subscriptions, sellable groups, ecommerce)
- Clone modal (name input, default "{name}-Copy")
- After create/clone → navigate to `/kit/:id`

**Verification:** Kit list loads with color-coded categories. Create kit → navigate to detail. Clone kit → refresh list. Soft-delete sets is_active=false.

---

### Phase 5 — Kit Detail (3 tabs)

**5.1 Backend: Kit detail endpoints**

| Endpoint | Implementation |
|----------|---------------|
| `GET /kit-products/v1/kit/{id}` | Kit fields + translations (via get_translations) + sellable group IDs + help_url |
| `PUT /kit-products/v1/kit/{id}` | Call `products.upd_kit(id, json)`. Map form fields. Handles customer group re-creation. |
| `PUT /kit-products/v1/kit/{id}/help` | UPSERT kit_help |
| `PUT /kit-products/v1/kit/{id}/translations` | Call `common.upd_translation(uuid, json)` |
| `GET /kit-products/v1/kit/{id}/products` | JOIN kit_product + product, ORDER BY position, group, name |
| `POST /kit-products/v1/kit/{id}/products` | Call `products.new_kit_product(json)` |
| `PUT /kit-products/v1/kit/{id}/products/{pid}` | Call `products.upd_kit_product(id, json)` |
| `PATCH /kit-products/v1/kit/{id}/products` | Batch upd_kit_product in transaction |
| `DELETE /kit-products/v1/kit/{id}/products/{pid}` | DELETE with kit_id ownership check |
| `GET /kit-products/v1/kit/{id}/custom-values` | SELECT with jsonb_pretty |
| `POST /kit-products/v1/kit/{id}/custom-values` | INSERT |
| `PUT /kit-products/v1/kit/{id}/custom-values/{cvid}` | UPDATE with kit_id ownership check |
| `DELETE /kit-products/v1/kit/{id}/custom-values/{cvid}` | DELETE with kit_id ownership check |

**5.2 Frontend: Kit Detail view**

- `/kit/:id` — tabbed editor with breadcrumb `← Tutti i Kit / Kit #{id}`
- Tab 1 (Dettagli): form with 16+ fields, bundle_prefix disabled on existing kits, billing period static select, sellable groups multi-select, help URL field. Separate save buttons for kit metadata and translations.
- Tab 2 (Prodotti): table with inline editing + toolbar (add/edit modal, batch save, delete with confirmation). Add/Edit modal with product select, group select, quantities, pricing, notes.
- Tab 3 (Valori Custom): table with inline editing + add-new-row + per-row delete. Key select from CustomFieldKey lookup.

**Verification:** Navigate from Kit List → Kit Detail. Edit all 3 tabs. Save metadata, save translations, batch-save products, add/delete products, manage custom values. Back button returns to Kit List.

---

### Phase 6 — Kit Discounts + Price Simulator (Mistra API proxy)

**6.1 Backend: Arak proxy endpoints**

| Local Path | Proxies To |
|-----------|-----------|
| `GET /kit-products/v1/mistra/kit` | `GET /products/v2/kit` |
| `GET /kit-products/v1/mistra/kit-discount` | `GET /products/v2/kit-discount` |
| `POST /kit-products/v1/mistra/kit-discount` | `POST /products/v2/kit-discount` |
| `GET /kit-products/v1/mistra/discounted-kit` | `GET /products/v2/discounted-kit` |
| `GET /kit-products/v1/mistra/discounted-kit/{id}` | `GET /products/v2/discounted-kit/{id}` |
| `GET /kit-products/v1/mistra/customer` | `GET /customers/v2/customer` |

Follow budget proxy pattern: `arakCli.Do(method, path, query, body)` → forward response.

**6.2 Frontend: Kit Discounts view**

- `/discounts` — master-detail (kit list left, discount groups right)
- Single modal for add/edit discount (title changes per mode)
- NRC defaults to MRC, MRC auto-fills from base_discount
- Max 100% for discounts

**6.3 Frontend: Price Simulator view**

- `/simulator` — customer dropdown → kits table → related products table
- Read-only, all data from proxy endpoints
- Flatten nested related_products with `.flatMap()`

**Verification:** Kit discounts: select kit, view discounts, add/edit via modal. Price simulator: select customer, browse kits, view per-product pricing. Both views use proxied Mistra API.

---

### Phase 7 — Polish + testing

- Error handling review (all endpoints return sanitized errors)
- Loading states (skeleton screens per UI/UX doc)
- Empty states
- Toast notifications for all save operations
- ERP sync warning toast
- Responsive behavior (stacks below 1000px for master-detail)
- `prefers-reduced-motion` support
- Keyboard accessibility (Escape closes modals, Enter submits)

---

## TypeScript Types (shared)

To be defined in `apps/kit-products/src/types/` or generated from API:

```
Kit, KitCreateRequest, KitUpdateRequest
KitProduct, KitProductCreateRequest, KitProductUpdateRequest
Product, ProductCreateRequest, ProductUpdateRequest, TranslationUpdateRequest
ProductCategory, ProductCategoryCreateRequest
CustomerGroup, CustomerGroupCreateRequest, CustomerGroupBatchUpdateRequest
KitCustomValue, KitCustomValueCreateRequest
AssetFlow, CustomFieldKey, VocabularyItem
DiscountedKit, DiscountedKitDetail, RelatedProduct
KitDiscount, KitDiscountCreateRequest
Customer
```

---

## File Tree (new files)

```
apps/kit-products/
├── package.json
├── vite.config.ts
├── tsconfig.json
├── index.html
├── src/
│   ├── main.tsx
│   ├── App.tsx
│   ├── routes.tsx
│   ├── styles/
│   │   └── global.css
│   ├── types/
│   │   └── index.ts
│   ├── api/
│   │   └── client.ts          # API client wrapper with typed methods
│   ├── components/
│   │   ├── KitList/
│   │   ├── KitDetail/
│   │   ├── ProductList/
│   │   ├── KitDiscounts/
│   │   ├── PriceSimulator/
│   │   └── Settings/
│   └── hooks/
│       └── useApi.ts           # react-query hooks per entity

backend/internal/kitproducts/
├── handler.go                  # Handler struct, RegisterRoutes, shared helpers
├── handler_kit.go              # Kit CRUD + clone + soft-delete
├── handler_kit_products.go     # KitProduct CRUD + batch
├── handler_kit_custom.go       # KitCustomValue CRUD
├── handler_kit_translations.go # Kit translation update
├── handler_product.go          # Product CRUD
├── handler_product_translations.go  # Product translation update + ERP dual-write
├── handler_category.go         # Category CRUD
├── handler_customer_group.go   # CustomerGroup CRUD + batch
├── handler_lookup.go           # Lookup endpoints (asset_flow, custom_field_key, vocabulary)
├── handler_proxy.go            # Mistra API proxy (kit-discount, discounted-kit, customer)
├── alyante.go                  # Alyante ERP adapter
└── models.go                   # Request/response structs
```

---

## Open Decisions

| Item | Status |
|------|--------|
| Q35: Order reference check on kit soft-delete | Deferred to implementation |
| Alyante retry mechanism for failed syncs | Post-MVP |
| Product list pagination (836 rows now, may grow) | Post-MVP if needed |
| MSSQL driver choice (`denisenkom` vs `microsoft`) | Resolve at Phase 3 |
