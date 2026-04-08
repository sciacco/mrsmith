# Listini e Sconti — Implementation Plan

> **Spec source:** `apps/listini-e-sconti/SPEC.md`
> **Date:** 2026-04-08
> **Status:** Draft — pending review

---

## Repo-Fit Checklist

### 1. Runtime Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| **Route/base path** | `/apps/listini-e-sconti/` (build), `/` (dev) | Budget/kit-products pattern: `vite.config.ts` base conditional on mode |
| **Deep links** | SPA fallback handled by `staticspa` handler — auto-discovers `/apps/listini-e-sconti/index.html` | `backend/internal/platform/staticspa/handler.go` — generic, no changes needed |
| **Dev split-server** | `LISTINI_APP_URL` env var, default `http://localhost:5177` | Budget/kit-products pattern in `main.go` lines 119-134 |
| **Catalog entry** | Update existing `listini-e-sconti` entry: href → `/apps/listini-e-sconti/`, add dedicated access role `app_listini_access`, set status `ready` | `catalog.go:127-134` — already exists with placeholder href `/apps/mkt-sales/listini-e-sconti` and `defaultAccessRoles` |

### 2. Dev Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| **Vite port** | `5177` (next after kit-products 5176) | Root `package.json` dev script, `config.go:43` CORS origins |
| **API proxy** | `/api` and `/config` → `http://localhost:8080` | Budget `vite.config.ts` |
| **Root scripts** | Add `dev:listini` to root `package.json`. Update `scripts.dev` to include `mrsmith-listini-e-sconti`. | Existing: `dev:budget`, `dev:compliance`, `dev:kit-products` |
| **Makefile** | Add `dev-listini` target + `.PHONY` entry | Existing: `dev-kit-products` pattern |
| **CORS** | Add port 5177 to `config.go` default CORS origins | `config.go:43` — currently has 5173,5174,5175,5176 |
| **Docker compose** | Add `listini-e-sconti` service to `docker-compose.dev.yaml` (port 5177, `VITE_DEV_BACKEND_URL=http://backend:8080`) + named volume | `docker-compose.dev.yaml` — follow kit-products service pattern |

### 3. Auth Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| **Keycloak role** | `app_listini_access` | Convention: `app_{appname}_access` per CLAUDE.md. Spec confirms. |
| **Bearer auth** | All `/listini/v1/*` endpoints wrapped in `acl.RequireRole()` | Compliance/kit-products pattern: `protect` closure in `RegisterRoutes` |
| **401/403** | Handled by existing `authMiddleware.Handler` on `/api/` mount | `main.go` middleware chain |
| **Frontend auth** | Same pattern as budget: fetch `/config` → init AuthProvider → Bearer on all API calls | `apps/budget/src/main.tsx` |
| **User identity** | `operated_by` field in credit transactions extracted via `auth.GetClaims(ctx)`. Fallback order: `claims.Email` → `claims.Name` → `claims.Subject`. | `backend/internal/auth/middleware.go:104-106` — `GetClaims` returns `Claims{Subject, Email, Name, Roles}`. `Name` is mapped from `preferred_username` at line 95. |

### 4. Data-Contract Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| **Customer PK (Mistra)** | `customers.customer.id` = ERP ID (integer) | `docs/IMPLEMENTATION-KNOWLEDGE.md` — verified mapping |
| **Customer PK (Grappa)** | `cli_fatturazione.id` = internal Grappa ID. Bridge: `codice_aggancio_gest` = ERP ID | `docs/IMPLEMENTATION-KNOWLEDGE.md` |
| **Kit PK** | `products.kit.id` bigint, auto-generated | Mistra schema |
| **IaaS Pricing PK** | `cdl_prezzo_risorse_iaas.id` auto-inc + unique on `id_anagrafica` | Grappa schema — UPSERT via `ON DUPLICATE KEY UPDATE` |
| **Credit Transaction PK** | `customer_credit_transaction.id` auto-inc. Immutable ledger — INSERT only | Mistra schema |
| **Custom Items PK** | Composite: `(key_label, customer_id)` — UPSERT via `ON CONFLICT` | Mistra schema |
| **Active-only vs all** | Grappa customers: `stato='attivo'` + `codice_aggancio_gest > 0` + exclusions. Mistra customers: all (ORDER BY name). Kits: `is_active = true AND ecommerce = false`. | Spec per-endpoint filters |
| **Nested resource ownership** | `/customers/:id/groups` — verify customer exists. `/customers/:id/transactions` — verify customer exists. `/grappa/customers/:id/*` — verify customer exists in Grappa. | IMPLEMENTATION-PLANNING.md requirement |

### 5. Deployment Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| **Dockerfile COPY** | `COPY --from=frontend /app/apps/listini-e-sconti/dist /static/apps/listini-e-sconti` | Existing pattern for budget/compliance/kit-products in `deploy/Dockerfile` |
| **Env vars — new** | `GRAPPA_DSN` (env: `GRAPPA_DSN`, **required** for listini), `HUBSPOT_API_KEY` (optional), `CARBONE_API_KEY` (optional), `LISTINI_APP_URL` (dev override) | `config.go` — add four new fields |
| **Env vars — existing** | `MISTRA_DSN` (already exists, **required** for listini) | `config.go:50` — already wired |
| **DB driver — MySQL** | Add `github.com/go-sql-driver/mysql` to `go.mod`. Import `_ "github.com/go-sql-driver/mysql"` in `main.go`. | Verified: NOT in current `go.mod`. No MySQL driver present. |
| **DB driver — pgx** | Already imported for Mistra | `main.go:13` |
| **Migration story** | **No migrations** — coexistence constraint means zero schema changes. Both Appsmith and new app access same databases. | Spec: coexistence requirement |
| **K8s deployment** | Add `GRAPPA_DSN` secret ref (`optional: false`), `HUBSPOT_API_KEY` secret ref (`optional: true`), `CARBONE_API_KEY` secret ref (`optional: true`) | `deploy/k8s/deployment.yaml` — follow MISTRA_DSN pattern |

### 6. Verification Fit

| Item | Decision |
|------|----------|
| **Transaction rollback** | Group sync (DELETE + INSERT) uses `BeginTx` + deferred rollback. Batch credit/discount updates use transactions. |
| **Deep-link refresh** | `staticspa` handler covers this automatically |
| **HubSpot failure** | All HubSpot calls are async, fire-and-forget. Failures logged server-side, never block the save response. Toast always shows success for the DB write. |
| **Structured logging** | `logging.FromContext(r.Context())` with `component=listini`, operation name per handler |
| **Panic recovery** | Existing `middleware.Recover(logger)` on `/api/` mount |
| **Error sanitization** | `httputil.InternalError` pattern — log real error, return generic 500 to client |
| **Coexistence validation** | Exclusion codes (385, 485) hardcoded to match Appsmith. No schema changes. Verify both apps can read/write same rows without conflict. |

---

## Review Findings

### Finding 1 (Blocker) — Grappa MySQL: new database driver and DSN

This app introduces the **first MySQL connection** in the codebase. No other app uses Grappa directly from Go.

**Requirements:**

1. Add Go MySQL driver: `github.com/go-sql-driver/mysql`
2. Add `GRAPPA_DSN` to `config.go` and wire through `main.go`
3. Open `grappaDB` via `database.New(database.Config{Driver: "mysql", DSN: cfg.GrappaDSN})`
4. Verify `database.New` supports `"mysql"` driver string — if not, use `sql.Open("mysql", dsn)` directly

**DSN format:** Standard MySQL DSN: `user:password@tcp(host:3306)/grappa?parseTime=true&charset=utf8mb4`

**Runtime strategy:**

| Env Var | Required? | Behavior when absent |
|---------|-----------|---------------------|
| `GRAPPA_DSN` | **Required for listini to be functional.** | The listini-e-sconti catalog entry is hidden when absent. All `/listini/v1/grappa/*` endpoints return 503. Mistra-only endpoints (kits, groups, credits, Timoo) also hidden — app ships as a unit. |
| `MISTRA_DSN` | **Required** (already exists) | Same behavior as kit-products: listini hidden when absent. |
| `HUBSPOT_API_KEY` | Optional | HubSpot audit silently skipped. Logged at startup (Info). All other functionality works. |
| `CARBONE_API_KEY` | Optional | PDF generation returns 503. Kit catalog browsing still works. |

**Catalog visibility in `main.go`:**
```go
if cfg.MistraDSN == "" || cfg.GrappaDSN == "" {
    // hide listini-e-sconti from catalog — requires both databases
}
```

**Concrete repo changes:**

| File | Change |
|------|--------|
| `backend/go.mod` | `go get github.com/go-sql-driver/mysql` |
| `backend/cmd/server/main.go` | Import `_ "github.com/go-sql-driver/mysql"`. Open `grappaDB`. Pass to `listini.RegisterRoutes`. Add href override for listini. Update catalog filter. |
| `backend/internal/platform/config/config.go` | Add `GrappaDSN`, `HubSpotAPIKey`, `CarboneAPIKey`, `ListiniAppURL` fields |
| `.env.preprod.example` | Add `GRAPPA_DSN=`, `HUBSPOT_API_KEY=`, `CARBONE_API_KEY=` |
| `deploy/k8s/deployment.yaml` | Add secret refs for all three |

---

### Finding 2 (High) — HubSpot service: cross-database company lookup

Three pages (IaaS Prezzi, IaaS Credito, Sconti Energia) create HubSpot audit notes/tasks after saves. All share the same company lookup pattern.

