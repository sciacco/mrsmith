# Kit and Products — Implementation Plan

> **Spec source:** `apps/kit-products/kit-products-migspec-E.md`
> **Date:** 2026-04-07
> **Status:** Rev 3.1 — approved (minor doc nits fixed)

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

## Review Findings Resolution

### Finding 1 (Blocker) — DB/ERP runtime contract

The plan introduces two new external dependencies (`MISTRA_DSN`, `ALYANTE_DSN`) that must be wired through the full deployment chain, not just `config.go`.

**Runtime strategy: external shared DSNs**

Both Mistra (Postgres) and Alyante (MSSQL) are existing shared databases managed by infrastructure. The app does NOT own these databases and does NOT run migrations (coexistence constraint).

**Dependency requirements:**

| Env Var | Required? | Behavior when absent |
|---------|-----------|---------------------|
| `MISTRA_DSN` | **Required for kit-products to be functional.** | The kit-products catalog entry is marked **hidden** (not `ready`) when the DSN is absent. All `/kit-products/v1/*` DB-backed endpoints return 503. The app is effectively unavailable. Arak proxy endpoints still work if `ARAK_BASE_URL` is set. |
| `ALYANTE_DSN` | Optional. | ERP sync is silently skipped. No warning to user. Logged at startup (Info level). All other functionality works normally. |

In `main.go`, the catalog entry's visibility is conditional:
```go
if cfg.MistraDSN != "" {
    // kit-products is ready — register with ready status
} else {
    // kit-products hidden — DB not configured
}
```

In K8s `deployment.yaml`, `MISTRA_DSN` uses `optional: false` (unlike ANISETTA_DSN). `ALYANTE_DSN` uses `optional: true`.

**Concrete repo changes required:**

| File | Change |
|------|--------|
| `backend/internal/platform/config/config.go` | Add `MistraDSN` (env: `MISTRA_DSN`), `AlyanteDSN` (env: `ALYANTE_DSN`) |
| `.env.preprod.example` | Add `MISTRA_DSN=` and `ALYANTE_DSN=` with comments |
| `deploy/k8s/deployment.yaml` | Add `MISTRA_DSN` secret ref (`optional: false`) and `ALYANTE_DSN` secret ref (`optional: true`) |
| `docker-compose.dev.yaml` | Add env vars to backend service, pointing to dev Mistra/Alyante instances |
| `backend/cmd/server/main.go` | Open `mistraDB` via `database.New` if `MistraDSN` set. Open `alyanteDB` via MSSQL driver if `AlyanteDSN` set. Pass both to `kitproducts.RegisterRoutes`. |

**Backend verification strategy:**

| Level | Scope | Method |
|-------|-------|--------|
| **Unit** | Handler logic, request parsing, error paths | Fake `database/sql` driver (compliance pattern from `handler_logging_test.go`) |
| **Integration** | Stored procedure calls, transactions, ownership checks | Tests against a reachable Mistra Postgres (gated by `MISTRA_DSN` env var — skipped in CI if not set) |
| **ERP not configured** | `ALYANTE_DSN` absent — adapter is nil | Handler test: nil adapter → 200 success, **no warning** (silent no-op, logged at startup as Info) |
| **ERP sync failure** | `ALYANTE_DSN` configured but sync fails | Handler test with failing `AlyanteAdapter` test double (returns error) → 200 + `{ "warning": "erp_sync_failed" }` |
| **Role gates** | ACL middleware for `app_kitproducts_access` | Unit test with mock JWT claims (existing compliance pattern) |
| **Transaction rollback** | Batch update failures | Integration test: begin tx, fail mid-batch, verify no partial writes |

Tests are organized as:
- `backend/internal/kitproducts/handler_test.go` — unit tests (fake driver, always run)
- `backend/internal/kitproducts/integration_test.go` — build-tagged `//go:build integration`, require `MISTRA_DSN`

---

### Finding 2 (High) — Root dev script must include kit-products

The root `pnpm dev` script (`package.json` line 6) currently runs only backend, portal, budget, compliance. The plan's Phase 1 verification says `make dev` starts all apps — this is false unless the script is updated.

**Fix:** The root `scripts.dev` command must be updated to include kit-products:

```json
"dev": "concurrently --names backend,portal,budget,compliance,kit-products --prefix-colors blue,green,magenta,cyan,yellow \"cd backend && air\" \"pnpm --filter mrsmith-portal dev\" \"pnpm --filter mrsmith-budget dev\" \"pnpm --filter mrsmith-compliance dev\" \"pnpm --filter mrsmith-kit-products dev\""
```

Also add the standalone script:
```json
"dev:kit-products": "pnpm --filter mrsmith-kit-products dev"
```

