# Compliance App — Implementation Plan V1.3

Source: `apps/compliance/compliance-migspec.md`
Reference app: `apps/budget/`
Revision: V1.2 + UI/UX review findings

---

## Resolved Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| FB1 | App path is `/apps/compliance/` (not `/apps/smart-apps/compliance/`) | `staticspa` fallback resolves `/apps/<segment[1]>/index.html` — nested paths would 404 on deep links |
| FB3 | Export via authenticated blob download, not `window.open()` | All endpoints require Bearer JWT; browser navigation cannot attach auth headers |
| FB4 | Origin creation requires explicit `{method_id, description}` | Existing PKs are human-chosen codes (AGCOM, GDF, MININT, POLPOST); auto-generation would be fragile |
| FB5 | Origins management page fetches with `include_inactive=true` | Page must show deactivated origins with status badges; active-only default is for creation dropdowns |
| FB6 | Reuse `backend/internal/platform/database/database.go` with `pgx` driver; handler struct with injected `*sql.DB`; env var `ANISETTA_DSN` | Avoids competing DB patterns; enables testability; single env var name across code/deploy/docs |
| FB-V1.2-1 | `DELETE /origins/:id` returns `200` with JSON body `{"method_id":"..."}` (not `204 No Content`) | Shared `ApiClient` always calls `res.json()` on success; 204 would throw a parse error |
| FB-V1.2-2 | Auth middleware 401/403 responses remain plain text | Shared middleware uses `http.Error()`; compliance documents its handler-level errors as JSON but does not promise JSON for middleware-generated auth failures |
| FB-V1.2-3 | `compliance-migspec.md` must be updated to match `POST /origins` contract (`{method_id, description}`) before implementation starts | Plan and source spec must agree; plan supersedes on this point |
| FB-V1.3-1 | **All edits use `Modal` from `@mrsmith/ui` — no in-panel edit mode** | Budget app uses `Modal` for every edit. In-panel edit creates unsolved problems: form overflow in 400px panel, dirty-state on row switch, cancel behavior. Detail panels are always read-only. |
| FB-V1.3-2 | **Domain editing uses `Modal`, not a custom popover** | No popover primitive exists in `@mrsmith/ui`. A custom popover requires focus trapping, Escape handling, positioning logic, and click-outside detection — all of which `Modal` provides for free via native `<dialog>`. |
| FB-V1.3-3 | **Domain status sub-tabs use local `useState` tab bar, not `TabNav`** | `TabNav` is route-based (uses `NavLink`). Sub-tabs are state-based with shared `searchQuery` lifted above them. Using `TabNav` would require sub-routes and break the shared search state. Follow `BudgetDetailPage.tsx:30-59` local tab pattern. |
| FB-V1.3-4 | **All views implement the three-state loading pattern** | Budget app pattern: `isLoading` → `Skeleton`, `isUpstreamAuthFailed(error)` → service-unavailable message, `data` → content. See `apps/budget/src/api/errors.ts` for helpers. |
| FB-V1.3-5 | **Destructive actions always require confirmation via `Modal`** | Budget app uses `GroupDeleteConfirm` and `CostCenterDisableConfirm` for all destructive actions. Origin deactivation must follow the same pattern. |

---

## Overview

4 phases, ordered by dependency. Phases 1–2 run in parallel. Phases 3–4 are sequential.

```
Phase 1: Backend (Go)          ──┐
                                  ├──► Phase 3: Frontend Views + Export ──► Phase 4: Integration & Polish
Phase 2: Frontend Scaffold      ──┘
```

Estimated file count: ~32 new files (backend: ~10, frontend: ~20, config/build: ~2), ~12 modified files.

---

## Phase 0: Spec Reconciliation (before implementation)

**Goal**: Single source of truth across plan and spec.

- [ ] Update `apps/compliance/compliance-migspec.md` section on `POST /origins` to require `{method_id, description}` (not just `{description}`)
- [ ] Update `apps/compliance/compliance-migspec.md` section on `DELETE /origins/:id` to document `200 {"method_id":"..."}` response (not `204 No Content`)
- [ ] Update `apps/compliance/compliance-migspec.md` auth error section to clarify: middleware-generated 401/403 are plain text (`http.Error`), handler-level errors (400, 404, 500) are JSON

---

## Phase 1: Backend — Go Module + Database + API

**Goal**: All API endpoints functional, tested, database-connected.

### 1.1 Database Connection & Config

**Modify** `backend/internal/platform/config/config.go`:
- Add `AnisettaDSN string` field, loaded from `ANISETTA_DSN`
- Add `ComplianceAppURL string` field, loaded from `COMPLIANCE_APP_URL` (split-server dev override, same pattern as `BudgetAppURL`)
- Add `http://localhost:5175` to default `CORS_ORIGINS`

**Modify** `backend/cmd/server/main.go`:
- Import `compliance` and `database` packages
- Add side-effect import: `_ "github.com/jackc/pgx/v5/stdlib"` — registers the `pgx` driver with `database/sql`
- If `cfg.AnisettaDSN != ""`, call `database.New(database.Config{Driver: "postgres", DSN: cfg.AnisettaDSN})` to get `*sql.DB`
- Call `compliance.RegisterRoutes(api, db)` (db may be nil — handlers return 503)
- Add compliance href override logic (same pattern as budget, lines 68-74)

**Modify** `backend/go.mod`:
- Add `github.com/jackc/pgx/v5` (the `stdlib` sub-package requires the module)
- Add `github.com/xuri/excelize/v2` (XLSX export)

**Reuse** `backend/internal/platform/database/database.go` — already supports postgres via `pgx` driver. The `driverName()` func maps `"postgres"` → `"pgx"`, which matches the driver name registered by `pgx/v5/stdlib`.

**Migration** — single SQL file `apps/compliance/migrations/001_add_is_active.sql`:
```sql
ALTER TABLE dns_bl_method ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT TRUE;
```
Manual execution, no migration runner. Document in deploy notes.

### 1.2 Access Roles

**Modify** `backend/internal/platform/applaunch/catalog.go`:
- Add `complianceAccessRoles = []string{"app_compliance_access"}`
- Add `ComplianceAccessRoles() []string` func
- Add `ComplianceAppID = "compliance"` and `ComplianceAppHref = "/apps/compliance/"` constants
- Update compliance app definition: `Href: "/apps/compliance/"`, `AccessRoles: complianceAccessRoles`
- **Remove compliance from default-roles-cdlan placeholder set** (it now requires `app_compliance_access`)

**Modify** `backend/internal/platform/applaunch/catalog_test.go`:
- `TestVisibleCategoriesDefaultRoleSeesAllPlaceholders`: update expected category count and app total (compliance is no longer a placeholder — currently expects 4 categories / 19 apps, both will decrease by the compliance entry)
- `TestVisibleCategoriesBothRolesSeesEverything`: update expected total and add `app_compliance_access` to the role set (currently expects 20 total)
- **Add** `TestVisibleCategoriesFiltersByComplianceRole`: same pattern as `TestVisibleCategoriesFiltersByBudgetRole` — pass `[]string{"app_compliance_access"}`, verify only compliance app visible
- **Add** `TestCatalogAppliesComplianceHrefOverride`: same pattern as budget href override test

**Modify** `backend/internal/portal/handler_test.go`:
- **Add** test: user with `app_compliance_access` sees only compliance app in portal response
- **Add** test: user with both `app_budget_access` and `app_compliance_access` sees both apps

### 1.3 Handler Struct & Route Registration

**Create** `backend/internal/compliance/handler.go`:

```go
type Handler struct {
    db *sql.DB
}

func RegisterRoutes(mux *http.ServeMux, db *sql.DB) {
    h := &Handler{db: db}
    protect := acl.RequireRole(applaunch.ComplianceAccessRoles()...)
    handle := func(pattern string, handler http.HandlerFunc) {
        mux.Handle(pattern, protect(http.HandlerFunc(handler)))
    }
    // Block requests
    handle("GET /compliance/blocks", h.handleListBlocks)
    handle("GET /compliance/blocks/{id}", h.handleGetBlock)
    handle("POST /compliance/blocks", h.handleCreateBlock)
    handle("PUT /compliance/blocks/{id}", h.handleUpdateBlock)
    handle("GET /compliance/blocks/{id}/domains", h.handleListBlockDomains)
    handle("POST /compliance/blocks/{id}/domains", h.handleAddBlockDomains)
    handle("PUT /compliance/blocks/{id}/domains/{domainId}", h.handleUpdateBlockDomain)
    // Release requests
    handle("GET /compliance/releases", h.handleListReleases)
    handle("GET /compliance/releases/{id}", h.handleGetRelease)
    handle("POST /compliance/releases", h.handleCreateRelease)
    handle("PUT /compliance/releases/{id}", h.handleUpdateRelease)
    handle("GET /compliance/releases/{id}/domains", h.handleListReleaseDomains)
    handle("POST /compliance/releases/{id}/domains", h.handleAddReleaseDomains)
    handle("PUT /compliance/releases/{id}/domains/{domainId}", h.handleUpdateReleaseDomain)
    // Domain status & history
    handle("GET /compliance/domains", h.handleListDomainStatus)
    handle("GET /compliance/domains/history", h.handleListHistory)
    // Origins
    handle("GET /compliance/origins", h.handleListOrigins)
    handle("POST /compliance/origins", h.handleCreateOrigin)
    handle("PUT /compliance/origins/{id}", h.handleUpdateOrigin)
    handle("DELETE /compliance/origins/{id}", h.handleDeleteOrigin)
}
```