**Lookup path (per `docs/IMPLEMENTATION-KNOWLEDGE.md`):**

```
Grappa customer ID (cli_fatturazione.id)
    → SELECT codice_aggancio_gest FROM cli_fatturazione WHERE id = ?       (Grappa MySQL)
    → SELECT id FROM loader.hubs_company WHERE numero_azienda = ?::varchar (Mistra PG)
    → HubSpot company ID (bigint)
```

**Service design:**

```go
type HubSpotService struct {
    grappaDB  *sql.DB    // for ERP ID lookup
    mistraDB  *sql.DB    // for HubSpot ID lookup
    apiKey    string     // HubSpot API key
    httpCli   *http.Client
}

func NewHubSpotService(grappaDB, mistraDB *sql.DB, apiKey string) *HubSpotService
```

**Methods:**

| Method | Purpose |
|--------|---------|
| `LookupCompanyID(ctx, grappaCustomerID int) (int64, error)` | Cross-DB lookup: Grappa → ERP ID → HubSpot ID |
| `CreateNote(ctx, companyID int64, body string) error` | POST to HubSpot Notes API |
| `CreateTask(ctx, companyID int64, subject, body, assigneeEmail string) error` | POST to HubSpot Tasks API |
| `CreateNoteAsync(ctx, companyID int64, body string)` | Fire-and-forget goroutine wrapper |
| `CreateNoteAndTaskAsync(ctx, companyID int64, noteBody, taskSubject, taskBody, assigneeEmail string)` | Fire-and-forget for Sconti Energia |

**Async pattern:**
```go
func (s *HubSpotService) CreateNoteAsync(ctx context.Context, companyID int64, body string) {
    if s == nil || s.apiKey == "" {
        return // silently skip when not configured
    }
    logger := logging.FromContext(ctx)
    go func() {
        if err := s.CreateNote(context.Background(), companyID, body); err != nil {
            logger.Error("hubspot note failed", "company_id", companyID, "error", err)
        }
    }()
}
```

**HubSpot is nil-safe:** when `HUBSPOT_API_KEY` is empty, `NewHubSpotService` returns nil. All handler code does `if h.hubspot != nil { h.hubspot.CreateNoteAsync(...) }` — no-op when nil.

---

### Finding 3 (High) — TabNavGroup: grouped horizontal navigation

The app has 7 pages in 4 groups — too many for flat `TabNav`. A new `TabNavGroup` component is needed in `packages/ui/`.

**Component spec:**

```typescript
interface TabGroup {
  label: string;
  items: TabNavItem[];  // reuses existing TabNavItem { label, path }
}

interface TabNavGroupProps {
  groups: TabGroup[];
}
```

**Behavior:**

| Interaction | Result |
|------------|--------|
| Single-item group click | Navigate directly to the page |
| Multi-item group hover | Open dropdown showing child pages |
| Multi-item group click | Navigate to first child page |
| Dropdown item click | Navigate to that page |
| Mouse leaves dropdown | Close after 150ms delay (prevents accidental close) |
| Keyboard: Enter on group | Open dropdown or navigate (single-item) |
| Keyboard: Escape | Close dropdown |

**Active state:** Both the group label AND the active page in the dropdown are highlighted. The sliding indicator underlines the active group.

**Mobile (< 640px):** Hamburger button → vertical expandable sections (accordion pattern).

**CSS:** Dropdown uses `--ease-spring` animation (same as existing dropdown patterns in UI/UX doc). Positioned absolutely below the group tab.

**Files:**

| File | Content |
|------|---------|
| `packages/ui/src/components/TabNavGroup/TabNavGroup.tsx` | Component implementation |
| `packages/ui/src/components/TabNavGroup/TabNavGroup.module.css` | Styles |
| `packages/ui/src/index.ts` | Export `TabNavGroup` |

**Navigation config for listini-e-sconti:**

```typescript
const navGroups: TabGroup[] = [
  {
    label: 'Catalogo',
    items: [{ label: 'Kit di vendita', path: '/kit' }],
  },
  {
    label: 'Prezzi',
    items: [
      { label: 'IaaS Prezzi risorse', path: '/iaas-prezzi' },
      { label: 'Timoo Prezzi Partner', path: '/timoo-prezzi' },
    ],
  },
  {
    label: 'Sconti',
    items: [
      { label: 'Gruppi sconto', path: '/gruppi-sconto' },
      { label: 'Sconti Energia', path: '/sconti-energia' },
    ],
  },
  {
    label: 'Crediti',
    items: [
      { label: 'Crediti Omaggio IaaS', path: '/iaas-crediti' },
      { label: 'Gestione crediti', path: '/gestione-crediti' },
    ],
  },
];
```

---

### Finding 4 (High) — Carbone PDF service

Kit PDF generation uses Carbone Cloud API. The service is simple: send JSON data + template ID, receive PDF bytes.

**Service design:**

```go
type CarboneService struct {
    apiKey     string
    httpCli    *http.Client
    templateID string  // hardcoded for now (constant in code)
}

func NewCarboneService(apiKey, templateID string) *CarboneService
func (s *CarboneService) GeneratePDF(ctx context.Context, data any) ([]byte, string, error)
// Returns: PDF bytes, filename, error
```

**Carbone API flow:**
1. `POST https://api.carbone.io/render` with `{ template: templateID, data: {...}, convertTo: "pdf" }` + `Authorization: Bearer {apiKey}`
2. Response: `{ success: true, data: { renderId: "..." } }`
3. `GET https://api.carbone.io/render/{renderId}` → PDF binary

**Template ID:** Hardcoded constant in `carbone.go`. Tracked in `docs/TODO.md` for future portal admin module.

**Nil-safe:** When `CARBONE_API_KEY` is empty, the PDF endpoint returns 503 with `{"error": "pdf_generation_unavailable"}`. Kit browsing still works.

---

### Finding 5 (High) — Frontend API client: local wrapper required

The shared `@mrsmith/api-client` (`packages/api-client/src/client.ts`) exposes only `get`, `post`, `put`, `delete`, and `getBlob` (GET-only). It is missing:
- **`patch`** — needed by 3 endpoints (group sync, batch credits, batch discounts)
- **`postBlob`** — needed for authenticated PDF download (POST that returns binary)

**Decision: scaffold a local `src/api/client.ts`** following the kit-products pattern (`apps/kit-products/src/api/client.ts`). This is a thin hook (`useApiClient`) that wraps `fetch` with Bearer auth from `useOptionalAuth()` and exposes the full method set.

**Local API client:**

```typescript
// apps/listini-e-sconti/src/api/client.ts
import { ApiError } from '@mrsmith/api-client';
import { useMemo } from 'react';
import { useOptionalAuth } from '../hooks/useOptionalAuth';

interface ApiClient {
  get: <T>(path: string) => Promise<T>;
  post: <T>(path: string, body: unknown) => Promise<T>;
  put: <T>(path: string, body: unknown) => Promise<T>;
  patch: <T>(path: string, body: unknown) => Promise<T>;
  delete: <T>(path: string) => Promise<T>;
  postBlob: (path: string, body: unknown) => Promise<Blob>;
}

export function useApiClient(): ApiClient {
  const { getAccessToken } = useOptionalAuth();

  return useMemo(() => {
    async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
      const token = await getAccessToken();
      const res = await fetch(`/api${path}`, {
        method,
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: body ? JSON.stringify(body) : undefined,
      });

      if (!res.ok) {
        let payload: unknown;
        try { payload = await res.json(); } catch { payload = undefined; }
        throw new ApiError(res.status, res.statusText, path, payload);
      }

      if (res.status === 204) return undefined as T;
      return res.json() as Promise<T>;
    }

    async function postBlob(path: string, body: unknown): Promise<Blob> {
      const token = await getAccessToken();
      const res = await fetch(`/api${path}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: body ? JSON.stringify(body) : undefined,
      });

      if (!res.ok) {
        let payload: unknown;
        try { payload = await res.json(); } catch { payload = undefined; }
        throw new ApiError(res.status, res.statusText, path, payload);
      }

      return res.blob();
    }

    return {
      get: <T>(path: string) => request<T>('GET', path),
      post: <T>(path: string, body: unknown) => request<T>('POST', path, body),
      put: <T>(path: string, body: unknown) => request<T>('PUT', path, body),
      patch: <T>(path: string, body: unknown) => request<T>('PATCH', path, body),
      delete: <T>(path: string) => request<T>('DELETE', path),
      postBlob,
    };
  }, [getAccessToken]);
}
```

This provides `patch` for all batch endpoints and `postBlob` for authenticated PDF download. The shared `@mrsmith/api-client` is still imported for `ApiError` only.

---

### Finding 6 (Medium) — Dual-database handler injection

The listini handler needs **both** Mistra (PG) and Grappa (MySQL), plus two optional services.

**Handler struct:**

```go
type Handler struct {
    mistraDB *sql.DB          // Mistra PostgreSQL (kits, customers, groups, credits, Timoo)
    grappaDB *sql.DB          // Grappa MySQL (IaaS pricing, accounts, racks, Grappa customers)
    hubspot  *HubSpotService  // nil if HUBSPOT_API_KEY not set
    carbone  *CarboneService  // nil if CARBONE_API_KEY not set
}
```

**Constructor:**

```go
func RegisterRoutes(mux *http.ServeMux, mistraDB, grappaDB *sql.DB, hubspot *HubSpotService, carbone *CarboneService) {
    h := &Handler{mistraDB: mistraDB, grappaDB: grappaDB, hubspot: hubspot, carbone: carbone}
    protect := acl.RequireRole(ListiniAccessRoles()...)
    handle := func(pattern string, handler http.HandlerFunc) {
        mux.Handle(pattern, protect(http.HandlerFunc(handler)))
    }
    // ... register all 22 endpoints
}
```

**Wiring in `main.go`:**

```go
// Grappa DB (listini module — MySQL)
var grappaDB *sql.DB
if cfg.GrappaDSN != "" {
    var err error
    grappaDB, err = sql.Open("mysql", cfg.GrappaDSN)
    if err != nil {
        logger.Error("failed to connect to grappa", "component", "listini", "error", err)
        os.Exit(1)
    }
    logger.Info("grappa database connected", "component", "listini")
}