This is part of Phase 1 scaffolding, not a later step.

---

### Finding 3 (High) — Mistra proxy contract fully specified

The upstream Mistra API uses paginated envelopes: `{ total_pages: int, items: [...] }` with required `page_number` query param on all list endpoints.

**Decision: pass-through proxy.** The Go backend proxies requests as-is (same as budget pattern). The frontend is responsible for sending required query params and consuming the paginated envelope.

**Proxy contract per route:**

| Local Path | Upstream | Required Query Params | Response Shape |
|-----------|---------|----------------------|---------------|
| `GET /kit-products/v1/mistra/kit` | `GET /products/v2/kit` | `page_number` (required), `disable_pagination`, `category_id`, `customer_group_id`, `commercial_profile_ids`, `only_ecommerce` | `PaginatedResponse<kit-brief>` |
| `GET /kit-products/v1/mistra/kit-discount` | `GET /products/v2/kit-discount` | `page_number` (required), `disable_pagination`, `customer_group_id`, `kit_id` | `PaginatedResponse<kit-discount>` |
| `POST /kit-products/v1/mistra/kit-discount` | `POST /products/v2/kit-discount` | — (body: `kit-discount-new`) | `{ message: string }` |
| `GET /kit-products/v1/mistra/discounted-kit` | `GET /products/v2/discounted-kit` | `page_number` (required), `disable_pagination`, `customer_id` (required), `category_id`, `only_ecommerce` | `PaginatedResponse<discounted-kit>` |
| `GET /kit-products/v1/mistra/discounted-kit/{id}` | `GET /products/v2/discounted-kit/{id}` | `customer_id` (required) | `discounted-kit-detail` (single object, not paginated) |
| `GET /kit-products/v1/mistra/customer` | `GET /customers/v2/customer` | `page_number` (required), `disable_pagination`, `search_string`, `customer_group_id`, `state_id` | `PaginatedResponse<customer>` |

**Upstream paginated envelope (forwarded verbatim):**

```typescript
interface PaginatedResponse<T> {
  total_number: number;   // total item count across all pages
  current_page: number;   // 1-based current page index
  total_pages: number;    // total page count
  items: T[];             // data array
}
```

All four fields are present in every list response from the Mistra API. The proxy forwards them unchanged.

**Frontend contract:**
- All list calls send `page_number=1&disable_pagination=true` to get full datasets (matching current Appsmith behavior)
- Response is always `PaginatedResponse<T>` — frontend reads `.items` array
- The `customer_id` param for discounted-kit endpoints comes from the customer dropdown selection
- The `kit_id` param for kit-discount comes from the selected kit row

---

### Finding 4 (Medium) — Dependency injection shape and asset_flow type corrected

**Handler struct (corrected):**

```go
type Handler struct {
    mistraDB *sql.DB          // Mistra Postgres (products, common, customers schemas)
    alyante  *AlyanteAdapter  // Alyante ERP (nil if not configured)
    arak     *arak.Client     // Mistra REST API proxy (nil if not configured)
}
```

**Constructor:**

```go
func RegisterRoutes(mux *http.ServeMux, mistraDB *sql.DB, alyante *AlyanteAdapter, arakCli *arak.Client) {
    h := &Handler{mistraDB: mistraDB, alyante: alyante, arak: arakCli}
    // ...
}
```

The `AlyanteAdapter` is constructed in `main.go` from the MSSQL `*sql.DB`:

```go
var alyanteAdapter *kitproducts.AlyanteAdapter
if alyanteDB != nil {
    alyanteAdapter = kitproducts.NewAlyanteAdapter(alyanteDB)
}
kitproducts.RegisterRoutes(api, mistraDB, alyanteAdapter, arakCli)
```

**Product update field types (corrected):**

```go
type ProductUpdateRequest struct {
    InternalName string  `json:"internal_name"`
    CategoryID   int     `json:"category_id"`    // integer FK
    NRC          float64 `json:"nrc"`
    MRC          float64 `json:"mrc"`
    ImgURL       *string `json:"img_url"`
    ERPSync      *bool   `json:"erp_sync"`
    AssetFlow    *string `json:"asset_flow"`     // string FK (varchar name key), NOT integer
}
```

`asset_flow` is a `varchar(50)` FK by name (e.g. `"activation"`, `"deactivation"`). The frontend sends the name string directly from the select dropdown value. The backend binds it as `$N` string parameter. No ID resolution needed.

---

### Finding 5 (Medium) — Warning toast variant

The current `ToastProvider` supports only `success` and `error`. The ERP best-effort path needs a warning state.

**Decision: extend the shared `ToastProvider`** with a `warning` type.

Changes to `packages/ui/src/components/Toast/`:

**`ToastProvider.tsx`** — add `'warning'` to `ToastType`:
```typescript
type ToastType = 'success' | 'error' | 'warning';
```

Add warning icon (triangle with exclamation) to the render switch.

**`Toast.module.css`** — add `.warning` class:
```css
.warning {
  background: rgba(245, 158, 11, 0.92);   /* amber, matches --color-warning */
  color: #fff;
  box-shadow: 0 8px 24px rgba(245, 158, 11, 0.3),
              0 2px 4px rgba(245, 158, 11, 0.2);
}
```

This is a minimal, non-breaking change. All existing `toast('message', 'success')` and `toast('message', 'error')` calls continue to work. The new `toast('message', 'warning')` is used only by kit-products for ERP sync failures.

This change is part of **Phase 1 scaffolding** so it's available from the start.

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
- `handler.go` — `Handler` struct with `mistraDB *sql.DB` + `alyante *AlyanteAdapter` + `arak *arak.Client`. `RegisterRoutes(mux, mistraDB, alyante, arakCli)`. Shared helpers: `requireDB`, `dbFailure`, `rowError`, `rowsDone`, `rollbackTx`.
- `alyante.go` — `AlyanteAdapter` struct with `db *sql.DB`. `NewAlyanteAdapter(db)` constructor. `SyncTranslation(ctx, code, lang, short) error` method.
- Empty handler files per domain area
- `handler_test.go` — initial test with fake driver verifying `requireDB` returns 503 when nil

**1.3 Infra wiring**

| File | Changes |
|------|---------|
| `backend/internal/platform/config/config.go` | Add `MistraDSN`, `AlyanteDSN`, `KitProductsAppURL` fields |
| `backend/cmd/server/main.go` | Open mistraDB if `MistraDSN` set. Open alyanteDB via MSSQL driver if `AlyanteDSN` set. Construct `AlyanteAdapter`. Call `kitproducts.RegisterRoutes(api, mistraDB, alyanteAdapter, arakCli)`. Add href override for kit-products. |
| `backend/internal/platform/applaunch/catalog.go` | Add `KitProductsAppID = "kit-e-prodotti"`, `kitProductsAccessRoles`, `KitProductsAccessRoles()`. Update catalog entry: href → `/apps/kit-products/`, AccessRoles → dedicated. |
| `backend/internal/platform/config/config.go` | Add `MistraDSN`, `AlyanteDSN`, `KitProductsAppURL`. Add port 5176 to CORS origins default. |
| `.env.preprod.example` | Add `MISTRA_DSN=` (required) and `ALYANTE_DSN=` (optional) with comments |
| `deploy/k8s/deployment.yaml` | Add `MISTRA_DSN` secret ref (`optional: false`) and `ALYANTE_DSN` secret ref (`optional: true`) |
| Root `package.json` | Update `scripts.dev` to include `mrsmith-kit-products`. Add `dev:kit-products`. |
| `packages/ui/src/components/Toast/ToastProvider.tsx` | Add `'warning'` to `ToastType` union + amber warning icon |
| `packages/ui/src/components/Toast/Toast.module.css` | Add `.warning` class (amber background) |
| `deploy/Dockerfile` | Add COPY line for kit-products dist |
| `docker-compose.dev.yaml` | Add kit-products service (port 5176) + volume |
| Root `package.json` | Add `dev:kit-products` script |
| `Makefile` | Add `dev-kit-products` target, update `.PHONY` |
| `go.mod` | Add MSSQL driver dependency |

**Verification:**
- `make dev` starts all apps **including kit-products** (root `scripts.dev` updated)
- `http://localhost:5176` shows empty app shell with auth working
- Portal card for "Kit e Prodotti" links to `/apps/kit-products/`
- `go test ./internal/kitproducts/...` passes (handler_test.go: requireDB returns 503 when nil)
- Warning toast variant renders correctly (can be verified in browser console: `toast('test', 'warning')`)

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

Already scaffolded in Phase 1 (`alyante.go`). Phase 3 implements the real SQL:
- `SyncTranslation(ctx, code, lang, shortDescription) error` — UPDATE MG87_ARTDESC with code padding (25 chars) and language mapping (it→ITA, en→ING)
- Returns non-nil error on MSSQL failure (caller decides how to surface it)

**ERP behavior contract (two distinct cases):**

| Case | Adapter State | Handler Behavior | Response | Log |
|------|--------------|-----------------|----------|-----|
| `ALYANTE_DSN` not set | `h.alyante == nil` | Skip ERP sync entirely | 200 success, **no warning** | Info at startup: "Alyante ERP adapter not configured" |
| `ALYANTE_DSN` set, sync fails | `h.alyante.SyncTranslation()` returns error | Postgres already committed, ERP failed | 200 success + `{ "warning": "erp_sync_failed", "message": "Salvato, ma sincronizzazione ERP fallita" }` | Error: full MSSQL error details |
| `ALYANTE_DSN` set, sync succeeds | `h.alyante.SyncTranslation()` returns nil | Normal success | 200 success, no warning | — |

