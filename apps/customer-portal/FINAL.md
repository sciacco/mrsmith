# `cp-backoffice` — Final Implementation Plan

Source of truth: `apps/customer-portal/PROMPT.md` and `apps/customer-portal/SPEC.md`. This document is the execution contract. All locked copy, labels, routes, APIs, flags, SQL objects, exceptions, and repo-fit decisions from the source are carried through verbatim. Where a derived slice narrows a source slice, the mapping back is stated explicitly.

## Context

Why this change: port the back-office surface of the legacy Appsmith Customer Portal into a standard MrSmith mini-app, removing dead features, hidden Appsmith switches, and direct browser access to `gw-int.cdlan.net` and Mistra PostgreSQL. The outcome is a production-ready `cp-backoffice` SPA with three registry-style routes (`Stato Aziende`, `Gestione Utenti`, `Accessi Biometrico`), a dedicated `/api/cp-backoffice/v1/` backend module, launcher wiring, deployment/static hosting updates, and parity-preserving operator workflows — all inside the approved clean mini-app family (`master_detail_crud` archetype).

This plan is explicitly a draft for pre-gate review. It assumes no external deadline and optimizes for clear sequencing, parallelism where safe, and pre-gate readiness.

---

## 1. Orchestration Protocol

### 1.1 Orchestrator Responsibilities And Limits
- Runs execution but **must not implement any slice**.
- Spawns exactly one executor sub-agent per active slice and at most one QA sub-agent per executor output.
- Maintains authoritative slice state (pending / in-progress / executor-done / qa-passed / qa-failed / complete) and the dependency gate.
- Refuses to promote a slice past QA without an explicit QA-pass artifact naming the specific locked items verified.
- Spawns the Final E2E QA Agent only after every slice is qa-passed and documentation is updated.
- Never softens `must`, `do not`, `preserve`, `keep`, `remove`, `leave untouched`, `exact`, or `not required` when restating source requirements to sub-agents.

### 1.2 QA Workflow Per Slice
1. Executor produces the slice artifacts against Acceptance Criteria, Test Plan, Docs Update, and locked constraints.
2. QA sub-agent re-reads `PROMPT.md` (not just the slice brief) and verifies:
   - Exact copy / label preservation and absence of forbidden terms (`server-side`, `datasource`, `widget`, `record`, `id.asc`, `Arak`, `Mistra`, `Keycloak`, `replica dell'app originale`).
   - Archetype is `master_detail_crud`, no KPI/stat rows, no launcher hero banners, no Appsmith clone chrome.
   - Repo-fit: paths, role id `app_cpbackoffice_access`, app id `cp-backoffice`, routes `/stato-aziende|/gestione-utenti|/accessi-biometrico`, API prefix `/api/cp-backoffice/v1/`, base path `/apps/cp-backoffice/`, Vite port `5187`, env vars (`MISTRA_DSN`, `ARAK_BASE_URL`, `ARAK_SERVICE_CLIENT_ID`, `ARAK_SERVICE_CLIENT_SECRET`, `ARAK_SERVICE_TOKEN_URL`, `CP_BACKOFFICE_APP_URL`), config field `CPBackofficeAppURL`, helper names `requireArak`, `requireMistra`, `dbFailure`, static path `/static/apps/cp-backoffice`.
   - Contract locks: `disable_pagination=true`, user-fetch guard on empty `customer_id`, `skip_keycloak: false` pinned, biometric DTO keys/types, `ORDER BY data_richiesta DESC`, stored function `customers.biometric_request_set_completed($1::bigint, $2::boolean)` returning `{ ok: true }`, `is_biometric_lenel` returned but not rendered.
   - Exceptions preserved with user-benefit rationale (top nav vs. source sidebar; inline Save/Discard in `Accessi Biometrico`; lowercase biometric labels in v1).
   - Required tests and manual review artifacts delivered.
3. QA returns PASS or FAIL with a pointer-list of locked items verified.
4. On FAIL the Orchestrator re-dispatches to the executor with QA findings. No slice may skip QA.

### 1.3 Iteration Rule On QA Failures
- Iterate until QA passes. No slice advances with an open finding.
- A finding tied to a locked item is never waived; it is fixed.
- A finding about a non-locked quality issue is either fixed or explicitly deferred to `docs/TODO.md` with rationale.

### 1.4 Final E2E QA
- Spawned after every slice is qa-passed and all docs are updated.
- Re-verifies every locked item at the composed-app level, runs the full test suite, walks the manual review artifacts, and produces the signoff report.
- Confirms rationale survived: comparable-app anchors, archetype fit, exceptions’ user-benefit rationale, v1 tradeoffs with TODO pointers, pre-gate framing.

---

## 2. Sub-Agent Roster

| Role | Purpose | Tools |
| --- | --- | --- |
| Orchestrator (`feature-dev:code-architect`, supervisor prompt) | Slice dispatch, gating, final-QA trigger. Does not edit code. | Read-only + Agent spawn |
| FE Scaffold Executor (`feature-dev:code-architect`) | Slice S1. Vite+React mini-app shell, routes, navigation. | Full file tools |
| BE Module Executor (`feature-dev:code-architect`) | Slices S2–S4. Go package `backend/internal/cpbackoffice/` incl. tests. | Full file tools + Bash for Go tests |
| FE Feature Executor (`feature-dev:code-architect`) | Slices S5a/S5b/S5c. Route UX, API hooks, modals. | Full file tools |
| Repo-Wiring Executor (`feature-dev:code-architect`) | Slices S0, S6. Root dev wiring, CORS, catalog, config, Dockerfile, server mount, docs. | Full file tools |
| Slice QA Agent (`feature-dev:code-reviewer` + `review-strict`) | Per-slice QA against locked items. | Read-only |
| Final E2E QA Agent (`feature-dev:code-reviewer` + `review-strict` + `ui-ux-reviewer`) | End-to-end signoff. | Read-only + Bash for test runs |

---

## 3. Slice Map (Source → Derived)

Source slices from `PROMPT.md` §“Implementation Slices”:
- Slice 1 — App Scaffolding And Shell
- Slice 2 — Backend Package And Contract Boundaries
- Slice 3 — Mistra NG Proxy Flows
- Slice 4 — Biometric Request DB Flows
- Slice 5a — Stato Aziende
- Slice 5b — Gestione Utenti
- Slice 5c — Accessi Biometrico

Derived executable slices (splits only where they improve ownership/parallelism/risk):