// HubSpot service (optional)
var hubspotSvc *listini.HubSpotService
if cfg.HubSpotAPIKey != "" && grappaDB != nil && mistraDB != nil {
    hubspotSvc = listini.NewHubSpotService(grappaDB, mistraDB, cfg.HubSpotAPIKey)
    logger.Info("hubspot service configured", "component", "listini")
} else {
    logger.Info("hubspot service not configured", "component", "listini")
}

// Carbone service (optional)
var carboneSvc *listini.CarboneService
if cfg.CarboneAPIKey != "" {
    carboneSvc = listini.NewCarboneService(cfg.CarboneAPIKey, listini.DefaultKitTemplateID)
    logger.Info("carbone service configured", "component", "listini")
}

listini.RegisterRoutes(api, mistraDB, grappaDB, hubspotSvc, carboneSvc)
```

---

### Finding 6 (Medium) — Verification strategy

| Level | Scope | Method |
|-------|-------|--------|
| **Unit** | Handler logic, request parsing, validation, error paths | Fake `database/sql` driver (compliance pattern) |
| **Integration** | SQL queries against real Mistra PG + Grappa MySQL | Gated by `MISTRA_DSN` + `GRAPPA_DSN` env vars — skipped in CI if not set |
| **HubSpot not configured** | `hubspot == nil` | Handler test: nil HubSpot service → 200 success, no side-effect |
| **HubSpot failure** | API call fails | Handler test with mock HTTP server returning 500 → 200 success (fire-and-forget), error logged |
| **Carbone not configured** | `carbone == nil` | Handler test: nil Carbone → 503 on PDF endpoint, other kit endpoints work |
| **Role gates** | ACL middleware for `app_listini_access` | Unit test with mock JWT claims |
| **Transaction rollback** | Group sync, batch updates | Integration test: begin tx, fail mid-batch, verify no partial writes |
| **Coexistence** | Exclusion codes match Appsmith | Integration test: verify customer lists exclude codes 385/485 as expected |

Tests organized as:
- `backend/internal/listini/handler_test.go` — unit tests (always run)
- `backend/internal/listini/integration_test.go` — `//go:build integration`, requires `MISTRA_DSN` + `GRAPPA_DSN`

---

## Implementation Sequence

### Phase 1 — Scaffolding (foundation)

**Goal:** Empty app shell running with auth, dual-DB wired, all infra in place, TabNavGroup component ready.

#### 1.1 Frontend app scaffold

Create `apps/listini-e-sconti/` with:

| File | Content |
|------|---------|
| `package.json` | name: `mrsmith-listini-e-sconti`, deps: `@mrsmith/ui`, `@mrsmith/auth-client`, `@mrsmith/api-client`, `react-router-dom`, `@tanstack/react-query` |
| `vite.config.ts` | Port 5177, base `/apps/listini-e-sconti/` in build, proxy `/api` + `/config` |
| `tsconfig.json` | Extends `../../tsconfig.base.json` |
| `index.html` | lang=it, data-theme=clean, DM Sans + JetBrains Mono fonts |
| `src/main.tsx` | Auth bootstrap from `/config`, router basename from `BASE_URL` (budget pattern) |
| `src/App.tsx` | `AppShell` + `TabNavGroup` with 4 groups |
| `src/routes.tsx` | Route definitions for all 7 pages (stub components) |
| `src/styles/global.css` | Import clean theme, keyframes (pageEnter, rowEnter, sectionEnter) |

**`vite.config.ts`:**
```typescript
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

const backendTarget = process.env.VITE_DEV_BACKEND_URL || 'http://localhost:8080';

export default defineConfig(({ command }) => ({
  base: command === 'build' ? '/apps/listini-e-sconti/' : '/',
  plugins: [react()],
  server: {
    port: 5177,
    proxy: {
      '/api': backendTarget,
      '/config': backendTarget,
    },
  },
}));
```

**Route structure:**
```typescript
const routes = [
  { path: '/kit', element: <KitPage /> },
  { path: '/iaas-prezzi', element: <IaaSPrezziPage /> },
  { path: '/timoo-prezzi', element: <TimooPrezziPage /> },
  { path: '/gruppi-sconto', element: <GruppiScontoPage /> },
  { path: '/sconti-energia', element: <ScontiEnergiaPage /> },
  { path: '/iaas-crediti', element: <IaaSCreditiPage /> },
  { path: '/gestione-crediti', element: <GestioneCreditiPage /> },
  { index: true, element: <Navigate to="/kit" replace /> },
];
```

#### 1.2 TabNavGroup component

Create `packages/ui/src/components/TabNavGroup/`:

**`TabNavGroup.tsx` — Props:**
```typescript
export interface TabGroup {
  label: string;
  items: TabNavItem[];
}

interface TabNavGroupProps {
  groups: TabGroup[];
}
```

**Key implementation details:**
- Reuses `useLocation` + `Link` from react-router (same as `TabNav`)
- Active group determined by matching any child item path against current pathname
- Sliding indicator reuses same `useRef` + `getBoundingClientRect` pattern as `TabNav`
- Dropdown: `position: absolute`, `top: 100%`, appears on `mouseenter` with 150ms `mouseleave` delay
- Animation: `transform: scale(0.98) translateY(-6px)` → `scale(1) translateY(0)` with `--ease-spring`
- Single-item groups: no dropdown, click navigates directly
- Mobile: behind hamburger menu, groups as expandable accordion sections

**`TabNavGroup.module.css` — Key classes:**
```css
.groups { display: flex; gap: var(--space-1); position: relative; border-bottom: 1px solid var(--color-border); }
.group { position: relative; padding: var(--space-3) var(--space-4); cursor: pointer; }
.groupActive { color: var(--color-accent); font-weight: 600; }
.dropdown {
  position: absolute; top: 100%; left: 0; min-width: 200px;
  background: var(--color-bg-elevated); border: 1px solid var(--color-border);
  border-radius: var(--radius-md); box-shadow: var(--shadow-lg);
  animation: dropdownEnter 0.25s var(--ease-spring) both;
  z-index: 10;
}
.dropdownItem { display: block; padding: var(--space-2) var(--space-4); }
.dropdownItemActive { background: var(--color-accent-subtle); color: var(--color-accent); font-weight: 600; }
.indicator { /* same sliding indicator as TabNav */ }
```

Update `packages/ui/src/index.ts` to export `TabNavGroup`.

#### 1.3 Backend module scaffold

Create `backend/internal/listini/`:

| File | Content |
|------|---------|
| `handler.go` | `Handler` struct with `mistraDB`, `grappaDB`, `hubspot`, `carbone`. `RegisterRoutes`. Shared helpers: `requireMistra`, `requireGrappa`, `dbFailure`. |
| `hubspot.go` | `HubSpotService` struct + `LookupCompanyID`, `CreateNote`, `CreateTask`, async wrappers |
| `carbone.go` | `CarboneService` struct + `GeneratePDF`. `DefaultKitTemplateID` constant. |
| `models.go` | Request/response structs (empty, filled per phase) |
| `handler_test.go` | Initial test: `requireMistra` returns 503 when nil, `requireGrappa` returns 503 when nil |

**`handler.go` route registration (all 22 endpoints):**

