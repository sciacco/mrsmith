# Panoramica Cliente — Implementation Plan

> **Spec source:** `apps/panoramica-cliente/SPEC.md`
> **Date:** 2026-04-08
> **Status:** Draft — revised per PANORAMICA-FB.md findings (2026-04-09, 2nd pass)

---

## Repo-Fit Checklist

### 1. Runtime Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| **Route/base path** | `/apps/panoramica-cliente/` (build), `/` (dev) | Same pattern as listini: `vite.config.ts` base conditional on `command` |
| **Deep links** | SPA fallback handled by `staticspa` handler — auto-discovers `/apps/panoramica-cliente/index.html` | `backend/internal/platform/staticspa/handler.go` — generic, no changes needed |
| **Dev split-server** | `PANORAMICA_APP_URL` env var for href override | Budget/listini pattern in `main.go` lines 119-134, `catalog.go` href overrides |
| **Catalog entry** | Update existing `panoramica-cliente` entry: set `Href` → `/apps/panoramica-cliente/`, add `app_panoramica_access` role, set `Status: "ready"` | `catalog.go:160-167` — already exists with placeholder href `/apps/smart-apps/panoramica-cliente` and `defaultAccessRoles` |

### 2. Dev Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| **Vite port** | `5178` (next after listini 5177) | Existing ports: portal=5173, budget=5174, compliance=5175, kit-products=5176, listini=5177 |
| **API proxy** | `/api` and `/config` → `http://localhost:8080` | All apps use same pattern |
| **Root scripts** | Add `"dev:panoramica": "pnpm --filter mrsmith-panoramica-cliente dev"` to root `package.json` | Existing: `dev:budget`, `dev:compliance`, `dev:kit-products`, `dev:listini` |
| **CORS** | Add port `5178` to `config.go` default CORS origins | `config.go:53` — currently has 5173-5177 |
| **Docker compose** | Add `panoramica-cliente` service to `docker-compose.dev.yaml` | Follow listini service pattern |

### 3. Auth Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| **Keycloak role** | `app_panoramica_access` | Convention: `app_{appname}_access` per CLAUDE.md |
| **Bearer auth** | All `/panoramica/v1/*` endpoints wrapped in `acl.RequireRole()` | Same `protect` closure pattern as listini/compliance |
| **401/403** | Handled by existing `authMiddleware.Handler` on `/api/` mount | `main.go` middleware chain |
| **Frontend auth** | Same pattern as listini: fetch `/config` → init AuthProvider → Bearer on all API calls | `apps/listini-e-sconti/src/main.tsx` |
| **User identity** | Not needed — this app is entirely read-only, no `operated_by` fields | — |

### 4. Data-Contract Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| **Customer ID (Mistra)** | `numero_azienda` = ERP ID (integer) — used by Fatture, Ordini pages | `docs/IMPLEMENTATION-KNOWLEDGE.md` |
| **Customer ID (Grappa)** | `cli_fatturazione.id` = internal Grappa ID. Bridge: `codice_aggancio_gest` = ERP ID | `docs/IMPLEMENTATION-KNOWLEDGE.md` |
| **Customer ID (Accessi)** | Uses Grappa internal IDs from `loader.grappa_cli_fatturazione.id` — NOT ERP IDs | Spec: as-is, each endpoint uses its original ID type |
| **IaaS domain key** | `cloudstack_domain` (UUID string) — passed as query parameter | Spec: IaaS charge queries keyed by domain UUID |
| **Read-only** | **Zero writes.** No INSERT/UPDATE/DELETE. No transactions needed. | Spec: entirely read-only |
| **Active-only defaults** | Ordini: active customers (dismissed filter). Accessi: active Grappa clients. IaaS: active accounts (attivo=1, fatturazione=1). | Spec per-endpoint filters |

### 5. Deployment Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| **Dockerfile COPY** | `COPY --from=frontend /app/apps/panoramica-cliente/dist /static/apps/panoramica-cliente` | Existing pattern: `deploy/Dockerfile` lines 20-24 |
| **Env vars — existing** | `MISTRA_DSN` (**required**), `GRAPPA_DSN` (**required**), `ANISETTA_DSN` (**required**) | All three already exist in `config.go` and are wired in `main.go` |
| **Env vars — new** | `PANORAMICA_APP_URL` (dev override, optional) | Follow `LISTINI_APP_URL` pattern |
| **DB drivers** | PostgreSQL (pgx) already imported. MySQL already imported (added for listini). No new drivers needed. | `go.mod`, `main.go` imports |
| **Migration story** | **No migrations** — coexistence means zero schema changes. This app only reads. | Spec: read-only, coexistence |
| **K8s deployment** | No new secrets needed — `MISTRA_DSN`, `GRAPPA_DSN`, `ANISETTA_DSN` already in deployment.yaml | `deploy/k8s/deployment.yaml` |

### 6. Verification Fit

| Item | Decision |
|------|----------|
| **Transaction rollback** | N/A — no writes, no transactions |
| **Deep-link refresh** | `staticspa` handler covers this automatically |
| **Structured logging** | `logging.FromContext(r.Context())` with `component=panoramica`, operation name per handler |
| **Panic recovery** | Existing `middleware.Recover(logger)` on `/api/` mount |
| **Error sanitization** | `httputil.InternalError` pattern — log real error, return generic 500 to client |
| **Coexistence** | Read-only — no conflict possible. No schema changes. |
| **CSV export** | Client-side only (frontend table feature) — no backend export endpoint |

---

## Implementation Sequence

The implementation is organized in **6 phases**. Each phase is self-contained and can be verified independently before moving to the next.

### Handoff Protocol

At the end of each phase, the implementer MUST:
1. Run `pnpm --filter mrsmith-panoramica-cliente exec tsc --noEmit` to verify type-checking
2. Run `cd backend && go build ./...` to verify Go compilation
3. Run `cd backend && go test ./...` to verify existing tests still pass (especially for phases that touch launcher/catalog wiring)
4. Verify the specific acceptance criteria listed in each phase
5. State what was completed and what the next phase should do

---

## Phase 1: Scaffolding — Backend Module + Frontend App Shell

**Goal:** Wire up the new app so it loads in the browser with navigation and empty pages. No data yet.

### Phase 1A: Backend module

**Create file `backend/internal/panoramica/handler.go`:**

```go
package panoramica

import (
	"database/sql"
	"net/http"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// Handler holds references to all three databases.
type Handler struct {
	mistraDB   *sql.DB // Mistra PostgreSQL (loader schema)
	grappaDB   *sql.DB // Grappa MySQL
	anisettaDB *sql.DB // Anisetta PostgreSQL
}

// RegisterRoutes mounts all panoramica endpoints on the given mux.
func RegisterRoutes(mux *http.ServeMux, mistraDB, grappaDB, anisettaDB *sql.DB) {
	h := &Handler{mistraDB: mistraDB, grappaDB: grappaDB, anisettaDB: anisettaDB}
	protect := acl.RequireRole(applaunch.PanoramicaAccessRoles()...)
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, protect(http.HandlerFunc(handler)))
	}

	// Routes will be added in subsequent phases.
	_ = handle // Avoid unused variable warning until routes are added.
}

// ── Shared helpers ──

func (h *Handler) requireMistra(w http.ResponseWriter) bool {
	if h.mistraDB == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "mistra_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) requireGrappa(w http.ResponseWriter) bool {
	if h.grappaDB == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "grappa_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) requireAnisetta(w http.ResponseWriter) bool {
	if h.anisettaDB == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "anisetta_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) dbFailure(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) {
	args := []any{"component", "panoramica", "operation", operation}
	args = append(args, attrs...)
	httputil.InternalError(w, r, err, "database operation failed", args...)
}

func (h *Handler) rowsDone(w http.ResponseWriter, r *http.Request, rows *sql.Rows, operation string) bool {
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, operation+"_rows", err)
		return false
	}
	return true
}
```