| Derived # | Source Parent | Reason For Split |
| --- | --- | --- |
| S0 — Pre-Code Verifications & Repo Wiring | supports all | Isolates infra (root `package.json`, `Makefile`, `applaunch/catalog.go`, `main.go`, `config.go`, `deploy/Dockerfile`, CORS) so UI and BE work can run in parallel without editing the same wiring files. |
| S1 — FE Scaffold & Shell | Slice 1 | 1:1 |
| S2 — BE Package & Contract Boundaries | Slice 2 | 1:1 |
| S3 — Mistra NG Proxy Flows | Slice 3 | 1:1 |
| S4 — Biometric DB Flows | Slice 4 | 1:1 |
| S5a — Stato Aziende | Slice 5a | 1:1 |
| S5b — Gestione Utenti | Slice 5b | 1:1 |
| S5c — Accessi Biometrico | Slice 5c | 1:1 |
| S6 — Docs & TODO Reconciliation | cross-cutting | Collects `docs/TODO.md` follow-ups (biometric labels, `skip_keycloak` re-enablement, biometric list hardening) and the `apps/customer-portal/README.md` pointer. |

Coverage: every locked item in `PROMPT.md` §“Locked Implementation Details To Preserve Exactly” is carried by exactly one derived slice (see each slice’s “Locked Items Covered”).

---

## 4. Dependency Graph And Sequencing

```
S0 ── Pre-Code Verifications & Repo Wiring
 │
 ├─► S1 FE Scaffold & Shell ───────────────┐
 │                                         │
 ├─► S2 BE Package & Contract Boundaries ──┤
 │        │                                │
 │        ├─► S3 Mistra NG Proxy Flows ────┼─► S5a Stato Aziende
 │        │                                │
 │        └─► S4 Biometric DB Flows ──────┐│
 │                                        │├─► S5b Gestione Utenti
 │                                        ││
 │                                        │└─► S5c Accessi Biometrico
 │                                        │
 └────────────────────────────────────────┴─► S6 Docs & TODO Reconciliation
                                                      │
                                                      ▼
                                              Final E2E QA & Signoff
```

- **Critical path**: S0 → S2 → (S3 ∥ S4) → S5 routes → S6 → Final QA.
- **Parallelism**:
  - S1 runs alongside S2 once S0 is done.
  - S3 and S4 run in parallel once S2 is done.
  - S5a needs S3 + S1; S5b needs S3 + S1; S5c needs S4 + S1.
- **Serialization**: S0 blocks all others; S6 blocks Final QA.

---

## 5. Repo Anchors (reuse, do not reinvent)

Verified in-repo anchors the executors must reuse:

- `@mrsmith/ui` — `AppShell` and `TabNav` exported from `packages/ui/src/index.ts`. Call-site template from `apps/compliance/src/App.tsx:22-32`:
  ```tsx
  <AppShell userName={user?.name ?? 'John Doe'} onLogout={logout}>
    <AppShell.Nav><TabNav items={navItems} /></AppShell.Nav>
    <AppShell.Content>{/* content */}</AppShell.Content>
  </AppShell>
  ```
- Arak client — `backend/internal/platform/arak/client.go:25-33` defines `Client`; main method `Client.Do(method, path, queryString string, body io.Reader) (*http.Response, error)` at `client.go:98`. Reference consumer: `backend/internal/afctools/gateway.go:38`.
- `httputil.InternalError` — `backend/internal/platform/httputil/respond.go:20`: `func InternalError(w http.ResponseWriter, r *http.Request, err error, message string, attrs ...any)`.
- ACL / auth middleware — `backend/cmd/server/main.go:369` mounts `Recover`, `RequestID`, `CORS`, `AccessLog`, `authMiddleware.Handler`. New routes inherit these automatically by mounting under `/api`.
- Catalog constants + role helpers — pattern at `backend/internal/platform/applaunch/catalog.go:11-12` (id/href constants) and `:387-389` (`BudgetAccessRoles()`); roles declared as module-level vars at `:51-68`.
- Config AppURL pattern — `backend/internal/platform/config/config.go:21` (`ComplianceAppURL string`), loaded at `:95` via `envOr("COMPLIANCE_APP_URL", "")`; `MistraDSN` at `:38` / `:108`; CORS defaults at `:92` (extend with `http://localhost:5187`).

---

## 6. Slices

Each slice specifies: Objective · Boundary rationale · Owned files · Inputs/deps · Tasks · Acceptance · Tests · Manual QA artifacts · Docs · Executor · QA role · QA checklist · Rollback notes · Locked items covered.

### Slice S0 — Pre-Code Verifications & Repo Wiring

**Objective**: unblock every other slice by adding dev wiring, CORS, catalog entry, config/env exposure, server mount, and Docker static copy — without touching product code.

**Boundary rationale**: infra/repo-fit work is high-risk for cross-file drift and benefits from single-owner, atomic delivery; isolated so UI and BE slices can run in parallel without editing the same wiring files.

**Owned files**:
- `apps/customer-portal/README.md` (add migration-workspace pointer to `apps/cp-backoffice/`, mirror `apps/zammu/` split pattern).
- root `package.json` — add `dev:cp-backoffice`; extend `dev` concurrently `--names`, `--prefix-colors`, and filter list in lockstep.
- root `Makefile` — add `dev-cp-backoffice` and add it to `.PHONY`.
- `backend/internal/platform/config/config.go` — add `CPBackofficeAppURL` field + `CP_BACKOFFICE_APP_URL` env; add `http://localhost:5187` to default CORS origins.
- `backend/.env.example` — add `CP_BACKOFFICE_APP_URL`.
- `.env.preprod.example` — add `CP_BACKOFFICE_APP_URL`.
- `backend/internal/platform/applaunch/catalog.go` — add SMART APPS entry: `ID: "cp-backoffice"`, `Href: "/apps/cp-backoffice/"`, `Icon: "users"`, `Status: "ready"`, `AccessRoles: CPBackofficeAccessRoles()`; add id/href constants `CPBackofficeAppID`, `CPBackofficeAppHref`, role helper `CPBackofficeAccessRoles()`; **remove** the superseded commented `customer-portal` placeholder (catalog.go:242-250); **leave the commented `customer-portal-settings` placeholder untouched** (catalog.go:280-288).
- `backend/cmd/server/main.go` — split-server href override to `http://localhost:5187` when `StaticDir == ""`; reserve the `cpbackoffice.RegisterRoutes(...)` mount call for S2; launcher visibility gate: include only when `arakCli != nil` and `cfg.MistraDSN != ""`.
- `deploy/Dockerfile` — `COPY --from=frontend /app/apps/cp-backoffice/dist /static/apps/cp-backoffice`.