All handlers are methods on `*Handler` — testable with injected DB.

### 1.4 Models & Validation

**Create** `backend/internal/compliance/models.go`:
- Go structs for all request/response types: `BlockRequest`, `BlockDomain`, `ReleaseRequest`, `ReleaseDomain`, `Origin`, `DomainStatus`, `HistoryEntry`
- `CreateOriginRequest` includes both `method_id` and `description`

**Create** `backend/internal/compliance/validation.go`:
- `ValidateFQDN(domain string) bool` — canonical regex
- `ValidateDomains(domains []string) (valid, invalid []string)`

FQDN regex (canonical, documented in both layers):
```
^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$
```

### 1.5 Handlers — Block Requests

**Create** `backend/internal/compliance/handler_blocks.go`:

Key behaviors:
- `POST /blocks` — transactional: insert header + batch domains. Validate all domains first; reject 400 with `{error: "invalid_domains", invalid: [...]}` if any fail. Rollback on partial failure.
- `POST /blocks/{id}/domains` — transactional batch insert. Same validation.
- `PUT /blocks/{id}` — update header fields (date, reference, method_id).
- `PUT /blocks/{id}/domains/{domainId}` — **ownership check**: `WHERE id = $1 AND block_id = $2` (not just `WHERE id = $1`). Returns 404 if domain doesn't belong to this block.
- List: join with `dns_bl_method` to include `method_description`.
- All queries parameterized (fixes legacy SQL injection).

### 1.6 Handlers — Release Requests

**Create** `backend/internal/compliance/handler_releases.go`:

Mirror of block handlers, no `method_id`. Same ownership check on domain updates: `WHERE id = $1 AND release_id = $2`.

### 1.7 Handlers — Domain Status & History

**Create** `backend/internal/compliance/handler_domains.go`:

Domain status (BR1): aggregate UNION query with `HAVING block_count > release_count` (blocked) or `<= release_count` (released).

History: UNION of block+release domains with request headers. **Deterministic ordering**: `ORDER BY request_date DESC, domain ASC, request_type ASC`.

Both endpoints support `?format=csv|xlsx` and `?search=` for export (see 1.8).

Search filter: `?search=` applies `WHERE domain ILIKE '%' || $1 || '%'` — same column as frontend client-side filter, ensuring **export matches visible rows**.

### 1.8 Export

**Create** `backend/internal/compliance/export.go`:
- `writeCSV(w http.ResponseWriter, filename string, headers []string, rows [][]string)` using `encoding/csv`
- `writeXLSX(w http.ResponseWriter, filename string, headers []string, rows [][]string)` using `excelize`
- Sets `Content-Disposition: attachment` and correct `Content-Type`

Response headers:
```
Content-Type: text/csv / application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
Content-Disposition: attachment; filename="domains-blocked-2026-04-07.csv"
```

### 1.9 Handlers — Origins

**Create** `backend/internal/compliance/handler_origins.go`:
- `GET /origins` — default returns `is_active=true` only. `?include_inactive=true` returns all.
- `POST /origins` — requires `{method_id, description}`. Validates `method_id` is non-empty and unique. Inserts with `is_active=true`.
- `PUT /origins/{id}` — updates `description` only (`method_id` is immutable PK).
- `DELETE /origins/{id}` — soft delete: `UPDATE dns_bl_method SET is_active = false WHERE method_id = $1`. **Returns `200 {"method_id":"..."}` (not 204)** — required for compatibility with shared `ApiClient` which always calls `res.json()`.

### 1.10 Tests

**Create** `backend/internal/compliance/validation_test.go`:
- FQDN edge cases: valid FQDNs, invalid (IPs, wildcards, empty, unicode, trailing dots)

**Create** `backend/internal/compliance/handler_test.go`:
- Handler tests via `httptest` with test DB or mock
- **Ownership validation**: attempt to update domain with wrong parent ID → 404
- **Transaction rollback**: create block with mix of valid/invalid domains → 400, verify no partial insert
- **Export**: verify export endpoints return correct Content-Type and Content-Disposition headers behind ACL
- **Filter parity**: verify export with `?search=` matches expected rows
- **Delete origin**: verify `DELETE /origins/:id` returns `200` with JSON body `{"method_id":"..."}`

**Note on auth testing**: Handler-level tests verify ACL (role enforcement) only. Token-level auth (`Authorization: Bearer`) is enforced by the shared `auth.Middleware` in `main.go` and is tested separately (see Phase 4, item 4.3).

### Phase 1 Deliverables

- [ ] `backend/internal/compliance/` — 10 Go files
- [ ] `backend/internal/platform/applaunch/catalog.go` — role + catalog update
- [ ] `backend/internal/platform/applaunch/catalog_test.go` — updated counts + new compliance role tests
- [ ] `backend/internal/portal/handler_test.go` — new compliance role visibility tests
- [ ] `backend/internal/platform/config/config.go` — `AnisettaDSN`, `ComplianceAppURL`, CORS
- [ ] `backend/cmd/server/main.go` — compliance module registration + DB init + href override + `pgx/stdlib` import
- [ ] `backend/go.mod` — `pgx/v5` + `excelize` dependencies
- [ ] Migration script for `is_active` column

---

## Phase 2: Frontend Scaffold (parallel with Phase 1)

**Goal**: App skeleton running with routing, layout, auth, and API client — no real views yet.

### 2.1 Project Setup

**Create** `apps/compliance/package.json`:
```json
{
  "name": "mrsmith-compliance",
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
    "@tanstack/react-virtual": "^3.0.0",
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

**Note**: Scripts match budget convention exactly: `build` uses `tsc -b`, `lint` uses `tsc --noEmit`, `preview` included.

**Create** `apps/compliance/vite.config.ts`:
```typescript
export default defineConfig(({ command }) => ({
  base: command === 'build' ? '/apps/compliance/' : '/',
  plugins: [react()],
  server: {
    port: 5175,
    proxy: {
      '/api': 'http://localhost:8080',
      '/config': 'http://localhost:8080',
    },
  },
}));
```

**Port `5175`**, both `/api` and `/config` proxied.

**Create** `apps/compliance/tsconfig.json` — copy from budget.
**Create** `apps/compliance/index.html` — copy from budget, update title to "Compliance".

### 2.2 Build Integration

**Modify** root `package.json` `scripts.dev`:
```
concurrently --names backend,portal,budget,compliance --prefix-colors blue,green,magenta,cyan "cd backend && air" "pnpm --filter mrsmith-portal dev" "pnpm --filter mrsmith-budget dev" "pnpm --filter mrsmith-compliance dev"
```
Add `"dev:compliance": "pnpm --filter mrsmith-compliance dev"`.

**Modify** `backend/internal/platform/config/config.go` — add `http://localhost:5175` to default CORS origins:
```go
CORSOrigins: envOr("CORS_ORIGINS", "http://localhost:5173,http://localhost:5174,http://localhost:5175"),
```

**Modify** `backend/cmd/server/main.go` — add compliance href override:
```go
if cfg.ComplianceAppURL != "" {
    hrefOverrides[applaunch.ComplianceAppID] = cfg.ComplianceAppURL
} else if cfg.StaticDir == "" {
    hrefOverrides[applaunch.ComplianceAppID] = "http://localhost:5175"
}
```

**Modify** `deploy/Dockerfile`:
```dockerfile
COPY --from=frontend /app/apps/compliance/dist /static/apps/compliance
```

**Modify** `docker-compose.dev.yaml` — add compliance frontend service:
```yaml
  compliance:
    image: node:20-slim
    working_dir: /app
    command: sh -c "corepack enable && pnpm install && pnpm --filter mrsmith-compliance dev --host 0.0.0.0"
    volumes:
      - .:/app
      - compliance_node_modules:/app/node_modules
    ports:
      - "5175:5175"
    depends_on:
      - backend
```
Add `compliance_node_modules:` to the `volumes:` section.

