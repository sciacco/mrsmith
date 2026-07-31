# Frontend Platform and Builds

Frontend build pipeline, shared Vite/portal platform rules, and CSS platform quirks.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

### CSS Module Keyframes Are Scoped; Global Animation Names Wake Dormant Rules

- Context: frontend styling in any `apps/*` CSS module.
- Discovery: `@keyframes` declared in a CSS module are scoped to that module. An `animation: someName` without a local keyframe of that name is a reference to a **global** keyframe — it silently does nothing until someone later defines `someName` in `global.css`, at which point dormant animations wake up in pages nobody touched (observed with `rowEnter` in Binocolo Iniziative).
- Practical rule: before adding a keyframe name to `global.css`, grep the workspace for existing `animation:` references to that name; keep module animations fully local (keyframe + usage in the same module) unless a global animation is intended.
- Evidence: Binocolo `/aziende` implementation review, 2026-07-18 (issue #76 remediation).
- Used by: all frontend apps.
- Open questions: none.

### Backend-Served SPAs Must Be Copied Explicitly Into `/static/apps/<slug>`

- Context: production and pre-production deployments where the Go server serves multiple Vite bundles from a shared static root.
- Discovery: adding an app to the launcher catalog and giving it a Vite `base` like `/apps/reports/` is not enough to make it deployable. The final runtime image must also copy that app's built dist directory into `/static/apps/<slug>`, otherwise the `staticspa` handler has no `index.html` to fall back to and deep links return the backend's plain 404.
- Practical rule: every new backend-served SPA needs the full pathing chain verified together: launcher/catalog href, Vite build base, local dev override if needed, Docker `COPY --from=frontend /app/apps/<slug>/dist /static/apps/<slug>`, and a `staticspa` deep-link regression test.
- Evidence: `deploy/Dockerfile`, `backend/internal/platform/staticspa/handler.go`, `backend/internal/platform/applaunch/catalog.go`, and the 2026-04-15 production `reports` regression where `/apps/reports/` 404ed because `/static/apps/reports/index.html` was missing from the image.
- Used by: `budget`, `compliance`, `kit-products`, `listini-e-sconti`, `panoramica-cliente`, `quotes`, `reports`.
- Open questions: none.

### Frontend Production Builds Must Exclude Node-Only Test Files

- Context: Vite/React mini-apps with `src/**/*.test.ts` files that use Node's built-in test modules such as `node:test` or `node:assert/strict`.
- Discovery: Docker production builds run `pnpm install --frozen-lockfile && pnpm -r build` in a fresh frontend stage. If a mini-app build script uses `tsc -b` against the default app `tsconfig.json`, TypeScript compiles tests included by `"include": ["src"]`. In that fresh deploy image, Node typings are not guaranteed to be available, so Node-only test imports can break the production build even when the app bundle itself is valid.
- Practical rule: when adding local TypeScript test files to a frontend mini-app, add or reuse `tsconfig.build.json` that extends the app `tsconfig.json` and excludes `src/**/*.test.ts` / `src/**/*.test.tsx`, then point the package `build` script at `tsc -b tsconfig.build.json && vite build`. Keep the test script responsible for running those files directly.
- Evidence: `apps/simulatori-vendita/tsconfig.build.json`; `apps/fornitori/tsconfig.build.json`; `make deploy-prod` failure on 2026-04-28 from `apps/fornitori/src/lib/providerAttention.test.ts` and `providerState.test.ts` importing `node:test` during the Docker frontend stage.
- Used by: `apps/simulatori-vendita`, `apps/fornitori`.
- Open questions: none.

### Packages Imported By `vite.config.ts` Must Ship Runnable JavaScript

- Context: shared workspace packages consumed from an app's `vite.config.ts`, such as `@mrsmith/vite-config`.
- Discovery: workspace packages normally ship TypeScript source (`"main": "src/index.ts"`), which works for app code because Vite transpiles it. Config files are different: Vite bundles `vite.config.ts` with esbuild but externalizes bare imports, so Node itself loads the imported package at config-load time. Locally this can still work by accident (Node ≥ 22.18 strips types natively), but the Docker frontend stage runs `node:20-slim`, which fails with `ERR_UNKNOWN_FILE_EXTENSION` on `.ts`.
- Practical rule: any workspace package meant to be imported from `vite.config.ts` (or any other Node-executed config) must ship plain ESM JavaScript with a hand-written `.d.ts` for editor types — no `.ts` entry point, no build step. Local success does not prove Docker success; verify with `docker build --target frontend -f deploy/Dockerfile .`.
- Evidence: `packages/vite-config/src/index.js`; `make deploy-prod` failure on 2026-07-13 (`apps/compliance build: ERR_UNKNOWN_FILE_EXTENSION ... /app/packages/vite-config/src/index.ts`) after the issue #49 migration, local Node 26 vs Docker `node:20-slim`.
- Used by: `packages/vite-config` and all 24 mini-app `vite.config.ts` files.
- Open questions: none.

### Docker Frontend Builds Must Exclude Local Vite Env Files

- Context: Vite mini-app production images built by `deploy/Dockerfile`.
- Discovery: `.gitignore` excludes `.env.local`, but Docker does not use `.gitignore`. Without a matching `.dockerignore`, `COPY apps/ apps/` can copy app-local Vite env files into the frontend build stage. Vite then inlines `VITE_*` values at build time, so a local `VITE_DEV_AUTH_BYPASS=true` can produce a production bundle that bypasses Keycloak and sends the literal `dev-token`.
- Practical rule: production Docker contexts must exclude `**/.env.local` and `**/.env.*.local`, and production bundles should be checked for dev-auth markers such as `dev-token` or `VITE_DEV_AUTH_BYPASS=true` before deploy. Runtime env changes cannot repair an already-built Vite bundle; rebuild from a clean context.
- Evidence: `deploy/Dockerfile` copies `apps/` wholesale; no repo `.dockerignore` was present during the 2026-04-28 Fornitori production auth incident; `apps/fornitori/.env.local` contained `VITE_DEV_AUTH_BYPASS=true`; the built `apps/fornitori/dist` bundle inlined the bypass condition as true and included `dev-token`.
- Used by: all Vite mini-app production builds, especially apps using `@mrsmith/auth-client`.
- Open questions: whether to add a hard production-build guard in `@mrsmith/auth-client` in addition to Docker context hygiene.

### Portal Launcher Tiles Must Use Supported Portal Icon Keys

- Context: adding or changing entries in `backend/internal/platform/applaunch/catalog.go`.
- Discovery: launcher tile icons are rendered from the portal-local registry in `apps/portal/src/components/Icon/icons.tsx`, not from an open-ended icon namespace. Reusing a string that is not in that registry leaves the tile without a matching portal icon; during Energia in DC wiring, `bolt` was rejected and the tile used the already-supported `chart` key instead.
- Practical rule: when wiring a new launcher tile, verify the icon key against `apps/portal/src/components/Icon/icons.tsx` or reuse an already-proven key from `apps/portal/src/data/apps.ts`. Do not invent icon names in `catalog.go` without checking portal support first.
- Evidence: `apps/portal/src/components/Icon/icons.tsx`, `apps/portal/src/data/apps.ts`, `backend/internal/platform/applaunch/catalog.go`.
- Used by: launcher-backed apps including `reports`, `coperture`, and `energia-dc`.
- Open questions: none.