**Modify `backend/internal/platform/applaunch/catalog.go`:**

1. Add constant and role variable after the existing ones (after line 20):
```go
PanoramicaAppID   = "panoramica-cliente"
PanoramicaAppHref = "/apps/panoramica-cliente/"
```

2. Add role variable (after line 28):
```go
panoramicaAccessRoles = []string{"app_panoramica_access"}
```

3. Update the existing `panoramica-cliente` entry in `Catalog()` (lines 160-167). Change:
   - `Href` from `"/apps/smart-apps/panoramica-cliente"` to `PanoramicaAppHref`
   - `Status` from empty to `"ready"`
   - `AccessRoles` from `defaultRoles` to `PanoramicaAccessRoles()`

4. Add export function (after `ListiniAccessRoles()`, line 316):
```go
func PanoramicaAccessRoles() []string {
	return slices.Clone(panoramicaAccessRoles)
}
```

**Update `backend/internal/platform/applaunch/catalog_test.go`:**

Panoramica moves from `defaultRoles` (placeholder) to a dedicated `app_panoramica_access` role. This changes the hard-coded counts:

1. `TestCatalogReturnsAllApps` (line 40): placeholder count drops from 16 to 15. Change:
   - `expected 16 placeholder apps` → `expected 15 placeholder apps`

2. `TestVisibleCategoriesBothRolesSeesEverything` (line 47): add `"app_panoramica_access"` to the roles list. Total stays 20 (15 placeholders + 5 role-gated). Update assertion message:
   - `// All 20 apps (16 placeholders + 1 budget + 1 compliance + 1 kit-products + 1 listini)` → `// All 20 apps (15 placeholders + 1 budget + 1 compliance + 1 kit-products + 1 listini + 1 panoramica)`

3. Add a new test `TestVisibleCategoriesFiltersByPanoramicaRole`:
```go
func TestVisibleCategoriesFiltersByPanoramicaRole(t *testing.T) {
	categories := VisibleCategories(Catalog(nil), []string{"app_panoramica_access"})
	if len(categories) != 1 {
		t.Fatalf("expected 1 category, got %d", len(categories))
	}
	if categories[0].ID != "smart-apps" {
		t.Fatalf("expected smart-apps category, got %q", categories[0].ID)
	}
	if len(categories[0].Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(categories[0].Apps))
	}
	if categories[0].Apps[0].ID != PanoramicaAppID {
		t.Fatalf("expected panoramica app, got %q", categories[0].Apps[0].ID)
	}
}
```

**Modify `backend/cmd/server/main.go`:**

1. Add import: `"github.com/sciacco/mrsmith/internal/panoramica"`

2. Add `hrefOverrides` block for Panoramica after the Listini block (after line 169):
```go
if cfg.PanoramicaAppURL != "" {
	hrefOverrides[applaunch.PanoramicaAppID] = cfg.PanoramicaAppURL
} else if cfg.StaticDir == "" {
	hrefOverrides[applaunch.PanoramicaAppID] = "http://localhost:5178"
}
```

3. Add catalog visibility filter for Panoramica in the existing filtering block (after the Listini filter, around line 179):
```go
if definition.ID == applaunch.PanoramicaAppID && cfg.MistraDSN == "" && cfg.GrappaDSN == "" && cfg.AnisettaDSN == "" {
	continue
}
```

**Rationale:** Panoramica spans three databases, but each page only touches one. Hide the app only when *all three* DSNs are missing (no useful data at all). When at least one DSN is configured, individual page handlers return 503 via `requireMistra`/`requireGrappa`/`requireAnisetta` for their unavailable sections — this is graceful degradation, consistent with how the per-handler nil guards are already designed.

4. Add route registration after the `listini.RegisterRoutes` line (after line 192):
```go
panoramica.RegisterRoutes(api, mistraDB, grappaDB, anisettaDB)
```

**Modify `backend/internal/platform/config/config.go`:**

1. Add config field after `ListiniAppURL` (line 16):
```go
PanoramicaAppURL  string
```

2. Add env load after the `ListiniAppURL` line (line 58):
```go
PanoramicaAppURL:  envOr("PANORAMICA_APP_URL", ""),
```

3. Add `5178` to the CORS origins default (line 53):
```go
CORSOrigins: envOr("CORS_ORIGINS", "http://localhost:5173,http://localhost:5174,http://localhost:5175,http://localhost:5176,http://localhost:5177,http://localhost:5178"),
```

### Phase 1B: Frontend app scaffold

**Create `apps/panoramica-cliente/package.json`:**
```json
{
  "name": "mrsmith-panoramica-cliente",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "lint": "tsc --noEmit",
    "preview": "vite preview"
  },
  "dependencies": {
    "@mrsmith/api-client": "workspace:*",
    "@mrsmith/auth-client": "workspace:*",
    "@mrsmith/ui": "workspace:*",
    "@tanstack/react-query": "^5.62.0",
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "react-router-dom": "^7.1.0"
  },
  "devDependencies": {
    "@types/react": "^18.3.12",
    "@types/react-dom": "^18.3.1",
    "@vitejs/plugin-react": "^4.3.4",
    "typescript": "~5.6.3",
    "vite": "^6.0.5"
  }
}
```

**Create `apps/panoramica-cliente/tsconfig.json`:**
Copy from `apps/listini-e-sconti/tsconfig.json` exactly.

**Create `apps/panoramica-cliente/index.html`:**
Copy from `apps/listini-e-sconti/index.html`, change `<title>` to `Panoramica Cliente`.

**Create `apps/panoramica-cliente/vite.config.ts`:**
```typescript
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

const backendTarget = process.env.VITE_DEV_BACKEND_URL || 'http://localhost:8080';

export default defineConfig(({ command }) => ({
  base: command === 'build' ? '/apps/panoramica-cliente/' : '/',
  plugins: [react()],
  server: {
    port: 5178,
    proxy: {
      '/api': backendTarget,
      '/config': backendTarget,
    },
  },
}));
```

**Create `apps/panoramica-cliente/src/vite-env.d.ts`:**
Copy from listini-e-sconti exactly.

**Create `apps/panoramica-cliente/src/styles/global.css`:**
Copy from `apps/listini-e-sconti/src/styles/global.css` exactly.

**Create `apps/panoramica-cliente/src/hooks/useOptionalAuth.ts`:**
Copy from `apps/listini-e-sconti/src/hooks/useOptionalAuth.ts` exactly.

**Create `apps/panoramica-cliente/src/api/client.ts`:**
Copy from `apps/listini-e-sconti/src/api/client.ts` exactly.

**Create `apps/panoramica-cliente/src/main.tsx`:**
Copy from `apps/listini-e-sconti/src/main.tsx`. Change error messages from "Listini e Sconti" to "Panoramica Cliente".