**Inputs/deps**: none.

**Pre-Code Verifications (required before any code lands)**:
1. Mistra NG error body shape still exposes `message` — verify via the `backend/internal/platform/arak` client and `docs/mistra-dist.yaml` error schemas. Record finding in the slice report. If gone, pause before S3.
2. Vite port `5187` unclaimed — verify across `apps/*/vite.config*` and root configs.

**Tasks**:
1. Run the two pre-code verifications; attach findings.
2. Add config field and env var; grow CORS origin list.
3. Grow root scripts/targets in lockstep (names + colors + filter).
4. Register catalog entry with visibility gate; remove superseded `customer-portal` comment; leave `customer-portal-settings` comment untouched.
5. Add dev href override in `main.go`.
6. Add Docker static copy.
7. Write the migration-workspace pointer in `apps/customer-portal/README.md`.

**Acceptance criteria**:
- `make dev-cp-backoffice` and `pnpm dev:cp-backoffice` both resolve.
- `/config` bootstrap returns the launcher entry only when Arak and Mistra are both configured.
- `apps/customer-portal/README.md` contains a pointer to `apps/cp-backoffice/` mirroring the `apps/zammu/` split pattern.
- Forbidden placeholder NOT removed: `customer-portal-settings` comment remains.

**Test plan**: Go unit test or review check confirming the catalog filter hides the entry when `cfg.MistraDSN == ""` OR `arakCli == nil`. No frontend tests yet.

**Manual QA artifacts**: diff of root `package.json` showing names/colors/filter grown in lockstep; diff of `catalog.go` showing the entry gated and the superseded comment removed.

**Docs update**: `apps/customer-portal/README.md`.

**Executor**: Repo-Wiring Executor. **QA**: Slice QA Agent (via `review-strict`).

**QA checklist**:
- [ ] Role id matches `app_{appname}_access` convention → `app_cpbackoffice_access`.
- [ ] Catalog id `cp-backoffice`, href `/apps/cp-backoffice/`, icon `users`, status `ready`.
- [ ] Launcher visibility gated on `arakCli != nil && cfg.MistraDSN != ""`.
- [ ] Commented `customer-portal` removed; commented `customer-portal-settings` **left untouched**.
- [ ] Root `package.json` concurrently `--names`/`--prefix-colors`/filter list grew in lockstep.
- [ ] `Makefile` target present and in `.PHONY`.
- [ ] `CPBackofficeAppURL` + `CP_BACKOFFICE_APP_URL` added in all three env/config files.
- [ ] `http://localhost:5187` added to default CORS origins.
- [ ] `deploy/Dockerfile` static copy line present and exact.
- [ ] Pre-code verifications documented.

**Rollback**: all changes additive; revert the S0 PR.

**Locked items covered**: repo-fit (package/app id, routes, base path, API prefix reservation, role id, dev port, CORS, env vars, config field, catalog entry, Dockerfile copy, runtime visibility gating), plus the superseded-vs-untouched comment rule.

---

### Slice S1 — FE Scaffold & Shell

**Source parent**: Slice 1.

**Objective**: stand up the SPA with the standard mini-app shell and three business-labeled routes. No business logic yet.

**Boundary rationale**: UI shell is independent of BE contracts and can run in parallel with S2; keeping it small prevents Appsmith-like chrome creep.

**Owned files**:
- `apps/cp-backoffice/package.json` — `name: "mrsmith-cp-backoffice"`, scripts mirroring existing apps, deps on `@mrsmith/ui`, `@mrsmith/auth-client`, `@mrsmith/api-client`.
- `apps/cp-backoffice/tsconfig.json`, `apps/cp-backoffice/tsconfig.node.json`.
- `apps/cp-backoffice/vite.config.ts` — build base `/apps/cp-backoffice/`, dev base `/`, `server.port 5187`, proxy `/api` and `/config` to `process.env.VITE_DEV_BACKEND_URL || http://localhost:8080`.
- `apps/cp-backoffice/index.html`.
- `apps/cp-backoffice/src/main.tsx`, `src/App.tsx`, `src/routes.tsx`, `src/navigation.ts`.
- `apps/cp-backoffice/src/styles/*` — import clean theme from `@mrsmith/ui`.

**Inputs/deps**: S0.

**Tasks**:
1. Scaffold mirroring `apps/budget`, `apps/compliance`, `apps/listini-e-sconti`. No bespoke layout.
2. `AppShell` + `TabNav` (not `TabNavGroup`) reusing the `apps/compliance/src/App.tsx:22-32` template, with items:
   - `Stato Aziende` → `/stato-aziende`
   - `Gestione Utenti` → `/gestione-utenti`
   - `Accessi Biometrico` → `/accessi-biometrico`
3. Index route redirects to `/stato-aziende`. Do **not** reintroduce `Home`.
4. Auth bootstrap via shared `@mrsmith/auth-client` + `/config`.
5. Placeholder route components rendering empty states only.

**Acceptance criteria**:
- `pnpm --filter mrsmith-cp-backoffice dev` serves on `5187` with the three tabs visible.
- Deep-link refresh at `/apps/cp-backoffice/` and nested routes works in build mode (verify via `pnpm --filter mrsmith-cp-backoffice build` + preview).
- `/config` bootstrap succeeds in split-server dev.
- No KPI/stat chrome, no hero banner, no sidebar.

**Test plan**: `pnpm --filter mrsmith-cp-backoffice exec tsc --noEmit` (workspace TS 5.x, per CLAUDE.md).

**Manual QA artifacts**: screenshots of desktop shell + narrow viewport with scroll.

**Docs**: none at this slice.

**Executor**: FE Scaffold Executor. **QA**: Slice QA Agent (+ `ui-ux-reviewer`).

**QA checklist**:
- [ ] `AppShell` + `TabNav` (not `TabNavGroup`).
- [ ] Route labels exact: `Stato Aziende`, `Gestione Utenti`, `Accessi Biometrico`.
- [ ] No `Home`, no `Area documentale`, no KPI/stat row, no launcher hero.
- [ ] Forbidden copy absent.
- [ ] Build base `/apps/cp-backoffice/`, dev base `/`, Vite port `5187`, proxy on `/api` + `/config`.