**Modify** `deploy/k8s/deployment.yaml` — add `ANISETTA_DSN` from a Secret:
```yaml
          envFrom:
            - configMapRef:
                name: mrsmith-config
          env:
            - name: ANISETTA_DSN
              valueFrom:
                secretKeyRef:
                  name: mrsmith-secrets
                  key: ANISETTA_DSN
                  optional: true
```
DSN is a credential — sourced from a Secret, not the ConfigMap. `optional: true` so the app still starts without it (handlers return 503).

### 2.3 Bootstrap & Layout

**Create** `apps/compliance/src/main.tsx` — same bootstrap pattern as budget (`apps/budget/src/main.tsx`):
1. Fetch `/config` endpoint
2. Validate Keycloak config completeness
3. Render provider tree: `AuthProvider` → `QueryClientProvider` → `BrowserRouter` (basename from `import.meta.env.BASE_URL`) → `ToastProvider` → `App`

**Create** `apps/compliance/src/App.tsx`:
- `AppShell` + `TabNav` with 5 tabs
- Copy budget's `data-theme="clean"` and clean-theme CSS import
- Handle `loading` and `reauthenticating` states (same as budget `App.tsx`)

| Tab Label | Path |
|-----------|------|
| Blocchi | `/blocks` |
| Rilasci | `/releases` |
| Stato domini | `/domains` |
| Riepilogo | `/history` |
| Provenienze | `/origins` |

**Create** `apps/compliance/src/routes.tsx` — route definitions, default redirect to `/blocks`.

### 2.4 API Layer

**Create** `apps/compliance/src/api/client.ts` — `useApiClient()` hook (identical pattern to `apps/budget/src/api/client.ts`).

**Create** `apps/compliance/src/api/errors.ts` — copy from `apps/budget/src/api/errors.ts`:
```typescript
// Must include both helpers:
export function isUpstreamAuthFailed(error: unknown): error is ApiError { ... }
export function getApiErrorMessage(error: unknown): string { ... }
```
These are used by every view for the three-state loading pattern (see §3.0).

**Create** `apps/compliance/src/api/types.ts`:
```typescript
export interface BlockRequest { id: number; request_date: string; reference: string; method_id: string; method_description: string; }
export interface BlockDomain { id: number; domain: string; }
export interface ReleaseRequest { id: number; request_date: string; reference: string; }
export interface ReleaseDomain { id: number; domain: string; }
export interface Origin { method_id: string; description: string; is_active: boolean; }
export interface DomainStatus { domain: string; block_count: number; release_count: number; }
export interface HistoryEntry { domain: string; request_date: string; reference: string; request_type: 'block' | 'release'; }
export interface ValidationErrorResponse { error: 'invalid_domains'; message: string; invalid: string[]; }
```

**Create** `apps/compliance/src/api/queries.ts` — query key factory + all React Query hooks (stubs initially):

```ts
export const complianceKeys = {
  blocks: ['compliance', 'blocks'] as const,
  block: (id: number) => ['compliance', 'blocks', id] as const,
  blockDomains: (id: number) => ['compliance', 'blocks', id, 'domains'] as const,
  releases: ['compliance', 'releases'] as const,
  release: (id: number) => ['compliance', 'releases', id] as const,
  releaseDomains: (id: number) => ['compliance', 'releases', id, 'domains'] as const,
  domains: (status: string) => ['compliance', 'domains', status] as const,
  history: ['compliance', 'domains', 'history'] as const,
  origins: ['compliance', 'origins'] as const,
};
```

### 2.5 Extend @mrsmith/api-client with getBlob()

**Modify** `packages/api-client/src/client.ts`:
- Add `getBlob(path: string): Promise<Blob>` to `ApiClient` interface
- Implementation: authenticated `fetch` with same token injection, returns `res.blob()`
- This enables authenticated file downloads across all apps

### 2.6 Shared Utilities

**Create** `apps/compliance/src/utils/fqdn.ts`:
- `isValidFQDN(domain: string): boolean` — same canonical regex as backend
- `parseDomains(text: string): { valid: string[], invalid: string[] }` — split by newline, trim, validate each

**Create** `apps/compliance/src/hooks/useOptionalAuth.ts` — copy from budget.

### 2.7 Placeholder Views

Create stub components for each route so routing works end-to-end:
- `apps/compliance/src/views/blocks/BlocksPage.tsx`
- `apps/compliance/src/views/releases/ReleasesPage.tsx`
- `apps/compliance/src/views/domains/DomainsPage.tsx`
- `apps/compliance/src/views/history/HistoryPage.tsx`
- `apps/compliance/src/views/origins/OriginsPage.tsx`

### Phase 2 Deliverables

- [ ] `apps/compliance/` — ~13 files (config + src scaffold, including `api/errors.ts`)
- [ ] Root `package.json` dev script updated
- [ ] `packages/api-client/src/client.ts` — `getBlob()` added
- [ ] `deploy/Dockerfile` updated
- [ ] `docker-compose.dev.yaml` — compliance service added
- [ ] `deploy/k8s/deployment.yaml` — `ANISETTA_DSN` from Secret
- [ ] Config: CORS origins, compliance href override
- [ ] App boots on port 5175, tabs navigate, auth works, API client ready

---

## Phase 3: Frontend Views + Export (depends on Phase 1 + 2)

**Goal**: All 5 views fully functional with real API integration, including export.

> **IMPORTANT — UI/UX ground rules for the entire phase:**
>
> 1. **All edits open a `Modal`** (from `@mrsmith/ui`). Detail panels are always read-only. Never morph a read panel into a form. Reference: every budget edit uses `Modal` — see `GroupEditModal`, `CostCenterEditModal`, `BudgetEditModal`.
> 2. **All destructive actions require a confirmation `Modal`**. Reference: `CostCenterDisableConfirm.tsx`, `GroupDeleteConfirm.tsx`, `BudgetDeleteConfirm.tsx`.
> 3. **All views implement the three-state loading pattern**: `isLoading` → `<Skeleton rows={N} />`, `isUpstreamAuthFailed(error)` → service-unavailable empty state, `data` → content. Import `isUpstreamAuthFailed` from `api/errors.ts`. Reference: `GruppiPage.tsx:37-46`, `CentriDiCostoPage.tsx:60-68`.
> 4. **Empty states** use the standard layout: 72×72px icon container (`--radius-xl`, `--color-surface` background), title at `0.9375rem` weight 600, subtitle at `0.8125rem` muted color. Reference: `GruppiPage.tsx:47-55` and `GruppiPage.module.css:349-381`.
> 5. **Error handling in mutations**: on `onError`, check `error instanceof ApiError` → toast error body message; else toast `'Errore di connessione'`. On `onSuccess`, fire a success toast and close the modal. Reference: `BudgetCreateModal.tsx:26-43`.
> 6. **Pending state on submit buttons**: while mutation is in flight, show Italian gerund text (e.g., `'Creazione...'`, `'Salvataggio...'`, `'Disabilitazione...'`) and set `disabled`. Reference: `BudgetCreateModal.tsx:79`, `CostCenterDisableConfirm.tsx:70`.
> 7. **CSS Modules per view**: each view gets its own `*.module.css`. Copy button/form/empty-state styles from `GruppiPage.module.css` — do not reinvent them. Use the same CSS custom property names (`--color-accent`, `--color-border`, `--radius-md`, etc.).
> 8. **Italian labels throughout** — never render raw English API values. `request_type: 'block'` renders as "Blocco", `'release'` renders as "Rilascio".
> 9. **Responsive layout**: master-detail pages use `@media (max-width: 1000px) { grid-template-columns: 1fr; }` with `position: static` on the detail panel. Reference: `GruppiPage.module.css:549-557`.

### 3.0 Error Utilities (prerequisite for all views)

**Create** `apps/compliance/src/api/errors.ts`:
Copy `apps/budget/src/api/errors.ts` verbatim. It provides:
- `isUpstreamAuthFailed(error)` — checks for `ApiError` with status 502 and code `UPSTREAM_AUTH_FAILED`
- `getApiErrorMessage(error)` — extracts user-facing error message from `ApiError.body`

Every view imports these. Every query's error branch must use them.

### 3.1 Shared Components (build first, reused across views)

#### 3.1.1 `AddDomainsModal` — `components/AddDomainsModal.tsx`

**Purpose**: Modal for bulk-adding domains to a block or release request.

**Props**:
```typescript
interface AddDomainsModalProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (domains: string[]) => void;
  isPending: boolean;   // from the parent's mutation
  title: string;        // "Aggiungi domini al blocco" or "Aggiungi domini al rilascio"
}
```

**Layout** (inside `<Modal open={open} onClose={onClose} title={title}>`):
1. **Textarea** — `<textarea>` with CSS class `.textarea` (see 3.7 for style spec). Placeholder: `"Inserisci un dominio per riga"`. Full width, min-height 120px, max-height 240px, `resize: vertical`.
2. **Domain preview** — `<DomainPreview>` renders below the textarea. Only visible when textarea is non-empty. Shows parsed results.
3. **Action bar** — same `.actions` layout as budget modals (`justify-content: flex-end`, `gap: var(--space-3)`, top border).
   - "Annulla" button (`btnSecondary`, calls `onClose`)
   - "Aggiungi" button (`btnPrimary`, calls `onSubmit(validDomains)`)
     - **Disabled when**: `isPending`, or textarea is empty, or any invalid domain exists
     - **Pending text**: `'Aggiunta...'`