**Create `apps/panoramica-cliente/src/routes.tsx`:**
```typescript
import { Navigate, type RouteObject } from 'react-router-dom';
import { OrdiniRicorrentiPage } from './pages/OrdiniRicorrentiPage';
import { OrdiniDettaglioPage } from './pages/OrdiniDettaglioPage';
import { FatturePage } from './pages/FatturePage';
import { AccessiPage } from './pages/AccessiPage';
import { IaaSPayPerUsePage } from './pages/IaaSPayPerUsePage';
import { TimooTenantsPage } from './pages/TimooTenantsPage';
import { LicenzeWindowsPage } from './pages/LicenzeWindowsPage';

export const routes: RouteObject[] = [
  { index: true, element: <Navigate to="/ordini-ricorrenti" replace /> },
  { path: 'ordini-ricorrenti', element: <OrdiniRicorrentiPage /> },
  { path: 'ordini-dettaglio', element: <OrdiniDettaglioPage /> },
  { path: 'fatture', element: <FatturePage /> },
  { path: 'accessi', element: <AccessiPage /> },
  { path: 'iaas-ppu', element: <IaaSPayPerUsePage /> },
  { path: 'timoo', element: <TimooTenantsPage /> },
  { path: 'licenze-windows', element: <LicenzeWindowsPage /> },
  { path: '*', element: <Navigate to="/ordini-ricorrenti" replace /> },
];
```

**Create `apps/panoramica-cliente/src/App.tsx`:**
```typescript
import { useRoutes } from 'react-router-dom';
import { AppShell } from '@mrsmith/ui';
import { TabNavGroup, type TabGroup } from '@mrsmith/ui';
import { routes } from './routes';
import { useOptionalAuth } from './hooks/useOptionalAuth';
import styles from './App.module.css';

const navGroups: TabGroup[] = [
  {
    label: 'Ordini',
    items: [
      { label: 'Ordini ricorrenti', path: '/ordini-ricorrenti' },
      { label: 'Ordini Ricorrenti e Spot', path: '/ordini-dettaglio' },
    ],
  },
  {
    label: 'Fatture',
    items: [{ label: 'Fatture', path: '/fatture' }],
  },
  {
    label: 'Servizi',
    items: [
      { label: 'Accessi', path: '/accessi' },
      { label: 'IaaS Pay Per Use', path: '/iaas-ppu' },
      { label: 'Timoo tenants', path: '/timoo' },
      { label: 'Licenze Windows', path: '/licenze-windows' },
    ],
  },
];

export function App() {
  const { user, loading, logout, status } = useOptionalAuth();
  const element = useRoutes(routes);

  if (loading) return null;

  if (status === 'reauthenticating') {
    return (
      <AppShell userName={user?.name ?? 'John Doe'} onLogout={logout}>
        <AppShell.Nav>
          <div className={styles.navRow}>
            <TabNavGroup groups={navGroups} />
          </div>
        </AppShell.Nav>
        <AppShell.Content>
          <section className={styles.reauthCard}>
            <p className={styles.eyebrow}>Autenticazione</p>
            <h1>Sessione in ripristino</h1>
            <p>La sessione e scaduta durante l&apos;inattivita. Reindirizzamento a Keycloak in corso.</p>
          </section>
        </AppShell.Content>
      </AppShell>
    );
  }

  return (
    <AppShell userName={user?.name ?? 'John Doe'} onLogout={logout}>
      <AppShell.Nav>
        <div className={styles.navRow}>
          <TabNavGroup groups={navGroups} />
        </div>
      </AppShell.Nav>
      <AppShell.Content>
        {element}
      </AppShell.Content>
    </AppShell>
  );
}
```

**Create `apps/panoramica-cliente/src/App.module.css`:**
Copy from `apps/listini-e-sconti/src/App.module.css` exactly.

**Create 7 placeholder page files** in `apps/panoramica-cliente/src/pages/`:

Each file follows this template (example for `OrdiniRicorrentiPage.tsx`):
```typescript
export function OrdiniRicorrentiPage() {
  return <div style={{ padding: '2rem' }}><h2>Ordini ricorrenti</h2><p>Coming soon.</p></div>;
}
```

Create: `OrdiniRicorrentiPage.tsx`, `OrdiniDettaglioPage.tsx`, `FatturePage.tsx`, `AccessiPage.tsx`, `IaaSPayPerUsePage.tsx`, `TimooTenantsPage.tsx`, `LicenzeWindowsPage.tsx`.

### Phase 1C: Dev wiring

**Modify root `package.json`:**

1. Add Panoramica to the root `dev` script so `make dev` launches it alongside all other apps. Add `,panoramica` to the `--names` list and append `\"pnpm --filter mrsmith-panoramica-cliente dev\"` to the concurrently command.

2. Add standalone script: `"dev:panoramica": "pnpm --filter mrsmith-panoramica-cliente dev"`

**Modify `Makefile`:**
Add target after `dev-listini` (line 37):
```makefile
dev-panoramica:       ## Solo panoramica-cliente app
	pnpm --filter mrsmith-panoramica-cliente dev
```
Also add `dev-panoramica` to the `.PHONY` list (line 99).

**Modify `docker-compose.dev.yaml`:**
Add service after `listini-e-sconti` (before the `volumes:` block, line 75):
```yaml
  panoramica-cliente:
    image: node:20-slim
    working_dir: /app
    command: sh -c "corepack enable && pnpm install && pnpm --filter mrsmith-panoramica-cliente dev --host 0.0.0.0"
    volumes:
      - .:/app
      - panoramica_node_modules:/app/node_modules
    environment:
      - VITE_DEV_BACKEND_URL=http://backend:8080
    ports:
      - "5178:5178"
    depends_on:
      - backend
```

Add named volume at the end of the `volumes:` block:
```yaml
  panoramica_node_modules:
```

**Modify `deploy/Dockerfile`:**
Add after the listini COPY line (line 24):
```dockerfile
COPY --from=frontend /app/apps/panoramica-cliente/dist /static/apps/panoramica-cliente
```

**Run `pnpm install`** from the repo root to wire up the workspace.

### Phase 1 — Acceptance Criteria

- [ ] `cd backend && go build ./...` compiles without errors
- [ ] `pnpm --filter mrsmith-panoramica-cliente exec tsc --noEmit` passes
- [ ] `pnpm dev:panoramica` starts Vite on port 5178
- [ ] Browser at `http://localhost:5178/` shows the AppShell with TabNavGroup navigation (Ordini / Fatture / Servizi)
- [ ] Clicking each nav item renders the placeholder page
- [ ] Deep-link refresh (e.g., `http://localhost:5178/fatture`) works
- [ ] Launching Panoramica from the portal works in local split-server development (card href → `http://localhost:5178`)
- [ ] With all three DSNs unset, Panoramica does not appear in the portal catalog

---

## Phase 2: Backend — Mistra Endpoints (Ordini + Fatture + Accessi)

**Goal:** Implement all 9 Mistra/loader endpoints. These serve the Ordini, Fatture, and Accessi pages.

### Phase 2A: Customer list endpoints

**Create `backend/internal/panoramica/handler_customer.go`:**

Implement 3 handlers. Each follows the listini pattern (`requireMistra` → `QueryContext` → scan loop → `httputil.JSON`).

**Handler: `handleListCustomersWithInvoices`**

```go
// GET /panoramica/v1/customers/with-invoices
// Returns customers from loader.erp_clienti_con_fatture ordered by ragione_sociale.
func (h *Handler) handleListCustomersWithInvoices(w http.ResponseWriter, r *http.Request) {
	if !h.requireMistra(w) { return }

	rows, err := h.mistraDB.QueryContext(r.Context(),
		`SELECT numero_azienda, ragione_sociale FROM loader.erp_clienti_con_fatture ORDER BY ragione_sociale`)
	// ... scan into []struct{ NumeroAzienda int `json:"numero_azienda"`; RagioneSociale string `json:"ragione_sociale"` }
}
```

**Handler: `handleListCustomersWithOrders`**

This handler serves BOTH Ordini pages. Accept query parameter `variant=a` or `variant=b` to select the dismissal filter.

