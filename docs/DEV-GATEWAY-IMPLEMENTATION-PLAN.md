# Single-Origin Dev Gateway For Mini-Apps — Implementation Plan

Source: `docs/IMPLEMENTATION-PLANNING.md`, `docs/AUTH-PLAN1.md`
Date: 2026-04-07
Status: Draft — future implementation
Execution target: This document is written for LLM-agent execution. It is intended to be decision-complete and directly implementable.

---

## Repo-Fit Checklist

### 1. Runtime Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| Browser origin in dev | `http://localhost:8080` is the only browser origin | `backend/cmd/server/main.go` already owns `/api`, `/config`, and the main HTTP server |
| Canonical app paths | Portal stays at `/`; mini-apps stay at `/apps/<app_id>/` | `docs/AUTH-PLAN1.md` defines the shared-origin production path model |
| Deep-link fallback | Reuse the existing SPA fallback behavior for root and `/apps/<id>` routes | `backend/internal/platform/staticspa/handler.go` and `handler_test.go` |
| Dev proxy scope | Only non-API app routes are gateway-managed; `/api/*`, `/config`, and health stay backend-owned | `backend/cmd/server/main.go` current route ownership |

### 2. Dev Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| Root workflow | Root `pnpm dev` changes from "start everything" to "backend + portal" | `package.json` currently starts backend, portal, budget, compliance together |
| App opt-in | Budget and compliance keep their dedicated `pnpm --filter ... dev` commands; future apps follow the same pattern | Root `package.json`, `Makefile` |
| Existing split-server pattern | Remove backend launcher rewrites to `http://localhost:517x` and replace them with gateway routing | `backend/cmd/server/main.go` currently rewrites budget/compliance hrefs in dev |
| Current app ports | Portal uses Vite default `5173`; budget uses `5174`; compliance uses `5175` | `apps/portal/vite.config.ts`, `apps/budget/vite.config.ts`, `apps/compliance/vite.config.ts` |

### 3. Auth Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| Auth bootstrap | Frontends continue fetching `/config` from the current browser origin | `apps/portal/src/main.tsx`, `apps/budget/src/main.tsx`, `apps/compliance/src/main.tsx` |
| API transport | Frontends continue calling `/api/*` relative to the current browser origin | Existing Vite proxy setup and shared API client usage |
| CORS direction | Long-term dev target is fewer browser origins, not more | `backend/internal/platform/config/config.go` currently encodes per-port CORS defaults |

### 4. Deployment Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| Production static hosting | No production route change; dev gateway must align with existing `/static` layout | `deploy/Dockerfile` copies portal to `/static` and mini-apps to `/static/apps/<id>` |
| Dist fallback | Local dev may serve existing built artifacts when the live dev server is absent | Existing build output locations under `apps/<app>/dist` |

### 5. Verification Fit

| Item | Decision | Verified Against |
|------|----------|-----------------|
| Deep-link refresh | Must remain covered by automated tests | `backend/internal/platform/staticspa/handler_test.go` |
| Missing asset behavior | Asset requests without a file must still return `404`, not HTML fallback | `backend/internal/platform/staticspa/handler_test.go` |
| Gateway correctness | Add backend tests for route precedence, proxy success, and static fallback | New work required |

---

## Resolved Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| DG1 | Dev browser origin is always `http://localhost:8080` | Keeps local routing, auth, and launcher behavior aligned with production |
| DG2 | Backend is the dev gateway | Backend already owns `/api`, `/config`, and the canonical app paths |
| DG3 | Catalog hrefs remain canonical `/apps/<app_id>/` in all environments | Launcher data must not leak Vite ports |
| DG4 | When an app dev server is unavailable, serve the app's built static bundle if present | Supports mixed live/static local development without changing browser origin |
| DG5 | Dev route mappings live in one backend-owned manifest, not one env var per app | Per-app env vars do not scale with the catalog |
| DG6 | Root `pnpm dev` starts `backend + portal` only | Portal is the entrypoint; app dev servers are opt-in |
| DG7 | App Vite servers remain independent processes started explicitly by the developer | Preserves app boundaries while removing browser-visible port sprawl |
| DG8 | Public `/api/*` contracts do not change in this work | This is a runtime/dev-experience refactor, not an API redesign |
| DG9 | Do not collapse all mini-apps into one frontend workspace | The repo remains a multi-app monorepo |
| DG10 | If no dev server and no built bundle exist for an app route, return `404` | Avoid redirecting to the portal or inventing a hidden fallback state |

---

## Overview

Target behavior in local development:

- The browser opens only `http://localhost:8080/`.
- `/` loads the portal through the backend gateway.
- `/apps/budget/` and `/apps/compliance/` stay on the same browser origin.
- If the target app Vite server is running, the backend proxies the request to it.
- If the target app Vite server is not running and that app has a local `dist/`, the backend serves the built app from the canonical route.
- If neither a live server nor a built bundle exists, the backend returns `404`.

Target developer workflow:

- `pnpm dev` starts backend + portal.
- To work on a specific mini-app, the developer starts that app explicitly in another terminal:
  - `pnpm --filter mrsmith-budget dev`
  - `pnpm --filter mrsmith-compliance dev`
- The browser remains on `localhost:8080` throughout.

Route precedence in dev:

1. `/api/*`, `/config`, and health routes are always served by the backend.
2. `/` and non-API root routes proxy to the portal dev server when reachable, otherwise fall back to portal static output if present.
3. `/apps/<app_id>/*` proxy to the mapped app dev server when reachable, otherwise fall back to that app's static output if present.
4. Missing asset requests return `404`.

---

## Implementation Sequence

### Phase 1 — Backend Dev Gateway Foundation

**Goal:** Introduce backend-owned route selection for live app proxying versus static fallback.

**Create** `backend/internal/platform/devgateway/`:

- `manifest.go`
- `proxy.go`
- `handler.go`
- `testdata/` only if needed for handler tests

**Modify** `backend/cmd/server/main.go`:

- Remove `hrefOverrides` logic for budget/compliance local URLs.
- Build the app catalog with canonical hrefs only: `applaunch.Catalog(nil)`.
- In dev mode (`cfg.StaticDir == ""`), mount the new dev gateway on `/`.
- In production mode (`cfg.StaticDir != ""`), keep using `staticspa.New(cfg.StaticDir)`.
- Keep `/api/*`, `/config`, and health registration unchanged and higher-priority than the gateway.

**Implementation rules:**

- The gateway must never proxy `/api/*`, `/config`, or health endpoints.
- Proxying must use Go's reverse-proxy support with WebSocket/upgrade compatibility so Vite HMR works through `:8080`.
- Proxy failures must fall back to static output only for `GET` and `HEAD`.
- The gateway must preserve the original request path and query string when proxying.

### Phase 2 — Checked-In Dev Route Manifest

**Goal:** Replace per-app localhost env overrides with one repo-owned source of truth.

**Create** `backend/internal/platform/devgateway/apps.dev.json`.

**Manifest schema:**

```json
[
  {
    "app_id": "portal",
    "workspace_name": "mrsmith-portal",
    "dev_url": "http://localhost:5173",
    "base_path": "/",
    "dist_dir": "apps/portal/dist"
  }
]
```

**Required fields per entry:**

- `app_id`
- `workspace_name`
- `dev_url`
- `base_path`
- `dist_dir`

**Seed entries:**

- `portal`
- `budget`
- `compliance`

**Manifest handling rules:**

- Load the manifest from disk at runtime in dev mode.
- Resolve `dist_dir` relative to the repo root / process working directory.
- Match app routes by `base_path`, not by hardcoded if/else chains in `main.go`.
- Future apps are added by updating this manifest, not by adding new backend env vars.

### Phase 3 — Static Fallback Integration

**Goal:** Allow the dev gateway to serve existing built bundles using the same canonical routes.

**Modify** `backend/internal/platform/staticspa/handler.go`.

**Modify** `backend/internal/platform/staticspa/handler_test.go`.

**Required refactor:**

- Extend `staticspa` so it can serve from either:
  - the existing merged production static root, or
  - a manifest-defined per-app dist directory layout in local dev
- Keep the existing production behavior intact
- Add a constructor or option-based API rather than encoding dev-only branching into `main.go`

**Fallback rules:**

- Portal root fallback serves `apps/portal/dist/index.html` in dev when portal live proxy is unavailable and the build exists.
- `/apps/budget/...` falls back to `apps/budget/dist/index.html` for deep links.
- `/apps/compliance/...` falls back to `apps/compliance/dist/index.html` for deep links.
- Requests for asset paths with extensions must return `404` if the file does not exist.

### Phase 4 — Frontend Dev-Server Compatibility

**Goal:** Ensure live Vite servers behave correctly when accessed through canonical app paths.

**Modify**:

- `apps/portal/vite.config.ts`
- `apps/budget/vite.config.ts`
- `apps/compliance/vite.config.ts`

**Required decisions:**

- Portal explicitly uses port `5173`.
- Budget keeps `5174`.
- Compliance keeps `5175`.
- Dev configs must support being reached through:
  - `/` for portal
  - `/apps/budget/` for budget
  - `/apps/compliance/` for compliance

**Implementation rules:**