**Rollback**: delete `apps/cp-backoffice/` and revert S0 entries for this app if needed.

**Locked items covered**: Slice 1 scaffolding and shell locks, route labels, routes, index redirect, clean mini-app family adherence.

---

### Slice S2 — BE Package & Contract Boundaries

**Source parent**: Slice 2.

**Objective**: create the Go module and its mount point with auth gating, dependency guards, and typed contracts — no upstream I/O yet beyond stubs.

**Boundary rationale**: isolating the package skeleton lets S3 and S4 run in parallel against a stable contract surface.

**Owned files**:
- `backend/internal/cpbackoffice/handler.go` — router, `RegisterRoutes`, `Deps` struct, guards.
- `backend/internal/cpbackoffice/doc.go` (optional package doc).
- `backend/cmd/server/main.go` — call `cpbackoffice.RegisterRoutes(...)` under the shared `/api` mux so `Recover`, `RequestID`, `CORS`, `AccessLog`, `authMiddleware.Handler` (see `main.go:369-375`) all apply automatically.

**Dependency shape** (per repo convention):
```go
type Deps struct {
    Arak    *arak.Client
    Mistra  *sql.DB
    Logger  *slog.Logger
}
```
Helpers `requireArak(d Deps) bool`, `requireMistra(d Deps) bool`, `dbFailure(w, r, err, op string)` — no package-global state.

**Endpoints registered** (bodies stubbed; real behavior lands in S3/S4). All gated by `app_cpbackoffice_access`:
- `GET    /api/cp-backoffice/v1/customers`
- `GET    /api/cp-backoffice/v1/customer-states`
- `PUT    /api/cp-backoffice/v1/customers/{id}/state`
- `GET    /api/cp-backoffice/v1/users?customer_id=...`
- `POST   /api/cp-backoffice/v1/admins`
- `GET    /api/cp-backoffice/v1/biometric-requests`
- `POST   /api/cp-backoffice/v1/biometric-requests/{id}/completion`

**Inputs/deps**: S0.

**Tasks**:
1. Declare package, `Deps`, `RegisterRoutes(mux, deps, acl)`.
2. Define typed request/response structs per endpoint; `BiometricRequestRow` with locked keys/types.
3. Implement guard helpers; stub handlers return `503` via `httputil.InternalError` when a dep is missing.
4. Structured logging with `component="cpbackoffice"` and route-level `operation` field.
5. Ensure no browser-facing endpoint bypasses auth middleware.

**Acceptance criteria**:
- All seven routes return `401` without auth and `403` without `app_cpbackoffice_access`.
- Missing Arak → `503` for Arak routes; missing Mistra → `503` for DB routes. Launcher tile hidden via S0 gate under these conditions.
- Internal 5xx responses use `httputil.InternalError` (signature: `func InternalError(w, r, err, message, attrs...)`); server logs keep the real cause.

**Test plan** (Go):
- `handler_test.go`: auth-gating test for the new route group (`401` / `403` / `200|stub-503` matrix).
- Dependency-guard tests for the `503` paths.

**Manual QA**: curl matrix showing auth and guard behavior.

**Executor**: BE Module Executor. **QA**: Slice QA Agent (+ `review-strict`).

**QA checklist**:
- [ ] Package path `backend/internal/cpbackoffice/`.
- [ ] `Deps` carries `Arak *arak.Client` and `Mistra *sql.DB`.
- [ ] Helpers named exactly `requireArak`, `requireMistra`, `dbFailure`; no package globals.
- [ ] All routes under `/api/cp-backoffice/v1/` and require `app_cpbackoffice_access`.
- [ ] Stubs use `httputil.InternalError` with `component="cpbackoffice"` and `operation`.
- [ ] `BiometricRequestRow` keys/types match the lock exactly.
- [ ] No anonymous pass-through maps where a typed struct applies.

**Rollback**: revert `main.go` mount and delete the package directory.

**Locked items covered**: API prefix, routes, role gating, dependency shape, helper names, logging/error surface, typed contracts.

---

### Slice S3 — Mistra NG Proxy Flows

**Source parent**: Slice 3.

**Objective**: implement the five Arak-backed handlers with exact upstream semantics.

**Boundary rationale**: upstream REST flows share error handling and query-param assembly; grouping them avoids duplicate harness code.

**Owned files**:
- `backend/internal/cpbackoffice/arak.go` (or split `customers.go`, `users.go`, `admins.go` if readability demands — all map back to Slice 3).
- `backend/internal/cpbackoffice/arak_test.go`.

**Inputs/deps**: S2.

**Tasks** (each handler uses `Deps.Arak.Do(method, path, queryString, body)` — see `backend/internal/platform/arak/client.go:98`):
1. `GET /customers` → Arak `GET /customers?disable_pagination=true`. Return full list. No frontend pagination.
2. `GET /customer-states` → Arak list endpoint with `disable_pagination=true`. Return full list.
3. `PUT /customers/{id}/state` → typed request `{ state_id: int64 }`. Proxy upstream state-edit endpoint. Surface upstream `message` on business error for the UI toast.
4. `GET /users?customer_id=...` → **backend guard**: reject missing or empty `customer_id` with `400`; do **not** proxy an invalid empty request upstream. When present, Arak `GET /users?customer_id={id}&disable_pagination=true`.
5. `POST /admins` → construct `user-admin-new`; request assembly **pins `skip_keycloak: false`** regardless of input; the hidden Appsmith switch is **not** exposed in v1. Re-enablement tracked in `docs/TODO.md`.
6. Return `httputil.InternalError` for transport / upstream 5xx; preserve real cause in logs.

**Acceptance criteria**:
- Each handler composes the correct path, query string, and body.
- `GET /users` returns `400` on missing/empty `customer_id` without hitting Arak.
- `POST /admins` request body always contains `"skip_keycloak": false`.
- Upstream `message` is surfaced on business error.

**Test plan** (Go, using `httptest` fake upstream):
- Arak proxy composition tests for path, query string (including `disable_pagination=true`), and request body for state update and admin creation (including `skip_keycloak: false`).
- `GET /users` empty-guard test.
- Upstream-error pass-through test preserving `message`.

**Manual QA**: modal-open state for `Stato Aziende` and `Nuovo Admin` against dev backend; upstream error simulated to confirm toast format.

**Executor**: BE Module Executor. **QA**: Slice QA Agent (+ `review-strict`).