```go
// GET /panoramica/v1/customers/with-orders?variant=a|b
// variant=a: includes IS NULL check (Ordini ricorrenti page)
// variant=b: excludes IS NULL check (Ordini R&S page)
```

Original query (variant A — "Ordini ricorrenti"):
```sql
SELECT DISTINCT odv.numero_azienda, odv.ragione_sociale
FROM loader.v_ordini_ricorrenti AS odv
JOIN loader.erp_anagrafiche_clienti AS cli
  ON cli.numero_azienda = odv.numero_azienda
  AND (cli.data_dismissione >= NOW() OR cli.data_dismissione = '0001-01-01 00:00:00' OR cli.data_dismissione IS NULL)
ORDER BY ragione_sociale
```

Original query (variant B — "Ordini R&S"):
```sql
SELECT DISTINCT odv.numero_azienda, odv.ragione_sociale
FROM loader.v_ordini_ricorrenti AS odv
JOIN loader.erp_anagrafiche_clienti AS cli
  ON cli.numero_azienda = odv.numero_azienda
  AND (cli.data_dismissione >= NOW() OR cli.data_dismissione = '0001-01-01 00:00:00')
ORDER BY ragione_sociale
```

**IMPORTANT:** Do NOT include the `UNION ALL SELECT -1, 'TUTTI I CLIENTI'` row. This was removed in the spec — customer selection is always required.

**Handler: `handleListCustomersWithAccessLines`**

```go
// GET /panoramica/v1/customers/with-access-lines
```

Original query:
```sql
SELECT DISTINCT cf.id, cf.intestazione
FROM loader.grappa_foglio_linee fl
JOIN loader.grappa_cli_fatturazione cf ON fl.id_anagrafica = cf.id
WHERE cf.codice_aggancio_gest IS NOT NULL AND cf.stato = 'attivo'
ORDER BY cf.intestazione
```

**Note:** This returns Grappa internal IDs (`cf.id`), not ERP IDs. The frontend sends these IDs back for the access lines query.

### Phase 2B: Order status endpoint

**Handler: `handleListOrderStatuses`**

```go
// GET /panoramica/v1/order-statuses
// Shared by both Ordini pages.
```

Original query:
```sql
SELECT DISTINCT stato_ordine FROM loader.v_ordini_ricorrenti ORDER BY stato_ordine
```

### Phase 2C: Orders summary endpoint

**Create `backend/internal/panoramica/handler_orders.go`:**

**Handler: `handleListOrdersSummary`**

```go
// GET /panoramica/v1/orders/summary?cliente=123&stati=Evaso,Confermato
```

Parse parameters:
- `cliente` (required, int) — from query string
- `stati` (required, comma-separated strings) — from query string

Build parameterized query. Original SQL (from `get_ordini_ricorrenti`):
```sql
SELECT stato, numero_ordine, descrizione_long, quantita, nrc, mrc, totale_mrc,
       stato_ordine, nome_testata_ordine, rn, numero_azienda, data_documento,
       stato_riga, data_ultima_fatt, serialnumber,
       metodo_pagamento, durata_servizio, durata_rinnovo, data_cessazione,
       data_attivazione, note_legali, sost_ord, sostituito_da,
       loader.get_reverse_order_history_path(nome_testata_ordine) AS storico
FROM loader.v_ordini_sintesi
WHERE numero_azienda = $1
  AND stato_ordine = ANY($2)
ORDER BY data_documento, nome_testata_ordine, rn
```

**IMPORTANT:** Use PostgreSQL parameterized query (`$1`, `$2`). For the `IN` clause with an array, build the placeholder list dynamically (see listini `handleListGrappaCustomers` pattern at `handler_customer.go:108-119`). Do **not** use `pq.Array` — the repo uses `pgx`, not `lib/pq`.

**Response type:**
```go
type OrderSummaryRow struct {
	Stato             string   `json:"stato"`
	NumeroOrdine      string   `json:"numero_ordine"`
	DescrizioneLong   string   `json:"descrizione_long"`
	Quantita          int      `json:"quantita"`
	NRC               float64  `json:"nrc"`
	MRC               float64  `json:"mrc"`
	TotaleMRC         float64  `json:"totale_mrc"`
	StatoOrdine       string   `json:"stato_ordine"`
	NomeTestataOrdine string   `json:"nome_testata_ordine"`
	RN                int      `json:"rn"`
	NumeroAzienda     int      `json:"numero_azienda"`
	DataDocumento     *string  `json:"data_documento"`
	StatoRiga         string   `json:"stato_riga"`
	DataUltimaFatt    *string  `json:"data_ultima_fatt"`
	Serialnumber      *string  `json:"serialnumber"`
	MetodoPagamento   *string  `json:"metodo_pagamento"`
	DurataServizio    *string  `json:"durata_servizio"`
	DurataRinnovo     *string  `json:"durata_rinnovo"`
	DataCessazione    *string  `json:"data_cessazione"`
	DataAttivazione   *string  `json:"data_attivazione"`
	NoteLegali        *string  `json:"note_legali"`
	SostOrd           *string  `json:"sost_ord"`
	SostituitoDa      *string  `json:"sostituito_da"`
	Storico           *string  `json:"storico"`
}
```

Use `*string` for nullable fields (dates and optional strings). Use `sql.NullString` / `sql.NullFloat64` during scan, then convert to pointer in the response struct.

### Phase 2D: Orders detail endpoint

**Handler: `handleListOrdersDetail`**

```go
// GET /panoramica/v1/orders/detail?cliente=123&stati=Evaso,Confermato
```

The full SQL is in `SPEC.md` under "Entity: Order — Detail". Copy it verbatim but replace Appsmith bindings with PostgreSQL parameters:
- `{{s_clienti.selectedOptionValue}}` → `$1`
- `{{ms_stati.selectedOptionValues.map(...)}}` → `= ANY($2::text[])`

This is the query with the critical `stato_riga` CASE, `NULLIF` sentinel dates, `data_ordine` CASE, `intestazione_ordine` concat, `descrizione_long` CASE, and `CDL-AUTO` exclusion. Copy the SQL exactly from the spec — do not simplify or rewrite it.

**Response type:** Define a struct with all 60+ fields. Use `*string` / `*float64` / `*int` for nullable columns. Group fields logically with comments.

### Phase 2E: Invoice lines endpoint

**Create `backend/internal/panoramica/handler_fatture.go`:**

**Handler: `handleListInvoices`**

```go
// GET /panoramica/v1/invoices?cliente=123&mesi=6
// mesi is optional: null/0 = no date filter
```

Original SQL (from `get_fatture`):
```sql
SELECT CASE WHEN rn = 1 THEN doc || ' ' || num_documento || CHR(13) || CHR(10) || to_char(data_documento, '(YYYY-MM-DD)') ELSE NULL END AS documento,
       descrizione_riga, qta, prezzo_unitario, prezzo_totale_netto, codice_articolo,
       data_documento, num_documento, id_cliente, progressivo_riga, serialnumber,
       riferimento_ordine_cliente, condizione_pagamento, scadenza, desc_conto_ricavo,
       gruppo, sottogruppo, rn
FROM loader.v_erp_fatture_nc
WHERE id_cliente = $1
```

For the period filter: if `mesi` parameter is present and > 0, append:
```sql
AND data_documento >= current_date - interval '$2 months'
```

**IMPORTANT:** PostgreSQL does not support parameterized intervals directly. Build the interval string safely using `fmt.Sprintf` with the validated integer (after parsing and validating `mesi` as a positive integer). This is safe because it's an integer, not user text.

