# Compliance App — Implementation Plan V1.1

Source: `apps/compliance/compliance-migspec.md`
Reference app: `apps/budget/`
Revision: incorporates all findings from `COMP-IMPL-V1-FB.md`

---

## Resolved Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| FB1 | App path is `/apps/compliance/` (not `/apps/smart-apps/compliance/`) | `staticspa` fallback resolves `/apps/<segment[1]>/index.html` — nested paths would 404 on deep links |
| FB3 | Export via authenticated blob download, not `window.open()` | All endpoints require Bearer JWT; browser navigation cannot attach auth headers |
| FB4 | Origin creation requires explicit `{method_id, description}` | Existing PKs are human-chosen codes (AGCOM, GDF, MININT, POLPOST); auto-generation would be fragile |
| FB5 | Origins management page fetches with `include_inactive=true` | Page must show deactivated origins with status badges; active-only default is for creation dropdowns |
| FB6 | Reuse `backend/internal/platform/database/database.go` with `pgx` driver; handler struct with injected `*sql.DB`; env var `ANISETTA_DSN` | Avoids competing DB patterns; enables testability; single env var name across code/deploy/docs |

---

## Overview

4 phases, ordered by dependency. Phases 1–2 run in parallel. Phases 3–4 are sequential.

```
Phase 1: Backend (Go)          ──┐
                                  ├──► Phase 3: Frontend Views + Export ──► Phase 4: Integration & Polish
Phase 2: Frontend Scaffold      ──┘
```

Estimated file count: ~30 new files (backend: ~10, frontend: ~18, config/build: ~2), ~7 modified files.

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
- If `cfg.AnisettaDSN != ""`, call `database.New(database.Config{Driver: "postgres", DSN: cfg.AnisettaDSN})` to get `*sql.DB`
- Call `compliance.RegisterRoutes(api, db)` (db may be nil — handlers return 503)
- Add compliance href override logic (same pattern as budget, lines 68-74)

**Reuse** `backend/internal/platform/database/database.go` — already supports postgres via `pgx` driver.

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

**Add** `excelize` to `backend/go.mod`.

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
- `DELETE /origins/{id}` — soft delete: `UPDATE dns_bl_method SET is_active = false WHERE method_id = $1`.

### 1.10 Tests

**Create** `backend/internal/compliance/validation_test.go`:
- FQDN edge cases: valid FQDNs, invalid (IPs, wildcards, empty, unicode, trailing dots)

**Create** `backend/internal/compliance/handler_test.go`:
- Handler tests via `httptest` with test DB or mock
- **Ownership validation**: attempt to update domain with wrong parent ID → 404
- **Transaction rollback**: create block with mix of valid/invalid domains → 400, verify no partial insert
- **Export auth**: verify export endpoints require Bearer token (not accessible via unauthenticated GET)
- **Filter parity**: verify export with `?search=` matches expected rows

### Phase 1 Deliverables

- [ ] `backend/internal/compliance/` — 10 Go files
- [ ] `backend/internal/platform/applaunch/catalog.go` — role + catalog update
- [ ] `backend/internal/platform/config/config.go` — `AnisettaDSN`, `ComplianceAppURL`, CORS
- [ ] `backend/cmd/server/main.go` — compliance module registration + DB init + href override
- [ ] `backend/go.mod` — `excelize` dependency
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
  "type": "module",
  "scripts": { "dev": "vite", "build": "tsc && vite build" },
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