- Keep router basename derived from `import.meta.env.BASE_URL`.
- Keep `/api` and `/config` browser calls relative-origin.
- Do not require developers to browse directly to `localhost:517x`.
- If a Vite config needs `base` differences for dev versus build, document the exact behavior in code comments.

### Phase 5 — Root Workflow Updates

**Goal:** Make the default local workflow match the gateway model.

**Modify**:

- `package.json`
- `Makefile`

**Required changes:**

- Root `pnpm dev` starts:
  - backend
  - portal
- Keep these explicit opt-in commands:
  - `pnpm --filter mrsmith-budget dev`
  - `pnpm --filter mrsmith-compliance dev`
- Keep existing `dev:budget` and `dev:compliance` commands available.
- Add `dev:portal` to the default root workflow if not already present.
- Update `Makefile` help text to state that `localhost:8080` is the primary browser URL.

**Non-goal:**

- Do not delete the dedicated per-app dev commands.

### Phase 6 — Catalog Cleanup

**Goal:** Keep launcher data environment-agnostic.

**Modify** `backend/internal/platform/applaunch/catalog.go`.

**Required changes:**

- Remove the `hrefOverrides` concept from the catalog API.
- Change `Catalog(hrefOverrides map[string]string)` to `Catalog() []Definition`.
- Remove the `strings` import if it becomes unused.
- Keep budget and compliance hrefs canonical:
  - `/apps/budget/`
  - `/apps/compliance/`
- Do not add future localhost URLs to launcher data.

**Follow-on requirement:**

- Update any tests or callers that still expect override behavior.

---

## Verification Checklist

### Automated

- Backend tests for the dev gateway:
  - portal root proxies to the portal dev server when reachable
  - `/apps/budget/...` proxies to the budget dev server when reachable
  - `/apps/compliance/...` proxies to the compliance dev server when reachable
  - proxy failure falls back to the correct app static bundle when present
  - `/api/*` and `/config` are not proxied
  - WebSocket/HMR upgrade paths do not break the proxy layer
- Static fallback tests:
  - root routes serve portal index fallback
  - app deep links serve app index fallback
  - missing asset requests still return `404`
- Catalog tests:
  - app hrefs remain canonical in all environments
  - no localhost override path remains in catalog generation

### Manual

1. Run `pnpm dev`.
2. Open `http://localhost:8080/` and confirm the portal loads.
3. Start `pnpm --filter mrsmith-budget dev`.
4. Open `http://localhost:8080/apps/budget/` and confirm the budget app loads without changing browser origin.
5. Verify budget HMR works while browsing through `localhost:8080`.
6. Stop the budget dev server and confirm `/apps/budget/` falls back to built output if `apps/budget/dist` exists.
7. Repeat the same checks for compliance.
8. Confirm `/config` still returns frontend auth settings and app login still works.
9. Confirm portal launcher links never show `localhost:517x`.

---

## File Inventory

### Backend Files To Create

- `backend/internal/platform/devgateway/manifest.go`
- `backend/internal/platform/devgateway/proxy.go`
- `backend/internal/platform/devgateway/handler.go`
- `backend/internal/platform/devgateway/apps.dev.json`

### Backend Files To Modify

- `backend/cmd/server/main.go`
- `backend/internal/platform/config/config.go`
- `backend/internal/platform/applaunch/catalog.go`
- `backend/internal/platform/staticspa/handler.go`
- `backend/internal/platform/staticspa/handler_test.go`
- `backend/internal/platform/applaunch/...` tests as needed for catalog API changes

### Frontend Files To Modify

- `apps/portal/vite.config.ts`
- `apps/budget/vite.config.ts`
- `apps/compliance/vite.config.ts`

### Root Workflow Files To Modify

- `package.json`
- `Makefile`

### Docs Files To Modify

- `docs/TODO.md`
- `MEMORY.md`

---

## Risks / Notes

- Reverse proxy plus Vite HMR is the main technical risk; implement proxying in backend tests before relying on manual verification.
- Static fallback and live proxy precedence must be deterministic; live dev server wins, static bundle is fallback only.
- This plan assumes built assets already exist for static fallback. It does not include automatic on-demand app builds.
- Avoid a second route model in dev. The point of this change is to make dev match the production path shape, not to hide a different architecture behind the gateway.
- The gateway manifest is a runtime/dev-experience contract. Keep it checked in and readable, not scattered across undocumented env vars.

---

## Assumptions

- Backend `:8080` remains the single browser-visible host in development.
- Only the actively edited mini-app needs a live Vite server.
- Static fallback uses existing `dist` artifacts only.
- Portal stays the default entrypoint and therefore starts with root `pnpm dev`.
- Future mini-app onboarding requires:
  - adding the frontend workspace
  - assigning a Vite port
  - adding one manifest entry
  - not adding new backend catalog localhost overrides