```go
mesi := r.URL.Query().Get("mesi")
if mesi != "" {
    mesiInt, err := strconv.Atoi(mesi)
    if err != nil || mesiInt <= 0 {
        httputil.Error(w, http.StatusBadRequest, "invalid_mesi_parameter")
        return
    }
    query += fmt.Sprintf(" AND data_documento >= current_date - interval '%d months'", mesiInt)
}
```

Append sort:
```sql
ORDER BY anno_documento DESC, mese_documento DESC, tipo_documento, num_documento, rn
```

### Phase 2F: Access lines endpoints

**Create `backend/internal/panoramica/handler_accessi.go`:**

**Handler: `handleListConnectionTypes`**

```go
// GET /panoramica/v1/connection-types
```

Original query:
```sql
SELECT DISTINCT tipo_conn FROM loader.grappa_foglio_linee ORDER BY tipo_conn
```

**Handler: `handleListAccessLines`**

```go
// GET /panoramica/v1/access-lines?clienti=1,2,3&stati=Attiva,Cessata&tipi=FTTH,FTTC
```

Parse three array parameters from query string (comma-separated). Build parameterized `IN` clauses using the same dynamic placeholder pattern as listini.

Original SQL is the complex multi-table join from the spec. Copy verbatim, replacing Appsmith bindings with parameterized placeholders. For PostgreSQL arrays, build `$1, $2, $3` style placeholders dynamically.

### Phase 2G: Register all routes

**Update `backend/internal/panoramica/handler.go` `RegisterRoutes`:**

```go
// ── Mistra: Customers ──
handle("GET /panoramica/v1/customers/with-invoices", h.handleListCustomersWithInvoices)
handle("GET /panoramica/v1/customers/with-orders", h.handleListCustomersWithOrders)
handle("GET /panoramica/v1/customers/with-access-lines", h.handleListCustomersWithAccessLines)

// ── Mistra: Orders ──
handle("GET /panoramica/v1/order-statuses", h.handleListOrderStatuses)
handle("GET /panoramica/v1/orders/summary", h.handleListOrdersSummary)
handle("GET /panoramica/v1/orders/detail", h.handleListOrdersDetail)

// ── Mistra: Invoices ──
handle("GET /panoramica/v1/invoices", h.handleListInvoices)

// ── Mistra: Access Lines ──
handle("GET /panoramica/v1/connection-types", h.handleListConnectionTypes)
handle("GET /panoramica/v1/access-lines", h.handleListAccessLines)
```

### Phase 2 — Acceptance Criteria

- [ ] `cd backend && go build ./...` compiles
- [ ] With backend running, `curl http://localhost:8080/api/panoramica/v1/customers/with-invoices` returns JSON array (with valid auth token)
- [ ] `curl .../panoramica/v1/order-statuses` returns distinct status values
- [ ] `curl .../panoramica/v1/orders/summary?cliente=123&stati=Evaso` returns order rows
- [ ] `curl .../panoramica/v1/invoices?cliente=123&mesi=6` returns invoice lines
- [ ] `curl .../panoramica/v1/invoices?cliente=123` (no mesi) returns all invoices without date filter
- [ ] `curl .../panoramica/v1/connection-types` returns connection type list
- [ ] All endpoints return `[]` (empty array) when no data matches — never `null`

---

## Phase 3: Backend — Grappa Endpoints (IaaS + Licenze Windows)

**Goal:** Implement all 5 Grappa endpoints for the IaaS PPU and Licenze Windows pages.

**Create `backend/internal/panoramica/handler_iaas.go`:**

### Handler: `handleListIaaSAccounts`

```go
// GET /panoramica/v1/iaas/accounts
```

Original query (MySQL):
```sql
SELECT c.intestazione, a.credito, domainuuid AS cloudstack_domain, id_cli_fatturazione,
       abbreviazione, codice_ordine, serialnumber, data_attivazione
FROM cdl_accounts a
JOIN cli_fatturazione c ON a.id_cli_fatturazione = c.id
WHERE id_cli_fatturazione > 0 AND attivo = 1 AND fatturazione = 1
  AND c.codice_aggancio_gest NOT IN (385, 485)
ORDER BY intestazione
```

**Note:** MySQL uses `?` placeholders, not `$1`. But this query has no parameters — the exclusion codes (385, 485) are hardcoded in the SQL (as per spec decision: as-is, not centralized).

### Handler: `handleListDailyCharges`

```go
// GET /panoramica/v1/iaas/daily-charges?domain=uuid-string
```

Original query (MySQL):
```sql
SELECT c.charge_day AS giorno, c.domainid,
    CAST(SUM(CASE WHEN c.usage_type = 9999 THEN c.usage_charge ELSE 0 END) AS DECIMAL(10,2)) AS utCredit,
    CAST(SUM(c.usage_charge) AS DECIMAL(10,2)) AS total_importo
FROM cdl_charges c
WHERE c.domainid = ? AND charge_day >= DATE_SUB(NOW(), INTERVAL 120 DAY)
GROUP BY c.charge_day, c.domainid
ORDER BY c.charge_day DESC
```

### Handler: `handleListMonthlyCharges`

```go
// GET /panoramica/v1/iaas/monthly-charges?domain=uuid-string
```

Original query (MySQL):
```sql
SELECT DATE_FORMAT(charge_day, '%Y-%m') AS mese, CAST(SUM(usage_charge) AS DECIMAL(7,2)) AS importo
FROM cdl_charges
WHERE domainid = ? AND charge_day >= DATE_SUB(NOW(), INTERVAL 365 DAY)
GROUP BY 1 ORDER BY 1 DESC LIMIT 12
```

### Handler: `handleChargeBreakdown`

```go
// GET /panoramica/v1/iaas/charge-breakdown?domain=uuid-string&day=2026-04-01
```

Original query (MySQL) returns flat columns. **The backend must transform the flat SQL result into a typed array.**

Run the original query, then in Go, build the response:

```go
type ChargeItem struct {
	Type   string  `json:"type"`
	Label  string  `json:"label"`
	Amount float64 `json:"amount"`
}

type ChargeBreakdownResponse struct {
	Charges []ChargeItem `json:"charges"`
	Total   float64      `json:"total"`
}
```

Map the SQL columns to ChargeItem entries, filtering out zero-value entries:

```go
var typeMap = []struct {
	column string
	typ    string
	label  string
}{
	{"utRunningVM", "RunningVM", "utRunningVM"},
	{"utAllocatedVM", "AllocatedVM", "utAllocatedVM"},
	{"utIpCharge", "IpCharge", "utIpCharge"},
	{"utVolume", "Volume", "utVolume"},
	{"utTemplate", "Template", "utTemplate"},
	{"utISO", "ISO", "utISO"},
	{"utSnapshot", "Snapshot", "utSnapshot"},
	{"utVolumeSecondary", "VolumeSecondary", "utVolumeSecondary"},
	{"utVmSnapshotOnPrimary", "VmSnapshotOnPrimary", "utVmSnapshotOnPrimary"},
	{"utCredit", "Credit", "utCredit"},
}
```

**Note:** Labels are kept as-is English technical names (spec decision Q15).

### Handler: `handleListWindowsLicenses`

```go
// GET /panoramica/v1/iaas/windows-licenses
```

Original query (MySQL):
```sql
SELECT charge_day AS x, COUNT(0) AS y
FROM cdl_charges
WHERE charge_day >= CURDATE() - INTERVAL 14 DAY AND usage_type = 9998
GROUP BY charge_day ORDER BY charge_day DESC
```

### Register Grappa routes

Add to `RegisterRoutes` in `handler.go`:

```go
// ── Grappa: IaaS ──
handle("GET /panoramica/v1/iaas/accounts", h.handleListIaaSAccounts)
handle("GET /panoramica/v1/iaas/daily-charges", h.handleListDailyCharges)
handle("GET /panoramica/v1/iaas/monthly-charges", h.handleListMonthlyCharges)
handle("GET /panoramica/v1/iaas/charge-breakdown", h.handleChargeBreakdown)
handle("GET /panoramica/v1/iaas/windows-licenses", h.handleListWindowsLicenses)
```

### Phase 3 — Acceptance Criteria

- [ ] `cd backend && go build ./...` compiles
- [ ] `curl .../panoramica/v1/iaas/accounts` returns account list (no 385/485)
- [ ] `curl .../panoramica/v1/iaas/daily-charges?domain=<uuid>` returns daily charge rows
- [ ] `curl .../panoramica/v1/iaas/charge-breakdown?domain=<uuid>&day=2026-04-01` returns `{"charges": [...], "total": N}` typed array
- [ ] `curl .../panoramica/v1/iaas/windows-licenses` returns last 14 days of license counts

---

## Phase 4: Backend — Anisetta Endpoints (Timoo)

**Goal:** Implement the 2 Anisetta endpoints for the Timoo page.

**Create `backend/internal/panoramica/handler_timoo.go`:**

### Handler: `handleListTimooTenants`

```go
// GET /panoramica/v1/timoo/tenants
```

Original query with the KlajdiandCo exclusion added:
```sql
SELECT * FROM public."as7_tenants" WHERE name != 'KlajdiandCo'
```

**IMPORTANT:** The original query uses `SELECT *`. Check `apps/compliance/anisetta_schema.json` for the actual column names and list them explicitly in the Go query. Never use `SELECT *` in production Go code — list the columns needed for the tenant selector (at minimum: `as7_tenant_id`, `name`).

### Handler: `handleGetPbxStats`

```go
// GET /panoramica/v1/timoo/pbx-stats?tenant=123
```

Original query:
```sql
SELECT as7_tenant_id, pbx_id, pbx_name, MAX(users) AS users, MAX(service_extensions) AS service_extensions
FROM public.as7_pbx_accounting apb
WHERE as7_tenant_id = $1
  AND to_char(data, 'YYYY-MM-DD') = (SELECT to_char(data, 'YYYY-MM-DD') FROM public.as7_pbx_accounting ORDER BY id DESC LIMIT 1)
GROUP BY as7_tenant_id, pbx_id, pbx_name
ORDER BY pbx_name
```

**Response:** Backend computes totals (moved from Appsmith JSObject `utils.pbxStats`):

```go
type PbxRow struct {
	PbxName           string `json:"pbx_name"`
	PbxID             int    `json:"pbx_id"`
	Users             int    `json:"users"`
	ServiceExtensions int    `json:"service_extensions"`
	Totale            int    `json:"totale"`
}

type PbxStatsResponse struct {
	Rows       []PbxRow `json:"rows"`
	TotalUsers int      `json:"totalUsers"`
	TotalSE    int      `json:"totalSE"`
}
```

After scanning rows, compute (use index-based mutation — `for _, row` copies the struct):
```go
for i := range rows {
    rows[i].Totale = rows[i].Users + rows[i].ServiceExtensions
    totalUsers += rows[i].Users
    totalSE += rows[i].ServiceExtensions
}
```

### Register Anisetta routes

```go
// ── Anisetta: Timoo ──
handle("GET /panoramica/v1/timoo/tenants", h.handleListTimooTenants)
handle("GET /panoramica/v1/timoo/pbx-stats", h.handleGetPbxStats)
```

### Phase 4B: Backend tests

**Create `backend/internal/panoramica/handler_test.go`:**

Focused tests following the pattern in `backend/internal/kitproducts/handler_test.go`.

**Nil-DB guard tests:**

```go
func TestRequireMistraReturns503WhenNil(t *testing.T) {
	h := &Handler{mistraDB: nil, grappaDB: nil, anisettaDB: nil}
	panoramica.RegisterRoutes(mux, nil, nil, nil)
	// Hit a Mistra endpoint, expect 503 with "mistra_database_not_configured"
}

func TestRequireGrappaReturns503WhenNil(t *testing.T) { /* same pattern for Grappa endpoint */ }
func TestRequireAnisettaReturns503WhenNil(t *testing.T) { /* same pattern for Anisetta endpoint */ }
```

**Bad-parameter validation tests:**

```go
func TestOrdersSummaryRequiresCliente(t *testing.T) {
	// GET /panoramica/v1/orders/summary (no cliente param) → 400
}

func TestOrdersSummaryRequiresStati(t *testing.T) {
	// GET /panoramica/v1/orders/summary?cliente=123 (no stati param) → 400
}

func TestOrdersSummaryRejectsNonIntCliente(t *testing.T) {
	// GET /panoramica/v1/orders/summary?cliente=abc&stati=Evaso → 400
}

func TestInvoicesRejectsInvalidMesi(t *testing.T) {
	// GET /panoramica/v1/invoices?cliente=123&mesi=-1 → 400
	// GET /panoramica/v1/invoices?cliente=123&mesi=abc → 400
}

func TestDailyChargesRequiresDomain(t *testing.T) {
	// GET /panoramica/v1/iaas/daily-charges (no domain) → 400
}

func TestChargeBreakdownRequiresDomainAndDay(t *testing.T) {
	// GET /panoramica/v1/iaas/charge-breakdown (no params) → 400
	// GET /panoramica/v1/iaas/charge-breakdown?domain=x (no day) → 400
}

func TestPbxStatsRequiresTenant(t *testing.T) {
	// GET /panoramica/v1/timoo/pbx-stats (no tenant) → 400
}
```

These tests use `httptest.NewRecorder` and a real `http.ServeMux` with `RegisterRoutes(mux, nil, nil, nil)` — nil DBs ensure the nil-guard tests work without database connections.

### Phase 4 — Acceptance Criteria

- [ ] `cd backend && go build ./...` compiles
- [ ] `cd backend && go test ./internal/panoramica/...` passes — all nil-guard and validation tests green
- [ ] `curl .../panoramica/v1/timoo/tenants` returns tenant list (no KlajdiandCo)
- [ ] `curl .../panoramica/v1/timoo/pbx-stats?tenant=123` returns `{"rows": [...], "totalUsers": N, "totalSE": N}`
- [ ] **All 16 endpoints** now compile and return data

---

## Phase 5: Frontend — API Client + Types + Simple Pages

**Goal:** Implement the API client hooks, TypeScript types, and the simpler pages (Fatture, Accessi, Licenze Windows, Timoo).

### Phase 5A: TypeScript types

**Create `apps/panoramica-cliente/src/types/index.ts`:**

Define all response types matching the backend JSON contracts. Key types:

```typescript
// Customer variants
export interface CustomerWithInvoices { numero_azienda: number; ragione_sociale: string; }
export interface CustomerWithOrders { numero_azienda: number; ragione_sociale: string; }
export interface CustomerWithAccessLines { id: number; intestazione: string; }

// Orders
export interface OrderSummaryRow { stato: string; numero_ordine: string; descrizione_long: string; quantita: number; nrc: number; mrc: number; totale_mrc: number; stato_ordine: string; nome_testata_ordine: string; rn: number; /* ... all 24 fields */ }
export interface OrderDetailRow { /* ... all 60+ fields, matching backend struct */ }

// Invoices
export interface InvoiceLine { documento: string | null; descrizione_riga: string; qta: number; prezzo_unitario: number; prezzo_totale_netto: number; /* ... all fields */ }

// IaaS
export interface IaaSAccount { intestazione: string; credito: number; cloudstack_domain: string; /* ... */ }
export interface DailyCharge { giorno: string; domainid: string; utCredit: number; total_importo: number; }
export interface MonthlyCharge { mese: string; importo: number; }
export interface ChargeItem { type: string; label: string; amount: number; }
export interface ChargeBreakdown { charges: ChargeItem[]; total: number; }
export interface WindowsLicense { x: string; y: number; }

// Timoo
export interface TimooTenant { as7_tenant_id: number; name: string; /* ... */ }
export interface PbxRow { pbx_name: string; pbx_id: number; users: number; service_extensions: number; totale: number; }
export interface PbxStatsResponse { rows: PbxRow[]; totalUsers: number; totalSE: number; }
```