```go
func RegisterRoutes(mux *http.ServeMux, mistraDB, grappaDB *sql.DB, hubspot *HubSpotService, carbone *CarboneService) {
    h := &Handler{mistraDB: mistraDB, grappaDB: grappaDB, hubspot: hubspot, carbone: carbone}
    protect := acl.RequireRole(applaunch.ListiniAccessRoles()...)
    handle := func(pattern string, handler http.HandlerFunc) {
        mux.Handle(pattern, protect(http.HandlerFunc(handler)))
    }

    // ── Mistra: Customers ──
    handle("GET /listini/v1/customers", h.handleListCustomers)
    handle("GET /listini/v1/customers/erp-linked", h.handleListERPLinkedCustomers)

    // ── Mistra: Kits ──
    handle("GET /listini/v1/kits", h.handleListKits)
    handle("GET /listini/v1/kits/{id}/products", h.handleGetKitProducts)
    handle("GET /listini/v1/kits/{id}/help-url", h.handleGetKitHelpURL)
    handle("POST /listini/v1/kits/{id}/pdf", h.handleGenerateKitPDF)

    // ── Mistra: Customer Groups ──
    handle("GET /listini/v1/customer-groups", h.handleListCustomerGroups)
    handle("GET /listini/v1/customer-groups/{id}/kit-discounts", h.handleListKitDiscountsByGroup)
    handle("GET /listini/v1/customers/{id}/groups", h.handleGetCustomerGroups)
    handle("PATCH /listini/v1/customers/{id}/groups", h.handleSyncCustomerGroups)

    // ── Mistra: Credits ──
    handle("GET /listini/v1/customers/{id}/credit", h.handleGetCreditBalance)
    handle("GET /listini/v1/customers/{id}/transactions", h.handleListTransactions)
    handle("POST /listini/v1/customers/{id}/transactions", h.handleCreateTransaction)

    // ── Mistra: Timoo ──
    handle("GET /listini/v1/customers/{id}/pricing/timoo", h.handleGetTimooPricing)
    handle("PUT /listini/v1/customers/{id}/pricing/timoo", h.handleUpsertTimooPricing)

    // ── Grappa: Customers ──
    handle("GET /listini/v1/grappa/customers", h.handleListGrappaCustomers)

    // ── Grappa: IaaS Pricing ──
    handle("GET /listini/v1/grappa/customers/{id}/iaas-pricing", h.handleGetIaaSPricing)
    handle("POST /listini/v1/grappa/customers/{id}/iaas-pricing", h.handleUpsertIaaSPricing)

    // ── Grappa: IaaS Accounts ──
    handle("GET /listini/v1/grappa/iaas-accounts", h.handleListIaaSAccounts)
    handle("PATCH /listini/v1/grappa/iaas-accounts/credits", h.handleBatchUpdateIaaSCredits)

    // ── Grappa: Racks ──
    handle("GET /listini/v1/grappa/customers/{id}/racks", h.handleListCustomerRacks)
    handle("PATCH /listini/v1/grappa/racks/discounts", h.handleBatchUpdateRackDiscounts)
}
```

#### 1.4 Infra wiring

| File | Changes |
|------|---------|
| `backend/internal/platform/config/config.go` | Add `GrappaDSN string` (env: `GRAPPA_DSN`), `HubSpotAPIKey string` (env: `HUBSPOT_API_KEY`), `CarboneAPIKey string` (env: `CARBONE_API_KEY`), `ListiniAppURL string` (env: `LISTINI_APP_URL`). Add `,5177` to CORS origins default. |
| `backend/go.mod` | `go get github.com/go-sql-driver/mysql` |
| `backend/cmd/server/main.go` | Import MySQL driver. Open `grappaDB` if `GrappaDSN` set. Construct `HubSpotService` + `CarboneService`. Call `listini.RegisterRoutes(api, mistraDB, grappaDB, hubspotSvc, carboneSvc)`. Add href override for listini. Update catalog filter to hide listini when `MistraDSN == ""` or `GrappaDSN == ""`. |
| `backend/internal/platform/applaunch/catalog.go` | Add `ListiniAppID = "listini-e-sconti"`, `ListiniAppHref = "/apps/listini-e-sconti/"`, `listiniAccessRoles = []string{"app_listini_access"}`, `ListiniAccessRoles()`. Update existing catalog entry: href → `ListiniAppHref`, AccessRoles → `ListiniAccessRoles()`, Status → `"ready"`. |
| `.env.preprod.example` | Add `GRAPPA_DSN=`, `HUBSPOT_API_KEY=`, `CARBONE_API_KEY=` with comments |
| `deploy/k8s/deployment.yaml` | Add `GRAPPA_DSN` (`optional: false`), `HUBSPOT_API_KEY` (`optional: true`), `CARBONE_API_KEY` (`optional: true`) |
| `deploy/Dockerfile` | Add `COPY --from=frontend /app/apps/listini-e-sconti/dist /static/apps/listini-e-sconti` |
| Root `package.json` | Update `scripts.dev` to include `mrsmith-listini-e-sconti`. Add `"dev:listini"`. |
| `Makefile` | Add `dev-listini` target, update `.PHONY` |
| `docker-compose.dev.yaml` | Add `listini-e-sconti` service (port 5177, `VITE_DEV_BACKEND_URL=http://backend:8080`) + `listini_node_modules` named volume |

**`docker-compose.dev.yaml` addition:**
```yaml
  listini-e-sconti:
    image: node:20-slim
    working_dir: /app
    command: sh -c "corepack enable && pnpm install && pnpm --filter mrsmith-listini-e-sconti dev --host 0.0.0.0"
    volumes:
      - .:/app
      - listini_node_modules:/app/node_modules
    environment:
      - VITE_DEV_BACKEND_URL=http://backend:8080
    ports:
      - "5177:5177"
    depends_on:
      - backend
```
Add `listini_node_modules:` to the `volumes:` section at the bottom.

**Verification:**
- `make dev` starts all apps including listini-e-sconti
- `http://localhost:5177` shows empty app shell with auth working and TabNavGroup rendering
- Portal card for "Listini e Sconti" links to `/apps/listini-e-sconti/`
- `go test ./internal/listini/...` passes (requireMistra/requireGrappa returns 503 when nil)
- TabNavGroup renders groups correctly with dropdown behavior
- Backend compiles with MySQL driver imported

---

### Phase 2 — Kit Catalog (read-only browsing + PDF)

**Goal:** Kit di vendita page fully functional with master-detail card view and PDF export.

#### 2.1 Backend: Kit endpoints

**`handler_kit.go`:**

| Endpoint | Handler | SQL |
|----------|---------|-----|
| `GET /listini/v1/kits` | `handleListKits` | See below |
| `GET /listini/v1/kits/{id}/products` | `handleGetKitProducts` | See below |
| `GET /listini/v1/kits/{id}/help-url` | `handleGetKitHelpURL` | See below |
| `POST /listini/v1/kits/{id}/pdf` | `handleGenerateKitPDF` | Fetch kit + products, send to Carbone |

**SQL — List kits:**
```sql
SELECT k.id, k.internal_name, k.billing_period,
       k.initial_subscription_months, k.next_subscription_months,
       k.activation_time_days, k.category_id,
       pc.name AS category_name, pc.color AS category_color,
       k.is_main_prd_sellable, k.sconto_massimo,
       k.variable_billing, k.h24_assurance,
       k.sla_resolution_hours, k.notes
FROM products.kit k
JOIN products.product_category pc ON pc.id = k.category_id
WHERE k.is_active = true AND k.ecommerce = false
ORDER BY pc.name, k.internal_name
```

**SQL — Kit products (with conditional main product):**
```sql
-- Component products
SELECT kpg.name AS group_name, p.internal_name,
       kp.nrc, kp.mrc, kp.minimum, kp.maximum,
       kp.required, kp.position, p.code AS product_code
FROM products.kit_product kp
JOIN products.product p ON p.code = kp.product_code
LEFT JOIN products.kit_product_group kpg ON kpg.kit_id = kp.kit_id AND kpg.name = kp.group_name
WHERE kp.kit_id = $1
ORDER BY kp.position, kp.group_name, p.internal_name
```

```sql
-- Main product (included only when is_main_prd_sellable = true)
SELECT p.internal_name, k.nrc, k.mrc, p.code AS product_code
FROM products.kit k
JOIN products.product p ON p.code = k.main_product_code
WHERE k.id = $1 AND k.is_main_prd_sellable = true
```

In the handler, if the main product query returns a row, prepend it to the product list with `required=true`, `position=0`, `group_name="Prodotto principale"`.

**SQL — Kit help URL:**
```sql
SELECT help_url FROM products.kit_help WHERE kit_id = $1
```
Returns `{"help_url": "..."}` or `{"help_url": null}` if no row.