**Parsing behavior**:
- Parsing runs on every `onChange` of the textarea, **debounced at 150ms** using a `setTimeout`/`clearTimeout` pattern (no external debounce library needed).
- `parseDomains(text)` from `utils/fqdn.ts` splits by newline, trims whitespace, filters empty lines, validates each with `isValidFQDN()`.
- Result: `{ valid: string[], invalid: string[] }` stored in component state.

**Post-submit behavior**: parent calls the mutation. On success: parent closes the modal via `onClose()` and fires `toast('Domini aggiunti')`. The modal does not manage its own close — the parent does. On next open, textarea state is reset (use `useEffect` on `open` to clear).

**Backend validation fallback**: if the backend returns 400 with `{ error: 'invalid_domains', invalid: [...] }`, the parent should extract `invalid` from the error body and display it. The recommended approach: catch this specific error in the mutation's `onError`, and instead of a generic toast, fire `toast('Alcuni domini non sono validi: ' + invalid.join(', '), 'error')`. This is simpler than re-rendering the modal with backend errors.

#### 3.1.2 `DomainPreview` — `components/DomainPreview.tsx`

**Purpose**: Read-only list of parsed domains with valid/invalid visual markers.

**Props**:
```typescript
interface DomainPreviewProps {
  valid: string[];
  invalid: string[];
}
```

**Layout**:
- Container div with `max-height: 240px`, `overflow-y: auto`, border `1px solid var(--color-border)`, `border-radius: var(--radius-md)`.
- Each domain is a row (`display: flex`, `align-items: center`, `gap: var(--space-2)`, `padding: var(--space-1) var(--space-3)`).
- **Valid domain row**: small green circle (7px, `--color-success`) + domain text in `--color-text` at `0.8125rem`.
- **Invalid domain row**: small red circle (7px, `--color-danger`) + domain text in `--color-danger` at `0.8125rem`, `font-weight: 500`.
- Invalid domains are listed first (so the user sees problems immediately), then valid domains.
- **Summary line** at the top of the container: `"{valid.length} validi, {invalid.length} non validi"` in `0.75rem`, `--color-text-muted`. Only show the invalid count part if `invalid.length > 0`, styled in `--color-danger`.

**No virtualization needed** — this is inside a modal for a single bulk-add operation. The `max-height: 240px` with scroll handles large pastes.

#### 3.1.3 `DomainList` — `components/DomainList.tsx`

**Purpose**: Read-only domain list displayed inside the detail panel of blocks/releases.

**Props**:
```typescript
interface DomainListProps {
  domains: Array<{ id: number; domain: string }>;
  onEdit: (domain: { id: number; domain: string }) => void;
}
```

**Layout**:
- Container div with `max-height: 320px`, `overflow-y: auto` (CSS scroll, **not** virtual scroll — individual block/release requests average ~8 domains, outliers up to ~100).
- Section label `"Domini"` above the list using `.sectionLabel` style (same as `GruppiPage.module.css:279-284`: `0.6875rem`, `font-weight: 600`, `uppercase`, `letter-spacing: 0.1em`, `--color-text-muted`).
- Each domain row: `display: flex`, `align-items: center`, `padding: var(--space-2) var(--space-3)`, `border-radius: var(--radius-md)`.
  - Domain text: `0.8125rem`, `--color-text`, `font-weight: 500`, monospace-style (`font-family: 'SF Mono', 'Fira Code', monospace; font-size: 0.8rem`).
  - Edit button: **always visible** (not hover-only — hover-only breaks on touch devices). Small 28×28px icon button, same style as `.allocEditBtn` in `BudgetDetailPage.module.css:299-316`. Pencil SVG icon.
  - On click: calls `onEdit(domain)`.

**Empty state**: if `domains.length === 0`, show `"Nessun dominio"` in `--color-text-muted`, `0.8125rem`.

#### 3.1.4 `DomainEditModal` — `components/DomainEditModal.tsx`

**Purpose**: Edit a single domain's FQDN value. Uses `Modal` from `@mrsmith/ui`.

> **Decision**: This replaces the `DomainEditPopover` from V1.2. Using `Modal` gives us focus trapping, Escape-to-close, click-outside-to-close, and `aria-modal` accessibility — all for free via the native `<dialog>` element in `@mrsmith/ui Modal`. A custom popover would require implementing all of these manually with no existing primitive.

**Props**:
```typescript
interface DomainEditModalProps {
  open: boolean;
  onClose: () => void;
  domain: { id: number; domain: string } | null;
  onSave: (id: number, newDomain: string) => void;
  isPending: boolean;
}
```

**Layout** (inside `<Modal open={open} onClose={onClose} title="Modifica dominio">`):
1. **Form group**: label `"Dominio"` (`.label` style) + input (`.input` style) pre-filled with `domain.domain`.
2. **Validation indicator below input**:
   - If input is valid FQDN: green text `"Dominio valido"` using `--color-success`, `0.75rem` (like `.helpText`).
   - If input is invalid FQDN: red text `"Dominio non valido"` using `--color-danger`, `0.75rem` (like `.errorText`). Input border turns red (`.inputError` style from `BudgetDetailPage.module.css:595-596`).
   - Validation runs on every keystroke — no debounce needed for a single input.
3. **Action bar**:
   - "Annulla" (`btnSecondary`, calls `onClose`)
   - "Salva" (`btnPrimary`, calls `onSave(domain.id, newValue)`)
     - **Disabled when**: `isPending`, or input is empty, or FQDN is invalid, or value unchanged from original
     - **Pending text**: `'Salvataggio...'`

#### 3.1.5 `ExportButtons` — `components/ExportButtons.tsx`

**Purpose**: CSV and XLSX download buttons that use authenticated blob fetch.

**Props**:
```typescript
interface ExportButtonsProps {
  basePath: string;    // e.g., '/compliance/domains' or '/compliance/domains/history'
  params: Record<string, string>;  // e.g., { status: 'blocked', search: 'example.com' }
}
```

**Layout**: two `btnSecondary` buttons side by side (`display: inline-flex`, `gap: var(--space-2)`):
- "CSV" button with a download icon SVG (small, 14×14)
- "XLSX" button with same icon

**Behavior**:
1. On click, set the button to a **loading state**: text changes to `'Esportazione...'`, `disabled={true}`. Use local `useState<'csv' | 'xlsx' | null>(null)` to track which button is loading.
2. Call `api.getBlob(basePath + '?' + new URLSearchParams({...params, format: 'csv'}).toString())`.
3. On success: create `URL.createObjectURL(blob)`, create a hidden `<a>` element with `download` attribute set to the filename (e.g., `domains-blocked-2026-04-07.csv`), trigger a click, then `URL.revokeObjectURL()`.
4. On error: fire `toast('Errore durante l\'esportazione', 'error')`. Reset button state.
5. On completion (success or error): reset loading state to `null`.

#### 3.1.6 `DeactivateOriginConfirm` — `components/DeactivateOriginConfirm.tsx`

**Purpose**: Confirmation modal before soft-deleting an origin.

**Pattern**: Identical to `CostCenterDisableConfirm.tsx` — read that file as the exact template.

**Props**:
```typescript
interface DeactivateOriginConfirmProps {
  open: boolean;
  onClose: () => void;
  origin: Origin | null; // { method_id, description, is_active }
}
```

**Layout** (inside `<Modal open={open} onClose={onClose} title="Disabilita provenienza">`):
1. **Confirmation message**: `"Disabilitare la provenienza "` + `<strong>{origin.method_id}</strong>` + `"?"` followed by an impact description: `"Non sara piu selezionabile per nuove richieste di blocco. Le richieste esistenti non saranno modificate."` Use `.confirmMessage` style.
2. **Action bar**:
   - "Annulla" (`btnSecondary`)
   - "Disabilita" (`btnDanger`)
     - **Pending text**: `'Disabilitazione...'`
     - On success: `toast('Provenienza disabilitata')`, `onClose()`, invalidate origins query.
     - On error: standard error toast pattern.

### 3.2 Blocks View (`/blocks`) — Master-Detail

**Reference implementation**: `GruppiPage.tsx` is the exact structural template. Copy its layout, state management, and three-state rendering pattern.

#### 3.2.1 `BlocksPage` — `views/blocks/BlocksPage.tsx`

**State**:
```typescript
const [selectedBlockId, setSelectedBlockId] = useState<number | null>(null);
const [showCreate, setShowCreate] = useState(false);
const [showEdit, setShowEdit] = useState(false);
const [showAddDomains, setShowAddDomains] = useState(false);
const [editingDomain, setEditingDomain] = useState<{ id: number; domain: string } | null>(null);
```