### Phase 5B: React Query hooks

**Create `apps/panoramica-cliente/src/api/queries.ts`:**

```typescript
import { useQuery } from '@tanstack/react-query';
import { useApiClient } from './client';
import type { CustomerWithInvoices, /* ... all types */ } from '../types';

// ── Customer lists ──
export function useCustomersWithInvoices() {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'customers', 'invoices'],
    queryFn: () => api.get<CustomerWithInvoices[]>('/panoramica/v1/customers/with-invoices'),
  });
}

export function useCustomersWithOrders(variant: 'a' | 'b') {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'customers', 'orders', variant],
    queryFn: () => api.get<CustomerWithOrders[]>(`/panoramica/v1/customers/with-orders?variant=${variant}`),
  });
}

// ... similar for all other endpoints

// ── Orders ──
export function useOrdersSummary(cliente: number | null, stati: string[]) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['panoramica', 'orders', 'summary', cliente, stati],
    queryFn: () => api.get<OrderSummaryRow[]>(
      `/panoramica/v1/orders/summary?cliente=${cliente}&stati=${stati.join(',')}`
    ),
    enabled: cliente !== null && stati.length > 0,
  });
}

// ── Invoices ──
export function useInvoices(cliente: number | null, mesi: number | null) {
  const api = useApiClient();
  const params = new URLSearchParams();
  if (cliente) params.set('cliente', String(cliente));
  if (mesi && mesi > 0) params.set('mesi', String(mesi));
  return useQuery({
    queryKey: ['panoramica', 'invoices', cliente, mesi],
    queryFn: () => api.get<InvoiceLine[]>(`/panoramica/v1/invoices?${params}`),
    enabled: cliente !== null,
  });
}

// ... etc for all 16 endpoints
```

**Key patterns:**
- `enabled: false` until required parameters are selected
- `queryKey` includes all parameters for correct cache invalidation
- URL parameters built safely

### Phase 5C: 503 graceful degradation

When a backend DSN is missing, the `require*` guards return HTTP 503. Each page must handle this gracefully.

**Create `apps/panoramica-cliente/src/components/shared/ServiceUnavailable.tsx`:**

```typescript
export function ServiceUnavailable({ service }: { service: string }) {
  return (
    <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--color-text-secondary)' }}>
      <h2>Servizio non disponibile</h2>
      <p>La connessione a {service} non è configurata. Questa sezione non è al momento disponibile.</p>
    </div>
  );
}
```

**Usage in pages:** Each page's React Query hook will receive a 503. Detect it in the query error and render `<ServiceUnavailable service="Mistra" />` (or `"Grappa"` / `"Anisetta"` depending on the page). Pattern:

```typescript
if (query.error && (query.error as any).status === 503) {
  return <ServiceUnavailable service="Mistra" />;
}
```

**Page-to-service mapping:**
- Ordini ricorrenti, Ordini R&S, Fatture → Mistra
- Accessi → Mistra
- IaaS PPU, Licenze Windows → Grappa
- Timoo → Anisetta

### Phase 5D: Implement simple pages

Implement these 4 pages first (they are simpler — no Master-Detail Drawer):

1. **`FatturePage.tsx`** — Customer selector (SingleSelect, required) + period selector (native `<select>` with options 6/12/24/36/null) + auto-refreshing table. Table uses the visual row grouping: bold document header on rows where `documento` is not null.

2. **`AccessiPage.tsx`** — 3 MultiSelects (clients, stati, tipi_conn) + "Cerca" button + results table. Default stati: `["Attiva"]`. Default tipi: all selected. No auto-query until button click.

3. **`LicenzeWindowsPage.tsx`** — Title text + chart. For charts, add `recharts` dependency: `pnpm --filter mrsmith-panoramica-cliente add recharts`. Use `<BarChart>` or `<LineChart>` from recharts. Auto-loads on mount.

4. **`TimooTenantsPage.tsx`** — Tenant selector (SingleSelect) + auto-load PBX stats on selection (spec decision Q17). Summary line with totals + PBX table.

**Table pattern:** Build tables with native HTML `<table>` + CSS Modules (same pattern as listini). Include:
- Client-side search (use `useTableFilter` from `@mrsmith/ui`)
- Sort by column (click header)
- CSV download button
- Staggered row animation

### Phase 5 — Acceptance Criteria

- [ ] `pnpm --filter mrsmith-panoramica-cliente exec tsc --noEmit` passes
- [ ] Fatture page: select customer → invoice lines load with visual grouping
- [ ] Fatture page: change period → table auto-refreshes
- [ ] Accessi page: select clients + click Cerca → access lines load
- [ ] Licenze Windows page: chart renders with 14 days of data
- [ ] Timoo page: select tenant → PBX stats load automatically with totals
- [ ] All tables have CSV export button
- [ ] With a DSN unset, the corresponding page shows "Servizio non disponibile" card instead of a broken state

---

## Phase 6: Frontend — Ordini Pages (Master-Detail Drawer) + IaaS PPU

**Goal:** Implement the two most complex pages: the Ordini pages with Master-Detail Drawer, and the IaaS PPU page with cascading selection.

### Phase 6A: Shared SlideOverPanel component

**Create `apps/panoramica-cliente/src/components/shared/SlideOverPanel.tsx`:**

A reusable slide-over panel component:
```typescript
interface SlideOverPanelProps {
  open: boolean;
  onClose: () => void;
  width?: number;  // default 480
  title: React.ReactNode;
  children: React.ReactNode;
}
```

- Position: fixed, right: 0, top: 0, height: 100vh
- Width: prop-driven (480px for summary, 600px for detail)
- Animation: `translateX(100%) → translateX(0)` with 300ms ease-out
- Backdrop: subtle shadow on the left edge
- Sticky header with title + close button (X)
- Scrollable content area
- Close on Escape key
- Trap focus inside panel when open

### Phase 6B: Ordini ricorrenti page

**Create `apps/panoramica-cliente/src/pages/OrdiniRicorrentiPage.tsx`:**

**Filter bar:** Customer selector (SingleSelect, required) + status MultiSelect (default: Evaso, Confermato) + "Cerca" button.

**Table:** Flat rows from `useOrdersSummary`. Visual grouping:
- First row of each order group (`rn === 1`): taller row (48px), bold `nome_testata_ordine`, `numero_ordine` in mono, top border separator.
- Subsequent rows: indented `descrizione_long`, tree-line connector (left border via CSS).

**Default visible columns (10):** stato_ordine (dot badge), numero_ordine, ordine/descrizione_long, quantita, nrc, mrc, totale_mrc, data_documento, stato_riga (badge), serialnumber.

**Row click → SlideOverPanel (480px):**
- Header: `nome_testata_ordine`, `numero_ordine`, `stato_ordine` badge
- Order metadata: label/value pairs (data_documento, metodo_pagamento, durata_servizio, durata_rinnovo, storico, sost_ord, sostituito_da, note_legali)
- Selected line card: all fields of the clicked row
- Sibling lines: scrollable list of other rows with same `nome_testata_ordine`