**PDF generation handler:**
```go
func (h *Handler) handleGenerateKitPDF(w http.ResponseWriter, r *http.Request) {
    if h.carbone == nil {
        httputil.JSON(w, http.StatusServiceUnavailable, map[string]string{"error": "pdf_generation_unavailable"})
        return
    }
    kitID := // parse from path
    // 1. Fetch kit metadata (same as handleListKits but for single kit)
    // 2. Fetch products (same as handleGetKitProducts)
    // 3. Build JSON payload matching Carbone template structure
    // 4. Call h.carbone.GeneratePDF(ctx, payload)
    // 5. Write PDF bytes to response with Content-Type: application/pdf
    w.Header().Set("Content-Type", "application/pdf")
    w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="kit-%d.pdf"`, kitID))
    w.Write(pdfBytes)
}
```

#### 2.2 Frontend: Kit di vendita page

**Layout — master-detail card view:**

```
┌─────────────────────────────────────────────────────────────────┐
│ TabNavGroup: [Catalogo▾] [Prezzi▾] [Sconti▾] [Crediti▾]        │
├──────────────┬──────────────────────────────────────────────────┤
│ Kit List     │ Kit Card (detail)                                │
│ (~250px)     │                                                  │
│              │ ┌─── Category Badge ── Kit Name ───────────────┐ │
│ [Search]     │ │                                              │ │
│              │ ├─── Metadata Grid (2 cols) ───────────────────┤ │
│ ▸ Category A │ │ Durata iniziale: 24 mesi                    │ │
│   Kit 1  ●   │ │ Rinnovo: 12 mesi                            │ │
│   Kit 2      │ │ Attivazione: 5 gg                           │ │
│              │ │ Fatturazione: Mensile                        │ │
│ ▸ Category B │ │ Sconto max: 10%                             │ │
│   Kit 3      │ │ Fatt. variabile: SI                         │ │
│   Kit 4      │ │ H24: NO                                     │ │
│              │ │ SLA ore: 8                                   │ │
│              │ ├─── Notes ────────────────────────────────────┤ │
│              │ │ Free-text notes block                        │ │
│              │ ├─── Product Table (grouped) ──────────────────┤ │
│              │ │ ▸ Gruppo A                                   │ │
│              │ │   Product 1   €10.00   €5.00                │ │
│              │ │   Product 2*  €20.00   €8.00                │ │
│              │ │ ▸ Gruppo B                                   │ │
│              │ │   Product 3   €15.00   €3.00                │ │
│              │ ├─── Actions ──────────────────────────────────┤ │
│              │ │ [Genera PDF]  [Supporto]                    │ │
│              │ ├─── Footer ───────────────────────────────────┤ │
│              │ │ Tutti i prezzi presenti sono IVA esclusa     │ │
│              │ └──────────────────────────────────────────────┘ │
└──────────────┴──────────────────────────────────────────────────┘
```

**Components:**

| Component | File | Props |
|-----------|------|-------|
| `KitPage` | `src/pages/KitPage.tsx` | — (page wrapper) |
| `KitList` | `src/components/Kit/KitList.tsx` | `kits: Kit[], selectedId: number | null, onSelect: (id) => void` |
| `KitCard` | `src/components/Kit/KitCard.tsx` | `kit: Kit, products: KitProduct[], helpUrl: string | null` |
| `KitMetadata` | `src/components/Kit/KitMetadata.tsx` | `kit: Kit` |
| `KitProductTable` | `src/components/Kit/KitProductTable.tsx` | `products: KitProduct[]` |

**Data flow:**
1. Page load → `GET /listini/v1/kits` → populate left list
2. Kit select → `GET /listini/v1/kits/:id/products` + `GET /listini/v1/kits/:id/help-url` → populate card
3. "Genera PDF" click → `POST /listini/v1/kits/:id/pdf` → browser downloads PDF
4. "Supporto" click → `window.open(helpUrl, '_blank')`

**Kit list features:**
- `SearchInput` at top for filtering by name
- Grouped by category with color badges (collapsible groups)
- Selected kit highlighted with accent bar (existing table row pattern)
- Skeleton loading while fetching

**Boolean display:** `variable_billing`, `h24_assurance` → render as `SI` (green) / `NO` (muted) with semantic color.

**PDF download auth:** POST via `postBlob` from the local API client (Bearer auth included), then create blob URL for download:
```typescript
const blob = await apiClient.postBlob(`/listini/v1/kits/${kitId}/pdf`, {});
const url = URL.createObjectURL(blob);
const a = document.createElement('a');
a.href = url;
a.download = `kit-${kitId}.pdf`;
a.click();
URL.revokeObjectURL(url);
```

**Verification:**
- Kit list loads with category grouping and search
- Selecting a kit shows the full card view with metadata, products, notes
- Product table grouped by `group_name` with NRC/MRC columns
- "Genera PDF" downloads a PDF (when Carbone configured) or shows error toast
- "Supporto" button visible only when help URL exists
- Responsive: stacks below 1000px

---

### Phase 3 — IaaS Pricing + Timoo Pricing

**Goal:** Both pricing pages functional with customer dropdown, form validation, and save.

#### 3.1 Backend: Customer list endpoints

**`handler_customer.go`:**

**SQL — All Mistra customers:**
```sql
SELECT id, name FROM customers.customer ORDER BY name
```

**SQL — ERP-linked customers (fatgamma > 0):**
```sql
SELECT c.id, c.name
FROM customers.customer c
JOIN loader.erp_clienti_provenienza ep ON ep.numero_azienda = c.id
WHERE ep.fatgamma > 0
ORDER BY c.name
```

**SQL — Active Grappa customers (with exclusions):**
```sql
SELECT id, intestazione, codice_aggancio_gest
FROM cli_fatturazione
WHERE stato = 'attivo'
  AND codice_aggancio_gest > 0
  AND codice_aggancio_gest NOT IN (385)
ORDER BY intestazione
```

Note: The exclusion set varies by caller. The endpoint accepts an optional `exclude` query param:
- Default (IaaS Prezzi): exclude `385`
- IaaS Credito: exclude `385, 485`
- Sconti Energia: no additional exclusions

Implementation: `exclude=385,485` parsed from query string, appended to the `NOT IN` clause.

#### 3.2 Backend: IaaS Pricing endpoints

**`handler_iaas_pricing.go`:**

**SQL — Get IaaS pricing (with fallback):**
```sql
SELECT charge_cpu, charge_ram_kvm, charge_ram_vmware,
       charge_pstor, charge_sstor, charge_ip, charge_prefix24
FROM cdl_prezzo_risorse_iaas
WHERE id_anagrafica = ?
UNION ALL
SELECT charge_cpu, charge_ram_kvm, charge_ram_vmware,
       charge_pstor, charge_sstor, charge_ip, charge_prefix24
FROM cdl_prezzo_risorse_iaas
WHERE id_anagrafica IS NULL
LIMIT 1
```

The first `SELECT` returns customer-specific pricing. If no rows, the `UNION ALL` falls through to the default (where `id_anagrafica IS NULL`). `LIMIT 1` ensures only one result.

Response includes an `is_default: bool` field so the frontend knows whether it's showing defaults or overrides.

**SQL — Upsert IaaS pricing:**
```sql
INSERT INTO cdl_prezzo_risorse_iaas
  (id_anagrafica, charge_cpu, charge_ram_kvm, charge_ram_vmware,
   charge_pstor, charge_sstor, charge_ip, charge_prefix24)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  charge_cpu = VALUES(charge_cpu),
  charge_ram_kvm = VALUES(charge_ram_kvm),
  charge_ram_vmware = VALUES(charge_ram_vmware),
  charge_pstor = VALUES(charge_pstor),
  charge_sstor = VALUES(charge_sstor),
  charge_ip = VALUES(charge_ip),
  charge_prefix24 = VALUES(charge_prefix24)
```

**Backend validation:**

```go
type IaaSPricingRequest struct {
    ChargeCPU      float64 `json:"charge_cpu"`
    ChargeRAMKVM   float64 `json:"charge_ram_kvm"`
    ChargeRAMVMware float64 `json:"charge_ram_vmware"`
    ChargePStor    float64 `json:"charge_pstor"`
    ChargeSStor    float64 `json:"charge_sstor"`
    ChargeIP       float64 `json:"charge_ip"`
    ChargePrefix24 *float64 `json:"charge_prefix24"` // optional, hidden from UI
}

var iaasValidation = map[string][2]float64{
    "charge_cpu":        {0.05, 0.1},
    "charge_ram_kvm":    {0.05, 0.2},
    "charge_ram_vmware": {0.18, 0.3},
    "charge_pstor":      {0.0005, 0.002},
    "charge_sstor":      {0.0005, 0.002},
    "charge_ip":         {0.02, 0},  // 0 = no max
}
```

Reject with 422 and field-level errors if any value is out of range.

**HubSpot audit (after successful upsert):**
```go
// Diff detection: compare old vs new values
// If any changed, create HubSpot note asynchronously
if h.hubspot != nil && hasChanges {
    // Lookup: Grappa customer ID → HubSpot company ID
    companyID, err := h.hubspot.LookupCompanyID(ctx, grappaCustomerID)
    if err == nil {
        body := formatIaaSPricingNote(oldPrices, newPrices, customerName)
        h.hubspot.CreateNoteAsync(ctx, companyID, body)
    }
}
```

#### 3.3 Backend: Timoo Pricing endpoints

**`handler_timoo.go`:**

**SQL — Get Timoo pricing (with fallback):**
```sql
SELECT prices FROM products.custom_items
WHERE key_label = 'timoo_indiretta' AND customer_id = $1
UNION ALL
SELECT prices FROM products.custom_items
WHERE key_label = 'timoo_indiretta' AND customer_id = -1
LIMIT 1
```

Response parses `prices` JSONB: `{"user_month": decimal, "se_month": decimal}`.
Includes `is_default: bool` field.

**SQL — Upsert Timoo pricing:**
```sql
INSERT INTO products.custom_items (key_label, customer_id, prices)
VALUES ('timoo_indiretta', $1, $2)
ON CONFLICT (key_label, customer_id) DO UPDATE
SET prices = EXCLUDED.prices
```

No min/max validation. No HubSpot audit (intentional per spec).

#### 3.4 Frontend: IaaS Prezzi risorse page

**Layout:**
```
┌──────────────────────────────────────────────────────┐
│ Customer dropdown (SingleSelect — Grappa customers)   │
│ Exclude: 385                                          │
├──────────────────────────────────────────────────────┤
│ Pricing form (2-column grid)                          │
│                                                       │
│ CPU (€/giorno)        [0.05 ─── 0.1]   [____]       │
│ RAM KVM (€/giorno)    [0.05 ─── 0.2]   [____]       │
│ RAM VMware (€/giorno) [0.18 ─── 0.3]   [____]       │
│ Disco primario (€/GB) [0.0005 ─ 0.002] [____]       │
│ Disco secondario      [0.0005 ─ 0.002] [____]       │
│ IP pubblico (€/g)     [0.02 ─── ∞]     [____]       │
│                                                       │
│ [Salva]                                               │
└──────────────────────────────────────────────────────┘
```

**Behavior:**
- Page load → fetch Grappa customer list (exclude=385)
- Customer select → `GET /grappa/customers/:id/iaas-pricing` → populate form
- Form shows min/max as `input[type=number]` with `min`/`max`/`step` attributes
- If showing defaults, display info badge: "Valori predefiniti — nessun prezzo personalizzato"
- Save → `POST /grappa/customers/:id/iaas-pricing` → toast success
- `charge_prefix24` NOT shown in UI (hidden field, sent as-is if present)

#### 3.5 Frontend: Timoo Prezzi Partner page

**Layout:**
```
┌──────────────────────────────────────────────────────┐
│ Customer dropdown (SingleSelect — Mistra customers)   │
├──────────────────────────────────────────────────────┤
│ Pricing form                                          │
│                                                       │
│ Prezzo utente/mese (€)   [____]                      │
│ Prezzo SE/mese (€)       [____]                      │
│                                                       │
│ [Salva]                                               │
└──────────────────────────────────────────────────────┘
```

**Behavior:**
- Page load → fetch Mistra customer list
- Customer select → `GET /customers/:id/pricing/timoo` → populate form
- No min/max validation
- If showing defaults, display info badge
- Save → `PUT /customers/:id/pricing/timoo` → toast success

**Verification:**
- IaaS Prezzi: select customer, see prices (or defaults), edit within min/max, save, toast
- IaaS Prezzi: out-of-range input rejected both client-side and server-side (422)
- Timoo: select customer, see prices (or defaults), edit, save, toast
- Timoo: UPSERT — creating a new price and updating an existing one both work
- HubSpot note created asynchronously after IaaS price change (when configured)

---

### Phase 4 — IaaS Credits + Rack Discounts

**Goal:** Inline-edit table views with batch save and HubSpot integration for both Grappa-backed pages.

#### 4.1 Backend: IaaS Account endpoints

**`handler_iaas_accounts.go`:**

**SQL — List IaaS accounts:**
```sql
SELECT ca.domainuuid, ca.id_cli_fatturazione, ca.abbreviazione,
       ca.serialnumber, ca.data_attivazione, ca.credito,
       cf.intestazione,
       cs.infrastructure_platform