**QA checklist**:
- [ ] `disable_pagination=true` on every list call.
- [ ] `GET /users` rejects empty `customer_id` locally.
- [ ] `createAdmin` pins `skip_keycloak: false`.
- [ ] No frontend pagination introduced.
- [ ] Upstream business `message` surfaced.
- [ ] Forbidden copy absent from any client-reachable strings.

**Rollback**: revert `arak.go` + tests; Slice S2 stubs remain viable.

**Locked items covered**: all Slice 3 behavioral locks, incl. hidden `skip_keycloak` and empty-`customer_id` guard.

---

### Slice S4 — Biometric DB Flows

**Source parent**: Slice 4.

**Objective**: implement the SQL-backed biometric list and completion endpoints with exact DTO vocabulary and ordering.

**Owned files**:
- `backend/internal/cpbackoffice/biometric.go`.
- `backend/internal/cpbackoffice/biometric_test.go`.

**Inputs/deps**: S2.

**Tasks**:
1. `GET /biometric-requests`: SQL join over exact anchors `customers.biometric_request`, `customers.user_struct`, `customers.customer`, `customers.user_entrance_detail`. Apply `ORDER BY data_richiesta DESC`. No pagination. No filters.
2. Response row keys fixed: `id`, `nome`, `cognome`, `email`, `azienda`, `tipo_richiesta`, `stato_richiesta` (bool), `data_richiesta`, `data_approvazione` (nullable), `is_biometric_lenel` (bool, returned but not rendered).
3. `POST /biometric-requests/{id}/completion`: call `customers.biometric_request_set_completed($1::bigint, $2::boolean)` with typed input `{ completed: bool }`; return `{ "ok": true }` on success.
4. Preserve boolean typing end-to-end; no string "ok"/"pending" mapping.

**Acceptance criteria**:
- List ordered by `data_richiesta DESC`.
- Nullable `data_approvazione` handled correctly.
- Completion mutation hits the exact function signature.
- `is_biometric_lenel` present in every row.

**Test plan** (Go):
- Biometric list scanning and ordering test, including nullable approval date handling.
- Biometric completion test asserting `bigint + boolean` function call and `{ok: true}` response.

**Manual QA**: populated desktop state, empty state, upstream-unavailable state for this route; narrow-width scroll prep for S5c.

**Executor**: BE Module Executor. **QA**: Slice QA Agent.

**QA checklist**:
- [ ] Exact table/join anchors used.
- [ ] Exact response keys and types.
- [ ] `ORDER BY data_richiesta DESC`.
- [ ] Stored function signature and return shape exact.
- [ ] `is_biometric_lenel` returned without rendering pressure into this slice.

**Rollback**: revert `biometric.go` + tests.

**Locked items covered**: all Slice 4 DB flow locks.

---

### Slice S5a — Stato Aziende

**Source parent**: Slice 5a.

**Objective**: table-first route with modal-backed state update.

**Owned files** (under `apps/cp-backoffice/src/`):
- `views/StatoAziende/StatoAziendePage.tsx`
- `views/StatoAziende/UpdateStateModal.tsx`
- `api/customers.ts`, `api/customerStates.ts`
- `hooks/useCustomers.ts`, `hooks/useCustomerStates.ts`, `hooks/useUpdateCustomerState.ts`

**Inputs/deps**: S1 + S3.

**Tasks**:
1. Fetch `GET /customers` and prefetch `GET /customer-states` on mount.
2. Single primary table; selecting a row enables CTA `Aggiorna {selectedCustomer.name}`.
3. CTA opens modal with a select backed by the prefetched states. Confirm label: `Conferma`.
4. On success: refetch customer list, close modal. On error: preserve business-facing toast format `{HTTP status} — {upstream message}`; fallback `Qualcosa e' andato storto`.
5. Empty-data state and upstream-unavailable state explicitly rendered.

**Acceptance criteria**:
- No KPI/stat row, no decorative summary.
- Table-first layout.
- CTA disabled when no row selected.
- Modal is the only mutation surface — no full detail page, no sticky save bar.

**Test plan**: `tsc --noEmit` and a smoke test of the hook against a mocked `fetch`. No broad snapshot or copy-only tests.

**Manual QA artifacts**: populated, empty, upstream-error, modal-open, narrow viewport.

**Executor**: FE Feature Executor. **QA**: Slice QA Agent (+ `ui-ux-reviewer`).

**QA checklist**:
- [ ] CTA exact: `Aggiorna {selectedCustomer.name}`.
- [ ] Confirm label exact: `Conferma`.
- [ ] Error toast format preserves HTTP status + message; fallback `Qualcosa e' andato storto`.
- [ ] No KPI/stat row.
- [ ] Forbidden copy absent.

**Rollback**: delete the view and its hooks; nav remains in place.

**Locked items covered**: Slice 5a behavioral locks.

---

### Slice S5b — Gestione Utenti

**Source parent**: Slice 5b.

**Objective**: customer-first selector, user table second, `Nuovo Admin` modal with correct DTO mapping.

**Owned files**:
- `views/GestioneUtenti/GestioneUtentiPage.tsx`
- `views/GestioneUtenti/CustomerSelector.tsx`
- `views/GestioneUtenti/NuovoAdminModal.tsx`
- `api/users.ts`, `api/admins.ts`
- `hooks/useUsersByCustomer.ts`, `hooks/useCreateAdmin.ts`

**Inputs/deps**: S1 + S3.

**Tasks**:
1. Render the exact greeting copy:
   `Ciao {operator.name || operator.email}, in questa applicazione vengono visualizzati tutti gli utenti inseriti per l'azienda selezionata - da indicare tramite la select`
   Operator name/email comes from auth bootstrap. No mention of the end-user app.
2. Customer select first (prefetched list reused from S5a if cached). **No user fetch runs until a customer is selected**.
3. `Nuovo Admin` button is **disabled until a customer is selected**.
4. Modal fields labeled exactly: `Nome`, `Cognome`, `Em@il`, `Telefono`, plus the notification checkbox group.
5. Notification checkbox group uses internal UI keys `'maintenance'` and `'marketing'`; **not part of the DTO**. Mapping at request assembly:
   - `'maintenance'` → `maintenance_on_primary_email`
   - `'marketing'` → `marketing_on_primary_email`
6. The hidden `skip_keycloak` switch is **not** rendered. BE pins `skip_keycloak: false` in S3; frontend must never send `skip_keycloak: true`.
7. Empty / no-selection state rendered. Zero user fetches before selection.
8. Confirm label: `Crea`.