**Interactions:**
- Click another row → panel content transitions (cross-fade)
- Arrow up/down navigates rows with panel following
- Escape closes panel

### Phase 6C: Ordini Ricorrenti e Spot page

**Create `apps/panoramica-cliente/src/pages/OrdiniDettaglioPage.tsx`:**

Same structure as OrdiniRicorrentiPage but:
- Uses `useOrdersDetail` hook instead of `useOrdersSummary`
- Default table columns: stato_ordine, ORDINE/descrizione_long, tipo_ordine, commerciale, data_ordine, quantita, mrc, stato_riga, serialnumber, codice_prodotto
- Panel width: 600px
- Panel has **4 tabs** (use native tab pattern — no external tab library):

**Tab "Testata":** 2-column grid of label/value pairs grouped into sections: Anagrafica, Condizioni, Referente Amm., Referente Tech., Referente Altro, Fatturazione, Sostituzioni. (See SPEC.md View: Ordini Ricorrenti e Spot for exact field lists per section.)

**Tab "Riga selezionata":** Full detail of clicked line grouped into: Prodotto, Importi, Date, Stato.

**Tab "Tutte le righe":** Mini-table of all lines for the current order. Click a row → switch to "Riga selezionata" tab.

**Tab "Storico":** Render the `storico` field (if present — comes from `get_reverse_order_history_path`). This is a string like `"ORD-001 > ORD-002 > ORD-003"`. Parse and render as a vertical timeline with dots and connecting lines.

### Phase 6D: IaaS Pay Per Use page

**Create `apps/panoramica-cliente/src/pages/IaaSPayPerUsePage.tsx`:**

**Layout:** Account table (top) → tabbed detail (bottom).

**Account table:** Selectable rows. Auto-select first row on data load. Columns: Intestazione, Credito, Abbreviazione, Serialnumber, Data attivazione. `cloudstack_domain` hidden but stored in state for API calls.

**Tabs** (use simple CSS-based tab toggle):

**Tab "Giornaliero":**
- Daily charges table (from `useDailyCharges`). Columns: Giorno, utCredit, Total Importo.
- When a day row is selected, fetch charge breakdown and render pie chart.
- Pie chart: use `<PieChart>` from recharts. Data from `useChargeBreakdown` typed array. Only non-zero charges shown (backend already filters). Labels from `charge.label`.

**Tab "Mensile":**
- Bar chart from `useMonthlyCharges`. X-axis: mese (YYYY-MM). Y-axis: importo.

**Cascading data loading:**
1. Page mount → `useIaaSAccounts` → auto-select first
2. Account select → trigger `useDailyCharges(domain)` + `useMonthlyCharges(domain)`
3. Day select in daily table → trigger `useChargeBreakdown(domain, day)`

### Phase 6 — Acceptance Criteria

- [ ] `pnpm --filter mrsmith-panoramica-cliente exec tsc --noEmit` passes
- [ ] Ordini ricorrenti: select customer + stati + Cerca → table with visual grouping loads
- [ ] Ordini ricorrenti: click row → panel opens with order metadata + line detail
- [ ] Ordini ricorrenti: click different row → panel transitions smoothly
- [ ] Ordini ricorrenti: Escape closes panel
- [ ] Ordini dettaglio: click row → panel with 4 tabs opens
- [ ] Ordini dettaglio: "Tutte le righe" tab → click line → switches to "Riga selezionata"
- [ ] IaaS PPU: accounts load → first auto-selected → daily+monthly load
- [ ] IaaS PPU: click day → pie chart renders with non-zero charge types
- [ ] IaaS PPU: click different account → data refreshes
- [ ] All 7 pages are functional end-to-end

---

## File Tree — New Files

```
backend/internal/panoramica/
├── handler.go                      # Handler struct, RegisterRoutes, shared helpers
├── handler_customer.go             # 3 customer list handlers
├── handler_orders.go               # 3 order handlers (statuses, summary, detail)
├── handler_fatture.go              # 1 invoice handler
├── handler_accessi.go              # 2 access line handlers
├── handler_iaas.go                 # 5 IaaS handlers
├── handler_timoo.go                # 2 Timoo handlers
└── handler_test.go                 # Nil-DB guards + param validation tests

apps/panoramica-cliente/
├── package.json
├── tsconfig.json
├── index.html
├── vite.config.ts
└── src/
    ├── main.tsx
    ├── App.tsx
    ├── App.module.css
    ├── routes.tsx
    ├── vite-env.d.ts
    ├── api/
    │   ├── client.ts               # useApiClient hook (copy from listini)
    │   └── queries.ts              # All React Query hooks (16 endpoints)
    ├── hooks/
    │   └── useOptionalAuth.ts      # Auth fallback (copy from listini)
    ├── types/
    │   └── index.ts                # All TypeScript types
    ├── styles/
    │   └── global.css              # Global styles (copy from listini)
    ├── components/
    │   └── shared/
    │       ├── ServiceUnavailable.tsx  # 503 graceful degradation card
    │       └── SlideOverPanel.tsx      # Reusable slide-over panel
    └── pages/
        ├── OrdiniRicorrentiPage.tsx
        ├── OrdiniRicorrentiPage.module.css
        ├── OrdiniDettaglioPage.tsx
        ├── OrdiniDettaglioPage.module.css
        ├── FatturePage.tsx
        ├── FatturePage.module.css
        ├── AccessiPage.tsx
        ├── AccessiPage.module.css
        ├── IaaSPayPerUsePage.tsx
        ├── IaaSPayPerUsePage.module.css
        ├── TimooTenantsPage.tsx
        ├── TimooTenantsPage.module.css
        ├── LicenzeWindowsPage.tsx
        └── LicenzeWindowsPage.module.css
```

## Modified Files

| File | Change |
|------|--------|
| `backend/internal/platform/applaunch/catalog.go` | Add `PanoramicaAppID`, `PanoramicaAppHref`, `panoramicaAccessRoles`, `PanoramicaAccessRoles()`. Update existing catalog entry. |
| `backend/internal/platform/applaunch/catalog_test.go` | Update placeholder count (16→15), add `app_panoramica_access` to all-roles test, add `TestVisibleCategoriesFiltersByPanoramicaRole` |
| `backend/cmd/server/main.go` | Add import, `hrefOverrides` block for Panoramica, catalog visibility filter, `panoramica.RegisterRoutes(...)` |
| `backend/internal/platform/config/config.go` | Add `PanoramicaAppURL` field + env load, add `5178` to CORS origins |
| `deploy/Dockerfile` | Add COPY line for panoramica-cliente dist |
| `package.json` (root) | Add Panoramica to root `dev` concurrently command + add `dev:panoramica` script |
| `Makefile` | Add `dev-panoramica` target + update `.PHONY` list |
| `docker-compose.dev.yaml` | Add `panoramica-cliente` service + `panoramica_node_modules` volume |

---

## Open Decisions

None — all questions resolved in SPEC.md and PANORAMICA-FB.md.

**Resolved via FB review:**
- **Partial DSN availability:** App stays visible when at least one DSN is configured. Pages with unavailable backends show an inline "Servizio non disponibile" card (Phase 5C).
- **Dev workflow:** Panoramica included in root `dev` script (Phase 1C).
- **Backend tests:** `handler_test.go` covers nil-DB guards and parameter validation (Phase 4B).
- **PostgreSQL array binding:** `pq.Array` removed; dynamic placeholders only (Phase 2C).