**Layout** — two-panel grid, identical to `GruppiPage`:
```css
.page {
  display: grid;
  grid-template-columns: 1fr 400px;
  gap: var(--space-8, 32px);
  min-height: calc(100vh - 60px - 80px);
  animation: pageEnter 0.5s var(--ease-out) both;
}
@media (max-width: 1000px) {
  .page { grid-template-columns: 1fr; }
  .detail { position: static; }
}
```

**Master panel** (left):
- Toolbar: page title `"Blocchi"`, subtitle `"Gestisci le richieste di blocco domini"`, primary button `"Nuova richiesta"` (opens `BlockCreateModal`).
- Table card (`.tableCard`):
  - Three-state rendering:
    - `blocksLoading` → `<Skeleton rows={5} />`
    - `isUpstreamAuthFailed(blocksError)` → service-unavailable empty state: title `"Servizio temporaneamente non disponibile"`, text `"L'elenco delle richieste di blocco non puo essere caricato in questo momento."`
    - `!blocks || blocks.length === 0` → empty state with icon, title `"Nessuna richiesta di blocco"`, text `"Crea la tua prima richiesta di blocco per iniziare"`
    - `blocks` present → `<BlocksTable>`
  - **Virtual scrolling**: use `@tanstack/react-virtual` for the block list. The DB has 1,255 block requests. The virtualizer wraps the row list inside `.tableBody`. Each row has a fixed estimated height of ~52px. Reference the `@tanstack/react-virtual` docs for `useVirtualizer({ count, getScrollElement, estimateSize })`.

**Detail panel** (right, sticky):
- Same panel structure as `GruppiPage.tsx:98-178`:
  - No selection: empty state with icon, title `"Seleziona una richiesta"`, text `"Scegli una richiesta dalla lista per vederne i dettagli"`
  - Loading: `<Skeleton rows={4} />`
  - Upstream auth failed: `"Dettaglio non disponibile"` / `"I dettagli della richiesta non sono al momento raggiungibili."`
  - Data loaded: `<BlockDetail>`

**Modals** (rendered at bottom of component, same pattern as `GruppiPage.tsx:181-199`):
```tsx
<BlockCreateModal open={showCreate} onClose={() => setShowCreate(false)} />
{selectedBlockId && (
  <>
    <BlockEditModal open={showEdit} onClose={() => setShowEdit(false)} blockId={selectedBlockId} />
    <AddDomainsModal
      open={showAddDomains}
      onClose={() => setShowAddDomains(false)}
      title="Aggiungi domini al blocco"
      onSubmit={(domains) => addBlockDomains.mutate({ blockId: selectedBlockId, domains })}
      isPending={addBlockDomains.isPending}
    />
    <DomainEditModal
      open={!!editingDomain}
      onClose={() => setEditingDomain(null)}
      domain={editingDomain}
      onSave={(id, newDomain) => updateBlockDomain.mutate({ blockId: selectedBlockId, domainId: id, domain: newDomain })}
      isPending={updateBlockDomain.isPending}
    />
  </>
)}
```

#### 3.2.2 `BlocksTable` — `views/blocks/BlocksTable.tsx`

**Props**:
```typescript
interface BlocksTableProps {
  blocks: BlockRequest[];
  selectedId: number | null;
  onSelect: (id: number) => void;
}
```

**Columns**: table header with `Data`, `Provenienza`, `Riferimento` columns.
**Row**: same grid/animation pattern as `GruppiPage.module.css` rows (accent bar, icon, hover/selected states, chevron). Each row shows `request_date` (formatted as `DD/MM/YYYY`), `method_description`, `reference`.

**Virtual scrolling**: the parent passes a scroll container ref. `BlocksTable` uses `useVirtualizer` from `@tanstack/react-virtual`. Each virtual row is rendered with `position: absolute`, `top: virtualRow.start`, `height: virtualRow.size`. The outer container has `position: relative` and `height: totalSize`.

#### 3.2.3 `BlockDetail` — `views/blocks/BlockDetail.tsx`

**Purpose**: Read-only detail panel for a selected block request. **Never contains a form.**

**Props**:
```typescript
interface BlockDetailProps {
  block: BlockRequest;
  domains: BlockDomain[];
  domainsLoading: boolean;
  onEdit: () => void;          // opens BlockEditModal
  onAddDomains: () => void;    // opens AddDomainsModal
  onEditDomain: (d: { id: number; domain: string }) => void; // opens DomainEditModal
}
```

**Layout** (inside `.detailContent`, same structure as `GruppiPage.tsx:118-177`):
1. **Header**: icon (48×48 gradient, same as `.detailIconLg`) + title showing the reference text + meta showing the date.
2. **Info section** (below divider):
   - Row: icon + label `"Provenienza"` + value `block.method_description`
   - Row: icon + label `"Riferimento"` + value `block.reference`
   - Row: icon + label `"Data"` + value formatted as `DD/MM/YYYY`
3. **Divider**
4. **Domains section**: `<DomainList domains={domains} onEdit={onEditDomain} />` (or `<Skeleton rows={3} />` while loading)
5. **Divider**
6. **Actions bar** (`.detailActions`):
   - "Modifica" button (`btnSecondary`, calls `onEdit`) — opens `BlockEditModal`
   - "Aggiungi domini" button (`btnPrimary`, calls `onAddDomains`) — opens `AddDomainsModal`

#### 3.2.4 `BlockCreateModal` — `views/blocks/BlockCreateModal.tsx`

**Pattern**: Same as `BudgetCreateModal.tsx` — `<Modal>` wrapping a `<form onSubmit>`.

**Props**:
```typescript
interface BlockCreateModalProps {
  open: boolean;
  onClose: () => void;
}
```

**Form fields**:
1. **Data** — `<input type="date">` with `.input` style. Default value: today (`new Date().toISOString().split('T')[0]`).
2. **Riferimento** — `<input type="text">` with `.input` style. Placeholder: `"Numero protocollo"`. Required.
3. **Provenienza** — `<select>` with `.input` style. Populated from `useOrigins()` (active origins only, no `includeInactive`).
   - **While origins are loading**: show `<option disabled>Caricamento...</option>` and disable the select.
   - **If origins list is empty after loading**: show `<option disabled>Nessuna provenienza disponibile</option>`. Also show a help text below: `"Vai alla sezione Provenienze per crearne una"` in `--color-danger`, `0.75rem`.
   - **Default selection**: first origin in the list (do NOT hardcode "AGCOM" — it may have been deactivated).
4. **Domini** — `<textarea>` same spec as `AddDomainsModal` textarea. Below it: `<DomainPreview valid={...} invalid={...} />`.

**Submit button**: `"Crea richiesta"` / `"Creazione..."`. Disabled when: `isPending`, or reference empty, or origins loading, or no origins available, or no valid domains, or any invalid domain exists.

**On success**: `toast('Richiesta di blocco creata')`, `onClose()`, reset form state. Invalidate `blocks`, `domains/*`, `history` query keys.

**On error**: if backend returns `{ error: 'invalid_domains', invalid: [...] }`, toast the specific invalids. Otherwise standard error toast.

#### 3.2.5 `BlockEditModal` — `views/blocks/BlockEditModal.tsx`

**Pattern**: Same as `GroupEditModal` — fetch current data, pre-fill form, submit update.

**Props**: `open`, `onClose`, `blockId`.

**Form fields**: same as create but without domains textarea (domains are managed separately via `AddDomainsModal` and `DomainEditModal`). Pre-filled with current block data from `useBlock(blockId)`.

**Submit**: `"Salva"` / `"Salvataggio..."`. On success: `toast('Richiesta aggiornata')`, `onClose()`, invalidate `blocks`, `block(id)`.

**Hooks** (in `api/queries.ts`):
- `useBlocks()`, `useBlock(id)`, `useBlockDomains(id)`
- `useCreateBlock()`, `useUpdateBlock()`, `useAddBlockDomains()`, `useUpdateBlockDomain()`

**Cache invalidation**:
- Create/update block → invalidate `blocks`, `domains/*`, `history`
- Add/update domains → invalidate `blockDomains(id)`, `domains/*`, `history`

### 3.3 Releases View (`/releases`) — Master-Detail

**Identical structure to Blocks** (§3.2), with these differences:
- No origin field (no `method_id`, no origin dropdown, no `Provenienza` column/row).
- Table header columns: `Data`, `Riferimento` only.
- Detail info section: only `Riferimento` and `Data` rows (no `Provenienza`).
- Create modal: only date, reference, and domains textarea (no origin select).
- **No virtual scrolling needed**: only 14 release requests. Use a regular mapped list.

**Components**:

| Component | File |
|-----------|------|
| `ReleasesPage` | `views/releases/ReleasesPage.tsx` |
| `ReleasesTable` | `views/releases/ReleasesTable.tsx` — regular list (14 rows, no virtual scroll) |
| `ReleaseDetail` | `views/releases/ReleaseDetail.tsx` — read-only, same pattern as `BlockDetail` minus origin |
| `ReleaseCreateModal` | `views/releases/ReleaseCreateModal.tsx` — date, reference, domains |
| `ReleaseEditModal` | `views/releases/ReleaseEditModal.tsx` — date, reference only |

**Cache invalidation**: create/update release → invalidate `releases`, `domains/*`, `history`.

### 3.4 Domain Status View (`/domains`) — Tabbed Read-Only List + Export

#### 3.4.1 `DomainsPage` — `views/domains/DomainsPage.tsx`

**Layout**:
1. **Toolbar**: page title `"Stato domini"`, subtitle `"Visualizza lo stato corrente di blocco/rilascio dei domini"`.
2. **Search + Export row** (below toolbar, above tabs): flex row with:
   - Search input (`<input type="text">` with `.input` style, placeholder `"Cerca dominio..."`, with a clear "✕" button visible when input is non-empty). Search state: `const [searchQuery, setSearchQuery] = useState('')`.
   - `<ExportButtons basePath="/compliance/domains" params={{ status: activeTab, search: searchQuery }} />` aligned to the right.
3. **Tab bar** (state-based, **NOT** `TabNav`):

   > **IMPORTANT**: Do NOT use `TabNav` from `@mrsmith/ui` here. `TabNav` is route-based (uses `NavLink` from react-router). These sub-tabs are state-based — they share `searchQuery` state that must persist across switches. Use a local tab bar with `useState`, following the exact pattern in `BudgetDetailPage.tsx:30-59` and `BudgetDetailPage.module.css:174-228`.

   Implementation:
   ```typescript
   const TABS = [
     { key: 'blocked', label: 'Bloccati' },
     { key: 'released', label: 'Rilasciati' },
   ] as const;
   type TabKey = (typeof TABS)[number]['key'];

   const [activeTab, setActiveTab] = useState<TabKey>('blocked');
   const tabRefs = useRef<Record<string, HTMLButtonElement | null>>({});
   const [indicatorStyle, setIndicatorStyle] = useState({ left: 0, width: 0 });

   useEffect(() => {
     const el = tabRefs.current[activeTab];
     if (el) setIndicatorStyle({ left: el.offsetLeft, width: el.offsetWidth });
   }, [activeTab]);
   ```

   CSS: copy `.tabBar`, `.tab`, `.tabActive`, `.tabIndicator`, `.tabContent` from `BudgetDetailPage.module.css:184-228`.

4. **Tab content**: `<DomainStatusTable domains={filteredDomains} />` where `filteredDomains` is computed via `useMemo`:
   ```typescript
   const filteredDomains = useMemo(() => {
     if (!domains) return [];
     const filtered = domains.filter(d =>
       activeTab === 'blocked' ? d.block_count > d.release_count : d.block_count <= d.release_count
     );
     if (!searchQuery) return filtered;
     const q = searchQuery.toLowerCase();
     return filtered.filter(d => d.domain.toLowerCase().includes(q));
   }, [domains, activeTab, searchQuery]);
   ```

**Three-state loading**: applied to the query that fetches all domain statuses. The tab content area shows `<Skeleton>` or service-unavailable or the table.

#### 3.4.2 `DomainStatusTable` — `views/domains/DomainStatusTable.tsx`

**Props**:
```typescript
interface DomainStatusTableProps {
  domains: DomainStatus[];
}
```

**Virtual scrolling**: YES — this table can have thousands of rows (up to ~10,300). Use `@tanstack/react-virtual` with `useVirtualizer`.

**Columns**: `Dominio`, `Blocchi`, `Rilasci` (three columns). Domain in monospace, counts as numeric badges (`.badge` style).

**Empty state**: `"Nessun dominio trovato"` when the filtered list is empty.

### 3.5 History View (`/history`) — Read-Only List + Export

#### 3.5.1 `HistoryPage` — `views/history/HistoryPage.tsx`

**Layout**:
1. **Toolbar**: title `"Riepilogo"`, subtitle `"Cronologia completa delle richieste di blocco e rilascio"`.
2. **Search + Export row**: same pattern as `DomainsPage`. Search input + `<ExportButtons basePath="/compliance/domains/history" params={{ search: searchQuery }} />`.
3. **Table card**: virtualized table (can have thousands of rows).

**Columns**: `Dominio`, `Data`, `Riferimento`, `Tipo`.

**`Tipo` column rendering** — must render Italian badges, NOT raw English strings:
- `request_type === 'block'` → badge with text `"Blocco"`, styled with `--color-danger` (red text, `rgba(239, 68, 68, 0.08)` background, same pattern as status badges in `CentriDiCostoPage`).
- `request_type === 'release'` → badge with text `"Rilascio"`, styled with `--color-success` (green text, `rgba(16, 185, 129, 0.08)` background).

Badge CSS:
```css
.typeBadge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 2px 10px;
  border-radius: 20px;
  font-size: 0.6875rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.typeBadgeBlock {
  color: var(--color-danger, #ef4444);
  background: rgba(239, 68, 68, 0.08);
}
.typeBadgeRelease {
  color: var(--color-success, #10b981);
  background: rgba(16, 185, 129, 0.08);
}
```

**Three-state loading**: same pattern as all other views.

**Empty state**: `"Nessun risultato"` / `"La cronologia e vuota"`.

### 3.6 Origins View (`/origins`) — CRUD

#### 3.6.1 `OriginsPage` — `views/origins/OriginsPage.tsx`

**Layout**:
1. **Toolbar**: title `"Provenienze"`, subtitle `"Gestisci le fonti delle richieste di blocco"`, primary button `"Nuova provenienza"` (opens `OriginCreateModal`).
2. **Table card**:
   - **Fetches with `include_inactive=true`** — the management page shows ALL origins.
   - Three-state loading (skeleton / service-unavailable / data).
   - Empty state: `"Nessuna provenienza"` / `"Crea la tua prima provenienza per iniziare"`.
   - Table header: `Codice`, `Descrizione`, `Stato`, (action column).
   - Rows: `method_id`, `description`, status badge.

**Status badge**:
- Active: green dot + `"Attivo"` (same style as `CentriDiCostoPage` status badges — `.statusBadge` + `.statusEnabled`).
- Inactive: gray dot + `"Disabilitato"` (`.statusBadge` + `.statusDisabled`).
- Inactive rows: add `.rowDisabled` class for slightly muted text (same as `CentriDiCostoPage.module.css` `.rowDisabled`).

**Row actions** (visible in the last column):
- "Modifica" icon button (pencil, 28×28, `.allocEditBtn` style) → opens `OriginEditModal`
- "Disabilita" icon button (circle-minus, 28×28, danger color on hover) → opens `DeactivateOriginConfirm`. **Only shown for active origins.**
- "Abilita" icon button (circle-check, 28×28, success color on hover) → directly calls `enableOrigin.mutate(method_id)` with pending state. **Only shown for inactive origins.** Same pattern as the "Abilita" button in `CentriDiCostoPage.tsx:263-273`.

**No virtual scrolling**: origin list is small (~4 rows currently, unlikely to exceed ~20).

#### 3.6.2 `OriginCreateModal` — `views/origins/OriginCreateModal.tsx`

**Pattern**: Same as `BudgetCreateModal`.

**Form fields**:
1. **Codice** — `<input type="text">` with `.input` style. Placeholder: `"Es. AGCOM"`. Required. This is the `method_id` (immutable PK).
2. **Descrizione** — `<input type="text">` with `.input` style. Placeholder: `"Descrizione della provenienza"`. Required.

**Submit**: `"Crea"` / `"Creazione..."`. On success: `toast('Provenienza creata')`, `onClose()`, invalidate origins.

#### 3.6.3 `OriginEditModal` — `views/origins/OriginEditModal.tsx`

**Form fields**: Only `Descrizione` (the `method_id` is immutable — show it as a read-only info row above the form, styled as `.ruleContext` with label `"Codice"` and value in bold).

**Submit**: `"Salva"` / `"Salvataggio..."`. On success: `toast('Provenienza aggiornata')`, `onClose()`, invalidate origins.

**Hooks**:
- `useOrigins(includeInactive?: boolean)` — management page passes `true`, creation dropdown omits it
- `useCreateOrigin()`, `useUpdateOrigin()`, `useDeleteOrigin()`
- `useEnableOrigin()` — for the inline enable button on inactive rows

### 3.7 Styles

**One CSS Module per view**: `BlocksPage.module.css`, `ReleasesPage.module.css`, `DomainsPage.module.css`, `HistoryPage.module.css`, `OriginsPage.module.css`.