**Acceptance criteria**:
- Zero `GET /users` requests before a customer is selected.
- `Nuovo Admin` button disabled until selection.
- DTO booleans mapped correctly from UI keys.
- Greeting copy rendered verbatim.

**Test plan**: `tsc --noEmit`; hook test asserting no fetch fires before selection.

**Manual QA artifacts**: populated, empty, no-selection, modal-open, narrow viewport.

**Executor**: FE Feature Executor. **QA**: Slice QA Agent (+ `ui-ux-reviewer`).

**QA checklist**:
- [ ] Exact greeting copy.
- [ ] `Nuovo Admin` disabled pre-selection.
- [ ] Zero user fetches pre-selection.
- [ ] Modal fields: `Nome`, `Cognome`, `Em@il`, `Telefono`.
- [ ] UI keys `'maintenance'` and `'marketing'` map to DTO `maintenance_on_primary_email` / `marketing_on_primary_email`; UI keys never sent on the wire.
- [ ] `skip_keycloak` switch absent from UI; frontend never sets it true.
- [ ] Confirm label `Crea`.
- [ ] Forbidden copy absent (notably no `Keycloak` mention).

**Rollback**: delete the view + its hooks.

**Locked items covered**: Slice 5b locks incl. UI-key→DTO mapping, no-fetch guard, disabled CTA, exact greeting.

---

### Slice S5c — Accessi Biometrico

**Source parent**: Slice 5c.

**Objective**: flat table with editable checkbox on `stato_richiesta`, row-level Save and Discard.

**Owned files**:
- `views/AccessiBiometrico/AccessiBiometricoPage.tsx`
- `api/biometric.ts`
- `hooks/useBiometricRequests.ts`, `hooks/useSetBiometricCompleted.ts`

**Inputs/deps**: S1 + S4.

**Tasks**:
1. Fetch `GET /biometric-requests` on mount.
2. Render flat table with columns in source order; **lowercase source labels preserved verbatim in v1**:
   `nome`, `cognome`, `email`, `azienda`, `tipo_richiesta`, `stato_richiesta`, `data conferma`, `data della richiesta`.
3. `stato_richiesta` is an editable boolean checkbox per row.
4. Row-level actions: **Save** (triggers `POST /biometric-requests/{id}/completion` then refetch) and **Discard** (local revert only). Both visible when a row is dirty.
5. Narrow widths: table scrolls horizontally; **no cardification**.
6. `is_biometric_lenel` returned from API but not rendered.
7. Do **not** port dead handlers `howAlert('success')`, `onCheckChange`.
8. Success toast: `Perfetto, stato biometrico cambiato`. Error fallback: `Qualcosa e' andato storto`.

**Acceptance criteria**:
- Lowercase source labels preserved.
- Save triggers mutation then refetch; Discard is local.
- `is_biometric_lenel` not rendered.
- Narrow viewport uses horizontal scroll.

**Test plan**: `tsc --noEmit`; hook test for the completion mutation (payload shape and refetch behavior).

**Manual QA artifacts**: populated, empty, upstream-error, inline row-edit (Save + Discard visible), narrow viewport.

**Executor**: FE Feature Executor. **QA**: Slice QA Agent (+ `ui-ux-reviewer`).

**QA checklist**:
- [ ] Column labels exactly `nome`, `cognome`, `email`, `azienda`, `tipo_richiesta`, `stato_richiesta`, `data conferma`, `data della richiesta`.
- [ ] Row-level Save + Discard; no modal, no full detail page.
- [ ] Horizontal scroll on narrow widths; no cardification.
- [ ] `is_biometric_lenel` not rendered.
- [ ] Dead handlers not ported.
- [ ] Success toast exact: `Perfetto, stato biometrico cambiato`.
- [ ] Error fallback exact: `Qualcosa e' andato storto`.
- [ ] Forbidden copy absent.

**Rollback**: delete the view + its hooks.

**Locked items covered**: Slice 5c locks, incl. exception 2 (inline Save/Discard inside `master_detail_crud`) and exception 3 (v1 lowercase labels).

---

### Slice S6 — Docs & TODO Reconciliation

**Objective**: land the deferred-work breadcrumbs and the migration-workspace pointer.

**Boundary rationale**: centralizing TODO updates avoids drift and is prerequisite to Final QA.

**Owned files**:
- `docs/TODO.md` — append three entries:
  1. **Biometric label polish** — column headers currently lowercase in v1 for operator parity; target proper Italian labels in a follow-up.
  2. **`skip_keycloak` re-enablement path** — hidden Appsmith switch intentionally omitted in v1; track the re-enablement surface if/when operators need it.
  3. **Biometric list defensive ceiling / filtering** — unpaginated list preserved in v1 by design; track server-side filtering + ceiling for scale.
- `apps/customer-portal/README.md` finalized if not already completed in S0.

**Inputs/deps**: S5a, S5b, S5c.

**Tasks**: append the three entries with rationale and pointers back to source slices; ensure pointer file references `apps/cp-backoffice/`.

**Acceptance criteria**: all three TODO entries present with rationale.

**Test plan**: none beyond review.

**Manual QA**: diff review.

**Executor**: Repo-Wiring Executor. **QA**: Slice QA Agent.

**QA checklist**:
- [ ] All three TODO entries present.
- [ ] Rationale (not just a one-liner) included.
- [ ] README pointer present and mirrors `apps/zammu/` split pattern.

**Rollback**: revert the commit.

---

## 7. Risk Register

| # | Risk | Likelihood | Impact | Mitigation |
| --- | --- | --- | --- | --- |
| R1 | Mistra NG error body no longer exposes `message` → toast formats break. | Low | Medium | Pre-code verification in S0. If gone, pause before S3. |
| R2 | Vite port `5187` collides with another local/CI override. | Low | Low | Pre-code verification in S0. |
| R3 | Unpaginated biometric list returns large volumes and degrades UX. | Medium | Medium | v1 deliberately preserves source behavior; defensive ceiling tracked in `docs/TODO.md`. Monitor post-launch. |
| R4 | `skip_keycloak` accidentally flips to `true` via UI/DTO drift. | Low | High | Pin at both FE request assembly and BE handler. QA in S3 and S5b. |
| R5 | Row-level Save/Discard in `Accessi Biometrico` drifts into a modal during review. | Low | Low | Exception 2 documented; QA verifies preservation. |
| R6 | Lowercase biometric labels “fixed” by a well-meaning reviewer before v1 ships. | Low | Low | Exception 3 locked; QA enforces verbatim labels. |
| R7 | Launcher tile advertises a broken app when Arak/Mistra unconfigured. | Low | Medium | Visibility gate `arakCli != nil && cfg.MistraDSN != ""` in S0; handlers still return `503`. |
| R8 | Dev wiring lists (`--names`, `--prefix-colors`, filters) drift out of lockstep. | Medium | Low | S0 QA checklist item; `review-strict` verifies. |
| R9 | Cross-database / cross-ID confusion when wiring customers. | Low | Medium | All ids are upstream-owned; this app creates no primary keys. Reference `docs/IMPLEMENTATION-KNOWLEDGE.md` during S3. |