**Create** `apps/compliance/src/api/types.ts`:
```typescript
interface BlockRequest { id: number; request_date: string; reference: string; method_id: string; method_description: string; }
interface BlockDomain { id: number; domain: string; }
interface ReleaseRequest { id: number; request_date: string; reference: string; }
interface ReleaseDomain { id: number; domain: string; }
interface Origin { method_id: string; description: string; is_active: boolean; }
interface DomainStatus { domain: string; block_count: number; release_count: number; }
interface HistoryEntry { domain: string; request_date: string; reference: string; request_type: 'block' | 'release'; }
interface ValidationErrorResponse { error: 'invalid_domains'; message: string; invalid: string[]; }
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

- [ ] `apps/compliance/` — ~12 files (config + src scaffold)
- [ ] Root `package.json` dev script updated
- [ ] `packages/api-client/src/client.ts` — `getBlob()` added
- [ ] `deploy/Dockerfile` updated
- [ ] Config: CORS origins, compliance href override
- [ ] App boots on port 5175, tabs navigate, auth works, API client ready

---

## Phase 3: Frontend Views + Export (depends on Phase 1 + 2)

**Goal**: All 5 views fully functional with real API integration, including export.

### 3.1 Shared Components (build first, reused across views)

| Component | File | Description |
|-----------|------|-------------|
| `AddDomainsModal` | `components/AddDomainsModal.tsx` | Domains textarea → parse → preview → submit. Used by blocks + releases. |
| `DomainPreview` | `components/DomainPreview.tsx` | Parsed domain list with per-line red highlighting for invalid entries. |
| `DomainList` | `components/DomainList.tsx` | Domain list with hover edit icon → triggers `DomainEditPopover`. |
| `DomainEditPopover` | `components/DomainEditPopover.tsx` | Mini-popover: FQDN field + Save/Cancel. Row-level success/error indicator. |
| `ExportButtons` | `components/ExportButtons.tsx` | CSV + XLSX buttons. Uses `getBlob()` from api-client → `URL.createObjectURL()` → triggers download. Receives current search/status filter as props to build the query string. |

### 3.2 Blocks View (`/blocks`) — Master-Detail + Modals

| Component | File |
|-----------|------|
| `BlocksPage` | `views/blocks/BlocksPage.tsx` — two-panel layout (master table + detail) |
| `BlocksTable` | `views/blocks/BlocksTable.tsx` — date, origin, reference columns. Row selection. |
| `BlockDetail` | `views/blocks/BlockDetail.tsx` — read mode (header + DomainList) / edit mode (form + "Salva"). Toggle via "Modifica". |
| `BlockCreateModal` | `views/blocks/BlockCreateModal.tsx` — date (default today), reference, origin dropdown (default AGCOM, populated from active origins via `GET /origins`), domains textarea with DomainPreview. |

**Hooks** (in `api/queries.ts`):
- `useBlocks()`, `useBlock(id)`, `useBlockDomains(id)`
- `useCreateBlock()`, `useUpdateBlock()`, `useAddBlockDomains()`, `useUpdateBlockDomain()`

**Cache invalidation**:
- Create/update block → invalidate `blocks`, `domains/*`, `history`
- Add/update domains → invalidate `blockDomains(id)`, `domains/*`, `history`

### 3.3 Releases View (`/releases`) — Master-Detail + Modals

Same master-detail pattern as blocks, minus origin field. Reuses `AddDomainsModal`, `DomainList`, `DomainEditPopover`, `DomainPreview`.

| Component | File |
|-----------|------|
| `ReleasesPage` | `views/releases/ReleasesPage.tsx` |
| `ReleasesTable` | `views/releases/ReleasesTable.tsx` — date, reference (no origin). |
| `ReleaseDetail` | `views/releases/ReleaseDetail.tsx` |
| `ReleaseCreateModal` | `views/releases/ReleaseCreateModal.tsx` — date, reference, domains textarea. No origin. |

**Cache invalidation**: same pattern — invalidate `releases`, `domains/*`, `history`.

### 3.4 Domain Status View (`/domains`) — Tabbed Read-Only List + Export

| Component | File |
|-----------|------|
| `DomainsPage` | `views/domains/DomainsPage.tsx` — sub-tabs Bloccati (default) / Rilasciati. **Shared `searchQuery` state lifted above tabs** — persists across tab switches. Search input with clear "X" button. `ExportButtons` with current status + search params. |
| `DomainStatusTable` | `views/domains/DomainStatusTable.tsx` — virtualized (`@tanstack/react-virtual`): domain, block_count, release_count. Client-side filter via `useMemo` on search query. |

### 3.5 History View (`/history`) — Read-Only List + Export

| Component | File |
|-----------|------|
| `HistoryPage` | `views/history/HistoryPage.tsx` — virtualized table: domain, request_date, reference, request_type. Search input. `ExportButtons` with search param. |

### 3.6 Origins View (`/origins`) — CRUD

| Component | File |
|-----------|------|
| `OriginsPage` | `views/origins/OriginsPage.tsx` — table: method_id, description, status badge (active/inactive). **Fetches with `include_inactive=true`**. Create, edit, deactivate actions. |
| `OriginCreateModal` | `views/origins/OriginCreateModal.tsx` — form: **method_id (code) + description**. |
| `OriginEditModal` | `views/origins/OriginEditModal.tsx` — form: description only (method_id is immutable PK). |

**Hooks**:
- `useOrigins(includeInactive?: boolean)` — management page passes `true`, creation dropdown omits it
- `useCreateOrigin()`, `useUpdateOrigin()`, `useDeleteOrigin()`

### 3.7 Styles

- CSS Modules per view: `BlocksPage.module.css`, etc.
- Follow budget app's visual patterns for consistency.
- Master-detail layout: two-column flex/grid pattern.

### Phase 3 Deliverables

- [ ] 5 fully functional views with real API integration
- [ ] ~18 component files + CSS modules
- [ ] All CRUD operations working end-to-end
- [ ] Domain parse/preview with error highlighting
- [ ] Domain inline edit with popover
- [ ] Virtual scrolling on large lists
- [ ] Export (CSV/XLSX) via authenticated blob download on domains + history views

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
```

### 4.3 E2E Smoke Tests

| Test | Verification |
|------|-------------|
| Auth flow | Login via Keycloak → token in header → API returns data |
| Role enforcement | User without `app_compliance_access` gets 403 on all endpoints |
| **Deep link refresh** | Browser refresh on `/apps/compliance/domains` serves compliance SPA (verifies staticspa fallback) |
| Block create | Fill form → submit → appears in list → domains visible in detail |
| Release create | Same flow, no origin |
| Add domains | Select existing request → add modal → domains appended |
| Edit request | Toggle edit mode → change fields → save → updated |
| Edit domain | Hover edit icon → popover → change → save → updated in list |
| **Ownership guard** | PUT block domain with wrong block_id → 404 |
| Domain status | Blocked/released tabs show correct counts per BR1 |
| **Export (authenticated)** | CSV/XLSX download via blob fetch with Bearer token; content matches visible filtered rows |
| Origins | Create with method_id + description → visible in dropdown. Deactivate → hidden from dropdown, visible in management page and history. |
| Search persistence | Type in search on Bloccati tab → switch to Rilasciati → search still active |
| **Appsmith coexistence** | Legacy app still reads/writes same tables after `is_active` column addition |

### 4.4 Keycloak Role Setup

- Create `app_compliance_access` role in Keycloak realm
- Assign to appropriate users/groups

### 4.5 Resolve Open Questions

| # | Question | Action |
|---|----------|--------|
| O4 | Canonical FQDN regex | Finalize regex, test identical edge-case suite in both Go and TypeScript |
| O6 | XLSX library | Use `excelize` (standard Go XLSX library) |

### Phase 4 Deliverables

- [ ] Type-check clean
- [ ] Backend tests passing (including ownership, transaction, export tests)
- [ ] All smoke tests passing
- [ ] Keycloak role created
- [ ] O4 and O6 resolved and documented

---

## File Inventory

### New Files (~30)

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
└── handler_test.go          — Handler tests (ownership, transactions, export)

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
    │   └── queries.ts
    ├── utils/
    │   └── fqdn.ts
    ├── hooks/
    │   └── useOptionalAuth.ts
    ├── components/
    │   ├── AddDomainsModal.tsx
    │   ├── DomainList.tsx
    │   ├── DomainEditPopover.tsx
    │   ├── DomainPreview.tsx
    │   └── ExportButtons.tsx
    ├── views/
    │   ├── blocks/
    │   │   ├── BlocksPage.tsx
    │   │   ├── BlocksPage.module.css
    │   │   ├── BlocksTable.tsx
    │   │   ├── BlockDetail.tsx
    │   │   └── BlockCreateModal.tsx
    │   ├── releases/
    │   │   ├── ReleasesPage.tsx
    │   │   ├── ReleasesPage.module.css
    │   │   ├── ReleasesTable.tsx
    │   │   ├── ReleaseDetail.tsx
    │   │   └── ReleaseCreateModal.tsx
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

### Modified Files (7)

```
backend/cmd/server/main.go                          — Register compliance, DB init, href override
backend/internal/platform/applaunch/catalog.go       — Roles, constants, href, catalog entry
backend/internal/platform/config/config.go           — AnisettaDSN, ComplianceAppURL, CORS origins
backend/go.mod                                       — excelize dependency
packages/api-client/src/client.ts                    — Add getBlob() to ApiClient interface
package.json                                         — Dev script includes compliance
deploy/Dockerfile                                    — Copy compliance dist to /static/apps/compliance
```

---

## Dependency Graph

```
Phase 1.1 (DB config) + 1.2 (Roles)
  └──► 1.3 (Route registration + handler struct)
         └──► 1.4 (Models/Validation)
                └──► 1.5, 1.6, 1.7, 1.8, 1.9 (All handlers — parallelizable)
                       └──► 1.10 (Tests)

Phase 2.1 (Project setup) + 2.2 (Build integration)
  └──► 2.3 (Bootstrap/layout)
         └──► 2.4 (API layer) + 2.5 (getBlob) + 2.6 (Utils) — parallelizable
                └──► 2.7 (Placeholder views)

Phase 3.1 (Shared components)
  └──► 3.2 (Blocks) + 3.3 (Releases) — parallelizable
       3.4 (Domains) + 3.5 (History) + 3.6 (Origins) — parallelizable after 3.1

Phase 4 depends on: all above
```

---

## Risk Register

| Risk | Mitigation |
|------|-----------|
| PostgreSQL connection is a new backend pattern (budget uses Arak proxy) | Reuse existing `platform/database` package with `pgx` driver. Handler struct with injected `*sql.DB` for testability. No new patterns introduced. |
| Schema retrocompatibility — `is_active` column addition | Column uses `DEFAULT TRUE` and `IF NOT EXISTS`. No impact on existing queries. Validate with Appsmith before deploying. |
| Large dataset performance (10K+ block domains) | Virtual scrolling (`@tanstack/react-virtual`) handles rendering. Full dataset from server. Revisit with server-side pagination if rows exceed ~100K. |
| FQDN regex divergence between frontend and backend | Canonical regex documented once, tested with identical edge-case suite in both Go and TypeScript. |
| Coexistence with Appsmith | Read-only validation: run Appsmith against same DB after migration. No schema-breaking changes by design. |
| Export auth failure in production | Authenticated blob download via `getBlob()` — tested in smoke tests with real JWT. |
| Cross-parent domain updates | Ownership check (`WHERE id = $1 AND parent_id = $2`) on all nested resource updates. Tested explicitly. |

---

## Verification Checklist

1. `pnpm install` — workspace resolves
2. `pnpm dev` — all 4 processes start (backend, portal, budget, compliance)
3. Open `http://localhost:5175/blocks` — compliance app loads with 5 tabs
4. `pnpm --filter mrsmith-compliance exec tsc --noEmit` — clean
5. `cd backend && go test ./internal/compliance/...` — all pass
6. `cd backend && go build ./cmd/server` — compiles
7. Manual: create block request with domains → verify in domain status view → export CSV via authenticated download
8. Manual: refresh browser on `/apps/compliance/domains` → SPA loads correctly (deep link test)