**One CSS Module for shared components**: `components/Compliance.module.css` — styles for `DomainPreview`, `DomainList`, `DomainEditModal`, `ExportButtons`, `DeactivateOriginConfirm`, `AddDomainsModal`.

**CSS source of truth**: Copy button, form, empty-state, table, row, detail, and responsive styles from `GruppiPage.module.css`. That file contains every CSS pattern needed. Do not invent new patterns. Specifically:
- **Buttons**: `.btnPrimary`, `.btnSecondary`, `.btnDanger` — copy verbatim from `GruppiPage.module.css:389-483`.
- **Form elements**: `.formGroup`, `.label`, `.input`, `.input:focus`, `.input::placeholder` — copy from `GruppiPage.module.css:486-526` or `BudgetDetailPage.module.css:565-608`.
- **Empty states**: `.emptyState`, `.emptyIcon`, `.emptyTitle`, `.emptyText` — copy from `GruppiPage.module.css:349-381`.
- **Table**: `.tableCard`, `.tableHeader`, `.tableBody`, `.row`, `.rowSelected`, `.rowAccent`, `.rowChevron` — copy from `GruppiPage.module.css:44-201`.
- **Detail panel**: `.detail`, `.detailOpen`, `.detailContent`, `.detailHeader`, `.detailIconLg`, `.detailTitle`, `.detailMeta`, `.divider`, `.detailActions` — copy from `GruppiPage.module.css:204-346`.
- **Responsive**: `@media (max-width: 1000px)` block — copy from `GruppiPage.module.css:549-557`.

**Textarea style** (new, for AddDomainsModal and BlockCreateModal):
```css
.textarea {
  width: 100%;
  min-height: 120px;
  max-height: 240px;
  padding: 11px 14px;
  border: 1.5px solid var(--color-border, #e2e8f0);
  border-radius: var(--radius-md, 10px);
  font-size: 0.875rem;
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
  outline: none;
  background: var(--color-bg, #fafbfd);
  color: var(--color-text, #0f172a);
  resize: vertical;
  transition: all var(--duration-normal, 250ms) var(--ease-out);
}
.textarea:hover { border-color: var(--color-text-muted, #94a3b8); }
.textarea:focus { border-color: var(--color-accent, #635bff); background: var(--color-bg-elevated, #fff); box-shadow: 0 0 0 3px var(--color-accent-glow, rgba(99, 91, 255, 0.15)); }
.textarea::placeholder { color: var(--color-text-faint, #cbd5e1); }
```

**Tab overflow at narrow viewports**: test the 5 top-level tabs at 640px viewport width. If labels overflow, add:
```css
@media (max-width: 768px) {
  /* TabNav is used for top-level navigation — if labels truncate, abbreviate in code */
}
```
If needed, the `App.tsx` tab labels can be conditionally shortened: `"Stato domini"` → `"Stato"`, `"Provenienze"` → `"Orig."`. Implement this only if overflow is observed during Phase 4 testing.

### Phase 3 Deliverables

- [ ] 5 fully functional views with real API integration
- [ ] ~20 component files + CSS modules
- [ ] All CRUD operations working end-to-end via Modals
- [ ] Domain parse/preview with error highlighting in `AddDomainsModal`
- [ ] Domain edit via `DomainEditModal` (Modal, not popover)
- [ ] Deactivate origin confirmation via `DeactivateOriginConfirm`
- [ ] Virtual scrolling on `BlocksTable`, `DomainStatusTable`, `HistoryPage`
- [ ] CSS-scroll `DomainList` in detail panels (`max-height: 320px`)
- [ ] Export (CSV/XLSX) via authenticated blob download with loading/error states
- [ ] Three-state loading pattern on all 5 views
- [ ] Empty states on all lists and detail panels
- [ ] Italian labels throughout (no raw English API values)
- [ ] Local state-based tab bar on DomainsPage (not `TabNav`)
- [ ] Responsive layout (1000px breakpoint) on master-detail pages

---

## Phase 4: Integration, Testing & Polish

**Goal**: Production-ready. All pieces connected, tested, verified.

### 4.1 Frontend Type-Check & Lint

```bash
pnpm --filter mrsmith-compliance exec tsc --noEmit
pnpm --filter mrsmith-compliance lint
```

### 4.2 Backend Tests

```bash
cd backend && go test ./internal/compliance/...
cd backend && go test ./internal/platform/applaunch/...
cd backend && go test ./internal/portal/...
```

### 4.3 Static SPA Deep-Link Test

**Modify** `backend/internal/platform/staticspa/handler_test.go`:
- Add compliance path to `buildStaticFixture()`: `writeFixtureFile(t, filepath.Join(root, "apps", "compliance", "index.html"), "<html>compliance-shell</html>")`
- **Add** `TestHandlerFallsBackToComplianceIndexForDeepLinks`: request `/apps/compliance/domains/123` → verify response contains `compliance-shell`
- This locks down the deep-link fallback behavior that motivated the FB1 path correction

### 4.4 E2E Smoke Tests

| Test | Verification |
|------|-------------|
| Auth flow | Login via Keycloak → token in header → API returns data |
| Role enforcement | User without `app_compliance_access` gets 403 on all endpoints |
| **Deep link refresh** | Browser refresh on `/apps/compliance/domains` serves compliance SPA (verifies staticspa fallback) |
| Block create | Open modal → fill form → submit → toast `"Richiesta di blocco creata"` → appears in list → select → domains visible in detail |
| Release create | Same flow, no origin |
| Add domains | Select existing request → "Aggiungi domini" → modal → paste domains → preview shows valid/invalid → submit → domains appended |
| Edit request | Select request → "Modifica" in detail → **Modal opens** → change fields → "Salva" → toast → updated |
| Edit domain | Select request → click edit icon on domain row → **DomainEditModal opens** → change FQDN → validation indicator → "Salva" → updated in list |
| **Deactivate origin** | Go to Provenienze → click deactivate icon → **confirmation modal** → confirm → toast → status badge changes to "Disabilitato" |
| Enable origin | Click enable button on disabled origin → pending state on button → toast → status badge changes to "Attivo" |
| **Ownership guard** | PUT block domain with wrong block_id → 404 |
| Domain status | Bloccati/Rilasciati sub-tabs show correct counts per BR1. **Search persists across tab switches.** |
| **Export (authenticated)** | CSV/XLSX buttons show "Esportazione..." during download → file downloads → button resets. Error case: disconnect network → export → error toast. |
| Origins CRUD | Create with method_id + description → visible in origins table and in block creation dropdown. Edit description. Deactivate → hidden from dropdown, visible in origins management page with "Disabilitato" badge. |
| **Search persistence** | Type in search on Bloccati tab → switch to Rilasciati → search still active → clear "✕" → search cleared |
| **Empty states** | Verify all empty states render correctly: no blocks, no releases, no domains, no origins, no detail selected |
| **Three-state loading** | Disconnect backend → reload → service-unavailable message on all views (not blank/error) |
| **Appsmith coexistence** | Legacy app still reads/writes same tables after `is_active` column addition |
| **Middleware auth** | Unauthenticated request to `/api/compliance/blocks` returns 401 plain text (not JSON) — validates that shared auth middleware gates all compliance endpoints |
| **Tab overflow** | At 640px viewport width, all 5 top-level tabs are visible and usable |

### 4.5 Server-Level Auth Test

**Add** a higher-level integration test in `backend/cmd/server/` or a dedicated test file:
- Build the full mux (including auth middleware + compliance routes)
- Verify unauthenticated `GET /api/compliance/domains?format=csv` → 401
- This confirms Bearer token enforcement through the real middleware stack, not just the compliance ACL layer

### 4.6 Keycloak Role Setup

- Create `app_compliance_access` role in Keycloak realm
- Assign to appropriate users/groups

### 4.7 Resolve Open Questions

| # | Question | Action |
|---|----------|--------|
| O4 | Canonical FQDN regex | Finalize regex, test identical edge-case suite in both Go and TypeScript |
| O6 | XLSX library | Use `excelize` (standard Go XLSX library) |

### Phase 4 Deliverables

- [ ] Type-check clean
- [ ] Backend tests passing (including ownership, transaction, export, delete-origin-200 tests)
- [ ] Catalog + portal tests updated and passing
- [ ] Static SPA deep-link test added and passing
- [ ] Server-level auth integration test passing
- [ ] All smoke tests passing (including UI/UX-specific tests)
- [ ] Keycloak role created
- [ ] O4 and O6 resolved and documented

---

## File Inventory

### New Files (~32)