---

## 8. Rollout Plan

1. Merge S0 behind the launcher visibility gate (tile invisible in any env lacking Arak + Mistra). Safe to merge early.
2. Merge S1 and S2 in parallel once S0 is green; launcher tile begins showing up in configured envs but routes still stub.
3. Merge S3 and S4 in parallel after S2.
4. Merge S5a/S5b/S5c as each unblocks; each route is usable independently.
5. Merge S6 last.
6. Final E2E QA runs against a fully-wired pre-prod environment; signoff report attached.

---

## 9. Rollback Plan

- **App-level kill switch**: remove the catalog entry from `backend/internal/platform/applaunch/catalog.go` to hide the launcher tile across environments. Routes remain auth-gated; no browser traffic reaches them.
- **Per-slice rollback**: revert the slice’s PR. S0/S6 are additive; S1 is self-contained under `apps/cp-backoffice/`; S2–S4 are self-contained under `backend/internal/cpbackoffice/`; S5a–c each delete cleanly.
- **Data rollback**: none required. App creates no primary keys, runs no migrations, and mutations go through locked upstream contracts (Arak + stored function `customers.biometric_request_set_completed`).

---

## 10. Final Validation Checklist

Confirm each has survived intact — both the locked value and the rationale.

### 10.1 Copy And Labels
- [ ] Route labels: `Stato Aziende`, `Gestione Utenti`, `Accessi Biometrico`.
- [ ] Action labels: `Aggiorna`, `Conferma`, `Nuovo Admin`, `Crea`.
- [ ] Success toast: `Perfetto, stato biometrico cambiato`.
- [ ] Error fallback toast: `Qualcosa e' andato storto`.
- [ ] Biometric column labels verbatim (lowercase v1 parity): `nome`, `cognome`, `email`, `azienda`, `tipo_richiesta`, `stato_richiesta`, `data conferma`, `data della richiesta`.
- [ ] Greeting copy exact: `Ciao {operator.name || operator.email}, in questa applicazione vengono visualizzati tutti gli utenti inseriti per l'azienda selezionata - da indicare tramite la select`.
- [ ] Forbidden terms absent: `server-side`, `datasource`, `widget`, `record`, `id.asc`, `Arak`, `Mistra`, `Keycloak`, `replica dell'app originale`, implementation-mechanics language.
- [ ] No metrics, no KPI cards.

### 10.2 Archetype And Comparable Apps
- [ ] Archetype `master_detail_crud` only.
- [ ] Comparable-app anchors respected: `apps/budget`, `apps/listini-e-sconti`, `apps/compliance`. `apps/reports/src/pages/OrdiniPage.tsx` pattern rejected.
- [ ] No Appsmith sidebar clone; top-nav `TabNav` used.

### 10.3 Repo-Fit
- [ ] App path `apps/cp-backoffice/`; migration workspace `apps/customer-portal/` retained with README pointer.
- [ ] Package name `mrsmith-cp-backoffice`.
- [ ] Build base `/apps/cp-backoffice/`; dev base `/`.
- [ ] Client routes `/stato-aziende`, `/gestione-utenti`, `/accessi-biometrico`; index redirects to `/stato-aziende`.
- [ ] `AppShell` + `TabNav` (not `TabNavGroup`).
- [ ] API prefix `/api/cp-backoffice/v1/`.
- [ ] Backend package `backend/internal/cpbackoffice/`; mounted via `cpbackoffice.RegisterRoutes(...)` from `backend/cmd/server/main.go`.
- [ ] `Deps{Arak *arak.Client; Mistra *sql.DB}`; helpers `requireArak`, `requireMistra`, `dbFailure`.
- [ ] Role `app_cpbackoffice_access`.
- [ ] Vite port `5187`; proxy `/api` + `/config` to `process.env.VITE_DEV_BACKEND_URL || http://localhost:8080`.
- [ ] CORS default origins include `http://localhost:5187`.
- [ ] Root `package.json` adds `dev:cp-backoffice`; concurrently `--names`, `--prefix-colors`, filter list grew in lockstep.
- [ ] `Makefile` has `dev-cp-backoffice` in `.PHONY`.
- [ ] Catalog entry: id `cp-backoffice`, href `/apps/cp-backoffice/`, icon `users`, status `ready`, access roles `CPBackofficeAccessRoles()`.
- [ ] Superseded `customer-portal` placeholder removed; `customer-portal-settings` placeholder **left untouched**.
- [ ] Split-server href override to `http://localhost:5187` when `StaticDir == ""`.
- [ ] `deploy/Dockerfile` contains `COPY --from=frontend /app/apps/cp-backoffice/dist /static/apps/cp-backoffice`.
- [ ] `CPBackofficeAppURL` + `CP_BACKOFFICE_APP_URL` added to `config.go`, `backend/.env.example`, `.env.preprod.example`.
- [ ] No new DSN env var added; reuse of `MISTRA_DSN`, `ARAK_BASE_URL`, `ARAK_SERVICE_CLIENT_ID`, `ARAK_SERVICE_CLIENT_SECRET`, `ARAK_SERVICE_TOKEN_URL` confirmed.