FROM cdl_accounts ca
JOIN cli_fatturazione cf ON cf.id = ca.id_cli_fatturazione
JOIN cdl_services cs ON cs.name = ca.cdl_service
WHERE ca.attivo = 1
  AND ca.fatturazione = 1
  AND cf.codice_aggancio_gest NOT IN (385, 485)
ORDER BY cf.intestazione, ca.abbreviazione
```

**SQL — Batch update credits:**
```go
// In a transaction:
for _, item := range req.Items {
    _, err := tx.ExecContext(ctx,
        `UPDATE cdl_accounts SET credito = ? WHERE domainuuid = ? AND id_cli_fatturazione = ?`,
        item.Credito, item.DomainUUID, item.IDCliFatturazione)
}
```

**Backend validation:**
- `infrastructure_platform` must be `'cloudstack'` for the row being updated (enforced by re-querying the platform before update)
- Credit value: no explicit min/max in spec

**HubSpot audit (per changed row):**
```go
for _, changed := range changedRows {
    companyID, err := h.hubspot.LookupCompanyID(ctx, changed.IDCliFatturazione)
    if err == nil {
        body := fmt.Sprintf("Credito IaaS aggiornato: %s → %s (account: %s)",
            formatDecimal(changed.OldCredito), formatDecimal(changed.NewCredito), changed.Abbreviazione)
        h.hubspot.CreateNoteAsync(ctx, companyID, body)
    }
}
```

#### 4.2 Backend: Rack Discount endpoints

**`handler_racks.go`:**

**SQL — List customer racks:**
```sql
SELECT r.id_rack, r.name, r.floor, r.island, r.type, r.sconto,
       dc.name AS room,
       db.name AS building
FROM racks r
JOIN datacenter dc ON dc.id_datacenter = r.id_datacenter
JOIN dc_build db ON db.id = dc.dc_build_id
WHERE r.id_anagrafica = (
    SELECT id FROM cli_fatturazione WHERE id = ? AND stato = 'attivo'
)
AND r.stato = 'attivo'
ORDER BY db.name, dc.name, r.name
```

**Rack customer dropdown** — dedicated endpoint `GET /listini/v1/grappa/rack-customers`:

This is a separate endpoint (not a query-param variant of the generic customer list) because its join logic is materially different: it filters to customers who have racks joined to `rack_sockets`, matching the audited Appsmith query (`AUDIT.md:205`).

```sql
SELECT DISTINCT cf.id, cf.intestazione
FROM cli_fatturazione cf
JOIN racks r ON r.id_anagrafica = cf.id
JOIN rack_sockets rs ON rs.rack_id = r.id_rack
WHERE r.stato = 'attivo'
ORDER BY cf.intestazione
```

> **Note:** The audited Appsmith query does NOT filter on `rack_sockets.status` or `cli_fatturazione.stato`. This plan preserves that exact behavior. If a tighter filter is needed, it must be verified against live data first and promoted to a spec change.

Add this endpoint to the route registration in `handler.go`:
```go
handle("GET /listini/v1/grappa/rack-customers", h.handleListRackCustomers)
```

**SQL — Batch update rack discounts:**
```go
for _, item := range req.Items {
    _, err := tx.ExecContext(ctx,
        `UPDATE racks SET sconto = ? WHERE id_rack = ?`,
        item.Sconto, item.IDRack)
}
```

**Backend validation:**
- Discount range: 0–20% (reject with 422 if out of range)

**HubSpot audit (note + task):**
```go
// Build HTML table of changes
var rows []string
for _, changed := range changedRacks {
    rows = append(rows, fmt.Sprintf("<tr><td>%s</td><td>%s%%</td><td>%s%%</td></tr>",
        changed.RackName, formatDecimal(changed.OldSconto), formatDecimal(changed.NewSconto)))
}
noteBody := fmt.Sprintf("<table><tr><th>Rack</th><th>Vecchio</th><th>Nuovo</th></tr>%s</table>",
    strings.Join(rows, ""))

companyID, err := h.hubspot.LookupCompanyID(ctx, grappaCustomerID)
if err == nil {
    h.hubspot.CreateNoteAndTaskAsync(ctx, companyID,
        noteBody,
        "Aggiornamento sconto energia",
        noteBody,
        "eva.grimaldi@cdlan.it",  // hardcoded per spec; TODO: make configurable
    )
}
```

#### 4.3 Frontend: IaaS Credito omaggio page

**Layout:**
```
┌──────────────────────────────────────────────────────────────────┐
│ Inline-edit table (all accounts loaded on page mount)             │
├────────────────┬──────────┬──────────┬───────────┬──────────────┤
│ Cliente        │ Account  │ Credito  │ Piattaf.  │ Attivazione  │
├────────────────┼──────────┼──────────┼───────────┼──────────────┤
│ Acme Corp      │ acme-01  │ [500.00] │ cloudstack│ 2024-01-15   │ ← editable
│ Acme Corp      │ acme-02  │  200.00  │ vmware    │ 2023-06-01   │ ← muted (not cloudstack)
│ Beta Inc       │ beta-01  │ [100.00] │ cloudstack│ 2024-03-20   │ ← editable
├────────────────┴──────────┴──────────┴───────────┴──────────────┤
│ [Salva modifiche] (disabled when no dirty rows)                  │
└──────────────────────────────────────────────────────────────────┘
```

**Behavior:**
- Page load → `GET /grappa/iaas-accounts` → populate table
- Non-CloudStack rows: `credito` cell is read-only, row has `opacity: 0.5`
- CloudStack rows: `credito` cell is `input[type=number]`
- Dirty-row tracking: when a value changes, mark row as dirty, enable save button
- Save → `PATCH /grappa/iaas-accounts/credits` with only changed rows → toast success

#### 4.4 Frontend: Sconti variabile energia page

**Layout:**
```
┌──────────────────────────────────────────────────────┐
│ Customer dropdown (Grappa customers with rack sockets) │
├───────────────┬──────────┬────────┬────────┬─────────┤
│ Rack          │ Edificio │ Sala   │ Piano  │ Sconto  │
├───────────────┼──────────┼────────┼────────┼─────────┤
│ RACK-A01      │ DC1      │ Sala A │ 1      │ [10.00] │
│ RACK-A02      │ DC1      │ Sala A │ 1      │ [ 5.00] │
│ RACK-B01      │ DC2      │ Sala B │ 2      │ [15.00] │
├───────────────┴──────────┴────────┴────────┴─────────┤
│ [Salva modifiche] (disabled when no dirty rows)       │
└──────────────────────────────────────────────────────┘
```

**Behavior:**
- Page load → fetch rack customer list (`GET /grappa/rack-customers`). **No rack query until customer selected.**
- Customer select → `GET /grappa/customers/:id/racks` → populate table
- Discount input: `input[type=number]` with `min=0` `max=20` `step=0.01`
- Dirty-row tracking, batch save
- Save → `PATCH /grappa/racks/discounts` → toast success

**Verification:**
- IaaS Crediti: all accounts load, CloudStack rows editable, others muted, batch save works
- Sconti Energia: select customer, see racks, edit discounts within 0–20%, batch save works
- Backend rejects discount > 20% with 422
- HubSpot note created for IaaS credit changes (when configured)
- HubSpot note + task created for rack discount changes (when configured)
- Transaction rollback: if one row fails in batch, none are committed

---

### Phase 5 — Customer Groups + Customer Credits

**Goal:** Group management with modal editor and immutable credit ledger.

#### 5.1 Backend: Customer Group endpoints

**`handler_groups.go`:**

**SQL — List all groups:**
```sql
SELECT id, name FROM customers.customer_group ORDER BY name
```

**SQL — Get customer's groups:**
```sql
SELECT ga.group_id
FROM customers.group_association ga
WHERE ga.customer_id = $1
```

> **Compatibility note — `group_association.active` column:**
> The schema marks `active` as `DEPRECATED, DO NOT USE !!!!` (see `mistra_customers.json:901`).
> However, Appsmith currently writes `active = true` on INSERT. During coexistence, reads
> must NOT filter on `active` (to see all rows regardless of flag state), and writes must
> still set `active = true` on INSERT to avoid breaking Appsmith's view of the data.
> Once Appsmith is decommissioned, the `active` column and the `active = true` write can
> be removed in a cleanup pass.

**SQL — Sync customer groups (transactional diff):**
```go
func (h *Handler) handleSyncCustomerGroups(w http.ResponseWriter, r *http.Request) {
    customerID := // parse from path
    var req struct { GroupIDs []int `json:"groupIds"` }
    // decode body

    tx, _ := h.mistraDB.BeginTx(ctx, nil)
    defer tx.Rollback()

    // Get current associations (do NOT filter on deprecated `active` column)
    rows, _ := tx.QueryContext(ctx,
        `SELECT group_id FROM customers.group_association WHERE customer_id = $1`,
        customerID)
    var current []int
    // scan rows

    // Compute diff
    toAdd := diff(req.GroupIDs, current)     // in desired but not current
    toRemove := diff(current, req.GroupIDs)   // in current but not desired

    // Remove (hard delete — matches spec's diff-based sync)
    for _, gid := range toRemove {
        tx.ExecContext(ctx,
            `DELETE FROM customers.group_association WHERE customer_id = $1 AND group_id = $2`,
            customerID, gid)
    }

    // Add (set active = true for Appsmith coexistence — see compatibility note above)
    for _, gid := range toAdd {
        tx.ExecContext(ctx,
            `INSERT INTO customers.group_association (customer_id, group_id, active)
             VALUES ($1, $2, true)
             ON CONFLICT DO NOTHING`,
            customerID, gid)
    }

    tx.Commit()
}
```

**SQL — Kit discounts by group:**
```sql
SELECT kcg.kit_id, k.internal_name AS kit_name,
       kcg.discount_mrc, kcg.discount_nrc