```
backend/internal/compliance/
├── handler.go               — Handler struct, RegisterRoutes, shared helpers
├── handler_blocks.go        — Block request + domain handlers
├── handler_releases.go      — Release request + domain handlers
├── handler_domains.go       — Domain status + history handlers (+ export dispatch)
├── handler_origins.go       — Origins CRUD handlers
├── export.go                — CSV/XLSX generation helpers
├── models.go                — Go structs (request/response types)
├── validation.go            — FQDN validation (canonical regex)
├── validation_test.go       — Validation tests
└── handler_test.go          — Handler tests (ownership, transactions, export, delete-origin)

apps/compliance/
├── package.json
├── vite.config.ts
├── tsconfig.json
├── index.html
├── migrations/
│   └── 001_add_is_active.sql
└── src/
    ├── main.tsx
    ├── App.tsx
    ├── routes.tsx
    ├── api/
    │   ├── client.ts
    │   ├── types.ts
    │   ├── errors.ts          — isUpstreamAuthFailed + getApiErrorMessage (copied from budget)
    │   └── queries.ts
    ├── utils/
    │   └── fqdn.ts
    ├── hooks/
    │   └── useOptionalAuth.ts
    ├── components/
    │   ├── AddDomainsModal.tsx
    │   ├── DomainList.tsx
    │   ├── DomainEditModal.tsx        — was DomainEditPopover in V1.2, now uses Modal
    │   ├── DomainPreview.tsx
    │   ├── ExportButtons.tsx
    │   ├── DeactivateOriginConfirm.tsx — confirmation modal for origin soft-delete
    │   └── Compliance.module.css       — shared component styles
    ├── views/
    │   ├── blocks/
    │   │   ├── BlocksPage.tsx
    │   │   ├── BlocksPage.module.css
    │   │   ├── BlocksTable.tsx
    │   │   ├── BlockDetail.tsx
    │   │   ├── BlockCreateModal.tsx
    │   │   └── BlockEditModal.tsx
    │   ├── releases/
    │   │   ├── ReleasesPage.tsx
    │   │   ├── ReleasesPage.module.css
    │   │   ├── ReleasesTable.tsx
    │   │   ├── ReleaseDetail.tsx
    │   │   ├── ReleaseCreateModal.tsx
    │   │   └── ReleaseEditModal.tsx
    │   ├── domains/
    │   │   ├── DomainsPage.tsx
    │   │   ├── DomainsPage.module.css
    │   │   └── DomainStatusTable.tsx
    │   ├── history/
    │   │   ├── HistoryPage.tsx
    │   │   └── HistoryPage.module.css
    │   └── origins/
    │       ├── OriginsPage.tsx
    │       ├── OriginsPage.module.css
    │       ├── OriginCreateModal.tsx
    │       └── OriginEditModal.tsx
    └── styles/
        └── global.css
```

### Modified Files (12)

```
backend/cmd/server/main.go                          — Register compliance, DB init, href override, pgx/stdlib import
backend/internal/platform/applaunch/catalog.go       — Roles, constants, href, catalog entry
backend/internal/platform/applaunch/catalog_test.go  — Updated counts, new compliance role tests
backend/internal/portal/handler_test.go              — New compliance role visibility tests
backend/internal/platform/config/config.go           — AnisettaDSN, ComplianceAppURL, CORS origins
backend/internal/platform/staticspa/handler_test.go  — Compliance deep-link fallback test
backend/go.mod                                       — pgx/v5 + excelize dependencies
packages/api-client/src/client.ts                    — Add getBlob() to ApiClient interface
package.json                                         — Dev script includes compliance
docker-compose.dev.yaml                              — Compliance frontend service
deploy/Dockerfile                                    — Copy compliance dist to /static/apps/compliance
deploy/k8s/deployment.yaml                           — ANISETTA_DSN from Secret
```

---

## Dependency Graph

```
Phase 0 (Spec reconciliation) — before everything

Phase 1.1 (DB config + pgx dep) + 1.2 (Roles + test updates)
  └──► 1.3 (Route registration + handler struct)
         └──► 1.4 (Models/Validation)
                └──► 1.5, 1.6, 1.7, 1.8, 1.9 (All handlers — parallelizable)
                       └──► 1.10 (Tests)

Phase 2.1 (Project setup) + 2.2 (Build integration + docker-compose + K8s)
  └──► 2.3 (Bootstrap/layout)
         └──► 2.4 (API layer + errors.ts) + 2.5 (getBlob) + 2.6 (Utils) — parallelizable
                └──► 2.7 (Placeholder views)

Phase 3.0 (Error utilities — already done in 2.4)
  └──► 3.1 (Shared components: AddDomainsModal, DomainPreview, DomainList, DomainEditModal, ExportButtons, DeactivateOriginConfirm)
         └──► 3.2 (Blocks) + 3.3 (Releases) — parallelizable
              3.4 (Domains) + 3.5 (History) + 3.6 (Origins) — parallelizable after 3.1

Phase 4 depends on: all above
  4.1–4.2 (Type-check + backend tests)
  4.3 (Static SPA deep-link test)
  4.4 (E2E smoke tests — now includes UI/UX-specific tests)
  4.5 (Server-level auth test)
  4.6 (Keycloak role setup)
```

---

## Risk Register

| Risk | Mitigation |
|------|-----------|
| PostgreSQL connection is a new backend pattern (budget uses Arak proxy) | Reuse existing `platform/database` package with `pgx` driver. Explicit `pgx/v5/stdlib` import in `main.go`. Handler struct with injected `*sql.DB` for testability. |
| Schema retrocompatibility — `is_active` column addition | Column uses `DEFAULT TRUE` and `IF NOT EXISTS`. No impact on existing queries. Validate with Appsmith before deploying. |
| Large dataset performance (10K+ block domains) | Virtual scrolling (`@tanstack/react-virtual`) on `BlocksTable` (1,255 rows), `DomainStatusTable` (10K+), `HistoryPage` (10K+). CSS scroll with `max-height` on `DomainList` in detail panels (~8 avg, ~100 max per request). `ReleasesTable` (14 rows) and `OriginsPage` (~4 rows) use regular lists. |
| FQDN regex divergence between frontend and backend | Canonical regex documented once, tested with identical edge-case suite in both Go and TypeScript. |
| Coexistence with Appsmith | Read-only validation: run Appsmith against same DB after migration. No schema-breaking changes by design. |
| Export auth failure in production | Authenticated blob download via `getBlob()` — tested in smoke tests with real JWT + server-level auth integration test. Export buttons show loading/error states. |
| Cross-parent domain updates | Ownership check (`WHERE id = $1 AND parent_id = $2`) on all nested resource updates. Tested explicitly. |
| Shared ApiClient assumes JSON responses | `DELETE /origins` returns `200` with JSON body. No `204 No Content` endpoints in compliance API. |
| Auth error body format mismatch | Documented: middleware 401/403 = plain text, handler errors = JSON. Frontend error handling does not assume JSON for auth failures. |
| Catalog test breakage from role migration | Tests updated in Phase 1.2 alongside the catalog change. |
| UI/UX inconsistency with budget app | All interactions use the same patterns: `Modal` for edits, confirmation `Modal` for destructive actions, three-state loading, standard empty states, copied CSS from `GruppiPage.module.css`. |
| Tab overflow on narrow viewports | 5 tabs tested at 640px. Labels can be abbreviated via CSS media query if needed. |
| Touch device accessibility for domain edit | Edit button is always visible (not hover-only), ensuring touch devices can trigger domain editing. |
| Empty origins blocking block creation | `BlockCreateModal` shows contextual error message and disables submit when no active origins exist, guiding user to the Provenienze page. |

---

## Verification Checklist

1. `pnpm install` — workspace resolves
2. `pnpm dev` — all 4 processes start (backend, portal, budget, compliance)
3. `make dev-docker` — all 4 services start via docker-compose
4. Open `http://localhost:5175/blocks` — compliance app loads with 5 tabs
5. `pnpm --filter mrsmith-compliance exec tsc --noEmit` — clean
6. `cd backend && go test ./internal/compliance/...` — all pass
7. `cd backend && go test ./internal/platform/applaunch/...` — all pass (updated counts)
8. `cd backend && go test ./internal/portal/...` — all pass (new role tests)
9. `cd backend && go test ./internal/platform/staticspa/...` — all pass (compliance deep-link)
10. `cd backend && go build ./cmd/server` — compiles (pgx import resolves)
11. Manual: create block request with domains → verify in domain status view → export CSV via authenticated download
12. Manual: refresh browser on `/apps/compliance/domains` → SPA loads correctly (deep link test)
13. Manual: delete origin → verify 200 JSON response, origin hidden from dropdowns but visible in management page
14. Manual: verify all empty states render (no blocks, no detail selected, no origins, service unavailable)
15. Manual: verify domain edit opens a Modal (not a popover), request edit opens a Modal (not inline form)
16. Manual: verify origin deactivation shows confirmation Modal before proceeding
17. Manual: verify domain status sub-tabs share search query across Bloccati/Rilasciati switches
18. Manual: verify export buttons show "Esportazione..." loading state during download