### 10.4 Contract Locks
- [ ] Browser never calls `gw-int.cdlan.net` directly.
- [ ] Browser never connects to Mistra PostgreSQL directly.
- [ ] `Home` page not reintroduced.
- [ ] `Area documentale` not in scope.
- [ ] Dead source handlers not ported: `howAlert('success')`, `onCheckChange`, `JSObject1`, `JSObject2`, `Api1`, `Query1`.
- [ ] `Nuovo Admin` disabled until customer selection.
- [ ] User list fetch deferred until customer selection.
- [ ] `skip_keycloak` hidden in UI and pinned `false` in `createAdmin` request assembly.
- [ ] `BiometricRequestRow` keys and boolean types fixed.
- [ ] No KPI cards, stat rows, decorative summaries, launcher hero, or Appsmith shell chrome.
- [ ] `disable_pagination=true` on all list upstream calls.
- [ ] `GET /users` rejects missing/empty `customer_id` locally without proxying upstream.
- [ ] Biometric query preserves anchors `customers.biometric_request`, `customers.user_struct`, `customers.customer`, `customers.user_entrance_detail`.
- [ ] Biometric response keys exact; `ORDER BY data_richiesta DESC`; boolean `stato_richiesta` end-to-end; `is_biometric_lenel` returned but not rendered.
- [ ] Completion call: `customers.biometric_request_set_completed($1::bigint, $2::boolean)`; returns `{ ok: true }`.

### 10.5 Exceptions (Value + Rationale)
- [ ] Top nav replaces source sidebar — **rationale**: consistency with mini-app family, lower maintenance.
- [ ] Inline row Save/Discard remains in `Accessi Biometrico` inside `master_detail_crud` — **rationale**: preserves current operator flow without a modal or full detail page.
- [ ] Lowercase biometric labels preserved in v1 — **rationale**: exact operator-facing parity during port window; polish tracked in `docs/TODO.md`.

### 10.6 Tests And Manual Artifacts
- [ ] Backend auth-gating test for the new route group.
- [ ] Backend Arak proxy composition tests (path, query string, request bodies incl. `skip_keycloak: false`).
- [ ] Backend biometric list scanning + ordering test incl. nullable approval date.
- [ ] Backend biometric completion mutation test with `bigint + boolean`.
- [ ] No broad snapshot or copy-only tests added.
- [ ] Manual review artifacts delivered: populated state for all three routes, empty and no-selection state, upstream error state, modal-open state, inline row-edit state for biometric requests, narrow viewport state.
- [ ] Runtime/auth checks: `/config` bootstrap works in split-server dev on `5187`; deep-link refresh at `/apps/cp-backoffice/` and nested routes; all `/api/cp-backoffice/v1/*` require `app_cpbackoffice_access`; launcher tile hidden when Arak or Mistra DB config missing; browser traffic stays on local `/api`; `createAdmin` sends `skip_keycloak: false`; internal failures use `httputil.InternalError` with `component="cpbackoffice"` in server logs.

### 10.7 Rationale Survived Decomposition
- [ ] Comparable-app anchors cited at review time (budget, listini, compliance) with rejected pattern (reports/OrdiniPage) acknowledged.
- [ ] Archetype rationale: `master_detail_crud` chosen as smallest approved archetype fit; not broadened to `data_workspace`.
- [ ] Repo-fit rationale: full slug `cp-backoffice` for app id + base path + API prefix; standard top nav + `TabNav`; reuse of Arak + Mistra deps; no bespoke infra.
- [ ] Exception rationale recorded alongside the exception, not stripped.
- [ ] Accepted v1 tradeoffs reference `docs/TODO.md` follow-ups.
- [ ] Pre-gate framing preserved: verification artifacts, tests, and manual review artifacts all present before signoff.

---

## 11. Assumptions And Missing Information

- **No external deadline**: plan is optimized for clear sequencing, parallelism where safe, and pre-gate readiness.
- **No hidden scope**: only the seven named endpoints, three routes, and wiring tasks are in scope. `Area documentale` remains out of scope; `Home` is not reintroduced.
- **Mistra NG error shape** is assumed stable until verified in S0 (R1).
- **Vite port 5187** assumed free until verified in S0 (R2).
- **Split-server dev** is assumed to be the operator’s default dev mode; the split-server href override in `main.go` is wired for it.
- **Docs/TODO additions** are the single accepted place for deferred v1 tradeoffs (lowercase biometric labels, `skip_keycloak` re-enablement, biometric list ceiling). No other backlog system is assumed.

---

## 12. Verification (End-to-End)

How to verify the integrated work before signoff:

1. **Dev smoke**:
   - `make dev-cp-backoffice` serves the app on `5187`; `make dev` runs it alongside backend + other apps.
   - Browser hits `/apps/cp-backoffice/` via the backend; `/config` bootstrap returns the launcher entry; deep-link refresh works at root and on all three nested routes.
2. **Auth**:
   - Without `app_cpbackoffice_access`, all `/api/cp-backoffice/v1/*` routes return `403`. With the role, `200/400/503` as expected per dep availability.
3. **Launcher visibility**:
   - Remove either `ARAK_BASE_URL` or `MISTRA_DSN` → launcher tile disappears. Routes still return `503` if called directly.
4. **Contract checks (via browser devtools)**:
   - `Stato Aziende` list and modal update work; failing upstream surfaces `{HTTP status} — {message}` toast.
   - `Gestione Utenti`: zero `/users` calls before selecting a customer; `Nuovo Admin` disabled pre-selection; `POST /admins` payload contains `"skip_keycloak": false`.
   - `Accessi Biometrico`: row edit flips checkbox, Save posts completion and refetches, Discard is local; `is_biometric_lenel` not rendered.
5. **Tests**:
   - `cd backend && go test ./internal/cpbackoffice/...` passes (auth gating, Arak composition, biometric list + completion).
   - `pnpm --filter mrsmith-cp-backoffice exec tsc --noEmit` passes.
6. **Docker**:
   - `deploy/Dockerfile` build copies `apps/cp-backoffice/dist` into `/static/apps/cp-backoffice`; running the image serves the SPA at `/apps/cp-backoffice/`.
7. **Manual review artifacts** (checklist 10.6): screenshots captured for populated, empty, no-selection, upstream-error, modal-open, inline row-edit, and narrow viewport states.

---

## 13. Ready-To-Execute Summary

- Orchestrator starts by running S0 (including the two Pre-Code Verifications) and, on pass, dispatches S1 and S2 in parallel.
- Once S2 passes QA, S3 and S4 dispatch in parallel.
- S5a/S5b/S5c dispatch as their BE+FE prerequisites clear, each carrying their own QA.
- S6 runs after the three product slices, then Final E2E QA produces the signoff report against §10.

Any deviation (e.g., a reviewer proposing summary cards, capitalized biometric labels, or exposing `skip_keycloak`) is out of scope for v1 and must be routed through `docs/TODO.md` rather than landed inline.