FROM products.kit_customer_group kcg
JOIN products.kit k ON k.id = kcg.kit_id
WHERE kcg.group_id = $1
  AND k.is_active = true
ORDER BY k.internal_name
```

#### 5.2 Backend: Credit endpoints

**`handler_credits.go`:**

**SQL — Get credit balance:**
```sql
SELECT credit FROM customers.customer_credits WHERE customer_id = $1
```

Returns `{"balance": decimal}` or `{"balance": 0}` if no row.

**SQL — List transactions:**
```sql
SELECT id, transaction_date, amount, operation_sign, description, operated_by
FROM customers.customer_credit_transaction
WHERE customer_id = $1
ORDER BY transaction_date DESC, id DESC
```

**SQL — Create transaction:**
```sql
INSERT INTO customers.customer_credit_transaction
  (customer_id, amount, operation_sign, description, operated_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, transaction_date
```

**Backend validation:**
```go
type TransactionRequest struct {
    Amount        float64 `json:"amount"`
    OperationSign string  `json:"operation_sign"` // "+" or "-"
    Description   string  `json:"description"`
}

// Validation:
// amount: 0 < amount <= 10000
// operation_sign: must be "+" or "-"
// description: required, max 255 chars
// operated_by: extracted from JWT via auth.GetClaims (fallback: Email → Name → Subject)
```

**User identity extraction:**
```go
claims, _ := auth.GetClaims(r.Context())
operatedBy := claims.Email
if operatedBy == "" {
    operatedBy = claims.Name  // preferred_username mapped to Name by middleware
}
if operatedBy == "" {
    operatedBy = claims.Subject
}
```

#### 5.3 Frontend: Gruppi di sconto x clienti page

**Layout:**
```
┌──────────────────────────────────────────────────────────────────────┐
│ Customer dropdown (Mistra ERP-linked customers, fatgamma > 0)        │
├─────────────────┬───────────────────────┬────────────────────────────┤
│ Customer Info   │ Gruppi associati      │ Sconti kit per gruppo      │
│                 │                       │                            │
│ Selected:       │ • Gruppo Standard     │ Kit CORE       MRC: 5%    │
│ Acme Corp       │ • Gruppo Partner      │ Kit PLUS       MRC: 10%   │
│                 │                       │ Kit PRO        MRC: 15%   │
│                 │ [Associa gruppi]      │                            │
│                 │ (tooltip: "Gestisci   │ Select a group to see     │
│                 │  associazioni")       │ kit discounts              │
├─────────────────┴───────────────────────┴────────────────────────────┤
└──────────────────────────────────────────────────────────────────────┘
```

**Modal — "Associa gruppi":**
- Opens `MultiSelect` with all available groups
- Pre-selected: customer's current groups
- Save → `PATCH /customers/:id/groups` with `{groupIds: [...]}` → toast, refresh

**Kit discounts panel:**
- Clicking a group in the middle column → `GET /customer-groups/:id/kit-discounts` → show right panel
- Read-only table: Kit name, MRC discount %, NRC discount %

#### 5.4 Frontend: Gestione credito cliente page

**Layout:**
```
┌──────────────────────────────────────────────────────────────────┐
│ Customer dropdown (Mistra customers)                              │
├──────────────────────────────────────────────────────────────────┤
│ Saldo attuale: € 1,250.00                                       │
├──────────────────────────────────────────────────────────────────┤
│ Transaction history                                              │
├──────────┬──────────┬─────┬──────────────────────┬──────────────┤
│ Data     │ Importo  │ +/- │ Descrizione          │ Operatore    │
├──────────┼──────────┼─────┼──────────────────────┼──────────────┤
│ 08/04/26 │ €500.00  │ +   │ Accredito promo Q2   │ mario@cdlan  │
│ 01/04/26 │ €250.00  │ -   │ Storno errore        │ anna@cdlan   │
│ 15/03/26 │ €1000.00 │ +   │ Credito iniziale     │ mario@cdlan  │
├──────────┴──────────┴─────┴──────────────────────┴──────────────┤
│ [Nuova transazione]                                              │
└──────────────────────────────────────────────────────────────────┘
```

**Modal — "Nuova transazione":**
```
┌─────────────────────────────────────────┐
│ Nuova transazione                       │
│                                         │
│ Importo (€)    [______] (0 – 10000)    │
│ Operazione     (●) Accredito (+)       │
│                (○) Debito (-)          │
│ Descrizione    [________________________│
│                 ________________________]│
│                 (obbligatorio, max 255) │
│                                         │
│ [Annulla]              [Registra]       │
└─────────────────────────────────────────┘
```

**Behavior:**
- Page load → fetch Mistra customer list. **No query until customer selected.**
- Customer select → `GET /customers/:id/credit` + `GET /customers/:id/transactions`
- "Nuova transazione" → modal → `POST /customers/:id/transactions` → toast, refresh list + balance
- No edit/delete on transactions (immutable ledger)
- `operated_by` not shown in modal — captured server-side from JWT

**Verification:**
- Gruppi sconto: select customer, see groups, associate/disassociate via modal, see kit discounts per group
- Group sync is transactional: if middle of diff fails, no partial writes
- Gestione crediti: select customer, see balance + history, add transaction (accredito/debito)
- Transaction amount validated 0–10000 both client and server
- Description required, max 255
- `operated_by` populated from Keycloak JWT (not user input)
- No edit/delete on existing transactions

---

### Phase 6 — Integration & Polish

**Goal:** End-to-end validation, HubSpot verified, coexistence checked, accessibility review.

#### 6.1 End-to-end integration

| Test | Method |
|------|--------|
| All 7 pages load without errors | Manual browser test |
| TabNavGroup navigation works for all groups/pages | Manual + keyboard nav |
| Deep-link refresh works for all routes | Navigate directly to `/apps/listini-e-sconti/sconti-energia` |
| Customer exclusion codes match Appsmith | Compare query results: exclude 385 for IaaS Prezzi, 385+485 for IaaS Credito |
| IaaS pricing UPSERT creates new + updates existing | Test both paths for same customer |
| Timoo UPSERT creates new + updates existing | Test both paths; verify no duplicates (fixed Appsmith bug) |
| Group sync diff-based | Add group, remove group, verify transactional |
| Credit ledger immutable | Verify no UPDATE/DELETE paths exist; only INSERT |
| Batch operations transactional | Partially invalid batch → 422, no partial writes |

#### 6.2 HubSpot side-effects verified

| Page | Side-effect | Verification |
|------|-------------|-------------|
| IaaS Prezzi | Note on price change | Change a price → check HubSpot company timeline |
| IaaS Credito | Note per changed row | Change 2 credits → 2 notes created |
| Sconti Energia | Note + Task | Change discount → note with HTML table + task assigned to `eva.grimaldi@cdlan.it` |
| HubSpot down | Graceful degradation | Mock 500 from HubSpot → save still succeeds, error logged server-side |

#### 6.3 Coexistence validation

| Check | Method |
|-------|--------|
| Both apps can read same IaaS pricing rows | Read in Appsmith, read in new app, compare |
| Both apps can write same IaaS pricing rows | Write in new app, verify in Appsmith (and vice versa) |
| Exclusion codes consistent | Verify same customers excluded in both apps |
| No schema changes | `git diff` on any migration files → empty |
| Credit transactions from both apps | Add transaction in Appsmith, verify visible in new app |

#### 6.4 Accessibility review

| Item | Implementation |
|------|---------------|
| `prefers-reduced-motion` | All animations disabled via global CSS |
| Keyboard navigation | Tab through TabNavGroup, Enter/Space activate, Escape closes dropdowns/modals |
| ARIA attributes | `aria-expanded` on group tabs, `role="menu"` on dropdowns, `aria-label` on form inputs |
| Focus management | Modal traps focus (native `<dialog>`), dropdown returns focus on close |
| Color contrast | All text meets WCAG AA against clean theme backgrounds |
| Touch targets | All interactive elements ≥ 44px height |

#### 6.5 Loading and error states

| State | Implementation |
|-------|---------------|
| Kit list loading | Skeleton rows (existing `Skeleton` component) |
| Kit card loading | Skeleton metadata grid + product table |
| Table loading | Skeleton rows with staggered animation |
| Empty customer list | Empty state with icon + message |
| No racks for customer | "Nessun rack attivo per questo cliente" |
| No transactions | "Nessuna transazione registrata" |
| API error | Error toast + console error log |
| PDF generation error | Error toast: "Generazione PDF non disponibile" |

**Verification:**
- All pages handle loading, empty, and error states gracefully
- HubSpot integration works end-to-end (or degrades silently)
- Both Appsmith and new app can coexist on same databases
- Keyboard-only navigation possible for all flows
- Responsive layout tested at 640px, 900px, 1200px breakpoints

---

## TypeScript Types

Defined in `apps/listini-e-sconti/src/types/`:

```typescript
// ── Mistra entities ──

interface Customer {
  id: number;
  name: string;
}

interface Kit {
  id: number;
  internal_name: string;
  billing_period: string;
  initial_subscription_months: number;
  next_subscription_months: number;
  activation_time_days: number;
  category_id: number;
  category_name: string;
  category_color: string;
  is_main_prd_sellable: boolean;
  sconto_massimo: number;
  variable_billing: boolean;
  h24_assurance: boolean;
  sla_resolution_hours: number;
  notes: string | null;
}

interface KitProduct {
  group_name: string | null;
  internal_name: string;
  nrc: number;
  mrc: number;
  minimum: number;
  maximum: number;
  required: boolean;
  position: number;
  product_code: string;
}

interface CustomerGroup {
  id: number;
  name: string;
}

interface KitGroupDiscount {
  kit_id: number;
  kit_name: string;
  discount_mrc: number;
  discount_nrc: number;
}

interface CreditBalance {
  balance: number;
}

interface CreditTransaction {
  id: number;
  transaction_date: string;
  amount: number;
  operation_sign: '+' | '-';
  description: string;
  operated_by: string;
}

interface TransactionRequest {
  amount: number;
  operation_sign: '+' | '-';
  description: string;
}

interface TimooPricing {
  user_month: number;
  se_month: number;
  is_default: boolean;
}

// ── Grappa entities ──

interface GrappaCustomer {
  id: number;
  intestazione: string;
  codice_aggancio_gest: number;
}

interface IaaSPricing {
  charge_cpu: number;
  charge_ram_kvm: number;
  charge_ram_vmware: number;
  charge_pstor: number;
  charge_sstor: number;
  charge_ip: number;
  charge_prefix24: number | null;
  is_default: boolean;
}

interface IaaSPricingRequest {
  charge_cpu: number;
  charge_ram_kvm: number;
  charge_ram_vmware: number;
  charge_pstor: number;
  charge_sstor: number;
  charge_ip: number;
  charge_prefix24?: number;
}

interface IaaSAccount {
  domainuuid: string;
  id_cli_fatturazione: number;
  intestazione: string;
  abbreviazione: string;
  serialnumber: string;
  data_attivazione: string;
  credito: number;
  infrastructure_platform: string;
}

interface IaaSCreditUpdateItem {
  domainuuid: string;
  id_cli_fatturazione: number;
  credito: number;
}

interface Rack {
  id_rack: number;
  name: string;
  building: string;
  room: string;
  floor: number | null;
  island: number | null;
  type: string | null;
  sconto: number;
}

interface RackDiscountUpdateItem {
  id_rack: number;
  sconto: number;
}
```

---

## File Tree (new files)

```
apps/listini-e-sconti/
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
│   │   └── client.ts              # Local API client hook (useApiClient) with get/post/put/patch/delete/postBlob
│   ├── pages/
│   │   ├── KitPage.tsx
│   │   ├── IaaSPrezziPage.tsx
│   │   ├── TimooPrezziPage.tsx
│   │   ├── GruppiScontoPage.tsx
│   │   ├── ScontiEnergiaPage.tsx
│   │   ├── IaaSCreditiPage.tsx
│   │   └── GestioneCreditiPage.tsx
│   ├── components/
│   │   ├── Kit/
│   │   │   ├── KitList.tsx
│   │   │   ├── KitList.module.css
│   │   │   ├── KitCard.tsx
│   │   │   ├── KitCard.module.css
│   │   │   ├── KitMetadata.tsx
│   │   │   └── KitProductTable.tsx
│   │   ├── Pricing/
│   │   │   ├── IaaSPricingForm.tsx
│   │   │   └── TimooPricingForm.tsx
│   │   ├── Credits/
│   │   │   ├── AccountsTable.tsx
│   │   │   ├── TransactionList.tsx
│   │   │   └── NewTransactionModal.tsx
│   │   ├── Discounts/
│   │   │   ├── RackTable.tsx
│   │   │   ├── GroupAssociations.tsx
│   │   │   └── AssociateGroupsModal.tsx
│   │   └── shared/
│   │       └── CustomerDropdown.tsx  # Reusable customer selector (3 variants)
│   └── hooks/
│       ├── useOptionalAuth.ts      # Auth hook wrapping @mrsmith/auth-client (kit-products pattern)
│       └── useApi.ts               # TanStack Query hooks per entity

packages/ui/src/components/TabNavGroup/
├── TabNavGroup.tsx
└── TabNavGroup.module.css

backend/internal/listini/
├── handler.go                      # Handler struct, RegisterRoutes, shared helpers
├── handler_customer.go             # Customer list endpoints (Mistra + Grappa + rack-customers)
├── handler_kit.go                  # Kit list, products, help URL, PDF
├── handler_iaas_pricing.go         # IaaS pricing GET/UPSERT
├── handler_iaas_accounts.go        # IaaS accounts list, batch credit update
├── handler_racks.go                # Rack list, batch discount update
├── handler_groups.go               # Customer groups, sync, kit discounts
├── handler_credits.go              # Credit balance, transactions, new transaction
├── handler_timoo.go                # Timoo pricing GET/UPSERT
├── hubspot.go                      # HubSpotService: lookup, note, task, async wrappers
├── carbone.go                      # CarboneService: PDF generation
├── models.go                       # Request/response structs
├── handler_test.go                 # Unit tests
└── integration_test.go             # Integration tests (//go:build integration)
```

---

## Modified Files Summary

| File | Change |
|------|--------|
| `backend/go.mod` | Add `github.com/go-sql-driver/mysql` |
| `backend/cmd/server/main.go` | Import MySQL driver, open `grappaDB`, construct services, call `listini.RegisterRoutes`, add href override, update catalog filter |
| `backend/internal/platform/config/config.go` | Add `GrappaDSN`, `HubSpotAPIKey`, `CarboneAPIKey`, `ListiniAppURL`. Add 5177 to CORS. |
| `backend/internal/platform/applaunch/catalog.go` | Add `ListiniAppID`, `ListiniAppHref`, `listiniAccessRoles`, `ListiniAccessRoles()`. Update existing entry. |
| `packages/ui/src/index.ts` | Export `TabNavGroup` |
| Root `package.json` | Add listini to dev script, add `dev:listini` |
| `Makefile` | Add `dev-listini` target, update `.PHONY` |
| `deploy/Dockerfile` | Add COPY line for listini-e-sconti dist |
| `deploy/k8s/deployment.yaml` | Add `GRAPPA_DSN`, `HUBSPOT_API_KEY`, `CARBONE_API_KEY` secret refs |
| `.env.preprod.example` | Add `GRAPPA_DSN=`, `HUBSPOT_API_KEY=`, `CARBONE_API_KEY=` |
| `docker-compose.dev.yaml` | Add `listini-e-sconti` service (port 5177) + `listini_node_modules` volume |

---

## Open Decisions

| Item | Status |
|------|--------|
| Carbone template ID | Hardcoded constant. Tracked in `docs/TODO.md` for portal admin module. |
| HubSpot task assignee | Hardcoded `eva.grimaldi@cdlan.it`. Tracked in `docs/TODO.md`. |
| Async HubSpot queue | Fire-and-forget for now. Shared queue with retry tracked in `docs/TODO.md`. |
| Bulk Kit PDF export | Single kit only. Tracked in `docs/TODO.md`. |
| Kit price versioning | Not versioned. Tracked in `docs/TODO.md`. |
| Discount approval workflow | No approval. Tracked in `docs/TODO.md`. |
| Rack customer filter tightening | Current query matches audited Appsmith behavior (no `rack_sockets.status` or `cli_fatturazione.stato` filter). If tighter filtering is needed, verify against live data and update spec. |
| `database.New` MySQL support | Verify the existing `database.New` helper accepts `"mysql"` driver string. If not, use `sql.Open("mysql", dsn)` directly in `main.go`. |
| `group_association.active` cleanup | Column is deprecated. Currently preserved for Appsmith coexistence (writes `true`, reads ignore). Remove after Appsmith decommission. |