**3.2 Backend: Product endpoints**

| Endpoint | Implementation |
|----------|---------------|
| `GET /kit-products/v1/product` | SELECT with category join + `common.get_translations(uuid)` |
| `POST /kit-products/v1/product` | INSERT product + INSERT IT/EN translations (empty). No Alyante write. |
| `PUT /kit-products/v1/product/{code}` | UPDATE product fields. `category_id` is integer FK. `asset_flow` is string name key (varchar FK, e.g. `"activation"`). |
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
| `DELETE /kit-products/v1/kit/{id}` | Soft-delete: `UPDATE products.kit SET is_active = false WHERE id = $1`. **No order-reference check** — soft-delete is safe because it only hides the kit from active lists; existing orders retain their `kit_id` reference and the row is not removed. The kit can be re-activated by setting `is_active = true` via the normal update endpoint. |
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

**6.1 Backend: Arak proxy endpoints (pass-through)**

All proxied endpoints forward query params and body as-is. Frontend is responsible for sending required upstream params. Responses are forwarded verbatim as `PaginatedResponse<T>` (see envelope definition in Finding 3 resolution) for list endpoints.

| Local Path | Upstream | Frontend Must Send |
|-----------|---------|-------------------|
| `GET /kit-products/v1/mistra/kit` | `GET /products/v2/kit` | `page_number=1&disable_pagination=true` |
| `GET /kit-products/v1/mistra/kit-discount` | `GET /products/v2/kit-discount` | `page_number=1&disable_pagination=true&kit_id={id}` |
| `POST /kit-products/v1/mistra/kit-discount` | `POST /products/v2/kit-discount` | Body: `kit-discount-new` JSON |
| `GET /kit-products/v1/mistra/discounted-kit` | `GET /products/v2/discounted-kit` | `page_number=1&disable_pagination=true&customer_id={id}` |
| `GET /kit-products/v1/mistra/discounted-kit/{id}` | `GET /products/v2/discounted-kit/{id}` | `customer_id={id}` |
| `GET /kit-products/v1/mistra/customer` | `GET /customers/v2/customer` | `page_number=1&disable_pagination=true` |

Implementation: `arakCli.Do(method, path, r.URL.RawQuery, body)` → write response to `w`. Follow budget proxy pattern.

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

```typescript
// Shared paginated envelope (matches upstream Mistra API verbatim)
PaginatedResponse<T> { total_number, current_page, total_pages, items: T[] }

// Entities
Kit, KitCreateRequest, KitUpdateRequest
KitProduct, KitProductCreateRequest, KitProductUpdateRequest
Product, ProductCreateRequest, ProductUpdateRequest, TranslationUpdateRequest
ProductCategory, ProductCategoryCreateRequest
CustomerGroup, CustomerGroupCreateRequest, CustomerGroupBatchUpdateRequest
KitCustomValue, KitCustomValueCreateRequest
AssetFlow, CustomFieldKey, VocabularyItem

// Proxied types (match upstream Mistra API shapes)
KitBrief                          // from GET /products/v2/kit
DiscountedKit, DiscountedKitDetail, RelatedProduct  // from GET /products/v2/discounted-kit
KitDiscountEntry, KitDiscountCreateRequest          // from GET/POST /products/v2/kit-discount
CustomerBrief                     // from GET /customers/v2/customer

// Frontend client wrapper (maps raw server responses, not a backend contract)
ApiResponse<T> { data: T; warning?: { code: string; message: string } }
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
├── handler_proxy.go            # Mistra API proxy (pass-through, kit-discount, discounted-kit, customer)
├── alyante.go                  # AlyanteAdapter struct + SyncTranslation method
├── models.go                   # Request/response structs
├── handler_test.go             # Unit tests (fake driver, role gates, error paths, ERP nil no-op, ERP failure warning)
└── integration_test.go         # Integration tests (//go:build integration, requires MISTRA_DSN)
```

---

## Open Decisions

| Item | Status |
|------|--------|
| Q35: Order reference check on kit soft-delete | **Resolved:** No check needed. Soft-delete only sets `is_active=false` — row stays, FK references intact. Re-activation possible via update. |
| Alyante retry mechanism for failed syncs | Post-MVP |
| Product list pagination (836 rows now, may grow) | Post-MVP if needed |
| MSSQL driver choice (`denisenkom` vs `microsoft`) | Resolve at Phase 3 |
